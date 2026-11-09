package main

import (
"log"
"net/http"
"time"

"github.com/ElzaraA/ai-buddy-avito/internal/agent"
"github.com/ElzaraA/ai-buddy-avito/internal/api"
"github.com/ElzaraA/ai-buddy-avito/internal/config"
"github.com/ElzaraA/ai-buddy-avito/internal/llm"
"github.com/ElzaraA/ai-buddy-avito/internal/rag"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

func main() {
cfg := config.Load()

chunks, err := rag.LoadDir("data/knowledge")
if err != nil {
log.Fatal("load knowledge: ", err)
}
idx := rag.NewIndex(chunks)

st := store.New()
st.SeedChecklist("intern-1", []store.ChecklistItem{
{ID: "c1", Title: "Познакомиться с командой", Stage: "week1", Done: true},
{ID: "c2", Title: "Получить доступы", Stage: "week1", Done: false},
{ID: "c3", Title: "Прочитать Playbook", Stage: "week2", Done: false},
{ID: "c4", Title: "Взять первую задачу", Stage: "week2", Done: false},
})

reg := tools.NewRegistry()
reg.Register(&tools.ChecklistStatus{S: st})
reg.Register(&tools.AbsenceCreate{S: st})
reg.Register(&tools.DirectoryFind{})

var llmClient llm.Client
if cfg.LLMMock {
llmClient = llm.MockClient{}
} else {
llmClient = &llm.OpenAIClient{
BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel,
HTTP: &http.Client{Timeout: 30 * time.Second},
}
}

ag := &agent.Agent{LLM: llmClient, Index: idx, Tools: reg}
h := &api.Handlers{Agent: ag}

log.Println("listening on", cfg.Addr)
log.Fatal(http.ListenAndServe(cfg.Addr, api.NewRouter(h)))
}
