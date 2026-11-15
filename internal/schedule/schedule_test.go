package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

var (
	intern = auth.DemoUsers["intern.backend"]
	mentor = auth.DemoUsers["mentor.backend"]
	lead   = auth.DemoUsers["lead.backend"]
)

func at(day, hour, min int) time.Time { return time.Date(2026, 11, day, hour, min, 0, 0, MSK) }

func kinds(occ []Occurrence) map[string]int {
	out := map[string]int{}
	for _, o := range occ {
		out[o.Kind]++
	}
	return out
}

func TestWeekOfInternMatchesTeamRituals(t *testing.T) {
	// неделя пн 16.11 – вс 22.11: вторая неделя первого спринта
	occ := Expand(TeamRituals(intern), nil, at(16, 0, 0), at(23, 0, 0))
	k := kinds(occ)
	if k[KindStandup] != 5 || k[KindOneOnOne] != 1 || k[KindRetro] != 1 || k[KindPlanning] != 0 {
		t.Fatalf("unexpected week: %v", k)
	}
	for _, o := range occ {
		if o.Kind == KindStandup && (o.Start.Hour() != 11 || o.Start.Weekday() == time.Saturday || o.Start.Weekday() == time.Sunday) {
			t.Fatalf("standup must be weekdays at 11:00 (как в базе знаний): %v", o.Start)
		}
		if o.Kind == KindOneOnOne && o.With != "Иван Петров" {
			t.Fatalf("1:1 must be with the team buddy, got %q", o.With)
		}
		if !o.Recurring || o.MeetingID != "" {
			t.Fatalf("team ritual must be recurring and not cancellable: %+v", o)
		}
	}
	// следующая неделя — первая неделя второго спринта: планирование есть, ретро нет
	next := kinds(Expand(TeamRituals(intern), nil, at(23, 0, 0), at(30, 0, 0)))
	if next[KindPlanning] != 1 || next[KindRetro] != 0 {
		t.Fatalf("sprint rhythm broken: %v", next)
	}
}

func TestOneOnOneDependsOnRole(t *testing.T) {
	week := func(u auth.User) map[string]int {
		return kinds(Expand(TeamRituals(u), nil, at(16, 0, 0), at(23, 0, 0)))
	}
	if week(lead)[KindOneOnOne] != 0 {
		t.Fatal("lead has no 1:1 with interns in this prototype")
	}
	occ := Expand(TeamRituals(mentor), nil, at(18, 0, 0), at(19, 0, 0))
	found := false
	for _, o := range occ {
		if o.Kind == KindOneOnOne {
			found = o.Title == "1:1 со стажёрами" && o.With == ""
		}
	}
	if !found {
		t.Fatalf("mentor must see 1:1 with interns: %+v", occ)
	}
	other := TeamRituals(auth.DemoUsers["intern.other"])
	for _, r := range other {
		if r.Kind == KindOneOnOne && r.With != "Анна Козлова" {
			t.Fatalf("frontend intern has own buddy, got %q", r.With)
		}
	}
}

func TestSprintRhythmBeforeStart(t *testing.T) {
	// даты до начала первого спринта не ломают расчёт чётности недель
	occ := Expand(TeamRituals(intern), nil, time.Date(2026, 10, 26, 0, 0, 0, 0, MSK), time.Date(2026, 11, 9, 0, 0, 0, 0, MSK))
	k := kinds(occ)
	if k[KindPlanning]+k[KindRetro] != 2 {
		t.Fatalf("one planning and one retro expected in any two weeks: %v", k)
	}
}

func TestExpandMergesPersonalAndSkipsFinished(t *testing.T) {
	personal := []store.Meeting{
		{ID: "MTG-1", Title: "Разбор первой задачи", Start: at(16, 15, 0), DurationMin: 30, With: "Иван Петров"},
		{ID: "MTG-2", Title: "Прошедший", Start: at(16, 9, 0), DurationMin: 30},
	}
	// с 11:05 понедельника: стендап 11:00–11:15 ещё идёт, созвон в 9:00 закончился
	occ := Expand(TeamRituals(intern), personal, at(16, 11, 5), at(16, 23, 0))
	if len(occ) != 2 || occ[0].Kind != KindStandup || occ[1].ID != "MTG-1" {
		t.Fatalf("got %+v", occ)
	}
	p := occ[1]
	if p.Kind != KindPersonal || p.MeetingID != "MTG-1" || p.Recurring || !p.End.Equal(at(16, 15, 30)) {
		t.Fatalf("personal meeting: %+v", p)
	}
}

func TestDueAndReminderText(t *testing.T) {
	occ := Expand(TeamRituals(intern), nil, at(18, 10, 0), at(18, 23, 0))
	now := at(18, 10, 45)
	due := Due(occ, now, 15*time.Minute)
	if len(due) != 1 || due[0].Kind != KindStandup {
		t.Fatalf("only standup is due in 15 minutes: %+v", due)
	}
	if got := ReminderText(due[0], now); got != "Через 15 минут — Стендап команды в 11:00, Яндекс Телемост." {
		t.Fatalf("text: %q", got)
	}
	// уже начавшаяся встреча не напоминается
	if len(Due(occ, at(18, 11, 1), 15*time.Minute)) != 0 {
		t.Fatal("started meeting must not be due")
	}
	one := Expand(TeamRituals(intern), nil, at(18, 15, 0), at(18, 17, 0))[0]
	if got := ReminderText(one, at(18, 15, 58)); !strings.HasPrefix(got, "Через 2 минуты — 1:1 с наставником в 16:00 (Иван Петров)") {
		t.Fatalf("text: %q", got)
	}
	if got := ReminderText(one, at(18, 15, 59)); !strings.HasPrefix(got, "Через 1 минуту") {
		t.Fatalf("text: %q", got)
	}
	if got := ReminderText(one, at(18, 16, 0)); !strings.HasPrefix(got, "Через 1 минуту") {
		t.Fatalf("at start time still says 1 minute: %q", got)
	}
}

func TestMinutesWordAndLabel(t *testing.T) {
	cases := map[int]string{1: "1 минуту", 2: "2 минуты", 5: "5 минут", 11: "11 минут", 12: "12 минут", 21: "21 минуту", 22: "22 минуты"}
	for n, want := range cases {
		if got := minutesWord(n); got != want {
			t.Fatalf("%d: %q", n, got)
		}
	}
	o := Occurrence{Title: "1:1 с наставником", Start: at(18, 16, 0)}
	if got := Label(o); got != "ср 18.11, 16:00 — 1:1 с наставником" {
		t.Fatalf("label: %q", got)
	}
}
