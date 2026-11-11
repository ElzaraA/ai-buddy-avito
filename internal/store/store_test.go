package store

import "testing"

func TestChecklist(t *testing.T) {
	s := New()
	s.SeedChecklist("u1", []ChecklistItem{{ID: "1", Title: "a"}, {ID: "2", Title: "b"}})
	if !s.MarkDone("u1", "2") {
		t.Fatal("must mark")
	}
	items := s.Checklist("u1")
	if !items[1].Done {
		t.Fatal("must be done")
	}
	if s.MarkDone("u1", "999") {
		t.Fatal("unknown id must not mark")
	}
}

func TestAbsence(t *testing.T) {
	s := New()
	a := s.CreateAbsence(Absence{ID: "a1", UserID: "u1"})
	if a.ID != "a1" {
		t.Fatal("wrong id")
	}
	got, ok := s.GetAbsence("a1")
	if !ok || got.UserID != "u1" {
		t.Fatal("must find absence")
	}
}
