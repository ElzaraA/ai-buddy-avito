// Package schedule — расписание встреч стажёра: ритуалы команды (повторяются по правилам)
// и личные созвоны пользователя. Ритуалы совпадают с базой знаний (data/knowledge/team-rituals.md),
// чтобы вкладка «Расписание» и ответы помощника говорили одно и то же.
package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// MSK — московское время без перехода на летнее. Фиксированная зона, а не LoadLocation:
// в минимальном Docker-образе может не быть базы часовых поясов.
var MSK = time.FixedZone("MSK", 3*60*60)

// Виды встреч: по ним интерфейс выбирает цвет карточки.
const (
	KindStandup  = "standup"
	KindOneOnOne = "one_on_one"
	KindPlanning = "planning"
	KindRetro    = "retro"
	KindPersonal = "personal"
)

// Ritual — повторяющаяся встреча команды.
type Ritual struct {
	ID       string
	Title    string
	Kind     string
	Weekdays []time.Weekday
	Hour     int
	Minute   int
	Duration time.Duration
	// EveryTwoWeeks — раз в спринт (две недели), считая от SprintStart.
	EveryTwoWeeks bool
	Place         string
	With          string
}

// SprintStart — понедельник начала первого спринта прототипа: от него считаются двухнедельные встречи.
var SprintStart = time.Date(2026, 11, 9, 0, 0, 0, 0, MSK)

// Occurrence — конкретная встреча в расписании.
type Occurrence struct {
	ID        string    `json:"id"` // ритуал@дата-время или id личного созвона — уникален для напоминаний
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Place     string    `json:"place,omitempty"`
	With      string    `json:"with,omitempty"`
	Recurring bool      `json:"recurring"`            // встреча команды; отменить из интерфейса нельзя
	MeetingID string    `json:"meeting_id,omitempty"` // для личного созвона — id для отмены
}

var weekdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}

// TeamRituals — встречи команды, которые видит пользователь (1:1 — только стажёру и наставнику).
func TeamRituals(u auth.User) []Ritual {
	c := auth.TeamContacts(u.TeamID)
	out := []Ritual{
		{ID: "standup", Title: "Стендап команды", Kind: KindStandup, Weekdays: weekdays,
			Hour: 11, Duration: 15 * time.Minute, Place: "Яндекс Телемост"},
		{ID: "planning", Title: "Планирование спринта", Kind: KindPlanning, Weekdays: []time.Weekday{time.Monday},
			Hour: 12, Duration: time.Hour, EveryTwoWeeks: true, Place: "Переговорная «Москва»"},
		{ID: "retro", Title: "Ретро спринта", Kind: KindRetro, Weekdays: []time.Weekday{time.Friday},
			Hour: 17, Duration: time.Hour, EveryTwoWeeks: true, Place: "Переговорная «Москва»"},
	}
	switch u.Role {
	case auth.RoleIntern:
		out = append(out, Ritual{ID: "one-on-one", Title: "1:1 с наставником", Kind: KindOneOnOne,
			Weekdays: []time.Weekday{time.Wednesday}, Hour: 16, Duration: 30 * time.Minute,
			Place: "Яндекс Телемост", With: c.Buddy})
	case auth.RoleBuddy:
		out = append(out, Ritual{ID: "one-on-one", Title: "1:1 со стажёрами", Kind: KindOneOnOne,
			Weekdays: []time.Weekday{time.Wednesday}, Hour: 16, Duration: 30 * time.Minute, Place: "Яндекс Телемост"})
	}
	return out
}

// sprintWeek — чётность недели относительно начала спринтов (0 — первая неделя спринта).
func sprintWeek(day time.Time) int {
	days := int(day.Sub(SprintStart).Hours() / 24)
	if days < 0 {
		days = -days + 7 // симметрично для дат до начала: на чётность влияет только номер недели
	}
	return (days / 7) % 2
}

func occurs(r Ritual, day time.Time) bool {
	matched := false
	for _, wd := range r.Weekdays {
		if day.Weekday() == wd {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	if !r.EveryTwoWeeks {
		return true
	}
	// планирование — в первый понедельник спринта, ретро — в последнюю пятницу (вторая неделя)
	if r.Kind == KindRetro {
		return sprintWeek(day) == 1
	}
	return sprintWeek(day) == 0
}

// Expand разворачивает ритуалы и личные созвоны в встречи на интервале [from, to), по времени начала.
// Встреча попадает в интервал, если ещё не закончилась к from.
func Expand(rituals []Ritual, personal []store.Meeting, from, to time.Time) []Occurrence {
	out := make([]Occurrence, 0)
	start := from.In(MSK)
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, MSK)
	for ; day.Before(to); day = day.AddDate(0, 0, 1) {
		for _, r := range rituals {
			if !occurs(r, day) {
				continue
			}
			s := day.Add(time.Duration(r.Hour)*time.Hour + time.Duration(r.Minute)*time.Minute)
			e := s.Add(r.Duration)
			if !e.After(from) || !s.Before(to) {
				continue
			}
			out = append(out, Occurrence{
				ID: r.ID + "@" + s.Format("2006-01-02T15:04"), Title: r.Title, Kind: r.Kind,
				Start: s, End: e, Place: r.Place, With: r.With, Recurring: true,
			})
		}
	}
	for _, m := range personal {
		s := m.Start.In(MSK)
		e := s.Add(time.Duration(m.DurationMin) * time.Minute)
		if !e.After(from) || !s.Before(to) {
			continue
		}
		out = append(out, Occurrence{
			ID: m.ID, Title: m.Title, Kind: KindPersonal, Start: s, End: e,
			With: m.With, MeetingID: m.ID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// Due — встречи, которые начнутся в ближайшие lead минут (уже начавшиеся не напоминаем).
func Due(occ []Occurrence, now time.Time, lead time.Duration) []Occurrence {
	out := make([]Occurrence, 0)
	for _, o := range occ {
		if o.Start.After(now) && !o.Start.After(now.Add(lead)) {
			out = append(out, o)
		}
	}
	return out
}

// minutesWord — «1 минуту», «3 минуты», «15 минут».
func minutesWord(n int) string {
	m10, m100 := n%10, n%100
	switch {
	case m10 == 1 && m100 != 11:
		return fmt.Sprintf("%d минуту", n)
	case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
		return fmt.Sprintf("%d минуты", n)
	default:
		return fmt.Sprintf("%d минут", n)
	}
}

// ReminderText — текст напоминания для чата: «Через 15 минут — Стендап команды в 11:00, Яндекс Телемост».
func ReminderText(o Occurrence, now time.Time) string {
	left := int(o.Start.Sub(now).Round(time.Minute) / time.Minute)
	if left < 1 {
		left = 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Через %s — %s в %s", minutesWord(left), o.Title, o.Start.In(MSK).Format("15:04"))
	if o.With != "" {
		fmt.Fprintf(&b, " (%s)", o.With)
	}
	if o.Place != "" {
		fmt.Fprintf(&b, ", %s", o.Place)
	}
	b.WriteString(".")
	return b.String()
}

var weekdayShort = map[time.Weekday]string{
	time.Monday: "пн", time.Tuesday: "вт", time.Wednesday: "ср", time.Thursday: "чт",
	time.Friday: "пт", time.Saturday: "сб", time.Sunday: "вс",
}

// Label — «ср 18.11, 16:00 — 1:1 с наставником» для коротких ответов помощника.
func Label(o Occurrence) string {
	s := o.Start.In(MSK)
	return fmt.Sprintf("%s %s, %s — %s", weekdayShort[s.Weekday()], s.Format("02.01"), s.Format("15:04"), o.Title)
}
