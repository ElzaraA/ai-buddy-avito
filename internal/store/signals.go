package store

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Signal — сообщение стажёра наставнику «что-то не так»: застрял, не работает, непонятно.
// В отличие от пробелов базы знаний сигнал именной: наставнику нужно знать, кому помочь.
// Стажёр сам решает отправить его (кнопка или явная просьба «передай наставнику»).
// Петля двусторонняя: наставник может ответить при разборе (reply), стажёр видит ответ в GET /signals/me.
type Signal struct {
	ID     string    `json:"id"`
	UserID string    `json:"user_id"`
	Text   string    `json:"text"`
	Topic  string    `json:"topic"` // blocked | tech | unclear | other
	At     time.Time `json:"at"`
	Done   bool      `json:"done"`
	// Reply — ответ наставника (персональные данные маскируются); ResolvedBy — имя того, кто разобрал.
	Reply      string     `json:"reply,omitempty"`
	ResolvedBy string     `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

const (
	maxSignalsPerTeam = 200
	maxSignalLen      = 500
	// MaxSignalReplyLen — максимальная длина ответа наставника.
	MaxSignalReplyLen = 500
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
	_, ok := s.ResolveSignalWithReply(teamID, id, "", "")
	return ok
}

// ResolveSignalWithReply отмечает сигнал разобранным, сохраняет ответ наставника (PII маскируются,
// не длиннее MaxSignalReplyLen), кто и когда разобрал. Пустой ответ не затирает прежний.
func (s *Store) ResolveSignalWithReply(teamID, id, by, reply string) (Signal, bool) {
	reply = strings.TrimSpace(MaskPII(reply))
	if r := []rune(reply); len(r) > MaxSignalReplyLen {
		reply = string(r[:MaxSignalReplyLen])
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.signals[teamID] {
		sig := &s.signals[teamID][i]
		if sig.ID != id {
			continue
		}
		sig.Done = true
		if reply != "" {
			sig.Reply = reply
		}
		if by != "" {
			sig.ResolvedBy = by
		}
		at := s.now()
		sig.ResolvedAt = &at
		s.touch()
		return *sig, true
	}
	return Signal{}, false
}

// SignalsOf — сигналы пользователя (по всем командам), новые сверху.
func (s *Store) SignalsOf(userID string) []Signal {
	s.mu.RLock()
	out := make([]Signal, 0)
	for _, list := range s.signals {
		for _, sig := range list {
			if sig.UserID == userID {
				out = append(out, sig)
			}
		}
	}
	s.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return out[i].ID > out[j].ID
		}
		return out[i].At.After(out[j].At)
	})
	return out
}
