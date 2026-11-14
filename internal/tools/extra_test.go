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

// R2-5/R2-6: имена — по команде пользователя; наставнику не советуем предупреждать наставника.
func TestAbsenceNamesByTeam(t *testing.T) {
	fe := auth.User{ID: "intern-2", Role: auth.RoleIntern, TeamID: "frontend"}
	res, _ := (&AbsenceGetPolicy{}).Run(context.Background(), fe, nil)
	if res.Data["buddy_name"] != "Анна Козлова" || res.Data["manager_name"] != "Дмитрий Орлов" || res.Data["suggest_notify_buddy"] != true {
		t.Fatalf("frontend policy: %+v", res.Data)
	}
	mentor := auth.User{ID: "mentor-1", Role: auth.RoleBuddy, TeamID: "backend"}
	res, _ = (&AbsenceGetPolicy{}).Run(context.Background(), mentor, nil)
	if res.Data["buddy_name"] != "Иван Петров" || res.Data["suggest_notify_buddy"] != false {
		t.Fatalf("mentor policy: %+v", res.Data)
	}
	res, err := (&AbsenceCreateRequest{S: store.New()}).Run(context.Background(), fe, map[string]string{"date_from": "2030-01-15", "date_to": "2030-01-15"})
	if err != nil || res.Data["manager_name"] != "Дмитрий Орлов" || res.Data["buddy_name"] != "Анна Козлова" {
		t.Fatalf("frontend absence: %+v %v", res, err)
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
