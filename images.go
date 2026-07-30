package jarvisclaw

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ImageClient provides image generation capabilities with smart routing.
type ImageClient struct{ *Client }

// NewImageClient creates a new ImageClient with the given options.
func NewImageClient(opts ...Option) (*ImageClient, error) {
	c, err := NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &ImageClient{c}, nil
}

// ImageOption configures an image generation call.
type ImageOption func(*imageOpts)

type imageOpts struct {
	Model       string
	Size        string
	N           int
	Wait        *bool         // nil means "wait" (default true)
	PollTimeout time.Duration // 0 means the 5 minute default
}

// WithImageModel sets the model for an image generation call. Defaults to "auto/image".
func WithImageModel(model string) ImageOption {
	return func(o *imageOpts) { o.Model = model }
}

// WithSize sets the image size (e.g., "1024x1024").
func WithSize(size string) ImageOption {
	return func(o *imageOpts) { o.Size = size }
}

// WithN sets the number of images to generate.
func WithN(n int) ImageOption {
	return func(o *imageOpts) { o.N = n }
}

// WithImageWait controls whether Generate blocks until the image is ready.
// Defaults to true. Slow models answer with an async job rather than an image.
func WithImageWait(wait bool) ImageOption {
	return func(o *imageOpts) { o.Wait = &wait }
}

// WithImagePollTimeout caps how long Generate waits for an async job.
// Defaults to 5 minutes; a zero or negative value restores that default.
func WithImagePollTimeout(d time.Duration) ImageOption {
	return func(o *imageOpts) { o.PollTimeout = d }
}

// Generate generates an image using smart routing based on prompt analysis.
// Model defaults to "auto/image" if not specified via WithImageModel.
//
// Fast models answer inline. Slower ones answer with an async job
// ({id, status, poll_url}); by default Generate polls that to completion. Use
// WithImageWait(false) to get the job back immediately and poll with Status.
func (ic *ImageClient) Generate(ctx context.Context, prompt string, opts ...ImageOption) (*ImageResponse, error) {
	o := &imageOpts{Model: "auto/image", N: 1}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"model":  o.Model,
		"prompt": prompt,
		"n":      o.N,
	}
	if o.Size != "" {
		payload["size"] = o.Size
	}

	raw, err := ic.doPostCtx(ctx, "/v1/images/generations", payload)
	if err != nil {
		return nil, err
	}

	img := imageResponseFromRaw(raw)

	// Inline result — done.
	if img.URL != "" || img.B64JSON != "" {
		return img, nil
	}
	// Async job. Without an id there is nothing to poll, so the response really
	// is unusable.
	if img.ID == "" {
		return nil, fmt.Errorf("unexpected image response format")
	}
	blocking := true
	if o.Wait != nil {
		blocking = *o.Wait
	}
	if !blocking {
		return img, nil
	}

	timeout := o.PollTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return ic.wait(pollCtx, img.ID)
}

// Status checks an async image job by id, without blocking.
//
// GET /v1/images/generations/:id — no auth required; the id is an unguessable
// UUID and generation was already paid for.
func (ic *ImageClient) Status(ctx context.Context, jobID string) (*ImageResponse, error) {
	raw, err := ic.doGetCtx(ctx, "/v1/images/generations/"+url.PathEscape(jobID), nil)
	if err != nil {
		return nil, err
	}
	img := imageResponseFromRaw(raw)
	if img.ID == "" {
		img.ID = jobID
	}
	return img, nil
}

// wait polls an image job until it completes, fails, or ctx expires.
// On timeout it returns the last known state along with the error, so the caller
// can retry via Status.
func (ic *ImageClient) wait(ctx context.Context, jobID string) (*ImageResponse, error) {
	const pollInterval = 5 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	last := &ImageResponse{ID: jobID, Status: "in_progress"}
	for {
		select {
		case <-ctx.Done():
			return last, fmt.Errorf("image poll timeout for job %s (retry with Status): %w", jobID, ctx.Err())
		case <-ticker.C:
			img, err := ic.Status(ctx, jobID)
			if err != nil {
				if ctx.Err() != nil {
					return last, fmt.Errorf("image poll timeout for job %s (retry with Status): %w", jobID, ctx.Err())
				}
				return nil, err
			}
			last = img
			if img.URL != "" || img.B64JSON != "" || img.Status == "completed" {
				return img, nil
			}
			if img.Status == "failed" {
				return img, fmt.Errorf("image generation failed for job %s", jobID)
			}
		}
	}
}

// imageResponseFromRaw parses both the inline shape
// ({"data":[{"url":...}]}) and the async job shape ({"id","status","poll_url"}).
func imageResponseFromRaw(raw map[string]any) *ImageResponse {
	img := &ImageResponse{Raw: raw}
	if v, ok := raw["id"].(string); ok {
		img.ID = v
	}
	if v, ok := raw["status"].(string); ok {
		img.Status = v
	}
	if data, ok := raw["data"].([]any); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]any); ok {
			if v, ok := item["url"].(string); ok {
				img.URL = v
			}
			if v, ok := item["b64_json"].(string); ok {
				img.B64JSON = v
			}
			if v, ok := item["revised_prompt"].(string); ok {
				img.RevisedPrompt = v
			}
		}
	}
	// Some completed job responses put the url at the top level instead.
	if img.URL == "" {
		if v, ok := raw["url"].(string); ok {
			img.URL = v
		}
	}
	return img
}

// ── Convenience methods on base Client (delegate to ImageClient) ─────────────

// ImageGenerate generates an image using the given model and prompt.
func (c *Client) ImageGenerate(ctx context.Context, model, prompt string) (*ImageResponse, error) {
	ic := &ImageClient{c}
	return ic.Generate(ctx, prompt, WithImageModel(model))
}

// ImageStatus checks an async image generation job by id.
func (c *Client) ImageStatus(ctx context.Context, jobID string) (*ImageResponse, error) {
	ic := &ImageClient{c}
	return ic.Status(ctx, jobID)
}
