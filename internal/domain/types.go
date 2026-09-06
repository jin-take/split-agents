package domain

import "time"

type Room struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Goal      string    `json:"goal"`
	PaneCount int       `json:"pane_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	At       time.Time `json:"at"`
	Role     string    `json:"role"`
	Content  string    `json:"content"`
	Model    string    `json:"model,omitempty"`
	Input    int       `json:"input_tokens,omitempty"`
	Output   int       `json:"output_tokens,omitempty"`
	Reasoning int      `json:"reasoning_tokens,omitempty"`
}

type Usage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
	CachedTokens    int `json:"cached_tokens"`
}

type Plan struct {
	Title           string   `json:"title,omitempty"`
	TaskSummary     string   `json:"task_summary"`
	Complexity      string   `json:"complexity"`
	Model           string   `json:"model"`
	Reasoning       string   `json:"reasoning"`
	ContextKeywords []string `json:"context_keywords"`
	Subagents       int      `json:"subagents"`
	RecentMessages  int      `json:"recent_messages"`
	PlannerUsed     bool     `json:"planner_used"`
}
