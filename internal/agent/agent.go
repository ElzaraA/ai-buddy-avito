package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

const (
	BuddyName   = "Иван Петров"
	ManagerName = "Мария Сидорова"
	// buddyDative — «обратитесь к наставнику Ивану Петрову».
	buddyDative = "Ивану Петрову"
	// maxHistory — сколько последних реплик диалога передаём модели.
	maxHistory = 6
)

// RefusalText — единый текст отказа, когда в базе нет ответа.
const RefusalText = "Не нашёл ответа в доступных источниках. Лучше спросить наставника — " + buddyDative + "."

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
}

type Response struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	Action   string            `json:"action,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
	Result   *tools.Result     `json:"result,omitempty"`
	Sources  []Source          `json:"sources,omitempty"`
	Buddy    string            `json:"buddy_name,omitempty"`
	Manager  string            `json:"manager_name,omitempty"`
	Degraded bool              `json:"degraded,omitempty"` // ответ дан резервной моделью
	// NoSource — отказ из-за отсутствия ответа в базе (сигнал «пробел в базе знаний»).
	NoSource bool `json:"-"`
}

func refusal(text string) Response {
	return Response{Type: "refusal", Text: text, Buddy: BuddyName}
}

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
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
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	hits := a.Index.Search(question, user.TeamID, string(user.Role), 4)
	srcs := make([]Source, 0, len(hits))
	for _, h := range hits {
		srcs = append(srcs, Source{ID: h.Chunk.ID, Source: h.Chunk.Source, Stage: h.Chunk.Stage, Title: h.Chunk.Title})
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
	resp := a.interpret(ctx, user, question, raw, srcs)
	resp.Degraded = degraded
	return resp, nil
}

// interpret превращает вывод модели в ответ, применяя инварианты из specs/agent/spec.md.
func (a *Agent) interpret(ctx context.Context, user auth.User, question, raw string, srcs []Source) Response {
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
			return refusal("Этот инструмент мне недоступен. Обратитесь к наставнику " + buddyDative + ".")
		}
		// Инвариант 2: право проверяется до любого действия, в том числе до подготовки черновика.
		if !auth.Can(user, t.RequiredPermission()) {
			return refusal("У вашей роли нет доступа к этому действию. Обратитесь к наставнику " + buddyDative + ".")
		}
		// Инвариант 4: мутирующие инструменты не выполняются сразу — только черновик.
		if t.RequiresConfirmation() {
			return a.draftFor(question, out.Tool, out.Args, srcs)
		}
		res, err := tools.Run(ctx, t, user, out.Args)
		if err != nil {
			return refusal("Действие сейчас недоступно. Обратитесь к наставнику " + buddyDative + ".")
		}
		return Response{Type: "action", Result: &res, Sources: srcs, Buddy: BuddyName, Manager: ManagerName}

	case "draft":
		tool := draftTool(out.Action)
		if t, ok := a.Tools.Get(tool); ok && !auth.Can(user, t.RequiredPermission()) {
			return refusal("У вашей роли нет доступа к этому действию. Обратитесь к наставнику " + buddyDative + ".")
		}
		return a.draftFor(question, tool, out.Args, srcs)

	case "refusal":
		text := out.Text
		if text == "" {
			text = RefusalText
		}
		r := refusal(text)
		r.NoSource = len(srcs) == 0
		return r
	}
	// Инвариант 1: нет источника — нет ответа (защита от галлюцинаций реальной LLM).
	if len(srcs) == 0 || strings.TrimSpace(out.Text) == "" {
		r := refusal(RefusalText)
		r.NoSource = true
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
func (a *Agent) DraftFor(question, tool string, args map[string]string) Response {
	return a.draftFor(question, tool, args, nil)
}

// draftFor строит черновик действия; само действие создаётся только после POST /actions/{id}/confirm.
func (a *Agent) draftFor(question, tool string, args map[string]string, srcs []Source) Response {
	switch tool {
	case "absence_create_request":
		return a.absenceDraft(question, args, srcs)
	case "access_request":
		res := strings.ToLower(strings.TrimSpace(args["resource"]))
		if res == "" {
			res = "jira"
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
			Sources: srcs,
			Buddy:   BuddyName,
			Manager: ManagerName,
		}
	}
	return refusal("Не знаю такого действия. Обратитесь к наставнику " + buddyDative + ".")
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

func (a *Agent) absenceDraft(question string, args map[string]string, srcs []Source) Response {
	from, to := args["date_from"], args["date_to"]
	if from == "" {
		from, to = args["dateFrom"], args["dateTo"]
	}
	// Даты от модели принимаем только корректные; иначе разбираем вопрос сами.
	if f, t := ParseDates(from + " " + to); f != "" {
		from, to = f, t
	} else {
		from, to = ParseDates(question)
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
		Buddy:   BuddyName,
		Manager: ManagerName,
	}
	if from == "" {
		base.Args = map[string]string{"reason": "personal"}
		example := a.now().AddDate(0, 0, 7).Format("2006-01-02")
		base.Text = "На какую дату оформить отгул? Напишите, например, «" + example + "» или «в пятницу». Согласует руководитель — " + ManagerName + "."
		return base
	}
	if from < a.now().Format("2006-01-02") {
		base.Args = map[string]string{"reason": "personal"}
		base.Text = "Дата " + from + " уже прошла. На какую будущую дату оформить отгул?"
		return base
	}
	base.Args = map[string]string{
		"date_from": from, "date_to": to, "reason": "personal",
		"suggest_notify_buddy": "true",
	}
	base.Text = "Подготовил черновик заявки на отгул. Её согласует руководитель — " + ManagerName +
		". Предупредите наставника " + BuddyName + " — сам писать ему я не буду. Проверьте даты и подтвердите."
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
