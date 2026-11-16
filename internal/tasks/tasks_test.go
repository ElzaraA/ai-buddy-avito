package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/recsys"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

var (
	intern  = auth.DemoUsers["intern.backend"]
	junior  = auth.DemoUsers["intern.junior"]
	foreign = auth.DemoUsers["intern.other"]
	mentor  = auth.DemoUsers["mentor.backend"]
)

func svc(t *testing.T, withModel bool) *Service {
	t.Helper()
	c, err := LoadCatalog("../../data/tasks/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: c, Store: store.New()}
	if withModel {
		if s.Model, err = recsys.Load("../../data/recsys/model.json"); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestCatalogIsTeamScoped(t *testing.T) {
	s := svc(t, true)
	be, fe := s.List(intern), s.List(foreign)
	if len(be) != 12 || len(fe) != 6 {
		t.Fatalf("backend=%d frontend=%d", len(be), len(fe))
	}
	for _, v := range be {
		if v.Team != "backend" || v.Status != "todo" || v.Done == "" || len(v.Steps) == 0 {
			t.Fatalf("backend view: %+v", v)
		}
	}
	if _, err := s.Get(foreign, "be-01"); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign team task must look missing")
	}
	if _, err := s.Get(intern, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown")
	}
}

func TestLifecycleAndLimits(t *testing.T) {
	s := svc(t, true)
	if _, err := s.Start(mentor, "be-01"); !errors.Is(err, ErrForbidden) {
		t.Fatal("mentor can't take tasks")
	}
	if _, err := s.Complete(intern, "be-01"); !errors.As(err, new(ConflictError)) {
		t.Fatal("complete before start")
	}
	v, err := s.Start(intern, "be-01")
	if err != nil || v.Status != store.TaskInProgress || v.StartedAt == nil {
		t.Fatalf("start: %+v %v", v, err)
	}
	if _, err := s.Start(intern, "be-01"); !errors.As(err, new(ConflictError)) {
		t.Fatal("double start")
	}
	s.Start(intern, "be-02")
	s.Start(intern, "be-03")
	if _, err := s.Start(intern, "be-04"); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit: %v", err)
	}
	v, err = s.Complete(intern, "be-01")
	if err != nil || v.Status != store.TaskDone || v.DoneAt == nil {
		t.Fatalf("complete: %+v %v", v, err)
	}
	if _, err := s.Skip(intern, "be-01"); err == nil {
		t.Fatal("done task can't be skipped")
	}
	if v, err = s.Skip(intern, "be-02"); err != nil || v.Status != store.TaskSkipped {
		t.Fatalf("skip: %v", err)
	}
	if v, err = s.Reopen(intern, "be-02"); err != nil || v.Status != "todo" {
		t.Fatalf("reopen: %+v %v", v, err)
	}
	if err := (ConflictError{Status: store.TaskDone}).Error(); err != "нельзя изменить задачу в статусе «сделано»" {
		t.Fatal(err)
	}
	sum := s.SummaryFor(intern.ID, intern.TeamID)
	if sum.Total != 12 || sum.Done != 1 || sum.InProgress != 1 || sum.Skipped != 0 {
		t.Fatalf("summary: %+v", sum)
	}
	if h := s.History(intern.ID); len(h) != 2 || h[0].ID == "" {
		t.Fatalf("history: %+v", h)
	}
	// другой стажёр не видит чужих статусов
	if v, _ := s.Get(junior, "be-01"); v.Status != "todo" {
		t.Fatal("statuses are per intern")
	}
}

func TestRecommendationsReactToProgress(t *testing.T) {
	s := svc(t, true)
	first := s.Recommend(intern.ID, intern.TeamID, 3)
	if first.Source != recsys.SourceModel || len(first.Items) != 3 {
		t.Fatalf("%+v", first)
	}
	for _, it := range first.Items {
		if it.Team != "backend" || len(it.Reasons) == 0 || it.Score <= 0 || it.Score >= 1 {
			t.Fatalf("item: %+v", it)
		}
	}
	// взятая задача уходит из рекомендаций, сделанная — повышает компетенции
	top := first.Items[0].ID
	s.Start(intern, top)
	s.Complete(intern, top)
	next := s.Recommend(intern.ID, intern.TeamID, 12)
	for _, it := range next.Items {
		if it.ID == top {
			t.Fatal("done task must not be recommended")
		}
	}
	if len(next.Items) != 11 {
		t.Fatalf("remaining: %d", len(next.Items))
	}
	before := s.Catalog.profiles[intern.ID]
	after := s.Profile(intern.ID, intern.TeamID).Competencies
	grew := false
	for _, tag := range s.Catalog.byID[top].Tags {
		grew = grew || after[tag] > before[tag]
	}
	if !grew {
		t.Fatal("competencies must grow after a done task")
	}
	// без модели — честный rule_based
	if r := svc(t, false).Recommend(junior.ID, junior.TeamID, 2); r.Source != recsys.SourceRules || r.Model != "" || len(r.Items) != 2 {
		t.Fatalf("rules: %+v", r)
	}
}

func TestLoadCatalogValidation(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		`{`,
		`{"tasks":[{"id":"","team":"b","title":"t","difficulty":1,"tags":["go"]}]}`,
		`{"tasks":[{"id":"a","team":"b","title":"t","difficulty":9,"tags":["go"]}]}`,
		`{"tasks":[{"id":"a","team":"b","title":"t","difficulty":1}]}`,
		`{"tasks":[{"id":"a","team":"b","title":"t","difficulty":1,"tags":["go"]},{"id":"a","team":"b","title":"t","difficulty":1,"tags":["go"]}]}`,
	}
	for i, body := range cases {
		p := filepath.Join(dir, "c.json")
		os.WriteFile(p, []byte(body), 0o600)
		if _, err := LoadCatalog(p); err == nil {
			t.Fatalf("case %d must fail", i)
		}
	}
	if _, err := LoadCatalog(filepath.Join(dir, "none.json")); err == nil {
		t.Fatal("missing")
	}
	p := filepath.Join(dir, "ok.json")
	os.WriteFile(p, []byte(`{"tasks":[{"id":"a","team":"b","title":"t","difficulty":1,"tags":["go"],"done":"готово"}]}`), 0o600)
	c, err := LoadCatalog(p)
	if err != nil || c.byID["a"].Done != "готово" || c.profiles == nil {
		t.Fatalf("ok: %v", err)
	}
}
