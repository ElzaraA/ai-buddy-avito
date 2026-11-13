package api

import (
	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// saveDraft сохраняет черновик действия из ответа агента и кладёт его id в args.action_id.
// Черновик отгула без дат не сохраняется: агент сначала уточняет дату.
func saveDraft(st *store.Store, u auth.User, resp *agent.Response) {
	if resp.Type != "draft" {
		return
	}
	if resp.Args == nil {
		resp.Args = map[string]string{}
	}
	switch resp.Action {
	case store.ActionAbsence:
		if resp.Args["date_from"] == "" {
			return
		}
		a := st.CreateAbsence(store.Absence{
			ID: st.NextID("ABS"), Type: store.ActionAbsence, UserID: u.ID, TeamID: u.TeamID,
			DateFrom: resp.Args["date_from"], DateTo: resp.Args["date_to"], Reason: resp.Args["reason"],
			Status: store.StatusDraft,
		})
		resp.Args["action_id"] = a.ID
	case store.ActionAccess:
		a := st.CreateAbsence(store.Absence{
			ID: st.NextID("REQ"), Type: store.ActionAccess, Resource: resp.Args["resource"],
			UserID: u.ID, TeamID: u.TeamID, Reason: resp.Args["reason"], Status: store.StatusDraft,
		})
		resp.Args["action_id"] = a.ID
	}
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
