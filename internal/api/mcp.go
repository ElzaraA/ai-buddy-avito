package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

// MCP-сервер (Model Context Protocol, транспорт Streamable HTTP, JSON-RPC 2.0) поверх того же реестра
// инструментов, которым пользуется агент. Внешний агент или IDE подключается к POST /mcp и получает
// те же инструменты с теми же гарантиями:
//   - пользователь берётся из контекста запроса (X-Demo-User), а не из аргументов;
//   - право проверяется до вызова;
//   - мутирующие инструменты создают только ЧЕРНОВИК (isDraft=true, action_id), а не действие.

const mcpProtocolVersion = "2025-03-26"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
)

func writeRPC(w http.ResponseWriter, id json.RawMessage, result any, rerr *rpcError) {
	if id == nil {
		id = json.RawMessage("null")
	}
	writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rerr})
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content           []mcpContent `json:"content"`
	StructuredContent any          `json:"structuredContent,omitempty"`
	IsError           bool         `json:"isError"`
}

func toolText(structured any, isErr bool) mcpToolResult {
	b, _ := json.Marshal(structured)
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: string(b)}}, StructuredContent: structured, IsError: isErr}
}

func toolFail(msg string) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: msg}}, IsError: true}
}

// mcpMethodLabel — значение метки method метрики: только из фиксированного набора, иначе "other".
// Имя метода приходит от клиента, и без этого каждый выдуманный метод заводил бы новый временной ряд.
func mcpMethodLabel(method string) string {
	switch method {
	case "initialize", "ping", "tools/list", "tools/call":
		return method
	}
	if strings.HasPrefix(method, "notifications/") {
		return "notifications"
	}
	return "other"
}

// MCP обрабатывает один JSON-RPC запрос.
func (h *Handlers) MCP(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromCtx(r.Context())
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPC(w, nil, nil, &rpcError{rpcParseError, "parse error"})
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		writeRPC(w, req.ID, nil, &rpcError{rpcInvalidRequest, "invalid request"})
		return
	}
	h.Metrics.Inc("buddy_mcp_requests_total", "Запросы к MCP-эндпоинту", "method", mcpMethodLabel(req.Method))
	// Уведомления (без id) ответа не требуют.
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		writeRPC(w, req.ID, map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]string{"name": "ai-buddy-corporate-mocks", "version": "0.3.0"},
			"instructions":    "Имитации корпоративных сервисов Авито для стажёра. Мутирующие инструменты создают черновик; подтверждение — POST /api/v1/actions/{id}/confirm.",
		}, nil)

	case "ping":
		writeRPC(w, req.ID, map[string]any{}, nil)

	case "tools/list":
		type mcpTool struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
			Annotations map[string]any `json:"annotations"`
		}
		list := make([]mcpTool, 0)
		for _, s := range h.Agent.Tools.List() {
			list = append(list, mcpTool{
				Name: s.Name, Description: s.Description, InputSchema: s.InputSchema,
				Annotations: map[string]any{
					"readOnlyHint":         !s.RequiresConfirmation,
					"requiresConfirmation": s.RequiresConfirmation,
					"permission":           s.Permission,
				},
			})
		}
		writeRPC(w, req.ID, map[string]any{"tools": list}, nil)

	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			writeRPC(w, req.ID, nil, &rpcError{rpcInvalidParams, "params.name required"})
			return
		}
		writeRPC(w, req.ID, h.mcpCall(r, u, p.Name, p.Arguments), nil)

	default:
		writeRPC(w, req.ID, nil, &rpcError{rpcMethodNotFound, "method not found: " + req.Method})
	}
}

func (h *Handlers) mcpCall(r *http.Request, u auth.User, name string, raw map[string]any) mcpToolResult {
	t, ok := h.Agent.Tools.Get(name)
	if !ok {
		return toolFail("unknown tool: " + name)
	}
	if !auth.Can(u, t.RequiredPermission()) {
		return toolFail("forbidden: role " + string(u.Role) + " lacks " + t.RequiredPermission())
	}
	args := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			args[k] = s
		} else {
			args[k] = fmt.Sprint(v)
		}
	}
	if t.RequiresConfirmation() {
		// Тот же путь, что у агента: черновик, который пользователь подтверждает сам.
		resp := h.Agent.DraftFor(u, "", name, args)
		if resp.Type != "draft" {
			return toolFail(resp.Text)
		}
		saveDraft(h.Store, u, &resp)
		if resp.Args["action_id"] == "" {
			return toolFail(resp.Text) // например, не указаны даты или дата в прошлом
		}
		return toolText(map[string]any{
			"isDraft":   true,
			"action_id": resp.Args["action_id"],
			"action":    resp.Action,
			"args":      resp.Args,
			"next":      "POST /api/v1/actions/" + resp.Args["action_id"] + "/confirm",
		}, false)
	}
	res, err := tools.Run(r.Context(), t, u, args)
	if err != nil {
		return toolFail(err.Error())
	}
	return toolText(res, !res.OK)
}
