package api

import (
	"time"
	"unicode/utf8"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// saveDraft сохраняет черновик действия из ответа агента и кладёт его id в args.action_id.
// Черновик отгула без дат не сохраняется: агент сначала уточняет дату.
// Черновик проверяется теми же правилами, что и POST /actions (store.ValidateAction):
// некорректный не сохраняется, а пользователь получает объяснение (в MCP — isError).
func saveDraft(st *store.Store, u auth.User, resp *agent.Response) {
	saveDraftAt(st, u, resp, time.Now())
}

func saveDraftAt(st *store.Store, u auth.User, resp *agent.Response, now time.Time) {
	if resp.Type != "draft" {
		return
	}
	if resp.Args == nil {
		resp.Args = map[string]string{}
	}
	a := store.Absence{Type: resp.Action, UserID: u.ID, TeamID: u.TeamID, Reason: resp.Args["reason"]}
	switch resp.Action {
	case store.ActionAbsence:
		if resp.Args["date_from"] == "" {
			return
		}
		a.DateFrom, a.DateTo = resp.Args["date_from"], resp.Args["date_to"]
		if a.DateTo == "" {
			a.DateTo = a.DateFrom
		}
	case store.ActionAccess:
		a.Resource = store.NormalizeResource(resp.Args["resource"])
	default:
		return
	}
	if err := store.ValidateAction(a, now); err != nil {
		rejectDraft(resp, draftProblem(a))
		return
	}
	a.ID = st.NextID(idPrefix(a.Type))
	created, err := st.CreateDraft(a)
	if err != nil {
		*resp = agent.Response{Type: "refusal", Buddy: agent.BuddyName,
			Text: "У вас уже много неподтверждённых черновиков. Подтвердите или отмените старые в разделе «Мои заявки» и повторите запрос."}
		return
	}
	resp.Args["action_id"] = created.ID
}

// rejectDraft оставляет пользователя без сохранённого черновика и объясняет почему.
// Отгул остаётся уточняющим черновиком без дат (диалог продолжается следующей датой),
// доступ превращается в отказ. В обоих случаях action_id нет — MCP вернёт isError.
func rejectDraft(resp *agent.Response, text string) {
	if resp.Action == store.ActionAbsence {
		resp.Args = map[string]string{"reason": "personal"}
		resp.Text = text
		return
	}
	*resp = agent.Response{Type: "refusal", Text: text, Buddy: agent.BuddyName}
}

// draftProblem — человеческое объяснение, почему черновик не прошёл проверку.
func draftProblem(a store.Absence) string {
	switch {
	case utf8.RuneCountInString(a.Reason) > store.MaxReasonRunes:
		return "Причина заявки слишком длинная — сократите её до 500 символов."
	case a.Type == store.ActionAccess:
		return "Могу запросить доступ только к Jira, GitLab, Grafana или Git. По другим системам подскажет наставник " + agent.BuddyName + "."
	}
	return "Не могу оформить отгул на эти даты: дата не должна быть в прошлом, окончание — не раньше начала, а отгул — не длиннее 30 дней. Напишите другие даты."
}

// permissionFor — право, необходимое для подтверждения действия данного типа.
func permissionFor(actionType string) string {
	if actionType == store.ActionAccess {
		return "access.request"
	}
	return "absence.create"
}

// syncStep поддерживает связь «заявка на доступ ↔ шаг чек-листа»:
// отправлена → шаг ждёт (waiting); одобрена → выполнен; отклонена или отменена → снова todo.
func syncStep(st *store.Store, a store.Absence) {
	if a.Type != store.ActionAccess {
		return
	}
	switch a.Status {
	case store.StatusPending:
		st.LinkStep(a.UserID, "access", a.ID, store.StepWaiting)
	case store.StatusApproved:
		st.LinkStep(a.UserID, "access", a.ID, store.StepDone)
	case store.StatusRejected, store.StatusCancelled:
		st.UnlinkStep(a.UserID, a.ID)
	}
}
