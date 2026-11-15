package agent

import (
	"strings"
	"unicode"

	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

// Коучинг стажёра: помощник сам понимает размытый запрос, предлагает следующий шаг
// и даёт простой путь «сообщить наставнику, что что-то не так».
//
// Приёмы, на которые опирается текст:
//   - нормализация («так бывает почти у всех») снимает страх «глупого вопроса»;
//   - архитектура выбора: 3–4 готовых варианта вместо пустого поля;
//   - один маленький следующий шаг вместо списка из десяти;
//   - подкрепление: сообщить о проблеме — правильное действие, а не беспокойство.

// SignalTool — имя «инструмента» в ответе, когда сообщение передано наставнику.
const SignalTool = "mentor_signal"

const (
	chipNext    = "Что мне делать дальше?"
	chipAbsence = "Хочу взять отгул в пятницу"
	chipAccess  = "Запроси доступ в GitLab"
	chipRules   = "Как у нас устроено код-ревью?"
	chipStuck   = "Передай наставнику: я застрял и не понимаю, что делать"
)

// signalPrefixes — явная просьба передать сообщение наставнику.
var signalPrefixes = []string{
	"передай наставнику", "сообщи наставнику", "напиши наставнику", "скажи наставнику",
	"передать наставнику", "сообщить наставнику", "позови наставника", "нужен наставник",
}

// signalRequest — текст для наставника, если стажёр явно просит передать сообщение.
func signalRequest(q string) (string, bool) {
	low := strings.ToLower(q)
	for _, p := range signalPrefixes {
		i := strings.Index(low, p)
		if i < 0 {
			continue
		}
		// всё после «передай наставнику[:,—]» — само сообщение
		rest := strings.TrimLeftFunc(q[i+len(p):], func(r rune) bool {
			return unicode.IsSpace(r) || strings.ContainsRune(":,—-–.", r)
		})
		if rest == "" {
			rest = strings.TrimSpace(q)
		}
		return rest, true
	}
	return "", false
}

// stuckMarkers — стажёр столкнулся с препятствием (с начала слова).
var stuckMarkers = []string{
	"не работает", "не работают", "сломал", "сломалось", "ошибка", "ошибку", "падает", "упал",
	"застрял", "застряла", "не могу", "не получается", "не выходит", "не запускается", "не открывается",
	"непонятно", "не понимаю", "не понял", "не поняла", "проблема", "проблемы", "блокер", "заблокирован",
	"не проходит", "не проходят", "не собирается", "не компилируется", "не билдится", "не пускает", "не грузится",
	"глючит", "лагает", "не знаю что делать",
}

func signalTopic(q string) string {
	w := " " + strings.Join(strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) }), " ") + " "
	switch {
	case containsWord(w, "не работает", "не работают", "ошибк", "падает", "упал", "сломал", "не запускается", "не открывается",
		"не проходят", "не проходит", "не собирается", "не компилируется", "не билдится", "не пускает", "глючит"):
		return "tech"
	case containsWord(w, "застрял", "не могу", "не получается", "не выходит", "блокер", "заблокирован"):
		return "blocked"
	case containsWord(w, "непонятно", "не понимаю", "не понял"):
		return "unclear"
	}
	return "other"
}

func containsWord(w string, phrases ...string) bool {
	for _, p := range phrases {
		if strings.Contains(w, " "+p) {
			return true
		}
	}
	return false
}

// isStuck — в реплике есть признак препятствия.
func isStuck(q string) bool { return signalTopic(q) != "other" || hasMarker(q, stuckMarkers) }

func hasMarker(q string, markers []string) bool {
	w := " " + strings.Join(strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) }), " ") + " "
	return containsWord(w, markers...)
}

// vagueMarkers — запрос без предмета: стажёр не знает, что спросить.
var vagueMarkers = []string{
	"помоги", "помогите", "помощь", "help", "что ты умеешь", "что умеешь", "что ты можешь", "что можешь",
	"с чего начать", "с чего мне начать", "не знаю что", "не знаю с чего", "я новенький", "я новенькая",
	"я новый", "я новая", "я стажер", "я стажёр", "меню", "start", "старт", "кто ты",
	"хелп", "как ты работаешь",
}

// isVague — короткий запрос без предмета («помоги», «с чего начать?», «что ты умеешь»).
func isVague(q string) bool {
	// «тесты не проходят, хз что делать» — это препятствие, а не размытый запрос: помогаем по шагам.
	if hasMarker(q, actionWords) || len(strings.Fields(q)) > 7 || isStuck(q) {
		return false
	}
	return hasMarker(q, vagueMarkers)
}

// menuText — ответ на размытый запрос: коротко о пользе и выбор из готовых вариантов.
func (c contacts) menuText() string {
	return "Я помогу разобраться без лишних сообщений в рабочем чате. Выберите, что нужно сейчас — " +
		"или напишите своими словами, я пойму:\n" +
		"• «Что мне делать дальше?» — покажу ваш следующий шаг адаптации и как его закрыть;\n" +
		"• «Хочу отгул в пятницу» или «Запроси доступ в GitLab» — заполню заявку, вам останется нажать «Отправить»;\n" +
		"• вопрос о правилах команды — отвечу со ссылкой на документ;\n" +
		"• «Передай наставнику: …» — если что-то не так, передам ваше сообщение наставнику (" + c.Buddy + ")."
}

// stuckText — ответ на «не работает / застрял», когда в базе знаний нет готового решения.
func (c contacts) stuckText() string {
	return "Застрять в первые недели — нормально: так бывает почти у всех. Давайте не тратить на это день.\n" +
		"1. Опишите одной фразой, что делаете и что видите (текст ошибки, экран, команду).\n" +
		"2. Если решения не нашлось за 15–20 минут — это сигнал позвать наставника, а не «дожать самому».\n" +
		"Нажмите «Передай наставнику» ниже — " + c.Buddy + " увидит сообщение в кабинете команды и ответит."
}

// signalResponse — подтверждение, что сообщение ушло наставнику (само сохранение делает API).
func (c contacts) signalResponse(text, topic string) Response {
	return Response{
		Type: "action",
		Tool: SignalTool,
		Result: &tools.Result{
			OK: true,
			Message: "Передал наставнику. Вы правильно сделали, что написали: " +
				"короткий вопрос сейчас экономит часы потом.",
			Data: map[string]any{"buddy_name": c.Buddy, "your_message": text, "what_next": "Наставник увидит сообщение в кабинете команды и ответит вам лично. Пока ждёте — можно перейти к следующему шагу плана."},
		},
		Buddy:       c.Buddy,
		Signal:      text,
		SignalTopic: topic,
		Suggestions: []string{chipNext},
	}
}

// quote обрезает вопрос для подсказки «передай наставнику».
func quote(q string) string {
	r := []rune(strings.TrimSpace(q))
	if len(r) > 80 {
		r = append(r[:80], '…')
	}
	return string(r)
}

// coach дополняет готовый ответ: при препятствии без решения в базе — пошаговая помощь,
// и всегда — 1–3 подсказки следующего шага, чтобы стажёр не оставался с пустым полем.
// question — исходная реплика (для цитаты в подсказке), norm — её нормальная форма (для распознавания).
func (c contacts) coach(question, norm string, resp Response) Response {
	if c.staff {
		return resp
	}
	stuck := isStuck(norm)
	switch {
	case stuck && resp.Type == "refusal" && resp.NoSource:
		resp.Type, resp.Text = "answer", c.stuckText()
	case stuck && resp.Type == "answer" && resp.Text != "":
		// найденный фрагмент базы может не решить проблему — шаги и выход на наставника показываем всегда
		resp.Text = c.stuckText() + "\n\nВозможно, поможет (из базы знаний):\n" + resp.Text
	}
	if len(resp.Suggestions) > 0 {
		return resp
	}
	ask := "Передай наставнику: " + quote(question)
	switch resp.Type {
	case "draft":
		// у черновика свои кнопки «Отправить» / «Отменить»
	case "action":
		if resp.Tool == "checklist_get" && resp.Result != nil {
			if how, _ := resp.Result.Data["how_to"].(string); strings.Contains(how, "доступ") {
				resp.Suggestions = []string{chipAccess, "Запроси доступ в Jira", chipStuck}
			} else {
				resp.Suggestions = []string{chipRules, chipAbsence, chipStuck}
			}
		} else {
			resp.Suggestions = []string{chipNext}
		}
	case "refusal":
		resp.Suggestions = []string{ask, chipNext}
	default:
		if stuck {
			resp.Suggestions = []string{ask, chipNext}
		} else {
			resp.Suggestions = []string{chipNext, "Передай наставнику: ответ не помог — " + quote(question)}
		}
	}
	return resp
}
