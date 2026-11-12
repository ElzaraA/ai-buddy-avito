package llm

import (
	"context"
	"strings"
	"testing"
)

func TestMockAbsenceDraft(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: Хочу отгул 2026-11-20\nCONTEXT: x"}})
	if err != nil || !strings.Contains(out, `"type":"draft"`) {
		t.Fatalf("must draft: %s", out)
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
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "QUESTION: hello\nCONTEXT: x"}})
	if err != nil || !strings.Contains(out, "refusal") {
		t.Fatalf("must refuse on unknown: %s", out)
	}
}
