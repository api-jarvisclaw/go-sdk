package jarvisclaw

import (
	"context"
	"encoding/json"
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
	// ResourceID is the handle every invocation path takes: CallAPI, InvokeAPI
	// and the raw FederationExecute body all key off it.
	//
	// Without it SearchFederation was a dead end: results could be listed and then
	// nothing could be invoked, because execute is keyed by resource id. The gateway's
	// own DTO comment (model.FederationResourcePublic) says the field is published
	// precisely so the SDKs can go from a listing to a call.
	ResourceID  int     `json:"resource_id"`
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

// ── Marketplace catalogue (the customer-facing view of the same capacity) ─────

// CatalogueAPI is one entry in the marketplace API catalogue.
//
// The same underlying resources ListFederationResources returns, priced in
// marketplace terms and with anything unpriced excluded — an unpriced row settles
// zero from the caller while the gateway still pays the upstream, so it is not
// sellable and does not appear here.
//
// ServiceID is a stable "federation/{id}" handle; ResourceID is the same integer
// on its own, which is what CallAPI and InvokeAPI take.
type CatalogueAPI struct {
	ServiceID    string  `json:"service_id"`
	ResourceID   int     `json:"resource_id"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	PriceUnit    string  `json:"price_unit"`
	DisplayPrice float64 `json:"display_price"`
	Method       string  `json:"method"`
	Description  string  `json:"description"`
	Tags         string  `json:"tags"`
	// ServerName is populated only when white-labelling is off.
	ServerName string `json:"server_name"`
	// Source is "federation" for bridged entries.
	Source string `json:"source"`
}

// CatalogueCategory is a category with its sellable-item count, so a client can
// offer a filter without paging the whole catalogue to discover what exists.
type CatalogueCategory struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// CataloguePage is one page of the marketplace API catalogue.
type CataloguePage struct {
	Items      []CatalogueAPI      `json:"items"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	Categories []CatalogueCategory `json:"categories"`
}

// CatalogueParams filters the marketplace API catalogue.
type CatalogueParams struct {
	Page     int    // 1-based; defaults to 1
	PageSize int    // defaults to 50, clamped to 200 server-side
	Category string // exact category match
	Keyword  string // free-text match on name/description/tags/category
}

// ListAPIs lists the marketplace API catalogue, paginated.
//
// GET /api/marketplace/apis — public, no auth required.
//
// This is the counterpart to ListFederationResources for callers who want the
// marketplace's own pricing view. Both hand back a resource id that
// CallAPI and InvokeAPI take.
func (c *Client) ListAPIs(ctx context.Context, params CatalogueParams) (*CataloguePage, error) {
	q := map[string]string{}
	if params.Page > 0 {
		q["page"] = strconv.Itoa(params.Page)
	}
	if params.PageSize > 0 {
		q["page_size"] = strconv.Itoa(params.PageSize)
	}
	if params.Category != "" {
		q["category"] = params.Category
	}
	if params.Keyword != "" {
		q["q"] = params.Keyword
	}

	var resp struct {
		Success bool          `json:"success"`
		Error   string        `json:"error"`
		Data    CataloguePage `json:"data"`
	}
	if err := c.doGetInto(ctx, "/api/marketplace/apis", q, &resp); err != nil {
		return nil, fmt.Errorf("list apis: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("list apis: %s", resp.Error)
	}
	return &resp.Data, nil
}

// ── Invocation ────────────────────────────────────────────────────────────────

// FederationExecute invokes a federated resource through this gateway, which
// settles payment with the peer on the caller's behalf.
//
// POST /v1/federation/execute — requires an API key or x402 payment.
//
// This takes the raw request body, so it can set fields this SDK does not model.
// For the common case use CallAPI, which builds the body, or InvokeAPI,
// which returns the upstream body with no envelope to unwrap.
func (c *Client) FederationExecute(ctx context.Context, req map[string]any) (map[string]any, error) {
	var resp map[string]any
	if err := c.doJSON(ctx, "POST", "/v1/federation/execute", req, &resp); err != nil {
		return nil, fmt.Errorf("federation execute: %w", err)
	}
	return resp, nil
}

// CallAPI invokes a catalogue resource by the id a listing handed back, which is
// the shape most callers want after SearchFederation or ListAPIs.
//
//	hits, _ := c.SearchFederation(ctx, jarvisclaw.FederationSearchParams{Query: "qr code"})
//	out, _ := c.CallAPI(ctx, hits[0].ResourceID, map[string]any{"url": "..."})
//
// FederationExecute takes an untyped map, so the caller had to know the request key
// is "resource_id" and not "id" or "resource". This wraps it, so the search → call
// flow is expressible without reading the gateway's source.
//
// The reply keeps execute's envelope: success, status_code, response_body, tx_hash,
// cost_usd, latency_ms. A non-2xx upstream comes back as success=false with the
// charge already settled — check the field, do not assume a nil error means the
// upstream answered. See InvokeAPI for the unwrapped form.
func (c *Client) CallAPI(ctx context.Context, resourceID int, payload map[string]any) (map[string]any, error) {
	if resourceID <= 0 {
		return nil, fmt.Errorf("call api: resource id must be positive, got %d", resourceID)
	}
	body := map[string]any{"resource_id": resourceID}
	// Omitted rather than sent when nil. It was always included, so a caller with no
	// payload sent "payload": null — and the gateway distinguishes an absent payload
	// from a present one, forwarding a body upstream only when Payload is non-nil,
	// because an empty or null body is a real difference to some upstreams.
	if payload != nil {
		body["payload"] = payload
	}
	return c.FederationExecute(ctx, body)
}

// InvokeAPI calls a catalogue resource under the marketplace's own URL shape and
// returns the upstream response body directly.
//
// POST /v1/marketplace/api/{id} — requires an API key or x402 payment.
//
// The difference from CallAPI is the envelope, not the billing: both adapt onto the
// same execute path, which owns settlement. This one hands back the upstream's own
// body, so a caller needs to know nothing about the federation subsystem; CallAPI
// keeps execute's wrapper, which is where tx_hash and cost_usd live.
//
// The stored resource carries its own HTTP method, so payload is sent as a JSON
// body regardless of whether the upstream endpoint is a GET — the gateway decides
// the verb. Pass nil for an endpoint that takes no input.
func (c *Client) InvokeAPI(ctx context.Context, resourceID int, payload map[string]any) (json.RawMessage, error) {
	if resourceID <= 0 {
		return nil, fmt.Errorf("invoke api: resource id must be positive, got %d", resourceID)
	}
	path := "/v1/marketplace/api/" + strconv.Itoa(resourceID)
	// A nil body is sent as no body at all rather than "{}", for the reason in CallAPI.
	var body any
	if payload != nil {
		body = payload
	}
	raw, err := c.doRawJSON(ctx, "POST", path, body)
	if err != nil {
		return nil, fmt.Errorf("invoke api %d: %w", resourceID, err)
	}
	return raw, nil
}

// FederationHealth reports the federation's health status.
// GET /v1/federation/health — public, no auth required.
func (c *Client) FederationHealth(ctx context.Context) (map[string]any, error) {
	var resp map[string]any
	if err := c.doGetInto(ctx, "/v1/federation/health", nil, &resp); err != nil {
		return nil, fmt.Errorf("federation health: %w", err)
	}
	return resp, nil
}
