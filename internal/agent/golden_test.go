package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

type goldenCase struct {
	ID           string `json:"id"`
	Question     string `json:"question"`
	Role         string `json:"role"`
	Team         string `json:"team"`
	ExpectType   string `json:"expect_type"`
	ExpectTool   string `json:"expect_tool"`
	ExpectAction string `json:"expect_action"`
	ExpectArg    string `json:"expect_arg"`  // key=value
	ExpectText   string `json:"expect_text"` // подстрока ответа
	ForbidText   string `json:"forbid_text"` // утечка: этой подстроки в ответе быть не должно
}

// TestGolden прогоняет реальную базу знаний data/knowledge и mock-LLM на фразах из сценариев README.
// Сегодня в тесте — среда 2026-11-11, поэтому «в пятницу» = 2026-11-13.
func TestGolden(t *testing.T) {
	raw, err := os.ReadFile("../../data/golden/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Cases []goldenCase }
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	chunks, err := rag.LoadDir("../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New()
	st.SeedChecklist("intern-1", []store.ChecklistItem{{ID: "c1-intern-1", Title: "a"}})
	reg := tools.NewRegistry()
	reg.Register(&tools.ChecklistGet{S: st})
	reg.Register(&tools.AbsenceCreateRequest{S: st})
	reg.Register(&tools.AbsenceGetPolicy{})
	reg.Register(&tools.AccessRequest{})
	reg.Register(&tools.DirectoryFindOwner{})
	a := &Agent{LLM: llm.MockClient{}, Index: rag.NewIndex(chunks), Tools: reg, Now: func() time.Time {
		return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	}}

	for _, c := range doc.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			u := auth.User{ID: "intern-1", Role: auth.Role(c.Role), TeamID: c.Team}
			resp, err := a.Ask(context.Background(), u, c.Question)
			if err != nil {
				t.Fatalf("err %v", err)
			}
			if c.ForbidText != "" && strings.Contains(resp.Text, c.ForbidText) {
				t.Fatalf("leaked %q: %q", c.ForbidText, resp.Text)
			}
			if c.ExpectType != "" && resp.Type != c.ExpectType {
				t.Fatalf("want type %s, got %s (text=%q)", c.ExpectType, resp.Type, resp.Text)
			}
			if c.ExpectTool != "" && (resp.Tool != c.ExpectTool || resp.Result == nil || !resp.Result.OK) {
				t.Fatalf("want tool %s, got %q with result %+v", c.ExpectTool, resp.Tool, resp.Result)
			}
			if c.ExpectAction != "" && resp.Action != c.ExpectAction {
				t.Fatalf("want action %s, got %s", c.ExpectAction, resp.Action)
			}
			if c.ExpectArg != "" {
				kv := strings.SplitN(c.ExpectArg, "=", 2)
				if resp.Args[kv[0]] != kv[1] {
					t.Fatalf("want %s, got args %v", c.ExpectArg, resp.Args)
				}
			}
			if c.ExpectText != "" && !strings.Contains(resp.Text, c.ExpectText) {
				t.Fatalf("want text containing %q, got %q", c.ExpectText, resp.Text)
			}
			if resp.Type == "answer" && len(resp.Sources) == 0 {
				t.Fatal("answer without sources")
			}
			if resp.Type == "refusal" && resp.Buddy == "" {
				t.Fatal("refusal without buddy name")
			}
		})
	}
}
