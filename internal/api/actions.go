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

// CreateAction создаёт черновик действия вручную (из UI). Подтверждение — отдельным запросом.
// Правила проверки общие с чатом и MCP: store.ValidateAction.
func (h *Handlers) CreateAction(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	var body actionReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if utf8.RuneCountInString(body.Reason) > store.MaxReasonRunes || utf8.RuneCountInString(body.Resource) > 50 {
		writeError(w, http.StatusBadRequest, "bad_request", "reason or resource too long")
		return
	}
	a := store.Absence{Type: body.Type, UserID: u.ID, TeamID: u.TeamID, Reason: body.Reason}
	switch body.Type {
	case store.ActionAbsence:
		a.DateFrom, a.DateTo = body.DateFrom, body.DateTo
	case store.ActionAccess:
		a.Resource = store.NormalizeResource(body.Resource)
	}
	if err := store.ValidateAction(a, time.Now()); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	a.ID = h.Store.NextID(idPrefix(a.Type))
	created, err := h.Store.CreateDraft(a)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, "too_many_drafts", "Слишком много неподтверждённых черновиков: подтвердите или отмените старые")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// idPrefix — префикс номера: ABS-… для отгула, REQ-… для доступа.
func idPrefix(actionType string) string {
	if actionType == store.ActionAccess {
		return "REQ"
	}
	return "ABS"
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
	// Повторная проверка: черновик мог устареть (дата прошла) или прийти в обход POST /actions.
	if a.Status == store.StatusDraft {
		if err := store.ValidateAction(a, time.Now()); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "draft is stale or invalid: "+err.Error())
			return
		}
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
