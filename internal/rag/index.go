package rag

import (
	"math"
	"sort"
	"strings"
)

type Index struct {
	chunks []Chunk
	df     map[string]int
	tf     []map[string]int
	lens   []int
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	return strings.FieldsFunc(s, func(r rune) bool {
		if r == 'ё' {
			return false
		}
		return !(r >= 'a' && r <= 'z') && !(r >= 'а' && r <= 'я') && !(r >= '0' && r <= '9')
	})
}

// stopwords — служебные и вопросительные слова, которые не несут смысла для поиска.
// Без них «Сколько платят в Авито?» находило бы фрагменты по предлогу «в».
var stopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`в во на и или а но с со к ко по у о об от до из за для при про над под
		не ни же ли бы то это этот эта эти тот та те так там тут где когда как какой какая какие каков
		что чем чего кто кого сколько почему зачем мне меня мой моя мои нам нас наш наша вам вас ваш вы
		я ты он она оно мы они его её их все всё весь есть был была было быть будет можно нужно надо
		ли уже еще ещё очень хочу хотел хотела нужен нужна нужны пожалуйста можешь подскажи расскажи
		скажи покажи the a an of to in on and or is are what how`) {
		stopwords[w] = true
	}
}

// terms — токены без стоп-слов и без слишком коротких (1 символ) слов.
func terms(s string) []string {
	toks := tokenize(s)
	out := toks[:0]
	for _, t := range toks {
		if len([]rune(t)) < 2 || stopwords[t] || isNumber(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func NewIndex(chunks []Chunk) *Index {
	idx := &Index{chunks: chunks, df: map[string]int{}}
	for _, c := range chunks {
		tf := map[string]int{}
		toks := terms(c.Title + " " + c.Text + " " + c.Stage)
		for _, t := range toks {
			tf[t]++
		}
		for t := range tf {
			idx.df[t]++
		}
		idx.tf = append(idx.tf, tf)
		idx.lens = append(idx.lens, len(toks))
	}
	return idx
}

type Hit struct {
	Chunk Chunk
	Score float64
}

func prefixMatch(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	n := 5
	if len(ra) < n {
		n = len(ra)
	}
	if len(rb) < n {
		n = len(rb)
	}
	if n < 4 {
		return false
	}
	return string(ra[:n]) == string(rb[:n])
}

const (
	minScore = 0.5
	// minCoverage — доля значимых слов запроса, которые должны найтись во фрагменте.
	// Одно общее слово («получить») не делает фрагмент релевантным.
	minCoverage = 0.5
)

// allowed — deny-by-default: если у чанка пустой audience — невидим.
func allowed(c Chunk, role string) bool {
	if len(c.Audience) == 0 {
		return false
	}
	for _, a := range c.Audience {
		if a == role || a == "all" {
			return true
		}
	}
	return false
}

func (idx *Index) Search(q string, teamID string, role string, k int) []Hit {
	qt := terms(q)
	n := float64(len(idx.chunks))
	avg := 0.0
	for _, l := range idx.lens {
		avg += float64(l)
	}
	if n > 0 {
		avg /= n
	}
	if avg == 0 {
		avg = 1
	}

	var hits []Hit
	for i, c := range idx.chunks {
		if teamID != "" && c.TeamID != "" && c.TeamID != teamID {
			continue
		}
		if !allowed(c, role) {
			continue
		}
		score := 0.0
		matched := 0
		for _, t := range qt {
			hit := false
			for ct, f := range idx.tf[i] {
				if prefixMatch(t, ct) {
					hit = true
					df := float64(idx.df[ct])
					idf := math.Log(1 + (n-df+0.5)/(df+0.5))
					score += idf * (float64(f) * 2.2) / (float64(f) + 1.2*(0.25+0.75*float64(idx.lens[i])/avg))
				}
			}
			if hit {
				matched++
			}
		}
		if len(qt) == 0 {
			continue
		}
		if score >= minScore && float64(matched)/float64(len(qt)) >= minCoverage {
			hits = append(hits, Hit{Chunk: c, Score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}
