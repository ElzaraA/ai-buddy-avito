package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
)

func TestOneOnOneQuestionFindsKB(t *testing.T) {
	chunks, err := rag.LoadDir("../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := agentWith("", nil)
	a.LLM, a.Index = llm.MockClient{}, rag.NewIndex(chunks)
	for _, q := range []string{"как проходит 1:1", "когда встреча 1 на 1 с наставником?"} {
		resp, _ := a.Ask(context.Background(), intern, q)
		if resp.Type != "answer" || !strings.Contains(resp.Text, "1:1") {
			t.Fatalf("%q: %+v", q, resp)
		}
	}
}
