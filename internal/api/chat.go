package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ChatHandlers struct {
	Agent  *agent.Agent
	Store  *store.Store
	Msgs   *store.MessageStore
	Broker *Broker
}

func (h *ChatHandlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	sess := h.Msgs.CreateSession(u.ID, uuid.NewString())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sess)
}

func (h *ChatHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	sess, ok := h.Msgs.GetSession(id)
	if !ok || sess.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your session")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(h.Msgs.MessagesBySession(id))
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

func (h *ChatHandlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	sessionID := chi.URLParam(r, "id")
	sess, ok := h.Msgs.GetSession(sessionID)
	if !ok || sess.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your session")
		return
	}
	var body sendMessageReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Q() == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "text required")
		return
	}

	h.Msgs.CreateMessage(store.Message{
		ID: uuid.NewString(), SessionID: sessionID, Role: "user", Content: body.Q(),
		Status: "done", CreatedAt: time.Now(),
	})
	asstMsg := h.Msgs.CreateMessage(store.Message{
		ID: uuid.NewString(), SessionID: sessionID, Role: "assistant",
		Status: "pending", CreatedAt: time.Now(),
	})

	go h.runAgent(sessionID, asstMsg.ID, body.Q(), u)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"messageId": asstMsg.ID,
		"status":    "pending",
	})
}

func (h *ChatHandlers) runAgent(sessionID, msgID, question string, u auth.User) {
	defer func() {
		if rec := recover(); rec != nil {
			h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "failed"; m.Content = "internal error" })
			h.Broker.Publish(sessionID, SSEEvent{Event: "message_done", Data: fmt.Sprintf(`{"messageId":%q,"status":"failed"}`, msgID)})
		}
	}()

	h.Broker.Publish(sessionID, SSEEvent{Event: "status", Data: fmt.Sprintf(`{"messageId":%q,"status":"in_progress"}`, msgID)})
	h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "in_progress" })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := h.Agent.Ask(ctx, u, question)
	if err != nil {
		h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "failed"; m.Content = "agent failed" })
		h.Broker.Publish(sessionID, SSEEvent{Event: "error", Data: fmt.Sprintf(`{"messageId":%q,"error":"agent failed"}`, msgID)})
		h.Broker.Publish(sessionID, SSEEvent{Event: "message_done", Data: fmt.Sprintf(`{"messageId":%q,"status":"failed"}`, msgID)})
		return
	}

	if resp.Type == "draft" && resp.Action == "absence" {
		if resp.Args == nil {
			resp.Args = map[string]string{}
		}
		from := resp.Args["date_from"]
		to := resp.Args["date_to"]
		if from == "" {
			from, to = agent.ParseDates(question)
		}
		if from != "" {
			a := h.Store.CreateAbsence(store.Absence{
				ID: h.Store.NextAbsenceID(), UserID: u.ID,
				DateFrom: from, DateTo: to, Reason: resp.Args["reason"],
				Status: "draft", CreatedAt: time.Now(),
			})
			resp.Args["action_id"] = a.ID
			resp.Args["date_from"] = from
			resp.Args["date_to"] = to
		}
	}

	if resp.Type == "action" && resp.Result != nil {
		b, _ := json.Marshal(resp.Result)
		h.Broker.Publish(sessionID, SSEEvent{Event: "tool_result", Data: fmt.Sprintf(`{"messageId":%q,"ok":%t,"data":%s}`, msgID, resp.Result.OK, string(b))})
	}

	b, _ := json.Marshal(resp)
	h.Msgs.UpdateMessage(msgID, func(m *store.Message) { m.Status = "done"; m.Result = string(b); m.Content = resp.Text })
	h.Broker.Publish(sessionID, SSEEvent{Event: "message_done", Data: fmt.Sprintf(`{"messageId":%q,"status":"done","result":%s}`, msgID, string(b))})
}

func (h *ChatHandlers) Events(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	sessionID := chi.URLParam(r, "id")
	sess, ok := h.Msgs.GetSession(sessionID)
	if !ok || sess.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your session")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "flusher not available")
		return
	}
	w.WriteHeader(http.StatusOK)
	fl.Flush()

	ch := h.Broker.Subscribe(sessionID)
	defer h.Broker.Unsubscribe(sessionID, ch)
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
			fl.Flush()
		}
	}
}
