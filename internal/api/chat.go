package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/metrics"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// heartbeatEvery — интервал комментария-пинга в SSE, чтобы прокси не рвали соединение.
var heartbeatEvery = 15 * time.Second

// maxPendingPerSession — сколько ответов помощника можно ждать одновременно в одном диалоге.
const maxPendingPerSession = 3

// maxMessagesPerSession — потолок сообщений в одном диалоге (вопрос + ответ = 2), защита памяти и файла состояния.
var maxMessagesPerSession = 200

type ChatHandlers struct {
	Agent   *agent.Agent
	Store   *store.Store
	Msgs    *store.MessageStore
	Broker  *Broker
	Metrics *metrics.Registry
}

// history собирает последние реплики диалога для модели (без текущего вопроса).
func (h *ChatHandlers) history(sessionID string) []llm.Message {
	var out []llm.Message
	for _, m := range h.Msgs.MessagesBySession(sessionID) {
		if m.Status != "done" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		out = append(out, llm.Message{Role: m.Role, Content: m.Content})
	}
	return out
}

// ownSession возвращает сессию текущего пользователя или пишет 403.
func (h *ChatHandlers) ownSession(w http.ResponseWriter, r *http.Request) (string, auth.User, bool) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	sess, ok := h.Msgs.GetSession(id)
	if !ok || sess.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your session")
		return "", u, false
	}
	return id, u, true
}

func (h *ChatHandlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	sess := h.Msgs.CreateSession(u.ID, uuid.NewString())
	writeJSON(w, http.StatusCreated, sess)
}

func (h *ChatHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	id, _, ok := h.ownSession(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.Msgs.MessagesBySession(id))
}

// emit публикует событие. В data дублируем ключи: message_id (фронт) и messageId (openapi), плюс type.
func (h *ChatHandlers) emit(sessionID, event, msgID string, kv map[string]any) {
	m := map[string]any{"type": event, "message_id": msgID, "messageId": msgID}
	for k, v := range kv {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	h.Broker.Publish(sessionID, SSEEvent{Event: event, Data: string(b)})
}

// lastAssistant — последнее сообщение помощника в сессии.
func (h *ChatHandlers) lastAssistant(sessionID string) (store.Message, bool) {
	ms := h.Msgs.MessagesBySession(sessionID)
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].Role == "assistant" {
			return ms[i], true
		}
	}
	return store.Message{}, false
}

// withDialogContext дополняет короткий ответ на уточнение помощника.
// Если помощник спросил дату отгула, а пользователь прислал только дату — продолжаем сценарий отгула.
func (h *ChatHandlers) withDialogContext(sessionID, q string) string {
	prev, ok := h.lastAssistant(sessionID)
	if !ok || prev.Result == "" {
		return q
	}
	var last agent.Response
	if json.Unmarshal([]byte(prev.Result), &last) != nil {
		return q
	}
	if last.Type != "draft" || last.Action != store.ActionAbsence || last.Args["action_id"] != "" {
		return q
	}
	if strings.Contains(strings.ToLower(q), "отгул") {
		return q
	}
	if from, _ := agent.ParseDatesAt(q, time.Now()); from != "" {
		return "Хочу отгул " + q
	}
	if _, ok := agent.ParseWeekday(q, time.Now()); ok {
		return "Хочу отгул " + q
	}
	return q
}

func (h *ChatHandlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	sessionID, u, ok := h.ownSession(w, r)
	if !ok {
		return
	}
	var body sendMessageReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	text := strings.TrimSpace(body.Q())
	if !validQuestion(w, text) {
		return
	}
	if h.Msgs.CountBySession(sessionID) >= maxMessagesPerSession {
		writeError(w, http.StatusTooManyRequests, "session_full", "Диалог слишком длинный — начните новый")
		return
	}
	if h.Msgs.PendingCount(sessionID) >= maxPendingPerSession {
		w.Header().Set("Retry-After", "3")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Помощник ещё отвечает на предыдущие вопросы, подождите")
		return
	}
	question := h.withDialogContext(sessionID, text)
	history := h.history(sessionID)

	userMsg := h.Msgs.CreateMessage(store.Message{
		ID: uuid.NewString(), SessionID: sessionID, Role: "user", Content: text,
		Status: "done", CreatedAt: time.Now(),
	})
	asstMsg := h.Msgs.CreateMessage(store.Message{
		ID: uuid.NewString(), SessionID: sessionID, Role: "assistant",
		Status: "pending", CreatedAt: time.Now(),
	})

	go h.runAgent(sessionID, asstMsg.ID, question, history, u)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"user_message_id":      userMsg.ID,
		"assistant_message_id": asstMsg.ID,
		"messageId":            asstMsg.ID, // openapi
		"status":               "pending",
	})
}

type sendMessageReq struct {
	Question string `json:"question"`
	Text     string `json:"text"`
}

func (r sendMessageReq) Q() string {
	if r.Text != "" {
		return r.Text
	}
	return r.Question
}

func (h *ChatHandlers) runAgent(sessionID, msgID, question string, history []llm.Message, u auth.User) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("agent panic", "session", sessionID, "message", msgID, "panic", rec)
			h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "failed"; m.Content = "internal error" })
			h.emit(sessionID, "message_done", msgID, map[string]any{"status": "failed"})
		}
	}()

	h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "in_progress" })
	h.emit(sessionID, "status", msgID, map[string]any{"status": "thinking"})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.Agent.AskWithHistory(ctx, u, question, history)
	h.Metrics.Add("buddy_agent_duration_seconds_sum", "Суммарное время работы агента", time.Since(start).Seconds())
	h.Metrics.Inc("buddy_agent_runs_total", "Запуски агента")
	if err != nil {
		h.Metrics.Inc("buddy_agent_errors_total", "Сбои агента (LLM недоступна)")
		slog.Warn("agent failed", "session", sessionID, "message", msgID, "err", err)
		h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "failed"; m.Content = "agent failed" })
		h.emit(sessionID, "error", msgID, map[string]any{"error": "Помощник временно недоступен. Попробуйте ещё раз."})
		h.emit(sessionID, "message_done", msgID, map[string]any{"status": "failed"})
		return
	}
	saveDraft(h.Store, u, &resp)
	observe(h.Metrics, h.Store, u, question, resp)

	if resp.Type == "action" && resp.Result != nil {
		h.emit(sessionID, "tool_result", msgID, map[string]any{"ok": resp.Result.OK, "data": resp.Result})
	}

	b, _ := json.Marshal(resp)
	h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "done"; m.Result = string(b); m.Content = resp.Text })
	h.emit(sessionID, "message_done", msgID, map[string]any{"status": "done", "result": resp})
	slog.Info("agent done", "user", u.ID, "type", resp.Type, "action", resp.Action, "ms", time.Since(start).Milliseconds())
}

func writeSSE(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}

// Events — SSE-поток сессии. Подписка регистрируется ДО отправки заголовков,
// а готовые ответы, отправленные до подключения, отдаются повторно (фронт дедуплицирует по message_id).
func (h *ChatHandlers) Events(w http.ResponseWriter, r *http.Request) {
	sessionID, _, ok := h.ownSession(w, r)
	if !ok {
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "flusher not available")
		return
	}
	ch := h.Broker.Subscribe(sessionID)
	defer h.Broker.Unsubscribe(sessionID, ch)
	h.Metrics.Gauge("buddy_sse_streams", "Открытые SSE-потоки", 1)
	defer h.Metrics.Gauge("buddy_sse_streams", "Открытые SSE-потоки", -1)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	for _, m := range h.Msgs.MessagesBySession(sessionID) {
		if m.Role != "assistant" || (m.Status != "done" && m.Status != "failed") {
			continue
		}
		ev := map[string]any{"type": "message_done", "message_id": m.ID, "messageId": m.ID, "status": m.Status}
		if m.Result != "" {
			ev["result"] = json.RawMessage(m.Result)
		}
		b, _ := json.Marshal(ev)
		writeSSE(w, "message_done", string(b))
	}
	fl.Flush()

	tick := time.NewTicker(heartbeatEvery)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, ev.Event, ev.Data)
			fl.Flush()
		}
	}
}
