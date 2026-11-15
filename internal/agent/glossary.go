package agent

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Словарь Бадди: внутренний жаргон, на котором спотыкаются стажёры в первую неделю.
// Объяснения — общие определения слов, а не правила команды: правила берутся только из базы знаний.

type glossaryTerm struct {
	Name string
	// Aliases — формы в нижнем регистре; «*» в конце — совпадение по началу слова («апрув*» → «апрува»).
	Aliases []string
	Plain   string
}

const (
	glossaryHeader = "Термины простыми словами:"
	glossaryMark   = "Из словаря Бадди: "
	// maxGlossaryTerms — сколько терминов пояснять под ответом, чтобы не утопить сам ответ.
	maxGlossaryTerms = 3
)

var glossary = []glossaryTerm{
	{"MR (merge request)", []string{"mr", "merge request", "мр", "мерж-реквест*", "мердж-реквест*"},
		"запрос на то, чтобы ваши изменения в коде влили в общий код; его смотрят коллеги на ревью."},
	{"PR (pull request)", []string{"pr", "pull request", "пулл-реквест*", "пул-реквест*"},
		"то же, что MR, только так называют в GitHub."},
	{"CI", []string{"ci"},
		"автоматические проверки (сборка и тесты), которые запускаются на каждый MR; «зелёный CI» — всё прошло."},
	{"CD", []string{"cd"},
		"автоматическая доставка кода на серверы после проверок — продолжение CI."},
	{"Пайплайн", []string{"пайплайн*", "pipeline*"},
		"цепочка автоматических шагов: собрать код, прогнать тесты, выкатить."},
	{"Ревью (код-ревью)", []string{"ревью*", "review*"},
		"проверка вашего кода коллегами до того, как он попадёт в общий код."},
	{"Апрув", []string{"апрув*", "approve*"},
		"одобрение MR ревьюером: «можно вливать»."},
	{"Мерж", []string{"мерж*", "мердж*", "смерж*", "смердж*"},
		"влить изменения из вашей ветки в основную ветку кода."},
	{"Стендап (дейли)", []string{"стендап*", "стэндап*", "дейли", "daily"},
		"короткая ежедневная встреча: что сделал, что буду делать, что мешает."},
	{"Ретро", []string{"ретро", "ретроспектив*"},
		"встреча в конце спринта: что было хорошо, что плохо и что поменять."},
	{"1:1", []string{"1:1", "one-on-one", "ван-ту-ван"},
		"личная встреча с наставником или руководителем один на один."},
	{"DRI", []string{"dri"},
		"человек, который отвечает за задачу или тему и принимает по ней решения."},
	{"OKR", []string{"okr", "окр"},
		"цели команды на квартал и измеримые результаты, по которым видно, достигнуты ли они."},
	{"Онбординг", []string{"онбординг*", "onboarding"},
		"период адаптации новичка: знакомство с командой, доступы, первые задачи."},
	{"Jira", []string{"jira", "джира", "джиры", "джиру", "джире"},
		"система, где ведут задачи команды."},
	{"Тикет", []string{"тикет*", "jira-тикет*"},
		"карточка задачи в Jira: что сделать, кто делает, статус."},
	{"Спринт", []string{"спринт*", "sprint*"},
		"отрезок работы фиксированной длины, на который команда планирует задачи."},
	{"Бэклог", []string{"бэклог*", "беклог*", "backlog"},
		"список задач, которые команда сделает позже."},
	{"Прод", []string{"прод", "прода", "проде", "проду", "продом", "продакшн*", "production"},
		"боевая среда: сервис, которым пользуются настоящие пользователи."},
	{"Стейдж", []string{"стейдж*", "стейджинг*", "staging"},
		"тестовая копия прода, где проверяют изменения перед выкаткой."},
	{"Деплой", []string{"деплой*", "задеплои*", "деплои*", "выкат*", "выклад*"},
		"выкладка новой версии кода на серверы."},
	{"Фича-флаг", []string{"фича-флаг*", "фичафлаг*", "feature flag*", "feature-flag*"},
		"переключатель, который включает или выключает новую функцию без нового деплоя."},
	{"VPN", []string{"vpn", "впн"},
		"защищённое подключение к рабочей сети; без него внутренние системы не открываются."},
	{"SSO", []string{"sso"},
		"единый вход: один рабочий логин и пароль для всех внутренних систем."},
	{"Grafana", []string{"grafana", "графан*"},
		"сервис с графиками и логами, где смотрят, как работают сервисы."},
	{"Логи", []string{"лог", "логи", "логов", "логам", "логах", "логами", "logs"},
		"записи о том, что происходило в сервисе; по ним ищут причины ошибок."},
	{"Метрики", []string{"метрик*"},
		"числа о работе сервиса во времени: нагрузка, ошибки, скорость ответа."},
	{"Алерт", []string{"алерт*", "alert*"},
		"автоматическое оповещение, что с сервисом что-то не так."},
	{"Репозиторий", []string{"репозитори*", "репо", "repo"},
		"хранилище кода проекта вместе с историей изменений."},
	{"Docker", []string{"docker", "докер*"},
		"инструмент, который запускает программы в изолированных контейнерах — одинаково на любом компьютере."},
}

// isWordRune — буква или цифра: граница слова там, где её нет.
func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// matchAlias ищет форму термина в тексте (оба — в нижнем регистре) с учётом границ слова.
func matchAlias(low, alias string) int {
	prefix := strings.HasSuffix(alias, "*")
	a := strings.TrimSuffix(alias, "*")
	for off := 0; off < len(low); {
		i := strings.Index(low[off:], a)
		if i < 0 {
			return -1
		}
		i += off
		before, _ := utf8.DecodeLastRuneInString(low[:i])
		after, _ := utf8.DecodeRuneInString(low[i+len(a):])
		if (i == 0 || !isWordRune(before)) && (prefix || i+len(a) == len(low) || !isWordRune(after)) {
			return i
		}
		off = i + len(a)
	}
	return -1
}

// termsIn — термины словаря, встречающиеся в тексте, в порядке первого упоминания.
func termsIn(text string) []glossaryTerm {
	low := strings.ToLower(text)
	type found struct {
		t   glossaryTerm
		pos int
	}
	var fs []found
	for _, t := range glossary {
		best := -1
		for _, a := range t.Aliases {
			if p := matchAlias(low, a); p >= 0 && (best < 0 || p < best) {
				best = p
			}
		}
		if best >= 0 {
			fs = append(fs, found{t, best})
		}
	}
	// сортировка вставками: терминов единицы
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && fs[j].pos < fs[j-1].pos; j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
	out := make([]glossaryTerm, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.t)
	}
	return out
}

// withGlossary дописывает к ответу до трёх терминов простыми словами.
func withGlossary(text string) string {
	if strings.Contains(text, glossaryHeader) {
		return text
	}
	ts := termsIn(text)
	if len(ts) == 0 {
		return text
	}
	if len(ts) > maxGlossaryTerms {
		ts = ts[:maxGlossaryTerms]
	}
	var sb strings.Builder
	sb.WriteString(text)
	sb.WriteString("\n\n" + glossaryHeader)
	for _, t := range ts {
		sb.WriteString("\n• " + t.Name + " — " + t.Plain)
	}
	return sb.String()
}

// definePrefixes и defineSuffixes — формы вопроса «что это за слово».
var definePrefixes = []string{
	"что такое", "что значит", "что означает", "что обозначает", "что за", "кто такой", "кто такая", "кто такие",
	"расшифруй", "расшифровка", "объясни", "поясни", "значение слова", "значение",
}

var defineSuffixes = []string{
	"это что такое", "что это такое", "это что", "что это", "что значит", "значит что", "это",
}

// fillerWords — слова-связки, которые не меняют смысл вопроса-определения.
var fillerWords = []string{"а ", "слово ", "термин ", "аббревиатура ", "аббревиатуру ", "сокращение ", "вообще "}

// glossaryQuestion распознаёт прямой вопрос о термине: «что такое MR?», «что значит ретро», «DRI — это что?».
// Вопрос с уточнением («Что такое Jira у нас?») — это вопрос к базе знаний команды, а не к словарю.
func glossaryQuestion(q string) (glossaryTerm, string, bool) {
	s := strings.ToLower(strings.TrimSpace(q))
	s = strings.TrimFunc(s, func(r rune) bool { return !isWordRune(r) })
	for _, p := range []string{"а ", "и ", "слушай ", "подскажи ", "скажи "} {
		s = strings.TrimPrefix(s, p)
	}
	matched := false
	for _, p := range definePrefixes {
		if strings.HasPrefix(s, p+" ") {
			s, matched = strings.TrimSpace(s[len(p):]), true
			break
		}
	}
	if !matched {
		for _, sfx := range defineSuffixes {
			if strings.HasSuffix(s, " "+sfx) {
				s, matched = strings.TrimSpace(s[:len(s)-len(sfx)]), true
				break
			}
		}
	}
	if !matched {
		return glossaryTerm{}, "", false
	}
	for again := true; again; {
		again = false
		for _, f := range fillerWords {
			if strings.HasPrefix(s, f) {
				s, again = strings.TrimSpace(s[len(f):]), true
			}
		}
	}
	s = strings.TrimFunc(s, func(r rune) bool { return !isWordRune(r) })
	if s == "" {
		return glossaryTerm{}, "", false
	}
	for _, t := range glossary {
		for _, a := range t.Aliases {
			base := strings.TrimSuffix(a, "*")
			if s == base {
				return t, base, true
			}
			// «апрувы», «спринты»: одно слово, начинающееся с формы термина
			if strings.HasSuffix(a, "*") && strings.HasPrefix(s, base) &&
				!strings.ContainsFunc(s[len(base):], func(r rune) bool { return !unicode.IsLetter(r) }) {
				return t, base, true
			}
		}
	}
	return glossaryTerm{}, "", false
}

// isGlossaryQuestion — вопрос-определение о термине из словаря (не пробел базы знаний).
func isGlossaryQuestion(q string) bool {
	_, _, ok := glossaryQuestion(q)
	return ok
}

// glossaryAnswer — ответ из словаря; teamLine — предложение из базы знаний команды с этим термином (может быть пустым).
func glossaryAnswer(t glossaryTerm, teamLine string) string {
	text := glossaryMark + t.Name + " — " + t.Plain
	if teamLine != "" {
		text += "\n\nКак у нас в команде: " + teamLine
	}
	return text
}

// mentions — упоминается ли термин в тексте.
func (t glossaryTerm) mentions(text string) bool {
	low := strings.ToLower(text)
	for _, a := range t.Aliases {
		if matchAlias(low, a) >= 0 {
			return true
		}
	}
	return false
}
