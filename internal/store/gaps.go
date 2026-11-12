package store

import (
	"sort"
	"strings"
	"time"
)

// Gap — вопрос, на который в базе знаний не нашлось ответа.
// Копится без привязки к пользователю: наставник видит, что дописать в базу, но не кто спрашивал.
type Gap struct {
	Question string    `json:"question"`
	Count    int       `json:"count"`
	LastAt   time.Time `json:"last_at"`
}

const (
	maxGapsPerTeam = 200
	maxGapLen      = 200
)

func normalizeQuestion(q string) string {
	q = strings.ToLower(strings.Join(strings.Fields(q), " "))
	return strings.TrimRight(q, "?!. ")
}

// RecordGap запоминает вопрос без ответа для команды.
func (s *Store) RecordGap(teamID, question string) {
	key := normalizeQuestion(question)
	if key == "" {
		return
	}
	if r := []rune(question); len(r) > maxGapLen {
		question = string(r[:maxGapLen]) + "…"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	team := s.gaps[teamID]
	if team == nil {
		team = map[string]*Gap{}
		s.gaps[teamID] = team
	}
	g := team[key]
	if g == nil {
		if len(team) >= maxGapsPerTeam {
			return // защита от разрастания
		}
		g = &Gap{Question: strings.TrimSpace(question)}
		team[key] = g
	}
	g.Count++
	g.LastAt = s.now()
	s.touch()
}

// Gaps — вопросы без ответа, самые частые сверху.
func (s *Store) Gaps(teamID string) []Gap {
	s.mu.RLock()
	out := make([]Gap, 0, len(s.gaps[teamID]))
	for _, g := range s.gaps[teamID] {
		out = append(out, *g)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Question < out[j].Question
	})
	return out
}
