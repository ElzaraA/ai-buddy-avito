package llm

import (
	"context"
	"encoding/json"
	"strings"
)

// MockClient — детерминированная «LLM» для демо и тестов: работает без ключа и интернета.
// Смотрит только на строку QUESTION (не на CONTEXT), а ответ по базе берёт из первого фрагмента CONTEXT.
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

// firstChunk возвращает текст первого фрагмента из блока CONTEXT без префикса [id].
func firstChunk(prompt string) string {
	const marker = "CONTEXT:\n"
	i := strings.Index(prompt, marker)
	if i < 0 {
		return ""
	}
	rest := prompt[i+len(marker):]
	if j := strings.Index(rest, "\n["); j >= 0 {
		rest = rest[:j]
	}
	if strings.HasPrefix(rest, "[") {
		if k := strings.Index(rest, "] "); k >= 0 {
			rest = rest[k+2:]
		}
	}
	return strings.TrimSpace(rest)
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func (MockClient) Complete(_ context.Context, msgs []Message) (string, error) {
	prompt := msgs[len(msgs)-1].Content
	q := extractQuestion(prompt)

	refuse := func(text string) (string, error) {
		return toJSON(map[string]any{"type": "refusal", "text": text})
	}
	toolCall := func(tool string, args map[string]string) (string, error) {
		return toJSON(map[string]any{"type": "tool_call", "tool": tool, "args": args})
	}

	if containsAny(q, "ignore previous", "игнорируй", "покажи все", "все отгулы", "чужие") {
		return refuse("Не могу выполнить это действие. Обратитесь к наставнику Ивану Петрову.")
	}

	if containsAny(q, "отгул", "отпуск", "выходной") {
		if containsAny(q, "как ", "как?", "порядок", "оформ", "правил") {
			return toolCall("absence_get_policy", map[string]string{})
		}
		return toolCall("absence_create_request", map[string]string{"reason": "personal"})
	}

	if containsAny(q, "доступ", "jira", "gitlab", "grafana", "джир", "гитлаб", "графан") {
		res := "jira"
		switch {
		case containsAny(q, "gitlab", "гитлаб"):
			res = "gitlab"
		case containsAny(q, "grafana", "графан"):
			res = "grafana"
		}
		return toolCall("access_request", map[string]string{"resource": res, "reason": "onboarding"})
	}

	if containsAny(q, "чек-лист", "чеклист", "недел", "следующий шаг", "что мне сделать", "прогресс") {
		return toolCall("checklist_get", map[string]string{})
	}

	if txt := firstChunk(prompt); txt != "" {
		return toJSON(map[string]any{"type": "answer", "text": txt})
	}
	return refuse("Не нашёл в доступных источниках. Обратитесь к наставнику Ивану Петрову.")
}
