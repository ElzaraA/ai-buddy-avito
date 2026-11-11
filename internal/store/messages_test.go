package store

import "testing"

func TestMessageStore(t *testing.T) {
	m := NewMessageStore()
	s := m.CreateSession("u1", "s1")
	if s.ID != "s1" || s.UserID != "u1" {
		t.Fatal("wrong session")
	}
	if _, ok := m.GetSession("s1"); !ok {
		t.Fatal("must find session")
	}
	if _, ok := m.GetSession("nope"); ok {
		t.Fatal("must not find")
	}

	msg := m.CreateMessage(Message{ID: "m1", SessionID: "s1", Role: "user", Content: "hi"})
	if msg.ID != "m1" {
		t.Fatal("wrong msg")
	}
	upd, ok := m.UpdateMessage("m1", func(x *Message) { x.Status = "done" })
	if !ok || upd.Status != "done" {
		t.Fatal("must update")
	}
	if _, ok := m.UpdateMessage("nope", func(x *Message) {}); ok {
		t.Fatal("must not update missing")
	}
	if _, ok := m.GetMessage("m1"); !ok {
		t.Fatal("must get")
	}
	if got := m.MessagesBySession("s1"); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}
