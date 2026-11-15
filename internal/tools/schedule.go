package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/schedule"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// ScheduleGet — встречи пользователя: ритуалы команды и личные созвоны.
type ScheduleGet struct {
	S   *store.Store
	Now func() time.Time // nil — time.Now
}

func (t *ScheduleGet) Name() string               { return "schedule_get" }
func (t *ScheduleGet) RequiredPermission() string { return "schedule.read" }
func (t *ScheduleGet) RequiresConfirmation() bool { return false }

func (t *ScheduleGet) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// noneOfKind — ответ, когда встреч выбранного вида в периоде нет.
var noneOfKind = map[string]string{
	schedule.KindStandup: "Стендапов", schedule.KindOneOnOne: "Встреч 1:1",
	schedule.KindPlanning: "Планирований", schedule.KindRetro: "Ретро",
}

// Run: when = today | tomorrow | week (по умолчанию — неделя); kind — только встречи одного вида
// (для «когда ретро?» ищем на две недели вперёд: ретро и планирование бывают раз в спринт).
func (t *ScheduleGet) Run(_ context.Context, u auth.User, args map[string]string) (Result, error) {
	if !auth.Can(u, t.RequiredPermission()) {
		return Result{}, ErrForbidden
	}
	now := t.now().In(schedule.MSK)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, schedule.MSK)
	from, to, period := now, now.Add(7*24*time.Hour), "на неделю"
	switch strings.TrimSpace(args["when"]) {
	case "today":
		to, period = dayStart.AddDate(0, 0, 1), "на сегодня"
	case "tomorrow":
		from, to, period = dayStart.AddDate(0, 0, 1), dayStart.AddDate(0, 0, 2), "на завтра"
	}
	kind := strings.TrimSpace(args["kind"])
	if _, known := noneOfKind[kind]; !known {
		kind = ""
	}
	if kind != "" && args["when"] != "today" && args["when"] != "tomorrow" {
		to, period = now.Add(14*24*time.Hour), "на две недели"
	}
	items := schedule.Expand(schedule.TeamRituals(u), t.S.MeetingsByOwner(u.ID), from, to)
	if kind != "" {
		filtered := items[:0]
		for _, o := range items {
			if o.Kind == kind {
				filtered = append(filtered, o)
			}
		}
		items = filtered
	}
	data := map[string]any{"items": items, "meetings_count": len(items)}
	if len(items) == 0 {
		what := "Встреч"
		if kind != "" {
			what = noneOfKind[kind]
		}
		return Result{OK: true, Message: what + " " + period + " нет", Data: data}, nil
	}
	data["next_meeting"] = schedule.Label(items[0])
	if items[0].Place != "" {
		data["place"] = items[0].Place
	}
	// следующие после ближайшей — коротким списком, чтобы ответ в чате оставался читаемым
	labels := make([]string, 0, 3)
	for _, o := range items[1:] {
		if len(labels) == 3 {
			break
		}
		labels = append(labels, schedule.Label(o))
	}
	if len(labels) > 0 {
		data["upcoming"] = strings.Join(labels, "; ")
	}
	msg := fmt.Sprintf("Ваше расписание %s: встреч — %d", period, len(items))
	return Result{OK: true, Message: msg, Data: data}, nil
}
