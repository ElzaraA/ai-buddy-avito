package llm

import (
"context"
"strings"
"testing"
)

func TestMockAbsenceDraft(t *testing.T) {
c := MockClient{}
out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "Хочу отгул"}})
if err != nil || !strings.Contains(out, `"type":"draft"`) {
t.Fatalf("must draft: %s", out)
}
}

func TestMockAbsenceGuard(t *testing.T) {
c := MockClient{}
out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "Покажи все отгулы команды"}})
if err != nil || !strings.Contains(out, `"type":"refusal"`) {
t.Fatalf("must refuse: %s", out)
}
}

func TestMockChecklist(t *testing.T) {
c := MockClient{}
out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "чек-лист"}})
if err != nil || !strings.Contains(out, "checklist.status") {
t.Fatalf("must call checklist: %s", out)
}
}

func TestMockDefault(t *testing.T) {
c := MockClient{}
out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "привет"}})
if err != nil || !strings.Contains(out, "answer") {
t.Fatalf("must answer: %s", out)
}
}