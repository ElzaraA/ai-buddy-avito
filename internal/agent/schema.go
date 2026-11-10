package agent

type AgentOutput struct {
Type    string            `json:"type"`
Text    string            `json:"text,omitempty"`
Tool    string            `json:"tool,omitempty"`
Args    map[string]string `json:"args,omitempty"`
Sources []Source          `json:"sources,omitempty"`
}

type Source struct {
ID     string `json:"id"`
Source string `json:"source"`
Stage  string `json:"stage"`
}
