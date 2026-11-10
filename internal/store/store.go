package store

import (
"fmt"
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
seq       int64
}

func New() *Store {
return &Store{
absences:  map[string]Absence{},
checklist: map[string][]ChecklistItem{},
}
}

func (s *Store) NextAbsenceID() string {
s.mu.Lock()
defer s.mu.Unlock()
s.seq++
return fmt.Sprintf("ABS-%06d", s.seq)
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
for i := range items {
items[i].Owner = userID
}
s.checklist[userID] = items
}

func (s *Store) Checklist(userID string) []ChecklistItem {
s.mu.RLock()
defer s.mu.RUnlock()
return s.checklist[userID]
}

func (s *Store) MarkDoneOwned(userID, itemID string) (found, owned bool) {
s.mu.Lock()
defer s.mu.Unlock()
for uid, items := range s.checklist {
for i := range items {
if items[i].ID == itemID {
found = true
if uid != userID {
return true, false
}
items[i].Done = true
s.checklist[uid] = items
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