package store

import (
	"fmt"
	"strings"
	"time"
)

// Signal — сообщение стажёра наставнику «что-то не так»: застрял, не работает, непонятно.
// В отличие от пробелов базы знаний сигнал именной: наставнику нужно знать, кому помочь.
// Стажёр сам решает отправить его (кнопка или явная просьба «передай наставнику»).
type Signal struct {
	ID     string    `json:"id"`
	UserID string    `json:"user_id"`
	Text   string    `json:"text"`
	Topic  string    `json:"topic"` // blocked | tech | unclear | other
	At     time.Time `json:"at"`
	Done   bool      `json:"done"`
}

const (
	maxSignalsPerTeam = 200
	maxSignalLen      = 500
)

// AddSignal сохраняет сигнал (с замаскированными персональными данными) и возвращает его.
func (s *Store) AddSignal(teamID, userID, topic, text string) Signal {
	text = strings.TrimSpace(MaskPII(text))
	if r := []rune(text); len(r) > maxSignalLen {
		text = string(r[:maxSignalLen]) + "…"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq["SIG"]++
	sig := Signal{ID: fmt.Sprintf("SIG-%06d", s.seq["SIG"]), UserID: userID, Text: text, Topic: topic, At: s.now()}
	list := append(s.signals[teamID], sig)
	if len(list) > maxSignalsPerTeam {
		list = list[len(list)-maxSignalsPerTeam:]
	}
	s.signals[teamID] = list
	s.teamStatsLocked(teamID).Signals++
	if us := s.userStatsLocked(userID); us.FirstSignalAt == nil {
		at := sig.At
		us.FirstSignalAt, us.FirstSignalTeam = &at, teamID
	}
	s.touch()
	return sig
}

// Signals — сигналы команды, новые сверху; открытые раньше закрытых.
func (s *Store) Signals(teamID string) []Signal {
	s.mu.RLock()
	src := s.signals[teamID]
	out := make([]Signal, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- {
		if !src[i].Done {
			out = append(out, src[i])
		}
	}
	for i := len(src) - 1; i >= 0; i-- {
		if src[i].Done {
			out = append(out, src[i])
		}
	}
	s.mu.RUnlock()
	return out
}

// ResolveSignal отмечает сигнал команды разобранным. false — такого сигнала в команде нет.
func (s *Store) ResolveSignal(teamID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.signals[teamID] {
		if s.signals[teamID][i].ID == id {
			s.signals[teamID][i].Done = true
			s.touch()
			return true
		}
	}
	return false
}
