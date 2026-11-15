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

func TestScheduleAddResolvesDayAndTime(t *testing.T) {
	// воскресенье 15.11.2026, 23:20 по Москве
	now := time.Date(2026, 11, 15, 23, 20, 0, 0, schedule.MSK)
	u := auth.DemoUsers["intern.backend"]
	cases := []struct {
		args  map[string]string
		start string
		dur   int
	}{
		{map[string]string{"time": "15.00", "date": "tomorrow", "title": "Встреча"}, "2026-11-16 15:00", 30},
		{map[string]string{"time": "17:00", "title": "Созвон с Катей"}, "2026-11-16 17:00", 30}, // сегодня уже поздно — завтра
		{map[string]string{"time": "23:45"}, "2026-11-15 23:45", 30},                            // сегодня ещё успеваем
		{map[string]string{"time": "10", "date": "sunday"}, "2026-11-22 10:00", 30},             // воскресенье прошло — следующее
		{map[string]string{"time": "11:30", "date": "friday", "duration_min": "60"}, "2026-11-20 11:30", 60},
		{map[string]string{"time": "9:00", "date": "day_after_tomorrow"}, "2026-11-17 09:00", 30},
		{map[string]string{"time": "15:00", "date": "20.11"}, "2026-11-20 15:00", 30},
		{map[string]string{"time": "15:00", "date": "5.01"}, "2027-01-05 15:00", 30},
		{map[string]string{"time": "15:00", "date": "2026-12-01"}, "2026-12-01 15:00", 30},
		{map[string]string{"in_min": "10"}, "2026-11-15 23:30", 30},
		{map[string]string{"in_min": "60", "duration_min": "45"}, "2026-11-16 00:20", 45},
	}
	for _, c := range cases {
		st := store.New()
		tool := &ScheduleAdd{S: st, Now: func() time.Time { return now }}
		res, err := tool.Run(context.Background(), u, c.args)
		if err != nil || !res.OK {
			t.Fatalf("%v: %v %+v", c.args, err, res)
		}
		m := st.MeetingsByOwner(u.ID)
		if len(m) != 1 || m[0].Start.Format("2006-01-02 15:04") != c.start || m[0].DurationMin != c.dur {
			t.Fatalf("%v: got %+v", c.args, m)
		}
		if !strings.HasPrefix(res.Message, "Добавил в расписание: ") || res.Data["reminder"] != "за 15 мин — в этом чате" {
			t.Fatalf("message: %+v", res)
		}
	}
}

func TestScheduleAddTitleAndErrors(t *testing.T) {
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, schedule.MSK)
	u := auth.DemoUsers["intern.backend"]
	st := store.New()
	tool := &ScheduleAdd{S: st, Now: func() time.Time { return now }, Lead: 10 * time.Minute}

	res, _ := tool.Run(context.Background(), u, map[string]string{"time": "12:00", "with": "Иван Петров"})
	if !res.OK || st.MeetingsByOwner(u.ID)[0].Title != "Созвон с Иван Петров" || res.Data["reminder"] != "за 10 мин — в этом чате" {
		t.Fatalf("default title from with: %+v", res)
	}
	res, _ = tool.Run(context.Background(), u, map[string]string{"time": "13:00"})
	if !res.OK || !strings.HasSuffix(res.Message, "— Созвон") {
		t.Fatalf("default title: %+v", res)
	}

	bad := map[string]map[string]string{
		"Во сколько":          {"date": "tomorrow"},
		"Через сколько":       {"in_min": "0"},
		"Не понял время":      {"time": "25:00"},
		"Не понял дату":       {"time": "15:00", "date": "когда-нибудь"},
		"уже прошло":          {"time": "09:00", "date": "today"},
		"не дальше":           {"time": "15:00", "date": "2027-03-01"},
		"от 5 минут":          {"time": "15:00", "duration_min": "300"},
		"слишком длинное":     {"time": "15:00", "title": strings.Repeat("я", 121)},
		"Не понял дату 31.02": {"time": "15:00", "date": "31.02"},
	}
	for want, args := range bad {
		res, err := tool.Run(context.Background(), u, args)
		if err != nil || res.OK || !strings.Contains(res.Message+" 31.02", strings.TrimSuffix(want, " 31.02")) {
			t.Fatalf("%s: %v %+v", want, err, res)
		}
	}
	for i := 0; i < maxUpcomingMeetings; i++ {
		st.CreateMeeting(store.Meeting{OwnerID: u.ID, Title: "x", Start: now.Add(time.Hour), DurationMin: 30})
	}
	if res, _ := tool.Run(context.Background(), u, map[string]string{"time": "15:00"}); res.OK || !strings.Contains(res.Message, "много") {
		t.Fatalf("limit: %+v", res)
	}
	if res, err := (&ScheduleAdd{S: store.New()}).Run(context.Background(), u, map[string]string{"time": "23:59", "date": "2026-12-31"}); err != nil {
		t.Fatalf("real clock: %v %+v", err, res)
	}
	if tool.Name() != "schedule_add" || tool.RequiredPermission() != "schedule.write" || tool.RequiresConfirmation() {
		t.Fatal("meta")
	}
}
