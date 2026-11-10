package llm

import (
"context"
"encoding/json"
"strings"
)

type MockClient struct{}

func (MockClient) Complete(_ context.Context, msgs []Message) (string, error) {
last := strings.ToLower(msgs[len(msgs)-1].Content)

// Защита: явные попытки инъекции или «покажи все отгулы» — не создаём.
if strings.Contains(last, "ignore previous") ||
strings.Contains(last, "покажи все") ||
strings.Contains(last, "все отгулы") {
b, _ := json.Marshal(map[string]any{
"type": "refusal",
"text": "Не могу выполнить это действие. Уточните запрос.",
})
return string(b), nil
}

// Отгул — только если есть намерение создать (хочу/оформить/взять).
if strings.Contains(last, "отгул") &&
(strings.Contains(last, "хочу") || strings.Contains(last, "оформ") || strings.Contains(last, "взять") || strings.Contains(last, "нужен")) {
b, _ := json.Marshal(map[string]any{
"type":   "draft",
"action": "absence",
"args":   map[string]string{"reason": "personal"},
})
return string(b), nil
}

if strings.Contains(last, "чек-лист") || strings.Contains(last, "следующий шаг") {
b, _ := json.Marshal(map[string]any{
"type": "tool_call",
"tool": "checklist.status",
"args": map[string]string{},
})
return string(b), nil
}

b, _ := json.Marshal(map[string]any{
"type": "answer",
"text": "Отвечаю по найденным источникам.",
})
return string(b), nil
}