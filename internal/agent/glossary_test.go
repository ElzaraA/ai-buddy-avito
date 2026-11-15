package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

func TestGlossarySize(t *testing.T) {
	if len(glossary) < 25 {
		t.Fatalf("glossary has %d terms, want ≥ 25", len(glossary))
	}
	seen := map[string]bool{}
	for _, g := range glossary {
		if g.Name == "" || g.Plain == "" || len(g.Aliases) == 0 || seen[g.Name] {
			t.Fatalf("bad term %+v", g)
		}
		seen[g.Name] = true
	}
}

func TestGlossaryQuestion(t *testing.T) {
	for q, want := range map[string]string{
		"что такое MR?":        "MR (merge request)",
		"Что значит ретро":     "Ретро",
		"что означает DRI?":    "DRI",
		"OKR — это что?":       "OKR",
		"а что такое апрув?":   "Апрув",
		"Что такое фича-флаг?": "Фича-флаг",
		"что такое 1:1":        "1:1",
		"Что такое прод?":      "Прод",
		"расшифруй SSO":        "SSO",
		"что за стейдж":        "Стейдж",
		"Что такое «бэклог»?":  "Бэклог",
		"что значит смержить?": "Мерж",
		"Что такое спринты?":   "Спринт",
	} {
		g, _, ok := glossaryQuestion(q)
		if !ok || g.Name != want {
			t.Fatalf("%q: got %q %v", q, g.Name, ok)
		}
	}
	// Вопрос о правилах команды — к базе знаний, а не к словарю.
	for _, q := range []string{"Что такое Jira у нас?", "Как у нас устроено код-ревью?", "что такое кафка", "MR", "что такое продукт", "когда можно мержить?"} {
		if _, _, ok := glossaryQuestion(q); ok {
			t.Fatalf("%q must not be a glossary question", q)
		}
	}
}

func TestGlossaryAnswer(t *testing.T) {
	a, _ := kbAgent(t)
	u := auth.User{ID: "intern-1", Role: auth.RoleIntern, TeamID: "backend"}
	// Термин есть в базе команды — дословное предложение оттуда и источник.
	r, err := a.Ask(context.Background(), u, "что значит ретро?")
	if err != nil || r.Type != "answer" || r.NoSource || !strings.HasPrefix(r.Text, glossaryMark+"Ретро — ") ||
		!strings.Contains(r.Text, "Как у нас в команде: ретро — в последнюю пятницу спринта.") || len(r.Sources) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	// Термина нет в базе — только словарь, без источника и без отказа.
	r, _ = a.Ask(context.Background(), u, "Что такое OKR?")
	if r.Type != "answer" || r.NoSource || len(r.Sources) != 0 || !strings.HasPrefix(r.Text, glossaryMark+"OKR — ") || len(r.Suggestions) == 0 {
		t.Fatalf("%+v", r)
	}
	// Стажёру другой команды не показываем предложения из базы Backend.
	fe := auth.User{ID: "intern-2", Role: auth.RoleIntern, TeamID: "frontend"}
	if r, _ = a.Ask(context.Background(), fe, "что такое стендап?"); strings.Contains(r.Text, "11:00") || len(r.Sources) != 0 {
		t.Fatalf("frontend: %+v", r)
	}
	// Вопрос-определение не пробел базы знаний.
	for _, q := range []string{"что такое MR?", "что значит ретро", "Что такое DRI?"} {
		if isKnowledgeGap(q) {
			t.Fatalf("%q must not be a knowledge gap", q)
		}
	}
	if !isKnowledgeGap("что такое кафка") {
		t.Fatal("unknown term is still a gap")
	}
}

func TestWithGlossary(t *testing.T) {
	got := withGlossary("Откройте MR, дождитесь CI и апрува, потом деплой.")
	want := "Откройте MR, дождитесь CI и апрува, потом деплой.\n\n" + glossaryHeader +
		"\n• MR (merge request) — " + glossary[0].Plain +
		"\n• CI — " + glossary[2].Plain +
		"\n• Апрув — " + glossary[6].Plain
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
	// Слова-омонимы не путаем с терминами.
	for _, s := range []string{"Продукт и продажи", "Логин и пароль", "Мария Сидорова", "Ретроградный", "Ciao"} {
		if out := withGlossary(s); out != s {
			t.Fatalf("%q: false positive %q", s, out)
		}
	}
	// Блок не дублируется.
	if withGlossary(got) != got {
		t.Fatal("glossary appended twice")
	}
}
