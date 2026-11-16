package tools

import (
	"context"
	"fmt"
	"strings"

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
	next, how, done := "", "", 0
	for _, it := range items {
		if it.Done {
			done++
			continue
		}
		if next == "" {
			next = it.Title
			how = stepHowTo(it)
		}
	}
	data := map[string]any{"items": items, "next_step": next}
	msg := "Все шаги адаптации пройдены"
	if next != "" {
		msg = "Ваш следующий шаг"
		data["how_to"] = how
		data["progress"] = fmt.Sprintf("%d из %d шагов", done, len(items))
	}
	return Result{OK: true, Message: msg, Data: data}, nil
}

// stepHowTo — короткая подсказка, как закрыть шаг самому, без вопроса наставнику.
func stepHowTo(it store.ChecklistItem) string {
	switch {
	case it.Status == store.StepWaiting:
		return "Заявка уже отправлена и ждёт руководителя. Шаг закроется сам после согласования — ничего делать не нужно."
	case it.Kind == store.ActionAccess && len(it.Missing) > 0 && len(it.Missing) < len(store.AccessSystems):
		// Шаг выполнен частично: называем, каких систем ещё не хватает.
		names := make([]string, 0, len(it.Missing))
		for _, m := range it.Missing {
			names = append(names, store.AccessSystemTitle(m))
		}
		return "Осталось запросить доступ: " + strings.Join(names, ", ") + ". Напишите здесь, например «Запроси доступ в " +
			names[0] + "». Шаг закроется, когда руководитель одобрит доступы ко всем системам (Git, Jira, Grafana)."
	case it.Kind == store.ActionAccess:
		return "Напишите здесь, к какой системе нужен доступ, например «Запроси доступ в GitLab». Помощник заполнит заявку, вы нажмёте «Отправить»."
	default:
		return "Выполните шаг и нажмите «Готово» напротив него в разделе «Мой план». Если непонятно, с чего начать, спросите здесь своими словами."
	}
}
