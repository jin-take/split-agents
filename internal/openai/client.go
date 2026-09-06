package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jin-take/SplitAgents/internal/domain"
)

type Client struct {
	APIKey string
	HTTP   *http.Client
}

func New() (*Client, error) {
	envFile := strings.TrimSpace(os.Getenv("SPLITAGENTS_ENV_FILE"))
	if envFile == "" {
		if cwd, err := os.Getwd(); err == nil {
			candidate := filepath.Join(cwd, ".env")
			if _, err := os.Stat(candidate); err == nil {
				envFile = candidate
				_ = os.Setenv("SPLITAGENTS_ENV_FILE", candidate)
			}
		}
	}

	if envFile != "" {
		if err := loadDotEnv(envFile); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}

	k := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if k == "" {
		return nil, errors.New("OPENAI_API_KEY is not set; run chatgpt from the split-agents directory containing .env")
	}
	return &Client{APIKey: k, HTTP: &http.Client{}}, nil
}

// loadDotEnv loads KEY=VALUE pairs from path without adding an external
// dependency. Existing process environment variables always take precedence.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}

		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (c *Client) request(ctx context.Context, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func (c *Client) Complete(ctx context.Context, model, reasoning, instructions, input string, maxOut int) (string, domain.Usage, error) {
	body := map[string]any{"model": model, "instructions": instructions, "input": input, "max_output_tokens": maxOut, "reasoning": map[string]any{"effort": reasoning}}
	resp, err := c.request(ctx, body)
	if err != nil {
		return "", domain.Usage{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", domain.Usage{}, fmt.Errorf("openai: %s", strings.TrimSpace(string(raw)))
	}
	var r map[string]any
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", domain.Usage{}, err
	}
	text := extractText(r)
	return text, extractUsage(r), nil
}

func (c *Client) Stream(ctx context.Context, model, reasoning, instructions, input string, maxOut int, onDelta func(string)) (string, domain.Usage, error) {
	body := map[string]any{"model": model, "instructions": instructions, "input": input, "max_output_tokens": maxOut, "reasoning": map[string]any{"effort": reasoning}, "stream": true}
	resp, err := c.request(ctx, body)
	if err != nil {
		return "", domain.Usage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return "", domain.Usage{}, fmt.Errorf("openai: %s", strings.TrimSpace(string(raw)))
	}
	var out strings.Builder
	var usage domain.Usage
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var ev map[string]any
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		typ, _ := ev["type"].(string)
		if typ == "response.output_text.delta" {
			if d, ok := ev["delta"].(string); ok {
				out.WriteString(d)
				onDelta(d)
			}
		}
		if typ == "response.completed" {
			if r, ok := ev["response"].(map[string]any); ok {
				usage = extractUsage(r)
			}
		}
	}
	return out.String(), usage, sc.Err()
}

func extractText(r map[string]any) string {
	if s, ok := r["output_text"].(string); ok {
		return s
	}
	var b strings.Builder
	outs, _ := r["output"].([]any)
	for _, o := range outs {
		om, _ := o.(map[string]any)
		cs, _ := om["content"].([]any)
		for _, c := range cs {
			cm, _ := c.(map[string]any)
			if t, ok := cm["text"].(string); ok {
				b.WriteString(t)
			}
		}
	}
	return b.String()
}

func extractUsage(r map[string]any) domain.Usage {
	var u domain.Usage
	m, _ := r["usage"].(map[string]any)
	u.InputTokens = num(m["input_tokens"])
	u.OutputTokens = num(m["output_tokens"])
	if d, ok := m["input_tokens_details"].(map[string]any); ok {
		u.CachedTokens = num(d["cached_tokens"])
	}
	if d, ok := m["output_tokens_details"].(map[string]any); ok {
		u.ReasoningTokens = num(d["reasoning_tokens"])
	}
	return u
}

func num(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}
