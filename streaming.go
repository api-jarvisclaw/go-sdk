package jarvisclaw

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// parseSSEStream reads an SSE stream from r and sends text delta chunks to ch.
// It closes ch when the stream ends or an error occurs.
func parseSSEStream(r io.Reader, ch chan<- string) {
	defer close(ch)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
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
		ch <- chunk
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
