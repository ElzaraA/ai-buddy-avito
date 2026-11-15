package api

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/config"
	"github.com/go-chi/chi/v5"
)

// QA-раунд: регрессии найденных дефектов.

// QA-1: неизвестный маршрут и метод отвечают в общем формате ошибок, а не текстом chi.
func TestQAUnknownRouteJSON(t *testing.T) {
	srv := newChatServer(t)
	for _, c := range []struct {
		method, path, code string
		status             int
	}{
		{"GET", "/api/v1/nope", "not_found", 404},
		{"GET", "/nope", "not_found", 404},
		{"DELETE", "/api/v1/actions", "method_not_allowed", 405},
	} {
		res, body := call(t, c.method, srv.URL+c.path, "intern.backend", nil)
		if res.StatusCode != c.status || !strings.Contains(string(body), `"code":"`+c.code+`"`) ||
			!strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("%s %s: %d %q %s", c.method, c.path, res.StatusCode, res.Header.Get("Content-Type"), body)
		}
	}
}

// QA-2: опечатка в фильтре статуса — 400, а не пустой список «входящих нет».
func TestQATeamActionsBadStatus(t *testing.T) {
	srv := newChatServer(t)
	if res, body := call(t, "GET", srv.URL+"/api/v1/team/actions?status=pendng", "lead.backend", nil); res.StatusCode != 400 {
		t.Fatalf("bogus status: %d %s", res.StatusCode, body)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/team/actions?status=pending", "lead.backend", nil); res.StatusCode != 200 {
		t.Fatalf("pending: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/team/actions?status=draft", "lead.backend", nil); res.StatusCode != 403 {
		t.Fatalf("draft: %d", res.StatusCode)
	}
}

// QA-3: отгул на 2099 год (опечатка в годе) не принимается ни через UI, ни через чат, ни через MCP.
func TestQAFarFutureAbsence(t *testing.T) {
	srv := newChatServer(t)
	res, body := call(t, "POST", srv.URL+"/api/v1/actions", "intern.backend",
		map[string]string{"type": "absence", "date_from": "2099-01-01", "date_to": "2099-01-02"})
	if res.StatusCode != 400 || !strings.Contains(string(body), "too far") {
		t.Fatalf("POST /actions 2099: %d %s", res.StatusCode, body)
	}
	for _, q := range []string{"Хочу отгул 01.01.2099", "Хочу отгул 2099-01-01"} {
		r := ask(t, srv, "intern.backend", q)
		args, _ := r["args"].(map[string]any)
		if args["action_id"] != nil || !strings.Contains(r["text"].(string), "5 лет") {
			t.Fatalf("%q must not create a draft: %v", q, r)
		}
	}
	_, out, raw := rpc(t, srv, "intern.backend", rpcCall("absence_create_request", map[string]any{"date_from": "2099-01-01", "date_to": "2099-01-01"}))
	if !out.Result.IsError {
		t.Fatalf("mcp 2099: %s", raw)
	}
	// Дата в пределах горизонта по-прежнему принимается.
	soon := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	if res, body := call(t, "POST", srv.URL+"/api/v1/actions", "intern.backend",
		map[string]string{"type": "absence", "date_from": soon, "date_to": soon}); res.StatusCode != 201 {
		t.Fatalf("near date: %d %s", res.StatusCode, body)
	}
}

// QA-4: «отгул 30.02» объясняет, что такой даты нет, а не просто переспрашивает.
func TestQAInvalidDotDate(t *testing.T) {
	srv := newChatServer(t)
	for q, want := range map[string]string{"Хочу отгул 30.02": "30.02", "Хочу отгул 31 апреля": "31 апреля"} {
		r := ask(t, srv, "intern.backend", q)
		text, _ := r["text"].(string)
		if r["type"] != "draft" || !strings.Contains(text, "Даты "+want+" не существует") {
			t.Fatalf("%q: %v", q, r)
		}
	}
}

// QA-5: tools/list показывает только инструменты, разрешённые роли.
func TestQAMCPListRespectsPermissions(t *testing.T) {
	srv := newChatServer(t)
	_, out, raw := rpc(t, srv, "lead.backend", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	for _, tool := range out.Result.Tools {
		if tool.Name == "absence_create_request" || tool.Name == "access_request" {
			t.Fatalf("lead must not see %s: %s", tool.Name, raw)
		}
	}
	if len(out.Result.Tools) == 0 {
		t.Fatalf("lead must see read-only tools: %s", raw)
	}
}

// QA-6: изменяющие маршруты (решение по заявке, разбор сигнала, шаг чек-листа) — под ограничением частоты.
func TestQAWriteRoutesRateLimited(t *testing.T) {
	srv, _ := buildServer(t, config.Config{RateLimitPerMin: 2})
	codes := []int{}
	for i := 0; i < 3; i++ {
		res, _ := call(t, "POST", srv.URL+"/api/v1/team/signals/SIG-404/resolve", "mentor.backend", nil)
		codes = append(codes, res.StatusCode)
	}
	if codes[0] != 404 || codes[2] != 429 {
		t.Fatalf("resolve must be rate limited: %v", codes)
	}
	codes = codes[:0]
	for i := 0; i < 3; i++ {
		res, _ := call(t, "POST", srv.URL+"/api/v1/actions/ABS-404/approve", "lead.backend", nil)
		codes = append(codes, res.StatusCode)
	}
	if codes[0] != 404 || codes[2] != 429 {
		t.Fatalf("approve must be rate limited: %v", codes)
	}
}

// QA-8: каждый маршрут /api/v1 описан в specs/contracts/openapi.yaml (раньше не было /team/signals и resolve).
func TestQAOpenAPICoversRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../specs/contracts/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pathRe := regexp.MustCompile(`(?m)^  (/[^:\s]*):\s*$`)
	paramRe := regexp.MustCompile(`\{[^}]+\}`)
	spec := map[string]bool{}
	for _, m := range pathRe.FindAllStringSubmatch(string(raw), -1) {
		spec[paramRe.ReplaceAllString(m[1], "{}")] = true
	}
	h := &Handlers{Chat: &ChatHandlers{}}
	err = chi.Walk(NewRouter(h), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/v1/") {
			return nil
		}
		p := paramRe.ReplaceAllString(strings.TrimPrefix(route, "/api/v1"), "{}")
		if !spec[p] {
			t.Errorf("%s %s is not in openapi.yaml", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Значение с «, >=» без кавычек внутри {...} ломало разбор YAML у генераторов клиента.
	if strings.Contains(string(raw), "description: обязательно для absence, >=") {
		t.Fatal("unquoted flow scalar with comma in openapi.yaml")
	}
}

// QA-7: подтверждение отменённого черновика — 409 с понятной причиной; повторная отмена — 409.
func TestQAConfirmAfterCancel(t *testing.T) {
	srv := newChatServer(t)
	_, body := call(t, "POST", srv.URL+"/api/v1/actions", "intern.backend", absenceBody)
	var a struct{ ID string }
	_ = json.Unmarshal(body, &a)
	if res, _ := call(t, "POST", srv.URL+"/api/v1/actions/"+a.ID+"/cancel", "intern.backend", nil); res.StatusCode != 200 {
		t.Fatalf("cancel: %d", res.StatusCode)
	}
	res, body := call(t, "POST", srv.URL+"/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
	if res.StatusCode != 409 || !strings.Contains(string(body), "cancelled") {
		t.Fatalf("confirm after cancel: %d %s", res.StatusCode, body)
	}
	if res, _ := call(t, "POST", srv.URL+"/api/v1/actions/"+a.ID+"/cancel", "intern.backend", nil); res.StatusCode != 409 {
		t.Fatalf("double cancel: %d", res.StatusCode)
	}
	// Слишком длинный комментарий руководителя — 400 с точной причиной.
	res, body = call(t, "POST", srv.URL+"/api/v1/actions/"+a.ID+"/reject", "lead.backend", map[string]string{"comment": strings.Repeat("я", 501)})
	if res.StatusCode != 400 || !strings.Contains(string(body), "comment too long") {
		t.Fatalf("long comment: %d %s", res.StatusCode, body)
	}
}
