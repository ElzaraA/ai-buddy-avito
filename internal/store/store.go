package store

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Типы действий, которые агент готовит как черновик и пользователь подтверждает.
const (
	ActionAbsence = "absence"
	ActionAccess  = "access"
)

// Статусы действия: draft → pending → approved | rejected; draft | pending → cancelled.
const (
	StatusDraft     = "draft"
	StatusPending   = "pending"
	StatusApproved  = "approved"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"
)

// Статусы шага чек-листа.
const (
	StepTodo    = "todo"
	StepWaiting = "waiting" // ждёт внешнего действия (например, выдачи доступа)
	StepDone    = "done"
)

// Event — запись истории действия: кто и когда перевёл его в статус.
type Event struct {
	At      time.Time `json:"at"`
	By      string    `json:"by"`
	Status  string    `json:"status"`
	Comment string    `json:"comment,omitempty"`
}

// Absence — запись действия пользователя (исторически только отгул; теперь также запрос доступа).
// Имя типа сохранено ради обратной совместимости JSON и тестов.
type Absence struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`               // absence | access
	Resource  string    `json:"resource,omitempty"` // для access: jira, gitlab, grafana…
	UserID    string    `json:"user_id"`
	TeamID    string    `json:"team_id,omitempty"`
	DateFrom  string    `json:"date_from"`
	DateTo    string    `json:"date_to"`
	Reason    string    `json:"reason"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	History   []Event   `json:"history,omitempty"`
}

type ChecklistItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Stage    string `json:"stage"`
	Done     bool   `json:"done"`
	Status   string `json:"status"`                      // todo | waiting | done
	Kind     string `json:"kind,omitempty"`              // access — шаг связан с заявками на доступ
	ActionID string `json:"related_action_id,omitempty"` // заявка, которой ждёт шаг
	Owner    string `json:"owner"`
}

func (it *ChecklistItem) normalize() {
	switch {
	case it.Done:
		it.Status = StepDone
	case it.Status == "" || it.Status == StepDone:
		it.Status = StepTodo
	}
}

type Store struct {
	mu        sync.RWMutex
	absences  map[string]Absence
	checklist map[string][]ChecklistItem
	seq       map[string]int64
	gaps      map[string]map[string]*Gap // team → нормализованный вопрос → счётчик
	version   atomic.Int64               // растёт при каждом изменении (для сохранения на диск)
	now       func() time.Time
}

func New() *Store {
	return &Store{
		absences:  map[string]Absence{},
		checklist: map[string][]ChecklistItem{},
		seq:       map[string]int64{},
		gaps:      map[string]map[string]*Gap{},
		now:       time.Now,
	}
}

// Version — счётчик изменений; по нему Persister понимает, что пора сохранять.
func (s *Store) Version() int64 { return s.version.Load() }

func (s *Store) touch() { s.version.Add(1) }

// NextID возвращает следующий ID с префиксом, например ABS-000001 или REQ-000001.
func (s *Store) NextID(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq[prefix]++
	s.touch()
	return fmt.Sprintf("%s-%06d", prefix, s.seq[prefix])
}

// NextAbsenceID оставлен для совместимости.
func (s *Store) NextAbsenceID() string { return s.NextID("ABS") }

func (s *Store) CreateAbsence(a Absence) Absence {
	if a.Type == "" {
		a.Type = ActionAbsence
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = s.now()
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = a.CreatedAt
	}
	if len(a.History) == 0 && a.Status != "" {
		a.History = []Event{{At: a.CreatedAt, By: a.UserID, Status: a.Status}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.absences[a.ID] = a
	s.touch()
	return cloneAction(a)
}

func cloneAction(a Absence) Absence {
	if a.History != nil {
		a.History = append([]Event(nil), a.History...)
	}
	return a
}

func (s *Store) GetAbsence(id string) (Absence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.absences[id]
	return cloneAction(a), ok
}

func sortActions(out []Absence) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
}

// ListByUser возвращает действия пользователя, новые сверху.
func (s *Store) ListByUser(userID string) []Absence {
	s.mu.RLock()
	out := make([]Absence, 0)
	for _, a := range s.absences {
		if a.UserID == userID {
			out = append(out, cloneAction(a))
		}
	}
	s.mu.RUnlock()
	sortActions(out)
	return out
}

// ListByTeam — действия команды в заданном статусе (пустой статус — все, кроме черновиков).
func (s *Store) ListByTeam(teamID, status string) []Absence {
	s.mu.RLock()
	out := make([]Absence, 0)
	for _, a := range s.absences {
		if a.TeamID != teamID {
			continue
		}
		if (status == "" && a.Status != StatusDraft) || a.Status == status {
			out = append(out, cloneAction(a))
		}
	}
	s.mu.RUnlock()
	sortActions(out)
	return out
}

// Transition атомарно переводит действие из статуса from в to (защита от двойного подтверждения).
func (s *Store) Transition(id, from, to string) (Absence, bool) {
	return s.TransitionBy(id, []string{from}, to, "", "")
}

// TransitionBy — атомарный переход из любого из статусов from в to с записью в историю.
func (s *Store) TransitionBy(id string, from []string, to, by, comment string) (Absence, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.absences[id]
	if !ok {
		return Absence{}, false
	}
	allowed := false
	for _, f := range from {
		if a.Status == f {
			allowed = true
			break
		}
	}
	if !allowed {
		return Absence{}, false
	}
	if by == "" {
		by = a.UserID
	}
	a.Status = to
	a.UpdatedAt = s.now()
	a.History = append(append([]Event(nil), a.History...), Event{At: a.UpdatedAt, By: by, Status: to, Comment: comment})
	s.absences[id] = a
	s.touch()
	return cloneAction(a), true
}

func (s *Store) SeedChecklist(userID string, items []ChecklistItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]ChecklistItem, len(items))
	copy(cp, items)
	for i := range cp {
		cp[i].Owner = userID
		cp[i].normalize()
	}
	s.checklist[userID] = cp
	s.touch()
}

// HasChecklist — есть ли у пользователя чек-лист (чтобы не затирать загруженное с диска сидом).
func (s *Store) HasChecklist(userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.checklist[userID]) > 0
}

// Checklist возвращает копию, чтобы вызывающий код не читал слайс без блокировки.
func (s *Store) Checklist(userID string) []ChecklistItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.checklist[userID]
	out := make([]ChecklistItem, len(items))
	copy(out, items)
	return out
}

func (s *Store) MarkDoneOwned(userID, itemID string) (found, owned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uid, items := range s.checklist {
		for i := range items {
			if items[i].ID == itemID {
				if uid != userID {
					return true, false
				}
				items[i].Done = true
				items[i].Status = StepDone
				s.touch()
				return true, true
			}
		}
	}
	return false, false
}

func (s *Store) MarkDone(userID, itemID string) bool {
	_, owned := s.MarkDoneOwned(userID, itemID)
	return owned
}

// LinkStep связывает первый незавершённый шаг пользователя вида kind с заявкой и ставит ему статус.
// Так шаг «Получить доступы» переходит в waiting после отправки заявки и в done после её одобрения.
func (s *Store) LinkStep(userID, kind, actionID, status string) (ChecklistItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.checklist[userID]
	for i := range items {
		if items[i].Kind != kind || items[i].Done {
			continue
		}
		items[i].ActionID = actionID
		items[i].Status = status
		items[i].Done = status == StepDone
		s.touch()
		return items[i], true
	}
	return ChecklistItem{}, false
}

// UnlinkStep возвращает шаг, ждавший заявку actionID, в todo (заявку отклонили или отменили).
func (s *Store) UnlinkStep(userID, actionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.checklist[userID]
	for i := range items {
		if items[i].ActionID == actionID && !items[i].Done {
			items[i].ActionID = ""
			items[i].Status = StepTodo
			s.touch()
		}
	}
}
