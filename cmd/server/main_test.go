package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// Сервер стартует с конфигом по умолчанию (mock без ключа), отвечает на /health и корректно останавливается.
func TestRunServesAndShutsDown(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv("ADDR", addr)
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("KNOWLEDGE_DIR", "../../data/knowledge")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for {
		res, err := http.Get("http://" + addr + "/health")
		if err == nil {
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("health %d", res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timeout")
	}
}

func startServer(t *testing.T, addr string) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if res, err := http.Get("http://" + addr + "/health"); err == nil {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("shutdown: %v", err)
			}
		case <-time.After(8 * time.Second):
			t.Fatal("shutdown timeout")
		}
	}
}

// С STATE_FILE заявка переживает перезапуск сервера.
func TestStateSurvivesRestart(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("KNOWLEDGE_DIR", "../../data/knowledge")
	t.Setenv("STATE_FILE", filepath.Join(t.TempDir(), "state.json"))

	addr := freeAddr(t)
	t.Setenv("ADDR", addr)
	stop := startServer(t, addr)
	res, err := http.Post("http://"+addr+"/api/v1/actions", "application/json",
		strings.NewReader(`{"type":"access","resource":"gitlab"}`))
	if err != nil || res.StatusCode != 201 {
		t.Fatalf("create: %v %v", err, res)
	}
	res.Body.Close()
	stop()

	addr = freeAddr(t)
	t.Setenv("ADDR", addr)
	stop = startServer(t, addr)
	defer stop()
	res, err = http.Get("http://" + addr + "/api/v1/actions/REQ-000001")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"resource":"gitlab"`) {
		t.Fatalf("action lost after restart: %d %s", res.StatusCode, body)
	}
}

// R2-11: подтверждённая заявка попадает на диск сразу, а не через 2 с по тику Persister.
func TestConfirmSavedImmediately(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("KNOWLEDGE_DIR", "../../data/knowledge")
	state := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("STATE_FILE", state)
	addr := freeAddr(t)
	t.Setenv("ADDR", addr)
	stop := startServer(t, addr)
	defer stop()

	res, err := http.Post("http://"+addr+"/api/v1/actions", "application/json",
		strings.NewReader(`{"type":"access","resource":"gitlab"}`))
	if err != nil || res.StatusCode != 201 {
		t.Fatalf("create: %v %v", err, res)
	}
	res.Body.Close()
	res, err = http.Post("http://"+addr+"/api/v1/actions/REQ-000001/confirm", "application/json", nil)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("confirm: %v %v", err, res)
	}
	res.Body.Close()
	deadline := time.Now().Add(time.Second) // заметно меньше интервала Persister (2 с)
	for {
		if raw, err := os.ReadFile(state); err == nil && strings.Contains(string(raw), `"status":"pending"`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("confirmed action was not saved immediately")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// main завершает процесс с кодом 1, если сервер не смог стартовать.
func TestMainExitCode(t *testing.T) {
	t.Setenv("KNOWLEDGE_DIR", "does-not-exist")
	code := -1
	exit = func(c int) { code = c }
	defer func() { exit = os.Exit }()
	main()
	if code != 1 {
		t.Fatalf("exit code: %d", code)
	}
}

// Открытый SSE-поток не даёт серверу остановиться штатно: по таймауту соединения закрываются принудительно.
func TestShutdownClosesHangingStreams(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("KNOWLEDGE_DIR", "../../data/knowledge")
	addr := freeAddr(t)
	t.Setenv("ADDR", addr)
	old := shutdownTimeout
	shutdownTimeout = 100 * time.Millisecond
	defer func() { shutdownTimeout = old }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()
	base := "http://" + addr
	deadline := time.Now().Add(3 * time.Second)
	for {
		if res, err := http.Get(base + "/health"); err == nil {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("server did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	res, err := http.Post(base+"/api/v1/chat/sessions", "application/json", nil)
	if err != nil || res.StatusCode != http.StatusCreated {
		cancel()
		t.Fatalf("session: %v %v", err, res)
	}
	var sess struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(res.Body).Decode(&sess)
	res.Body.Close()
	stream, err := http.Get(base + "/api/v1/chat/sessions/" + sess.ID + "/events")
	if err != nil || stream.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("sse: %v", err)
	}
	defer stream.Body.Close()

	start := time.Now()
	cancel()
	select {
	case <-done: // ошибка принудительного закрытия не важна: важно, что сервер остановился
	case <-time.After(5 * time.Second):
		t.Fatal("server hung on open SSE stream")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("shutdown took %v", d)
	}
}

func TestRunBadKnowledgeDir(t *testing.T) {
	t.Setenv("KNOWLEDGE_DIR", "does-not-exist")
	if err := run(context.Background()); err == nil {
		t.Fatal("want error for missing knowledge dir")
	}
}

func TestRunPortBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	t.Setenv("ADDR", l.Addr().String())
	t.Setenv("KNOWLEDGE_DIR", "../../data/knowledge")
	if err := run(context.Background()); err == nil {
		t.Fatal("want error for busy port")
	}
}
