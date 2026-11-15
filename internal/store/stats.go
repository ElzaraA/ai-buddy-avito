package store

import "time"

// Счётчики «пользы»: сколько вопросов стажёров Бадди закрыл сам, сколько передал наставнику
// и сколько раз отказал. Плюс личный счётчик вопросов стажёра (для достижений).

// Исходы ответа на вопрос стажёра.
const (
	OutcomeClosed  = "closed"  // answer / action / draft — вопрос закрыт без человека
	OutcomeSignal  = "signal"  // сообщение передано наставнику
	OutcomeRefusal = "refusal" // отказ: нет в базе знаний, нет прав и т. п.
)

// TeamStats — счётчики команды.
type TeamStats struct {
	Closed   int64 `json:"closed"`
	Signals  int64 `json:"signals"`
	Refusals int64 `json:"refusals"`
}

// UserStats — личные счётчики пользователя.
type UserStats struct {
	Questions       int64     `json:"questions"`
	FirstQuestionAt time.Time `json:"first_question_at,omitempty"`
}

// RecordQuestion учитывает вопрос стажёра и его исход. Исход signal здесь не считается:
// сигналы учитывает AddSignal (сигнал может появиться и без вопроса).
func (s *Store) RecordQuestion(teamID, userID, outcome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	us := s.userStats[userID]
	if us == nil {
		us = &UserStats{}
		s.userStats[userID] = us
	}
	us.Questions++
	if us.FirstQuestionAt.IsZero() {
		us.FirstQuestionAt = s.now()
	}
	ts := s.teamStatsLocked(teamID)
	switch outcome {
	case OutcomeClosed:
		ts.Closed++
	case OutcomeRefusal:
		ts.Refusals++
	}
	s.touch()
}

func (s *Store) teamStatsLocked(teamID string) *TeamStats {
	ts := s.teamStats[teamID]
	if ts == nil {
		ts = &TeamStats{}
		s.teamStats[teamID] = ts
	}
	return ts
}

// TeamStatsOf — копия счётчиков команды.
func (s *Store) TeamStatsOf(teamID string) TeamStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ts := s.teamStats[teamID]; ts != nil {
		return *ts
	}
	return TeamStats{}
}

// UserStatsOf — копия личных счётчиков.
func (s *Store) UserStatsOf(userID string) UserStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if us := s.userStats[userID]; us != nil {
		return *us
	}
	return UserStats{}
}

// FirstSignalAt — когда пользователь впервые написал наставнику (nil — ещё не писал).
func (s *Store) FirstSignalAt(teamID, userID string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sig := range s.signals[teamID] {
		if sig.UserID == userID {
			at := sig.At
			return &at
		}
	}
	return nil
}
