package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OpenAIClient struct {
	BaseURL, APIKey, Model string
	HTTP                   *http.Client
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

func (c *OpenAIClient) Complete(ctx context.Context, msgs []Message) (string, error) {
	body, _ := json.Marshal(req{
		Model: c.Model, Messages: msgs, Temperature: 0.2,
		ResponseFormat: respFormat{Type: "json_object"},
	})
	r, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.HTTP.Do(r)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("llm status %d", res.StatusCode)
	}
	var out resp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	return out.Choices[0].Message.Content, nil
}
