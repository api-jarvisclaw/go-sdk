package jarvisclaw

import (
	"context"
	"fmt"
)

// FederationPeer represents a remote AIP-compatible platform.
type FederationPeer struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Protocol  string `json:"protocol,omitempty"`  // "aip", "a2a", etc.
	Status    string `json:"status,omitempty"`    // "active", "unreachable"
	AddedAt   string `json:"added_at,omitempty"`
	LastCrawl string `json:"last_crawl,omitempty"`
}

// FederationPeersResponse is the response from GET /v1/aip/federation/peers.
type FederationPeersResponse struct {
	Peers []FederationPeer `json:"peers"`
	Total int              `json:"total"`
}

// CrawlRequest is the request body for POST /v1/aip/federation/crawl.
type CrawlRequest struct {
	URL   string `json:"url,omitempty"`   // Specific URL to crawl; empty = crawl all peers
	Depth int    `json:"depth,omitempty"` // Crawl depth (default: 1)
}

// CrawlResponse is the response from POST /v1/aip/federation/crawl.
type CrawlResponse struct {
	Discovered int              `json:"discovered"`
	Peers      []FederationPeer `json:"peers,omitempty"`
	Errors     []string         `json:"errors,omitempty"`
}

// FederationPeers returns all registered federation peers.
// GET /v1/aip/federation/peers — requires auth.
func (c *Client) FederationPeers(ctx context.Context) (*FederationPeersResponse, error) {
	var resp FederationPeersResponse
	if err := c.doJSON(ctx, "GET", "/v1/aip/federation/peers", nil, &resp); err != nil {
		return nil, fmt.Errorf("federation peers: %w", err)
	}
	return &resp, nil
}

// AddFederationPeer registers a new federation peer.
// POST /v1/aip/federation/peers — requires auth.
func (c *Client) AddFederationPeer(ctx context.Context, peer FederationPeer) (*FederationPeer, error) {
	var resp FederationPeer
	if err := c.doJSON(ctx, "POST", "/v1/aip/federation/peers", peer, &resp); err != nil {
		return nil, fmt.Errorf("add federation peer: %w", err)
	}
	return &resp, nil
}

// DeleteFederationPeer removes a peer by ID.
// DELETE /v1/aip/federation/peers/:id — requires auth.
func (c *Client) DeleteFederationPeer(ctx context.Context, peerID string) error {
	path := fmt.Sprintf("/v1/aip/federation/peers/%s", peerID)
	if err := c.doJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete federation peer: %w", err)
	}
	return nil
}

// FederationCrawl triggers a federation crawl to discover new peers.
// POST /v1/aip/federation/crawl — requires auth.
func (c *Client) FederationCrawl(ctx context.Context, req CrawlRequest) (*CrawlResponse, error) {
	var resp CrawlResponse
	if err := c.doJSON(ctx, "POST", "/v1/aip/federation/crawl", req, &resp); err != nil {
		return nil, fmt.Errorf("federation crawl: %w", err)
	}
	return &resp, nil
}
