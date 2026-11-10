package agent

import (
"context"
"encoding/json"
"strings"
"time"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/llm"
"github.com/ElzaraA/ai-buddy-avito/internal/rag"
"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

type Agent struct {
LLM   llm.Client
Index *rag.Index
Tools *tools.Registry
Now   func() time.Time
}

type Response struct {
Type    string            `json:"type"`
Text    string            `json:"text,omitempty"`
Action  string            `json:"action,omitempty"`
Args    map[string]string `json:"args,omitempty"`
Result  *tools.Result     `json:"result,omitempty"`
Sources []Source          `json:"sources,omitempty"`
}

func (a *Agent) Ask(ctx context.Context, user auth.User, question string) (Response, error) {
ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
defer cancel()

hits := a.Index.Search(question, user.TeamID, 4)
srcs := make([]Source, 0, len(hits))
for _, h := range hits {
srcs = append(srcs, Source{ID: h.Chunk.ID, Source: h.Chunk.Source, Stage: h.Chunk.Stage})
}
if len(srcs) == 0 {
return Response{Type: "refusal", Text: "Не нашёл в доступных источниках. Уточните у наставника."}, nil
}

msgs := []llm.Message{
{Role: "system", Content: SystemPrompt},
{Role: "user", Content: UserPrompt(question, hits, string(user.Role))},
}
raw, err := a.LLM.Complete(ctx, msgs)
if err != nil {
return Response{}, err
}

raw = stripCodeFence(raw)
var out AgentOutput
if err := json.Unmarshal([]byte(raw), &out); err != nil {
return Response{Type: "answer", Text: raw, Sources: srcs}, nil
}

switch out.Type {
case "tool_call":
t, ok := a.Tools.Get(out.Tool)
if !ok {
return Response{Type: "refusal", Text: "Инструмент недоступен"}, nil
}
res, err := tools.Run(ctx, t, user, out.Args)
if err != nil {
return Response{Type: "refusal", Text: "Действие недоступно: " + err.Error()}, nil
}
return Response{Type: "action", Result: &res, Sources: srcs}, nil

case "draft":
return a.buildDraft(question, out, srcs), nil

case "refusal":
return Response{Type: "refusal", Text: out.Text}, nil
}
return Response{Type: "answer", Text: out.Text, Sources: srcs}, nil
}

func (a *Agent) buildDraft(question string, out AgentOutput, srcs []Source) Response {
if out.Action != "absence" {
return Response{Type: "refusal", Text: "Неизвестное действие"}
}
from, to := ParseDates(question)
if from == "" {
now := time.Now()
if a.Now != nil {
now = a.Now()
}
if d, ok := ParseWeekday(question, now); ok {
from, to = d, d
}
}
if from == "" {
return Response{
Type:   "draft",
Action: "absence",
Args:   map[string]string{"reason": "personal"},
Text:   "На какую дату оформить отгул? Укажите в формате 2026-11-20.",
Sources: srcs,
}
}
return Response{
Type:    "draft",
Action:  "absence",
Args:    map[string]string{"date_from": from, "date_to": to, "reason": "personal"},
Text:    "Черновик заявки на отгул. Подтвердите через POST /api/v1/actions/{id}/confirm.",
Sources: srcs,
}
}

func stripCodeFence(s string) string {
s = strings.TrimSpace(s)
if strings.HasPrefix(s, "```") {
if i := strings.Index(s, "\n"); i >= 0 {
s = s[i+1:]
}
s = strings.TrimSuffix(s, "```")
s = strings.TrimSpace(s)
}
return s
}