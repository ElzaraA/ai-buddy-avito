package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

func newTestRouter() http.Handler {
	chunks := rag.ChunkMarkdown("k.md", "# X\n<!-- stage: week1 team: backend audience: intern -->\ntext")
	st := store.New()
	st.SeedChecklist("intern-1", []store.ChecklistItem{{ID: "c1-intern-1", Title: "a"}})
	st.SeedChecklist("intern-2", []store.ChecklistItem{{ID: "c1-intern-2", Title: "b"}})
	reg := tools.NewRegistry()
	reg.Register(&tools.ChecklistGet{S: st})
	reg.Register(&tools.AbsenceCreateRequest{S: st})
	ag := &agent.Agent{LLM: llm.MockClient{}, Index: rag.NewIndex(chunks), Tools: reg}
	h := &Handlers{Agent: ag, Store: st}
	return NewRouter(h)
}

func doReq(t *testing.T, r http.Handler, method, path, user string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if user != "" {
		req.Header.Set("X-Demo-User", user)
	}
	r.ServeHTTP(rr, req)
	return rr
}

func TestConfirmNotFound(t *testing.T) {
	r := newTestRouter()
	rr := doReq(t, r, http.MethodPost, "/api/v1/actions/nope/confirm", "intern.backend", nil)
	if rr.Code != 404 {
		t.Fatalf("want 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestStepUpdate(t *testing.T) {
	r := newTestRouter()
	rr := doReq(t, r, http.MethodPatch, "/api/v1/onboarding/steps/c1-intern-1", "intern.backend", nil)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, r, http.MethodPatch, "/api/v1/onboarding/steps/c1-intern-2", "intern.backend", nil)
	if rr.Code != 403 {
		t.Fatalf("want 403, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, r, http.MethodPatch, "/api/v1/onboarding/steps/xxx", "intern.backend", nil)
	if rr.Code != 404 {
		t.Fatalf("want 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUnauthorized(t *testing.T) {
	r := newTestRouter()
	rr := doReq(t, r, http.MethodGet, "/api/v1/me", "unknown", nil)
	if rr.Code != 401 {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}
