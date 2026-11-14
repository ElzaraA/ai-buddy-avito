package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

type actionView struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	TeamID  string `json:"team_id"`
	History []struct {
		By      string `json:"by"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	} `json:"history"`
}

func post(t *testing.T, srv *httptest.Server, path, user string, body any) (int, actionView, string) {
	t.Helper()
	res, raw := call(t, "POST", srv.URL+path, user, body)
	var a actionView
	_ = json.Unmarshal(raw, &a)
	return res.StatusCode, a, string(raw)
}

func newDraft(t *testing.T, srv *httptest.Server, user string, body map[string]string) actionView {
	t.Helper()
	code, a, raw := post(t, srv, "/api/v1/actions", user, body)
	if code != 201 {
		t.Fatalf("create: %d %s", code, raw)
	}
	return a
}

var absenceBody = map[string]string{"type": "absence", "date_from": "2030-01-15", "date_to": "2030-01-16"}

// Полный путь заявки: стажёр подтверждает → руководитель согласует; история хранит, кто и что сделал.
func TestAbsenceApprovalFlow(t *testing.T) {
	srv := newChatServer(t)
	a := newDraft(t, srv, "intern.backend", absenceBody)
	if a.TeamID != "backend" || len(a.History) != 1 {
		t.Fatalf("draft: %+v", a)
	}

	// Черновик — личный: руководитель его не видит и согласовать не может.
	if res, _ := call(t, "GET", srv.URL+"/api/v1/actions/"+a.ID, "lead.backend", nil); res.StatusCode != 403 {
		t.Fatalf("lead reads draft: %d", res.StatusCode)
	}
	if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/approve", "lead.backend", nil); code != 409 {
		t.Fatalf("approve draft: want 409, got %d", code)
	}
	if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil); code != 200 {
		t.Fatalf("confirm: %d", code)
	}

	// Кто НЕ может согласовать: сам стажёр, наставник, стажёр/чужая команда.
	for _, who := range []string{"intern.backend", "mentor.backend", "intern.other", "intern.junior"} {
		if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/approve", who, nil); code != 403 {
			t.Fatalf("%s approve: want 403, got %d", who, code)
		}
	}
	// Входящие руководителя и просмотр наставником.
	_, raw := call(t, "GET", srv.URL+"/api/v1/team/actions?status=pending", "lead.backend", nil)
	if !strings.Contains(string(raw), a.ID) {
		t.Fatalf("lead inbox: %s", raw)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/actions/"+a.ID, "mentor.backend", nil); res.StatusCode != 200 {
		t.Fatalf("mentor reads pending: %d", res.StatusCode)
	}

	code, upd, raw2 := post(t, srv, "/api/v1/actions/"+a.ID+"/approve", "lead.backend", map[string]string{"comment": "Хорошего отдыха"})
	if code != 200 || upd.Status != "approved" || len(upd.History) != 3 {
		t.Fatalf("approve: %d %s", code, raw2)
	}
	last := upd.History[2]
	if last.By != "lead-1" || last.Status != "approved" || last.Comment != "Хорошего отдыха" {
		t.Fatalf("history: %+v", upd.History)
	}
	// Решение окончательное.
	for _, path := range []string{"/approve", "/reject"} {
		if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+path, "lead.backend", nil); code != 409 {
			t.Fatalf("%s after decision: %d", path, code)
		}
	}
	if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/cancel", "intern.backend", nil); code != 409 {
		t.Fatalf("cancel after decision: %d", code)
	}
}

func TestRejectAndCancel(t *testing.T) {
	srv := newChatServer(t)
	a := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/reject", "lead.backend", "{bad json"); code != 400 {
		t.Fatalf("bad json: %d", code)
	}
	if code, _, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/reject", "lead.backend", map[string]string{"comment": strings.Repeat("я", 501)}); code != 400 {
		t.Fatalf("long comment: %d", code)
	}
	if code, upd, _ := post(t, srv, "/api/v1/actions/"+a.ID+"/reject", "lead.backend", map[string]string{"comment": "Релиз в эти дни"}); code != 200 || upd.Status != "rejected" {
		t.Fatalf("reject: %d %+v", code, upd)
	}

	b := newDraft(t, srv, "intern.backend", absenceBody)
	if code, _, _ := post(t, srv, "/api/v1/actions/"+b.ID+"/cancel", "intern.other", nil); code != 403 {
		t.Fatalf("foreign cancel: %d", code)
	}
	if code, upd, _ := post(t, srv, "/api/v1/actions/"+b.ID+"/cancel", "intern.backend", nil); code != 200 || upd.Status != "cancelled" {
		t.Fatalf("cancel draft: %d %+v", code, upd)
	}
	if code, _, _ := post(t, srv, "/api/v1/actions/"+b.ID+"/confirm", "intern.backend", nil); code != 409 {
		t.Fatalf("confirm cancelled: %d", code)
	}
	if code, _, _ := post(t, srv, "/api/v1/actions/nope/approve", "lead.backend", nil); code != 404 {
		t.Fatalf("missing: %d", code)
	}
	// Руководитель не согласует собственную заявку (и вообще не может её подтвердить).
	own := newDraft(t, srv, "lead.backend", map[string]string{"type": "access", "resource": "grafana"})
	if code, _, _ := post(t, srv, "/api/v1/actions/"+own.ID+"/approve", "lead.backend", nil); code != 403 {
		t.Fatalf("self-approve: %d", code)
	}
}

func stepStatus(t *testing.T, srv *httptest.Server, user, stepID string) (status, actionID string, percent int) {
	t.Helper()
	_, raw := call(t, "GET", srv.URL+"/api/v1/onboarding/progress", user, nil)
	var p struct {
		Items []struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			ActionID string `json:"related_action_id"`
		} `json:"items"`
		PercentDone int    `json:"percent_done"`
		NextStep    string `json:"next_step"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	for _, it := range p.Items {
		if it.ID == stepID {
			return it.Status, it.ActionID, p.PercentDone
		}
	}
	t.Fatalf("step %s not found in %s", stepID, raw)
	return
}

// S2: заявка на доступ двигает шаг чек-листа: отправлена → waiting, одобрена → done, отклонена → todo.
func TestAccessRequestDrivesChecklistStep(t *testing.T) {
	srv := newChatServer(t)
	const step = "c2-intern-1"
	if st, _, pct := stepStatus(t, srv, "intern.backend", step); st != "todo" || pct != 25 {
		t.Fatalf("initial: %s %d", st, pct)
	}
	a := newDraft(t, srv, "intern.backend", map[string]string{"type": "access", "resource": "gitlab"})
	if st, _, _ := stepStatus(t, srv, "intern.backend", step); st != "todo" {
		t.Fatalf("draft must not move step: %s", st)
	}
	post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	if st, id, _ := stepStatus(t, srv, "intern.backend", step); st != "waiting" || id != a.ID {
		t.Fatalf("after confirm: %s %s", st, id)
	}
	// Стажёр не закрывает ждущий шаг в обход руководителя.
	if res, _ := call(t, "PATCH", srv.URL+"/api/v1/onboarding/steps/"+step, "intern.backend", nil); res.StatusCode != 409 {
		t.Fatalf("manual close of waiting step: want 409, got %d", res.StatusCode)
	}
	post(t, srv, "/api/v1/actions/"+a.ID+"/reject", "lead.backend", nil)
	if st, id, _ := stepStatus(t, srv, "intern.backend", step); st != "todo" || id != "" {
		t.Fatalf("after reject: %s %s", st, id)
	}

	b := newDraft(t, srv, "intern.backend", map[string]string{"type": "access", "resource": "gitlab"})
	post(t, srv, "/api/v1/actions/"+b.ID+"/confirm", "intern.backend", nil)
	post(t, srv, "/api/v1/actions/"+b.ID+"/approve", "lead.backend", nil)
	if st, _, pct := stepStatus(t, srv, "intern.backend", step); st != "done" || pct != 50 {
		t.Fatalf("after approve: %s %d", st, pct)
	}
	// Отгул чек-лист не трогает.
	c := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+c.ID+"/confirm", "intern.backend", nil)
	if _, _, pct := stepStatus(t, srv, "intern.backend", step); pct != 50 {
		t.Fatalf("absence changed checklist: %d", pct)
	}
}

func TestTeamViews(t *testing.T) {
	srv := newChatServer(t)
	paths := []string{"/team/progress", "/team/actions", "/team/knowledge-gaps", "/team/summary"}
	for _, p := range paths {
		if res, _ := call(t, "GET", srv.URL+"/api/v1"+p, "intern.backend", nil); res.StatusCode != 403 {
			t.Fatalf("intern %s: want 403, got %d", p, res.StatusCode)
		}
		if res, _ := call(t, "GET", srv.URL+"/api/v1"+p, "mentor.backend", nil); res.StatusCode != 200 {
			t.Fatalf("mentor %s: %d", p, res.StatusCode)
		}
	}
	// Наставник видит стажёров только своей команды.
	_, raw := call(t, "GET", srv.URL+"/api/v1/team/progress", "mentor.backend", nil)
	s := string(raw)
	if !strings.Contains(s, `"intern-1"`) || !strings.Contains(s, `"intern-3"`) || strings.Contains(s, `"intern-2"`) {
		t.Fatalf("team progress: %s", s)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/team/actions?status=draft", "lead.backend", nil); res.StatusCode != 403 {
		t.Fatalf("drafts must be private: %d", res.StatusCode)
	}

	// Пробелы базы знаний: вопрос без ответа копится обезличенно и виден наставнику команды.
	for i := 0; i < 2; i++ {
		call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Сколько платят в Авито?"})
	}
	call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Какой стек у команды Backend?"}) // есть ответ
	call(t, "POST", srv.URL+"/api/ask", "intern.other", map[string]string{"question": "Где парковка?"})                   // другая команда
	_, raw = call(t, "GET", srv.URL+"/api/v1/team/knowledge-gaps", "mentor.backend", nil)
	var gaps []struct {
		Question string `json:"question"`
		Count    int    `json:"count"`
	}
	_ = json.Unmarshal(raw, &gaps)
	if len(gaps) != 1 || gaps[0].Count != 2 || !strings.Contains(gaps[0].Question, "платят") {
		t.Fatalf("gaps: %s", raw)
	}
	if strings.Contains(string(raw), "intern") {
		t.Fatalf("gaps must be anonymous: %s", raw)
	}

	a := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	_, raw = call(t, "GET", srv.URL+"/api/v1/team/summary", "lead.backend", nil)
	var sum struct {
		Interns            int            `json:"interns"`
		AvgPercentDone     int            `json:"avg_percent_done"`
		Actions            map[string]int `json:"actions"`
		UnansweredTopics   int            `json:"unanswered_topics"`
		UnansweredRequests int            `json:"unanswered_requests"`
	}
	_ = json.Unmarshal(raw, &sum)
	if sum.Interns != 2 || sum.AvgPercentDone != 50 || sum.Actions["pending"] != 1 || sum.UnansweredTopics != 1 || sum.UnansweredRequests != 2 {
		t.Fatalf("summary: %s", raw)
	}
}

func TestPastAndLongAbsenceRejected(t *testing.T) {
	srv := newChatServer(t)
	for _, b := range []map[string]string{
		{"type": "absence", "date_from": "2020-01-01", "date_to": "2020-01-02"},
		{"type": "absence", "date_from": "2030-01-01", "date_to": "2030-03-01"},
		{"type": "absence", "date_from": "2030-01-15", "date_to": "2030-01-15", "reason": strings.Repeat("a", 501)},
	} {
		if code, _, raw := post(t, srv, "/api/v1/actions", "intern.backend", b); code != 400 {
			t.Fatalf("%v: want 400, got %d %s", b, code, raw)
		}
	}
	// Агент тоже не оформляет отгул на прошедшую дату — переспрашивает.
	_, raw := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Хочу отгул 2020-01-01"})
	if strings.Contains(string(raw), "action_id") || !strings.Contains(string(raw), "уже прошла") {
		t.Fatalf("past date via agent: %s", raw)
	}
}
