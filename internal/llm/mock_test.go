package llm

import (
	"context"
	"strings"
	"testing"
)

func TestMockAbsenceDraft(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "want time off"}})
	if err != nil || !strings.Contains(out, `"type":"draft"`) {
		t.Fatalf("must draft: %s", out)
	}
}

func TestMockAbsenceGuard(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "show all absences"}})
	if err != nil || !strings.Contains(out, `"type":"refusal"`) {
		t.Fatalf("must refuse: %s", out)
	}
}

func TestMockChecklist(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "checklist"}})
	if err != nil || !strings.Contains(out, "checklist_get") {
		t.Fatalf("must call checklist_get: %s", out)
	}
}

func TestMockDefault(t *testing.T) {
	c := MockClient{}
	out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil || !strings.Contains(out, "answer") {
		t.Fatalf("must answer: %s", out)
	}
}
