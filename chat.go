package jarvisclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ChatClient provides chat completion capabilities with smart routing.
type ChatClient struct{ *Client }

// NewChatClient creates a new ChatClient with the given options.
func NewChatClient(opts ...Option) (*ChatClient, error) {
	c, err := NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &ChatClient{c}, nil
}

// ChatOption configures a Chat call.
type ChatOption func(*chatOpts)

type chatOpts struct {
	Model       string
	System      string
	Temperature float64
}

// WithModel sets the model for a chat call. Defaults to "auto".
func WithChatModel(model string) ChatOption {
	return func(o *chatOpts) { o.Model = model }
}

// WithSystem sets a system prompt for the chat call.
func WithSystem(s string) ChatOption { return func(o *chatOpts) { o.System = s } }

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) ChatOption { return func(o *chatOpts) { o.Temperature = t } }

// Complete sends a single user message and returns the assistant's response text.
// Model defaults to "auto" if not specified via WithChatModel.
func (cc *ChatClient) Complete(ctx context.Context, message string, opts ...ChatOption) (string, error) {
	messages := []Message{{Role: "user", Content: message}}
	resp, err := cc.Completion(ctx, messages, opts...)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// Completion sends a full message array and returns a ChatResponse.
// Model defaults to "auto" if not specified via WithChatModel.
func (cc *ChatClient) Completion(ctx context.Context, messages []Message, opts ...ChatOption) (*ChatResponse, error) {
	o := &chatOpts{Model: "auto"}
	for _, opt := range opts {
		opt(o)
	}

	if o.System != "" {
		messages = append([]Message{{Role: "system", Content: o.System}}, messages...)
	}

	payload := map[string]any{
		"model":    o.Model,
		"messages": messages,
	}
	if o.Temperature != 0 {
		payload["temperature"] = o.Temperature
	}

	raw, err := cc.doPostCtx(ctx, "/v1/chat/completions", payload)
	if err != nil {
		return nil, err
	}

	return chatResponseFromRaw(raw)
}

// StreamReader wraps a channel of text chunks from an SSE stream.
type StreamReader struct {
	ch <-chan string
}

// Read returns the next text chunk from the stream, or empty string and false when done.
func (sr *StreamReader) Read() (string, bool) {
	chunk, ok := <-sr.ch
	return chunk, ok
}

// Channel returns the underlying channel for range-based iteration.
func (sr *StreamReader) Channel() <-chan string {
	return sr.ch
}

// Stream sends a user message and returns a StreamReader for consuming text chunks.
// Model defaults to "auto" if not specified via WithChatModel.
func (cc *ChatClient) Stream(ctx context.Context, message string, opts ...ChatOption) (*StreamReader, error) {
	o := &chatOpts{Model: "auto"}
	for _, opt := range opts {
		opt(o)
	}

	messages := []Message{{Role: "user", Content: message}}
	if o.System != "" {
		messages = append([]Message{{Role: "system", Content: o.System}}, messages...)
	}

	payload := map[string]any{
		"model":    o.Model,
		"messages": messages,
		"stream":   true,
	}
	if o.Temperature != 0 {
		payload["temperature"] = o.Temperature
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	url := cc.buildURL("/v1/chat/completions", nil)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	cc.applyAuth(req)

	resp, err := cc.executeRaw(req, bodyBytes)
	if err != nil {
		return nil, err
	}

	ch := make(chan string, 32)
	go parseSSEStream(ctx, resp.Body, ch)
	return &StreamReader{ch: ch}, nil
}

// ── Convenience methods on base Client (delegate to ChatClient) ──────────────

// Chat sends a single user message and returns the assistant's response text.
func (c *Client) Chat(ctx context.Context, model, message string, opts ...ChatOption) (string, error) {
	cc := &ChatClient{c}
	return cc.Complete(ctx, message, append([]ChatOption{WithChatModel(model)}, opts...)...)
}

// ChatCompletion sends a full message array and returns a ChatResponse.
func (c *Client) ChatCompletion(ctx context.Context, model string, messages []Message, opts ...ChatOption) (*ChatResponse, error) {
	cc := &ChatClient{c}
	return cc.Completion(ctx, messages, append([]ChatOption{WithChatModel(model)}, opts...)...)
}

// ChatStream sends a user message and returns a channel that emits text chunks.
// The channel is closed when the stream ends.
func (c *Client) ChatStream(ctx context.Context, model, message string) (<-chan string, error) {
	cc := &ChatClient{c}
	sr, err := cc.Stream(ctx, message, WithChatModel(model))
	if err != nil {
		return nil, err
	}
	return sr.Channel(), nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

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
