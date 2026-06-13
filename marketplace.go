package jarvisclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// MarketplaceClient provides access to generic marketplace services.
type MarketplaceClient struct{ *Client }

// NewMarketplaceClient creates a new MarketplaceClient with the given options.
func NewMarketplaceClient(opts ...Option) (*MarketplaceClient, error) {
	c, err := NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &MarketplaceClient{c}, nil
}

// MarketplaceOption configures a marketplace call.
type MarketplaceOption func(*marketplaceOpts)

type marketplaceOpts struct {
	Params map[string]string
}

// WithParams sets query parameters for a marketplace GET call.
func WithParams(params map[string]string) MarketplaceOption {
	return func(o *marketplaceOpts) { o.Params = params }
}

// Call performs a GET request to a marketplace service endpoint.
func (mc *MarketplaceClient) Call(ctx context.Context, service, path string, opts ...MarketplaceOption) (map[string]any, error) {
	o := &marketplaceOpts{}
	for _, opt := range opts {
		opt(o)
	}
	fullPath := "/v1/marketplace/" + service + path
	return mc.doGetCtx(ctx, fullPath, o.Params)
}

// Post performs a POST request to a marketplace service endpoint.
func (mc *MarketplaceClient) Post(ctx context.Context, service, path string, body any) (map[string]any, error) {
	fullPath := "/v1/marketplace/" + service + path
	return mc.doPostCtx(ctx, fullPath, body)
}

// ─── RPC Convenience Methods ─────────────────────────────────────────────────

// RPCRequest represents a single JSON-RPC call in a batch.
type RPCRequest struct {
	Method string
	Params any
}

// RPCCall sends a JSON-RPC 2.0 request to a blockchain network.
// chain is the chain identifier (e.g., "ethereum", "solana", "base").
// method is the JSON-RPC method (e.g., "eth_blockNumber", "getBalance").
// params are the method parameters.
func (mc *MarketplaceClient) RPCCall(ctx context.Context, chain, method string, params any) (map[string]any, error) {
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	return mc.Post(ctx, "rpc", "/"+chain, body)
}

// RPCBatch sends multiple JSON-RPC 2.0 requests in a single batch call.
func (mc *MarketplaceClient) RPCBatch(ctx context.Context, chain string, calls []RPCRequest) ([]map[string]any, error) {
	batch := make([]map[string]any, len(calls))
	for i, c := range calls {
		batch[i] = map[string]any{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"method":  c.Method,
			"params":  c.Params,
		}
	}
	fullPath := "/v1/marketplace/rpc/" + chain
	raw, err := mc.doPostRawBytes(ctx, fullPath, batch)
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("RPCBatch: unmarshal response: %w", err)
	}
	return results, nil
}

// ─── DeFi Convenience Methods ────────────────────────────────────────────────

// DefiProtocols returns TVL data for DeFi protocols.
func (mc *MarketplaceClient) DefiProtocols(ctx context.Context, opts ...MarketplaceOption) (map[string]any, error) {
	return mc.Call(ctx, "defi", "/protocols", opts...)
}

// DefiProtocol returns data for a specific DeFi protocol by slug.
func (mc *MarketplaceClient) DefiProtocol(ctx context.Context, slug string, opts ...MarketplaceOption) (map[string]any, error) {
	return mc.Call(ctx, "defi", "/protocol/"+slug, opts...)
}

// DefiYields returns current yield/APY data across DeFi protocols.
func (mc *MarketplaceClient) DefiYields(ctx context.Context, opts ...MarketplaceOption) (map[string]any, error) {
	return mc.Call(ctx, "defi", "/yields", opts...)
}

// DefiTVL returns TVL data (alias for protocols listing).
func (mc *MarketplaceClient) DefiTVL(ctx context.Context, opts ...MarketplaceOption) (map[string]any, error) {
	return mc.Call(ctx, "defi", "/protocols", opts...)
}

