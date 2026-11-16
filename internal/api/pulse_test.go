package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/config"
)

func newAppServer(t *testing.T) (*httptest.Server, *App) {
	t.Helper()
	app, err := Build(config.Config{LLMMock: true}, "../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler)
	t.Cleanup(srv.Close)
	return srv, app
}

type teamPulseView struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Today      *int     `json:"today"`
	Avg7       *float64 `json:"avg7"`
	Trend      string   `json:"trend"`
	Risk       bool     `json:"risk"`
	RiskReason string   `json:"risk_reason"`
	Note       string   `json:"note"`
	Days       []struct {
		Date string `json:"date"`
		Mood *int   `json:"mood"`
	} `json:"days"`
}

func TestPulseEndpoints(t *testing.T) {
	srv, app := newAppServer(t)
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.Local)
	app.Store.SetClock(func() time.Time { return now })

	// Валидация и права.
	for _, body := range []any{`{bad`, map[string]any{"mood": 0}, map[string]any{"mood": 6}, map[string]any{"mood": 3, "note": strings.Repeat("я", 301)}} {
		if res, raw := call(t, "POST", srv.URL+"/api/v1/pulse", "intern.backend", body); res.StatusCode != 400 {
			t.Fatalf("%v: want 400, got %d %s", body, res.StatusCode, raw)
		}
	}
	for _, who := range []string{"mentor.backend", "lead.backend"} {
		if res, _ := call(t, "POST", srv.URL+"/api/v1/pulse", who, map[string]any{"mood": 3}); res.StatusCode != 403 {
			t.Fatalf("%s submit: want 403, got %d", who, res.StatusCode)
		}
		if res, _ := call(t, "GET", srv.URL+"/api/v1/pulse/me", who, nil); res.StatusCode != 403 {
			t.Fatalf("%s pulse/me: want 403", who)
		}
	}

	// Два дня «тяжело/так себе», сегодня переотметка: 1 → 4.
	call(t, "POST", srv.URL+"/api/v1/pulse", "intern.backend", map[string]any{"mood": 2})
	now = now.AddDate(0, 0, 1)
	call(t, "POST", srv.URL+"/api/v1/pulse", "intern.backend", map[string]any{"mood": 1, "note": "не понимаю задачу, тел. +7 999 123-45-67"})
	now = now.AddDate(0, 0, 1)
	if res, raw := call(t, "POST", srv.URL+"/api/v1/pulse", "intern.backend", map[string]any{"mood": 1}); res.StatusCode != 200 {
		t.Fatalf("submit: %d %s", res.StatusCode, raw)
	}
	call(t, "POST", srv.URL+"/api/v1/pulse", "intern.backend", map[string]any{"mood": 4})
	call(t, "POST", srv.URL+"/api/v1/pulse", "intern.other", map[string]any{"mood": 1, "note": "секрет другой команды"})

	_, raw := call(t, "GET", srv.URL+"/api/v1/pulse/me", "intern.backend", nil)
	var me struct {
		Today   string `json:"today"`
		Entries []struct {
			Date string `json:"date"`
			Mood int    `json:"mood"`
		} `json:"entries"`
	}
	_ = json.Unmarshal(raw, &me)
	if len(me.Entries) != 3 || me.Entries[2].Mood != 4 || me.Entries[2].Date != me.Today {
		t.Fatalf("pulse/me: %s", raw)
	}

	// Стажёр не видит пульс команды.
	if res, _ := call(t, "GET", srv.URL+"/api/v1/team/pulse", "intern.backend", nil); res.StatusCode != 403 {
		t.Fatalf("intern team pulse: want 403, got %d", res.StatusCode)
	}
	_, raw = call(t, "GET", srv.URL+"/api/v1/team/pulse", "mentor.backend", nil)
	var team []teamPulseView
	if err := json.Unmarshal(raw, &team); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "intern-2") || strings.Contains(string(raw), "секрет") {
		t.Fatalf("other team leaked: %s", raw)
	}
	if len(team) != 2 || team[0].User.ID != "intern-1" || !team[0].Risk || team[1].Risk {
		t.Fatalf("risk interns first: %s", raw)
	}
	p := team[0]
	if p.Today == nil || *p.Today != 4 || p.Trend != "up" || !strings.Contains(p.RiskReason, "2 из 3") || len(p.Days) != 7 {
		t.Fatalf("intern-1 pulse: %s", raw)
	}
	if strings.Contains(p.Note, "999") || !strings.Contains(p.Note, "не понимаю задачу") {
		t.Fatalf("note must be visible to mentor and masked: %q", p.Note)
	}
	if team[1].Today != nil || team[1].Avg7 != nil {
		t.Fatalf("intern without check-ins: %s", raw)
	}

	// Руководитель другой команды видит только своих.
	_, raw = call(t, "GET", srv.URL+"/api/v1/team/pulse", "lead.frontend", nil)
	if strings.Contains(string(raw), "intern-1") || !strings.Contains(string(raw), "секрет другой команды") {
		t.Fatalf("frontend lead: %s", raw)
	}
	if app.Metrics.Sum("buddy_pulse_checkins_total") != 5 {
		t.Fatalf("metric: %v", app.Metrics.Sum("buddy_pulse_checkins_total"))
	}
}

func TestSummarySavedMinutes(t *testing.T) {
	srv, _ := newAppServer(t)
	ask := func(user, q string) {
		if res, raw := call(t, "POST", srv.URL+"/api/ask", user, map[string]string{"question": q}); res.StatusCode != 200 {
			t.Fatalf("ask: %d %s", res.StatusCode, raw)
		}
	}
	ask("intern.backend", "Какой стек у команды Backend?")       // закрыт
	ask("intern.junior", "Какой стек у команды Backend?")        // закрыт
	ask("intern.backend", "Сколько платят в Авито?")             // отказ
	ask("intern.backend", "передай наставнику: не работает VPN") // сигнал
	ask("mentor.backend", "Какой стек у команды Backend?")       // не стажёр — не считается
	ask("intern.other", "Какой стек у команды Frontend?")        // другая команда
	// Честный учёт: меню, small talk, просмотр чек-листа и черновик не экономят время наставника.
	ask("intern.backend", "помоги")
	ask("intern.backend", "привет")
	ask("intern.backend", "Что мне делать дальше?")
	ask("intern.backend", "Хочу отгул в пятницу")
	// Термин из словаря — засчитывается.
	ask("intern.backend", "Что такое MR?")
	// Подтверждённая заявка засчитывается в момент подтверждения; отменённая — никогда.
	confirmed := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+confirmed.ID+"/confirm", "intern.backend", nil)
	cancelled := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+cancelled.ID+"/confirm", "intern.backend", nil)
	post(t, srv, "/api/v1/actions/"+cancelled.ID+"/cancel", "intern.backend", nil)
	draftOnly := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+draftOnly.ID+"/cancel", "intern.backend", nil)

	_, raw := call(t, "GET", srv.URL+"/api/v1/team/summary", "lead.backend", nil)
	var sum struct {
		QuestionsClosed    int64   `json:"questions_closed"`
		KBAnswers          int64   `json:"kb_answers"`
		GlossaryAnswers    int64   `json:"glossary_answers"`
		ConfirmedActions   int64   `json:"confirmed_actions"`
		SignalsTotal       int64   `json:"signals_total"`
		RefusalsTotal      int64   `json:"refusals_total"`
		MentorMinutesSaved int64   `json:"mentor_minutes_saved"`
		MinutesPerQuestion int     `json:"minutes_per_question"`
		ClosedShare        float64 `json:"closed_share"`
		Interns            int     `json:"interns"`
	}
	_ = json.Unmarshal(raw, &sum)
	// 2 ответа по базе + 1 термин + 1 подтверждённая заявка = 4; подтверждённая и затем отменённая заявка
	// списывается, черновик без подтверждения не считается.
	if sum.KBAnswers != 2 || sum.GlossaryAnswers != 1 || sum.ConfirmedActions != 1 || sum.QuestionsClosed != 4 ||
		sum.SignalsTotal != 1 || sum.RefusalsTotal != 1 || sum.MentorMinutesSaved != 20 ||
		sum.MinutesPerQuestion != 5 || sum.ClosedShare != 0.67 || sum.Interns != 2 {
		t.Fatalf("summary: %s", raw)
	}
}

func TestAchievementsEndpoint(t *testing.T) {
	srv, _ := newAppServer(t)
	if res, _ := call(t, "GET", srv.URL+"/api/v1/achievements", "mentor.backend", nil); res.StatusCode != 403 {
		t.Fatalf("mentor achievements: want 403, got %d", res.StatusCode)
	}
	get := func() map[string]bool {
		_, raw := call(t, "GET", srv.URL+"/api/v1/achievements", "intern.backend", nil)
		var list []struct {
			ID       string `json:"id"`
			Unlocked bool   `json:"unlocked"`
			Hint     string `json:"hint"`
		}
		if err := json.Unmarshal(raw, &list); err != nil || len(list) != 7 {
			t.Fatalf("achievements: %s", raw)
		}
		m := map[string]bool{}
		for _, a := range list {
			m[a.ID] = a.Unlocked
		}
		return m
	}
	// Демо-стажёр: неделя 1 ещё не закрыта (доступы ждут), половины пути нет.
	start := get()
	if start["first_question"] || start["brave_ask"] || start["week1_done"] || start["half_way"] {
		t.Fatalf("start: %v", start)
	}
	call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Какой стек у команды Backend?"})
	call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "передай наставнику: застрял с задачей"})
	a := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	got := get()
	if !got["first_question"] || !got["brave_ask"] || !got["first_action"] || got["pulse_streak"] {
		t.Fatalf("after actions: %v", got)
	}
}
