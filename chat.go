package jarvisclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func marshalJSON(v any) ([]byte, error)         { return json.Marshal(v) }
func newReader(b []byte) *bytes.Reader           { return bytes.NewReader(b) }
func parseJSONBytes(b []byte, v any) error       { return json.Unmarshal(b, v) }

// ChatOption configures a Chat or ChatCompletion call.
type ChatOption func(*chatOpts)

type chatOpts struct {
	System      string
	Temperature float64
}

// WithSystem sets a system prompt for the chat call.
func WithSystem(s string) ChatOption { return func(o *chatOpts) { o.System = s } }

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) ChatOption { return func(o *chatOpts) { o.Temperature = t } }

// Chat sends a single user message and returns the assistant's response text.
func (c *Client) Chat(ctx context.Context, model, message string, opts ...ChatOption) (string, error) {
	messages := []Message{{Role: "user", Content: message}}
	resp, err := c.ChatCompletion(ctx, model, messages, opts...)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ChatCompletion sends a full message array and returns a ChatResponse.
func (c *Client) ChatCompletion(ctx context.Context, model string, messages []Message, opts ...ChatOption) (*ChatResponse, error) {
	o := &chatOpts{}
	for _, opt := range opts {
		opt(o)
	}

	if o.System != "" {
		messages = append([]Message{{Role: "system", Content: o.System}}, messages...)
	}

	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if o.Temperature != 0 {
		payload["temperature"] = o.Temperature
	}

	raw, err := c.doPost("/v1/chat/completions", payload)
	if err != nil {
		return nil, err
	}

	return chatResponseFromRaw(raw)
}

// ChatStream sends a user message and returns a channel that emits text chunks.
// The channel is closed when the stream ends.
func (c *Client) ChatStream(ctx context.Context, model, message string) (<-chan string, error) {
	payload := map[string]any{
		"model":    model,
		"messages": []Message{{Role: "user", Content: message}},
		"stream":   true,
	}

	bodyBytes, err := marshalJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	url := c.buildURL("/v1/chat/completions", nil)
	req, err := http.NewRequestWithContext(ctx, "POST", url, newReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	resp, err := c.executeRaw(req, bodyBytes)
	if err != nil {
		return nil, err
	}

	ch := make(chan string, 32)
	go parseSSEStream(ctx, resp.Body, ch)
	return ch, nil
}

func chatResponseFromRaw(raw map[string]any) (*ChatResponse, error) {
	cr := &ChatResponse{Raw: raw}

	if v, ok := raw["model"].(string); ok {
		cr.Model = v
	}
	if v, ok := raw["id"].(string); ok {
		cr.ID = v
	}
	if v, ok := raw["usage"].(map[string]any); ok {
		cr.Usage = v
	}

	// Extract content from choices[0].message.content
	choices, ok := raw["choices"].([]any)
	if !ok || len(choices) == 0 {
		return cr, nil
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return cr, nil
	}
	msg, ok := choice["message"].(map[string]any)
	if !ok {
		return cr, nil
	}
	if content, ok := msg["content"].(string); ok {
		cr.Content = content
	}
	return cr, nil
}
