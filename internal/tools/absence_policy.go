package tools

import (
"context"

"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

type AbsenceGetPolicy struct{}

func (t *AbsenceGetPolicy) Name() string               { return "absence_get_policy" }
func (t *AbsenceGetPolicy) RequiredPermission() string { return "checklist.read" }

func (t *AbsenceGetPolicy) Run(ctx context.Context, u auth.User, _ map[string]string) (Result, error) {
if !auth.Can(u, t.RequiredPermission()) {
return Result{}, ErrForbidden
}
return Result{
OK:      true,
Message: "Порядок оформления отгула",
Data: map[string]any{
"policy":               "Согласовать с руководителем, затем создать заявку в системе отсутствий.",
"manager_name":         "Мария Сидорова",
"buddy_name":           "Иван Петров",
"suggest_notify_buddy": true,
},
}, nil
}