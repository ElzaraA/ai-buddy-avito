package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

type rpcOut struct {
	ID     json.RawMessage `json:"id"`
	Result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Tools           []struct {
			Name        string         `json:"name"`
			InputSchema map[string]any `json:"inputSchema"`
			Annotations map[string]any `json:"annotations"`
		} `json:"tools"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent map[string]any `json:"structuredContent"`
		IsError           bool           `json:"isError"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func rpc(t *testing.T, srv *httptest.Server, user string, body any) (int, rpcOut, string) {
	t.Helper()
	res, raw := call(t, "POST", srv.URL+"/mcp", user, body)
	var out rpcOut
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out, string(raw)
}

func rpcCall(name string, args map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}}
}

func TestMCPHandshakeAndList(t *testing.T) {
	srv := newChatServer(t)
	code, out, raw := rpc(t, srv, "intern.backend", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	if code != 200 || out.Result.ProtocolVersion == "" || string(out.ID) != "1" {
		t.Fatalf("initialize: %d %s", code, raw)
	}
	// Уведомление (без id) — 202 без тела.
	if code, _, raw := rpc(t, srv, "intern.backend", map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); code != 202 || raw != "" {
		t.Fatalf("notification: %d %q", code, raw)
	}
	if code, out, _ := rpc(t, srv, "intern.backend", map[string]any{"jsonrpc": "2.0", "id": "p", "method": "ping"}); code != 200 || out.Error != nil {
		t.Fatalf("ping: %d %+v", code, out.Error)
	}

	_, out, raw = rpc(t, srv, "intern.backend", map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	if len(out.Result.Tools) != 5 {
		t.Fatalf("tools/list: %s", raw)
	}
	mutating := map[string]bool{"absence_create_request": true, "access_request": true}
	for _, tool := range out.Result.Tools {
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("%s: bad schema %v", tool.Name, tool.InputSchema)
		}
		if tool.Annotations["readOnlyHint"] != !mutating[tool.Name] {
			t.Fatalf("%s: readOnlyHint %v", tool.Name, tool.Annotations)
		}
	}
}

func TestMCPErrors(t *testing.T) {
	srv := newChatServer(t)
	cases := []struct {
		name string
		body any
		code int
	}{
		{"parse", "not json", rpcParseError},
		{"version", map[string]any{"jsonrpc": "1.0", "id": 1, "method": "ping"}, rpcInvalidRequest},
		{"unknown method", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "resources/list"}, rpcMethodNotFound},
		{"no tool name", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{}}, rpcInvalidParams},
	}
	for _, c := range cases {
		if _, out, raw := rpc(t, srv, "intern.backend", c.body); out.Error == nil || out.Error.Code != c.code {
			t.Fatalf("%s: want rpc error %d, got %s", c.name, c.code, raw)
		}
	}
	if code, _, _ := rpc(t, srv, "hacker", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}); code != 401 {
		t.Fatalf("unknown user: %d", code)
	}
}

// Через MCP действуют те же гарантии, что и в чате: права, владелец из контекста, мутации — только черновик.
func TestMCPToolCalls(t *testing.T) {
	srv := newChatServer(t)

	_, out, raw := rpc(t, srv, "intern.backend", rpcCall("checklist_get", nil))
	if out.Result.IsError || out.Result.StructuredContent["ok"] != true || len(out.Result.Content) != 1 {
		t.Fatalf("checklist_get: %s", raw)
	}

	_, out, raw = rpc(t, srv, "intern.backend", rpcCall("access_request", map[string]any{"resource": "GitLab", "priority": 1}))
	sc := out.Result.StructuredContent
	id, _ := sc["action_id"].(string)
	if out.Result.IsError || sc["isDraft"] != true || !strings.HasPrefix(id, "REQ-") {
		t.Fatalf("access_request must return draft: %s", raw)
	}
	_, body := call(t, "GET", srv.URL+"/api/v1/actions/"+id, "intern.backend", nil)
	if !strings.Contains(string(body), `"status":"draft"`) || !strings.Contains(string(body), `"resource":"gitlab"`) {
		t.Fatalf("mcp must not execute mutation: %s", body)
	}

	_, out, raw = rpc(t, srv, "intern.backend", rpcCall("absence_create_request", map[string]any{"date_from": "2030-01-15", "date_to": "2030-01-16"}))
	if id, _ := out.Result.StructuredContent["action_id"].(string); !strings.HasPrefix(id, "ABS-") {
		t.Fatalf("absence draft: %s", raw)
	}
	for name, args := range map[string]map[string]any{
		"absence_create_request": {},                            // нет дат
		"delete_everything":      {},                            // нет такого инструмента
		"directory_find_owner":   {"query": "неизвестная тема"}, // инструмент вернул ok=false
	} {
		if _, out, raw := rpc(t, srv, "intern.backend", rpcCall(name, args)); !out.Result.IsError {
			t.Fatalf("%s: want isError, got %s", name, raw)
		}
	}
	// Руководитель не может оформить отгул и через MCP.
	if _, out, raw := rpc(t, srv, "lead.backend", rpcCall("absence_create_request", map[string]any{"date_from": "2030-01-15", "date_to": "2030-01-15"})); !out.Result.IsError || !strings.Contains(raw, "forbidden") {
		t.Fatalf("lead via mcp: %s", raw)
	}
}
