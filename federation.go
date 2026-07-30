package jarvisclaw

import (
	"context"
	"fmt"
	"strconv"
)

// FederationPeer is a remote AIP-compatible platform known to this gateway.
//
// Field names follow the server's camelCase JSON, which differs from the
// snake_case used elsewhere in this SDK.
type FederationPeer struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Status        string `json:"status"` // "online" | "offline"
	LastSeen      string `json:"lastSeen"`
	ResourceCount int    `json:"resourceCount"`
	Capabilities  string `json:"capabilities,omitempty"`
	AIPVersion    string `json:"aipVersion,omitempty"`
	DiscoverURL   string `json:"discoverUrl,omitempty"`
	LatencyMs     int    `json:"latencyMs"`
}

// Healthy reports whether the peer answered its last health check.
func (p FederationPeer) Healthy() bool { return p.Status == "online" }

// CrawlResult reports the outcome of a manual federation crawl.
type CrawlResult struct {
	Message      string           `json:"message"`
	PeersCrawled int              `json:"peers_crawled"`
	Healthy      int              `json:"healthy"`
	Results      []FederationPeer `json:"results"`
}

// FederationPeers returns all registered federation peers.
//
// GET /v1/aip/federation/peers
//
// Admin-only: this route is behind AdminAuth, which requires a dashboard session
// or an access token plus a New-Api-User header. An API key or x402 wallet will
// get 401 here. Use Discover for the caller-accessible view of the federation.
func (c *Client) FederationPeers(ctx context.Context) ([]FederationPeer, error) {
	var resp struct {
		Success bool             `json:"success"`
		Error   string           `json:"error"`
		Data    []FederationPeer `json:"data"`
	}
	if err := c.doGetInto(ctx, "/v1/aip/federation/peers", nil, &resp); err != nil {
		return nil, fmt.Errorf("federation peers: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("federation peers: %s", resp.Error)
	}
	return resp.Data, nil
}

// AddFederationPeer registers a peer domain, to be crawled on the next cycle.
//
// POST /v1/aip/federation/peers — admin-only (see FederationPeers).
// domain is a bare host or base URL, not a peer id.
func (c *Client) AddFederationPeer(ctx context.Context, domain string) error {
	if domain == "" {
		return fmt.Errorf("add federation peer: domain is required")
	}
	body := map[string]string{"domain": domain}
	if err := c.doJSON(ctx, "POST", "/v1/aip/federation/peers", body, nil); err != nil {
		return fmt.Errorf("add federation peer: %w", err)
	}
	return nil
}

// RemoveFederationPeer deregisters a peer.
//
// DELETE /v1/aip/federation/peers — admin-only (see FederationPeers).
//
// The peer is identified by domain in the request body, not by id in the path.
func (c *Client) RemoveFederationPeer(ctx context.Context, domain string) error {
	if domain == "" {
		return fmt.Errorf("remove federation peer: domain is required")
	}
	body := map[string]string{"domain": domain}
	if err := c.doJSON(ctx, "DELETE", "/v1/aip/federation/peers", body, nil); err != nil {
		return fmt.Errorf("remove federation peer: %w", err)
	}
	return nil
}

// FederationCrawl triggers an immediate crawl of every known peer and returns
// the refreshed peer states.
//
// POST /v1/aip/federation/crawl — admin-only (see FederationPeers).
//
// The crawl covers all registered peers; it takes no per-request seed or depth.
// Register targets with AddFederationPeer first.
func (c *Client) FederationCrawl(ctx context.Context) (*CrawlResult, error) {
	var resp CrawlResult
	if err := c.doJSON(ctx, "POST", "/v1/aip/federation/crawl", struct{}{}, &resp); err != nil {
		return nil, fmt.Errorf("federation crawl: %w", err)
	}
	return &resp, nil
}

// ── Federation registry (no admin rights required) ───────────────────────────

// FederationResource is one callable resource published by a federation peer.
//
// SellPrice is what this gateway charges; PriceInput/PriceOutput are the peer's
// published per-unit rates. The peer's own cost to us is never exposed.
type FederationResource struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Method      string  `json:"method"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Tags        string  `json:"tags"`
	PriceInput  float64 `json:"price_input"`
	PriceOutput float64 `json:"price_output"`
	SellPrice   float64 `json:"sell_price"`
	PriceUnit   string  `json:"price_unit"`
	Currency    string  `json:"currency"`
	Network     string  `json:"network"`
	Popular     bool    `json:"popular"`
	CallCount   int64   `json:"call_count"`
	ServerName  string  `json:"server_name"`
	UpdatedAt   int64   `json:"updated_at"`
}

// FederationServer is a peer as published by the public registry.
type FederationServer struct {
	ServerUUID    string `json:"server_uuid"`
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	Description   string `json:"description"`
	Network       string `json:"network"`
	Verified      bool   `json:"verified"`
	ResourceCount int    `json:"resource_count"`
	Healthy       bool   `json:"healthy"`
	LastCheckedAt int64  `json:"last_checked_at"`
	Capabilities  string `json:"capabilities"`
	AIPVersion    string `json:"aip_version"`
	DiscoverURL   string `json:"discover_url"`
	LatencyMs     int    `json:"latency_ms"`
}

// FederationSearchParams filters a federation resource search.
type FederationSearchParams struct {
	Query    string // free-text match
	Category string // exact category match
	Limit    int    // default 20 server-side
}

// SearchFederation searches callable resources across every known peer.
//
// GET /v1/federation/search — public, no auth required.
func (c *Client) SearchFederation(ctx context.Context, params FederationSearchParams) ([]FederationResource, error) {
	q := map[string]string{}
	if params.Query != "" {
		q["q"] = params.Query
	}
	if params.Category != "" {
		q["category"] = params.Category
	}
	if params.Limit > 0 {
		q["limit"] = strconv.Itoa(params.Limit)
	}

	var resp struct {
		Success bool                 `json:"success"`
		Message string               `json:"message"`
		Data    []FederationResource `json:"data"`
		Count   int                  `json:"count"`
	}
	if err := c.doGetInto(ctx, "/v1/federation/search", q, &resp); err != nil {
		return nil, fmt.Errorf("federation search: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("federation search: %s", resp.Message)
	}
	return resp.Data, nil
}

// ListFederationServers lists the peers in the public federation registry,
// paginated. Page is 1-based; pageSize is clamped to 1..100 server-side.
//
// GET /v1/federation/servers — public, no auth required. Unlike FederationPeers
// this needs no admin rights.
func (c *Client) ListFederationServers(ctx context.Context, page, pageSize int) ([]FederationServer, int, error) {
	q := map[string]string{}
	if page > 0 {
		q["page"] = strconv.Itoa(page)
	}
	if pageSize > 0 {
		q["page_size"] = strconv.Itoa(pageSize)
	}

	var resp struct {
		Success bool               `json:"success"`
		Message string             `json:"message"`
		Data    []FederationServer `json:"data"`
		Total   int                `json:"total"`
	}
	if err := c.doGetInto(ctx, "/v1/federation/servers", q, &resp); err != nil {
		return nil, 0, fmt.Errorf("list federation servers: %w", err)
	}
	if !resp.Success {
		return nil, 0, fmt.Errorf("list federation servers: %s", resp.Message)
	}
	return resp.Data, resp.Total, nil
}

// ListFederationResources lists resources published across the federation,
// paginated. Page is 1-based; pageSize is clamped to 1..100 server-side.
//
// GET /v1/federation/resources — public, no auth required.
func (c *Client) ListFederationResources(ctx context.Context, page, pageSize int) ([]FederationResource, int, error) {
	q := map[string]string{}
	if page > 0 {
		q["page"] = strconv.Itoa(page)
	}
	if pageSize > 0 {
		q["page_size"] = strconv.Itoa(pageSize)
	}

	var resp struct {
		Success bool                 `json:"success"`
		Message string               `json:"message"`
		Data    []FederationResource `json:"data"`
		Total   int                  `json:"total"`
	}
	if err := c.doGetInto(ctx, "/v1/federation/resources", q, &resp); err != nil {
		return nil, 0, fmt.Errorf("list federation resources: %w", err)
	}
	if !resp.Success {
		return nil, 0, fmt.Errorf("list federation resources: %s", resp.Message)
	}
	return resp.Data, resp.Total, nil
}

// FederationExecute invokes a federated resource through this gateway, which
// settles payment with the peer on the caller's behalf.
//
// POST /v1/federation/execute — requires an API key or x402 payment.
func (c *Client) FederationExecute(ctx context.Context, req map[string]any) (map[string]any, error) {
	var resp map[string]any
	if err := c.doJSON(ctx, "POST", "/v1/federation/execute", req, &resp); err != nil {
		return nil, fmt.Errorf("federation execute: %w", err)
	}
	return resp, nil
}
