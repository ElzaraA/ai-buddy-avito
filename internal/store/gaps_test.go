package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// R2-4: в пробелы базы знаний персональные данные не попадают.
func TestRecordGapMasksPII(t *testing.T) {
	s := New()
	s.RecordGap("backend", "мой телефон +7 999 123-45-67 и почта a@b.ru, паспорт 4510 123456")
	s.RecordGap("backend", "Мой телефон 8 (912) 000-11-22 и почта ivan.petrov@avito.ru, паспорт 4511 654321")
	g := s.Gaps("backend")
	if len(g) != 1 || g[0].Count != 2 {
		t.Fatalf("same question with different personal data must merge: %+v", g)
	}
	q := g[0].Question
	for _, leak := range []string{"999", "123-45", "a@b.ru", "4510", "123456", "@"} {
		if strings.Contains(q, leak) {
			t.Fatalf("leaked %q in %q", leak, q)
		}
	}
	if q != "мой телефон [число] и почта [email], паспорт [число]" {
		t.Fatalf("masked: %q", q)
	}
}

func TestMaskPIIKeepsShortNumbers(t *testing.T) {
	for in, want := range map[string]string{
		"Во сколько стендап, в 11:00?":     "Во сколько стендап, в 11:00?",
		"Какую версию Go 1.22 ставить?":    "Какую версию Go 1.22 ставить?",
		"Номер счёта 40817810099910004312": "Номер счёта [число]",
		"СНИЛС 112-233-445 95":             "СНИЛС [число]",
	} {
		if got := MaskPII(in); got != want {
			t.Fatalf("%q: got %q, want %q", in, got, want)
		}
	}
}

// R2-11: по сигналу RequestSave Persister сохраняет сразу, не дожидаясь тика.
func TestPersisterSavesOnRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, m := New(), NewMessageStore()
	p := &Persister{Path: path, Store: s, Msgs: m}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); p.Run(ctx, time.Hour) }()
	defer func() { cancel(); <-done }()

	s.CreateAbsence(Absence{ID: "ABS-000001", UserID: "u1", Status: StatusPending})
	s.RequestSave()
	s.RequestSave() // повторный сигнал не блокирует
	deadline := time.Now().Add(2 * time.Second)
	for {
		if raw, err := os.ReadFile(path); err == nil && strings.Contains(string(raw), "ABS-000001") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("state was not saved on request")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Без Persister сигнал не блокирует и ничего не меняет.
func TestRequestSaveWithoutPersister(t *testing.T) {
	s := New()
	for i := 0; i < 5; i++ {
		s.RequestSave()
	}
	if len(s.SaveRequests()) != 1 {
		t.Fatal("signals must collapse into one")
	}
}
