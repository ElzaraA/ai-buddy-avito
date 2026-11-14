package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/config"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/metrics"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

// scriptedLLM — управляемая «модель»: может падать, паниковать, блокироваться и запоминает, что ей прислали.
type scriptedLLM struct {
	mu      sync.Mutex
	err     error
	panicky bool
	block   chan struct{}
	out     string
	seen    [][]llm.Message
}

func (s *scriptedLLM) Complete(ctx context.Context, msgs []llm.Message) (string, error) {
	s.mu.Lock()
	s.seen = append(s.seen, append([]llm.Message(nil), msgs...))
	err, out, block, panicky := s.err, s.out, s.block, s.panicky
	s.mu.Unlock()
	if panicky {
		panic("llm client bug")
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if err != nil {
		return "", err
	}
	return out, nil
}

// customServer собирает приложение с подменённой моделью (как api.Build, но с нужным LLM).
func customServer(t *testing.T, model llm.Client, fallback llm.Client, limit int) (*httptest.Server, *metrics.Registry) {
	t.Helper()
	chunks, err := rag.LoadDir("../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New()
	SeedDemo(st)
	reg := tools.NewRegistry()
	reg.Register(&tools.ChecklistGet{S: st})
	reg.Register(&tools.AbsenceCreateRequest{S: st})
	reg.Register(&tools.AccessRequest{})
	m := metrics.New()
	ag := &agent.Agent{LLM: model, Fallback: fallback, Index: rag.NewIndex(chunks), Tools: reg,
		OnFallback: func(error) { m.Inc("buddy_llm_fallback_total", "fallback") }}
	chat := &ChatHandlers{Agent: ag, Store: st, Msgs: store.NewMessageStore(), Broker: NewBroker(), Metrics: m}
	h := &Handlers{Agent: ag, Store: st, Chat: chat, Metrics: m, Limiter: NewRateLimiter(limit, time.Minute)}
	srv := httptest.NewServer(NewRouter(h))
	t.Cleanup(srv.Close)
	return srv, m
}

// Основная модель недоступна → отвечает резервная, ответ помечен degraded, пользователь продолжает работать.
func TestFallbackWhenLLMDown(t *testing.T) {
	srv, m := customServer(t, &scriptedLLM{err: errors.New("llm status 503")}, llm.MockClient{}, 0)
	res, raw := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Как у нас устроено код-ревью?"})
	var r map[string]any
	_ = json.Unmarshal(raw, &r)
	if res.StatusCode != 200 || r["type"] != "answer" || r["degraded"] != true {
		t.Fatalf("fallback answer: %d %s", res.StatusCode, raw)
	}
	if m.Value("buddy_llm_fallback_total") != 1 || m.Value("buddy_agent_degraded_total") != 1 {
		t.Fatal("fallback metrics not recorded")
	}
}

// Без резервной модели: синхронно — 502 без внутренних деталей, в чате — событие error и message_done failed.
func TestLLMDownWithoutFallback(t *testing.T) {
	srv, m := customServer(t, &scriptedLLM{err: errors.New(`Post "https://api.openai.com/v1": secret-key-123`)}, nil, 0)
	res, raw := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Где смотреть логи?"})
	if res.StatusCode != http.StatusBadGateway || strings.Contains(string(raw), "openai") || strings.Contains(string(raw), "secret") {
		t.Fatalf("must hide internals: %d %s", res.StatusCode, raw)
	}

	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")
	msg := send(t, srv, sid, "intern.backend", "Где смотреть логи?")
	sawError := false
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-stream.events:
			if strings.Contains(ev[1], "secret") || strings.Contains(ev[1], "openai") {
				t.Fatalf("internals leaked to SSE: %s", ev[1])
			}
			if ev[0] == "error" {
				sawError = true
			}
			if ev[0] == "message_done" && strings.Contains(ev[1], msg) {
				if !sawError || !strings.Contains(ev[1], `"status":"failed"`) {
					t.Fatalf("want error then failed, got %s", ev[1])
				}
				if m.Value("buddy_agent_errors_total") != 2 {
					t.Fatalf("errors metric: %v", m.Value("buddy_agent_errors_total"))
				}
				return
			}
		case <-deadline:
			t.Fatal("no message_done")
		}
	}
}

// Паника внутри клиента модели не роняет сервер: сообщение завершается как failed.
func TestAgentPanicIsContained(t *testing.T) {
	srv, _ := customServer(t, &scriptedLLM{panicky: true}, nil, 0)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")
	ev := stream.waitDone(t, send(t, srv, sid, "intern.backend", "Где смотреть логи?"))
	if ev["status"] != "failed" {
		t.Fatalf("want failed, got %v", ev)
	}
	if res, _ := call(t, "GET", srv.URL+"/health", "", nil); res.StatusCode != 200 {
		t.Fatal("server died after panic")
	}
	// Синхронный путь защищён middleware.Recoverer.
	if res, _ := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "x"}); res.StatusCode != 500 {
		t.Fatalf("sync panic: %d", res.StatusCode)
	}
}

// Модель получает последние реплики диалога; медленная модель не блокирует HTTP; лишние вопросы — 429.
func TestHistoryAndPendingCap(t *testing.T) {
	model := &scriptedLLM{out: `{"type":"answer","text":"Через GitLab"}`}
	srv, _ := customServer(t, model, nil, 0)
	sid := createSession(t, srv, "intern.backend")
	stream := openSSE(t, srv.URL+"/api/v1/chat/sessions/"+sid+"/events")
	stream.waitDone(t, send(t, srv, sid, "intern.backend", "Как у нас устроено код-ревью?"))
	stream.waitDone(t, send(t, srv, sid, "intern.backend", "А как устроен деплой?"))

	model.mu.Lock()
	second := model.seen[1]
	model.mu.Unlock()
	if len(second) != 4 || second[0].Role != "system" || second[1].Role != "user" ||
		!strings.Contains(second[1].Content, "код-ревью") || second[2].Role != "assistant" || second[2].Content != "Через GitLab" {
		t.Fatalf("history not passed: %+v", second)
	}

	// Модель «зависла»: три вопроса приняты мгновенно, четвёртый — 429.
	block := make(chan struct{})
	model.mu.Lock()
	model.block = block
	model.mu.Unlock()
	url := srv.URL + "/api/v1/chat/sessions/" + sid + "/messages"
	var last string
	for i := 0; i < maxPendingPerSession; i++ {
		start := time.Now()
		last = send(t, srv, sid, "intern.backend", "Как устроен деплой?")
		if d := time.Since(start); d > time.Second {
			t.Fatalf("POST blocked for %v while LLM is slow", d)
		}
	}
	res, raw := call(t, "POST", url, "intern.backend", map[string]string{"text": "ещё вопрос"})
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" {
		t.Fatalf("want 429 with Retry-After, got %d %s", res.StatusCode, raw)
	}
	close(block)
	stream.waitDone(t, last)
}

func TestRateLimit(t *testing.T) {
	srv, _ := customServer(t, llm.MockClient{}, nil, 3)
	for i := 0; i < 3; i++ {
		if res, _ := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Привет"}); res.StatusCode != 200 {
			t.Fatalf("request %d: %d", i, res.StatusCode)
		}
	}
	res, raw := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Привет"})
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" || !strings.Contains(string(raw), "rate_limited") {
		t.Fatalf("want 429: %d %s", res.StatusCode, raw)
	}
	// Лимит персональный и не касается чтения.
	if res, _ := call(t, "POST", srv.URL+"/api/ask", "intern.other", map[string]string{"question": "Привет"}); res.StatusCode != 200 {
		t.Fatalf("other user limited: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/onboarding/progress", "intern.backend", nil); res.StatusCode != 200 {
		t.Fatalf("reads must not be limited: %d", res.StatusCode)
	}
}

func TestRateLimiterWindow(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	l := NewRateLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	if ok, _ := l.allow("u"); !ok {
		t.Fatal("first")
	}
	if ok, _ := l.allow("u"); !ok {
		t.Fatal("second")
	}
	if ok, retry := l.allow("u"); ok || retry < 1 || retry > 61 {
		t.Fatalf("third must be limited, retry=%d", retry)
	}
	now = now.Add(61 * time.Second)
	if ok, _ := l.allow("u"); !ok {
		t.Fatal("window must reset")
	}
	if NewRateLimiter(0, time.Minute) != nil {
		t.Fatal("0 = no limiter")
	}
}

func TestMetricsAndRequestID(t *testing.T) {
	h, err := New(config.Config{LLMMock: true}, "../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	res, _ := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Сколько платят в Авито?"})
	if res.Header.Get("X-Request-Id") == "" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers: %v", res.Header)
	}
	call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Как у нас устроено код-ревью?"})
	a := newDraft(t, srv, "intern.backend", absenceBody)
	post(t, srv, "/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	call(t, "GET", srv.URL+"/api/v1/actions/"+a.ID, "intern.backend", nil)

	res, raw := call(t, "GET", srv.URL+"/metrics", "", nil)
	text := string(raw)
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("metrics: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	for _, want := range []string{
		`buddy_agent_responses_total{type="refusal"} 1`,
		`buddy_agent_responses_total{type="answer"} 1`,
		`buddy_knowledge_gaps_total 1`,
		`buddy_actions_total{type="absence",status="pending"} 1`,
		`buddy_http_requests_total{method="POST",route="/api/ask",status="200"} 2`,
		// метка route — шаблон, а не сырой путь: кардинальность не растёт от id
		`route="/api/v1/actions/{id}"`,
		`# TYPE buddy_knowledge_chunks counter`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, a.ID) {
		t.Fatal("raw ids leaked into metric labels")
	}
}

// Нагрузочный тест: 100 параллельных диалогов — каждый получает свой ответ, ничего не теряется и не перемешивается.
func TestHundredParallelChats(t *testing.T) {
	srv := newChatServer(t)
	const n = 100
	users := []string{"intern.backend", "intern.other", "mentor.backend", "intern.junior"}
	questions := map[string]string{
		"Что мне сделать на этой неделе?": "action",
		"Сколько платят в Авито?":         "refusal",
		"Запроси доступ в GitLab":         "draft",
	}
	// t.Fatal/FailNow нельзя вызывать из горутин теста: ошибки собираются в канал и выводятся после wg.Wait.
	var wg sync.WaitGroup
	errs := make(chan error, n*len(questions))
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			user := users[i%len(users)]
			if err := parallelChat(srv, user, questions); err != nil {
				errs <- fmt.Errorf("chat %d (%s): %w", i, user, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	failed := 0
	for err := range errs {
		failed++
		t.Error(err)
	}
	if failed > 0 {
		t.Fatalf("%d chats failed under load", failed)
	}
}

// parallelChat — один диалог нагрузочного теста без t.Fatal: создаёт сессию, открывает SSE,
// задаёт вопросы и сверяет тип ответа. Возвращает первую ошибку.
func parallelChat(srv *httptest.Server, user string, questions map[string]string) error {
	var sess struct {
		ID string `json:"id"`
	}
	if code, err := doJSON("POST", srv.URL+"/api/v1/chat/sessions", user, nil, &sess); err != nil || code != http.StatusCreated || sess.ID == "" {
		return fmt.Errorf("create session: %d %v", code, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/chat/sessions/"+sess.ID+"/events?user="+user, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("open SSE: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("open SSE: %d", res.StatusCode)
	}
	events := make(chan map[string]any, 32)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				var m map[string]any
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m) == nil && m["type"] == "message_done" {
					events <- m
				}
			}
		}
	}()
	for q, want := range questions {
		if user == "mentor.backend" && want == "draft" {
			continue
		}
		var sent map[string]string
		if code, err := doJSON("POST", srv.URL+"/api/v1/chat/sessions/"+sess.ID+"/messages", user, map[string]string{"question": q}, &sent); err != nil || code != http.StatusAccepted {
			return fmt.Errorf("send %q: %d %v", q, code, err)
		}
		ev, err := waitDoneEvent(events, sent["assistant_message_id"])
		if err != nil {
			return fmt.Errorf("%q: %w", q, err)
		}
		if r, _ := ev["result"].(map[string]any); r["type"] != want {
			return fmt.Errorf("%q: want %s, got %v", q, want, r["type"])
		}
	}
	return nil
}

// doJSON — HTTP-запрос без *testing.T (безопасен в горутинах): декодирует JSON-ответ в out.
func doJSON(method, url, user string, body, out any) (int, error) {
	raw, _ := json.Marshal(body)
	if body == nil {
		raw = nil
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(raw)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Demo-User", user)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, json.NewDecoder(res.Body).Decode(out)
}

// waitDoneEvent ждёт message_done для сообщения msgID (без t.Fatal).
func waitDoneEvent(events <-chan map[string]any, msgID string) (map[string]any, error) {
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil, errors.New("stream closed")
			}
			if ev["message_id"] == msgID {
				return ev, nil
			}
		case <-deadline:
			return nil, fmt.Errorf("message_done for %s not received", msgID)
		}
	}
}
