package jarvisclaw

import (
	"context"
	"fmt"
)

// SearchClient provides web search capabilities.
type SearchClient struct{ *Client }

// NewSearchClient creates a new SearchClient with the given options.
func NewSearchClient(opts ...Option) (*SearchClient, error) {
	c, err := NewClient(opts...)
	return &SearchClient{c}, err
}

// SearchOption configures a search call.
type SearchOption func(*searchOpts)

type searchOpts struct {
	NumResults int
}

// WithNumResults sets the number of search results to return.
func WithNumResults(n int) SearchOption {
	return func(o *searchOpts) { o.NumResults = n }
}

// Query performs a web search using "auto/search" and returns a list of results.
// The backend treats /v1/search as a chat completions request, so we send the
// query as a chat message.
func (sc *SearchClient) Query(ctx context.Context, query string, opts ...SearchOption) ([]SearchResult, error) {
	o := &searchOpts{NumResults: 10}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"model": "auto/search",
		"messages": []map[string]string{
			{"role": "user", "content": query},
		},
		"max_results": o.NumResults,
	}

	raw, err := sc.doPostCtx(ctx, "/v1/search", payload)
	if err != nil {
		return nil, err
	}

	// Try structured results first
	data, ok := raw["results"].([]any)
	if !ok {
		// Fall back to chat completion format — extract content
		choices, _ := raw["choices"].([]any)
		if len(choices) > 0 {
			choice, _ := choices[0].(map[string]any)
			msg, _ := choice["message"].(map[string]any)
			content, _ := msg["content"].(string)
			if content != "" {
				return []SearchResult{{Title: "Search Result", Snippet: content}}, nil
			}
		}
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

// ── Convenience method on base Client (delegate to SearchClient) ─────────────

// Search performs a web search and returns a list of results.
func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	sc := &SearchClient{c}
	return sc.Query(ctx, query)
}
