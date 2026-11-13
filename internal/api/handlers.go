package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/metrics"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/go-chi/chi/v5"
)

// maxQuestionRunes — ограничение длины вопроса (защита от огромных промптов).
const maxQuestionRunes = 2000

type Handlers struct {
	Agent   *agent.Agent
	Store   *store.Store
	Chat    *ChatHandlers
	Metrics *metrics.Registry // может быть nil (в unit-тестах)
	Limiter *RateLimiter      // может быть nil — без ограничения
}

type askReq struct {
	Question string `json:"question"`
	Text     string `json:"text"`
}

func (r askReq) Q() string {
	if r.Text != "" {
		return r.Text
	}
	return r.Question
}

// validQuestion проверяет вопрос и пишет 400, если он пустой или слишком длинный.
func validQuestion(w http.ResponseWriter, q string) bool {
	if q == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "text required")
		return false
	}
	if utf8.RuneCountInString(q) > maxQuestionRunes {
		writeError(w, http.StatusBadRequest, "bad_request", "text too long (max 2000 chars)")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeOptional читает необязательное JSON-тело: пустое тело — не ошибка.
func decodeOptional(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// observe учитывает ответ агента в метриках пользы и запоминает вопросы без ответа.
func observe(m *metrics.Registry, st *store.Store, u auth.User, question string, resp agent.Response) {
	m.Inc("buddy_agent_responses_total", "Ответы помощника по типу", "type", resp.Type)
	if resp.Degraded {
		m.Inc("buddy_agent_degraded_total", "Ответы, данные резервной моделью")
	}
	if resp.NoSource {
		st.RecordGap(u.TeamID, question)
		m.Inc("buddy_knowledge_gaps_total", "Вопросы, на которые в базе знаний нет ответа")
	}
}

// Ask — синхронный вариант чата (удобен для проверки curl'ом).
func (h *Handlers) Ask(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "no user")
		return
	}
	var body askReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if !validQuestion(w, body.Q()) {
		return
	}
	resp, err := h.Agent.Ask(r.Context(), u, body.Q())
	if err != nil {
		h.Metrics.Inc("buddy_agent_errors_total", "Сбои агента (LLM недоступна)")
		writeError(w, http.StatusBadGateway, "agent_error", "Помощник временно недоступен, попробуйте ещё раз")
		return
	}
	saveDraft(h.Store, u, &resp)
	observe(h.Metrics, h.Store, u, body.Q(), resp)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MetricsHandler отдаёт метрики в формате Prometheus.
func (h *Handlers) MetricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	h.Metrics.Write(w)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	writeJSON(w, http.StatusOK, u)
}

type progress struct {
	Items       []store.ChecklistItem `json:"items"`
	NextStep    string                `json:"next_step"`
	PercentDone int                   `json:"percent_done"`
}

func progressOf(items []store.ChecklistItem) progress {
	p := progress{Items: items}
	done := 0
	for _, it := range items {
		if it.Done {
			done++
		} else if p.NextStep == "" && it.Status != store.StepWaiting {
			p.NextStep = it.Title
		}
	}
	if len(items) > 0 {
		p.PercentDone = done * 100 / len(items)
	}
	return p
}

func (h *Handlers) OnboardingProgress(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	writeJSON(w, http.StatusOK, progressOf(h.Store.Checklist(u.ID)))
}

type stepReq struct {
	Status string `json:"status"`
}

func (h *Handlers) OnboardingStepUpdate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	// Тело необязательно; если передан status — поддерживаем только done.
	var body stepReq
	if err := decodeOptional(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if body.Status != "" && body.Status != store.StepDone {
		writeError(w, http.StatusBadRequest, "bad_request", "only status=done is supported")
		return
	}
	found, owned := h.Store.MarkDoneOwned(u.ID, id)
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "step not found")
		return
	}
	if !owned {
		writeError(w, http.StatusForbidden, "forbidden", "step belongs to another user")
		return
	}
	h.Metrics.Inc("buddy_steps_done_total", "Шаги чек-листа, отмеченные выполненными")
	h.OnboardingProgress(w, r)
}
