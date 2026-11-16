package store

import (
	"sort"
	"time"
)

// Статусы задачи адаптации у стажёра. Отсутствие записи — задача ещё не взята.
const (
	TaskInProgress = "in_progress"
	TaskDone       = "done"
	TaskSkipped    = "skipped"
)

// TaskState — что стажёр сделал с задачей каталога.
type TaskState struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
	DoneAt    time.Time `json:"done_at,omitempty"`
}

// TaskStates — состояния задач пользователя (копия).
func (s *Store) TaskStates(userID string) map[string]TaskState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]TaskState, len(s.tasks[userID]))
	for k, v := range s.tasks[userID] {
		out[k] = v
	}
	return out
}

// TaskHistory — состояния задач по времени изменения, новые первыми.
func (s *Store) TaskHistory(userID string) []TaskState {
	m := s.TaskStates(userID)
	out := make([]TaskState, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].TaskID < out[j].TaskID
	})
	return out
}

// MoveTask меняет статус задачи, только если текущий статус входит в from ("" — задача не взята).
// Возвращает новое состояние и ok=false, если переход недопустим (текущее состояние — в cur).
func (s *Store) MoveTask(userID, taskID, to string, from ...string) (st TaskState, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.tasks[userID][taskID]
	allowed := false
	for _, f := range from {
		allowed = allowed || cur.Status == f
	}
	if !allowed {
		return cur, false
	}
	now := s.now()
	next := TaskState{TaskID: taskID, Status: to, UpdatedAt: now, StartedAt: cur.StartedAt}
	switch to {
	case TaskInProgress:
		next.StartedAt = now
	case TaskDone:
		next.DoneAt = now
		if next.StartedAt.IsZero() {
			next.StartedAt = now
		}
	}
	if s.tasks[userID] == nil {
		s.tasks[userID] = map[string]TaskState{}
	}
	if to == "" { // вернуть в «не взята»
		delete(s.tasks[userID], taskID)
		s.touch()
		return TaskState{TaskID: taskID}, true
	}
	s.tasks[userID][taskID] = next
	s.touch()
	return next, true
}

// CountTasks — сколько задач пользователя в статусе status.
func (s *Store) CountTasks(userID, status string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, v := range s.tasks[userID] {
		if v.Status == status {
			n++
		}
	}
	return n
}
