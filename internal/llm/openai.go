package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient — клиент OpenAI-совместимого /chat/completions с повторами при временных сбоях.
type OpenAIClient struct {
	BaseURL, APIKey, Model string
	HTTP                   *http.Client
	MaxRetries             int           // повторы после первой попытки; по умолчанию 2
	Backoff                time.Duration // пауза перед первым повтором, дальше ×3; по умолчанию 300 мс
}

type respFormat struct {
	Type string `json:"type"`
}

type req struct {
	Model          string     `json:"model"`
	Messages       []Message  `json:"messages"`
	Temperature    float64    `json:"temperature"`
	ResponseFormat respFormat `json:"response_format"`
}

type resp struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// retryable — временная ошибка: имеет смысл повторить.
type retryable struct{ err error }

func (r retryable) Error() string { return r.err.Error() }

func (c *OpenAIClient) Complete(ctx context.Context, msgs []Message) (string, error) {
	body, _ := json.Marshal(req{
		Model: c.Model, Messages: msgs, Temperature: 0.2,
		ResponseFormat: respFormat{Type: "json_object"},
	})
	retries := c.MaxRetries
	if retries == 0 {
		retries = 2
	}
	backoff := c.Backoff
	if backoff == 0 {
		backoff = 300 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 3
		}
		out, err := c.once(ctx, body)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if _, ok := err.(retryable); !ok || ctx.Err() != nil {
			break
		}
	}
	return "", lastErr
}

func (c *OpenAIClient) once(ctx context.Context, body []byte) (string, error) {
	r, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.HTTP.Do(r)
	if err != nil {
		return "", retryable{err} // сеть, таймаут
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return "", retryable{fmt.Errorf("llm status %d", res.StatusCode)}
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("llm status %d", res.StatusCode) // 4xx — повтор не поможет
	}
	var out resp
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	return out.Choices[0].Message.Content, nil
}
