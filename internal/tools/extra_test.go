package tools

import (
	"context"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

func TestAccessRequest(t *testing.T) {
	tool := &AccessRequest{}
	res, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, map[string]string{"resource": "jira"})
	if err != nil || !res.OK {
		t.Fatal("must create access")
	}
	if res.Data["request_id"] == nil {
		t.Fatal("must have request_id")
	}
	if tool.Name() != "access_request" || !tool.RequiresConfirmation() {
		t.Fatal("wrong meta")
	}
}

func TestAbsenceGetPolicy(t *testing.T) {
	tool := &AbsenceGetPolicy{}
	res, err := tool.Run(context.Background(), auth.User{ID: "u1", Role: auth.RoleIntern}, nil)
	if err != nil || !res.OK {
		t.Fatal("must return policy")
	}
	if res.Data["manager_name"] == nil || res.Data["buddy_name"] == nil {
		t.Fatal("must have names")
	}
	if tool.Name() != "absence_get_policy" || tool.RequiresConfirmation() {
		t.Fatal("wrong meta")
	}
}

func TestTransition(t *testing.T) {
	s := store.New()
	a := s.CreateAbsence(store.Absence{ID: "ABS-1", Status: "draft"})
	upd, ok := s.Transition("ABS-1", "draft", "pending")
	if !ok || upd.Status != "pending" {
		t.Fatal("must transition")
	}
	if _, ok := s.Transition("ABS-1", "draft", "pending"); ok {
		t.Fatal("must not transition twice")
	}
	_ = a
}
