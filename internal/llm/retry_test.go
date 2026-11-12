package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const okBody = `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`

func flaky(t *testing.T, statuses ...int) (*OpenAIClient, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		if n <= len(statuses) {
			w.WriteHeader(statuses[n-1])
			return
		}
		_, _ = w.Write([]byte(okBody))
	}))
	t.Cleanup(srv.Close)
	return &OpenAIClient{BaseURL: srv.URL, HTTP: srv.Client(), Backoff: time.Millisecond}, &calls
}

func TestRetryOnTransientErrors(t *testing.T) {
	c, calls := flaky(t, 503, 429)
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}})
	if err != nil || out != "ok" || calls.Load() != 3 {
		t.Fatalf("want success on 3rd attempt: %q %v calls=%d", out, err, calls.Load())
	}
}

func TestRetryGivesUp(t *testing.T) {
	c, calls := flaky(t, 500, 500, 500, 500)
	if _, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil || calls.Load() != 3 {
		t.Fatalf("want error after 3 attempts: %v calls=%d", err, calls.Load())
	}
}

// 4xx (неверный ключ, плохой запрос) повторять бессмысленно.
func TestNoRetryOnClientError(t *testing.T) {
	c, calls := flaky(t, 401)
	if _, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil || calls.Load() != 1 {
		t.Fatalf("want single attempt: %v calls=%d", err, calls.Load())
	}
}

func TestRetryStopsOnContextCancel(t *testing.T) {
	c, calls := flaky(t, 500, 500, 500)
	c.Backoff = 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Complete(ctx, []Message{{Role: "user", Content: "x"}}); err == nil {
		t.Fatal("want error")
	}
	if time.Since(start) > 150*time.Millisecond || calls.Load() != 1 {
		t.Fatalf("must stop at ctx deadline: %v calls=%d", time.Since(start), calls.Load())
	}
}

func TestNetworkErrorIsRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // порт закрыт — сетевая ошибка
	c := &OpenAIClient{BaseURL: url, HTTP: &http.Client{Timeout: time.Second}, Backoff: time.Millisecond, MaxRetries: 1}
	if _, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil {
		t.Fatal("want network error")
	}
}
