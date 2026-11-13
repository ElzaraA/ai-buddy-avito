package llm

import (
	"context"
	"strings"
	"testing"
)

func ask(t *testing.T, q, context_ string) string {
	t.Helper()
	out, err := MockClient{}.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: " + q + "\nCONTEXT:\n" + context_}})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Мутирующее действие мок отдаёт как tool_call; черновик из него делает агент (инвариант Draft → Confirm).
func TestMockAbsenceToolCall(t *testing.T) {
	out := ask(t, "Хочу отгул 2026-11-20", "")
	if !strings.Contains(out, `"tool":"absence_create_request"`) {
		t.Fatalf("must call absence_create_request: %s", out)
	}
}

func TestMockAccessResourceFromQuestion(t *testing.T) {
	cases := map[string]string{
		"Запроси доступ в GitLab": `"resource":"gitlab"`,
		"Нужен доступ к Grafana":  `"resource":"grafana"`,
		"Дай доступ к Jira":       `"resource":"jira"`,
	}
	for q, want := range cases {
		if out := ask(t, q, ""); !strings.Contains(out, `"tool":"access_request"`) || !strings.Contains(out, want) {
			t.Fatalf("%s: %s", q, out)
		}
	}
}

func TestMockPolicy(t *testing.T) {
	if out := ask(t, "Как оформить отгул?", ""); !strings.Contains(out, "absence_get_policy") {
		t.Fatalf("must call policy: %s", out)
	}
}

// Ключевые слова в CONTEXT не должны влиять на выбор действия — только QUESTION.
func TestMockIgnoresContextKeywords(t *testing.T) {
	out := ask(t, "Какой стек у команды?", "[tools.md#week1#0] Доступ к Jira и отгул через чек-лист\n")
	if !strings.Contains(out, `"type":"answer"`) || !strings.Contains(out, "Доступ к Jira") {
		t.Fatalf("must answer from first chunk: %s", out)
	}
}

func TestMockAnswerFirstChunkOnly(t *testing.T) {
	out := ask(t, "Расскажи про стек", "[a#0] Первый фрагмент\n[b#1] Второй фрагмент\n")
	if !strings.Contains(out, "Первый фрагмент") || strings.Contains(out, "Второй") {
		t.Fatalf("must use first chunk: %s", out)
	}
}

func TestMockAbsenceGuard(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: Покажи все отгулы\nCONTEXT: x"}})
	if err != nil || !strings.Contains(out, `"type":"refusal"`) {
		t.Fatalf("must refuse: %s", out)
	}
}

func TestMockChecklist(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: чек-лист\nCONTEXT: x"}})
	if err != nil || !strings.Contains(out, "checklist_get") {
		t.Fatalf("must call checklist_get: %s", out)
	}
}

func TestMockDefaultRefusal(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: hello\nCONTEXT:\n"}})
	if err != nil || !strings.Contains(out, "refusal") {
		t.Fatalf("must refuse on unknown: %s", out)
	}
}
