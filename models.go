package jarvisclaw

import (
	"context"
	"fmt"
)

// ListModels returns the list of models available on the API.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	raw, err := c.doGet("/v1/models", nil)
	if err != nil {
		return nil, err
	}

	// Expected: { "data": [ { "id": "...", "object": "model", "owned_by": "..." } ] }
	data, ok := raw["data"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected models response format")
	}

	models := make([]Model, 0, len(data))
	for _, item := range data {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		model := Model{}
		if v, ok := m["id"].(string); ok {
			model.ID = v
		}
		if v, ok := m["object"].(string); ok {
			model.Object = v
		}
		if v, ok := m["owned_by"].(string); ok {
			model.OwnedBy = v
		}
		models = append(models, model)
	}
	return models, nil
}
