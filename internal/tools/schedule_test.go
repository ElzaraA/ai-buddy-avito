package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/schedule"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

func TestScheduleGetPeriodsAndKinds(t *testing.T) {
	st := store.New()
	// воскресенье 15.11, 22:00 — как сейчас у демо
	now := time.Date(2026, 11, 15, 22, 0, 0, 0, schedule.MSK)
	st.CreateMeeting(store.Meeting{OwnerID: "intern-1", Title: "Разбор задачи", Start: now.Add(13 * time.Hour), DurationMin: 30})
	tool := &ScheduleGet{S: st, Now: func() time.Time { return now }}
	u := auth.DemoUsers["intern.backend"]
	run := func(args map[string]string) Result {
		t.Helper()
		res, err := tool.Run(context.Background(), u, args)
		if err != nil || !res.OK {
			t.Fatalf("%v: %v %+v", args, err, res)
		}
		return res
	}

	today := run(map[string]string{"when": "today"})
	if today.Message != "Встреч на сегодня нет" || today.Data["meetings_count"] != 0 {
		t.Fatalf("sunday evening: %+v", today)
	}

	tomorrow := run(map[string]string{"when": "tomorrow"})
	if tomorrow.Data["meetings_count"] != 2 || tomorrow.Data["next_meeting"] != "пн 16.11, 11:00 — Стендап команды" ||
		tomorrow.Data["place"] != "Яндекс Телемост" || tomorrow.Data["upcoming"] != "пн 16.11, 11:00 — Разбор задачи" {
		t.Fatalf("tomorrow: %+v", tomorrow.Data)
	}

	week := run(map[string]string{})
	if !strings.HasPrefix(week.Message, "Ваше расписание на неделю") || strings.Count(week.Data["upcoming"].(string), ";") != 2 {
		t.Fatalf("week: %+v", week)
	}

	one := run(map[string]string{"kind": "one_on_one"})
	if one.Data["meetings_count"] != 2 || one.Data["next_meeting"] != "ср 18.11, 16:00 — 1:1 с наставником" ||
		!strings.Contains(one.Message, "на две недели") {
		t.Fatalf("1:1: %+v", one)
	}
	if bad := run(map[string]string{"kind": "unknown"}); bad.Data["meetings_count"].(int) < 7 {
		t.Fatalf("unknown kind is ignored: %+v", bad.Data)
	}
	if none := run(map[string]string{"when": "today", "kind": "retro"}); none.Message != "Ретро на сегодня нет" {
		t.Fatalf("no retro today: %q", none.Message)
	}

	// по умолчанию часы — текущее время: просто работает
	if res, err := (&ScheduleGet{S: st}).Run(context.Background(), u, nil); err != nil || !res.OK {
		t.Fatalf("real clock: %v", err)
	}
	if tool.Name() != "schedule_get" || tool.RequiredPermission() != "schedule.read" || tool.RequiresConfirmation() {
		t.Fatal("tool meta")
	}
}
