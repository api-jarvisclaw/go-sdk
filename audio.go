package jarvisclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
)

// AudioClient provides speech synthesis and music generation capabilities.
type AudioClient struct{ *Client }

// NewAudioClient creates a new AudioClient with the given options.
func NewAudioClient(opts ...Option) (*AudioClient, error) {
	c, err := NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &AudioClient{c}, nil
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

	raw, err := ac.doPostCtx(ctx, "/v1/audio/generations", payload)
	if err != nil {
		return nil, err
	}

	return audioResponseFromRaw(raw)
}

// Speech generates speech audio from text and returns an AudioResponse.
// Model defaults to "auto/tts" if not specified via WithAudioModel.
func (ac *AudioClient) Speech(ctx context.Context, text string, opts ...AudioOption) (*AudioResponse, error) {
	o := &audioOpts{Model: "auto/tts", Voice: "sarah"}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"model": o.Model,
		"input": text,
		"voice": o.Voice,
	}

	resp, err := ac.doPostRawCtx(ctx, "/v1/audio/speech", payload)
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

// TranscriptionRequest is the input for POST /v1/audio/transcriptions.
type TranscriptionRequest struct {
	// Model is the transcription model. Required.
	Model string
	// Filename is the name sent in the multipart part. The extension matters:
	// upstreams use it to detect the audio format, so ".mp3"/".wav"/etc. should
	// match the actual content.
	Filename string
	// Audio is the audio content.
	Audio io.Reader
	// Language is an optional ISO-639-1 hint (e.g. "en", "zh").
	Language string
	// Prompt biases the transcription toward particular spellings or terms.
	Prompt string
	// ResponseFormat is "json" (default), "text", "srt", "verbose_json" or "vtt".
	// Non-json formats return plain text in TranscriptionResponse.Text.
	ResponseFormat string
	// Temperature is the sampling temperature (0-1).
	Temperature float64
}

// TranscriptionResponse is the result of a transcription request.
type TranscriptionResponse struct {
	Text string `json:"text"`
	// Raw is the undecoded body, which carries segments and timings when
	// ResponseFormat is "verbose_json".
	Raw []byte `json:"-"`
}

// Transcribe converts speech audio to text.
//
// POST /v1/audio/transcriptions (multipart) — requires an API key or x402 payment.
//
// Note: with x402 the request is retried after payment, which means the audio is
// read twice. Pass an io.Seeker (e.g. *os.File or *bytes.Reader) so the retry can
// rewind; a non-seekable stream would upload an empty body on the retry, so it is
// buffered into memory here instead.
func (ac *AudioClient) Transcribe(ctx context.Context, req TranscriptionRequest) (*TranscriptionResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("transcribe: model is required")
	}
	if req.Audio == nil {
		return nil, fmt.Errorf("transcribe: audio is required")
	}
	filename := req.Filename
	if filename == "" {
		filename = "audio.mp3"
	}

	// Buffer the multipart body up front: executeRaw re-sends it on 402 and on
	// retryable 5xx, and a streamed body cannot be replayed.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("transcribe: build form: %w", err)
	}
	if _, err := io.Copy(part, req.Audio); err != nil {
		return nil, fmt.Errorf("transcribe: read audio: %w", err)
	}

	fields := map[string]string{"model": req.Model}
	if req.Language != "" {
		fields["language"] = req.Language
	}
	if req.Prompt != "" {
		fields["prompt"] = req.Prompt
	}
	if req.ResponseFormat != "" {
		fields["response_format"] = req.ResponseFormat
	}
	if req.Temperature != 0 {
		fields["temperature"] = strconv.FormatFloat(req.Temperature, 'f', -1, 64)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("transcribe: write field %s: %w", k, err)
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("transcribe: close form: %w", err)
	}

	bodyBytes := buf.Bytes()
	u := ac.buildURL("/v1/audio/transcriptions", nil)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	ac.applyAuth(httpReq)

	resp, err := ac.executeRaw(httpReq, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("transcribe: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("transcribe: read response: %w", err)
	}

	out := &TranscriptionResponse{Raw: respBytes}
	// Non-json response_format returns bare text, which is not a JSON object.
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err == nil && parsed.Text != "" {
		out.Text = parsed.Text
	} else {
		out.Text = string(respBytes)
	}
	return out, nil
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

// AudioTranscribe converts speech audio to text.
func (c *Client) AudioTranscribe(ctx context.Context, model, filename string, audio io.Reader) (string, error) {
	ac := &AudioClient{c}
	resp, err := ac.Transcribe(ctx, TranscriptionRequest{Model: model, Filename: filename, Audio: audio})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
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
