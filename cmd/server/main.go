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
{ID: "c1", Title: "РџРѕР·РЅР°РєРѕРјРёС‚СЊСЃСЏ СЃ РєРѕРјР°РЅРґРѕР№", Stage: "week1", Done: true},
{ID: "c2", Title: "РџРѕР»СѓС‡РёС‚СЊ РґРѕСЃС‚СѓРїС‹", Stage: "week1", Done: false},
{ID: "c3", Title: "РџСЂРѕС‡РёС‚Р°С‚СЊ Playbook", Stage: "week2", Done: false},
{ID: "c4", Title: "Р’Р·СЏС‚СЊ РїРµСЂРІСѓСЋ Р·Р°РґР°С‡Сѓ", Stage: "week2", Done: false},
})
st.SeedChecklist("intern-2", []store.ChecklistItem{
{ID: "c1", Title: "РџРѕР·РЅР°РєРѕРјРёС‚СЊСЃСЏ СЃ РєРѕРјР°РЅРґРѕР№", Stage: "week1", Done: false},
})

reg := tools.NewRegistry()
reg.Register(&tools.ChecklistGet{S: st})
reg.Register(&tools.AbsenceCreateRequest{S: st})
reg.Register(&tools.DirectoryFindOwner{})

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

log.Println("listening on", cfg.Addr)
log.Fatal(http.ListenAndServe(cfg.Addr, api.BuildApp(ag, st)))
}