package auth

import (
	"context"
	"testing"
)

func TestCan(t *testing.T) {
	intern := User{Role: RoleIntern}
	if !Can(intern, "absence.create") {
		t.Fatal("intern must create absence")
	}
	hr := User{Role: RoleHRBP}
	if Can(hr, "absence.create") {
		t.Fatal("hr must not create absence")
	}
	if !Can(hr, "absence.read.all") {
		t.Fatal("hr must read all")
	}
	if Can(intern, "absence.read.all") {
		t.Fatal("intern must not read all")
	}
	if !Can(intern, "checklist.read") {
		t.Fatal("everyone reads checklist")
	}
	if !Can(User{Role: RoleLead}, "directory.read") {
		t.Fatal("lead reads directory")
	}
	if Can(User{Role: RoleHRBP}, "directory.read") {
		t.Fatal("hrbp not in directory")
	}
	if !Can(intern, "access.request") || Can(User{Role: RoleLead}, "access.request") {
		t.Fatal("access.request: intern yes, lead no")
	}
	if Can(intern, "unknown.action") {
		t.Fatal("unknown must be forbidden")
	}
}

func TestTeamPermissions(t *testing.T) {
	lead, buddy, intern := User{Role: RoleLead}, User{Role: RoleBuddy}, User{Role: RoleIntern}
	if !Can(lead, "action.approve") || Can(buddy, "action.approve") || Can(intern, "action.approve") {
		t.Fatal("only lead approves")
	}
	if !Can(lead, "team.read") || !Can(buddy, "team.read") || Can(intern, "team.read") {
		t.Fatal("team.read: mentor and lead only")
	}
}

func TestTeamInternsAndByID(t *testing.T) {
	got := TeamInterns("backend")
	if len(got) != 2 || got[0].ID != "intern-1" || got[1].ID != "intern-3" {
		t.Fatalf("backend interns: %+v", got)
	}
	if len(TeamInterns("nobody")) != 0 {
		t.Fatal("unknown team")
	}
	if u, ok := ByID("lead-1"); !ok || u.Role != RoleLead {
		t.Fatal("ByID")
	}
	if _, ok := ByID("ghost"); ok {
		t.Fatal("ByID unknown")
	}
}

// R2-5: у каждой команды свой наставник и руководитель; неизвестная команда — контакты по умолчанию.
func TestTeamContacts(t *testing.T) {
	be, fe := TeamContacts("backend"), TeamContacts("frontend")
	if be.Buddy != "Иван Петров" || be.BuddyDative != "Ивану Петрову" || be.Manager != "Мария Сидорова" {
		t.Fatalf("backend: %+v", be)
	}
	if fe.Buddy != "Анна Козлова" || fe.BuddyDative != "Анне Козловой" || fe.BuddyAccusative != "Анну Козлову" || fe.Manager != "Дмитрий Орлов" {
		t.Fatalf("frontend: %+v", fe)
	}
	if TeamContacts("nobody").Buddy != be.Buddy {
		t.Fatal("unknown team falls back to default")
	}
	if got := ForeignContactStems("frontend"); len(got) != 2 || got[0] != "Петров" {
		t.Fatalf("foreign stems: %v", got)
	}
	if u := DemoUsers["lead.frontend"]; u.ID != "lead-2" || u.Role != RoleLead || u.TeamID != "frontend" {
		t.Fatalf("lead.frontend: %+v", u)
	}
}

func TestContext(t *testing.T) {
	u := User{ID: "1", Role: RoleIntern}
	ctx := WithUser(context.Background(), u)
	got, ok := FromCtx(ctx)
	if !ok || got.ID != "1" {
		t.Fatal("must roundtrip")
	}
	if _, ok := FromCtx(context.Background()); ok {
		t.Fatal("empty ctx must fail")
	}
}
