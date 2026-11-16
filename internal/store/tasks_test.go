package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMoveTaskTransitionsAndPersistence(t *testing.T) {
	s := New()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	if _, ok := s.MoveTask("u", "t1", TaskDone, TaskInProgress); ok {
		t.Fatal("done requires in_progress")
	}
	st, ok := s.MoveTask("u", "t1", TaskInProgress, "")
	if !ok || !st.StartedAt.Equal(now) || s.CountTasks("u", TaskInProgress) != 1 {
		t.Fatalf("start: %+v", st)
	}
	now = now.Add(time.Hour)
	st, _ = s.MoveTask("u", "t1", TaskDone, TaskInProgress)
	if !st.DoneAt.Equal(now) || st.StartedAt.Equal(now) {
		t.Fatalf("done keeps start: %+v", st)
	}
	// сразу «сделано» без старта — старт = окончание
	s.MoveTask("u", "t2", TaskSkipped, "")
	st, _ = s.MoveTask("u", "t2", TaskDone, TaskSkipped)
	if st.StartedAt.IsZero() {
		t.Fatal("start filled")
	}
	s.MoveTask("u", "t3", TaskSkipped, "")
	if st, ok := s.MoveTask("u", "t3", "", TaskSkipped); !ok || st.Status != "" || len(s.TaskStates("u")) != 2 {
		t.Fatal("reopen removes state")
	}
	if h := s.TaskHistory("u"); len(h) != 2 {
		t.Fatalf("history: %+v", h)
	}

	path := filepath.Join(t.TempDir(), "state.json")
	if err := (&Persister{Path: path, Store: s, Msgs: NewMessageStore()}).Save(); err != nil {
		t.Fatal(err)
	}
	restored := New()
	if err := (&Persister{Path: path, Store: restored, Msgs: NewMessageStore()}).Load(); err != nil {
		t.Fatal(err)
	}
	if restored.TaskStates("u")["t1"].Status != TaskDone {
		t.Fatal("task states survive restart")
	}
	restored.Reset()
	if len(restored.TaskStates("u")) != 0 {
		t.Fatal("reset clears tasks")
	}
}

func TestSessionsByUser(t *testing.T) {
	m := NewMessageStore()
	t0 := time.Now().Add(time.Hour) // сообщения позже создания диалога
	m.CreateSession("u", "s1")
	m.CreateSession("u", "s-empty")
	m.CreateSession("other", "s3")
	m.CreateMessage(Message{ID: "m1", SessionID: "s1", Role: "user", Content: "как оформить отгул?", CreatedAt: t0})
	m.CreateMessage(Message{ID: "m2", SessionID: "s1", Role: "assistant", Content: "…", CreatedAt: t0.Add(time.Minute)})
	m.CreateMessage(Message{ID: "m3", SessionID: "s3", Role: "user", Content: "чужое", CreatedAt: t0})
	got := m.SessionsByUser("u")
	if len(got) != 1 || got[0].FirstQuestion != "как оформить отгул?" || got[0].Messages != 2 || !got[0].LastAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("%+v", got)
	}
}
