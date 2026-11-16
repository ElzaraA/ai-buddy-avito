package rag

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Section — раздел статьи базы знаний.
type Section struct {
	Title string `json:"title"`
	Stage string `json:"stage,omitempty"`
	Text  string `json:"text"`
}

// Doc — статья базы знаний: только разделы, которые видит пользователь.
type Doc struct {
	ID         string    `json:"id"` // имя файла, например tools.md — так же, как source в ответах Бадди
	Title      string    `json:"title"`
	Stages     []string  `json:"stages"`
	Sections   []Section `json:"sections,omitempty"`
	Disclaimer string    `json:"disclaimer"`
}

// Library — база знаний для просмотра: те же фрагменты и те же правила доступа, что у поиска Бадди.
type Library struct {
	chunks []Chunk
	titles map[string]string
	order  []string
}

// Disclaimer — пометка о синтетических инструкциях прототипа.
const Disclaimer = "Тестовая инструкция для прототипа AI-buddy. Не является регламентом Авито."

// NewLibrary — библиотека по фрагментам и заголовкам файлов (# в первой строке-заголовке).
func NewLibrary(chunks []Chunk, dir string) *Library {
	l := &Library{chunks: chunks, titles: map[string]string{}}
	seen := map[string]bool{}
	for _, c := range chunks {
		if !seen[c.Source] {
			seen[c.Source] = true
			l.order = append(l.order, c.Source)
		}
	}
	sort.Strings(l.order)
	for _, src := range l.order {
		b, err := os.ReadFile(filepath.Join(dir, src))
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(ln, "# ") {
				l.titles[src] = strings.TrimSpace(strings.TrimPrefix(ln, "# "))
				break
			}
		}
	}
	return l
}

// Visible — тот же фильтр, что у поиска: роль в audience (deny-by-default) и своя команда.
func Visible(c Chunk, teamID, role string) bool {
	return allowed(c, role) && (c.TeamID == "" || teamID == "" || c.TeamID == teamID)
}

func (l *Library) doc(src, teamID, role string, withText bool) (Doc, bool) {
	d := Doc{ID: src, Title: l.titles[src], Disclaimer: Disclaimer}
	stages := map[string]bool{}
	for _, c := range l.chunks {
		if c.Source != src || !Visible(c, teamID, role) {
			continue
		}
		if c.Stage != "" && !stages[c.Stage] {
			stages[c.Stage] = true
			d.Stages = append(d.Stages, c.Stage)
		}
		if withText {
			d.Sections = append(d.Sections, Section{Title: c.Title, Stage: c.Stage, Text: c.Text})
		}
	}
	if len(stages) == 0 && (!withText || len(d.Sections) == 0) {
		return Doc{}, false
	}
	if d.Title == "" {
		d.Title = src
	}
	sort.Strings(d.Stages)
	return d, true
}

// List — статьи, в которых пользователю виден хотя бы один раздел.
func (l *Library) List(teamID, role string) []Doc {
	out := make([]Doc, 0)
	for _, src := range l.order {
		if d, ok := l.doc(src, teamID, role, true); ok {
			d.Sections = nil
			out = append(out, d)
		}
	}
	return out
}

// Get — статья с видимыми разделами; чужая или закрытая — false.
func (l *Library) Get(src, teamID, role string) (Doc, bool) {
	return l.doc(src, teamID, role, true)
}
