package api

import (
"encoding/json"
"net/http"

"github.com/ElzaraA/ai-buddy-avito/internal/agent"
"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

type Handlers struct {
Agent *agent.Agent
}

type askReq struct {
Question string `json:"question"`
}

func (h *Handlers) Ask(w http.ResponseWriter, r *http.Request) {
u, ok := auth.FromCtx(r.Context())
if !ok {
http.Error(w, "unauthorized", http.StatusUnauthorized)
return
}
var body askReq
if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Question == "" {
http.Error(w, "bad request", http.StatusBadRequest)
return
}
resp, err := h.Agent.Ask(r.Context(), u, body.Question)
if err != nil {
http.Error(w, "agent error: "+err.Error(), http.StatusInternalServerError)
return
}
w.Header().Set("Content-Type", "application/json")
_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
w.WriteHeader(http.StatusOK)
_, _ = w.Write([]byte(`{"status":"ok"}`))
}
