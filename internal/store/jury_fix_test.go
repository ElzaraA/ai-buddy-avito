package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// «Сегодня» — по Москве: 22:30 UTC — это уже следующий день по МСК. Без tzdata — фиксированный UTC+3.
func TestTodayIsMoscow(t *testing.T) {
	s := New()
	s.SetClock(func() time.Time { return time.Date(2026, 11, 13, 22, 30, 0, 0, time.UTC) })
	if got := s.Today(); got != "2026-11-14" {
		t.Fatalf("today: %s", got)
	}
	if e := s.SetPulse("u1", 3, ""); e.Date != "2026-11-14" {
		t.Fatalf("pulse date: %s", e.Date)
	}
	if loc := LoadLocation("No/Such_Zone"); loc != mskFixed {
		t.Fatalf("fallback must be fixed UTC+3, got %v", loc)
	}
	if _, off := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).In(LoadLocation("")).Zone(); off != 3*3600 {
		t.Fatalf("default zone offset: %d", off)
	}
	// Отгул «на сегодня» по Москве допустим, даже если по UTC ещё вчера.
	now := time.Date(2026, 11, 13, 22, 30, 0, 0, time.UTC)
	if err := ValidateAbsenceDates("2026-11-14", "2026-11-14", now); err != nil {
		t.Fatalf("moscow today rejected: %v", err)
	}
	if err := ValidateAbsenceDates("2026-11-13", "2026-11-13", now); err == nil {
		t.Fatal("moscow yesterday accepted")
	}
}

// Ответ наставника сохраняется в снимке, PII маскируются, свои сигналы — новые сверху.
func TestSignalReplyPersisted(t *testing.T) {
	s := New()
	a := s.AddSignal("backend", "u1", "tech", "не работает VPN")
	s.AddSignal("backend", "u1", "other", "вопрос по задаче")
	s.AddSignal("backend", "u2", "other", "чужой")
	sig, ok := s.ResolveSignalWithReply("backend", a.ID, "Иван Петров", "пиши на ivan@example.com "+strings.Repeat("я", 600))
	if !ok || strings.Contains(sig.Reply, "ivan@example.com") || len([]rune(sig.Reply)) > MaxSignalReplyLen ||
		sig.ResolvedBy != "Иван Петров" || sig.ResolvedAt == nil || !sig.Done {
		t.Fatalf("resolve: %+v", sig)
	}
	if _, ok := s.ResolveSignalWithReply("frontend", a.ID, "x", ""); ok {
		t.Fatal("other team resolved")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := (&Persister{Path: path, Store: s, Msgs: NewMessageStore()}).Save(); err != nil {
		t.Fatal(err)
	}
	s2 := New()
	if err := (&Persister{Path: path, Store: s2, Msgs: NewMessageStore()}).Load(); err != nil {
		t.Fatal(err)
	}
	mine := s2.SignalsOf("u1")
	if len(mine) != 2 || mine[0].Text != "вопрос по задаче" || mine[1].Reply == "" || mine[1].ResolvedBy != "Иван Петров" {
		t.Fatalf("signals of u1 after load: %+v", mine)
	}
}

// Шаг «Получить доступы (Git, Jira, Grafana)» закрывается только после одобрения всех трёх систем.
func TestAccessStepNeedsAllSystems(t *testing.T) {
	s := New()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "acc", Title: "Получить доступы (Git, Jira, Grafana)", Kind: ActionAccess}})
	it := s.Checklist("u1")[0]
	if it.SubDone == nil || *it.SubDone != 0 || it.SubTotal != 3 || len(it.Missing) != 3 {
		t.Fatalf("seed: %+v", it)
	}
	add := func(id, res, status string) {
		s.CreateAbsence(Absence{ID: id, Type: ActionAccess, Resource: res, UserID: "u1", Status: status})
	}
	add("R1", "git", StatusApproved) // git = gitlab
	it, _ = s.SyncAccessStep("u1")
	if it.Done || it.Status != StepTodo || *it.SubDone != 1 || strings.Join(it.Missing, ",") != "jira,grafana" {
		t.Fatalf("after git: %+v", it)
	}
	add("R2", "jira", StatusPending)
	add("R3", "grafana", StatusPending)
	if it, _ = s.SyncAccessStep("u1"); it.Status != StepWaiting || len(it.Missing) != 0 {
		t.Fatalf("all requested: %+v", it)
	}
	s.TransitionBy("R2", []string{StatusPending}, StatusApproved, "lead", "")
	s.TransitionBy("R3", []string{StatusPending}, StatusApproved, "lead", "")
	if it, _ = s.SyncAccessStep("u1"); !it.Done || it.Status != StepDone || *it.SubDone != 3 {
		t.Fatalf("all approved: %+v", it)
	}
	if st := s.AccessStatus("u1"); st["gitlab"] != StatusApproved || st["jira"] != StatusApproved {
		t.Fatalf("access status: %v", st)
	}
}

// Сброс возвращает хранилище в пустое состояние.
func TestResetClearsEverything(t *testing.T) {
	s := New()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "a", Title: "a"}})
	s.AddSignal("backend", "u1", "other", "x")
	s.SetPulse("u1", 2, "")
	s.RecordQuestion("backend", "u1", OutcomeKB)
	s.RecordGap("backend", "Где парковка?")
	s.CreateAbsence(Absence{ID: s.NextID("ABS"), UserID: "u1", Status: StatusPending})
	v := s.Version()
	s.Reset()
	if s.HasChecklist("u1") || len(s.SignalsOf("u1")) != 0 || len(s.Pulse("u1", 0)) != 0 || s.TeamStatsOf("backend") != (TeamStats{}) ||
		s.UserStatsOf("u1").Questions != 0 || len(s.Gaps("backend")) != 0 || len(s.ListByUser("u1")) != 0 || s.Version() <= v {
		t.Fatal("reset left state behind")
	}
	if id := s.NextID("ABS"); id != "ABS-000001" {
		t.Fatalf("seq not reset: %s", id)
	}
	m := NewMessageStore()
	m.CreateSession("u1", "s1")
	m.Reset()
	if _, ok := m.GetSession("s1"); ok {
		t.Fatal("sessions not reset")
	}
}

// Честный учёт: только ответы по базе, термины и подтверждённые действия; отмена списывает.
func TestHonestStats(t *testing.T) {
	s := New()
	for _, o := range []string{OutcomeKB, OutcomeGlossary, OutcomeNeutral, OutcomeNeutral, OutcomeRefusal} {
		s.RecordQuestion("backend", "u1", o)
	}
	s.RecordConfirmedAction("backend")
	s.RecordConfirmedAction("backend")
	s.UncountConfirmedAction("backend")
	ts := s.TeamStatsOf("backend")
	if ts.Closed != 3 || ts.KBAnswers != 1 || ts.Glossary != 1 || ts.ConfirmedActions != 1 || ts.Refusals != 1 {
		t.Fatalf("stats: %+v", ts)
	}
	if s.UserStatsOf("u1").Questions != 5 {
		t.Fatal("questions counter (achievements) must count every question")
	}
}
