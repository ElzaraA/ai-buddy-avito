package llm

import (
"context"
"strings"
"testing"
)

func TestMockAbsence(t *testing.T) {
c := MockClient{}
out, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "как оформить отгул"}})
if err != nil || !strings.Contains(out, "absence.create") {
t.Fatalf("must call absence: %s", out)
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