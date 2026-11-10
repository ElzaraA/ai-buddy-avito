package api

import (
"net/http"

"github.com/ElzaraA/ai-buddy-avito/internal/agent"
"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// BuildApp собирает роутер из зависимостей — удобно для тестов.
func BuildApp(ag *agent.Agent, st *store.Store) http.Handler {
h := &Handlers{Agent: ag, Store: st}
return NewRouter(h)
}