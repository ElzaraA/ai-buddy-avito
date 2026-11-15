package tools

import (
	"context"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

type AbsenceGetPolicy struct{}

func (t *AbsenceGetPolicy) Name() string               { return "absence_get_policy" }
func (t *AbsenceGetPolicy) RequiredPermission() string { return "checklist.read" }
func (t *AbsenceGetPolicy) RequiresConfirmation() bool { return false }

func (t *AbsenceGetPolicy) Run(ctx context.Context, u auth.User, _ map[string]string) (Result, error) {
	if !auth.Can(u, t.RequiredPermission()) {
		return Result{}, ErrForbidden
	}
	// Имена — по команде пользователя; наставнику и руководителю не советуем предупреждать наставника.
	c := auth.TeamContacts(u.TeamID)
	return Result{
		OK:      true,
		Message: "Порядок оформления отгула",
		Data: map[string]any{
			// Порядок по шагам (тот же, что в базе знаний, checklist.md «Отгул») — простыми словами для стажёра.
			"policy": "1. Договоритесь о датах с руководителем.\n" +
				"2. Создайте заявку в системе отсутствий: напишите, например, «Хочу отгул в пятницу», и подтвердите черновик.\n" +
				"3. Передумали — отмените заявку в разделе «Мои заявки», пока её не рассмотрели.",
			"manager_name":         c.Manager,
			"buddy_name":           c.Buddy,
			"suggest_notify_buddy": u.Role == auth.RoleIntern,
		},
	}, nil
}
