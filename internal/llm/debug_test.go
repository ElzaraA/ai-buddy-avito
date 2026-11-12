package llm

import (
	"context"
	"testing"
)

func TestDebugMock(t *testing.T) {
	c := MockClient{}
	for _, q := range []string{
		"Хочу отгул 2026-11-20",
		"Сколько платят в Авито?",
		"Какая погода в Москве?",
		"Как оформить отгул?",
	} {
		out, _ := c.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: " + q + "\nCONTEXT: x"}})
		t.Logf("q=%q -> %s", q, out)
	}
}
