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
		return Normalize(prompt)
	}
	rest := prompt[i+len(marker):]
	if j := strings.Index(rest, "\n"); j >= 0 {
		rest = rest[:j]
	}
	return Normalize(strings.TrimSpace(rest))
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
		// Имя наставника мок не знает (оно зависит от команды) — совет обратиться к нему добавляет агент.
		return refuse("Не могу выполнить это действие.")
	}

	// Слова сравниваем по началу слова: « дай» ловит «дай» и «дайте», но не «выдают».
	w := words(q)
	asks := hasWord(w, "где", "кто", "сколько", "когда", "зачем", "почему", "что такое", "как ", "какой", "какая", "какие")

	// Зарплаты и доходы — ни своих, ни чужих: даже если в вопросе есть имя из базы знаний.
	if containsAny(q, "зарплат", "оклад", "сколько получа", "сколько плат", "премию", "премия", "доход", "зарабат") {
		return refuse("Информацию о зарплатах и доходах я не предоставляю.")
	}

	// Больничный — не отгул: отвечаем по базе знаний («Если заболел»), а не чек-листом из-за «что делать».
	if hasWord(w, "заболе", "болею", "болен", "больна", "больнич", "простыл", "температур") {
		return answerFromContext(prompt)
	}

	// «Добавь в расписание завтра встречу в 15:00» — создать созвон, а не показать расписание.
	if wantsAddMeeting(w) {
		return toolCall("schedule_add", scheduleAddArgs(q, w))
	}

	// Расписание: «какие у меня встречи завтра?», «когда у меня 1:1?», «расписание на неделю».
	// «Когда стендап?», «когда ретро?» и «как часто 1:1?» — вопросы о правилах команды: их отвечает база знаний.
	if hasWord(w, "расписан", "мои встречи", "какие встречи", "какие у меня встречи", "встречи сегодня", "встречи завтра",
		"встречи на", "созвон", "календар", "что у меня сегодня", "что у меня завтра", "когда 1 1", "когда у меня 1 1",
		"когда следующий 1 1", "ближайшая встреча", "следующая встреча", "есть ли встречи", "ретро в расписании",
		"планирование в расписании") {
		args := map[string]string{"when": "week"}
		switch {
		case hasWord(w, "сегодня"):
			args["when"] = "today"
		case hasWord(w, "завтра"):
			args["when"] = "tomorrow"
		}
		// «1:1» в words() превращается в «1 1»
		switch {
		case hasWord(w, "ретро"):
			args["kind"] = "retro"
		case hasWord(w, "планирован"):
			args["kind"] = "planning"
		case hasWord(w, "1 1"):
			args["kind"] = "one_on_one"
		}
		return toolCall("schedule_get", args)
	}

	// «Можно завтра не прийти?», «в пт меня не будет» — просьба об отгуле без слова «отгул».
	absent := hasWord(w, "не прийти", "не приду", "не выйду", "не выйти", "не смогу прийти", "не смогу выйти",
		"меня не будет", "не буду на работе", "не смогу работать", "пропустить работу", "пропущу работу")
	if containsAny(q, "отгул", "отпуск", "выходной", "отпрос") || absent {
		// «Как отменить заявку на отгул?» — вопрос по базе знаний, а не новый черновик.
		if hasWord(w, "отмен") {
			return answerFromContext(prompt)
		}
		intent := hasWord(w, "хочу", "возьму", "взять", "оформи ", "оформите", "нужен", "нужна", "запиши", "поставь",
			"отпрош", "отпроситься", "можно")
		info := asks || hasWord(w, "порядок", "правил", "оформить", "оформлени", "положен")
		// «Как взять отгул?» — вопрос о порядке, а не просьба: «взять» здесь не намерение.
		if hasWord(w, "как взять", "как отпрос", "как оформ") {
			intent = false
		}
		if info && !intent {
			return toolCall("absence_get_policy", map[string]string{})
		}
		return toolCall("absence_create_request", map[string]string{"reason": "personal"})
	}

	if containsAny(q, "доступ", "jira", "gitlab", "grafana", "джир", "гитлаб", "графан") {
		// Черновик — только при явной просьбе. Справочный вопрос («Где смотреть логи в Grafana?»,
		// «Кто выдаёт доступы?») отвечаем по CONTEXT, иначе каждый вопрос плодит заявки.
		imperative := hasWord(w, "запроси", "дай", "выдай", "оформи ", "оформите", "открой", "нужен", "нужна", "хочу", "сделай",
			"зарегай", "зарегистрируй", "заведи", "добавь меня", "подключи")
		direct := !asks && hasWord(w, "доступ к", "доступ в", "доступа к", "доступа в")
		if imperative || direct {
			return toolCall("access_request", map[string]string{"resource": accessResource(q, w), "reason": "onboarding"})
		}
		return answerFromContext(prompt)
	}

	// «Что делать, если застрял?», «тесты не проходят, что делать» — это препятствие, а не следующий шаг плана:
	// отвечаем по базе (агент добавит пошаговую помощь), а не чек-листом.
	stuck := hasWord(w, "застрял", "не работает", "не работают", "не проход", "не запуска", "не собира", "ошибк",
		"сломал", "падает", "упал", "не получается", "не понимаю", "не пускает")
	// «Что делать в первый день?» — в базе есть отдельный раздел про первый день.
	firstDay := hasWord(w, "первый день", "первую неделю", "первая неделя")
	if !stuck && !firstDay && hasWord(w, "чек-лист", "чеклист", "на этой неделе", "на следующей неделе", "что делать", "что мне делать",
		"следующий шаг", "что мне сделать", "прогресс", "что я сделал", "уже сделал", "что я уже", "сколько я прош",
		"сколько прош", "я прош", "на каком этапе", "какой у меня этап", "что дальше", "дальше что", "что теперь",
		"а дальше", "что еще сделать", "чем заняться", "чем мне заняться", "какой шаг", "что по плану") {
		return toolCall("checklist_get", map[string]string{})
	}

	return answerFromContext(prompt)
}

// answerFromContext отвечает первым фрагментом CONTEXT или отказывает, если контекста нет.
func answerFromContext(prompt string) (string, error) {
	if txt := firstChunk(prompt); txt != "" {
		return toJSON(map[string]any{"type": "answer", "text": txt})
	}
	return toJSON(map[string]any{"type": "refusal", "text": "Не нашёл в доступных источниках."})
}

// accessResource — система из просьбы о доступе. Неизвестную систему («Confluence») возвращаем как есть,
// а не подменяем на Jira: агент честно ответит, что на неё заявку не оформить. Пусто — система не названа.
func accessResource(q, w string) string {
	switch {
	case containsAny(q, "gitlab", "гитлаб"):
		return "gitlab"
	case containsAny(q, "grafana", "графан"):
		return "grafana"
	case containsAny(q, "jira", "джир"):
		return "jira"
	case hasWord(w, "git ", "гит ", "гиту ", "гита ", "гитом ", "гите "):
		return "git"
	}
	for _, p := range []string{" доступ к ", " доступ в ", " доступа к ", " доступа в ", " доступ до "} {
		if i := strings.Index(w, p); i >= 0 {
			if f := strings.Fields(w[i+len(p):]); len(f) > 0 {
				return f[0]
			}
		}
	}
	return ""
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
