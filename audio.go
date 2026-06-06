package jarvisclaw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// AudioClient provides audio generation and transcription capabilities.
type AudioClient struct{ *Client }

// NewAudioClient creates a new AudioClient with the given options.
func NewAudioClient(opts ...Option) (*AudioClient, error) {
	c, err := NewClient(opts...)
	return &AudioClient{c}, err
}

// AudioOption configures an audio call.
type AudioOption func(*audioOpts)

type audioOpts struct {
	Model        string
	Voice        string
	Instrumental bool
}

// WithAudioModel sets the model for an audio call.
func WithAudioModel(model string) AudioOption {
	return func(o *audioOpts) { o.Model = model }
}

// WithVoice sets the voice for speech synthesis.
func WithVoice(voice string) AudioOption {
	return func(o *audioOpts) { o.Voice = voice }
}

// WithInstrumental sets whether to generate instrumental-only music.
func WithInstrumental(v bool) AudioOption {
	return func(o *audioOpts) { o.Instrumental = v }
}

// Music generates music from a text prompt.
// Model defaults to "auto/music" if not specified via WithAudioModel.
func (ac *AudioClient) Music(ctx context.Context, prompt string, opts ...AudioOption) (*AudioResponse, error) {
	o := &audioOpts{Model: "auto/music"}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"model":  o.Model,
		"prompt": prompt,
	}
	if o.Instrumental {
		payload["instrumental"] = true
	}

	raw, err := ac.doPost("/v1/audio/music", payload)
	if err != nil {
		return nil, err
	}

	return audioResponseFromRaw(raw)
}

// Speech generates speech audio from text and returns an AudioResponse.
// Model defaults to "auto/tts" if not specified via WithAudioModel.
func (ac *AudioClient) Speech(ctx context.Context, text string, opts ...AudioOption) (*AudioResponse, error) {
	o := &audioOpts{Model: "auto/tts", Voice: "alloy"}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"model": o.Model,
		"input": text,
		"voice": o.Voice,
	}

	resp, err := ac.doRequestRaw("POST", "/v1/audio/speech", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &AudioResponse{Data: data, ContentType: resp.Header.Get("Content-Type")}, nil
}

// Transcribe transcribes audio data and returns the transcript text.
func (ac *AudioClient) Transcribe(ctx context.Context, audioData io.Reader, opts ...AudioOption) (string, error) {
	o := &audioOpts{Model: "whisper-1"}
	for _, opt := range opts {
		opt(o)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("model", o.Model); err != nil {
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

	bodyBytes := buf.Bytes()
	u := ac.buildURL("/v1/audio/transcriptions", nil)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	ac.applyAuth(req)

	resp, err := ac.executeRaw(req, bodyBytes)
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

// ── Convenience methods on base Client (delegate to AudioClient) ─────────────

// AudioSpeech generates speech audio from text and returns the raw audio bytes.
func (c *Client) AudioSpeech(ctx context.Context, model, text, voice string) ([]byte, error) {
	ac := &AudioClient{c}
	resp, err := ac.Speech(ctx, text, WithAudioModel(model), WithVoice(voice))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// AudioTranscribe transcribes audio data using the given model and returns the transcript text.
func (c *Client) AudioTranscribe(ctx context.Context, audioData io.Reader, model string) (string, error) {
	ac := &AudioClient{c}
	return ac.Transcribe(ctx, audioData, WithAudioModel(model))
}

// ── Internal helpers ─────────────────────────────────────────────────────────

func audioResponseFromRaw(raw map[string]any) (*AudioResponse, error) {
	resp := &AudioResponse{}
	if v, ok := raw["url"].(string); ok {
		resp.URL = v
	}
	if v, ok := raw["id"].(string); ok {
		resp.ID = v
	}
	if v, ok := raw["status"].(string); ok {
		resp.Status = v
	}
	return resp, nil
}
