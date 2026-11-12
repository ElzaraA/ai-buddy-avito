package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/config"
)

// newChatServer поднимает приложение целиком (как в main) на реальной базе знаний.
func newChatServer(t *testing.T) *httptest.Server {
	t.Helper()
	h, err := New(config.Config{LLMMock: true}, "../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, method, url, user string, body any) (*http.Response, []byte) {
	t.Helper()
	var rdr *bytes.Reader
	switch b := body.(type) {
	case nil:
		rdr = bytes.NewReader(nil)
	case string:
		rdr = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		rdr = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-Demo-User", user)
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

func createSession(t *testing.T, srv *httptest.Server, user string) string {
	t.Helper()
	res, body := call(t, "POST", srv.URL+"/api/v1/chat/sessions", user, nil)
	var s struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &s)
	if res.StatusCode != http.StatusCreated || s.ID == "" {
		t.Fatalf("create session: %d %s", res.StatusCode, body)
	}
	return s.ID
}

type sseStream struct {
	res    *http.Response
	events chan [2]string // event, data
}

func openSSE(t *testing.T, url string) *sseStream {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("events: %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}
	s := &sseStream{res: res, events: make(chan [2]string, 32)}
	go func() {
		sc := bufio.NewScanner(res.Body)
		var event string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				s.events <- [2]string{event, strings.TrimPrefix(line, "data: ")}
			}
		}
		close(s.events)
	}()
	return s
}

// waitDone ждёт message_done для конкретного сообщения и возвращает его data.
func (s *sseStream) waitDone(t *testing.T, msgID string) map[string]any {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				t.Fatal("stream closed")
			}
			if ev[0] != "message_done" {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(ev[1]), &m); err != nil {
				t.Fatalf("bad json %s", ev[1])
			}
			if m["message_id"] == msgID {
				return m
			}
		case <-deadline:
			t.Fatalf("message_done for %s not received", msgID)
		}
	}
}

func send(t *testing.T, srv *httptest.Server, sessionID, user, text string) string {
	t.Helper()
	res, body := call(t, "POST", srv.URL+"/api/v1/chat/sessions/"+sessionID+"/messages", user, map[string]string{"question": text})
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("send: %d %s", res.StatusCode, body)
	}
	var r map[string]string
	_ = json.Unmarshal(body, &r)
	// Контракт фронта (schemas.ts) + openapi.
	for _, k := range []string{"user_message_id", "assistant_message_id", "messageId", "status"} {
		if r[k] == "" {
			t.Fatalf("response has no %s: %s", k, body)
		}
	}
	return r["assistant_message_id"]
}

func result(t *testing.T, ev map[string]any) map[string]any {
	t.Helper()
	if ev["status"] != "done" {
		t.Fatalf("status %v", ev["status"])
	}
	r, ok := ev["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", ev)
	}
	return r
}

func TestChatS1DraftThenConfirm(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events?user=intern.backend")

	msg := send(t, srv, sid, "intern.backend", "Хочу взять отгул 2030-01-15")
	r := result(t, stream.waitDone(t, msg))
	if r["type"] != "draft" || r["action"] != "absence" {
		t.Fatalf("want absence draft: %v", r)
	}
	args := r["args"].(map[string]any)
	id, _ := args["action_id"].(string)
	if !strings.HasPrefix(id, "ABS-") {
		t.Fatalf("no action_id: %v", args)
	}
	if strings.Contains(r["text"].(string), "POST /api") {
		t.Fatal("API path leaked into user text")
	}

	// Без подтверждения заявка — черновик.
	res, body := call(t, "GET", srv.URL+"/api/v1/actions/"+id, "intern.backend", nil)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"status":"draft"`) {
		t.Fatalf("get draft: %d %s", res.StatusCode, body)
	}
	res, body = call(t, "POST", srv.URL+"/api/v1/actions/"+id+"/confirm", "intern.backend", nil)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"status":"pending"`) {
		t.Fatalf("confirm: %d %s", res.StatusCode, body)
	}
	// Действие видно в списке после «перезагрузки».
	res, body = call(t, "GET", srv.URL+"/api/v1/actions", "intern.backend", nil)
	if res.StatusCode != 200 || !strings.Contains(string(body), id) {
		t.Fatalf("list: %d %s", res.StatusCode, body)
	}
}

func TestChatS1DateClarification(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")

	r := result(t, stream.waitDone(t, send(t, srv, sid, "intern.backend", "Хочу взять отгул")))
	if r["type"] != "draft" || r["args"].(map[string]any)["action_id"] != nil {
		t.Fatalf("want draft asking for date: %v", r)
	}
	r = result(t, stream.waitDone(t, send(t, srv, sid, "intern.backend", "2030-01-15")))
	args := r["args"].(map[string]any)
	if r["type"] != "draft" || args["date_from"] != "2030-01-15" || args["action_id"] == nil {
		t.Fatalf("clarification must complete draft: %v", r)
	}
}

func TestChatS2AccessDraft(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")

	r := result(t, stream.waitDone(t, send(t, srv, sid, "intern.backend", "Запроси доступ в GitLab")))
	args := r["args"].(map[string]any)
	if r["action"] != "access" || args["resource"] != "gitlab" || !strings.HasPrefix(args["action_id"].(string), "REQ-") {
		t.Fatalf("want access draft for gitlab: %v", r)
	}
	id := args["action_id"].(string)
	// Чужой стажёр не может подтвердить заявку, владелец — может, повторно — 409.
	if res, _ := call(t, "POST", srv.URL+"/api/v1/actions/"+id+"/confirm", "intern.other", nil); res.StatusCode != 403 {
		t.Fatalf("foreign confirm: %d", res.StatusCode)
	}
	if res, body := call(t, "POST", srv.URL+"/api/v1/actions/"+id+"/confirm", "intern.backend", nil); res.StatusCode != 200 {
		t.Fatalf("confirm: %d %s", res.StatusCode, body)
	}
	if res, _ := call(t, "POST", srv.URL+"/api/v1/actions/"+id+"/confirm", "intern.backend", nil); res.StatusCode != 409 {
		t.Fatalf("double confirm: %d", res.StatusCode)
	}
}

func TestChatS3AnswerAndRefusal(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")

	r := result(t, stream.waitDone(t, send(t, srv, sid, "intern.backend", "Какой стек у команды Backend?")))
	if r["type"] != "answer" || r["sources"] == nil {
		t.Fatalf("want answer with sources: %v", r)
	}
	r = result(t, stream.waitDone(t, send(t, srv, sid, "intern.backend", "Сколько платят в Авито?")))
	if r["type"] != "refusal" || r["buddy_name"] == "" {
		t.Fatalf("want refusal: %v", r)
	}
}

func TestChatToolResultEvent(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")
	send(t, srv, sid, "intern.backend", "Что мне сделать на этой неделе?")
	seen := map[string]bool{}
	deadline := time.After(3 * time.Second)
	for !seen["message_done"] {
		select {
		case ev := <-stream.events:
			seen[ev[0]] = true
			if !strings.Contains(ev[1], `"type":"`+ev[0]+`"`) {
				t.Fatalf("event %s has no type field: %s", ev[0], ev[1])
			}
		case <-deadline:
			t.Fatalf("events seen: %v", seen)
		}
	}
	if !seen["status"] || !seen["tool_result"] {
		t.Fatalf("want status → tool_result → message_done, got %v", seen)
	}
}

// Если ответ готов раньше, чем открылся поток, он приходит повторно при подключении.
func TestChatReplayOnConnect(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	msg := send(t, srv, sid, "intern.backend", "Сколько платят в Авито?")
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, body := call(t, "GET", srv.URL+"/api/v1/chat/sessions/"+sid+"/messages", "intern.backend", nil)
		if strings.Contains(string(body), `"status":"done","result"`) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")
	if r := result(t, stream.waitDone(t, msg)); r["type"] != "refusal" {
		t.Fatalf("replay: %v", r)
	}
}

func TestChatForeignSessionForbidden(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	for _, tc := range []struct{ method, path string }{
		{"GET", "/messages"}, {"POST", "/messages"}, {"GET", "/events"},
	} {
		res, _ := call(t, tc.method, srv.URL+"/api/v1/chat/sessions/"+sid+tc.path, "intern.other", map[string]string{"question": "x"})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s: want 403, got %d", tc.method, tc.path, res.StatusCode)
		}
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/chat/sessions/nope/messages", "intern.backend", nil); res.StatusCode != 403 {
		t.Fatalf("unknown session: %d", res.StatusCode)
	}
}

func TestChatBadInput(t *testing.T) {
	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	url := srv.URL + "/api/v1/chat/sessions/" + sid + "/messages"
	cases := []struct {
		name string
		body any
	}{
		{"invalid json", "not json"},
		{"empty", map[string]string{}},
		{"spaces", map[string]string{"text": "   "}},
		{"too long", map[string]string{"text": strings.Repeat("я", maxQuestionRunes+1)}},
	}
	for _, c := range cases {
		if res, body := call(t, "POST", url, "intern.backend", c.body); res.StatusCode != 400 {
			t.Fatalf("%s: want 400, got %d %s", c.name, res.StatusCode, body)
		}
	}
	// openapi-поле text тоже принимается.
	if res, _ := call(t, "POST", url, "intern.backend", map[string]string{"text": "Привет"}); res.StatusCode != 202 {
		t.Fatalf("text field: %d", res.StatusCode)
	}
}

func TestBodyLimit(t *testing.T) {
	srv := newChatServer(t)
	big := `{"question":"` + strings.Repeat("a", maxBodyBytes+10) + `"}`
	if res, _ := call(t, "POST", srv.URL+"/api/ask", "intern.backend", big); res.StatusCode != 400 {
		t.Fatalf("want 400 for oversized body, got %d", res.StatusCode)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newChatServer(t)
	req, _ := http.NewRequest("OPTIONS", srv.URL+"/api/v1/chat/sessions", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type,x-demo-user")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("preflight: %d %v", res.StatusCode, res.Header)
	}
}

func TestEventsHeartbeat(t *testing.T) {
	old := heartbeatEvery
	heartbeatEvery = 20 * time.Millisecond
	defer func() { heartbeatEvery = old }()

	srv := newChatServer(t)
	sid := createSession(t, srv, "intern.backend")
	res, err := http.Get(srv.URL + "/api/v1/chat/sessions/" + sid + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	line := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), ": ping") {
				line <- sc.Text()
				return
			}
		}
	}()
	select {
	case <-line:
	case <-time.After(2 * time.Second):
		t.Fatal("no heartbeat")
	}
}

func TestEventsWithoutFlusher(t *testing.T) {
	h, err := New(config.Config{LLMMock: true}, "../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/chat/sessions", nil))
	var s struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &s)
	rec := httptest.NewRecorder()
	h.ServeHTTP(noFlush{rec}, httptest.NewRequest("GET", "/api/v1/chat/sessions/"+s.ID+"/events", nil))
	if rec.Code != 500 {
		t.Fatalf("want 500 without flusher, got %d", rec.Code)
	}
}

// noFlush — ResponseWriter без http.Flusher.
type noFlush struct{ rec *httptest.ResponseRecorder }

func (n noFlush) Header() http.Header         { return n.rec.Header() }
func (n noFlush) Write(b []byte) (int, error) { return n.rec.Write(b) }
func (n noFlush) WriteHeader(code int)        { n.rec.WriteHeader(code) }
