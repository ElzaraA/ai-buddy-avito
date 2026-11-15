package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/schedule"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// понедельник 16.11.2026, 10:45 по Москве — за 15 минут до стендапа
var mondayMorning = time.Date(2026, 11, 16, 10, 45, 0, 0, schedule.MSK)

func scheduleRouter(now *time.Time) (http.Handler, *store.Store) {
	st := store.New()
	h := &Handlers{Store: st, Now: func() time.Time { return *now }}
	return NewRouter(h), st
}

func decodeInto[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

func TestScheduleReturnsTeamRitualsForUser(t *testing.T) {
	now := mondayMorning
	r, _ := scheduleRouter(&now)
	rr := doReq(t, r, http.MethodGet, "/api/v1/schedule", "intern.backend", nil)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	resp := decodeInto[scheduleResponse](t, rr.Body.Bytes())
	if resp.ReminderLeadMin != 15 || len(resp.Items) != 7 || resp.Items[0].Title != "Стендап команды" {
		t.Fatalf("intern week: lead=%d items=%d", resp.ReminderLeadMin, len(resp.Items))
	}

	short := decodeInto[scheduleResponse](t, doReq(t, r, http.MethodGet, "/api/v1/schedule?days=1", "lead.backend", nil).Body.Bytes())
	for _, o := range short.Items {
		if o.Kind == schedule.KindOneOnOne {
			t.Fatal("lead has no 1:1")
		}
	}
	for _, bad := range []string{"0", "32", "abc"} {
		if rr := doReq(t, r, http.MethodGet, "/api/v1/schedule?days="+bad, "intern.backend", nil); rr.Code != 400 {
			t.Fatalf("days=%s: %d", bad, rr.Code)
		}
	}
}

func TestCreateAndCancelPersonalMeeting(t *testing.T) {
	now := mondayMorning
	r, st := scheduleRouter(&now)
	body := `{"title":"Разбор первой задачи","start":"2026-11-16T15:00","duration_min":45,"with":"Иван Петров"}`
	rr := doReq(t, r, http.MethodPost, "/api/v1/schedule", "intern.backend", []byte(body))
	if rr.Code != 201 {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	occ := decodeInto[schedule.Occurrence](t, rr.Body.Bytes())
	if occ.Kind != schedule.KindPersonal || occ.MeetingID == "" || occ.Start.Hour() != 15 ||
		occ.End.Sub(occ.Start) != 45*time.Minute || occ.With != "Иван Петров" {
		t.Fatalf("created: %+v", occ)
	}

	// созвон виден только владельцу
	mine := decodeInto[scheduleResponse](t, doReq(t, r, http.MethodGet, "/api/v1/schedule?days=1", "intern.backend", nil).Body.Bytes())
	other := decodeInto[scheduleResponse](t, doReq(t, r, http.MethodGet, "/api/v1/schedule?days=1", "intern.junior", nil).Body.Bytes())
	if !containsID(mine.Items, occ.ID) || containsID(other.Items, occ.ID) {
		t.Fatal("personal meeting must be visible to its owner only")
	}

	// чужой созвон не отменить — 404, свой — 204, повторно — 404
	if rr := doReq(t, r, http.MethodPost, "/api/v1/schedule/"+occ.MeetingID+"/cancel", "intern.junior", nil); rr.Code != 404 {
		t.Fatalf("foreign cancel: %d", rr.Code)
	}
	if rr := doReq(t, r, http.MethodPost, "/api/v1/schedule/"+occ.MeetingID+"/cancel", "intern.backend", nil); rr.Code != 204 {
		t.Fatalf("own cancel: %d", rr.Code)
	}
	if rr := doReq(t, r, http.MethodPost, "/api/v1/schedule/"+occ.MeetingID+"/cancel", "intern.backend", nil); rr.Code != 404 {
		t.Fatalf("second cancel: %d", rr.Code)
	}
	if len(st.MeetingsByOwner("intern-1")) != 0 {
		t.Fatal("cancelled")
	}

	// RFC3339 и длительность по умолчанию
	rr = doReq(t, r, http.MethodPost, "/api/v1/schedule", "intern.backend", []byte(`{"title":"Кофе","start":"2026-11-17T09:00:00Z"}`))
	occ = decodeInto[schedule.Occurrence](t, rr.Body.Bytes())
	if rr.Code != 201 || occ.End.Sub(occ.Start) != 30*time.Minute || occ.Start.In(schedule.MSK).Hour() != 12 {
		t.Fatalf("rfc3339: %d %+v", rr.Code, occ)
	}
}

func containsID(items []schedule.Occurrence, id string) bool {
	for _, o := range items {
		if o.ID == id {
			return true
		}
	}
	return false
}

func TestCreateMeetingValidation(t *testing.T) {
	now := mondayMorning
	r, st := scheduleRouter(&now)
	long := strings.Repeat("я", 121)
	cases := map[string]string{
		"битый JSON":     `{`,
		"пустое имя":     `{"title":"  ","start":"2026-11-16T15:00"}`,
		"длинное имя":    `{"title":"` + long + `","start":"2026-11-16T15:00"}`,
		"длинное с кем":  `{"title":"a","with":"` + long + `","start":"2026-11-16T15:00"}`,
		"нет времени":    `{"title":"a"}`,
		"кривое время":   `{"title":"a","start":"завтра"}`,
		"прошлое":        `{"title":"a","start":"2026-11-16T10:00"}`,
		"далеко":         `{"title":"a","start":"2027-03-01T10:00"}`,
		"короткий":       `{"title":"a","start":"2026-11-16T15:00","duration_min":3}`,
		"слишком долгий": `{"title":"a","start":"2026-11-16T15:00","duration_min":300}`,
	}
	for name, body := range cases {
		if rr := doReq(t, r, http.MethodPost, "/api/v1/schedule", "intern.backend", []byte(body)); rr.Code != 400 {
			t.Fatalf("%s: want 400, got %d %s", name, rr.Code, rr.Body)
		}
	}
	// лимит будущих созвонов
	for i := 0; i < maxUpcomingMeetings; i++ {
		st.CreateMeeting(store.Meeting{OwnerID: "intern-1", Title: "x", Start: now.Add(time.Hour), DurationMin: 30})
	}
	if rr := doReq(t, r, http.MethodPost, "/api/v1/schedule", "intern.backend", []byte(`{"title":"a","start":"2026-11-16T15:00"}`)); rr.Code != 409 {
		t.Fatalf("limit: %d", rr.Code)
	}
}

func TestRemindersAreShownOnce(t *testing.T) {
	now := mondayMorning
	r, st := scheduleRouter(&now)
	st.CreateMeeting(store.Meeting{OwnerID: "intern-1", Title: "Созвон с наставником", Start: now.Add(10 * time.Minute), DurationMin: 30})

	list := decodeInto[[]reminder](t, doReq(t, r, http.MethodGet, "/api/v1/reminders", "intern.backend", nil).Body.Bytes())
	if len(list) != 2 || list[0].Kind != schedule.KindPersonal || list[1].Kind != schedule.KindStandup {
		t.Fatalf("due: %+v", list)
	}
	if list[1].Text != "Через 15 минут — Стендап команды в 11:00, Яндекс Телемост." {
		t.Fatalf("text: %q", list[1].Text)
	}

	if rr := doReq(t, r, http.MethodPost, "/api/v1/reminders/"+list[1].ID+"/ack", "intern.backend", nil); rr.Code != 204 {
		t.Fatalf("ack: %d", rr.Code)
	}
	again := decodeInto[[]reminder](t, doReq(t, r, http.MethodGet, "/api/v1/reminders", "intern.backend", nil).Body.Bytes())
	if len(again) != 1 || again[0].Kind != schedule.KindPersonal {
		t.Fatalf("acked reminder must not repeat: %+v", again)
	}
	// другой пользователь со своей отметкой не связан
	junior := decodeInto[[]reminder](t, doReq(t, r, http.MethodGet, "/api/v1/reminders", "intern.junior", nil).Body.Bytes())
	if len(junior) != 1 || junior[0].Kind != schedule.KindStandup {
		t.Fatalf("junior: %+v", junior)
	}

	// через час ни о чём напоминать не нужно
	now = now.Add(time.Hour)
	if late := decodeInto[[]reminder](t, doReq(t, r, http.MethodGet, "/api/v1/reminders", "intern.backend", nil).Body.Bytes()); len(late) != 0 {
		t.Fatalf("late: %+v", late)
	}
	if rr := doReq(t, r, http.MethodPost, "/api/v1/reminders/"+strings.Repeat("x", 81)+"/ack", "intern.backend", nil); rr.Code != 400 {
		t.Fatalf("long id: %d", rr.Code)
	}
}

func TestReminderLeadIsConfigurable(t *testing.T) {
	now := mondayMorning.Add(-30 * time.Minute) // 10:15 — до стендапа 45 минут
	st := store.New()
	r := NewRouter(&Handlers{Store: st, Now: func() time.Time { return now }, ReminderLead: time.Hour})
	list := decodeInto[[]reminder](t, doReq(t, r, http.MethodGet, "/api/v1/reminders", "intern.backend", nil).Body.Bytes())
	if len(list) != 1 || !strings.HasPrefix(list[0].Text, "Через 45 минут") {
		t.Fatalf("lead 60 min: %+v", list)
	}
	// без часов в Handlers используется текущее время — запрос просто работает
	if rr := doReq(t, NewRouter(&Handlers{Store: st}), http.MethodGet, "/api/v1/schedule", "intern.backend", nil); rr.Code != 200 {
		t.Fatalf("real clock: %d", rr.Code)
	}
}
