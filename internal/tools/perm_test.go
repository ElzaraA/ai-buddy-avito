package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// Каждый инструмент сам проверяет право до выполнения (инвариант 2).
func TestToolsForbidden(t *testing.T) {
	st := store.New()
	nobody := auth.User{ID: "x", Role: auth.Role("guest")}
	for _, tool := range []Tool{
		&AccessRequest{}, &AbsenceCreateRequest{S: st}, &DirectoryFindOwner{},
		&ChecklistGet{S: st}, &AbsenceGetPolicy{},
	} {
		_, err := tool.Run(context.Background(), nobody, map[string]string{"date_from": "2026-11-20", "date_to": "2026-11-20"})
		if tool.Name() == "checklist_get" || tool.Name() == "absence_get_policy" {
			continue // доступны всем ролям
		}
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("%s: want ErrForbidden, got %v", tool.Name(), err)
		}
	}
}

func TestToolMeta(t *testing.T) {
	st := store.New()
	mutating := map[string]bool{"absence_create_request": true, "access_request": true}
	reg := NewRegistry()
	for _, tool := range []Tool{
		&AccessRequest{}, &AbsenceCreateRequest{S: st}, &DirectoryFindOwner{},
		&ChecklistGet{S: st}, &AbsenceGetPolicy{},
	} {
		reg.Register(tool)
		if tool.RequiresConfirmation() != mutating[tool.Name()] {
			t.Fatalf("%s: RequiresConfirmation must be %v", tool.Name(), mutating[tool.Name()])
		}
		if tool.RequiredPermission() == "" {
			t.Fatalf("%s: no permission", tool.Name())
		}
	}
	got, ok := reg.Get("checklist_get")
	if !ok {
		t.Fatal("registered tool not found")
	}
	if res, err := Run(context.Background(), got, auth.User{ID: "u", Role: auth.RoleIntern}, nil); err != nil || !res.OK {
		t.Fatalf("Run: %v %+v", err, res)
	}
}

func TestAbsenceCreateCamelCaseArgs(t *testing.T) {
	st := store.New()
	tool := &AbsenceCreateRequest{S: st}
	u := auth.User{ID: "u1", Role: auth.RoleIntern}
	res, err := tool.Run(context.Background(), u, map[string]string{"dateFrom": "2026-11-20", "dateTo": "2026-11-21"})
	if err != nil || res.Data["dateFrom"] != "2026-11-20" {
		t.Fatalf("camelCase args: %v %+v", err, res)
	}
	if _, err := tool.Run(context.Background(), u, map[string]string{}); err == nil {
		t.Fatal("dates required")
	}
}
