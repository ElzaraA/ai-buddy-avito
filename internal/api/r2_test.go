package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/config"
	"github.com/ElzaraA/ai-buddy-avito/internal/metrics"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// r2Server — приложение целиком (как в main) с доступом к хранилищу и метрикам.
func r2Server(t *testing.T) (*httptest.Server, *store.Store, *metrics.Registry) {
	t.Helper()
	app, err := Build(config.Config{LLMMock: true}, "../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler)
	t.Cleanup(srv.Close)
	return srv, app.Store, app.Metrics
}

// callHdr — запрос с произвольными заголовками (Origin, X-Demo-User).
func callHdr(t *testing.T, method, url string, hdr map[string]string, body any) (*http.Response, []byte) {
	t.Helper()
	raw, _ := json.Marshal(body)
	if body == nil {
		raw = nil
	}
	req, _ := http.NewRequest(method, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(res.Body)
	return res, buf.Bytes()
}

func ask(t *testing.T, srv *httptest.Server, user, q string) map[string]any {
	t.Helper()
	res, raw := call(t, "POST", srv.URL+"/api/ask", user, map[string]string{"question": q})
	var r map[string]any
	if res.StatusCode != 200 || json.Unmarshal(raw, &r) != nil {
		t.Fatalf("ask %q: %d %s", q, res.StatusCode, raw)
	}
	return r
}

func pendingAbsence(t *testing.T, srv *httptest.Server, user string) string {
	t.Helper()
	a := newDraft(t, srv, user, absenceBody)
	if code, _, raw := post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", user, nil); code != 200 {
		t.Fatalf("confirm: %d %s", code, raw)
	}
	return a.ID
}

// R2-1: ?user= работает только для GET-потока событий чата.
func TestQueryUserOnlyForEventStream(t *testing.T) {
	srv, _, _ := r2Server(t)
	id := pendingAbsence(t, srv, "intern.backend")
	// Без заголовка запрос идёт от intern.backend (по умолчанию), ?user=lead.backend игнорируется.
	res, raw := call(t, "POST", srv.URL+"/api/v1/actions/"+id+"/approve?user=lead.backend", "", nil)
	if res.StatusCode != 403 {
		t.Fatalf("approve via ?user= must be 403, got %d %s", res.StatusCode, raw)
	}
	if _, raw := call(t, "GET", srv.URL+"/api/v1/actions/"+id, "intern.backend", nil); !strings.Contains(string(raw), `"status":"pending"`) {
		t.Fatalf("action must stay pending: %s", raw)
	}
	// SSE: ?user= определяет пользователя (EventSource не умеет заголовки).
	sid := createSession(t, srv, "mentor.backend")
	openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events?user=mentor.backend")
	if res, _ := call(t, "GET", srv.URL+"/api/v1/chat/sessions/"+sid+"/events?user=intern.backend", "", nil); res.StatusCode != 403 {
		t.Fatalf("foreign SSE via ?user=: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/chat/sessions/"+sid+"/messages?user=mentor.backend", "", nil); res.StatusCode != 403 {
		t.Fatalf("?user= must be ignored on messages: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/me?user=hacker", "", nil); res.StatusCode != 200 {
		t.Fatalf("?user= outside SSE must not even be validated: %d", res.StatusCode)
	}
}

func TestIsEventStream(t *testing.T) {
	for path, want := range map[string]bool{
		"/api/v1/chat/sessions/abc/events":      true,
		"/api/v1/chat/sessions//events":         false,
		"/api/v1/chat/sessions/a/b/events":      false,
		"/api/v1/chat/sessions/abc/messages":    false,
		"/api/v1/actions/ABS-000001/approve":    false,
		"/api/v1/chat/sessions/abc/events/x":    false,
		"/x/api/v1/chat/sessions/abc/events":    false,
		"/api/v1/chat/sessions/abc/eventsextra": false,
	} {
		r := httptest.NewRequest("GET", path, nil)
		if got := isEventStream(r); got != want {
			t.Fatalf("%s: got %v", path, got)
		}
	}
	if isEventStream(httptest.NewRequest("POST", "/api/v1/chat/sessions/abc/events", nil)) {
		t.Fatal("only GET")
	}
}

// R2-2: небезопасный запрос с чужим Origin — 403; свой Origin и запрос без Origin проходят.
func TestCrossOriginWritesBlocked(t *testing.T) {
	srv, _, _ := r2Server(t)
	a := newDraft(t, srv, "intern.backend", absenceBody)
	url := srv.URL + "/api/v1/actions/" + a.ID + "/confirm"
	res, raw := callHdr(t, "POST", url, map[string]string{"X-Demo-User": "intern.backend", "Origin": "http://evil.example"}, nil)
	if res.StatusCode != 403 || !strings.Contains(string(raw), `"code":"forbidden"`) || !strings.Contains(string(raw), "cross-origin request blocked") {
		t.Fatalf("evil origin: %d %s", res.StatusCode, raw)
	}
	res, raw = callHdr(t, "POST", url, map[string]string{"X-Demo-User": "intern.backend", "Origin": "http://localhost:5173"}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("allowed origin: %d %s", res.StatusCode, raw)
	}
	b := newDraft(t, srv, "intern.backend", absenceBody) // без Origin (curl, MCP-клиенты)
	if code, _, raw := post(t, srv, "/api/v1/actions/"+b.ID+"/confirm", "intern.backend", nil); code != 200 {
		t.Fatalf("no origin: %d %s", code, raw)
	}
	// Чтение с чужим Origin не блокируется этим правилом (его ответ закрывает CORS).
	if res, _ := callHdr(t, "GET", srv.URL+"/api/v1/actions", map[string]string{"Origin": "http://evil.example"}, nil); res.StatusCode != 200 {
		t.Fatalf("GET with foreign origin: %d", res.StatusCode)
	}
	for _, m := range []string{"PATCH", "PUT", "DELETE"} {
		if res, _ := callHdr(t, m, srv.URL+"/api/v1/onboarding/steps/c3-intern-1", map[string]string{"Origin": "http://evil.example"}, nil); res.StatusCode != 403 {
			t.Fatalf("%s with foreign origin: %d", m, res.StatusCode)
		}
	}
}

func TestSameOriginWritesWildcard(t *testing.T) {
	h := SameOriginWrites([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("Origin", "http://any.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 204 {
		t.Fatalf("wildcard origins must allow: %d", rec.Code)
	}
}

// R2-3: неизвестная система не превращается в Jira — честный отказ без черновика.
func TestAccessToUnknownSystem(t *testing.T) {
	srv, _, _ := r2Server(t)
	for _, q := range []string{"Запроси доступ в Confluence", "Дай доступ к Slack", "Нужен доступ к Kubernetes", "Запроси доступ"} {
		r := ask(t, srv, "intern.backend", q)
		text, _ := r["text"].(string)
		if r["type"] != "refusal" || !strings.Contains(text, "Jira, GitLab, Grafana или Git") || r["args"] != nil {
			t.Fatalf("%q: %v", q, r)
		}
	}
	if _, raw := call(t, "GET", srv.URL+"/api/v1/actions", "intern.backend", nil); string(raw) != "[]\n" {
		t.Fatalf("no drafts must be created: %s", raw)
	}
	r := ask(t, srv, "intern.backend", "Запроси доступ в Confluence")
	if text, _ := r["text"].(string); !strings.Contains(text, "Confluence") {
		t.Fatalf("system must be named: %v", r)
	}
}

// R2-5: у стажёра frontend — свой наставник; заявку согласует руководитель frontend, а не backend.
func TestFrontendTeamContactsAndLead(t *testing.T) {
	srv, _, _ := r2Server(t)
	r := ask(t, srv, "intern.other", "Сколько платят в Авито?")
	text, _ := r["text"].(string)
	if r["type"] != "refusal" || r["buddy_name"] != "Анна Козлова" || !strings.Contains(text, "Анне Козловой") || strings.Contains(text, "Петров") {
		t.Fatalf("frontend refusal: %v", r)
	}
	r = ask(t, srv, "intern.other", "Хочу отгул 2030-01-15")
	if r["type"] != "draft" || r["manager_name"] != "Дмитрий Орлов" || r["buddy_name"] != "Анна Козлова" {
		t.Fatalf("frontend draft: %v", r)
	}
	if text, _ := r["text"].(string); !strings.Contains(text, "Анну Козлову") || strings.Contains(text, "Сидоров") {
		t.Fatalf("frontend draft text: %v", r)
	}

	id := pendingAbsence(t, srv, "intern.other")
	if code, _, _ := post(t, srv, "/api/v1/actions/"+id+"/approve", "lead.backend", nil); code != 403 {
		t.Fatalf("lead.backend must not approve frontend action: %d", code)
	}
	if _, raw := call(t, "GET", srv.URL+"/api/v1/team/actions?status=pending", "lead.frontend", nil); !strings.Contains(string(raw), id) {
		t.Fatalf("lead.frontend inbox: %s", raw)
	}
	code, upd, raw := post(t, srv, "/api/v1/actions/"+id+"/approve", "lead.frontend", nil)
	if code != 200 || upd.Status != "approved" || upd.History[len(upd.History)-1].By != "lead-2" {
		t.Fatalf("lead.frontend approve: %d %s", code, raw)
	}
}

// R2-6: наставнику и руководителю не советуем обращаться к наставнику и предупреждать его.
func TestStaffTextsDoNotReferToMentor(t *testing.T) {
	srv, _, _ := r2Server(t)
	r := ask(t, srv, "mentor.backend", "Хочу отгул 2030-01-15")
	text, _ := r["text"].(string)
	args, _ := r["args"].(map[string]any)
	if r["type"] != "draft" || strings.Contains(text, "Предупредите наставника") || args["suggest_notify_buddy"] != nil {
		t.Fatalf("mentor draft: %v", r)
	}
	for _, user := range []string{"mentor.backend", "lead.backend"} {
		r = ask(t, srv, user, "Сколько платят в Авито?")
		text, _ = r["text"].(string)
		if r["type"] != "refusal" || strings.Contains(strings.ToLower(text), "наставник") || !strings.Contains(text, "HR BP") {
			t.Fatalf("%s refusal: %v", user, r)
		}
	}
	r = ask(t, srv, "lead.backend", "Запроси доступ в GitLab")
	text, _ = r["text"].(string)
	if r["type"] != "refusal" || strings.Contains(strings.ToLower(text), "наставник") {
		t.Fatalf("lead role refusal: %v", r)
	}
	// А стажёру совет обратиться к наставнику остаётся.
	r = ask(t, srv, "intern.backend", "Хочу отгул 2030-01-15")
	if text, _ := r["text"].(string); !strings.Contains(text, "Предупредите наставника Ивана Петрова") {
		t.Fatalf("intern draft: %v", r)
	}
}

// R2-7: метка method у MCP-метрики — только из фиксированного набора.
func TestMCPMethodLabelCardinality(t *testing.T) {
	srv, _, _ := r2Server(t)
	for _, m := range []string{"x/1", "x/2", "x/3", "resources/list", "ping", "tools/list"} {
		rpc(t, srv, "intern.backend", map[string]any{"jsonrpc": "2.0", "id": 1, "method": m})
	}
	rpc(t, srv, "intern.backend", map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	_, raw := call(t, "GET", srv.URL+"/metrics", "", nil)
	text := string(raw)
	for _, want := range []string{
		`buddy_mcp_requests_total{method="other"} 4`,
		`buddy_mcp_requests_total{method="ping"} 1`,
		`buddy_mcp_requests_total{method="tools/list"} 1`,
		`buddy_mcp_requests_total{method="notifications"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, `method="x/`) || strings.Contains(text, "resources/list") {
		t.Fatal("client-supplied method leaked into metric labels")
	}
	for in, want := range map[string]string{"initialize": "initialize", "tools/call": "tools/call", "notifications/cancelled": "notifications", "": "other"} {
		if got := mcpMethodLabel(in); got != want {
			t.Fatalf("%q: %s", in, got)
		}
	}
}

// R2-8: вопрос стажёра к материалу «для наставников» — отказ с объяснением, но не пробел базы знаний.
func TestMentorOnlyMaterialIsNotAGap(t *testing.T) {
	srv, st, _ := r2Server(t)
	r := ask(t, srv, "intern.backend", "Заполняет ли наставник форму обратной связи?")
	text, _ := r["text"].(string)
	if r["type"] != "refusal" || !strings.Contains(text, "доступен наставникам") || strings.Contains(text, "обратной связи") {
		t.Fatalf("restricted refusal: %v", r)
	}
	if g := st.Gaps("backend"); len(g) != 0 {
		t.Fatalf("restricted question must not be a gap: %+v", g)
	}
	// Наставник получает ответ.
	if r := ask(t, srv, "mentor.backend", "Заполняет ли наставник форму обратной связи?"); r["type"] != "answer" {
		t.Fatalf("mentor answer: %v", r)
	}
}

// R2-9: запуски агента и их длительность считаются и для синхронного /api/ask.
func TestAskCountsAgentRuns(t *testing.T) {
	srv, _, m := r2Server(t)
	ask(t, srv, "intern.backend", "Как у нас устроено код-ревью?")
	call(t, "POST", srv.URL+"/api/v1/ask", "intern.backend", map[string]string{"text": "Сколько платят в Авито?"})
	if m.Value("buddy_agent_runs_total") != 2 {
		t.Fatalf("runs: %v", m.Value("buddy_agent_runs_total"))
	}
	_, raw := call(t, "GET", srv.URL+"/metrics", "", nil)
	if !strings.Contains(string(raw), "buddy_agent_duration_seconds_sum") {
		t.Fatal("duration metric missing")
	}
}

// R2-10: эмоциональная реплика — поддержка и предложение поговорить с наставником своей команды, не пробел.
func TestEmotionalSupport(t *testing.T) {
	srv, st, _ := r2Server(t)
	for _, q := range []string{"Мне плохо, я не справляюсь", "Я устала", "Хочу уволиться"} {
		r := ask(t, srv, "intern.other", q)
		text, _ := r["text"].(string)
		if r["type"] != "answer" || !strings.Contains(text, "Анна Козлова") || strings.Contains(text, "Не нашёл") {
			t.Fatalf("%q: %v", q, r)
		}
	}
	if g := st.Gaps("frontend"); len(g) != 0 {
		t.Fatalf("emotional message must not be a gap: %+v", g)
	}
}

// R2-11: решение по заявке сразу просит Persister сохранить состояние.
func TestDecisionRequestsSave(t *testing.T) {
	srv, st, _ := r2Server(t)
	a := newDraft(t, srv, "intern.backend", absenceBody)
	if len(st.SaveRequests()) != 0 {
		t.Fatal("draft alone does not need an immediate save")
	}
	post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	if len(st.SaveRequests()) != 1 {
		t.Fatal("confirm must request an immediate save")
	}
	<-st.SaveRequests()
	post(t, srv, "/api/v1/actions/"+a.ID+"/approve", "lead.backend", nil)
	if len(st.SaveRequests()) != 1 {
		t.Fatal("approve must request an immediate save")
	}
}
