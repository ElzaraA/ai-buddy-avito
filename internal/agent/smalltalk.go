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
// Допускаем только короткую отсылку к наставнику команды пользователя без цифр: в ней нет места
// выдуманным фактам. Наставнику и руководителю такой текст не подходит (советовал бы обратиться к себе).
func safeRefusal(text string, c contacts) bool {
	if c.staff || utf8.RuneCountInString(text) > maxModelRefusalRunes || len(c.Stems) == 0 ||
		!strings.Contains(text, c.Stems[0]) || c.mentionsForeign(text) {
		return false
	}
	return !strings.ContainsFunc(text, unicode.IsDigit)
}

// emotionalMarkers — признаки того, что стажёру тяжело (с начала слова). Это не вопрос к базе знаний.
var emotionalMarkers = []string{
	"мне плохо", "плохо себя чувствую", "не справляюсь", "не справлюсь", "устал", "выгора", "выгорел",
	"хочу уволиться", "уволюсь", "думаю уволиться", "мне тяжело", "опускаются руки", "нет сил", "депресс",
	"хочу всё бросить", "хочу все бросить", "ничего не получается", "у меня ничего не выходит",
}

// actionWords — реплика с ними остаётся запросом действия («мне плохо, хочу отгул завтра» → черновик отгула).
var actionWords = []string{"отгул", "отпуск", "выходн", "доступ", "больнич"}

// isEmotional — реплика про самочувствие или выгорание, а не вопрос к базе и не запрос действия.
func isEmotional(q string) bool {
	low := strings.ToLower(q)
	for _, w := range actionWords {
		if strings.Contains(low, w) {
			return false
		}
	}
	words := " " + strings.Join(strings.FieldsFunc(low, func(r rune) bool { return !unicode.IsLetter(r) }), " ") + " "
	for _, m := range emotionalMarkers {
		if strings.Contains(words, " "+m) {
			return true
		}
	}
	return false
}

// supportText — поддерживающий ответ с предложением поговорить с человеком.
func (c contacts) supportText() string {
	if c.staff {
		return "Понимаю, сейчас непросто. Предлагаю обсудить нагрузку с руководителем или с HR BP команды — " +
			"они помогут найти решение. Если вы заболели, оформите отгул: напишите, например, «Хочу отгул завтра»."
	}
	return "Понимаю, сейчас непросто — в начале стажировки так бывает почти у всех, и это нормально. " +
		"Предлагаю обсудить это с наставником — " + c.Buddy + ": напишите в личные сообщения или попросите внеочередную встречу 1:1. " +
		"Если хочется поговорить с кем-то вне команды, можно обратиться к HR BP. " +
		"Если вы заболели, оформите отгул: напишите, например, «Хочу отгул завтра»."
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
	return rag.Meaningful(q) && !looksLikeInjection(q) && smallTalk(q) == "" && !isEmotional(q)
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
