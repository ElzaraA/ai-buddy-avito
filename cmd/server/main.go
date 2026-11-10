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
config.LoadDotEnv(".env")
cfg := config.Load()

chunks, err := rag.LoadDir("data/knowledge")
if err != nil {
log.Fatal("load knowledge: ", err)
}
idx := rag.NewIndex(chunks)

st := store.New()
st.SeedChecklist("intern-1", []store.ChecklistItem{
{ID: "c1-intern-1", Title: "Познакомиться с командой", Stage: "week1", Done: true},
{ID: "c2-intern-1", Title: "Получить доступы", Stage: "week1", Done: false},
{ID: "c3-intern-1", Title: "Прочитать Playbook", Stage: "week2", Done: false},
{ID: "c4-intern-1", Title: "Взять первую задачу", Stage: "week2", Done: false},
})
st.SeedChecklist("intern-2", []store.ChecklistItem{
{ID: "c1-intern-2", Title: "Познакомиться с командой", Stage: "week1", Done: false},
})

reg := tools.NewRegistry()
reg.Register(&tools.ChecklistGet{S: st})
reg.Register(&tools.AbsenceCreateRequest{S: st})
reg.Register(&tools.DirectoryFindOwner{})
reg.Register(&tools.AbsenceGetPolicy{})
reg.Register(&tools.AccessRequest{})

var llmClient llm.Client
if cfg.LLMMock {
log.Println("LLM mode: mock")
llmClient = llm.MockClient{}
} else {
log.Println("LLM mode: real, model=", cfg.LLMModel)
llmClient = &llm.OpenAIClient{
BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel,
HTTP: &http.Client{Timeout: 60 * time.Second},
}
}

ag := &agent.Agent{LLM: llmClient, Index: idx, Tools: reg, Now: time.Now}

msgs := store.NewMessageStore()
broker := api.NewBroker()
chat := &api.ChatHandlers{Agent: ag, Store: st, Msgs: msgs, Broker: broker}
handlers := &api.Handlers{Agent: ag, Store: st, Chat: chat}

log.Println("listening on", cfg.Addr)
log.Fatal(http.ListenAndServe(cfg.Addr, api.NewRouter(handlers)))
}