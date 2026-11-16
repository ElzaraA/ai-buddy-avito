package api

import (
	"net/http"

	"github.com/ElzaraA/ai-buddy-avito/internal/auth"
)

// Демо-режим: показ жюри начинается с чистого состояния одной кнопкой.

type demoInfo struct {
	DemoMode bool   `json:"demo_mode"`
	LLM      string `json:"llm"` // mock | real
}

// DemoInfo — GET /api/v1/demo: включён ли демо-режим и какая модель отвечает.
func (h *Handlers) DemoInfo(w http.ResponseWriter, _ *http.Request) {
	llm := "real"
	if h.LLMMock {
		llm = "mock"
	}
	writeJSON(w, http.StatusOK, demoInfo{DemoMode: h.DemoMode, LLM: llm})
}

// DemoReset — POST /api/v1/demo/reset: сбрасывает всё состояние в памяти (заявки, чек-листы, сигналы,
// «Пульс», пробелы базы, «Сэкономлено наставнику», счётчики достижений, диалоги чата) к исходному сиду,
// как после свежего старта. Только при DEMO_MODE=true (иначе 404), доступно любому демо-пользователю.
func (h *Handlers) DemoReset(w http.ResponseWriter, r *http.Request) {
	if !h.DemoMode {
		notFound(w, r)
		return
	}
	if _, ok := auth.FromCtx(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "no user")
		return
	}
	h.Store.Reset()
	if h.Chat != nil && h.Chat.Msgs != nil {
		h.Chat.Msgs.Reset()
	}
	SeedDemo(h.Store)
	h.Store.RequestSave()
	h.Metrics.Inc("buddy_demo_resets_total", "Сбросы демо-состояния")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
