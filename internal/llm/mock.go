package llm

import (
"context"
"encoding/json"
"strings"
)

type MockClient struct{}

func (MockClient) Complete(_ context.Context, msgs []Message) (string, error) {
last := strings.ToLower(msgs[len(msgs)-1].Content)

if strings.Contains(last, "ignore previous") ||
strings.Contains(last, "show all") ||
strings.Contains(last, "all absences") {
b, _ := json.Marshal(map[string]any{
"type": "refusal",
"text": "Cannot perform this action. Contact your buddy Ivan Petrov.",
})
return string(b), nil
}

if (strings.Contains(last, "absence") || strings.Contains(last, "time off")) &&
(strings.Contains(last, "how") || strings.Contains(last, "policy") || strings.Contains(last, "rules")) &&
!strings.Contains(last, "want") && !strings.Contains(last, "2026-") && !strings.Contains(last, "friday") {
b, _ := json.Marshal(map[string]any{
"type": "tool_call",
"tool": "absence_get_policy",
"args": map[string]string{},
})
return string(b), nil
}

if (strings.Contains(last, "absence") || strings.Contains(last, "time off")) &&
(strings.Contains(last, "want") || strings.Contains(last, "need") || strings.Contains(last, "2026-") || strings.Contains(last, "friday")) {
b, _ := json.Marshal(map[string]any{
"type":   "draft",
"action": "absence",
"args":   map[string]string{"reason": "personal"},
})
return string(b), nil
}

if strings.Contains(last, "access") || strings.Contains(last, "jira") {
b, _ := json.Marshal(map[string]any{
"type": "tool_call",
"tool": "access_request",
"args": map[string]string{"resource": "jira", "reason": "onboarding"},
})
return string(b), nil
}

if strings.Contains(last, "checklist") || strings.Contains(last, "next step") {
b, _ := json.Marshal(map[string]any{
"type": "tool_call",
"tool": "checklist_get",
"args": map[string]string{},
})
return string(b), nil
}

b, _ := json.Marshal(map[string]any{
"type": "answer",
"text": "Answering from sources.",
})
return string(b), nil
}