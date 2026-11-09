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
