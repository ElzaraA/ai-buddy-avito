package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/config"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/tasks"
)

func phase2App(t *testing.T) http.Handler {
	t.Helper()
	h, err := New(config.Config{LLMMock: true}, "../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func jsonBody[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

func TestTasksAPI(t *testing.T) {
	r := phase2App(t)
	list := jsonBody[[]tasks.View](t, doReq(t, r, "GET", "/api/v1/tasks", "intern.backend", nil).Body.Bytes())
	if len(list) != 12 {
		t.Fatalf("backend tasks: %d", len(list))
	}
	if rr := doReq(t, r, "GET", "/api/v1/tasks/fe-01", "intern.backend", nil); rr.Code != 404 {
		t.Fatalf("foreign team task: %d", rr.Code)
	}
	if rr := doReq(t, r, "GET", "/api/v1/tasks/be-02", "intern.backend", nil); rr.Code != 200 || !strings.Contains(rr.Body.String(), "definition_of_done") {
		t.Fatalf("get: %d", rr.Code)
	}
	// жизненный цикл и коды ошибок
	steps := []struct {
		user, path string
		code       int
	}{
		{"intern.backend", "/api/v1/tasks/be-02/complete", 409},
		{"intern.backend", "/api/v1/tasks/be-02/start", 200},
		{"intern.backend", "/api/v1/tasks/be-02/start", 409},
		{"mentor.backend", "/api/v1/tasks/be-02/start", 403},
		{"intern.backend", "/api/v1/tasks/be-02/fly", 404},
		{"intern.backend", "/api/v1/tasks/nope/start", 404},
		{"intern.backend", "/api/v1/tasks/be-03/start", 200},
		{"intern.backend", "/api/v1/tasks/be-04/start", 200},
		{"intern.backend", "/api/v1/tasks/be-05/start", 409}, // больше трёх в работе
		{"intern.backend", "/api/v1/tasks/be-02/complete", 200},
		{"intern.backend", "/api/v1/tasks/be-03/skip", 200},
		{"intern.backend", "/api/v1/tasks/be-03/reopen", 200},
	}
	for _, s := range steps {
		if rr := doReq(t, r, "POST", s.path, s.user, nil); rr.Code != s.code {
			t.Fatalf("%s %s: want %d got %d %s", s.user, s.path, s.code, rr.Code, rr.Body)
		}
	}
	done := jsonBody[[]tasks.View](t, doReq(t, r, "GET", "/api/v1/tasks?status=done", "intern.backend", nil).Body.Bytes())
	if len(done) != 1 || done[0].ID != "be-02" {
		t.Fatalf("filter: %+v", done)
	}
	if rr := doReq(t, r, "GET", "/api/v1/tasks?status=weird", "intern.backend", nil); rr.Code != 400 {
		t.Fatal("bad status filter")
	}
}

func TestRecommendationsAPI(t *testing.T) {
	r := phase2App(t)
	rec := jsonBody[tasks.Recommendations](t, doReq(t, r, "GET", "/api/v1/recommendations", "intern.backend", nil).Body.Bytes())
	if rec.Source != "model" || rec.Model != "logreg-v1" || len(rec.Items) != 3 || len(rec.Items[0].Reasons) == 0 {
		t.Fatalf("%+v", rec)
	}
	fe := jsonBody[tasks.Recommendations](t, doReq(t, r, "GET", "/api/v1/recommendations?limit=10", "intern.other", nil).Body.Bytes())
	for _, it := range fe.Items {
		if it.Team != "frontend" {
			t.Fatal("recommendations stay in own team")
		}
	}
	for path, code := range map[string]int{
		"/api/v1/recommendations?limit=0":  400,
		"/api/v1/recommendations?limit=11": 400,
		"/api/v1/recommendations?limit=x":  400,
	} {
		if rr := doReq(t, r, "GET", path, "intern.backend", nil); rr.Code != code {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
	if rr := doReq(t, r, "GET", "/api/v1/recommendations", "lead.backend", nil); rr.Code != 403 {
		t.Fatal("staff gets 403")
	}
	if rr := doReq(t, r, "GET", "/api/v1/recommendations", "ghost", nil); rr.Code != 401 {
		t.Fatal("unknown user gets 401")
	}
}

func TestKnowledgeAPIRespectsRoleAndTeam(t *testing.T) {
	r := phase2App(t)
	type listResp struct {
		Documents []rag.Doc `json:"documents"`
	}
	ids := func(user string) map[string]bool {
		out := map[string]bool{}
		for _, d := range jsonBody[listResp](t, doReq(t, r, "GET", "/api/v1/knowledge", user, nil).Body.Bytes()).Documents {
			out[d.ID] = true
			if d.Title == "" || d.Disclaimer == "" || len(d.Sections) != 0 {
				t.Fatalf("list item: %+v", d)
			}
		}
		return out
	}
	be, fe, mentor := ids("intern.backend"), ids("intern.other"), ids("mentor.backend")
	if !be["access.md"] || be["frontend.md"] || be["mentors.md"] {
		t.Fatalf("backend intern: %v", be)
	}
	if !fe["frontend.md"] || fe["access.md"] {
		t.Fatalf("frontend intern: %v", fe)
	}
	if !mentor["mentors.md"] {
		t.Fatal("mentor sees mentor-only article")
	}
	// статья: только видимые разделы; закрытая — 404
	doc := jsonBody[rag.Doc](t, doReq(t, r, "GET", "/api/v1/knowledge/tools.md", "intern.backend", nil).Body.Bytes())
	mdoc := jsonBody[rag.Doc](t, doReq(t, r, "GET", "/api/v1/knowledge/tools.md", "mentor.backend", nil).Body.Bytes())
	if len(doc.Sections) == 0 || len(mdoc.Sections) <= len(doc.Sections) {
		t.Fatalf("intern %d sections, mentor %d", len(doc.Sections), len(mdoc.Sections))
	}
	for path, user := range map[string]string{
		"/api/v1/knowledge/mentors.md":  "intern.backend",
		"/api/v1/knowledge/frontend.md": "intern.backend",
		"/api/v1/knowledge/missing.md":  "intern.backend",
		"/api/v1/knowledge/..%2Fgo.mod": "intern.backend",
	} {
		if rr := doReq(t, r, "GET", path, user, nil); rr.Code != 404 {
			t.Fatalf("%s as %s: %d", path, user, rr.Code)
		}
	}
	// поиск: находит доступное, не находит закрытое
	type searchResp struct {
		Hits []struct {
			DocID   string `json:"doc_id"`
			Snippet string `json:"snippet"`
		} `json:"hits"`
	}
	hits := jsonBody[searchResp](t, doReq(t, r, "GET", "/api/v1/knowledge?q=доступ%20gitlab", "intern.backend", nil).Body.Bytes()).Hits
	found := false
	for _, h := range hits {
		found = found || (h.DocID == "access.md" && h.Snippet != "")
	}
	if !found {
		t.Fatalf("search: %+v", hits)
	}
	for _, h := range jsonBody[searchResp](t, doReq(t, r, "GET", "/api/v1/knowledge?q=оценка%20стажёра%20грейд", "intern.backend", nil).Body.Bytes()).Hits {
		if h.DocID == "mentors.md" {
			t.Fatal("mentor-only article leaked in search")
		}
	}
	if rr := doReq(t, r, "GET", "/api/v1/knowledge?q="+strings.Repeat("а", 301), "intern.backend", nil); rr.Code != 400 {
		t.Fatal("long query")
	}
}

func TestTeamInternCard(t *testing.T) {
	r := phase2App(t)
	doReq(t, r, "POST", "/api/v1/tasks/be-01/start", "intern.backend", nil)
	rr := doReq(t, r, "GET", "/api/v1/team/interns/intern-1", "mentor.backend", nil)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	card := jsonBody[struct {
		User     struct{ ID string } `json:"user"`
		Progress struct {
			Items []any `json:"items"`
		} `json:"progress"`
		Tasks struct {
			Summary         tasks.Summary         `json:"summary"`
			Active          []tasks.View          `json:"active"`
			Recommendations tasks.Recommendations `json:"recommendations"`
		} `json:"tasks"`
	}](t, rr.Body.Bytes())
	if card.User.ID != "intern-1" || len(card.Progress.Items) == 0 || card.Tasks.Summary.InProgress != 1 ||
		len(card.Tasks.Active) != 1 || len(card.Tasks.Recommendations.Items) != 3 {
		t.Fatalf("card: %+v", card)
	}
	for path, want := range map[string]struct {
		user string
		code int
	}{
		"/api/v1/team/interns/intern-2": {"mentor.backend", 404}, // другая команда
		"/api/v1/team/interns/lead-1":   {"mentor.backend", 404}, // не стажёр
		"/api/v1/team/interns/nobody":   {"lead.backend", 404},
		"/api/v1/team/interns/intern-3": {"intern.backend", 403}, // стажёр не видит коллег
	} {
		if rr := doReq(t, r, "GET", path, want.user, nil); rr.Code != want.code {
			t.Fatalf("%s as %s: %d", path, want.user, rr.Code)
		}
	}
}

func TestChatHistoryIsPerUser(t *testing.T) {
	r := phase2App(t)
	rr := doReq(t, r, "POST", "/api/v1/chat/sessions", "intern.backend", nil)
	sid := jsonBody[struct {
		ID string `json:"id"`
	}](t, rr.Body.Bytes()).ID
	doReq(t, r, "POST", "/api/v1/chat/sessions/"+sid+"/messages", "intern.backend", []byte(`{"text":"как оформить отгул?"}`))
	mine := doReq(t, r, "GET", "/api/v1/chat/sessions", "intern.backend", nil)
	other := doReq(t, r, "GET", "/api/v1/chat/sessions", "intern.junior", nil)
	if !strings.Contains(mine.Body.String(), "как оформить отгул?") || strings.TrimSpace(other.Body.String()) != "[]" {
		t.Fatalf("mine=%s other=%s", mine.Body, other.Body)
	}
}

func TestTasksDisabledWithoutCatalog(t *testing.T) {
	dir := t.TempDir()
	kb := filepath.Join(dir, "knowledge")
	os.MkdirAll(kb, 0o755)
	os.WriteFile(filepath.Join(kb, "a.md"), []byte("# A\n<!-- stage: week1 team: backend audience: intern -->\nтекст"), 0o600)
	h, err := New(config.Config{LLMMock: true}, kb)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/api/v1/tasks", "/api/v1/tasks/x", "/api/v1/recommendations", "/api/v1/tasks/x/start"} {
		method := "GET"
		if strings.HasSuffix(p, "start") {
			method = "POST"
		}
		if rr := doReq(t, h, method, p, "intern.backend", nil); rr.Code != 503 {
			t.Fatalf("%s: %d", p, rr.Code)
		}
	}
	// без весов модели — правило, если каталог есть
	os.MkdirAll(filepath.Join(dir, "tasks"), 0o755)
	raw, _ := os.ReadFile("../../data/tasks/catalog.json")
	os.WriteFile(filepath.Join(dir, "tasks", "catalog.json"), raw, 0o600)
	h, _ = New(config.Config{LLMMock: true}, kb)
	rec := jsonBody[tasks.Recommendations](t, doReq(t, h, "GET", "/api/v1/recommendations", "intern.backend", nil).Body.Bytes())
	if rec.Source != "rule_based" || len(rec.Items) != 3 {
		t.Fatalf("rule based: %+v", rec)
	}
}

func TestChatRecommendsTasks(t *testing.T) {
	r := phase2App(t)
	rr := doReq(t, r, "POST", "/api/v1/ask", "intern.backend", []byte(`{"question":"какую задачу мне взять?"}`))
	if !strings.Contains(rr.Body.String(), "Советую взять") || !strings.Contains(rr.Body.String(), "tasks_recommend") {
		t.Fatalf("%s", rr.Body)
	}
	rr = doReq(t, r, "POST", "/api/v1/ask", "lead.backend", []byte(`{"question":"какую задачу мне взять?"}`))
	if strings.Contains(rr.Body.String(), "Советую взять") {
		t.Fatalf("lead: %s", rr.Body)
	}
}
