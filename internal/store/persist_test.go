package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func filled(t *testing.T) (*Store, *MessageStore) {
	t.Helper()
	s, m := New(), NewMessageStore()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "s1", Title: "Доступы", Kind: "access"}, {ID: "s2", Title: "Playbook"}})
	s.CreateAbsence(Absence{ID: s.NextID("REQ"), Type: ActionAccess, Resource: "gitlab", UserID: "u1", TeamID: "backend", Status: StatusDraft})
	s.TransitionBy("REQ-000001", []string{StatusDraft}, StatusPending, "u1", "")
	s.LinkStep("u1", "access", "REQ-000001", StepWaiting)
	s.RecordGap("backend", "Сколько платят?")
	m.CreateSession("u1", "sess")
	m.CreateMessage(Message{ID: "m1", SessionID: "sess", Role: "user", Content: "привет", Status: "done"})
	m.CreateMessage(Message{ID: "m2", SessionID: "sess", Role: "assistant", Status: "in_progress"})
	return s, m
}

// Перезапуск сервера не теряет заявки, историю, чек-лист, пробелы базы и диалоги.
func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "state.json")
	s, m := filled(t)
	if err := (&Persister{Path: path, Store: s, Msgs: m}).Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file must be renamed away")
	}

	s2, m2 := New(), NewMessageStore()
	if err := (&Persister{Path: path, Store: s2, Msgs: m2}).Load(); err != nil {
		t.Fatal(err)
	}
	a, ok := s2.GetAbsence("REQ-000001")
	if !ok || a.Status != StatusPending || a.Resource != "gitlab" || len(a.History) != 2 {
		t.Fatalf("action lost: %+v", a)
	}
	if id := s2.NextID("REQ"); id != "REQ-000002" {
		t.Fatalf("sequence must continue, got %s", id)
	}
	items := s2.Checklist("u1")
	if len(items) != 2 || items[0].Status != StepWaiting || items[0].ActionID != "REQ-000001" {
		t.Fatalf("checklist lost: %+v", items)
	}
	if g := s2.Gaps("backend"); len(g) != 1 || g[0].Count != 1 {
		t.Fatalf("gaps lost: %+v", g)
	}
	if sess, ok := m2.GetSession("sess"); !ok || sess.UserID != "u1" {
		t.Fatal("session lost")
	}
	msgs := m2.MessagesBySession("sess")
	if len(msgs) != 2 || msgs[0].Content != "привет" {
		t.Fatalf("messages lost: %+v", msgs)
	}
	// Ответ, не досчитанный до остановки, помечается failed, а не висит вечно.
	if msgs[1].Status != "failed" || msgs[1].Content == "" {
		t.Fatalf("in-flight answer must fail: %+v", msgs[1])
	}
}

func TestLoadEdgeCases(t *testing.T) {
	dir := t.TempDir()
	p := &Persister{Path: filepath.Join(dir, "missing.json"), Store: New(), Msgs: NewMessageStore()}
	if err := p.Load(); err != nil {
		t.Fatalf("missing file is a clean start, got %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte("{not json"), 0o600)
	if err := (&Persister{Path: bad, Store: New(), Msgs: NewMessageStore()}).Load(); err == nil {
		t.Fatal("corrupted snapshot must return error")
	}
	old := filepath.Join(dir, "old.json")
	_ = os.WriteFile(old, []byte(`{"v":99}`), 0o600)
	if err := (&Persister{Path: old, Store: New(), Msgs: NewMessageStore()}).Load(); err == nil {
		t.Fatal("unknown version must return error")
	}
	if err := (&Persister{Path: dir, Store: New(), Msgs: NewMessageStore()}).Load(); err == nil {
		t.Fatal("directory instead of file must return error")
	}
}

func TestRunSavesOnChangeAndOnShutdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, m := New(), NewMessageStore()
	p := &Persister{Path: path, Store: s, Msgs: m}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); p.Run(ctx, 5*time.Millisecond) }()

	s.CreateAbsence(Absence{ID: "ABS-1", UserID: "u1", Status: StatusDraft})
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("periodic save did not happen")
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.CreateSession("u1", "late") // изменение прямо перед остановкой
	cancel()
	<-done

	s2, m2 := New(), NewMessageStore()
	if err := (&Persister{Path: path, Store: s2, Msgs: m2}).Load(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.GetAbsence("ABS-1"); !ok {
		t.Fatal("action not saved")
	}
	if _, ok := m2.GetSession("late"); !ok {
		t.Fatal("final save on shutdown missed the last change")
	}
}

func TestSaveToUnwritablePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(file, []byte("x"), 0o600)
	// Родитель — обычный файл, каталог создать нельзя.
	p := &Persister{Path: filepath.Join(file, "sub", "state.json"), Store: New(), Msgs: NewMessageStore()}
	if err := p.Save(); err == nil {
		t.Fatal("want error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.Run(ctx, time.Millisecond) // ошибка сохранения логируется, а не паникует
}

func TestLifecycleStoreOps(t *testing.T) {
	s := New()
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { now = now.Add(time.Minute); return now }

	a := s.CreateAbsence(Absence{ID: "A1", UserID: "u1", TeamID: "backend", Status: StatusDraft})
	if len(a.History) != 1 || a.UpdatedAt.IsZero() {
		t.Fatalf("history must start at creation: %+v", a)
	}
	a.History[0].Status = "hacked" // копия: наружу не утекает внутренний слайс
	if got, _ := s.GetAbsence("A1"); got.History[0].Status != StatusDraft {
		t.Fatal("history slice leaked")
	}
	if _, ok := s.TransitionBy("A1", []string{StatusPending}, StatusApproved, "lead", ""); ok {
		t.Fatal("draft cannot be approved")
	}
	if _, ok := s.TransitionBy("nope", []string{StatusDraft}, StatusPending, "", ""); ok {
		t.Fatal("missing action")
	}
	s.TransitionBy("A1", []string{StatusDraft}, StatusPending, "", "")
	upd, ok := s.TransitionBy("A1", []string{StatusPending}, StatusApproved, "lead", "ок")
	if !ok || len(upd.History) != 3 || upd.History[1].By != "u1" || upd.History[2].Comment != "ок" || !upd.UpdatedAt.After(upd.CreatedAt) {
		t.Fatalf("history: %+v", upd)
	}

	s.CreateAbsence(Absence{ID: "A2", UserID: "u2", TeamID: "backend", Status: StatusDraft})
	s.CreateAbsence(Absence{ID: "A3", UserID: "u3", TeamID: "frontend", Status: StatusPending})
	if got := s.ListByTeam("backend", ""); len(got) != 1 || got[0].ID != "A1" {
		t.Fatalf("team list must hide drafts and other teams: %+v", got)
	}
	if got := s.ListByTeam("backend", StatusDraft); len(got) != 1 || got[0].ID != "A2" {
		t.Fatalf("status filter: %+v", got)
	}
}

func TestLinkAndUnlinkStep(t *testing.T) {
	s := New()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "s0", Kind: "access", Done: true}, {ID: "s1", Kind: "access"}, {ID: "s2"}})
	if !s.HasChecklist("u1") || s.HasChecklist("u2") {
		t.Fatal("HasChecklist")
	}
	it, ok := s.LinkStep("u1", "access", "REQ-1", StepWaiting)
	if !ok || it.ID != "s1" || it.Status != StepWaiting || it.Done {
		t.Fatalf("must link first unfinished access step: %+v", it)
	}
	s.UnlinkStep("u1", "REQ-other") // чужая заявка шаг не трогает
	if s.Checklist("u1")[1].Status != StepWaiting {
		t.Fatal("unlink by wrong action")
	}
	s.UnlinkStep("u1", "REQ-1")
	if got := s.Checklist("u1")[1]; got.Status != StepTodo || got.ActionID != "" {
		t.Fatalf("unlink: %+v", got)
	}
	if it, _ := s.LinkStep("u1", "access", "REQ-2", StepDone); !it.Done {
		t.Fatal("done link")
	}
	if _, ok := s.LinkStep("u1", "access", "REQ-3", StepWaiting); ok {
		t.Fatal("no unfinished access steps left")
	}
	if _, ok := s.LinkStep("nobody", "access", "REQ-4", StepWaiting); ok {
		t.Fatal("unknown user")
	}
}

func TestGaps(t *testing.T) {
	s := New()
	s.RecordGap("backend", "  Сколько   платят? ")
	s.RecordGap("backend", "сколько платят")
	s.RecordGap("backend", "Где парковка?")
	s.RecordGap("backend", "   ")
	s.RecordGap("frontend", "Где столовая?")
	long := make([]rune, maxGapLen+50)
	for i := range long {
		long[i] = 'я'
	}
	s.RecordGap("backend", string(long))

	g := s.Gaps("backend")
	if len(g) != 3 || g[0].Count != 2 || g[0].Question != "Сколько   платят?" {
		t.Fatalf("gaps: %+v", g)
	}
	for _, x := range g {
		if len([]rune(x.Question)) > maxGapLen+1 {
			t.Fatalf("question not truncated: %d", len([]rune(x.Question)))
		}
	}
	if len(s.Gaps("nobody")) != 0 {
		t.Fatal("unknown team")
	}
	// Ограничение роста: больше maxGapsPerTeam разных вопросов не копим.
	for i := 0; i < maxGapsPerTeam+50; i++ {
		s.RecordGap("flood", string(rune('а'+i%30))+string(rune('0'+i%10))+string(rune('a'+i/30)))
	}
	if n := len(s.Gaps("flood")); n > maxGapsPerTeam {
		t.Fatalf("gaps unbounded: %d", n)
	}
}

func TestPendingCount(t *testing.T) {
	m := NewMessageStore()
	m.CreateMessage(Message{ID: "1", SessionID: "s", Role: "assistant", Status: "pending"})
	m.CreateMessage(Message{ID: "2", SessionID: "s", Role: "assistant", Status: "in_progress"})
	m.CreateMessage(Message{ID: "3", SessionID: "s", Role: "assistant", Status: "done"})
	m.CreateMessage(Message{ID: "4", SessionID: "s", Role: "user", Status: "pending"})
	if n := m.PendingCount("s"); n != 2 {
		t.Fatalf("want 2, got %d", n)
	}
}
