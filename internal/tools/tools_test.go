package tools

import (
	"context"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

func TestChecklistGet(t *testing.T) {
	st := store.New()
	st.SeedChecklist("u1", []store.ChecklistItem{{ID: "1", Title: "a"}})
	tool := &ChecklistGet{S: st}
	res, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, nil)
	if err != nil || !res.OK {
		t.Fatal("must succeed")
	}
	if tool.Name() != "checklist_get" {
		t.Fatal("wrong name")
	}
}

func TestAbsenceCreateRequest(t *testing.T) {
	st := store.New()
	tool := &AbsenceCreateRequest{S: st}
	res, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, map[string]string{"dateFrom": "2026-11-20", "dateTo": "2026-11-20"})
	if err != nil || !res.OK {
		t.Fatal("must create")
	}
	if tool.Name() != "absence_create_request" {
		t.Fatal("wrong name")
	}
}

func TestAbsenceForbidden(t *testing.T) {
	tool := &AbsenceCreateRequest{S: store.New()}
	_, err := tool.Run(context.Background(), auth.User{Role: auth.RoleHRBP}, map[string]string{"dateFrom": "a", "dateTo": "b"})
	if err != ErrForbidden {
		t.Fatal("must forbid")
	}
}

func TestDirectoryFindOwner(t *testing.T) {
	tool := &DirectoryFindOwner{}
	res, err := tool.Run(context.Background(), auth.User{Role: auth.RoleIntern}, map[string]string{"query": "отгул"})
	if err != nil || !res.OK {
		t.Fatal("must find")
	}
	if tool.Name() != "directory_find_owner" {
		t.Fatal("wrong name")
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(&DirectoryFindOwner{})
	if _, ok := r.Get("directory_find_owner"); !ok {
		t.Fatal("must get")
	}
}
