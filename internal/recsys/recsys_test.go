package recsys

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T) *Model {
	t.Helper()
	m, err := Load("../../data/recsys/model.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Эталон — predict_proba настоящей sklearn-модели ml/recsys/artifacts/recsys_lr.joblib
// на тех же входах через ml/recsys/features.py (см. ml/recsys/README.md, «Совпадение с Go»).
func TestScoreMatchesSklearn(t *testing.T) {
	m := load(t)
	cases := []struct {
		p     Profile
		c     Candidate
		prior Prior
		want  float64
	}{
		{Profile{"backend", map[string]int{"go": 2, "sql": 1, "docker": 1, "testing": 1}}, Candidate{"x", "backend", 2, []string{"go", "testing", "review"}, 22}, Prior{}, 0.747144},
		{Profile{"backend", map[string]int{"go": 1, "linux": 2}}, Candidate{"y", "backend", 5, []string{"go", "kafka", "metrics", "observability"}, 3}, Prior{1, 1}, 0.573663},
		{Profile{"frontend", map[string]int{"typescript": 2, "react": 2, "testing": 1}}, Candidate{"z", "backend", 1, []string{"go", "docker"}, 29}, Prior{0, 2}, 0.4358},
	}
	for _, c := range cases {
		if got := m.Score(Features(c.p, c.c, c.prior)); math.Abs(got-c.want) > 1e-6 {
			t.Fatalf("%s: go=%.6f sklearn=%.6f", c.c.ID, got, c.want)
		}
	}
}

func TestRankOrdersExplainsAndCuts(t *testing.T) {
	p := Profile{"backend", map[string]int{"go": 2, "testing": 1}}
	cands := []Candidate{
		{"hard", "backend", 5, []string{"kafka", "go"}, 2},
		{"fit", "backend", 2, []string{"go", "testing"}, 20},
		{"other", "frontend", 2, []string{"react"}, 20},
	}
	for _, m := range []*Model{load(t), nil} {
		res := Rank(m, p, cands, Prior{}, 2)
		if len(res.Items) != 2 || res.Items[0].ID != "fit" {
			t.Fatalf("model=%v: %+v", m != nil, res)
		}
		if (m == nil) != (res.Source == SourceRules) || (m != nil) != (res.Model == ModelName) {
			t.Fatalf("source: %+v", res)
		}
		if !strings.Contains(strings.Join(res.Items[0].Reasons, " "), "go, testing") {
			t.Fatalf("reasons: %v", res.Items[0].Reasons)
		}
	}
	if all := Rank(nil, p, cands, Prior{}, 0); len(all.Items) != 3 {
		t.Fatal("k<=0 returns all")
	}
	// равные оценки — детерминированно по id
	tie := Rank(nil, Profile{}, []Candidate{{ID: "b", Difficulty: 1}, {ID: "a", Difficulty: 1}}, Prior{}, 0)
	if tie.Items[0].ID != "a" {
		t.Fatal("tie order")
	}
}

func TestExplainLevels(t *testing.T) {
	p := Profile{"backend", map[string]int{"go": 2}}
	cases := map[int]string{2: "По силам", 3: "Чуть выше", 5: "Сложная"}
	for d, want := range cases {
		if got := strings.Join(Explain(p, Candidate{Difficulty: d, Tags: []string{"go"}}), " "); !strings.Contains(got, want) {
			t.Fatalf("difficulty %d: %q", d, got)
		}
	}
	if got := Explain(Profile{}, Candidate{Difficulty: 3, Popularity: 20}); len(got) != 2 || !strings.Contains(got[1], "20 стажёров") {
		t.Fatalf("no competencies: %v", got)
	}
}

func TestLoadValidates(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "m.json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	bad := []string{
		`{`,
		`{"feature_names":[],"coef":[]}`,
		`{"feature_names":["track_match"],"coef":[1,2]}`,
		`{"feature_names":["unknown"],"coef":[1]}`,
	}
	for _, b := range bad {
		if _, err := Load(write(b)); err == nil {
			t.Fatalf("must reject %s", b)
		}
	}
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file")
	}
	m, err := Load(write(`{"feature_names":["track_match"],"coef":[2],"intercept":-1}`))
	if err != nil || math.Abs(m.Score(map[string]float64{"track_match": 1})-1/(1+math.Exp(-1))) > 1e-12 {
		t.Fatalf("subset of features: %v", err)
	}
}

func TestRuleScoreBounds(t *testing.T) {
	if RuleScore(map[string]float64{"track_match": 1, "tag_overlap_ratio": 1}) != 0.9 {
		t.Fatal("max")
	}
	if RuleScore(map[string]float64{"difficulty_penalty": 3}) != 0 {
		t.Fatal("floor at 0")
	}
}
