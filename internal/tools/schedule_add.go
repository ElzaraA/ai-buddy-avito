package tools

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/schedule"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// Ограничения — те же, что у POST /api/v1/schedule.
const (
	maxUpcomingMeetings = 50
	maxMeetingAheadDays = 60
)

// ScheduleAdd — добавить личный созвон в своё расписание из чата («добавь созвон с Катей завтра в 17:00»).
// Созвон личный и отменяется в один клик в разделе «Расписание», поэтому подтверждение не нужно.
type ScheduleAdd struct {
	S   *store.Store
	Now func() time.Time // nil — time.Now
	// Lead — за сколько до начала Бадди напомнит; только для текста ответа.
	Lead time.Duration
}

func (t *ScheduleAdd) Name() string               { return "schedule_add" }
func (t *ScheduleAdd) RequiredPermission() string { return "schedule.write" }
func (t *ScheduleAdd) RequiresConfirmation() bool { return false }

func (t *ScheduleAdd) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

var weekdays = map[string]time.Weekday{
	"monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday, "thursday": time.Thursday,
	"friday": time.Friday, "saturday": time.Saturday, "sunday": time.Sunday,
}

var (
	reClock   = regexp.MustCompile(`^(\d{1,2})(?:[:.](\d{2}))?$`)
	reDayMon  = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})$`)
	reISODate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// resolveDay: today | tomorrow | day_after_tomorrow | monday…sunday (ближайший) | ДД.ММ | ГГГГ-ММ-ДД; пусто — "".
func resolveDay(raw string, now time.Time) (time.Time, bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, schedule.MSK)
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "today":
		return today, true
	case "tomorrow":
		return today.AddDate(0, 0, 1), true
	case "day_after_tomorrow":
		return today.AddDate(0, 0, 2), true
	}
	if wd, ok := weekdays[raw]; ok {
		diff := (int(wd) - int(today.Weekday()) + 7) % 7
		return today.AddDate(0, 0, diff), true // сегодняшний день недели — сегодня; прошедшее время сдвинем ниже
	}
	if m := reDayMon.FindStringSubmatch(raw); m != nil {
		d, _ := strconv.Atoi(m[1])
		mon, _ := strconv.Atoi(m[2])
		day := time.Date(today.Year(), time.Month(mon), d, 0, 0, 0, 0, schedule.MSK)
		if day.Day() != d || int(day.Month()) != mon {
			return time.Time{}, false
		}
		if day.Before(today) { // «5.01» в ноябре — это следующий год
			day = day.AddDate(1, 0, 0)
		}
		return day, true
	}
	if reISODate.MatchString(raw) {
		day, err := time.ParseInLocation("2006-01-02", raw, schedule.MSK)
		return day, err == nil
	}
	return time.Time{}, false
}

// withTime — копия args с датой и временем из start (для «через 10 минут»).
func withTime(args map[string]string, start time.Time) map[string]string {
	out := make(map[string]string, len(args)+2)
	for k, v := range args {
		out[k] = v
	}
	out["date"] = start.Format("2006-01-02")
	out["time"] = start.Format("15:04")
	return out
}

// Run: in_min (через сколько минут — вместо date и time), title, time (15:00 / 15.00 / 15), date (см. resolveDay; пусто — сегодня, а если время прошло — завтра),
// duration_min (по умолчанию 30), with (с кем).
func (t *ScheduleAdd) Run(_ context.Context, u auth.User, args map[string]string) (Result, error) {
	if !auth.Can(u, t.RequiredPermission()) {
		return Result{}, ErrForbidden
	}
	now := t.now().In(schedule.MSK)
	fail := func(msg string) (Result, error) { return Result{OK: false, Message: msg}, nil }

	if in := strings.TrimSpace(args["in_min"]); in != "" {
		n, err := strconv.Atoi(in)
		if err != nil || n < 1 || n > 24*60 {
			return fail("Через сколько минут? Например: «добавь созвон через 10 минут».")
		}
		args = withTime(args, now.Add(time.Duration(n)*time.Minute).Truncate(time.Minute))
	}
	m := reClock.FindStringSubmatch(strings.TrimSpace(args["time"]))
	if m == nil {
		return fail("Во сколько поставить встречу? Напишите, например: «добавь созвон завтра в 15:00».")
	}
	hour, _ := strconv.Atoi(m[1])
	minute := 0
	if m[2] != "" {
		minute, _ = strconv.Atoi(m[2])
	}
	if hour > 23 || minute > 59 {
		return fail("Не понял время. Напишите, например: «в 15:00».")
	}

	dateArg := strings.TrimSpace(args["date"])
	day, ok := resolveDay(dateArg, now)
	if dateArg == "" {
		day, ok = resolveDay("today", now)
	}
	if !ok {
		return fail("Не понял дату. Напишите «сегодня», «завтра», день недели или число, например 20.11.")
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, schedule.MSK)
	if !start.After(now) {
		_, weekday := weekdays[strings.ToLower(dateArg)]
		switch {
		case dateArg == "": // «созвон в 17:00» поздно вечером — значит, завтра
			start = start.AddDate(0, 0, 1)
		case weekday: // «в среду в 10:00», а среда уже прошла — следующая неделя
			start = start.AddDate(0, 0, 7)
		default:
			return fail("Это время уже прошло — выберите время позже.")
		}
	}
	if start.After(now.AddDate(0, 0, maxMeetingAheadDays)) {
		return fail(fmt.Sprintf("Созвон можно поставить не дальше чем на %d дней вперёд.", maxMeetingAheadDays))
	}

	duration := 30
	if d := strings.TrimSpace(args["duration_min"]); d != "" {
		n, err := strconv.Atoi(d)
		if err != nil || n < 5 || n > 240 {
			return fail("Длительность — от 5 минут до 4 часов.")
		}
		duration = n
	}
	with := strings.TrimSpace(args["with"])
	title := strings.TrimSpace(args["title"])
	if title == "" {
		title = "Созвон"
		if with != "" {
			title = "Созвон с " + with
		}
	}
	if utf8.RuneCountInString(title) > 120 || utf8.RuneCountInString(with) > 120 {
		return fail("Название слишком длинное — до 120 символов.")
	}
	if t.S.CountUpcomingMeetings(u.ID, now) >= maxUpcomingMeetings {
		return fail("В расписании уже много будущих созвонов — отмените лишние в разделе «Расписание».")
	}

	m2 := t.S.CreateMeeting(store.Meeting{OwnerID: u.ID, Title: title, Start: start, DurationMin: duration, With: with})
	lead := t.Lead
	if lead <= 0 {
		lead = 15 * time.Minute
	}
	label := schedule.Label(schedule.Occurrence{Title: title, Start: start})
	return Result{
		OK:      true,
		Message: "Добавил в расписание: " + label,
		Data: map[string]any{
			"meeting_id": m2.ID,
			"meeting":    label,
			"duration":   fmt.Sprintf("%d мин", duration),
			"reminder":   fmt.Sprintf("за %d мин — в этом чате", int(lead.Minutes())),
		},
	}, nil
}
