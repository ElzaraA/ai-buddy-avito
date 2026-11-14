package agent

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
)

// maxModelRefusalRunes — длиннее этого текст отказа модели без источников не показываем.
const maxModelRefusalRunes = 200

// safeRefusal — можно ли показать текст отказа модели, если источников нет.
// Допускаем только короткую отсылку к наставнику без цифр: в ней нет места выдуманным фактам.
func safeRefusal(text string) bool {
	if utf8.RuneCountInString(text) > maxModelRefusalRunes || !strings.Contains(text, "Петров") {
		return false
	}
	return !strings.ContainsFunc(text, unicode.IsDigit)
}

// injectionMarkers — признаки попытки переписать инструкции помощника.
var injectionMarkers = []string{
	"ignore previous", "ignore all", "игнорируй", "забудь все", "забудь инструкц", "забудь правила",
	"системный промпт", "system prompt", "ты теперь", "you are now", "system:", "role=", "dan ",
}

// looksLikeInjection — вопрос похож на prompt injection (такие отказы не являются пробелом базы знаний).
func looksLikeInjection(q string) bool {
	q = strings.ToLower(q) + " "
	for _, m := range injectionMarkers {
		if strings.Contains(q, m) {
			return true
		}
	}
	return false
}

// isKnowledgeGap — стоит ли записать вопрос без ответа в «пробелы базы знаний» для наставника.
// Small talk, инъекции и вопросы без значимых слов пробелом не считаются.
func isKnowledgeGap(q string) bool {
	return rag.Meaningful(q) && !looksLikeInjection(q) && smallTalk(q) == ""
}

// Реплики small talk: помощник отвечает вежливо, без поиска по базе и без записи в пробелы.
const (
	greetReply  = "Привет! Я помогу с онбордингом: отвечу по базе знаний команды, покажу чек-лист и подготовлю заявку на отгул или доступ."
	thanksReply = "Пожалуйста! Если появятся вопросы — пишите."
	ackReply    = "Хорошо. Если что-то понадобится — спрашивайте."
	cancelReply = "Черновик отменяется кнопкой «Отменить» на его карточке, а отправленную заявку можно отменить в разделе «Мои заявки»."
)

// smallTalkWords — слово → вид реплики. Реплика считается small talk, только если ВСЕ её слова отсюда.
var smallTalkWords = map[string]string{
	"привет": greetReply, "приветик": greetReply, "здравствуй": greetReply, "здравствуйте": greetReply,
	"добрый": greetReply, "доброе": greetReply, "день": greetReply, "утро": greetReply, "вечер": greetReply,
	"хай": greetReply, "hi": greetReply, "hello": greetReply,
	"спасибо": thanksReply, "спс": thanksReply, "благодарю": thanksReply, "большое": thanksReply, "thanks": thanksReply,
	"ок": ackReply, "окей": ackReply, "ok": ackReply, "понятно": ackReply, "ясно": ackReply,
	"хорошо": ackReply, "отлично": ackReply, "супер": ackReply, "пока": ackReply, "до": ackReply, "свидания": ackReply,
	"отмена": cancelReply, "отмени": cancelReply, "отменить": cancelReply, "отменяй": cancelReply,
}

// smallTalk возвращает ответ на приветствие, благодарность, «ок» или «отмени»; пусто — это не small talk.
// При смешанной реплике («спасибо, ок») приоритет у отмены, затем у благодарности и приветствия.
func smallTalk(q string) string {
	words := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) })
	if len(words) == 0 || len(words) > 4 {
		return ""
	}
	seen := map[string]bool{}
	for _, w := range words {
		reply, ok := smallTalkWords[w]
		if !ok {
			return ""
		}
		seen[reply] = true
	}
	for _, reply := range []string{cancelReply, thanksReply, greetReply, ackReply} {
		if seen[reply] {
			return reply
		}
	}
	return ""
}
