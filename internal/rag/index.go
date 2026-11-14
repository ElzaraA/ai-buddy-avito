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
		скажи покажи какую какое каких каким какого какому устроен устроено устроена устроены
		такое такой такая такие смотреть посмотреть
		the a an of to in on and or is are what how`) {
		stopwords[w] = true
	}
}

// synonyms — транслитерация, опечатки и разговорные формы → слово из базы знаний (по префиксу).
// Применяется и к запросу, и к фрагментам, поэтому обе стороны приходят к одной форме.
var synonyms = []struct{ prefix, canon string }{
	{"стэндап", "стендап"},
	{"фронтенд", "frontend"}, {"фронт", "frontend"},
	{"бэкенд", "backend"}, {"бекенд", "backend"},
	{"мердж", "merge"}, {"мерж", "merge"},
	{"гитлаб", "gitlab"}, {"джир", "jira"}, {"графан", "grafana"},
	{"поставить", "установить"}, {"ставить", "установить"},
	{"поднима", "поднять"}, {"подня", "поднять"},
}

func canonical(t string) string {
	for _, s := range synonyms {
		if strings.HasPrefix(t, s.prefix) {
			return s.canon
		}
	}
	return t
}

// terms — токены без стоп-слов и без слишком коротких (1 символ) слов, с приведением синонимов.
func terms(s string) []string {
	toks := tokenize(s)
	out := toks[:0]
	for _, t := range toks {
		if len([]rune(t)) < 2 || stopwords[t] || isNumber(t) {
			continue
		}
		out = append(out, canonical(t))
	}
	return out
}

// Meaningful — есть ли в тексте значимые слова (не только стоп-слова, числа и знаки).
func Meaningful(s string) bool { return len(terms(s)) > 0 }

// endings — типовые окончания русских слов (облегчённый стемминг), от длинных к коротким.
var endings = []string{
	"ться", "тся", "ает", "яет", "ают", "яют", "аёт", "ить", "ать", "ять", "еть",
	"ами", "ями", "ого", "его", "ему", "ому", "ыми", "ими",
	"ую", "юю", "ая", "яя", "ой", "ей", "ий", "ый", "ом", "ем", "ов", "ев", "ах", "ях", "ам", "ям", "ия", "ие", "ью",
	"ёт", "ет", "ут", "ют", "ит", "ат", "ят", "ть",
	"у", "а", "я", "ы", "и", "е", "о", "ь", "ю",
}

// stem отрезает окончание, оставляя основу не короче трёх букв: «базу», «база» → «баз».
func stem(w string) string {
	r := []rune(w)
	for _, e := range endings {
		re := []rune(e)
		if len(r)-len(re) >= 3 && strings.HasSuffix(w, e) {
			return string(r[:len(r)-len(re)])
		}
	}
	return w
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

// termMatch — слово запроса и слово фрагмента совпадают: точно, по общему префиксу
// или по основе после отрезания окончания («базу» ~ «база»).
func termMatch(a, b string) bool {
	if prefixMatch(a, b) {
		return true
	}
	sa, sb := stem(a), stem(b)
	return len([]rune(sa)) >= 3 && sa == sb
}

const (
	minScore = 0.5
	// minCoverage — доля «веса» запроса (сумма IDF значимых слов), которая должна найтись во фрагменте.
	// Частые слова («стажёр», «заявка», «получить») весят мало и не тянут нерелевантный ответ.
	minCoverage = 0.6
	// shortQuery — в запросе из стольких значимых слов (и меньше) должны совпасть все.
	shortQuery = 2
)

func idfOf(n, df float64) float64 {
	return math.Log(1 + (n-df+0.5)/(df+0.5))
}

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

	if len(qt) == 0 {
		return nil
	}
	// Вес слова запроса — его IDF по базе: в скольких фрагментах оно встречается.
	// Слово, которого нет в базе, весит как редкое (df=1): оно важно, и без него ответа нет.
	weights := make([]float64, len(qt))
	total := 0.0
	for j, t := range qt {
		df := 0
		for i := range idx.chunks {
			for ct := range idx.tf[i] {
				if termMatch(t, ct) {
					df++
					break
				}
			}
		}
		if df == 0 {
			df = 1
		}
		weights[j] = idfOf(n, float64(df))
		total += weights[j]
	}

	var hits []Hit
	for i, c := range idx.chunks {
		if teamID != "" && c.TeamID != "" && c.TeamID != teamID {
			continue
		}
		if !allowed(c, role) {
			continue
		}
		score, covered := 0.0, 0.0
		matched := 0
		for j, t := range qt {
			// Все словоформы слова запроса во фрагменте складываются в одну частоту,
			// а вес берётся у слова запроса: редкая словоформа («наставником») не перевешивает тему фрагмента.
			f := 0
			for ct, cnt := range idx.tf[i] {
				if termMatch(t, ct) {
					f += cnt
				}
			}
			if f == 0 {
				continue
			}
			matched++
			covered += weights[j]
			score += weights[j] * (float64(f) * 2.2) / (float64(f) + 1.2*(0.25+0.75*float64(idx.lens[i])/avg))
		}
		if score < minScore {
			continue
		}
		if len(qt) <= shortQuery && matched < len(qt) {
			continue
		}
		if total > 0 && covered/total < minCoverage {
			continue
		}
		hits = append(hits, Hit{Chunk: c, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}
