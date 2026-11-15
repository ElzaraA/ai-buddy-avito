package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
)

func TestCoachSignalToMentor(t *testing.T) {
	a, _ := agentWith("", nil)
	a.LLM = llm.MockClient{}
	resp, err := a.Ask(context.Background(), intern, "Передай наставнику: не работает VPN, не могу зайти в GitLab")
	if err != nil || resp.Type != "action" || resp.Tool != SignalTool {
		t.Fatalf("%+v %v", resp, err)
	}
	if resp.Signal != "не работает VPN, не могу зайти в GitLab" || resp.SignalTopic != "tech" {
		t.Fatalf("signal %q topic %q", resp.Signal, resp.SignalTopic)
	}
}

func TestCoachVagueGetsMenu(t *testing.T) {
	a, _ := agentWith("", nil)
	a.LLM = llm.MockClient{}
	for _, q := range []string{"помоги", "С чего начать?", "что ты умеешь", "я новенький"} {
		resp, _ := a.Ask(context.Background(), intern, q)
		if resp.Type != "answer" || !strings.Contains(resp.Text, "Передай наставнику") || len(resp.Suggestions) < 3 {
			t.Fatalf("%q: %+v", q, resp)
		}
	}
	// с предметом — это уже не размытый запрос
	if isVague("помоги оформить отгул на завтра") {
		t.Fatal("request with action word must not be vague")
	}
}

func TestCoachStuckWithoutAnswerGivesSteps(t *testing.T) {
	a, _ := agentWith("", nil)
	a.LLM = llm.MockClient{}
	resp, _ := a.Ask(context.Background(), intern, "у меня не запускается проект локально")
	if resp.Type != "answer" || !strings.Contains(resp.Text, "15–20 минут") {
		t.Fatalf("%+v", resp)
	}
	if len(resp.Suggestions) == 0 || !strings.HasPrefix(resp.Suggestions[0], "Передай наставнику:") {
		t.Fatalf("suggestions: %v", resp.Suggestions)
	}
}

func TestCoachEveryInternAnswerHasNextStep(t *testing.T) {
	a, _ := agentWith("", nil)
	a.LLM = llm.MockClient{}
	for _, q := range []string{"Как оформить отгул?", "Какой следующий шаг по чек-листу?", "Сколько платят?"} {
		resp, _ := a.Ask(context.Background(), intern, q)
		if len(resp.Suggestions) == 0 {
			t.Fatalf("%q: no suggestions: %+v", q, resp)
		}
	}
	// наставнику подсказки стажёра не нужны
	mentor := auth.User{ID: "m1", Role: auth.RoleBuddy, TeamID: "backend"}
	if resp, _ := a.Ask(context.Background(), mentor, "передай наставнику: тест"); resp.Tool == SignalTool {
		t.Fatal("mentor must not create signals")
	}
}
