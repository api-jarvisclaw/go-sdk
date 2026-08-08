package jarvisclaw

// Deprecated aliases for the federation→network rename (2026-08).
//
// The gateway renamed its public surface from "federation" to "network": what
// used to be a federation of peers is presented as one AIP network, and peer
// resources are presented as APIs. The SDK follows that naming.
//
// Everything below keeps existing code compiling. These aliases are frozen —
// new fields and methods land on the Network* names only. They will be removed
// in a future major version.
//
// The wire paths are deliberately unchanged: the gateway still serves
// /v1/federation/* in production, and /v1/aip/federation/{peers,crawl} has no
// network-named counterpart at all. Renaming the Go symbols does not renegotiate
// the HTTP contract.

import "context"

// Deprecated: use NetworkPeer.
type FederationPeer = NetworkPeer

// Deprecated: use NetworkAPI.
type FederationResource = NetworkAPI

// Deprecated: use NetworkServer.
type FederationServer = NetworkServer

// Deprecated: use NetworkSearchParams.
type FederationSearchParams = NetworkSearchParams

// Deprecated: use Client.NetworkPeers.
func (c *Client) FederationPeers(ctx context.Context) ([]NetworkPeer, error) {
	return c.NetworkPeers(ctx)
}

// Deprecated: use Client.AddNetworkPeer.
func (c *Client) AddFederationPeer(ctx context.Context, domain string) error {
	return c.AddNetworkPeer(ctx, domain)
}

// Deprecated: use Client.RemoveNetworkPeer.
func (c *Client) RemoveFederationPeer(ctx context.Context, domain string) error {
	return c.RemoveNetworkPeer(ctx, domain)
}

// Deprecated: use Client.NetworkCrawl.
func (c *Client) FederationCrawl(ctx context.Context) (*CrawlResult, error) {
	return c.NetworkCrawl(ctx)
}

// Deprecated: use Client.SearchNetwork.
func (c *Client) SearchFederation(ctx context.Context, params NetworkSearchParams) ([]NetworkAPI, error) {
	return c.SearchNetwork(ctx, params)
}

// Deprecated: use Client.ListNetworkServers.
func (c *Client) ListFederationServers(ctx context.Context, page, pageSize int) ([]NetworkServer, int, error) {
	return c.ListNetworkServers(ctx, page, pageSize)
}

// Deprecated: use Client.ListNetworkAPIs.
func (c *Client) ListFederationResources(ctx context.Context, page, pageSize int) ([]NetworkAPI, int, error) {
	return c.ListNetworkAPIs(ctx, page, pageSize)
}

// Deprecated: use Client.NetworkExecute.
func (c *Client) FederationExecute(ctx context.Context, req map[string]any) (map[string]any, error) {
	return c.NetworkExecute(ctx, req)
}

// Deprecated: use Client.NetworkHealth.
func (c *Client) FederationHealth(ctx context.Context) (map[string]any, error) {
	return c.NetworkHealth(ctx)
}
