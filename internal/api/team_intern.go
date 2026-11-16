package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tasks"
)

// internDetail — карточка стажёра для наставника и руководителя своей команды.
type internDetail struct {
	User     auth.User         `json:"user"`
	Progress progress          `json:"progress"`
	Pulse    store.PulseStatus `json:"pulse"`
	Actions  []store.Absence   `json:"actions"`
	Signals  []store.Signal    `json:"signals"`
	Tasks    *internTasks      `json:"tasks,omitempty"`
}

type internTasks struct {
	Summary         tasks.Summary         `json:"summary"`
	Active          []tasks.View          `json:"active"` // в работе и сделанные, новые первыми
	Recommendations tasks.Recommendations `json:"recommendations"`
}

// TeamIntern — GET /team/interns/{id}: прогресс, «Пульс», заявки (без черновиков), обращения и задачи.
// Стажёр другой команды — 404: наставник не узнаёт даже о его существовании.
func (h *Handlers) TeamIntern(w http.ResponseWriter, r *http.Request) {
	u, ok := h.teamUser(w, r)
	if !ok {
		return
	}
	intern, found := auth.ByID(chi.URLParam(r, "id"))
	if !found || intern.Role != auth.RoleIntern || intern.TeamID != u.TeamID {
		writeError(w, http.StatusNotFound, "not_found", "intern not found in your team")
		return
	}
	d := internDetail{
		User:     intern,
		Progress: progressOf(h.Store.Checklist(intern.ID)),
		Pulse:    store.AssessPulse(h.Store.Pulse(intern.ID, 0), h.Store.Today()),
		Actions:  make([]store.Absence, 0),
		Signals:  h.Store.SignalsOf(intern.ID),
	}
	for _, a := range h.Store.ListByUser(intern.ID) {
		if a.Status != store.StatusDraft { // черновики — личное дело стажёра
			d.Actions = append(d.Actions, a)
		}
	}
	if h.Tasks != nil {
		t := &internTasks{
			Summary:         h.Tasks.SummaryFor(intern.ID, intern.TeamID),
			Active:          make([]tasks.View, 0),
			Recommendations: h.Tasks.Recommend(intern.ID, intern.TeamID, 3),
		}
		for _, v := range h.Tasks.History(intern.ID) {
			if v.Status != store.TaskSkipped {
				t.Active = append(t.Active, v)
			}
		}
		d.Tasks = t
	}
	writeJSON(w, http.StatusOK, d)
}

// ListSessions — GET /chat/sessions: история своих диалогов с Бадди, новые первыми.
func (h *Handlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	if h.Chat == nil || h.Chat.Msgs == nil {
		writeJSON(w, http.StatusOK, []store.SessionSummary{})
		return
	}
	writeJSON(w, http.StatusOK, h.Chat.Msgs.SessionsByUser(u.ID))
}
