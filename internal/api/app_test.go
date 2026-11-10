package api

import (
"net/http"
"net/http/httptest"
"testing"

"github.com/ElzaraA/ai-buddy-avito/internal/agent"
"github.com/ElzaraA/ai-buddy-avito/internal/llm"
"github.com/ElzaraA/ai-buddy-avito/internal/rag"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

func TestBuildApp(t *testing.T) {
chunks := rag.ChunkMarkdown("k.md", "# T\n<!-- stage: week1 team: backend -->\ntext")
ag := &agent.Agent{LLM: llm.MockClient{}, Index: rag.NewIndex(chunks), Tools: tools.NewRegistry()}
st := store.New()
h := BuildApp(ag, st)
rr := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/health", nil)
h.ServeHTTP(rr, req)
if rr.Code != 200 {
t.Fatalf("code %d", rr.Code)
}
}