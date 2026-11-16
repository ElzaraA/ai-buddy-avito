package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/tasks"
)

const (
	defaultRecommendations = 3
	maxRecommendations     = 10
)

func (h *Handlers) tasksService(w http.ResponseWriter) (*tasks.Service, bool) {
	if h.Tasks == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "task catalog is not loaded")
		return nil, false
	}
	return h.Tasks, true
}

// ListTasks — задачи адаптации своей команды; у стажёра — с его статусами (?status= фильтр).
func (h *Handlers) ListTasks(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.tasksService(w)
	if !ok {
		return
	}
	u, _ := auth.FromCtx(r.Context())
	status := r.URL.Query().Get("status")
	switch status {
	case "", "todo", "in_progress", "done", "skipped":
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "status must be one of: todo, in_progress, done, skipped")
		return
	}
	out := make([]tasks.View, 0)
	for _, v := range svc.List(u) {
		if status == "" || v.Status == status {
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GetTask — задача своей команды; чужая — 404.
func (h *Handlers) GetTask(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.tasksService(w)
	if !ok {
		return
	}
	u, _ := auth.FromCtx(r.Context())
	v, err := svc.Get(u, chi.URLParam(r, "id"))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// MoveTask — POST /tasks/{id}/{start|complete|skip|reopen}.
func (h *Handlers) MoveTask(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.tasksService(w)
	if !ok {
		return
	}
	u, _ := auth.FromCtx(r.Context())
	id := chi.URLParam(r, "id")
	var (
		v   tasks.View
		err error
	)
	switch chi.URLParam(r, "op") {
	case "start":
		v, err = svc.Start(u, id)
	case "complete":
		v, err = svc.Complete(u, id)
	case "skip":
		v, err = svc.Skip(u, id)
	case "reopen":
		v, err = svc.Reopen(u, id)
	default:
		writeError(w, http.StatusNotFound, "not_found", "unknown task operation")
		return
	}
	if err != nil {
		writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// Recommendations — рекомендованные следующие задачи стажёра (?limit=1..10, по умолчанию 3).
func (h *Handlers) Recommendations(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.tasksService(w)
	if !ok {
		return
	}
	u, _ := auth.FromCtx(r.Context())
	if u.Role != auth.RoleIntern {
		writeError(w, http.StatusForbidden, "forbidden", "recommendations are for interns; mentors see them in the intern profile")
		return
	}
	k := defaultRecommendations
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxRecommendations {
			writeError(w, http.StatusBadRequest, "bad_request", "limit must be 1..10")
			return
		}
		k = n
	}
	writeJSON(w, http.StatusOK, svc.Recommend(u.ID, u.TeamID, k))
}

func writeTaskError(w http.ResponseWriter, err error) {
	var conflict tasks.ConflictError
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "task not found")
	case errors.Is(err, tasks.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "only interns take tasks")
	case errors.Is(err, tasks.ErrLimit):
		writeError(w, http.StatusConflict, "too_many_in_progress", err.Error())
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "task update failed")
	}
}
