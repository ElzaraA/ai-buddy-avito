package agent

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

// kbAgent — агент на реальной базе знаний data/knowledge и mock-LLM (как в демо).
func kbAgent(t *testing.T) (*Agent, []rag.Chunk) {
	t.Helper()
	chunks, err := rag.LoadDir("../../data/knowledge")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New()
	st.SeedChecklist("intern-1", []store.ChecklistItem{{ID: "c1", Title: "a"}})
	reg := tools.NewRegistry()
	reg.Register(&tools.ChecklistGet{S: st})
	reg.Register(&tools.AbsenceCreateRequest{S: st})
	reg.Register(&tools.AbsenceGetPolicy{})
	reg.Register(&tools.AccessRequest{})
	reg.Register(&tools.DirectoryFindOwner{})
	return &Agent{LLM: llm.MockClient{}, Index: rag.NewIndex(chunks), Tools: reg, Now: func() time.Time {
		return time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	}}, chunks
}

func TestFormatKBStructure(t *testing.T) {
	be := contactsFor(intern)
	in := "Мержить можно после апрува. Описание MR обязательно.\nЧто сделать:\n1. Откройте MR.\n2) Дождитесь CI.\nКого спросить: наставника."
	want := "Коротко: Мержить можно после апрува.\nОписание MR обязательно.\n\n" +
		"Что сделать:\n1. Откройте MR.\n2. Дождитесь CI.\n\nКого спросить: наставника."
	if got := be.formatKB(in); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// Маркированный список остаётся маркированным, «Кого спросить» без процедуры не добавляется.
	got := be.formatKB("Стек: Go.\nГде что лежит:\n- логи — в Grafana;\n- задачи — в Jira.")
	if got != "Коротко: Стек: Go.\n\nГде что лежит:\n• логи — в Grafana;\n• задачи — в Jira." {
		t.Fatalf("bullets: %q", got)
	}
	// Одна строка без списка — только «Коротко».
	if got := be.formatKB("Frontend-стек: React, TypeScript."); got != "Коротко: Frontend-стек: React, TypeScript." {
		t.Fatalf("single: %q", got)
	}
}

// Процедура без ответственного: стажёру называем наставника его команды, наставнику и руководителю — нет.
func TestFormatKBFallbackWho(t *testing.T) {
	in := "Порядок такой.\n1. Шаг один.\n2. Шаг два."
	if got := contactsFor(feIntern).formatKB(in); !strings.HasSuffix(got, "Кого спросить: наставник Анна Козлова — если на каком-то шаге застряли.") {
		t.Fatalf("intern: %q", got)
	}
	if got := contactsFor(lead).formatKB(in); strings.Contains(got, "Кого спросить") {
		t.Fatalf("lead: %q", got)
	}
}

func TestSplitSentences(t *testing.T) {
	got := splitSentences("Установите Go 1.22. Напишите в IT Support (Олег Смирнов). «Готово» — и всё")
	want := []string{"Установите Go 1.22.", "Напишите в IT Support (Олег Смирнов).", "«Готово» — и всё"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
}

var markerRe = regexp.MustCompile(`^(Коротко: |\d+\. |• |Кого спросить: )`)

// Ответ по базе знаний: «Коротко» первым, источник — тот фрагмент, из которого ответ собран,
// и ни одной строки, которой нет во фрагменте (кроме словарика терминов).
func TestKBAnswersAreStructuredAndGrounded(t *testing.T) {
	a, chunks := kbAgent(t)
	byID := map[string]string{}
	for _, c := range chunks {
		byID[c.ID] = c.Text
	}
	u := auth.User{ID: "intern-1", Role: auth.RoleIntern, TeamID: "backend"}
	for q, want := range map[string]string{
		"Как у нас устроено код-ревью?":                    "1. Откройте MR в GitLab.",
		"как настроить окружение?":                         "Кого спросить: наставника",
		"Где смотреть логи и метрики?":                     "Коротко: Логи и метрики смотрим в Grafana",
		"Как вести себя на ревью?":                         "Этикет ревью:",
		"Как правильно задать вопрос?":                     "1. Что хотите сделать — одной фразой.",
		"Не работает VPN":                                  "войдите заново через SSO",
		"Что важно в первый рабочий день и первую неделю?": "Познакомьтесь с наставником",
		"Кто мой наставник?":                               "Иван Петров",
		"Я заболел, что делать?":                           "сообщите руководителю",
	} {
		r, err := a.Ask(context.Background(), u, q)
		if err != nil || r.Type != "answer" || len(r.Sources) == 0 {
			t.Fatalf("%q: %+v %v", q, r, err)
		}
		if !strings.Contains(r.Text, "Коротко: ") || !strings.Contains(r.Text, want) {
			t.Fatalf("%q: want %q in\n%s", q, want, r.Text)
		}
		src := byID[r.Sources[0].ID]
		body := r.Text
		if i := strings.Index(body, "Коротко: "); i > 0 {
			body = body[i:] // коучинг «застрял» стоит перед ответом из базы
		}
		if i := strings.Index(body, "\n\n"+glossaryHeader); i >= 0 {
			body = body[:i]
		}
		for _, ln := range strings.Split(body, "\n") {
			ln = markerRe.ReplaceAllString(strings.TrimSpace(ln), "")
			if ln != "" && !strings.Contains(src, ln) {
				t.Fatalf("%q: line %q is not in source %s", q, ln, r.Sources[0].ID)
			}
		}
	}
}

// Застрял + есть ответ в базе: сначала шаги коучинга и правило 15–20 минут, потом структурированный ответ.
func TestStuckCoachingWrapsStructuredAnswer(t *testing.T) {
	a, _ := kbAgent(t)
	u := auth.User{ID: "intern-1", Role: auth.RoleIntern, TeamID: "backend"}
	r, _ := a.Ask(context.Background(), u, "не работает VPN, не могу зайти в GitLab")
	if r.Type != "answer" || !strings.HasPrefix(r.Text, "Застрять в первые недели") ||
		!strings.Contains(r.Text, "15–20 минут") || !strings.Contains(r.Text, "(из базы знаний):\nКоротко: ") {
		t.Fatalf("%q", r.Text)
	}
	if len(r.Suggestions) == 0 || !strings.HasPrefix(r.Suggestions[0], "Передай наставнику:") {
		t.Fatalf("suggestions: %v", r.Suggestions)
	}
}

// Ответ реальной модели (не дословный фрагмент) не переставляем — только словарик терминов.
func TestPresentKeepsModelText(t *testing.T) {
	a, _ := agentWith(`{"type":"answer","text":"Согласуйте отгул с руководителем и сделайте MR."}`, nil)
	r, _ := a.Ask(context.Background(), intern, "оформить отгул")
	if !strings.HasPrefix(r.Text, "Согласуйте отгул с руководителем и сделайте MR.\n\n"+glossaryHeader) || strings.Contains(r.Text, "Коротко") {
		t.Fatalf("%q", r.Text)
	}
}
