package store

import "time"

// Счётчики «пользы»: сколько вопросов стажёров Бадди закрыл сам, сколько передал наставнику
// и сколько раз отказал. Плюс личный счётчик вопросов стажёра (для достижений).

// Исходы ответа на вопрос стажёра. «Сэкономлено наставнику» честное: закрытым считается только
// ответ по базе знаний с источником и объяснение термина из словаря. Подтверждённое действие
// учитывается отдельно — в момент подтверждения (RecordConfirmedAction), а не черновика.
// Меню, small talk, просмотр чек-листа и черновики не считаются ни закрытыми, ни отказами.
const (
	OutcomeClosed   = "closed"   // ответ по базе знаний с источником (синоним OutcomeKB)
	OutcomeKB       = "closed"   // ответ по базе знаний с источником
	OutcomeGlossary = "glossary" // термин из словаря Бадди
	OutcomeSignal   = "signal"   // сообщение передано наставнику
	OutcomeRefusal  = "refusal"  // отказ: нет в базе знаний, нет прав и т. п.
	OutcomeNeutral  = "neutral"  // меню, small talk, чек-лист, черновик — не экономия и не отказ
)

// TeamStats — счётчики команды. Closed = KBAnswers + Glossary + ConfirmedActions.
type TeamStats struct {
	Closed           int64 `json:"closed"`
	KBAnswers        int64 `json:"kb_answers"`
	Glossary         int64 `json:"glossary"`
	ConfirmedActions int64 `json:"confirmed_actions"`
	Signals          int64 `json:"signals"`
	Refusals         int64 `json:"refusals"`
}

// UserStats — личные счётчики пользователя.
type UserStats struct {
	Questions       int64     `json:"questions"`
	FirstQuestionAt time.Time `json:"first_question_at,omitempty"`
	// FirstSignalAt — первое сообщение наставнику. Хранится отдельно от списка сигналов команды:
	// тот ограничен maxSignalsPerTeam, и при вытеснении старых сигналов достижение «Смелость спросить» пропадало.
	FirstSignalAt   *time.Time `json:"first_signal_at,omitempty"`
	FirstSignalTeam string     `json:"first_signal_team,omitempty"`
}

// RecordQuestion учитывает вопрос стажёра и его исход. Исход signal здесь не считается:
// сигналы учитывает AddSignal (сигнал может появиться и без вопроса).
func (s *Store) RecordQuestion(teamID, userID, outcome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	us := s.userStatsLocked(userID)
	us.Questions++
	if us.FirstQuestionAt.IsZero() {
		us.FirstQuestionAt = s.now()
	}
	ts := s.teamStatsLocked(teamID)
	switch outcome {
	case OutcomeKB:
		ts.KBAnswers++
		ts.Closed++
	case OutcomeGlossary:
		ts.Glossary++
		ts.Closed++
	case OutcomeRefusal:
		ts.Refusals++
	}
	s.touch()
}

// RecordConfirmedAction учитывает подтверждённое стажёром действие (draft → pending): заявку
// не пришлось оформлять с наставником. Черновики и отменённые заявки не считаются.
func (s *Store) RecordConfirmedAction(teamID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.teamStatsLocked(teamID)
	ts.ConfirmedActions++
	ts.Closed++
	s.touch()
}

// UncountConfirmedAction списывает подтверждённое действие, которое стажёр затем отменил:
// отмены в «Сэкономлено наставнику» не попадают никогда.
func (s *Store) UncountConfirmedAction(teamID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.teamStatsLocked(teamID)
	if ts.ConfirmedActions > 0 {
		ts.ConfirmedActions--
		ts.Closed--
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

func (s *Store) userStatsLocked(userID string) *UserStats {
	us := s.userStats[userID]
	if us == nil {
		us = &UserStats{}
		s.userStats[userID] = us
	}
	return us
}

// FirstSignalAt — когда пользователь впервые написал наставнику (nil — ещё не писал).
func (s *Store) FirstSignalAt(teamID, userID string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if us := s.userStats[userID]; us != nil && us.FirstSignalAt != nil && us.FirstSignalTeam == teamID {
		at := *us.FirstSignalAt
		return &at
	}
	// Снимки состояния до появления поля first_signal_at — ищем в списке сигналов команды.
	for _, sig := range s.signals[teamID] {
		if sig.UserID == userID {
			at := sig.At
			return &at
		}
	}
	return nil
}
