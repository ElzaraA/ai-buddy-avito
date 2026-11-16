package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tasks"
)

func TestTasksRecommend(t *testing.T) {
	c, err := tasks.LoadCatalog("../../data/tasks/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	svc := &tasks.Service{Catalog: c, Store: store.New()}
	tool := &TasksRecommend{Svc: svc}
	u := auth.DemoUsers["intern.backend"]
	res, err := tool.Run(context.Background(), u, nil)
	if err != nil || !res.OK || !strings.HasPrefix(res.Message, "Советую взять: ") || res.Data["why"] == "" || res.Data["alternatives"] == nil {
		t.Fatalf("%+v %v", res, err)
	}
	if res, _ := tool.Run(context.Background(), auth.DemoUsers["lead.backend"], nil); res.OK {
		t.Fatal("lead gets a polite no")
	}
	if res, err := tool.Run(context.Background(), auth.User{Role: "guest"}, nil); err != nil || res.OK {
		t.Fatal("non-intern gets no recommendations")
	}
	// все задачи взяты — честный ответ
	for _, v := range svc.List(u) {
		svc.Skip(u, v.ID)
	}
	if res, _ := tool.Run(context.Background(), u, nil); !res.OK || !strings.Contains(res.Message, "спросите наставника") {
		t.Fatalf("empty: %+v", res)
	}
	if tool.Name() != "tasks_recommend" || tool.RequiresConfirmation() || tool.RequiredPermission() != "tasks.read" {
		t.Fatal("meta")
	}
}
