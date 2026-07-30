package jarvisclaw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// UserAPI is a community-published API listed in the UAPI marketplace.
//
// PricePerCall is the post-markup price a caller actually pays, so it is the
// number to budget against.
type UserAPI struct {
	ID           int     `json:"id"`
	Slug         string  `json:"slug"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	PricePerCall float64 `json:"price_per_call"`
	MaxRPM       int     `json:"max_rpm"`
	LogoURL      string  `json:"logo_url"`
	DocURL       string  `json:"doc_url"`
	Category     string  `json:"category"`
	Status       int     `json:"status"`
	AvgRating    float64 `json:"avg_rating"`
	TotalCalls   int64   `json:"total_calls"`
	VerifiedAt   int64   `json:"verified_at"`
	CreatedAt    int64   `json:"created_at"`
}

// UserAPIListParams filters the UAPI marketplace listing.
type UserAPIListParams struct {
	Page     int    // 1-based; defaults to 1
	PageSize int    // clamped to 1..100 server-side; defaults to 20
	Category string //
	Search   string // free-text match on name/description
	Sort     string // server-defined sort key
}

// ListUserAPIs browses the UAPI marketplace.
//
// GET /api/user-api/list — public, no auth required. Returns the entries and the
// total count.
//
// Note this route family is /api/user-api (browse), while invoking a published
// API goes through /v1/uapi/{slug} (see CallUserAPI).
func (c *Client) ListUserAPIs(ctx context.Context, params UserAPIListParams) ([]UserAPI, int, error) {
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
	if params.Search != "" {
		q["search"] = params.Search
	}
	if params.Sort != "" {
		q["sort"] = params.Sort
	}

	var resp struct {
		Success bool      `json:"success"`
		Message string    `json:"message"`
		Data    []UserAPI `json:"data"`
		Total   int       `json:"total"`
	}
	if err := c.doGetInto(ctx, "/api/user-api/list", q, &resp); err != nil {
		return nil, 0, fmt.Errorf("list user apis: %w", err)
	}
	// This route answers 200 with success:false on failure rather than a 4xx.
	if !resp.Success {
		return nil, 0, fmt.Errorf("list user apis: %s", resp.Message)
	}
	return resp.Data, resp.Total, nil
}

// GetUserAPI returns one published API by slug, with its endpoint list.
//
// GET /api/user-api/detail/:slug — public, no auth required.
//
// The endpoint list schema is provider-defined, so the raw payload is returned
// alongside the typed header fields.
func (c *Client) GetUserAPI(ctx context.Context, slug string) (map[string]any, error) {
	if slug == "" {
		return nil, fmt.Errorf("get user api: slug is required")
	}
	var resp struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	path := "/api/user-api/detail/" + url.PathEscape(slug)
	if err := c.doGetInto(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get user api: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("get user api: %s", resp.Message)
	}
	if resp.Data != nil {
		return resp.Data, nil
	}
	// Some responses put the API fields at the top level rather than under data.
	return map[string]any{}, nil
}

// CallUserAPI invokes a published API through the gateway, which settles payment
// with the provider on the caller's behalf.
//
// {method} /v1/uapi/{slug}/{path} — requires an API key or x402 payment.
//
// path is the API-relative sub-path, with or without a leading slash. body is
// sent as JSON for methods that take one, and may be nil. The upstream response
// is arbitrary, so the raw bytes are returned unparsed — use CallUserAPIRaw if
// you also need headers or a streaming body.
func (c *Client) CallUserAPI(ctx context.Context, method, slug, path string, body any) ([]byte, error) {
	if slug == "" {
		return nil, fmt.Errorf("call user api: slug is required")
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		m = http.MethodGet
	}

	sub := strings.TrimPrefix(strings.TrimSpace(path), "/")
	full := "/v1/uapi/" + url.PathEscape(slug) + "/" + sub

	raw, err := c.doRawJSON(ctx, m, full, body)
	if err != nil {
		return nil, fmt.Errorf("call user api: %w", err)
	}
	return raw, nil
}

// CallUserAPIRaw is CallUserAPI with access to the live response, for streaming
// or non-JSON payloads. The caller must close the returned body.
func (c *Client) CallUserAPIRaw(ctx context.Context, method, slug, path string, body io.Reader, contentType string) (*http.Response, error) {
	if slug == "" {
		return nil, fmt.Errorf("call user api: slug is required")
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		m = http.MethodGet
	}
	sub := strings.TrimPrefix(strings.TrimSpace(path), "/")
	u := c.buildURL("/v1/uapi/"+url.PathEscape(slug)+"/"+sub, nil)

	// Buffer the body so executeRaw can replay it after a 402 or a retryable 5xx.
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("call user api: read body: %w", err)
		}
	}

	var reader io.Reader
	if bodyBytes != nil {
		reader = strings.NewReader(string(bodyBytes))
	}
	req, err := http.NewRequestWithContext(ctx, m, u, reader)
	if err != nil {
		return nil, err
	}
	if bodyBytes != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	c.applyAuth(req)

	resp, err := c.executeRaw(req, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("call user api: %w", err)
	}
	return resp, nil
}

// UserAPILeaderboard returns the top published APIs by usage.
// GET /api/user-api/leaderboard — public, no auth required.
func (c *Client) UserAPILeaderboard(ctx context.Context) (map[string]any, error) {
	var resp map[string]any
	if err := c.doGetInto(ctx, "/api/user-api/leaderboard", nil, &resp); err != nil {
		return nil, fmt.Errorf("user api leaderboard: %w", err)
	}
	return resp, nil
}

// UserAPIRatings returns the user ratings for a published API.
// GET /api/user-api/ratings/{slug} — public, no auth required.
func (c *Client) UserAPIRatings(ctx context.Context, slug string) (map[string]any, error) {
	if slug == "" {
		return nil, fmt.Errorf("user api ratings: slug is required")
	}
	var resp map[string]any
	path := "/api/user-api/ratings/" + url.PathEscape(slug)
	if err := c.doGetInto(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("user api ratings: %w", err)
	}
	return resp, nil
}
