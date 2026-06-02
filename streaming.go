package jarvisclaw

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// parseSSEStream reads an SSE stream from r and sends text delta chunks to ch.
// Respects context cancellation to avoid goroutine leaks.
// Closes ch and r when done.
func parseSSEStream(ctx context.Context, r io.ReadCloser, ch chan<- string) {
	defer close(ch)
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// Check context cancellation before processing
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return
		}
		chunk, err := extractDeltaContent(data)
		if err != nil || chunk == "" {
			continue
		}
		// Send with context cancellation support
		select {
		case ch <- chunk:
		case <-ctx.Done():
			return
		}
	}
}

// extractDeltaContent parses a single SSE data JSON line and returns the
// content delta from choices[0].delta.content.
func extractDeltaContent(data string) (string, error) {
	var obj struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return "", err
	}
	if len(obj.Choices) == 0 {
		return "", nil
	}
	return obj.Choices[0].Delta.Content, nil
}
