package tools

import (
	"context"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

type ChecklistGet struct{ S *store.Store }

func (t *ChecklistGet) Name() string               { return "checklist_get" }
func (t *ChecklistGet) RequiredPermission() string { return "checklist.read" }
func (t *ChecklistGet) RequiresConfirmation() bool { return false }

func (t *ChecklistGet) Run(ctx context.Context, u auth.User, _ map[string]string) (Result, error) {
	if !auth.Can(u, t.RequiredPermission()) {
		return Result{}, ErrForbidden
	}
	items := t.S.Checklist(u.ID)
	next := ""
	for _, it := range items {
		if !it.Done {
			next = it.Title
			break
		}
	}
	return Result{
		OK:      true,
		Message: "Текущий статус чек-листа",
		Data:    map[string]any{"items": items, "next_step": next},
	}, nil
}
