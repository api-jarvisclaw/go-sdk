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
	if err != nil {
		return nil, err
	}
	return &SearchClient{c}, nil
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

// SearchResponse contains the full search result including summary and sources.
type SearchResponse struct {
	Query       string         `json:"query"`
	Summary     string         `json:"summary"`
	Citations   []SearchResult `json:"citations"`
	SourcesUsed int            `json:"sources_used"`
	Model       string         `json:"model"`
}

// Query performs a web search and returns a SearchResponse with summary and citations.
func (sc *SearchClient) Query(ctx context.Context, query string, opts ...SearchOption) (*SearchResponse, error) {
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

	resp := &SearchResponse{}

	// Primary format: {query, summary, citations, sources_used, model}
	if v, ok := raw["query"].(string); ok {
		resp.Query = v
	}
	if v, ok := raw["summary"].(string); ok {
		resp.Summary = v
	}
	if v, ok := raw["model"].(string); ok {
		resp.Model = v
	}
	if v, ok := raw["sources_used"].(float64); ok {
		resp.SourcesUsed = int(v)
	}

	// Parse citations array
	if citations, ok := raw["citations"].([]any); ok {
		for _, item := range citations {
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
			resp.Citations = append(resp.Citations, sr)
		}
	}

	// Fallback: structured results array
	if resp.Summary == "" {
		if data, ok := raw["results"].([]any); ok {
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
				resp.Citations = append(resp.Citations, sr)
			}
		}
	}

	// Fallback: chat completion format
	if resp.Summary == "" {
		choices, _ := raw["choices"].([]any)
		if len(choices) > 0 {
			choice, _ := choices[0].(map[string]any)
			msg, _ := choice["message"].(map[string]any)
			if content, ok := msg["content"].(string); ok {
				resp.Summary = content
			}
		}
	}

	if resp.Summary == "" && len(resp.Citations) == 0 {
		return nil, fmt.Errorf("unexpected search response format")
	}
	return resp, nil
}

// ── Convenience method on base Client (delegate to SearchClient) ─────────────

// Search performs a web search and returns a SearchResponse.
func (c *Client) Search(ctx context.Context, query string) (*SearchResponse, error) {
	sc := &SearchClient{c}
	return sc.Query(ctx, query)
}

// ─── Exa Marketplace Methods ─────────────────────────────────────────────────

// FindSimilar finds pages semantically similar to the given URL via the Exa service.
func (sc *SearchClient) FindSimilar(ctx context.Context, url string, opts ...SearchOption) ([]SearchResult, error) {
	o := &searchOpts{NumResults: 10}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"url":        url,
		"numResults": o.NumResults,
	}

	raw, err := sc.doPostCtx(ctx, "/v1/marketplace/exa/find-similar", payload)
	if err != nil {
		return nil, err
	}

	return parseSearchResults(raw), nil
}

// Contents retrieves full text content from specific URLs via the Exa service.
func (sc *SearchClient) Contents(ctx context.Context, urls []string) ([]map[string]any, error) {
	payload := map[string]any{
		"ids": urls,
	}

	raw, err := sc.doPostCtx(ctx, "/v1/marketplace/exa/contents", payload)
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	if data, ok := raw["results"].([]any); ok {
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				results = append(results, m)
			}
		}
	} else if data, ok := raw["data"].([]any); ok {
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				results = append(results, m)
			}
		}
	}
	return results, nil
}

// Answer gets an AI-generated answer grounded in live web search results via the Exa service.
func (sc *SearchClient) Answer(ctx context.Context, query string, opts ...SearchOption) (*SearchResponse, error) {
	o := &searchOpts{NumResults: 10}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]any{
		"query":      query,
		"numResults": o.NumResults,
	}

	raw, err := sc.doPostCtx(ctx, "/v1/marketplace/exa/answer", payload)
	if err != nil {
		return nil, err
	}

	resp := &SearchResponse{Query: query}
	if v, ok := raw["answer"].(string); ok {
		resp.Summary = v
	}
	resp.Citations = parseSearchResults(raw)
	return resp, nil
}

// parseSearchResults extracts SearchResult items from a raw response map.
func parseSearchResults(raw map[string]any) []SearchResult {
	var results []SearchResult
	data, ok := raw["results"].([]any)
	if !ok {
		data, _ = raw["data"].([]any)
	}
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
		} else if v, ok := m["text"].(string); ok {
			sr.Snippet = v
		}
		results = append(results, sr)
	}
	return results
}
