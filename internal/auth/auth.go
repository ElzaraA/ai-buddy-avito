package auth

import "context"

type Role string

const (
	RoleIntern Role = "intern"
	RoleBuddy  Role = "buddy"
	RoleLead   Role = "lead"
	RoleHRBP   Role = "hr_bp"
)

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   Role   `json:"role"`
	TeamID string `json:"team_id"`
}

type ctxKey struct{}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func FromCtx(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// DemoUsers — тестовые учётки для проверки прав (X-Demo-User).
var DemoUsers = map[string]User{
	"intern.backend": {ID: "intern-1", Name: "Стажёр Backend", Role: RoleIntern, TeamID: "backend"},
	"mentor.backend": {ID: "mentor-1", Name: "Наставник Backend", Role: RoleBuddy, TeamID: "backend"},
	"intern.other":   {ID: "intern-2", Name: "Стажёр другой команды", Role: RoleIntern, TeamID: "frontend"},
	"lead.backend":   {ID: "lead-1", Name: "Руководитель Backend", Role: RoleLead, TeamID: "backend"},
}

func Can(u User, action string) bool {
	switch action {
	case "checklist.read":
		return true
	case "absence.create":
		return u.Role == RoleIntern || u.Role == RoleBuddy
	case "directory.read":
		return u.Role == RoleIntern || u.Role == RoleBuddy || u.Role == RoleLead
	case "absence.read.all":
		return u.Role == RoleLead || u.Role == RoleHRBP
	default:
		return false
	}
}
