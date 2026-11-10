package llm

import (
"context"
"net/http"
"net/http/httptest"
"strings"
"testing"
)

func TestOpenAIComplete(t *testing.T) {
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
}))
defer srv.Close()

c := &OpenAIClient{BaseURL: srv.URL, APIKey: "k", Model: "m", HTTP: srv.Client()}
out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}})
if err != nil || out != "hi" {
t.Fatalf("got %q err %v", out, err)
}
}

func TestOpenAIStatusError(t *testing.T) {
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(500)
}))
defer srv.Close()
c := &OpenAIClient{BaseURL: srv.URL, HTTP: srv.Client()}
_, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}})
if err == nil || !strings.Contains(err.Error(), "500") {
t.Fatal("must error")
}
}

func TestOpenAIEmptyChoices(t *testing.T) {
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
w.Write([]byte(`{"choices":[]}`))
}))
defer srv.Close()
c := &OpenAIClient{BaseURL: srv.URL, HTTP: srv.Client()}
_, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}})
if err == nil {
t.Fatal("must error on empty choices")
}
}

func TestOpenAIBadJSON(t *testing.T) {
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Write([]byte(`not json`))
}))
defer srv.Close()
c := &OpenAIClient{BaseURL: srv.URL, HTTP: srv.Client()}
_, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "x"}})
if err == nil {
t.Fatal("must error on bad json")
}
}