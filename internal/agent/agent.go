package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

const BuddyName = "Иван Петров"
const ManagerName = "Мария Сидорова"

type Agent struct {
	LLM   llm.Client
	Index *rag.Index
	Tools *tools.Registry
	Now   func() time.Time
}

type Response struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	Action  string            `json:"action,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Result  *tools.Result     `json:"result,omitempty"`
	Sources []Source          `json:"sources,omitempty"`
	Buddy   string            `json:"buddy_name,omitempty"`
	Manager string            `json:"manager_name,omitempty"`
}

func (a *Agent) Ask(ctx context.Context, user auth.User, question string) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	hits := a.Index.Search(question, user.TeamID, string(user.Role), 4)
	srcs := make([]Source, 0, len(hits))
	for _, h := range hits {
		srcs = append(srcs, Source{ID: h.Chunk.ID, Source: h.Chunk.Source, Stage: h.Chunk.Stage})
	}
	if len(srcs) == 0 {
		return Response{
			Type:  "refusal",
			Text:  "Не нашёл в доступных источниках. Обратитесь к наставнику " + BuddyName + ".",
			Buddy: BuddyName,
		}, nil
	}

	msgs := []llm.Message{
		{Role: "system", Content: SystemPrompt},
		{Role: "user", Content: UserPrompt(question, hits, string(user.Role))},
	}
	raw, err := a.LLM.Complete(ctx, msgs)
	if err != nil {
		return Response{}, err
	}

	raw = stripCodeFence(raw)
	var out AgentOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return Response{Type: "answer", Text: raw, Sources: srcs}, nil
	}

	switch out.Type {
	case "tool_call":
		t, ok := a.Tools.Get(out.Tool)
		if !ok {
			return Response{Type: "refusal", Text: "Инструмент недоступен. Обратитесь к наставнику " + BuddyName + ".", Buddy: BuddyName}, nil
		}
		if t.RequiresConfirmation() {
			return Response{
				Type:    "draft",
				Action:  "absence",
				Args:    out.Args,
				Text:    "Черновик. Подтвердите через POST /api/v1/actions/{id}/confirm.",
				Sources: srcs,
				Buddy:   BuddyName,
				Manager: ManagerName,
			}, nil
		}
		res, err := tools.Run(ctx, t, user, out.Args)
		if err != nil {
			return Response{Type: "refusal", Text: "Действие недоступно. Наставник: " + BuddyName + ".", Buddy: BuddyName}, nil
		}
		return Response{Type: "action", Result: &res, Sources: srcs, Buddy: BuddyName, Manager: ManagerName}, nil

	case "draft":
		return a.buildDraft(question, out, srcs), nil

	case "refusal":
		return Response{Type: "refusal", Text: out.Text, Buddy: BuddyName}, nil
	}
	return Response{Type: "answer", Text: out.Text, Sources: srcs}, nil
}

func (a *Agent) buildDraft(question string, out AgentOutput, srcs []Source) Response {
	if out.Action != "absence" {
		return Response{Type: "refusal", Text: "Неизвестное действие. Наставник: " + BuddyName + ".", Buddy: BuddyName}
	}
	from, to := ParseDates(question)
	if from == "" {
		now := time.Now()
		if a.Now != nil {
			now = a.Now()
		}
		if d, ok := ParseWeekday(question, now); ok {
			from, to = d, d
		}
	}
	base := Response{
		Type:    "draft",
		Action:  "absence",
		Sources: srcs,
		Buddy:   BuddyName,
		Manager: ManagerName,
	}
	if from == "" {
		base.Args = map[string]string{"reason": "personal"}
		base.Text = "На какую дату оформить отгул? Укажите в формате 2026-11-20. Руководитель: " + ManagerName + ", наставник: " + BuddyName + "."
		return base
	}
	base.Args = map[string]string{
		"date_from": from, "date_to": to, "reason": "personal",
		"suggest_notify_buddy": "true",
	}
	base.Text = "Черновик заявки. Согласуйте с руководителем " + ManagerName + " и предупредите наставника " + BuddyName + ". Подтвердите через POST /api/v1/actions/{id}/confirm."
	return base
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
