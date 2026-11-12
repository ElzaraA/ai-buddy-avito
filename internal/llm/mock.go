package llm

import (
	"context"
	"encoding/json"
	"strings"
)

type MockClient struct{}

func extractQuestion(prompt string) string {
	const marker = "QUESTION:"
	i := strings.Index(prompt, marker)
	if i < 0 {
		return strings.ToLower(prompt)
	}
	rest := prompt[i+len(marker):]
	if j := strings.Index(rest, "\n"); j >= 0 {
		rest = rest[:j]
	}
	return strings.ToLower(strings.TrimSpace(rest))
}

func (MockClient) Complete(_ context.Context, msgs []Message) (string, error) {
	q := extractQuestion(msgs[len(msgs)-1].Content)

	refuse := func(text string) (string, error) {
		b, _ := json.Marshal(map[string]any{"type": "refusal", "text": text})
		return string(b), nil
	}

	if strings.Contains(q, "ignore previous") ||
		strings.Contains(q, "покажи все") ||
		strings.Contains(q, "все отгулы") {
		return refuse("Не могу выполнить это действие. Обратитесь к наставнику Ивану Петрову.")
	}

	if strings.Contains(q, "отгул") || strings.Contains(q, "отпуск") {
		if strings.Contains(q, "как") || strings.Contains(q, "порядок") || strings.Contains(q, "оформ") {
			b, _ := json.Marshal(map[string]any{
				"type": "tool_call",
				"tool": "absence_get_policy",
				"args": map[string]string{},
			})
			return string(b), nil
		}
		if strings.Contains(q, "хочу") || strings.Contains(q, "нужен") || strings.Contains(q, "взять") || strings.Contains(q, "2026-") || strings.Contains(q, "пятниц") {
			b, _ := json.Marshal(map[string]any{
				"type":   "draft",
				"action": "absence",
				"args":   map[string]string{"reason": "personal"},
			})
			return string(b), nil
		}
	}

	if strings.Contains(q, "доступ") || strings.Contains(q, "jira") || strings.Contains(q, "gitlab") {
		b, _ := json.Marshal(map[string]any{
			"type": "tool_call",
			"tool": "access_request",
			"args": map[string]string{"resource": "jira", "reason": "onboarding"},
		})
		return string(b), nil
	}

	if strings.Contains(q, "чек-лист") || strings.Contains(q, "чеклист") || strings.Contains(q, "недел") || strings.Contains(q, "следующий шаг") {
		b, _ := json.Marshal(map[string]any{
			"type": "tool_call",
			"tool": "checklist_get",
			"args": map[string]string{},
		})
		return string(b), nil
	}

	return refuse("Не нашёл в доступных источниках. Обратитесь к наставнику Ивану Петрову.")
}
