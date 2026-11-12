package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ElzaraA/ai-buddy-avito/internal/agent"
	"github.com/ElzaraA/ai-buddy-avito/internal/config"
	"github.com/ElzaraA/ai-buddy-avito/internal/llm"
	"github.com/ElzaraA/ai-buddy-avito/internal/metrics"
	"github.com/ElzaraA/ai-buddy-avito/internal/rag"
	"github.com/ElzaraA/ai-buddy-avito/internal/store"
	"github.com/ElzaraA/ai-buddy-avito/internal/tools"
)

// BuildApp собирает роутер из готовых зависимостей (без чата) — для unit-тестов.
func BuildApp(ag *agent.Agent, st *store.Store) http.Handler {
	return NewRouter(&Handlers{Agent: ag, Store: st})
}

// SeedDemo заполняет синтетические чек-листы тестовых стажёров (не затирая загруженное с диска).
func SeedDemo(st *store.Store) {
	seed := func(userID string, items []store.ChecklistItem) {
		if !st.HasChecklist(userID) {
			st.SeedChecklist(userID, items)
		}
	}
	seed("intern-1", []store.ChecklistItem{
		{ID: "c1-intern-1", Title: "Познакомиться с командой", Stage: "week1", Done: true},
		{ID: "c2-intern-1", Title: "Получить доступы (Git, Jira, Grafana)", Stage: "week1", Kind: "access"},
		{ID: "c3-intern-1", Title: "Прочитать Playbook", Stage: "week2"},
		{ID: "c4-intern-1", Title: "Взять первую задачу", Stage: "week2"},
	})
	seed("intern-2", []store.ChecklistItem{
		{ID: "c1-intern-2", Title: "Познакомиться с командой", Stage: "week1"},
		{ID: "c2-intern-2", Title: "Настроить окружение фронтенда", Stage: "week1"},
	})
	seed("intern-3", []store.ChecklistItem{
		{ID: "c1-intern-3", Title: "Познакомиться с командой", Stage: "week1", Done: true},
		{ID: "c2-intern-3", Title: "Получить доступы (Git, Jira, Grafana)", Stage: "week1", Done: true, Kind: "access"},
		{ID: "c3-intern-3", Title: "Прочитать Playbook", Stage: "week2", Done: true},
		{ID: "c4-intern-3", Title: "Взять первую задачу", Stage: "week2"},
	})
}

// NewLLM выбирает mock или OpenAI-совместимый клиент по конфигу.
func NewLLM(cfg config.Config) llm.Client {
	if cfg.LLMMock {
		return llm.MockClient{}
	}
	return &llm.OpenAIClient{
		BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel,
		HTTP: &http.Client{Timeout: 20 * time.Second},
	}
}

// App — собранное приложение.
type App struct {
	Handler   http.Handler
	Persister *store.Persister // nil, если STATE_FILE не задан
	Metrics   *metrics.Registry
	Store     *store.Store
}

// New собирает приложение и возвращает только HTTP-обработчик (удобно для тестов).
func New(cfg config.Config, knowledgeDir string) (http.Handler, error) {
	app, err := Build(cfg, knowledgeDir)
	if err != nil {
		return nil, err
	}
	return app.Handler, nil
}

// Build собирает всё приложение: база знаний, хранилище, инструменты, агент, чат, метрики и роутер.
func Build(cfg config.Config, knowledgeDir string) (*App, error) {
	chunks, err := rag.LoadDir(knowledgeDir)
	if err != nil {
		return nil, err
	}
	st := store.New()
	msgs := store.NewMessageStore()

	var persister *store.Persister
	if cfg.StateFile != "" {
		persister = &store.Persister{Path: cfg.StateFile, Store: st, Msgs: msgs}
		if err := persister.Load(); err != nil {
			// Повреждённый снимок не должен мешать запуску: стартуем с чистого состояния.
			slog.Error("state load failed, starting clean", "path", cfg.StateFile, "err", err)
		}
	}
	SeedDemo(st)

	reg := tools.NewRegistry()
	reg.Register(&tools.ChecklistGet{S: st})
	reg.Register(&tools.AbsenceCreateRequest{S: st})
	reg.Register(&tools.DirectoryFindOwner{})
	reg.Register(&tools.AbsenceGetPolicy{})
	reg.Register(&tools.AccessRequest{})

	m := metrics.New()
	m.Add("buddy_knowledge_chunks", "Фрагментов в базе знаний", float64(len(chunks)))

	ag := &agent.Agent{LLM: NewLLM(cfg), Index: rag.NewIndex(chunks), Tools: reg, Now: time.Now}
	if !cfg.LLMMock && cfg.LLMFallback {
		// Реальная модель недоступна → отвечает детерминированная резервная, ответ помечается degraded.
		ag.Fallback = llm.MockClient{}
		ag.OnFallback = func(err error) {
			m.Inc("buddy_llm_fallback_total", "Переходы на резервную модель из-за сбоя основной")
			slog.Warn("llm failed, using fallback", "err", err)
		}
	}

	chat := &ChatHandlers{Agent: ag, Store: st, Msgs: msgs, Broker: NewBroker(), Metrics: m}
	origins := DefaultOrigins
	if len(cfg.CORSOrigins) > 0 {
		origins = cfg.CORSOrigins
	}
	h := &Handlers{
		Agent: ag, Store: st, Chat: chat, Metrics: m,
		Limiter: NewRateLimiter(cfg.RateLimitPerMin, time.Minute),
	}
	return &App{Handler: NewRouterWithOrigins(h, origins), Persister: persister, Metrics: m, Store: st}, nil
}
