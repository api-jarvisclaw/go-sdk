package jarvisclaw

import "context"

// Prediction calls a prediction market API endpoint and returns the raw JSON response.
// method is the HTTP method ("GET" or "POST"), path is the API path (e.g., "/v1/predictions/...").
func (c *Client) Prediction(ctx context.Context, method, path string) (map[string]any, error) {
	return c.doRequest(method, path, nil)
}
