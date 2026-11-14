package store

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
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

var (
	emailRe = regexp.MustCompile(`[\p{L}\d._%+\-]+@[\p{L}\d.\-]+\.\p{L}{2,}`)
	// numberRe — число с разделителями: телефон «+7 (999) 123-45-67», паспорт «4510 123456», счёт.
	numberRe = regexp.MustCompile(`[+(]?\d[\d\s()\-]*\d\)?`)
)

// minMaskedDigits — последовательность из стольких цифр и больше считается персональными данными.
const minMaskedDigits = 6

// MaskPII убирает из текста персональные данные: email → [email], телефоны, номера паспортов и прочие
// последовательности из 6+ цифр (с пробелами, дефисами, скобками, плюсом) → [число].
// Пробелы базы знаний видит наставник — личные данные стажёра туда попадать не должны.
func MaskPII(q string) string {
	q = emailRe.ReplaceAllString(q, "[email]")
	return numberRe.ReplaceAllStringFunc(q, func(m string) string {
		n := 0
		for _, r := range m {
			if unicode.IsDigit(r) {
				n++
			}
		}
		if n < minMaskedDigits {
			return m
		}
		return "[число]"
	})
}

// RecordGap запоминает вопрос без ответа для команды (с замаскированными персональными данными).
func (s *Store) RecordGap(teamID, question string) {
	question = MaskPII(question)
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
