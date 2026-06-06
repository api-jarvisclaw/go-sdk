package jarvisclaw

import (
	"context"
	"fmt"
	"time"
)

// VideoClient provides video generation capabilities with smart routing.
type VideoClient struct{ *Client }

// NewVideoClient creates a new VideoClient with the given options.
func NewVideoClient(opts ...Option) (*VideoClient, error) {
	c, err := NewClient(opts...)
	return &VideoClient{c}, err
}

// VideoOption configures a video generation call.
type VideoOption func(*videoOpts)

type videoOpts struct {
	Model string
	Duration int
	Wait     *bool // nil means "wait" (default true)
}

// WithVideoModel sets the model for a video generation call. Defaults to "auto/video".
func WithVideoModel(model string) VideoOption {
	return func(o *videoOpts) { o.Model = model }
}

// WithDuration sets the video duration in seconds.
func WithDuration(d int) VideoOption {
	return func(o *videoOpts) { o.Duration = d }
}

// WithWait controls whether Generate blocks until the video is ready.
// WithWait(true) — block until complete (default).
// WithWait(false) — return immediately after job submission.
func WithWait(wait bool) VideoOption {
	return func(o *videoOpts) { o.Wait = &wait }
}

// Generate submits a video generation job and returns a VideoJob with the initial status.
// Model defaults to "auto/video" if not specified via WithVideoModel.
// By default (or with WithWait(true)), Generate blocks until the video is ready.
// Use WithWait(false) to return immediately after submission.
func (vc *VideoClient) Generate(ctx context.Context, prompt string, opts ...VideoOption) (*VideoJob, error) {
	o := &videoOpts{Model: "auto/video"}
	for _, opt := range opts {
		opt(o)
	}

	// Default: wait = true (blocking)
	blocking := true
	if o.Wait != nil {
		blocking = *o.Wait
	}

	payload := map[string]any{
		"model":  o.Model,
		"prompt": prompt,
	}
	if o.Duration > 0 {
		payload["duration"] = o.Duration
	}

	raw, err := vc.doPostCtx(ctx, "/v1/video/generations", payload)
	if err != nil {
		return nil, err
	}

	job, err := videoJobFromRaw(raw)
	if err != nil {
		return nil, err
	}

	// Non-blocking: return immediately with the submitted job
	if !blocking {
		return job, nil
	}

	// Blocking: poll until complete or context expires
	if job.Status == "completed" || job.URL != "" {
		return job, nil
	}
	if job.ID == "" {
		return job, nil
	}

	return vc.wait(ctx, job.ID)
}

// Status checks the status of a video generation job by job ID.
func (vc *VideoClient) Status(ctx context.Context, jobID string) (*VideoJob, error) {
	raw, err := vc.doGetCtx(ctx, "/v1/video/generations/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	return videoJobFromRaw(raw)
}

// ── Convenience methods on base Client (delegate to VideoClient) ─────────────

// VideoGenerate submits a video generation job and returns a VideoJob with the initial status.
func (c *Client) VideoGenerate(ctx context.Context, model string, req *VideoRequest) (*VideoJob, error) {
	vc := &VideoClient{c}
	var opts []VideoOption
	if model != "" {
		opts = append(opts, WithVideoModel(model))
	}
	if req.Duration > 0 {
		opts = append(opts, WithDuration(req.Duration))
	}
	return vc.Generate(ctx, req.Prompt, opts...)
}

// VideoStatus checks the status of a video generation job by job ID.
func (c *Client) VideoStatus(ctx context.Context, jobID string) (*VideoJob, error) {
	vc := &VideoClient{c}
	return vc.Status(ctx, jobID)
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// wait polls a video job until it is complete or the context is cancelled.
func (vc *VideoClient) wait(ctx context.Context, jobID string) (*VideoJob, error) {
	const pollInterval = 5 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			job, err := vc.Status(ctx, jobID)
			if err != nil {
				return nil, err
			}
			if job.Status == "completed" || job.URL != "" {
				return job, nil
			}
			if job.Status == "failed" {
				return job, fmt.Errorf("video generation failed for job %s", jobID)
			}
		}
	}
}

func videoJobFromRaw(raw map[string]any) (*VideoJob, error) {
	job := &VideoJob{}
	if v, ok := raw["id"].(string); ok {
		job.ID = v
	}
	if v, ok := raw["status"].(string); ok {
		job.Status = v
	}
	if v, ok := raw["url"].(string); ok {
		job.URL = v
	}
	if job.ID == "" {
		return nil, fmt.Errorf("unexpected video job response format")
	}
	return job, nil
}
