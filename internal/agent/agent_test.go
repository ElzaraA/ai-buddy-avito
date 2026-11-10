package agent

import (
"context"
"strings"
"testing"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/llm"
"github.com/ElzaraA/ai-buddy-avito/internal/rag"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

func newAgent(t *testing.T) *Agent {
md := "# Отгул\n<!-- stage: week1 team: backend -->\nПорядок оформления отгула\n\n# Чек-лист\n<!-- stage: week1 team: backend -->\nСледующий шаг по чек-листу"
chunks := rag.ChunkMarkdown("k.md", md)
st := store.New()
st.SeedChecklist("u1", []store.ChecklistItem{{ID: "1", Title: "a"}})
reg := tools.NewRegistry()
reg.Register(&tools.ChecklistStatus{S: st})
reg.Register(&tools.AbsenceCreate{S: st})
reg.Register(&tools.DirectoryFind{})
return &Agent{LLM: llm.MockClient{}, Index: rag.NewIndex(chunks), Tools: reg}
}

func TestAskRefusalNoSources(t *testing.T) {
a := newAgent(t)
resp, err := a.Ask(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern, TeamID: "backend"}, "неведомый вопрос")
if err != nil || resp.Type != "refusal" {
t.Fatalf("must refuse: %v %v", resp, err)
}
}

func TestAskAbsenceAction(t *testing.T) {
a := newAgent(t)
resp, err := a.Ask(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern, TeamID: "backend"}, "как оформить отгул")
if err != nil {
t.Fatal(err)
}
if resp.Type != "action" || resp.Action == nil || !resp.Action.OK {
t.Fatalf("must create absence: %+v", resp)
}
if !strings.Contains(resp.Action.Message, "Заявка") {
t.Fatalf("wrong message: %s", resp.Action.Message)
}
}

func TestAskChecklistAction(t *testing.T) {
a := newAgent(t)
resp, err := a.Ask(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern, TeamID: "backend"}, "чек-лист следующий шаг")
if err != nil {
t.Fatal(err)
}
if resp.Type != "action" || resp.Action == nil {
t.Fatalf("must action: %+v", resp)
}
}

func TestAskToolMissing(t *testing.T) {
a := newAgent(t)
a.Tools = tools.NewRegistry()
resp, err := a.Ask(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern, TeamID: "backend"}, "как оформить отгул")
if err != nil || resp.Type != "refusal" {
t.Fatalf("must refuse when tool missing: %+v", resp)
}
}