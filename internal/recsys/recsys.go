// Package recsys — рекомендации задач адаптации стажёру.
//
// Модель — логистическая регрессия, обученная офлайн в ml/recsys (train.py) на синтетических
// взаимодействиях стажёров с задачами. Python нужен только для обучения: веса экспортируются
// в JSON (extract_coefs.py), а здесь скоринг считается тем же набором признаков, что
// ml/recsys/features.py. Если файла весов нет или он битый — прозрачное правило (rule_based),
// и в ответе честно указан источник.
package recsys

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// Признаки — строго как FEATURE_NAMES в ml/recsys/features.py.
var knownFeatures = map[string]bool{
	"tag_overlap_count": true, "tag_overlap_ratio": true, "avg_competency_level": true,
	"max_competency_level": true, "track_match": true, "task_difficulty": true, "skill_gap": true,
	"difficulty_penalty": true, "task_popularity": true, "intern_prior_pos": true, "intern_prior_neg": true,
}

// Model — веса логистической регрессии.
type Model struct {
	names     []string
	coef      []float64
	intercept float64
}

type artifact struct {
	FeatureNames []string  `json:"feature_names"`
	Coef         []float64 `json:"coef"`
	Intercept    float64   `json:"intercept"`
}

// Load читает веса, экспортированные ml/recsys/extract_coefs.py.
func Load(path string) (*Model, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a artifact
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("recsys: %s: %w", path, err)
	}
	if len(a.FeatureNames) == 0 || len(a.FeatureNames) != len(a.Coef) {
		return nil, errors.New("recsys: feature_names and coef must be non-empty and of equal length")
	}
	for _, n := range a.FeatureNames {
		if !knownFeatures[n] {
			return nil, fmt.Errorf("recsys: unknown feature %q — обновите Featurize вслед за features.py", n)
		}
	}
	for _, c := range append(append([]float64{}, a.Coef...), a.Intercept) {
		if math.IsNaN(c) || math.IsInf(c, 0) {
			return nil, errors.New("recsys: NaN/Inf in weights")
		}
	}
	return &Model{names: a.FeatureNames, coef: a.Coef, intercept: a.Intercept}, nil
}

// Profile — трек (команда) и уровни компетенций стажёра 1..5.
type Profile struct {
	Track        string
	Competencies map[string]int
}

// Candidate — задача-кандидат.
type Candidate struct {
	ID         string
	Track      string
	Difficulty int // 1..5
	Tags       []string
	Popularity int // сколько стажёров уже выполнили
}

// Prior — история стажёра: сделанные и отложенные задачи.
type Prior struct{ Pos, Neg int }

// Ranked — рекомендация с объяснением.
type Ranked struct {
	ID      string   `json:"task_id"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}

// Result — ранжированный список и его источник.
type Result struct {
	Source string   `json:"source"` // model | rule_based
	Model  string   `json:"model,omitempty"`
	Items  []Ranked `json:"items"`
}

const (
	SourceModel = "model"
	SourceRules = "rule_based"
	// ModelName — версия весов в data/recsys/model.json (ml/recsys/train.py, seed 42).
	ModelName = "logreg-v1"
)

// Features — признаки пары «стажёр — задача» по именам (как features.featurize).
func Features(p Profile, c Candidate, prior Prior) map[string]float64 {
	sum, maxLvl := 0, 0
	for _, l := range p.Competencies {
		sum += l
		if l > maxLvl {
			maxLvl = l
		}
	}
	avg := 0.0
	if len(p.Competencies) > 0 {
		avg = float64(sum) / float64(len(p.Competencies))
	}
	overlap := len(overlapTags(p, c))
	ratio := float64(overlap) / float64(max(1, len(uniq(c.Tags))))
	gap := float64(c.Difficulty) - avg
	track := 0.0
	if p.Track == c.Track {
		track = 1
	}
	return map[string]float64{
		"tag_overlap_count":    float64(overlap),
		"tag_overlap_ratio":    ratio,
		"avg_competency_level": avg,
		"max_competency_level": float64(maxLvl),
		"track_match":          track,
		"task_difficulty":      float64(c.Difficulty),
		"skill_gap":            gap,
		"difficulty_penalty":   math.Max(0, gap-1),
		"task_popularity":      float64(c.Popularity),
		"intern_prior_pos":     float64(prior.Pos),
		"intern_prior_neg":     float64(prior.Neg),
	}
}

// Score — вероятность того, что задача подойдёт стажёру (sigmoid(w·x + b)).
func (m *Model) Score(f map[string]float64) float64 {
	z := m.intercept
	for i, n := range m.names {
		z += m.coef[i] * f[n]
	}
	return 1 / (1 + math.Exp(-z))
}

// RuleScore — прозрачное правило без модели: трек, пересечение технологий, штраф за слишком сложное.
func RuleScore(f map[string]float64) float64 {
	s := 0.4*f["track_match"] + 0.5*f["tag_overlap_ratio"] - 0.15*math.Min(1, f["difficulty_penalty"])
	return math.Max(0, math.Min(1, s))
}

// Rank ранжирует кандидатов; m == nil — правило вместо модели. k <= 0 — все.
func Rank(m *Model, p Profile, cands []Candidate, prior Prior, k int) Result {
	res := Result{Source: SourceRules, Items: make([]Ranked, 0, len(cands))}
	if m != nil {
		res.Source, res.Model = SourceModel, ModelName
	}
	for _, c := range cands {
		f := Features(p, c, prior)
		s := RuleScore(f)
		if m != nil {
			s = m.Score(f)
		}
		res.Items = append(res.Items, Ranked{ID: c.ID, Score: math.Round(s*1000) / 1000, Reasons: Explain(p, c)})
	}
	// при равенстве — по id: выдача детерминирована
	sort.SliceStable(res.Items, func(i, j int) bool {
		if res.Items[i].Score != res.Items[j].Score {
			return res.Items[i].Score > res.Items[j].Score
		}
		return res.Items[i].ID < res.Items[j].ID
	})
	if k > 0 && len(res.Items) > k {
		res.Items = res.Items[:k]
	}
	return res
}

// Explain — почему задача подходит, человеческими словами (по тем же признакам, что видит модель).
func Explain(p Profile, c Candidate) []string {
	var out []string
	if common := overlapTags(p, c); len(common) > 0 {
		out = append(out, "Знакомые технологии: "+strings.Join(common, ", "))
	}
	f := Features(p, c, Prior{})
	switch gap := f["skill_gap"]; {
	case len(p.Competencies) == 0:
		out = append(out, fmt.Sprintf("Сложность %d из 5", c.Difficulty))
	case gap <= 0:
		out = append(out, fmt.Sprintf("По силам: сложность %d из 5 не выше вашего уровня", c.Difficulty))
	case gap <= 1:
		out = append(out, fmt.Sprintf("Чуть выше вашего уровня — хорошо для роста (сложность %d из 5)", c.Difficulty))
	default:
		out = append(out, fmt.Sprintf("Сложная задача (%d из 5) — лучше взять с наставником", c.Difficulty))
	}
	if c.Popularity >= 15 {
		out = append(out, fmt.Sprintf("Её уже сделали %d стажёров — есть у кого спросить", c.Popularity))
	}
	return out
}

func overlapTags(p Profile, c Candidate) []string {
	var out []string
	for _, t := range uniq(c.Tags) {
		if _, ok := p.Competencies[t]; ok {
			out = append(out, t)
		}
	}
	return out
}

func uniq(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
