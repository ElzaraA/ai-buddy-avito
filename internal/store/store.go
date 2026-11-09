package store

import (
"sync"
"time"
)

type Absence struct {
ID        string    `json:"id"`
UserID    string    `json:"user_id"`
DateFrom  string    `json:"date_from"`
DateTo    string    `json:"date_to"`
Reason    string    `json:"reason"`
Status    string    `json:"status"`
CreatedAt time.Time `json:"created_at"`
}

type ChecklistItem struct {
ID    string `json:"id"`
Title string `json:"title"`
Stage string `json:"stage"`
Done  bool   `json:"done"`
Owner string `json:"owner"`
}

type Store struct {
mu        sync.RWMutex
absences  map[string]Absence
checklist map[string][]ChecklistItem
}

func New() *Store {
return &Store{
absences:  map[string]Absence{},
checklist: map[string][]ChecklistItem{},
}
}

func (s *Store) CreateAbsence(a Absence) Absence {
s.mu.Lock()
defer s.mu.Unlock()
s.absences[a.ID] = a
return a
}

func (s *Store) GetAbsence(id string) (Absence, bool) {
s.mu.RLock()
defer s.mu.RUnlock()
a, ok := s.absences[id]
return a, ok
}

func (s *Store) SeedChecklist(userID string, items []ChecklistItem) {
s.mu.Lock()
defer s.mu.Unlock()
s.checklist[userID] = items
}

func (s *Store) Checklist(userID string) []ChecklistItem {
s.mu.RLock()
defer s.mu.RUnlock()
return s.checklist[userID]
}

func (s *Store) MarkDone(userID, itemID string) bool {
s.mu.Lock()
defer s.mu.Unlock()
items := s.checklist[userID]
for i := range items {
if items[i].ID == itemID {
items[i].Done = true
s.checklist[userID] = items
return true
}
}
return false
}
