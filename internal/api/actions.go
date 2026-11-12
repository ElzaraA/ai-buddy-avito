package api

import (
	"encoding/json"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/go-chi/chi/v5"
)

type actionReq struct {
	Type     string `json:"type"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Reason   string `json:"reason"`
	Resource string `json:"resource"`
}

// validAbsenceDates: формат YYYY-MM-DD, конец не раньше начала, начало не в прошлом, не дольше 30 дней.
func validAbsenceDates(from, to string, now time.Time) (string, bool) {
	f, err1 := time.Parse("2006-01-02", from)
	t, err2 := time.Parse("2006-01-02", to)
	switch {
	case err1 != nil || err2 != nil:
		return "date_from/date_to must be YYYY-MM-DD", false
	case t.Before(f):
		return "date_to must be >= date_from", false
	case from < now.Format("2006-01-02"):
		return "date_from is in the past", false
	case t.Sub(f) > 30*24*time.Hour:
		return "absence longer than 30 days is not supported", false
	}
	return "", true
}

// CreateAction создаёт черновик действия вручную (из UI). Подтверждение — отдельным запросом.
func (h *Handlers) CreateAction(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	var body actionReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if utf8.RuneCountInString(body.Reason) > 500 || utf8.RuneCountInString(body.Resource) > 50 {
		writeError(w, http.StatusBadRequest, "bad_request", "reason or resource too long")
		return
	}
	switch body.Type {
	case store.ActionAbsence:
		if msg, ok := validAbsenceDates(body.DateFrom, body.DateTo, time.Now()); !ok {
			writeError(w, http.StatusBadRequest, "bad_request", msg)
			return
		}
		a := h.Store.CreateAbsence(store.Absence{
			ID: h.Store.NextID("ABS"), Type: store.ActionAbsence, UserID: u.ID, TeamID: u.TeamID,
			DateFrom: body.DateFrom, DateTo: body.DateTo, Reason: body.Reason, Status: store.StatusDraft,
		})
		writeJSON(w, http.StatusCreated, a)
	case store.ActionAccess:
		if body.Resource == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "resource required")
			return
		}
		a := h.Store.CreateAbsence(store.Absence{
			ID: h.Store.NextID("REQ"), Type: store.ActionAccess, Resource: body.Resource,
			UserID: u.ID, TeamID: u.TeamID, Reason: body.Reason, Status: store.StatusDraft,
		})
		writeJSON(w, http.StatusCreated, a)
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "type must be absence or access")
	}
}

func (h *Handlers) ListActions(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	writeJSON(w, http.StatusOK, h.Store.ListByUser(u.ID))
}

// sameTeamReviewer — наставник или руководитель той же команды (видит отправленные заявки, не черновики).
func sameTeamReviewer(u auth.User, a store.Absence) bool {
	return auth.Can(u, "team.read") && a.TeamID != "" && a.TeamID == u.TeamID && a.Status != store.StatusDraft
}

// loadAction возвращает действие или пишет 404.
func (h *Handlers) loadAction(w http.ResponseWriter, r *http.Request) (store.Absence, auth.User, bool) {
	u, _ := auth.FromCtx(r.Context())
	a, ok := h.Store.GetAbsence(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "action not found")
		return store.Absence{}, u, false
	}
	return a, u, true
}

// ownAction возвращает действие текущего пользователя или пишет 404/403.
func (h *Handlers) ownAction(w http.ResponseWriter, r *http.Request) (store.Absence, auth.User, bool) {
	a, u, ok := h.loadAction(w, r)
	if !ok {
		return a, u, false
	}
	if a.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "not your action")
		return store.Absence{}, u, false
	}
	return a, u, true
}

func (h *Handlers) GetAction(w http.ResponseWriter, r *http.Request) {
	a, u, ok := h.loadAction(w, r)
	if !ok {
		return
	}
	if a.UserID != u.ID && !sameTeamReviewer(u, a) {
		writeError(w, http.StatusForbidden, "forbidden", "not your action")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// finish записывает переход в метрики, синхронизирует чек-лист и отвечает клиенту.
func (h *Handlers) finish(w http.ResponseWriter, a store.Absence) {
	syncStep(h.Store, a)
	h.Metrics.Inc("buddy_actions_total", "Действия пользователей по типу и статусу", "type", a.Type, "status", a.Status)
	writeJSON(w, http.StatusOK, a)
}

// ConfirmAction: draft → pending. Только владелец и только при наличии права роли.
func (h *Handlers) ConfirmAction(w http.ResponseWriter, r *http.Request) {
	a, u, ok := h.ownAction(w, r)
	if !ok {
		return
	}
	if !auth.Can(u, permissionFor(a.Type)) {
		writeError(w, http.StatusForbidden, "forbidden", "your role cannot confirm this action")
		return
	}
	upd, ok := h.Store.TransitionBy(a.ID, []string{store.StatusDraft}, store.StatusPending, u.ID, "")
	if !ok {
		writeError(w, http.StatusConflict, "conflict", "already confirmed")
		return
	}
	h.finish(w, upd)
}

// CancelAction: владелец отменяет черновик или отправленную заявку.
func (h *Handlers) CancelAction(w http.ResponseWriter, r *http.Request) {
	a, u, ok := h.ownAction(w, r)
	if !ok {
		return
	}
	upd, ok := h.Store.TransitionBy(a.ID, []string{store.StatusDraft, store.StatusPending}, store.StatusCancelled, u.ID, "")
	if !ok {
		writeError(w, http.StatusConflict, "conflict", "action is already decided")
		return
	}
	h.finish(w, upd)
}

type decisionReq struct {
	Comment string `json:"comment"`
}

// decide — решение руководителя по заявке своей команды: pending → approved | rejected.
func (h *Handlers) decide(w http.ResponseWriter, r *http.Request, status string) {
	a, u, ok := h.loadAction(w, r)
	if !ok {
		return
	}
	if !auth.Can(u, "action.approve") || a.TeamID == "" || a.TeamID != u.TeamID {
		writeError(w, http.StatusForbidden, "forbidden", "only the team lead can decide on this action")
		return
	}
	if a.UserID == u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "cannot decide on your own action")
		return
	}
	var body decisionReq
	if err := decodeOptional(r, &body); err != nil || utf8.RuneCountInString(body.Comment) > 500 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json or comment too long")
		return
	}
	upd, ok := h.Store.TransitionBy(a.ID, []string{store.StatusPending}, status, u.ID, body.Comment)
	if !ok {
		writeError(w, http.StatusConflict, "conflict", "action is not pending")
		return
	}
	h.finish(w, upd)
}

func (h *Handlers) ApproveAction(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, store.StatusApproved)
}

func (h *Handlers) RejectAction(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, store.StatusRejected)
}
