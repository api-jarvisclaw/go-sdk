package jarvisclaw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// AudioSpeech generates speech audio from text and returns the raw audio bytes.
func (c *Client) AudioSpeech(ctx context.Context, model, text, voice string) ([]byte, error) {
	payload := map[string]any{
		"model": model,
		"input": text,
		"voice": voice,
	}

	resp, err := c.doRequestRaw("POST", "/v1/audio/speech", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// AudioTranscribe transcribes audio data using the given model and returns the transcript text.
func (c *Client) AudioTranscribe(ctx context.Context, audioData io.Reader, model string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("model", model); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}

	fw, err := w.CreateFormFile("file", "audio.mp3")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(fw, audioData); err != nil {
		return "", fmt.Errorf("copy audio data: %w", err)
	}
	w.Close()

	url := c.buildURL("/v1/audio/transcriptions", nil)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	c.applyAuth(req)

	resp, err := c.executeRaw(req, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Try JSON response first: { "text": "..." }
	var result map[string]any
	if jsonErr := parseJSONBytes(body, &result); jsonErr == nil {
		if text, ok := result["text"].(string); ok {
			return text, nil
		}
	}
	// Fall back to plain text
	return string(body), nil
}
