package api

import (
	"net/http"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// Кабинет наставника и руководителя: всё ограничено своей командой и правом team.read.

func (h *Handlers) teamUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	u, _ := auth.FromCtx(r.Context())
	if !auth.Can(u, "team.read") {
		writeError(w, http.StatusForbidden, "forbidden", "only mentors and leads can see team data")
		return u, false
	}
	return u, true
}

type internProgress struct {
	User auth.User `json:"user"`
	progress
}

// TeamProgress — прогресс адаптации стажёров команды.
func (h *Handlers) TeamProgress(w http.ResponseWriter, r *http.Request) {
	u, ok := h.teamUser(w, r)
	if !ok {
		return
	}
	out := make([]internProgress, 0)
	for _, intern := range auth.TeamInterns(u.TeamID) {
		out = append(out, internProgress{User: intern, progress: progressOf(h.Store.Checklist(intern.ID))})
	}
	writeJSON(w, http.StatusOK, out)
}

// TeamActions — заявки команды (по умолчанию все отправленные; ?status=pending — входящие на согласование).
func (h *Handlers) TeamActions(w http.ResponseWriter, r *http.Request) {
	u, ok := h.teamUser(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status == store.StatusDraft {
		writeError(w, http.StatusForbidden, "forbidden", "drafts are private")
		return
	}
	writeJSON(w, http.StatusOK, h.Store.ListByTeam(u.TeamID, status))
}

// TeamKnowledgeGaps — вопросы стажёров, на которые в базе знаний не нашлось ответа (обезличенно).
func (h *Handlers) TeamKnowledgeGaps(w http.ResponseWriter, r *http.Request) {
	u, ok := h.teamUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.Store.Gaps(u.TeamID))
}

type teamSummary struct {
	Interns            int            `json:"interns"`
	AvgPercentDone     int            `json:"avg_percent_done"`
	StepsWaiting       int            `json:"steps_waiting"`
	Actions            map[string]int `json:"actions"`
	UnansweredTopics   int            `json:"unanswered_topics"`
	UnansweredRequests int            `json:"unanswered_requests"`
}

// TeamSummary — сводка «метрик пользы» по команде для руководителя.
func (h *Handlers) TeamSummary(w http.ResponseWriter, r *http.Request) {
	u, ok := h.teamUser(w, r)
	if !ok {
		return
	}
	s := teamSummary{Actions: map[string]int{}}
	total := 0
	for _, intern := range auth.TeamInterns(u.TeamID) {
		p := progressOf(h.Store.Checklist(intern.ID))
		s.Interns++
		total += p.PercentDone
		for _, it := range p.Items {
			if it.Status == store.StepWaiting {
				s.StepsWaiting++
			}
		}
	}
	if s.Interns > 0 {
		s.AvgPercentDone = total / s.Interns
	}
	for _, a := range h.Store.ListByTeam(u.TeamID, "") {
		s.Actions[a.Status]++
	}
	for _, g := range h.Store.Gaps(u.TeamID) {
		s.UnansweredTopics++
		s.UnansweredRequests += g.Count
	}
	writeJSON(w, http.StatusOK, s)
}
