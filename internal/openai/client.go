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
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
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
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var ev map[string]any
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}

		typ, _ := ev["type"].(string)
		switch typ {
		case "response.output_text.delta":
			if d, ok := ev["delta"].(string); ok && d != "" {
				out.WriteString(d)
				if onDelta != nil {
					onDelta(d)
				}
			}
		case "response.completed":
			r, _ := ev["response"].(map[string]any)
			usage = extractUsage(r)

			// Some transports or API versions may not expose text deltas in the
			// exact shape above. The completed response still contains the final
			// output, so use it as a safe fallback rather than returning an empty
			// assistant message.
			if out.Len() == 0 {
				finalText := extractText(r)
				if finalText != "" {
					out.WriteString(finalText)
					if onDelta != nil {
						onDelta(finalText)
					}
				}
			}
		case "response.failed", "response.incomplete":
			if r, ok := ev["response"].(map[string]any); ok {
				return out.String(), extractUsage(r), fmt.Errorf("openai stream %s: %s", typ, responseError(r))
			}
			return out.String(), usage, fmt.Errorf("openai stream %s", typ)
		case "error":
			return out.String(), usage, fmt.Errorf("openai stream error: %s", eventError(ev))
		}
	}

	if err := sc.Err(); err != nil {
		return out.String(), usage, err
	}
	if strings.TrimSpace(out.String()) == "" {
		return "", usage, errors.New("openai stream completed without text output")
	}
	return out.String(), usage, nil
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

func responseError(r map[string]any) string {
	if e, ok := r["error"].(map[string]any); ok {
		if msg, ok := e["message"].(string); ok && msg != "" {
			return msg
		}
	}
	if reason, ok := r["incomplete_details"].(map[string]any); ok {
		if v, ok := reason["reason"].(string); ok && v != "" {
			return v
		}
	}
	return "unknown response error"
}

func eventError(ev map[string]any) string {
	if msg, ok := ev["message"].(string); ok && msg != "" {
		return msg
	}
	if e, ok := ev["error"].(map[string]any); ok {
		if msg, ok := e["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return "unknown stream error"
}

func num(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}
