package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func day(d int) string { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC).Format(dateLayout) }

func entries(moods map[int]int) []PulseEntry {
	var out []PulseEntry
	for d := 1; d <= 30; d++ {
		if m, ok := moods[d]; ok {
			out = append(out, PulseEntry{Date: day(d), Mood: m, At: time.Date(2026, 11, d, 10, 0, 0, 0, time.UTC)})
		}
	}
	return out
}

func TestAssessPulseRiskRules(t *testing.T) {
	cases := []struct {
		name   string
		moods  map[int]int
		risk   bool
		reason string
		trend  string
	}{
		{"нет отметок", nil, false, "", "flat"},
		{"сегодня тяжело", map[int]int{14: 4, 15: 1}, true, "сегодня отметил(а) «Тяжело»", "down"},
		{"сегодня так себе", map[int]int{15: 2}, true, "«Так себе»", "flat"},
		{"две из трёх последних ≤2", map[int]int{12: 2, 13: 4, 14: 1}, true, "2 из 3 последних", "down"},
		{"среднее ≤2.5", map[int]int{9: 3, 10: 3, 11: 1, 12: 3, 13: 3, 14: 2, 15: 3}, false, "", "up"},
		{"низкое среднее без двух низких подряд", map[int]int{9: 1, 10: 1, 11: 2, 13: 3, 14: 3, 15: 3}, true, "среднее за неделю — 2,2 из 5", "up"},
		{"всё хорошо", map[int]int{13: 4, 14: 5, 15: 5}, false, "", "up"},
		{"старые тяжёлые дни вне окна не считаются", map[int]int{1: 1, 2: 1, 3: 1, 15: 4}, false, "", "flat"},
		{"стабильно", map[int]int{13: 3, 14: 3, 15: 3}, false, "", "flat"},
	}
	for _, c := range cases {
		st := AssessPulse(entries(c.moods), day(15))
		if st.Risk != c.risk || (c.reason != "" && !strings.Contains(st.RiskReason, c.reason)) || st.Trend != c.trend {
			t.Errorf("%s: risk=%v reason=%q trend=%s", c.name, st.Risk, st.RiskReason, st.Trend)
		}
		if len(st.Days) != PulseWindowDays || st.Days[6].Date != day(15) || st.Days[0].Date != day(9) {
			t.Errorf("%s: days %+v", c.name, st.Days)
		}
	}
}

func TestAssessPulseFields(t *testing.T) {
	es := entries(map[int]int{13: 4, 14: 3})
	es[0].Note = "устал"
	st := AssessPulse(es, day(15))
	if st.Today != nil {
		t.Fatal("no entry today")
	}
	if st.Avg7 == nil || *st.Avg7 != 3.5 || st.Note != "устал" || st.NoteDate != day(13) {
		t.Fatalf("status: %+v", st)
	}
	if st.Days[4].Mood == nil || *st.Days[4].Mood != 4 || st.Days[6].Mood != nil {
		t.Fatalf("days: %+v", st.Days)
	}
	if bad := AssessPulse(es, "not-a-date"); bad.Risk || len(bad.Days) != 0 {
		t.Fatal("bad date")
	}
}

func TestSetPulseOverwritesTodayAndMasks(t *testing.T) {
	s := New()
	now := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	s.SetPulse("u1", 2, "позвоните +7 999 123-45-67")
	s.SetPulse("u1", 4, "  ")
	got := s.Pulse("u1", 0)
	if len(got) != 1 || got[0].Mood != 4 || got[0].Note != "" || got[0].Date != "2026-11-15" {
		t.Fatalf("overwrite: %+v", got)
	}
	e := s.SetPulse("u1", 1, "звоните +7 999 123-45-67 "+strings.Repeat("я", 400))
	if strings.Contains(e.Note, "999") || len([]rune(e.Note)) > MaxPulseNoteLen {
		t.Fatalf("note must be masked and trimmed: %q", e.Note)
	}
	for i := 1; i <= maxPulsePerUser+10; i++ {
		now = now.AddDate(0, 0, 1)
		s.SetPulse("u1", 3, "")
	}
	if n := len(s.Pulse("u1", 0)); n != maxPulsePerUser {
		t.Fatalf("history must be bounded: %d", n)
	}
	if n := len(s.Pulse("u1", 14)); n != 14 {
		t.Fatalf("last n: %d", n)
	}
	if s.Today() != now.Format(dateLayout) {
		t.Fatal("today")
	}
}

func TestPulseStreak(t *testing.T) {
	if best, at := PulseStreak(entries(map[int]int{1: 3, 2: 3, 4: 3, 5: 3}), 3); best != 2 || at != nil {
		t.Fatalf("gap breaks streak: %d %v", best, at)
	}
	best, at := PulseStreak(entries(map[int]int{1: 3, 3: 3, 4: 3, 5: 3, 6: 3}), 3)
	if best != 4 || at == nil || at.Day() != 5 {
		t.Fatalf("streak: %d %v", best, at)
	}
}

func TestStatsAndAchievements(t *testing.T) {
	s := New()
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	s.SeedChecklist("u1", []ChecklistItem{
		{ID: "a", Stage: "week1", Done: true},
		{ID: "b", Stage: "week1", Kind: ActionAccess},
		{ID: "c", Stage: "week2"},
		{ID: "d", Stage: "week2"},
	})
	unlocked := func() map[string]bool {
		m := map[string]bool{}
		for _, a := range s.Achievements("backend", "u1") {
			m[a.ID] = a.Unlocked
			if a.Title == "" || a.Hint == "" || a.Description == "" {
				t.Fatalf("achievement texts: %+v", a)
			}
			if !a.Unlocked && a.UnlockedAt != nil {
				t.Fatalf("locked with date: %+v", a)
			}
		}
		return m
	}
	if got := unlocked(); len(got) != 7 {
		t.Fatalf("want 7 achievements: %v", got)
	}
	for id, ok := range unlocked() {
		if ok {
			t.Fatalf("%s must be locked at start", id)
		}
	}

	s.RecordQuestion("backend", "u1", OutcomeClosed)
	s.RecordQuestion("backend", "u1", OutcomeRefusal)
	s.RecordQuestion("frontend", "u2", OutcomeClosed)
	s.AddSignal("backend", "u1", "tech", "не работает VPN")
	if ts := s.TeamStatsOf("backend"); ts.Closed != 1 || ts.Refusals != 1 || ts.Signals != 1 {
		t.Fatalf("team stats: %+v", ts)
	}
	if us := s.UserStatsOf("u1"); us.Questions != 2 || us.FirstQuestionAt.IsZero() {
		t.Fatalf("user stats: %+v", us)
	}
	if (s.TeamStatsOf("nobody") != TeamStats{}) {
		t.Fatal("unknown team")
	}

	s.CreateAbsence(Absence{ID: "R1", UserID: "u1", TeamID: "backend", Status: StatusDraft})
	if unlocked()["first_action"] {
		t.Fatal("draft is not a sent action")
	}
	s.TransitionBy("R1", []string{StatusDraft}, StatusPending, "u1", "")
	s.LinkStep("u1", ActionAccess, "R1", StepDone)
	for i := 0; i < 3; i++ {
		s.SetPulse("u1", 4, "")
		now = now.AddDate(0, 0, 1)
	}
	got := unlocked()
	for _, id := range []string{"first_question", "first_action", "brave_ask", "pulse_streak", "access_done", "half_way", "week1_done"} {
		if !got[id] {
			t.Fatalf("%s must be unlocked: %v", id, got)
		}
	}
	// Сигнал чужой команды не засчитывается.
	if s.Achievements("frontend", "u1")[2].Unlocked {
		t.Fatal("signal from another team")
	}
}

func TestPulseAndStatsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, m := New(), NewMessageStore()
	s.SetPulse("u1", 2, "тяжело")
	s.RecordQuestion("backend", "u1", OutcomeClosed)
	if err := (&Persister{Path: path, Store: s, Msgs: m}).Save(); err != nil {
		t.Fatal(err)
	}
	s2 := New()
	if err := (&Persister{Path: path, Store: s2, Msgs: NewMessageStore()}).Load(); err != nil {
		t.Fatal(err)
	}
	if p := s2.Pulse("u1", 0); len(p) != 1 || p[0].Mood != 2 || p[0].Note != "тяжело" {
		t.Fatalf("pulse lost: %+v", p)
	}
	if s2.TeamStatsOf("backend").Closed != 1 || s2.UserStatsOf("u1").Questions != 1 {
		t.Fatal("stats lost")
	}
}
