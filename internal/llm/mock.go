package llm

import (
"context"
"encoding/json"
"strings"
)

type MockClient struct{}

func (MockClient) Complete(_ context.Context, msgs []Message) (string, error) {
last := strings.ToLower(msgs[len(msgs)-1].Content)
switch {
case strings.Contains(last, "отгул") || strings.Contains(last, "отпуск"):
b, _ := json.Marshal(map[string]any{
"type": "tool_call",
"tool": "absence.create",
"args": map[string]string{"date_from": "2026-02-10", "date_to": "2026-02-10", "reason": "personal"},
})
return string(b), nil
case strings.Contains(last, "чек-лист") || strings.Contains(last, "следующий шаг"):
b, _ := json.Marshal(map[string]any{
"type": "tool_call",
"tool": "checklist.status",
"args": map[string]string{},
})
return string(b), nil
default:
b, _ := json.Marshal(map[string]any{
"type": "answer",
"text": "Отвечаю по найденным источникам.",
})
return string(b), nil
}
}
