package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr            string
	LLMBaseURL      string
	LLMAPIKey       string
	LLMModel        string
	LLMMock         bool
	LLMFallback     bool // при сбое реальной LLM отвечать резервной моделью
	KnowledgeDir    string
	CORSOrigins     []string
	StateFile       string // путь к снимку состояния; пусто — только в памяти
	RateLimitPerMin int    // вопросов в минуту на пользователя; 0 — без ограничения
}

func Load() Config {
	apiKey := os.Getenv("LLM_API_KEY")
	// Без ключа всегда mock: демо не должно падать из-за пустого LLM_API_KEY.
	mock := os.Getenv("LLM_MOCK") == "true" || apiKey == ""
	return Config{
		Addr:            getenv("ADDR", ":8080"),
		LLMBaseURL:      getenv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:       apiKey,
		LLMModel:        getenv("LLM_MODEL", "gpt-4o-mini"),
		LLMMock:         mock,
		LLMFallback:     os.Getenv("LLM_FALLBACK") != "false",
		KnowledgeDir:    getenv("KNOWLEDGE_DIR", "data/knowledge"),
		CORSOrigins:     splitList(os.Getenv("CORS_ORIGINS")),
		StateFile:       os.Getenv("STATE_FILE"),
		RateLimitPerMin: getint("RATE_LIMIT_PER_MIN", 60),
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func getint(k string, d int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n >= 0 {
		return n
	}
	return d
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
