package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]apiError{"error": {Code: code, Message: msg}})
}

// badBody отвечает на ошибку разбора тела: превышение лимита LimitBody — 413, иначе — 400 invalid json.
// Раньше слишком большое тело выдавалось за «invalid json», и клиент не понимал, что сократить.
func badBody(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large (max 64 KiB)")
		return
	}
	writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
}

// notFound и methodNotAllowed — ответы роутера в общем формате ошибок (а не текстом chi «404 page not found»).
func notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}
