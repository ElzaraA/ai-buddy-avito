package store

// Reset возвращает хранилище в состояние «как после свежего старта»: без заявок, чек-листов,
// сигналов, «Пульса», пробелов базы знаний, счётчиков «Сэкономлено наставнику» и достижений.
// Часы (SetClock) не трогаются. Чек-листы демо-стажёров после сброса заново засевает api.SeedDemo.
// Версия растёт — Persister сохранит пустое состояние в снимок.
func (s *Store) Reset() {
	s.mu.Lock()
	s.absences = map[string]Absence{}
	s.checklist = map[string][]ChecklistItem{}
	s.seq = map[string]int64{}
	s.gaps = map[string]map[string]*Gap{}
	s.signals = map[string][]Signal{}
	s.pulse = map[string][]PulseEntry{}
	s.teamStats = map[string]*TeamStats{}
	s.userStats = map[string]*UserStats{}
	s.touch()
	s.mu.Unlock()
}

// Reset удаляет все диалоги и сообщения чата.
func (s *MessageStore) Reset() {
	s.mu.Lock()
	s.sessions = map[string]Session{}
	s.messages = map[string]Message{}
	s.bySess = map[string][]string{}
	s.version.Add(1)
	s.mu.Unlock()
}
