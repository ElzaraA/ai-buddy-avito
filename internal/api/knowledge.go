package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

const maxKnowledgeQuery = 300

type knowledgeHit struct {
	DocID   string  `json:"doc_id"`
	Title   string  `json:"title"`
	Section string  `json:"section"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// ListKnowledge — статьи базы знаний, доступные пользователю; ?q= — поиск по разделам
// тем же поиском и с теми же правами, что у Бадди.
func (h *Handlers) ListKnowledge(w http.ResponseWriter, r *http.Request) {
	if h.Knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "knowledge base is not loaded")
		return
	}
	u, _ := auth.FromCtx(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"documents": h.Knowledge.List(u.TeamID, string(u.Role))})
		return
	}
	if utf8.RuneCountInString(q) > maxKnowledgeQuery {
		writeError(w, http.StatusBadRequest, "bad_request", "query is too long")
		return
	}
	hits := make([]knowledgeHit, 0)
	if h.Agent != nil && h.Agent.Index != nil {
		for _, hit := range h.Agent.Index.Search(q, u.TeamID, string(u.Role), 8) {
			doc, ok := h.Knowledge.Get(hit.Chunk.Source, u.TeamID, string(u.Role))
			if !ok {
				continue
			}
			hits = append(hits, knowledgeHit{
				DocID: hit.Chunk.Source, Title: doc.Title, Section: hit.Chunk.Title,
				Snippet: snippet(hit.Chunk.Text, 220), Score: hit.Score,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "hits": hits})
}

// GetKnowledge — статья с видимыми пользователю разделами; чужая или закрытая — 404.
func (h *Handlers) GetKnowledge(w http.ResponseWriter, r *http.Request) {
	if h.Knowledge == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "knowledge base is not loaded")
		return
	}
	u, _ := auth.FromCtx(r.Context())
	doc, ok := h.Knowledge.Get(chi.URLParam(r, "id"), u.TeamID, string(u.Role))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "article not found")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func snippet(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndex(string(r), " "); i > n/2 {
		return string(r)[:i] + "…"
	}
	return string(r) + "…"
}
