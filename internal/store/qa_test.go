package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// QA: «Смелость спросить» не пропадает, когда старые сигналы команды вытеснены лимитом, и переживает перезапуск.
func TestQABraveAskSurvivesSignalTrimAndRestart(t *testing.T) {
	s := New()
	s.AddSignal("backend", "u1", "blocked", "застрял")
	for i := 0; i < maxSignalsPerTeam+5; i++ {
		s.AddSignal("backend", "u2", "other", "шум")
	}
	if s.FirstSignalAt("backend", "u1") == nil {
		t.Fatal("first signal lost after trim")
	}
	if s.FirstSignalAt("frontend", "u1") != nil {
		t.Fatal("signal of another team must not count")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := (&Persister{Path: path, Store: s, Msgs: NewMessageStore()}).Save(); err != nil {
		t.Fatal(err)
	}
	s2 := New()
	if err := (&Persister{Path: path, Store: s2, Msgs: NewMessageStore()}).Load(); err != nil {
		t.Fatal(err)
	}
	if s2.FirstSignalAt("backend", "u1") == nil || !s2.Achievements("backend", "u1")[2].Unlocked {
		t.Fatal("brave_ask must survive restart")
	}
	if got := len(s2.Signals("backend")); got != maxSignalsPerTeam {
		t.Fatalf("signals after restart: %d", got)
	}
}

// QA: горизонт планирования отгула — опечатка в годе (2099) отклоняется, ближайшие даты принимаются.
func TestQAAbsenceHorizon(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	if err := ValidateAbsenceDates("2099-01-01", "2099-01-01", now); !errors.Is(err, ErrTooFarAhead) {
		t.Fatalf("2099: %v", err)
	}
	if err := ValidateAbsenceDates("2031-11-15", "2031-11-15", now); err != nil {
		t.Fatalf("exactly 5 years ahead: %v", err)
	}
	if err := ValidateAbsenceDates("2031-11-16", "2031-11-16", now); !errors.Is(err, ErrTooFarAhead) {
		t.Fatalf("5 years + 1 day: %v", err)
	}
	if err := ValidateAbsenceDates("2027-02-30", "2027-03-01", now); err == nil {
		t.Fatal("Feb 30 must be rejected")
	}
}
