package jarvisclaw

import (
	"context"
	"fmt"
	"strings"
)

// Prediction calls a prediction-market endpoint and returns the raw JSON response.
//
// GET|POST /v1/prediction/<path> — requires an API key or x402 payment.
//
// path is the service-relative sub-path, with or without a leading slash
// (e.g. "markets" or "/markets"); the /v1/prediction prefix is added for you.
// Passing a full "/v1/prediction/..." path also works and is not double-prefixed.
//
// method must be "GET" or "POST"; body is sent only for POST and may be nil.
func (c *Client) Prediction(ctx context.Context, method, path string, body any) (map[string]any, error) {
	const prefix = "/v1/prediction"

	p := strings.TrimSpace(path)
	switch {
	case strings.HasPrefix(p, prefix):
		// already fully qualified
	case p == "" || p == "/":
		p = prefix + "/"
	case strings.HasPrefix(p, "/"):
		p = prefix + p
	default:
		p = prefix + "/" + p
	}

	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "":
		return c.doGetCtx(ctx, p, nil)
	case "POST":
		return c.doPostCtx(ctx, p, body)
	default:
		return nil, fmt.Errorf("prediction: unsupported method %q (use GET or POST)", method)
	}
}
