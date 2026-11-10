package agent

import (
"context"
"encoding/json"
"errors"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
"github.com/ElzaraA/ai-buddy-avito/internal/llm"
"github.com/ElzaraA/ai-buddy-avito/internal/rag"
"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

type Agent struct {
LLM   llm.Client
Index *rag.Index
Tools *tools.Registry
}

type Response struct {
Type    string        `json:"type"`
Text    string        `json:"text,omitempty"`
Action  *tools.Result `json:"action,omitempty"`
Sources []Source      `json:"sources,omitempty"`
}

func (a *Agent) Ask(ctx context.Context, user auth.User, question string) (Response, error) {
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
{Role: "user", Content: UserPrompt(question, srcs, string(user.Role))},
}
raw, err := a.LLM.Complete(ctx, msgs)
if err != nil {
return Response{}, err
}

var out AgentOutput
if err := json.Unmarshal([]byte(raw), &out); err != nil {
return Response{}, errors.New("invalid llm json")
}

if out.Type == "tool_call" {
t, ok := a.Tools.Get(out.Tool)
if !ok {
return Response{Type: "refusal", Text: "Инструмент недоступен"}, nil
}
res, err := t.Run(ctx, user, out.Args)
if err != nil {
return Response{Type: "refusal", Text: "Действие недоступно: " + err.Error()}, nil
}
return Response{Type: "action", Action: &res, Sources: srcs}, nil
}
if out.Type == "refusal" {
return Response{Type: "refusal", Text: out.Text}, nil
}
return Response{Type: "answer", Text: out.Text, Sources: srcs}, nil
}
