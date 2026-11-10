package config

import "os"

type Config struct {
Addr       string
LLMBaseURL string
LLMAPIKey  string
LLMModel   string
LLMMock    bool
}

func Load() Config {
apiKey := os.Getenv("LLM_API_KEY")
mock := os.Getenv("LLM_MOCK") == "true" || apiKey == ""
return Config{
Addr:       getenv("ADDR", ":8080"),
LLMBaseURL: getenv("LLM_BASE_URL", "https://api.openai.com/v1"),
LLMAPIKey:  apiKey,
LLMModel:   getenv("LLM_MODEL", "gpt-4o-mini"),
LLMMock:    mock,
}
}

func getenv(k, d string) string {
if v := os.Getenv(k); v != "" {
return v
}
return d
}