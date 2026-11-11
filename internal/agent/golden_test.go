package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

type goldenCase struct {
	ID         string `json:"id"`
	Question   string `json:"question"`
	Role       string `json:"role"`
	Team       string `json:"team"`
	ExpectType string `json:"expect_type"`
	ExpectTool string `json:"expect_tool"`
}

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
	idx := rag.NewIndex(chunks)
	st := store.New()
	st.SeedChecklist("intern-1", []store.ChecklistItem{{ID: "c1-intern-1", Title: "a"}})
	reg := tools.NewRegistry()
	reg.Register(&tools.ChecklistGet{S: st})
	reg.Register(&tools.AbsenceCreateRequest{S: st})
	reg.Register(&tools.AbsenceGetPolicy{})
	reg.Register(&tools.AccessRequest{})
	reg.Register(&tools.DirectoryFindOwner{})
	a := &Agent{LLM: llm.MockClient{}, Index: idx, Tools: reg, Now: func() time.Time {
		return time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	}}

	pass, fail := 0, 0
	for _, c := range doc.Cases {
		u := auth.User{ID: "intern-1", Role: auth.Role(c.Role), TeamID: c.Team}
		resp, err := a.Ask(context.Background(), u, c.Question)
		if err != nil {
			t.Logf("FAIL %s: err %v", c.ID, err)
			fail++
			continue
		}
		ok := resp.Type == c.ExpectType
		if c.ExpectTool != "" && resp.Result != nil {
			ok = ok && resp.Result.OK
		}
		if ok {
			pass++
		} else {
			t.Logf("FAIL %s: want %s, got %s (text=%q)", c.ID, c.ExpectType, resp.Type, resp.Text)
			fail++
		}
	}
	t.Logf("golden: pass=%d fail=%d", pass, fail)
	if fail > 0 {
		t.Fatalf("golden failed: %d", fail)
	}
}
