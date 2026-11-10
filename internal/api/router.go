package api

import "github.com/go-chi/chi/v5"

func NewRouter(h *Handlers) *chi.Mux {
r := chi.NewRouter()
r.Use(DemoUser)
r.Get("/health", h.Health)
r.Post("/api/ask", h.Ask)
return r
}
