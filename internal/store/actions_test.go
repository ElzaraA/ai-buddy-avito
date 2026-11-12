package store

import (
	"sync"
	"testing"
	"time"
)

func TestNextIDPrefixes(t *testing.T) {
	s := New()
	if id := s.NextID("ABS"); id != "ABS-000001" {
		t.Fatal(id)
	}
	if id := s.NextID("REQ"); id != "REQ-000001" {
		t.Fatal(id)
	}
	if id := s.NextAbsenceID(); id != "ABS-000002" {
		t.Fatal(id)
	}
}

func TestListByUserSortedAndScoped(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	s.CreateAbsence(Absence{ID: "A1", UserID: "u1", CreatedAt: t0})
	s.CreateAbsence(Absence{ID: "A2", UserID: "u1", Type: ActionAccess, CreatedAt: t0.Add(time.Minute)})
	s.CreateAbsence(Absence{ID: "A3", UserID: "u2", CreatedAt: t0})
	got := s.ListByUser("u1")
	if len(got) != 2 || got[0].ID != "A2" || got[1].ID != "A1" {
		t.Fatalf("%+v", got)
	}
	if got[1].Type != ActionAbsence {
		t.Fatalf("default type must be absence: %+v", got[1])
	}
	if len(s.ListByUser("nobody")) != 0 {
		t.Fatal("must be empty")
	}
}

// Checklist отдаёт копию: изменения снаружи не портят хранилище,
// а параллельные MarkDone/Checklist безопасны (проверяется go test -race в CI).
func TestChecklistCopyAndConcurrency(t *testing.T) {
	s := New()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "s1"}, {ID: "s2"}})
	got := s.Checklist("u1")
	got[0].Done = true
	if s.Checklist("u1")[0].Done {
		t.Fatal("external mutation leaked into store")
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); s.MarkDone("u1", "s2") }()
		go func() {
			defer wg.Done()
			for _, it := range s.Checklist("u1") {
				_ = it.Done
			}
		}()
	}
	wg.Wait()
	if !s.Checklist("u1")[1].Done {
		t.Fatal("s2 must be done")
	}
	if found, owned := s.MarkDoneOwned("u2", "s1"); !found || owned {
		t.Fatal("foreign step must be found but not owned")
	}
}
