package jarvisclaw

import (
	"context"
	"fmt"
)

// ImageClient provides image generation capabilities with smart routing.
type ImageClient struct{ *Client }

// NewImageClient creates a new ImageClient with the given options.
func NewImageClient(opts ...Option) (*ImageClient, error) {
	c, err := NewClient(opts...)
	return &ImageClient{c}, err
}

// ImageOption configures an image generation call.
type ImageOption func(*imageOpts)

type imageOpts struct {
	Model string
	Size  string
	N     int
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

// Generate generates an image using smart routing based on prompt analysis.
// Model defaults to "auto/image" if not specified via WithImageModel.
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

	// Parse response: { "data": [ { "url": "...", "b64_json": "...", "revised_prompt": "..." } ] }
	data, ok := raw["data"].([]any)
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("unexpected image response format")
	}
	item, ok := data[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected image item format")
	}

	img := &ImageResponse{}
	if v, ok := item["url"].(string); ok {
		img.URL = v
	}
	if v, ok := item["b64_json"].(string); ok {
		img.B64JSON = v
	}
	if v, ok := item["revised_prompt"].(string); ok {
		img.RevisedPrompt = v
	}
	return img, nil
}

// ── Convenience method on base Client (delegate to ImageClient) ──────────────

// ImageGenerate generates an image using the given model and prompt.
func (c *Client) ImageGenerate(ctx context.Context, model, prompt string) (*ImageResponse, error) {
	ic := &ImageClient{c}
	return ic.Generate(ctx, prompt, WithImageModel(model))
}
