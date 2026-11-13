package main

import (
	"context"
	"io"
	"net"
	"net/http"
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
