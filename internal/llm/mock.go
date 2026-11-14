package llm

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"
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

	if containsAny(q, "ignore previous", "игнорируй", "покажи все", "все отгулы", "чужие",
		"забудь все", "забудь инструкц", "системный промпт", "system prompt", "ты теперь", "system:") {
		return refuse("Не могу выполнить это действие. Обратитесь к наставнику Ивану Петрову.")
	}

	// Слова сравниваем по началу слова: « дай» ловит «дай» и «дайте», но не «выдают».
	w := words(q)
	asks := hasWord(w, "где", "кто", "сколько", "когда", "зачем", "почему", "что такое", "как ", "какой", "какая", "какие")

	if containsAny(q, "отгул", "отпуск", "выходной") {
		// «Как отменить заявку на отгул?» — вопрос по базе знаний, а не новый черновик.
		if hasWord(w, "отмен") {
			return answerFromContext(prompt)
		}
		intent := hasWord(w, "хочу", "возьму", "взять", "оформи ", "оформите", "нужен", "нужна", "запиши", "поставь")
		info := asks || hasWord(w, "порядок", "правил", "оформить", "оформлени", "положен")
		if info && !intent {
			return toolCall("absence_get_policy", map[string]string{})
		}
		return toolCall("absence_create_request", map[string]string{"reason": "personal"})
	}

	if containsAny(q, "доступ", "jira", "gitlab", "grafana", "джир", "гитлаб", "графан") {
		// Черновик — только при явной просьбе. Справочный вопрос («Где смотреть логи в Grafana?»,
		// «Кто выдаёт доступы?») отвечаем по CONTEXT, иначе каждый вопрос плодит заявки.
		imperative := hasWord(w, "запроси", "дай", "выдай", "оформи ", "оформите", "открой", "нужен", "нужна", "хочу", "сделай")
		direct := !asks && hasWord(w, "доступ к", "доступ в", "доступа к", "доступа в")
		if imperative || direct {
			res := "jira"
			switch {
			case containsAny(q, "gitlab", "гитлаб"):
				res = "gitlab"
			case containsAny(q, "grafana", "графан"):
				res = "grafana"
			}
			return toolCall("access_request", map[string]string{"resource": res, "reason": "onboarding"})
		}
		return answerFromContext(prompt)
	}

	if hasWord(w, "чек-лист", "чеклист", "на этой неделе", "на следующей неделе", "что делать", "что мне делать",
		"следующий шаг", "что мне сделать", "прогресс") {
		return toolCall("checklist_get", map[string]string{})
	}

	return answerFromContext(prompt)
}

// answerFromContext отвечает первым фрагментом CONTEXT или отказывает, если контекста нет.
func answerFromContext(prompt string) (string, error) {
	if txt := firstChunk(prompt); txt != "" {
		return toJSON(map[string]any{"type": "answer", "text": txt})
	}
	return toJSON(map[string]any{"type": "refusal", "text": "Не нашёл в доступных источниках. Обратитесь к наставнику Ивану Петрову."})
}

// words нормализует вопрос в « слово слово … » (знаки препинания → пробелы, дефис сохраняется).
func words(q string) string {
	f := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
	return " " + strings.Join(f, " ") + " "
}

// hasWord — встречается ли фраза с начала слова («нужн» найдёт «нужно»; «как » — только «как»).
func hasWord(w string, phrases ...string) bool {
	for _, p := range phrases {
		if strings.Contains(w, " "+p) {
			return true
		}
	}
	return false
}
