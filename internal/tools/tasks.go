package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/tasks"
)

// TasksRecommend — «какую задачу мне взять?»: топ задач адаптации от модели рекомендаций
// с объяснением. Только чтение: взять задачу стажёр решает сам кнопкой в разделе «Задачи».
type TasksRecommend struct {
	Svc *tasks.Service
}

func (t *TasksRecommend) Name() string               { return "tasks_recommend" }
func (t *TasksRecommend) RequiredPermission() string { return "tasks.read" }
func (t *TasksRecommend) RequiresConfirmation() bool { return false }

func (t *TasksRecommend) Run(_ context.Context, u auth.User, _ map[string]string) (Result, error) {
	if !auth.Can(u, t.RequiredPermission()) {
		return Result{}, ErrForbidden
	}
	if u.Role != auth.RoleIntern {
		return Result{OK: false, Message: "Рекомендации задач — для стажёров. Задачи стажёра видны в его карточке в кабинете команды."}, nil
	}
	rec := t.Svc.Recommend(u.ID, u.TeamID, 3)
	if len(rec.Items) == 0 {
		return Result{OK: true, Message: "Все задачи каталога уже взяты или сделаны — спросите наставника о следующей", Data: map[string]any{"tasks": rec.Items}}, nil
	}
	top := rec.Items[0]
	others := make([]string, 0, 2)
	for _, it := range rec.Items[1:] {
		others = append(others, it.Title)
	}
	data := map[string]any{
		"tasks":       rec.Items,
		"source":      rec.Source,
		"recommended": top.Title,
		"task_id":     top.ID,
		"why":         strings.Join(top.Reasons, "; "),
		"difficulty":  fmt.Sprintf("%d из 5", top.Difficulty),
		"estimate":    fmt.Sprintf("≈ %d ч", top.EstimateHours),
	}
	if len(others) > 0 {
		data["alternatives"] = strings.Join(others, "; ")
	}
	return Result{OK: true, Message: "Советую взять: " + top.Title, Data: data}, nil
}
