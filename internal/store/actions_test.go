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
	if found, owned, _ := s.MarkDoneOwned("u2", "s1"); !found || owned {
		t.Fatal("foreign step must be found but not owned")
	}
}

// Шаг доступа закрывается только одобрением заявки, а не кнопкой стажёра (BR-6).
func TestAccessStepIsLocked(t *testing.T) {
	s := New()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "acc", Kind: "access"}, {ID: "w", Status: StepWaiting}, {ID: "p"}})
	for _, id := range []string{"acc", "w"} {
		if found, owned, locked := s.MarkDoneOwned("u1", id); !found || !owned || !locked {
			t.Fatalf("%s must be locked: %v %v %v", id, found, owned, locked)
		}
		if s.MarkDone("u1", id) {
			t.Fatalf("%s: MarkDone must refuse locked step", id)
		}
	}
	if !s.MarkDone("u1", "p") || !s.MarkDone("u1", "p") {
		t.Fatal("plain step must be closable (idempotent)")
	}
	// После одобрения заявки шаг доступа закрыт — и повторная отметка не считается ошибкой.
	s.LinkStep("u1", "access", "REQ-1", StepDone)
	if _, _, locked := s.MarkDoneOwned("u1", "acc"); locked {
		t.Fatal("already done step is not locked")
	}
}

func TestValidateAction(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	long := make([]rune, 501)
	for i := range long {
		long[i] = 'я'
	}
	bad := []Absence{
		{Type: ActionAbsence, DateFrom: "abc", DateTo: "2026-11-20"},
		{Type: ActionAbsence, DateFrom: "2026-11-20", DateTo: "2026-11-19"},
		{Type: ActionAbsence, DateFrom: "2026-11-01", DateTo: "2026-11-02"},
		{Type: ActionAbsence, DateFrom: "2026-12-01", DateTo: "2099-12-31"},
		{Type: ActionAbsence, DateFrom: "2026-11-20", DateTo: "2026-11-20", Reason: string(long)},
		{Type: ActionAccess},
		{Type: ActionAccess, Resource: "<img src=x onerror=alert(1)>"},
		{Type: "salary"},
	}
	for _, a := range bad {
		if err := ValidateAction(a, now); err == nil {
			t.Fatalf("%+v must be invalid", a)
		}
	}
	for _, a := range []Absence{
		{Type: ActionAbsence, DateFrom: "2026-11-11", DateTo: "2026-12-11"},
		{Type: ActionAccess, Resource: NormalizeResource(" GitLab ")},
	} {
		if err := ValidateAction(a, now); err != nil {
			t.Fatalf("%+v: %v", a, err)
		}
	}
}

func TestCreateDraftLimit(t *testing.T) {
	s := New()
	for i := 0; i < MaxOpenDrafts; i++ {
		if _, err := s.CreateDraft(Absence{ID: s.NextID("REQ"), UserID: "u1", Type: ActionAccess, Resource: "jira"}); err != nil {
			t.Fatalf("draft %d: %v", i, err)
		}
	}
	if _, err := s.CreateDraft(Absence{ID: "X", UserID: "u1"}); err != ErrTooManyDrafts {
		t.Fatalf("want ErrTooManyDrafts, got %v", err)
	}
	// Лимит личный: другой пользователь черновик создаёт.
	if a, err := s.CreateDraft(Absence{ID: "Y", UserID: "u2"}); err != nil || a.Status != StatusDraft || len(a.History) != 1 {
		t.Fatalf("other user: %+v %v", a, err)
	}
	// Подтверждённый черновик освобождает место.
	s.Transition("REQ-000001", StatusDraft, StatusPending)
	if _, err := s.CreateDraft(Absence{ID: "Z", UserID: "u1"}); err != nil {
		t.Fatalf("after confirm: %v", err)
	}
}
