package llm

import (
	"reflect"
	"testing"
)

func TestScheduleAddArgs(t *testing.T) {
	cases := map[string]map[string]string{
		"добавь в расписание завтра встречу в 15.00":    {"time": "15:00", "date": "tomorrow", "title": "Встреча"},
		"и в расписание добавь созвон с Катей на 17.00": {"time": "17:00", "title": "Созвон с Катей"},
		"и на 16.00 добавь синк":                        {"time": "16:00", "title": "Синк"},
		"запланируй созвон с наставником в пятницу в 11:30 на час": {
			"time": "11:30", "date": "friday", "duration_min": "60", "title": "Созвон с наставником"},
		"поставь встречу 20.11 в 3 дня":          {"time": "15:00", "date": "20.11", "title": "Встреча"},
		"добавь созвон в среду":                  {"date": "wednesday", "title": "Созвон"},
		"запиши 1:1 сегодня в 9 утра на полчаса": {"time": "9:00", "date": "today", "duration_min": "30", "title": "1:1"},
		"добавь звонок с Димой послезавтра в 10 утра на 45 минут": {
			"time": "10:00", "date": "day_after_tomorrow", "duration_min": "45", "title": "Созвон с Димой"},
		"поставь созвон через 10 минут":                {"in_min": "10", "title": "Созвон"},
		"добавь встречу через час на полчаса":          {"in_min": "60", "duration_min": "30", "title": "Встреча"},
		"создай встречу с утра в 9:15 на полтора часа": {"time": "9:15", "duration_min": "90", "title": "Встреча"},
	}
	for q, want := range cases {
		n := Normalize(q)
		w := words(n)
		if !wantsAddMeeting(w) {
			t.Fatalf("%q: not recognised as add", q)
		}
		if got := scheduleAddArgs(n, w); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q:\n got %v\nwant %v", q, got, want)
		}
	}
	for _, q := range []string{"какие у меня встречи завтра?", "покажи расписание", "добавь меня в гитлаб", "когда стендап?"} {
		if wantsAddMeeting(words(Normalize(q))) {
			t.Fatalf("%q must not add a meeting", q)
		}
	}
}
