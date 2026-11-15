package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMeetingsLifecycle(t *testing.T) {
	s := New()
	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	m := s.CreateMeeting(Meeting{OwnerID: "intern-1", Title: "Разбор задачи", Start: base.Add(2 * time.Hour), DurationMin: 30})
	s.CreateMeeting(Meeting{OwnerID: "intern-1", Title: "Раньше", Start: base.Add(time.Hour), DurationMin: 15})
	s.CreateMeeting(Meeting{OwnerID: "intern-1", Title: "Прошлый", Start: base.Add(-time.Hour), DurationMin: 15})
	s.CreateMeeting(Meeting{OwnerID: "intern-3", Title: "Чужой", Start: base.Add(time.Hour), DurationMin: 15})

	if m.ID == "" || !m.CreatedAt.Equal(base) {
		t.Fatalf("created: %+v", m)
	}
	list := s.MeetingsByOwner("intern-1")
	if len(list) != 3 || list[0].Title != "Прошлый" || list[1].Title != "Раньше" {
		t.Fatalf("sorted by start: %+v", list)
	}
	if n := s.CountUpcomingMeetings("intern-1", base); n != 2 {
		t.Fatalf("upcoming: %d", n)
	}

	// чужой созвон отменить нельзя и нельзя узнать, что он существует
	if found, owned := s.CancelMeeting(m.ID, "intern-3"); found || owned {
		t.Fatal("foreign cancel must look like not found")
	}
	if found, owned := s.CancelMeeting(m.ID, "intern-1"); !found || !owned {
		t.Fatal("owner cancel")
	}
	if found, _ := s.CancelMeeting(m.ID, "intern-1"); found {
		t.Fatal("second cancel is not found")
	}
	if found, _ := s.CancelMeeting("MTG-404", "intern-1"); found {
		t.Fatal("unknown id")
	}
	if len(s.MeetingsByOwner("intern-1")) != 2 {
		t.Fatal("cancelled meeting must disappear")
	}
}

func TestReminderAcksArePrunedAndPersisted(t *testing.T) {
	s := New()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	s.AckReminder("intern-1", "standup@2026-11-16T11:00")
	if !s.ReminderAcked("intern-1", "standup@2026-11-16T11:00") || s.ReminderAcked("intern-3", "standup@2026-11-16T11:00") {
		t.Fatal("acks are per user")
	}
	// через три дня старая отметка удаляется при следующей
	now = now.Add(72 * time.Hour)
	s.AckReminder("intern-1", "standup@2026-11-19T11:00")
	if s.ReminderAcked("intern-1", "standup@2026-11-16T11:00") {
		t.Fatal("old ack must be pruned")
	}

	s.CreateMeeting(Meeting{OwnerID: "intern-1", Title: "Созвон", Start: now.Add(time.Hour), DurationMin: 30})
	path := filepath.Join(t.TempDir(), "state.json")
	p := &Persister{Path: path, Store: s, Msgs: NewMessageStore()}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	restored := New()
	if err := (&Persister{Path: path, Store: restored, Msgs: NewMessageStore()}).Load(); err != nil {
		t.Fatal(err)
	}
	if len(restored.MeetingsByOwner("intern-1")) != 1 || !restored.ReminderAcked("intern-1", "standup@2026-11-19T11:00") {
		t.Fatal("meetings and reminder acks must survive restart")
	}
}
