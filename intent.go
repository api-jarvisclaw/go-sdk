package jarvisclaw

import (
	"context"
	"fmt"
)

// ResolveRequest is the input for intent resolution.
type ResolveRequest struct {
	Intent      string      `json:"intent"`
	Constraints Constraints `json:"constraints,omitempty"`
	Preferences Preferences `json:"preferences,omitempty"`
}

// Constraints limits the set of providers considered during resolution.
type Constraints struct {
	MaxPriceUSD *float64 `json:"max_price_usd,omitempty"`
	Features    []string `json:"features,omitempty"`
}

// Preferences express soft optimization goals for resolution.
type Preferences struct {
	OptimizeFor string `json:"optimize_for,omitempty"` // "cost", "quality", "latency"
}

// ResolveResponse is the result of an intent resolution request.
type ResolveResponse struct {
	Matches        []Match `json:"matches"`
	IntentType     string  `json:"intent_type"`
	TotalAvailable int     `json:"total_available"`
}

// Match represents a single provider candidate returned by intent resolution.
type Match struct {
	ProviderID        string  `json:"provider_id"`
	Score             float64 `json:"score"`
	EstimatedPriceUSD float64 `json:"estimated_price_usd"`
	Endpoint          string  `json:"endpoint"`
	Model             string  `json:"model"`
	Reason            string  `json:"reason"`
}

// Resolve finds the optimal provider for a given intent.
// POST /v1/intent/resolve — free endpoint, auth optional but accepted.
func (c *Client) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResponse, error) {
	var resp ResolveResponse
	if err := c.doJSON(ctx, "POST", "/v1/intent/resolve", req, &resp); err != nil {
		return nil, fmt.Errorf("resolve intent: %w", err)
	}
	return &resp, nil
}

// ListProviders returns all available providers.
// GET /v1/providers
func (c *Client) ListProviders(ctx context.Context) ([]Match, error) {
	var resp struct {
		Providers []Match `json:"providers"`
		Total     int     `json:"total"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/providers", nil, &resp); err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	return resp.Providers, nil
}

// ListIntentTypes returns the supported intent type identifiers.
// GET /v1/intent/types
func (c *Client) ListIntentTypes(ctx context.Context) ([]string, error) {
	var resp struct {
		IntentTypes []string `json:"intent_types"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/intent/types", nil, &resp); err != nil {
		return nil, fmt.Errorf("list intent types: %w", err)
	}
	return resp.IntentTypes, nil
}
