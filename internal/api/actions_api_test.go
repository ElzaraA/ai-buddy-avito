package api

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestCreateActionValidation(t *testing.T) {
	srv := newChatServer(t)
	url := srv.URL + "/api/v1/actions"
	bad := []any{
		"not json",
		map[string]string{"type": "absence", "date_from": "abc", "date_to": "2030-01-15"},
		map[string]string{"type": "absence", "date_from": "2026-13-45", "date_to": "2026-13-45"},
		map[string]string{"type": "absence", "date_from": "2030-01-15", "date_to": "2030-01-14"},
		map[string]string{"type": "access"},
		map[string]string{"type": "salary"},
	}
	for _, b := range bad {
		if res, body := call(t, "POST", url, "intern.backend", b); res.StatusCode != 400 {
			t.Fatalf("%v: want 400, got %d %s", b, res.StatusCode, body)
		}
	}
	res, body := call(t, "POST", url, "intern.backend", map[string]string{"type": "absence", "date_from": "2030-01-15", "date_to": "2030-01-16"})
	if res.StatusCode != 201 || !strings.Contains(string(body), `"status":"draft"`) || !strings.Contains(string(body), `"type":"absence"`) {
		t.Fatalf("create absence: %d %s", res.StatusCode, body)
	}
	res, body = call(t, "POST", url, "intern.backend", map[string]string{"type": "access", "resource": "gitlab"})
	if res.StatusCode != 201 || !strings.Contains(string(body), `"id":"REQ-`) {
		t.Fatalf("create access: %d %s", res.StatusCode, body)
	}
}

func TestActionsOwnershipAndRoles(t *testing.T) {
	srv := newChatServer(t)
	_, body := call(t, "POST", srv.URL+"/api/v1/actions", "lead.backend", map[string]string{"type": "absence", "date_from": "2030-01-15", "date_to": "2030-01-15"})
	var a struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &a)
	// Руководитель не подтверждает отгул сам себе (право absence.create только у стажёра и наставника).
	if res, _ := call(t, "POST", srv.URL+"/api/v1/actions/"+a.ID+"/confirm", "lead.backend", nil); res.StatusCode != 403 {
		t.Fatalf("lead confirm: want 403, got %d", res.StatusCode)
	}
	// Чужие действия не видны и не читаются.
	if res, _ := call(t, "GET", srv.URL+"/api/v1/actions/"+a.ID, "intern.backend", nil); res.StatusCode != 403 {
		t.Fatalf("foreign get: %d", res.StatusCode)
	}
	if _, body := call(t, "GET", srv.URL+"/api/v1/actions", "intern.backend", nil); strings.Contains(string(body), a.ID) {
		t.Fatalf("foreign action in list: %s", body)
	}
	if res, _ := call(t, "GET", srv.URL+"/api/v1/actions/ABS-999999", "intern.backend", nil); res.StatusCode != 404 {
		t.Fatalf("missing: %d", res.StatusCode)
	}
	if _, body := call(t, "GET", srv.URL+"/api/v1/actions", "intern.other", nil); string(body) != "[]\n" {
		t.Fatalf("empty list must be [], got %q", body)
	}
}

// Под нагрузкой ровно одно подтверждение успешно, остальные — 409.
func TestConfirmConcurrent(t *testing.T) {
	srv := newChatServer(t)
	_, body := call(t, "POST", srv.URL+"/api/v1/actions", "intern.backend", map[string]string{"type": "absence", "date_from": "2030-01-15", "date_to": "2030-01-15"})
	var a struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &a)

	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := call(t, "POST", srv.URL+"/api/v1/actions/"+a.ID+"/confirm", "intern.backend", nil)
			mu.Lock()
			codes[res.StatusCode]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[200] != 1 || codes[409] != 29 {
		t.Fatalf("codes: %v", codes)
	}
}

func TestProgressAndSteps(t *testing.T) {
	srv := newChatServer(t)
	res, body := call(t, "GET", srv.URL+"/api/v1/onboarding/progress", "intern.backend", nil)
	var p struct {
		Items       []map[string]any `json:"items"`
		NextStep    string           `json:"next_step"`
		PercentDone int              `json:"percent_done"`
	}
	_ = json.Unmarshal(body, &p)
	if res.StatusCode != 200 || len(p.Items) != 4 || p.NextStep == "" || p.PercentDone != 25 {
		t.Fatalf("progress: %d %s", res.StatusCode, body)
	}
	if res, body := call(t, "PATCH", srv.URL+"/api/v1/onboarding/steps/c2-intern-1", "intern.backend", map[string]string{"status": "bogus"}); res.StatusCode != 400 {
		t.Fatalf("bogus status: %d %s", res.StatusCode, body)
	}
	if res, _ := call(t, "PATCH", srv.URL+"/api/v1/onboarding/steps/c2-intern-1", "intern.backend", "{bad"); res.StatusCode != 400 {
		t.Fatalf("bad json: %d", res.StatusCode)
	}
	// Шаг доступов вручную не закрывается — только одобрением заявки (BR-6).
	if res, body := call(t, "PATCH", srv.URL+"/api/v1/onboarding/steps/c2-intern-1", "intern.backend", map[string]string{"status": "done"}); res.StatusCode != 409 {
		t.Fatalf("access step: want 409, got %d %s", res.StatusCode, body)
	}
	res, body = call(t, "PATCH", srv.URL+"/api/v1/onboarding/steps/c3-intern-1", "intern.backend", map[string]string{"status": "done"})
	if res.StatusCode != 200 || !strings.Contains(string(body), `"percent_done":50`) {
		t.Fatalf("patch: %d %s", res.StatusCode, body)
	}
	if res, _ := call(t, "PATCH", srv.URL+"/api/v1/onboarding/steps/c1-intern-1", "intern.other", nil); res.StatusCode != 403 {
		t.Fatalf("foreign step: %d", res.StatusCode)
	}
	// Наставник без чек-листа: пустой прогресс, без падений.
	if res, body := call(t, "GET", srv.URL+"/api/v1/onboarding/progress", "mentor.backend", nil); res.StatusCode != 200 || !strings.Contains(string(body), `"percent_done":0`) {
		t.Fatalf("mentor progress: %d %s", res.StatusCode, body)
	}
}

func TestAuthAndMe(t *testing.T) {
	srv := newChatServer(t)
	if res, body := call(t, "GET", srv.URL+"/api/v1/me", "", nil); res.StatusCode != 200 || !strings.Contains(string(body), `"intern-1"`) {
		t.Fatalf("default user: %d %s", res.StatusCode, body)
	}
	if res, body := call(t, "GET", srv.URL+"/api/v1/me?user=mentor.backend", "", nil); !strings.Contains(string(body), `"mentor-1"`) {
		t.Fatalf("query user: %d %s", res.StatusCode, body)
	}
	if res, body := call(t, "GET", srv.URL+"/api/v1/me", "hacker", nil); res.StatusCode != 401 || !strings.Contains(string(body), `"unauthorized"`) {
		t.Fatalf("unknown user: %d %s", res.StatusCode, body)
	}
	if res, _ := call(t, "GET", srv.URL+"/health", "hacker", nil); res.StatusCode != 200 {
		t.Fatalf("health must not require user: %d", res.StatusCode)
	}
}

func TestAskSyncEndpoint(t *testing.T) {
	srv := newChatServer(t)
	res, body := call(t, "POST", srv.URL+"/api/ask", "intern.backend", map[string]string{"question": "Хочу отгул 2030-01-15"})
	if res.StatusCode != 200 || !strings.Contains(string(body), `"action_id":"ABS-`) {
		t.Fatalf("ask: %d %s", res.StatusCode, body)
	}
	res, body = call(t, "POST", srv.URL+"/api/v1/ask", "intern.backend", map[string]string{"text": "Сколько платят в Авито?"})
	if res.StatusCode != 200 || !strings.Contains(string(body), `"type":"refusal"`) {
		t.Fatalf("ask v1: %d %s", res.StatusCode, body)
	}
}
