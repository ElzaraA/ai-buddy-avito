package config

import "testing"

func TestDefaultsAndOverrides(t *testing.T) {
	for _, k := range []string{"ADDR", "LLM_MOCK", "LLM_API_KEY", "LLM_FALLBACK", "STATE_FILE", "RATE_LIMIT_PER_MIN", "CORS_ORIGINS", "KNOWLEDGE_DIR"} {
		t.Setenv(k, "")
	}
	c := Load()
	// Без ключа — всегда mock: демо не падает из-за пустого LLM_API_KEY.
	if !c.LLMMock || !c.LLMFallback || c.Addr != ":8080" || c.RateLimitPerMin != 60 || c.StateFile != "" ||
		c.KnowledgeDir != "data/knowledge" || len(c.CORSOrigins) != 0 {
		t.Fatalf("defaults: %+v", c)
	}

	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_MOCK", "false")
	t.Setenv("LLM_FALLBACK", "false")
	t.Setenv("STATE_FILE", "/tmp/s.json")
	t.Setenv("RATE_LIMIT_PER_MIN", "0")
	t.Setenv("CORS_ORIGINS", " https://a.example , ,https://b.example")
	c = Load()
	if c.LLMMock || c.LLMFallback || c.StateFile != "/tmp/s.json" || c.RateLimitPerMin != 0 ||
		len(c.CORSOrigins) != 2 || c.CORSOrigins[1] != "https://b.example" {
		t.Fatalf("overrides: %+v", c)
	}
	// Мусор и отрицательные значения — значение по умолчанию.
	for _, bad := range []string{"abc", "-5"} {
		t.Setenv("RATE_LIMIT_PER_MIN", bad)
		if Load().RateLimitPerMin != 60 {
			t.Fatalf("bad limit %q must fall back to default", bad)
		}
	}
}
