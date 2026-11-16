// Package tasks — задачи адаптации стажёра: каталог команды, статусы («в работе», «сделано»,
// «отложено») и рекомендации следующей задачи моделью из internal/recsys.
package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/recsys"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// MaxInProgress — сколько задач стажёр держит в работе одновременно (больше — распыление).
const MaxInProgress = 3

// Task — задача каталога.
type Task struct {
	ID            string   `json:"id"`
	Team          string   `json:"team"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Steps         []string `json:"steps"`
	Done          string   `json:"definition_of_done"`
	Difficulty    int      `json:"difficulty"`
	Tags          []string `json:"tags"`
	Popularity    int      `json:"popularity"`
	EstimateHours int      `json:"estimate_hours"`
	Knowledge     string   `json:"knowledge,omitempty"` // файл базы знаний, например tools.md
}

type rawTask struct {
	Task
	DoneText string `json:"done"`
}

// Catalog — задачи всех команд и стартовые компетенции стажёров.
type Catalog struct {
	tasks    []Task
	byID     map[string]Task
	profiles map[string]map[string]int
}

// LoadCatalog читает data/tasks/catalog.json и проверяет его.
func LoadCatalog(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Tasks    []rawTask                 `json:"tasks"`
		Profiles map[string]map[string]int `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("tasks: %s: %w", path, err)
	}
	c := &Catalog{byID: map[string]Task{}, profiles: f.Profiles}
	for _, r := range f.Tasks {
		t := r.Task
		t.Done = r.DoneText
		switch {
		case t.ID == "" || t.Team == "" || t.Title == "":
			return nil, fmt.Errorf("tasks: task without id/team/title: %+v", t)
		case t.Difficulty < 1 || t.Difficulty > 5:
			return nil, fmt.Errorf("tasks: %s: difficulty must be 1..5", t.ID)
		case len(t.Tags) == 0:
			return nil, fmt.Errorf("tasks: %s: no tags", t.ID)
		}
		if _, dup := c.byID[t.ID]; dup {
			return nil, fmt.Errorf("tasks: duplicate id %s", t.ID)
		}
		c.byID[t.ID] = t
		c.tasks = append(c.tasks, t)
	}
	if c.profiles == nil {
		c.profiles = map[string]map[string]int{}
	}
	return c, nil
}

// Ошибки сервиса; API превращает их в 403/404/409.
var (
	ErrNotFound  = errors.New("task not found")
	ErrForbidden = errors.New("only interns take tasks")
	ErrLimit     = fmt.Errorf("в работе уже %d задачи — завершите или отложите одну", MaxInProgress)
)

// ConflictError — недопустимый переход статуса.
type ConflictError struct{ Status string }

func (e ConflictError) Error() string {
	return "нельзя изменить задачу в статусе " + statusName(e.Status)
}

func statusName(s string) string {
	switch s {
	case store.TaskInProgress:
		return "«в работе»"
	case store.TaskDone:
		return "«сделано»"
	case store.TaskSkipped:
		return "«отложена»"
	}
	return "«не начата»"
}

// View — задача глазами конкретного стажёра.
type View struct {
	Task
	Status    string     `json:"status"` // todo | in_progress | done | skipped
	StartedAt *time.Time `json:"started_at,omitempty"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

// Recommendation — рекомендованная задача с оценкой модели и объяснением.
type Recommendation struct {
	View
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}

// Recommendations — ответ рекомендаций.
type Recommendations struct {
	Source string           `json:"source"` // model | rule_based
	Model  string           `json:"model,omitempty"`
	Items  []Recommendation `json:"items"`
}

// Service — задачи и рекомендации.
type Service struct {
	Catalog *Catalog
	Model   *recsys.Model // nil — правило вместо модели
	Store   *store.Store
}

func (s *Service) view(t Task, st store.TaskState) View {
	v := View{Task: t, Status: "todo"}
	if st.Status != "" {
		v.Status = st.Status
	}
	if !st.StartedAt.IsZero() {
		at := st.StartedAt
		v.StartedAt = &at
	}
	if !st.DoneAt.IsZero() {
		at := st.DoneAt
		v.DoneAt = &at
	}
	return v
}

// List — задачи команды пользователя; для стажёра — с его статусами.
func (s *Service) List(u auth.User) []View {
	return s.ListFor(u.ID, u.TeamID)
}

// ListFor — задачи команды teamID со статусами пользователя userID.
func (s *Service) ListFor(userID, teamID string) []View {
	states := s.Store.TaskStates(userID)
	out := make([]View, 0)
	for _, t := range s.Catalog.tasks {
		if t.Team == teamID {
			out = append(out, s.view(t, states[t.ID]))
		}
	}
	return out
}

// Get — задача своей команды (чужая — как несуществующая).
func (s *Service) Get(u auth.User, id string) (View, error) {
	t, ok := s.Catalog.byID[id]
	if !ok || t.Team != u.TeamID {
		return View{}, ErrNotFound
	}
	return s.view(t, s.Store.TaskStates(u.ID)[id]), nil
}

// Profile — компетенции стажёра: стартовые плюс +1 за каждую технологию сделанной задачи (до 5).
func (s *Service) Profile(userID, teamID string) recsys.Profile {
	comp := map[string]int{}
	for k, v := range s.Catalog.profiles[userID] {
		comp[k] = v
	}
	for id, st := range s.Store.TaskStates(userID) {
		if st.Status != store.TaskDone {
			continue
		}
		for _, tag := range s.Catalog.byID[id].Tags {
			comp[tag] = min(5, comp[tag]+1)
		}
	}
	return recsys.Profile{Track: teamID, Competencies: comp}
}

// Recommend — k лучших ещё не взятых задач команды для стажёра.
func (s *Service) Recommend(userID, teamID string, k int) Recommendations {
	states := s.Store.TaskStates(userID)
	prior := recsys.Prior{}
	var cands []recsys.Candidate
	for _, t := range s.Catalog.tasks {
		st := states[t.ID].Status
		switch st {
		case store.TaskDone:
			prior.Pos++
		case store.TaskSkipped:
			prior.Neg++
		}
		if t.Team != teamID || st != "" {
			continue
		}
		cands = append(cands, recsys.Candidate{ID: t.ID, Track: t.Team, Difficulty: t.Difficulty, Tags: t.Tags, Popularity: t.Popularity})
	}
	res := recsys.Rank(s.Model, s.Profile(userID, teamID), cands, prior, k)
	out := Recommendations{Source: res.Source, Model: res.Model, Items: make([]Recommendation, 0, len(res.Items))}
	for _, it := range res.Items {
		out.Items = append(out.Items, Recommendation{View: s.view(s.Catalog.byID[it.ID], store.TaskState{}), Score: it.Score, Reasons: it.Reasons})
	}
	return out
}

// Start / Complete / Skip / Reopen — смена статуса стажёром.
func (s *Service) Start(u auth.User, id string) (View, error) {
	return s.move(u, id, store.TaskInProgress, "", store.TaskSkipped)
}

func (s *Service) Complete(u auth.User, id string) (View, error) {
	return s.move(u, id, store.TaskDone, store.TaskInProgress)
}

func (s *Service) Skip(u auth.User, id string) (View, error) {
	return s.move(u, id, store.TaskSkipped, "", store.TaskInProgress)
}

// Reopen возвращает отложенную задачу в «не начата».
func (s *Service) Reopen(u auth.User, id string) (View, error) {
	return s.move(u, id, "", store.TaskSkipped)
}

func (s *Service) move(u auth.User, id, to string, from ...string) (View, error) {
	if u.Role != auth.RoleIntern {
		return View{}, ErrForbidden
	}
	t, err := s.Get(u, id)
	if err != nil {
		return View{}, err
	}
	if to == store.TaskInProgress && t.Status != store.TaskInProgress && s.Store.CountTasks(u.ID, store.TaskInProgress) >= MaxInProgress {
		return View{}, ErrLimit
	}
	st, ok := s.Store.MoveTask(u.ID, id, to, from...)
	if !ok {
		return View{}, ConflictError{Status: st.Status}
	}
	return s.view(t.Task, st), nil
}

// Summary — сводка по задачам стажёра (для профиля и кабинета наставника).
type Summary struct {
	Total      int `json:"total"`
	InProgress int `json:"in_progress"`
	Done       int `json:"done"`
	Skipped    int `json:"skipped"`
}

// SummaryFor — сводка задач стажёра в его команде.
func (s *Service) SummaryFor(userID, teamID string) Summary {
	var sum Summary
	for _, v := range s.ListFor(userID, teamID) {
		sum.Total++
		switch v.Status {
		case store.TaskInProgress:
			sum.InProgress++
		case store.TaskDone:
			sum.Done++
		case store.TaskSkipped:
			sum.Skipped++
		}
	}
	return sum
}

// History — последние изменения задач стажёра, новые первыми.
func (s *Service) History(userID string) []View {
	var out []View
	for _, st := range s.Store.TaskHistory(userID) {
		if t, ok := s.Catalog.byID[st.TaskID]; ok {
			out = append(out, s.view(t, st))
		}
	}
	return out
}
