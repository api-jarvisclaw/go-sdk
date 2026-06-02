package jarvisclaw

import (
	"context"
	"fmt"
)

// Search performs a web search and returns a list of results.
func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	payload := map[string]any{
		"query": query,
	}

	raw, err := c.doPost("/v1/search", payload)
	if err != nil {
		return nil, err
	}

	// Expected response: { "results": [ { "title": "...", "url": "...", "snippet": "..." } ] }
	data, ok := raw["results"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected search response format")
	}

	results := make([]SearchResult, 0, len(data))
	for _, item := range data {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sr := SearchResult{}
		if v, ok := m["title"].(string); ok {
			sr.Title = v
		}
		if v, ok := m["url"].(string); ok {
			sr.URL = v
		}
		if v, ok := m["snippet"].(string); ok {
			sr.Snippet = v
		}
		results = append(results, sr)
	}
	return results, nil
}
