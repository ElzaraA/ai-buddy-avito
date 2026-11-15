package store

import "time"

// «Достижения» стажёра: награждают правильное поведение в адаптации — спрашивать Бадди,
// оформлять заявки самому, отмечать самочувствие и, главное, не стесняться просить помощи у наставника.
// Всё выводится из уже сохранённого состояния, поэтому детерминировано и переживает перезапуск.

type Achievement struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Unlocked    bool       `json:"unlocked"`
	UnlockedAt  *time.Time `json:"unlocked_at,omitempty"`
	Hint        string     `json:"hint"`
}

// PulseStreakDays — сколько дней подряд нужно отмечать «Пульс» для достижения.
const PulseStreakDays = 3

// Achievements — достижения пользователя в фиксированном порядке (от самых простых к сложным).
func (s *Store) Achievements(teamID, userID string) []Achievement {
	us := s.UserStatsOf(userID)
	items := s.Checklist(userID)

	var firstQ *time.Time
	if us.Questions > 0 {
		at := us.FirstQuestionAt
		firstQ = &at
	}

	// Первая отправленная заявка — самое раннее событие pending в истории заявок.
	var firstAction *time.Time
	for _, a := range s.ListByUser(userID) {
		for _, ev := range a.History {
			if ev.Status == StatusPending && (firstAction == nil || ev.At.Before(*firstAction)) {
				at := ev.At
				firstAction = &at
			}
		}
	}

	accessDone, week1Total, week1Done, done := false, 0, 0, 0
	for _, it := range items {
		if it.Done {
			done++
		}
		if it.Kind == ActionAccess && it.Done {
			accessDone = true
		}
		if it.Stage == "week1" {
			week1Total++
			if it.Done {
				week1Done++
			}
		}
	}
	half := len(items) > 0 && done*100/len(items) >= 50

	_, streakAt := PulseStreak(s.Pulse(userID, 0), PulseStreakDays)
	signalAt := s.FirstSignalAt(teamID, userID)

	mk := func(id, title, desc, hint string, ok bool, at *time.Time) Achievement {
		a := Achievement{ID: id, Title: title, Description: desc, Hint: hint, Unlocked: ok}
		if ok {
			a.UnlockedAt = at
		}
		return a
	}
	return []Achievement{
		mk("first_question", "Первый вопрос", "Задали Бадди первый вопрос",
			"Спросите Бадди о чём угодно — например, «С чего начать?»", firstQ != nil, firstQ),
		mk("first_action", "Первая заявка", "Отправили первую заявку через Бадди",
			"Попросите Бадди оформить доступ или отгул и подтвердите черновик", firstAction != nil, firstAction),
		mk("brave_ask", "Смелость спросить", "Написали наставнику, когда застряли, — это сильный ход",
			"Застряли больше 30 минут? Нажмите «Сообщить наставнику»", signalAt != nil, signalAt),
		mk("pulse_streak", "Пульс 3 дня подряд", "Три дня подряд отмечали, как вы",
			"Отвечайте на «Как вы сегодня?» три дня подряд", streakAt != nil, streakAt),
		mk("access_done", "Доступы получены", "Шаг с доступами закрыт",
			"Запросите доступы через Бадди — шаг закроется после согласования", accessDone, nil),
		mk("half_way", "Половина пути", "Пройдена половина плана адаптации",
			"Закройте половину шагов в «Моём плане»", half, nil),
		mk("week1_done", "Неделя 1 пройдена", "Все шаги первой недели выполнены",
			"Закройте все шаги первой недели в «Моём плане»", week1Total > 0 && week1Done == week1Total, nil),
	}
}
