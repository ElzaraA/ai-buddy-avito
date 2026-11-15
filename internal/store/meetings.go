package store

import (
	"sort"
	"time"
)

// Meeting — личный созвон, который пользователь добавил в своё расписание
// (встречи команды повторяются по правилам и в хранилище не лежат — см. пакет schedule).
type Meeting struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	Start       time.Time `json:"start"`
	DurationMin int       `json:"duration_min"`
	With        string    `json:"with,omitempty"` // с кем созвон, свободным текстом
	Cancelled   bool      `json:"cancelled,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateMeeting сохраняет личный созвон.
func (s *Store) CreateMeeting(m Meeting) Meeting {
	m.ID = s.NextID("MTG")
	s.mu.Lock()
	defer s.mu.Unlock()
	m.CreatedAt = s.now()
	s.meetings[m.ID] = m
	s.touch()
	return m
}

// MeetingsByOwner — неотменённые созвоны пользователя, по времени начала.
func (s *Store) MeetingsByOwner(ownerID string) []Meeting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Meeting, 0)
	for _, m := range s.meetings {
		if m.OwnerID == ownerID && !m.Cancelled {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// CountUpcomingMeetings — сколько будущих созвонов у пользователя (для лимита на создание).
func (s *Store) CountUpcomingMeetings(ownerID string, now time.Time) int {
	n := 0
	for _, m := range s.MeetingsByOwner(ownerID) {
		if m.Start.After(now) {
			n++
		}
	}
	return n
}

// CancelMeeting отменяет созвон владельца. Возвращает (найден, владелец ли).
// Чужой созвон для постороннего «не найден» — так нельзя узнать, существует ли чужой id.
func (s *Store) CancelMeeting(id, ownerID string) (found, owned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.meetings[id]
	if !ok || m.Cancelled || m.OwnerID != ownerID {
		return false, false
	}
	m.Cancelled = true
	s.meetings[id] = m
	s.touch()
	return true, true
}

// AckReminder помечает напоминание о встрече как показанное пользователю, чтобы оно не повторялось.
// Отметки старше двух суток удаляются, чтобы состояние не росло бесконечно.
func (s *Store) AckReminder(userID, occurrenceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	acks := s.reminderAcks[userID]
	if acks == nil {
		acks = map[string]time.Time{}
		s.reminderAcks[userID] = acks
	}
	cutoff := now.Add(-48 * time.Hour)
	for id, at := range acks {
		if at.Before(cutoff) {
			delete(acks, id)
		}
	}
	acks[occurrenceID] = now
	s.touch()
}

// ReminderAcked — показывали ли уже напоминание об этой встрече.
func (s *Store) ReminderAcked(userID, occurrenceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.reminderAcks[userID][occurrenceID]
	return ok
}
