package agent

type AgentOutput struct {
	Type    string            `json:"type"` // answer | tool_call | refusal | draft
	Text    string            `json:"text,omitempty"`
	Tool    string            `json:"tool,omitempty"`
	Action  string            `json:"action,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Sources []Source          `json:"sources,omitempty"`
}

type Source struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Stage  string `json:"stage"`
	Title  string `json:"title,omitempty"` // заголовок раздела базы знаний
}
