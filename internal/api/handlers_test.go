package api

import (
"bytes"
"net/http"
"net/http/httptest"
"testing"

"github.com/ElzaraA/ai-buddy-avito/internal/agent"
"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/llm"
"github.com/ElzaraA/ai-buddy-avito/internal/rag"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

func newHandlers() *Handlers {
chunks := rag.ChunkMarkdown("k.md", "# Absence\n<!-- stage: week1 team: backend -->\nAbsence rules")
st := store.New()
reg := tools.NewRegistry()
reg.Register(&tools.AbsenceCreateRequest{S: st})
ag := &agent.Agent{LLM: llm.MockClient{}, Index: rag.NewIndex(chunks), Tools: reg}
return &Handlers{Agent: ag, Store: st}
}

func TestHealth(t *testing.T) {
h := newHandlers()
rr := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/health", nil)
h.Health(rr, req)
if rr.Code != 200 {
t.Fatalf("code %d", rr.Code)
}
}

func TestAskBadRequest(t *testing.T) {
h := newHandlers()
rr := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewBufferString("bad"))
req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "u1", Role: auth.RoleIntern, TeamID: "backend"}))
h.Ask(rr, req)
if rr.Code != 400 {
t.Fatalf("code %d", rr.Code)
}
}

func TestAskUnauthorized(t *testing.T) {
h := newHandlers()
rr := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewBufferString(`{"question":"x"}`))
h.Ask(rr, req)
if rr.Code != 401 {
t.Fatalf("code %d", rr.Code)
}
}

func TestAskOK(t *testing.T) {
h := newHandlers()
rr := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewBufferString(`{"question":"absence"}`))
req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "u1", Role: auth.RoleIntern, TeamID: "backend"}))
h.Ask(rr, req)
if rr.Code != 200 {
t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
}
}

func TestRouter(t *testing.T) {
h := newHandlers()
r := NewRouter(h)
if r == nil {
t.Fatal("router nil")
}
rr := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/health", nil)
r.ServeHTTP(rr, req)
if rr.Code != 200 {
t.Fatalf("router code %d", rr.Code)
}
}