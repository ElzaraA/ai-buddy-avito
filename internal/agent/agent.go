package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

const (
	// BuddyName и ManagerName — контакты команды по умолчанию (backend). Для конкретного пользователя
	// имена берутся из справочника команд auth.TeamContacts.
	BuddyName   = "Иван Петров"
	ManagerName = "Мария Сидорова"
	// maxHistory — сколько последних реплик диалога передаём модели.
	maxHistory = 6
)

// RefusalText — текст отказа, когда в базе нет ответа, для стажёра команды по умолчанию (backend).
// Для пользователя он строится по его команде и роли: contactsFor(u).refusalText().
const RefusalText = "Не нашёл ответа в доступных источниках. Лучше написать наставнику — Ивану Петрову."

// resourceList — системы, на которые можно оформить заявку на доступ (store.ValidResource).
const resourceList = "Jira, GitLab, Grafana или Git"

// contacts — к кому направлять пользователя. Стажёра — к наставнику его команды;
// наставнику и руководителю советовать «обратиться к наставнику» (к себе или к подчинённому) нельзя.
type contacts struct {
	auth.Contacts
	foreign []string // фамилии наставников и руководителей других команд
	staff   bool     // наставник или руководитель
}

func contactsFor(u auth.User) contacts {
	return contacts{
		Contacts: auth.TeamContacts(u.TeamID),
		foreign:  auth.ForeignContactStems(u.TeamID),
		staff:    u.Role == auth.RoleBuddy || u.Role == auth.RoleLead,
	}
}

// help — короткий совет, к кому обратиться.
func (c contacts) help() string {
	if c.staff {
		return "Обратитесь к HR BP команды."
	}
	return "Обратитесь к наставнику " + c.BuddyDative + "."
}

// refusalText — отказ, когда в базе нет ответа.
func (c contacts) refusalText() string {
	if c.staff {
		return "Не нашёл ответа в доступных источниках. Уточните вопрос у HR BP команды."
	}
	return "Не нашёл ответа в доступных источниках. Лучше написать наставнику — " + c.BuddyDative + "."
}

// otherSystems — куда идти с доступом к системе не из списка.
func (c contacts) otherSystems() string {
	if c.staff {
		return "По другим системам обратитесь в IT Support."
	}
	return "По другим системам подскажет наставник " + c.Buddy + "."
}

func (c contacts) refusal(text string) Response {
	return Response{Type: "refusal", Text: text, Buddy: c.Buddy}
}

// mentionsForeign — в тексте назван наставник или руководитель чужой команды.
func (c contacts) mentionsForeign(text string) bool {
	for _, s := range c.foreign {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

// withHelp приводит текст отказа модели к контактам пользователя: чужое имя или совет
// «обратиться к наставнику» для наставника/руководителя заменяются, отсутствующий совет дописывается.
func (c contacts) withHelp(text string) string {
	low := strings.ToLower(text)
	if c.mentionsForeign(text) || (c.staff && strings.Contains(low, "наставник")) {
		return "Не могу помочь с этим вопросом. " + c.help()
	}
	if strings.Contains(low, "наставник") || strings.Contains(text, "HR BP") {
		return text
	}
	text = strings.TrimSpace(text)
	if !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, "!") && !strings.HasSuffix(text, "?") {
		text += "."
	}
	return text + " " + c.help()
}

type Agent struct {
	LLM llm.Client
	// Fallback — резервная модель (детерминированный mock): используется, если основная недоступна.
	// Ответ помечается degraded=true, сервис продолжает работать.
	Fallback llm.Client
	Index    *rag.Index
	Tools    *tools.Registry
	Now      func() time.Time
	// OnFallback вызывается при переходе на резервную модель (метрика).
	OnFallback func(err error)
	// AccessStatus — состояние заявок пользователя на доступ по системам (store.AccessStatus);
	// nil — подсказки не сверяются с заявками.
	AccessStatus func(userID string) map[string]string
}

type Response struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	Action   string            `json:"action,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
	Result   *tools.Result     `json:"result,omitempty"`
	Tool     string            `json:"tool,omitempty"` // имя выполненного инструмента (для type=action)
	Sources  []Source          `json:"sources,omitempty"`
	Buddy    string            `json:"buddy_name,omitempty"`
	Manager  string            `json:"manager_name,omitempty"`
	Degraded bool              `json:"degraded,omitempty"` // ответ дан резервной моделью
	// NoSource — отказ из-за отсутствия ответа в базе (сигнал «пробел в базе знаний»).
	NoSource bool `json:"-"`
	// Suggestions — готовые следующие реплики стажёра (кнопки под ответом).
	Suggestions []string `json:"suggestions,omitempty"`
	// Signal — сообщение наставнику, которое API сохранит в кабинет команды.
	Signal      string `json:"-"`
	SignalTopic string `json:"-"`
	// Kind — служебная пометка происхождения ответа для учёта пользы (KindGlossary — словарь Бадди).
	Kind string `json:"-"`
}

// KindGlossary — ответ из словаря терминов Бадди.
const KindGlossary = "glossary"

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return store.Now()
}

// Ask отвечает на одиночный вопрос без истории диалога.
func (a *Agent) Ask(ctx context.Context, user auth.User, question string) (Response, error) {
	return a.AskWithHistory(ctx, user, question, nil)
}

// complete вызывает основную модель, а при её сбое — резервную.
func (a *Agent) complete(ctx context.Context, msgs []llm.Message) (raw string, degraded bool, err error) {
	raw, err = a.LLM.Complete(ctx, msgs)
	if err == nil {
		return raw, false, nil
	}
	if a.Fallback == nil || errors.Is(ctx.Err(), context.Canceled) {
		return "", false, err
	}
	if a.OnFallback != nil {
		a.OnFallback(err)
	}
	// Резервной модели даём свой короткий таймаут: основной контекст мог уже истечь.
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	raw, ferr := a.Fallback.Complete(fctx, msgs)
	if ferr != nil {
		return "", false, err
	}
	return raw, true, nil
}

// AskWithHistory отвечает на вопрос с учётом последних реплик диалога (history — в хронологическом порядке).
func (a *Agent) AskWithHistory(ctx context.Context, user auth.User, question string, history []llm.Message) (Response, error) {
	resp, err := a.askWithHistory(ctx, user, question, history)
	// Не предлагаем «Запроси доступ в GitLab», если заявка на GitLab уже отправлена или одобрена.
	if err == nil && a.AccessStatus != nil && len(resp.Suggestions) > 0 {
		resp.Suggestions = freshAccessChips(resp.Suggestions, a.AccessStatus(user.ID))
	}
	return resp, err
}

func (a *Agent) askWithHistory(ctx context.Context, user auth.User, question string, history []llm.Message) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	c := contactsFor(user)
	// nq — реплика в нормальной форме («чё» → «что», «ё» → «е», «пн» → «понедельник»): по ней распознаём намерение.
	// Исходный текст остаётся для модели, цитат в подсказках и сообщения наставнику.
	nq := llm.Normalize(question)
	// «Мне плохо», «я не справляюсь» — не вопрос к базе знаний: поддерживаем и предлагаем живого человека.
	if isEmotional(nq) {
		return Response{Type: "answer", Text: c.supportText(), Buddy: c.Buddy}, nil
	}
	// Приветствие, «спасибо», «ок», «отмени» — не вопрос к базе знаний: отвечаем сразу, без LLM и без отказа.
	// «Передай наставнику: …» — стажёр сам решил позвать человека: передаём без вопросов.
	if !c.staff {
		text, ok := signalRequest(question)
		if !ok {
			// опечатка в «наставнику» — ищем по нормальной форме
			text, ok = signalRequest(nq)
		}
		if ok {
			return c.signalResponse(text, signalTopic(text)), nil
		}
	}
	if reply := smallTalk(nq); reply != "" {
		r := Response{Type: "answer", Text: reply, Buddy: c.Buddy}
		if !c.staff {
			r.Suggestions = []string{chipNext}
			if reply == greetReply {
				r.Suggestions = []string{chipNext, chipAbsence, chipAccess, chipRules}
			}
		}
		return r, nil
	}
	// «Помоги», «с чего начать?» — стажёр не знает, что спросить: предлагаем выбор.
	if !c.staff && isVague(nq) {
		r := Response{Type: "answer", Text: c.menuText(), Buddy: c.Buddy,
			Suggestions: []string{chipNext, chipAbsence, chipAccess, chipRules}}
		return r, nil
	}
	// «Что такое MR?», «что значит ретро» — объясняем термин из словаря Бадди (это не пробел базы знаний).
	if t, alias, ok := glossaryQuestion(nq); ok {
		return a.glossaryResponse(user, c, question, t, alias), nil
	}
	// Зарплаты и доходы — чувствительные данные: отказываем явно и подсказываем, к кому идти,
	// а не общим «не нашёл» (вопрос всё равно попадает в пробелы базы знаний для наставника).
	// «Кому написать про условия стажировки?» — безопасный следующий шаг после отказа про зарплату.
	if isConditionsQuestion(nq) && !isSalaryQuestion(nq) {
		r := Response{Type: "answer", Text: c.conditionsText(), Buddy: c.Buddy}
		if !c.staff {
			r.Suggestions = []string{chipNext}
		}
		return r, nil
	}
	if isSalaryQuestion(nq) {
		r := c.refusal(c.salaryText())
		r.NoSource = isKnowledgeGap(nq)
		return c.coach(question, nq, r), nil
	}

	hits := a.Index.Search(nq, user.TeamID, string(user.Role), 4)
	srcs := make([]Source, 0, len(hits))
	for _, h := range hits {
		srcs = append(srcs, Source{ID: h.Chunk.ID, Source: h.Chunk.Source, Stage: h.Chunk.Stage, Title: h.Chunk.Title})
	}
	// Ответ есть, но в материале для другой аудитории (например, «только для наставников»): это не пробел базы.
	restricted := len(hits) == 0 && a.Index.Restricted(nq, user.TeamID, string(user.Role))
	// Кадровые темы вне базы (командировка, ДМС, декрет…) похожи на действия «отгул/заявка», но ими не являются:
	// без фрагмента базы честно отказываем, не давая модели подменить вопрос политикой отгулов.
	if len(hits) == 0 && !restricted && outOfScope(nq) {
		r := c.refusal(c.refusalText())
		r.NoSource = isKnowledgeGap(nq)
		return c.coach(question, nq, r), nil
	}

	// Действия (отгул, доступ, чек-лист) не зависят от базы знаний, поэтому LLM вызываем всегда,
	// а инвариант «нет источника — нет ответа» проверяем ниже для текстовых ответов.
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	msgs := make([]llm.Message, 0, len(history)+2)
	msgs = append(msgs, llm.Message{Role: "system", Content: SystemPrompt})
	msgs = append(msgs, history...)
	msgs = append(msgs, llm.Message{Role: "user", Content: UserPrompt(question, hits, string(user.Role))})

	raw, degraded, err := a.complete(ctx, msgs)
	if err != nil {
		return Response{}, err
	}
	resp := c.coach(question, nq, c.present(a.interpret(ctx, user, nq, raw, srcs, restricted), hits))
	resp.Degraded = degraded
	return resp, nil
}

// present оформляет ответ для стажёра первой недели: дословный фрагмент базы знаний — структурой
// «Коротко → шаги → кого спросить», любой ответ по базе — блоком «Термины простыми словами».
// Источники (resp.Sources) не меняются.
func (c contacts) present(resp Response, hits []rag.Hit) Response {
	if resp.Type != "answer" || len(resp.Sources) == 0 || strings.TrimSpace(resp.Text) == "" {
		return resp
	}
	txt := strings.TrimSpace(resp.Text)
	for _, h := range hits {
		if txt == strings.TrimSpace(h.Chunk.Text) {
			resp.Text = c.formatKB(h.Chunk.Text)
			break
		}
	}
	resp.Text = withGlossary(resp.Text)
	return resp
}

// glossaryResponse — ответ на «что такое X?» из словаря Бадди. Если в базе знаний команды есть
// предложение с этим термином, добавляем его дословно и прикладываем источник.
func (a *Agent) glossaryResponse(user auth.User, c contacts, question string, t glossaryTerm, alias string) Response {
	var line string
	var srcs []Source
	for _, q := range []string{alias, question} {
		for _, h := range a.Index.Search(q, user.TeamID, string(user.Role), 4) {
			for _, s := range sentences(h.Chunk.Text) {
				if t.mentions(s) {
					line = s
					srcs = []Source{{ID: h.Chunk.ID, Source: h.Chunk.Source, Stage: h.Chunk.Stage, Title: h.Chunk.Title}}
					break
				}
			}
			if line != "" {
				break
			}
		}
		if line != "" {
			break
		}
	}
	r := Response{Type: "answer", Text: glossaryAnswer(t, line), Sources: srcs, Buddy: c.Buddy, Kind: KindGlossary}
	if !c.staff {
		r.Suggestions = []string{chipNext, "Передай наставнику: ответ не помог — " + quote(question)}
	}
	return r
}

// restrictedRefusal — материал по вопросу есть, но доступен наставникам и руководителям.
func (c contacts) restrictedRefusal() Response {
	if c.staff {
		return c.refusal("Этот материал предназначен для другой роли в команде. " + c.help())
	}
	return c.refusal("Этот материал доступен наставникам и руководителям команды. " + c.help())
}

// interpret превращает вывод модели в ответ, применяя инварианты из specs/agent/spec.md.
// restricted — по вопросу нашёлся только материал для другой аудитории (не пробел базы знаний).
func (a *Agent) interpret(ctx context.Context, user auth.User, question, raw string, srcs []Source, restricted bool) Response {
	c := contactsFor(user)
	raw = stripCodeFence(raw)
	var out AgentOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		// Модель ответила не JSON — показываем как текст, но только если есть источники.
		out = AgentOutput{Type: "answer", Text: raw}
	}

	switch out.Type {
	case "tool_call":
		t, ok := a.Tools.Get(out.Tool)
		if !ok {
			return c.refusal("Этот инструмент мне недоступен. " + c.help())
		}
		// Инвариант 2: право проверяется до любого действия, в том числе до подготовки черновика.
		if !auth.Can(user, t.RequiredPermission()) {
			return c.refusal("У вашей роли нет доступа к этому действию. " + c.help())
		}
		// Инвариант 4: мутирующие инструменты не выполняются сразу — только черновик.
		if t.RequiresConfirmation() {
			return a.draftFor(user, question, out.Tool, out.Args, srcs)
		}
		res, err := tools.Run(ctx, t, user, out.Args)
		if err != nil {
			return c.refusal("Действие сейчас недоступно. " + c.help())
		}
		return Response{Type: "action", Result: &res, Tool: t.Name(), Sources: srcs, Buddy: c.Buddy, Manager: c.Manager}

	case "draft":
		tool := draftTool(out.Action)
		if t, ok := a.Tools.Get(tool); ok && !auth.Can(user, t.RequiredPermission()) {
			return c.refusal("У вашей роли нет доступа к этому действию. " + c.help())
		}
		return a.draftFor(user, question, tool, out.Args, srcs)

	case "refusal":
		if restricted {
			return c.restrictedRefusal()
		}
		text := out.Text
		// Инвариант 1 действует и для отказа: без источников текст модели не показываем дословно,
		// иначе выдуманный факт («Зарплата стажёра 120 000 ₽») просочится под видом отказа.
		switch {
		case len(srcs) == 0 && looksLikeInjection(question):
			text = "Не могу выполнить это действие. " + c.help()
		case text == "" || (len(srcs) == 0 && !safeRefusal(text, c)):
			text = c.refusalText()
		default:
			text = c.withHelp(text)
		}
		r := c.refusal(text)
		r.NoSource = len(srcs) == 0 && isKnowledgeGap(question)
		return r
	}
	// Инвариант 1: нет источника — нет ответа (защита от галлюцинаций реальной LLM).
	if len(srcs) == 0 || strings.TrimSpace(out.Text) == "" {
		if restricted {
			return c.restrictedRefusal()
		}
		r := c.refusal(c.refusalText())
		r.NoSource = isKnowledgeGap(question)
		return r
	}
	return Response{Type: "answer", Text: out.Text, Sources: srcs}
}

func draftTool(action string) string {
	switch action {
	case store.ActionAbsence:
		return "absence_create_request"
	case store.ActionAccess:
		return "access_request"
	}
	return action
}

// DraftFor строит черновик действия для инструмента (используется и агентом, и MCP-эндпоинтом).
// Пользователь нужен, чтобы назвать наставника и руководителя его команды.
func (a *Agent) DraftFor(u auth.User, question, tool string, args map[string]string) Response {
	return a.draftFor(u, question, tool, args, nil)
}

// draftFor строит черновик действия; само действие создаётся только после POST /actions/{id}/confirm.
func (a *Agent) draftFor(u auth.User, question, tool string, args map[string]string, srcs []Source) Response {
	c := contactsFor(u)
	switch tool {
	case "absence_create_request":
		return a.absenceDraft(c, question, args, titled(srcs, "отгул"))
	case "access_request":
		res := store.NormalizeResource(args["resource"])
		// Система не названа или не из списка — честно говорим, на что можно оформить заявку,
		// и черновик не создаём (раньше неизвестная система молча становилась Jira).
		if res == "" {
			return c.refusal("К какой системе нужен доступ? Заявку можно оформить только на " + resourceList +
				" — напишите, например, «Запроси доступ в GitLab».")
		}
		if !store.ValidResource(res) {
			return c.refusal("Заявку на доступ к " + foreignResource(res) + " оформить не могу: заявку можно оформить только на " +
				resourceList + ". " + c.otherSystems())
		}
		reason := args["reason"]
		if reason == "" {
			reason = "onboarding"
		}
		return Response{
			Type:    "draft",
			Action:  store.ActionAccess,
			Args:    map[string]string{"resource": res, "reason": reason},
			Text:    "Подготовил заявку на доступ к " + displayResource(res) + ". Проверьте и подтвердите отправку — без подтверждения заявка не уйдёт.",
			Sources: titled(srcs, "доступ"),
			Buddy:   c.Buddy,
			Manager: c.Manager,
		}
	}
	return c.refusal("Не знаю такого действия. " + c.help())
}

// foreignResource — имя системы не из списка для текста ответа: короткое слово показываем
// с заглавной буквы («confluence» → «Confluence»), всё остальное (разметку, длинные строки) — не повторяем.
func foreignResource(r string) string {
	rs := []rune(r)
	if len(rs) == 0 || len(rs) > 30 {
		return "этой системе"
	}
	for _, x := range rs {
		if !unicode.IsLetter(x) && !unicode.IsDigit(x) && x != '-' && x != ' ' && x != '.' {
			return "этой системе"
		}
	}
	return string(unicode.ToUpper(rs[0])) + string(rs[1:])
}

// titled оставляет у черновика только источники по его теме: «Хочу отгул послезавтра»
// не должен прикладывать код-ревью только потому, что «после» совпало с «после апрува».
func titled(srcs []Source, topic string) []Source {
	var out []Source
	for _, s := range srcs {
		if strings.Contains(strings.ToLower(s.Title), topic) {
			out = append(out, s)
		}
	}
	return out
}

func displayResource(r string) string {
	switch r {
	case "jira":
		return "Jira"
	case "gitlab":
		return "GitLab"
	case "grafana":
		return "Grafana"
	case "git":
		return "Git"
	}
	return r
}

func (a *Agent) absenceDraft(c contacts, question string, args map[string]string, srcs []Source) Response {
	from, to := args["date_from"], args["date_to"]
	if from == "" {
		from, to = args["dateFrom"], args["dateTo"]
	}
	// Даты от модели принимаем только корректные; иначе разбираем вопрос сами.
	if f, t := ParseDates(from + " " + to); f != "" {
		from, to = f, t
	} else {
		from, to = ParseDatesAt(question, a.now())
	}
	if from == "" {
		if d, ok := ParseWeekday(question, a.now()); ok {
			from, to = d, d
		}
	}
	base := Response{
		Type:    "draft",
		Action:  store.ActionAbsence,
		Sources: srcs,
		Buddy:   c.Buddy,
		Manager: c.Manager,
	}
	if from == "" {
		base.Args = map[string]string{"reason": "personal"}
		example := HumanDate(a.now().AddDate(0, 0, 7))
		base.Text = "На какую дату оформить отгул? Напишите, например, «" + example + "», «20.11» или «в пятницу». Согласует руководитель — " + c.Manager + "."
		if bad := InvalidDate(question); bad != "" {
			base.Text = "Даты " + bad + " не существует. " + base.Text
		}
		return base
	}
	if from < a.now().Format("2006-01-02") {
		base.Args = map[string]string{"reason": "personal"}
		base.Text = "Дата " + from + " уже прошла. На какую будущую дату оформить отгул?"
		return base
	}
	// Те же правила, что у POST /actions: отгул не длиннее 30 дней.
	if err := store.ValidateAbsenceDates(from, to, a.now()); err != nil {
		base.Args = map[string]string{"reason": "personal"}
		if errors.Is(err, store.ErrTooFarAhead) {
			base.Text = "Дата " + from + " слишком далеко: отгул можно оформить не дальше чем на 5 лет вперёд. Проверьте год — на какую дату оформить отгул?"
			return base
		}
		base.Text = "Отгул можно оформить не больше чем на 30 дней подряд. Для более долгого отсутствия обратитесь к руководителю — " + c.Manager + ". На какие даты оформить отгул?"
		return base
	}
	base.Args = map[string]string{"date_from": from, "date_to": to, "reason": "personal"}
	base.Text = "Подготовил черновик заявки на отгул. Её согласует руководитель — " + c.Manager + ". "
	if !c.staff {
		// Наставнику и руководителю не советуем «предупредить наставника» — себя или подчинённого.
		base.Args["suggest_notify_buddy"] = "true"
		base.Text += "Предупредите наставника " + c.BuddyAccusative + " — сам писать я не буду. "
	}
	base.Text += "Проверьте даты и подтвердите."
	return base
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
