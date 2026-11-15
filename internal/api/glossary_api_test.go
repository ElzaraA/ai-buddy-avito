package api

import (
	"strings"
	"testing"
)

// Вопрос «что такое X?» отвечается из словаря Бадди и не попадает в «пробелы базы знаний».
func TestGlossaryQuestionIsNotAGap(t *testing.T) {
	srv, st, _ := r2Server(t)
	for _, q := range []string{"что такое MR?", "Что значит ретро", "Что такое OKR?"} {
		r := ask(t, srv, "intern.backend", q)
		text, _ := r["text"].(string)
		if r["type"] != "answer" || !strings.HasPrefix(text, "Из словаря Бадди: ") {
			t.Fatalf("%q: %v", q, r)
		}
	}
	if g := st.Gaps("backend"); len(g) != 0 {
		t.Fatalf("glossary question must not be a gap: %+v", g)
	}
}
