package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	Agent *agent.Agent
	Store *store.Store
	Chat  *ChatHandlers
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

func (h *Handlers) Ask(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "no user")
		return
	}
	var body askReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Q() == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "question required")
		return
	}
	resp, err := h.Agent.Ask(r.Context(), u, body.Q())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent_error", "agent failed")
		return
	}
	h.maybeCreateDraft(u, body.Q(), &resp)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) maybeCreateDraft(u auth.User, question string, resp *agent.Response) {
	if resp.Type != "draft" || resp.Action != "absence" {
		return
	}
	if resp.Args == nil {
		resp.Args = map[string]string{}
	}
	from := resp.Args["date_from"]
	to := resp.Args["date_to"]
	if from == "" {
		from, to = agent.ParseDates(question)
	}
	if from == "" {
		return
	}
	a := h.Store.CreateAbsence(store.Absence{
		ID: h.Store.NextAbsenceID(), UserID: u.ID,
		DateFrom: from, DateTo: to, Reason: resp.Args["reason"],
		Status: "draft", CreatedAt: time.Now(),
	})
	resp.Args["action_id"] = a.ID
	resp.Args["date_from"] = from
	resp.Args["date_to"] = to
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(u)
}

func (h *Handlers) OnboardingProgress(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	items := h.Store.Checklist(u.ID)
	next := ""
	for _, it := range items {
		if !it.Done {
			next = it.Title
			break
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "next_step": next})
}

func (h *Handlers) OnboardingStepUpdate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	found, owned := h.Store.MarkDoneOwned(u.ID, id)
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "step not found")
		return
	}
	if !owned {
		writeError(w, http.StatusForbidden, "forbidden", "step belongs to another user")
		return
	}
	h.OnboardingProgress(w, r)
}

type actionReq struct {
	Type     string `json:"type"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Reason   string `json:"reason"`
}

func (h *Handlers) CreateAction(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	var body actionReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if body.Type != "absence" || body.DateFrom == "" || body.DateTo == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "type=absence and dates required")
		return
	}
	a := h.Store.CreateAbsence(store.Absence{
		ID: h.Store.NextAbsenceID(), UserID: u.ID,
		DateFrom: body.DateFrom, DateTo: body.DateTo, Reason: body.Reason,
		Status: "draft", CreatedAt: time.Now(),
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(a)
}

func (h *Handlers) ConfirmAction(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	a, ok := h.Store.GetAbsence(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "action not found")
		return
	}
	if a.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your action")
		return
	}
	if !auth.Can(u, "absence.create") {
		writeError(w, http.StatusForbidden, "forbidden", "role cannot create absence")
		return
	}
	upd, ok := h.Store.Transition(id, "draft", "pending")
	if !ok {
		writeError(w, http.StatusConflict, "conflict", "already confirmed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(upd)
}

func (h *Handlers) GetAction(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	a, ok := h.Store.GetAbsence(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "action not found")
		return
	}
	if a.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your action")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(a)
}

func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "flusher not available")
		return
	}
	for i := 0; i < 5; i++ {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(time.Second):
			fmt.Fprintf(w, "event: ping\ndata: {\"tick\":%d}\n\n", i)
			fl.Flush()
		}
	}
}
