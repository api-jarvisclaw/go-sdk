package jarvisclaw

import (
	"context"
	"fmt"
)

// ImageGenerate generates an image using the given model and prompt.
func (c *Client) ImageGenerate(ctx context.Context, model, prompt string) (*ImageResponse, error) {
	payload := map[string]any{
		"model":  model,
		"prompt": prompt,
		"n":      1,
	}

	raw, err := c.doPost("/v1/images/generations", payload)
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
