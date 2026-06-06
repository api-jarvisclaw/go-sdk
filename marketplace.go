package jarvisclaw

import "context"

// MarketplaceClient provides access to generic marketplace services.
type MarketplaceClient struct{ *Client }

// NewMarketplaceClient creates a new MarketplaceClient with the given options.
func NewMarketplaceClient(opts ...Option) (*MarketplaceClient, error) {
	c, err := NewClient(opts...)
	return &MarketplaceClient{c}, err
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
