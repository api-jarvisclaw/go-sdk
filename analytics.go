package jarvisclaw

import (
	"context"
	"fmt"
	"strconv"
)

// AnalyticsParams configures an analytics query.
//
// Period is a coarse lookback window accepted by the server: "24h", "7d", "30d"
// or "90d". Anything else falls back to "7d" server-side.
type AnalyticsParams struct {
	// Period is the lookback window: "24h", "7d" (default), "30d", "90d".
	Period string
	// Model restricts results to a single model name.
	Model string
	// GroupBy selects the aggregation dimensions. Valid values: "day", "model",
	// "api_source", "principal_type", "channel", "group", "client_id".
	// Defaults to day,model,api_source when empty.
	GroupBy []string
	// UserID widens the scope to another user. Admin-only and ignored otherwise:
	// non-admin callers are always pinned to their own data server-side.
	// Zero means "no override" (self for users, global for admins).
	UserID int
	// Filter* apply exact-match filters on the corresponding dimension.
	FilterAPISource     string
	FilterPrincipalType string
	FilterClientID      string
	FilterChannel       string
	FilterGroup         string
}

func (p AnalyticsParams) toMap() map[string]string {
	m := make(map[string]string)
	if p.Period != "" {
		m["period"] = p.Period
	}
	if p.Model != "" {
		m["model"] = p.Model
	}
	if len(p.GroupBy) > 0 {
		joined := ""
		for i, d := range p.GroupBy {
			if i > 0 {
				joined += ","
			}
			joined += d
		}
		m["group_by"] = joined
	}
	if p.UserID > 0 {
		m["user_id"] = strconv.Itoa(p.UserID)
	}
	if p.FilterAPISource != "" {
		m["filter_api_source"] = p.FilterAPISource
	}
	if p.FilterPrincipalType != "" {
		m["filter_principal_type"] = p.FilterPrincipalType
	}
	if p.FilterClientID != "" {
		m["filter_client_id"] = p.FilterClientID
	}
	if p.FilterChannel != "" {
		m["filter_channel"] = p.FilterChannel
	}
	if p.FilterGroup != "" {
		m["filter_group"] = p.FilterGroup
	}
	return m
}

// SpendRow is one aggregated spend bucket returned by Spend.
//
// The dimension fields (Day, Model, APISource, …) are only populated for the
// dimensions requested via AnalyticsParams.GroupBy; the rest stay zero.
type SpendRow struct {
	Day           string `json:"day,omitempty"`
	Model         string `json:"model,omitempty"`
	APISource     string `json:"api_source,omitempty"`
	PrincipalType string `json:"principal_type,omitempty"`
	Channel       int    `json:"channel,omitempty"`
	Group         string `json:"group,omitempty"`
	ClientID      string `json:"client_id,omitempty"`

	TotalQuota   float64 `json:"total_quota"`
	TotalReqs    int64   `json:"total_reqs"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	RevenueUSD   float64 `json:"revenue_usd"`
	SettleDone   int64   `json:"settle_done"`
	SettleFailed int64   `json:"settle_failed"`
	Delivered    int64   `json:"delivered"`
	Undelivered  int64   `json:"undelivered"`
	LossUSD      float64 `json:"loss_usd"`
}

// Spend returns aggregated spend and settlement data for the caller.
//
// GET /api/analytics/aggregate — accepts API tokens and x402 callers. Scope is
// enforced server-side from the auth context: a non-admin only ever sees their
// own rows regardless of AnalyticsParams.UserID.
//
// This replaces the removed /v1/aip/analytics/* endpoints; AIP usage appears
// here with api_source="aip".
func (c *Client) Spend(ctx context.Context, params AnalyticsParams) ([]SpendRow, error) {
	var resp struct {
		Success bool       `json:"success"`
		Message string     `json:"message"`
		Data    []SpendRow `json:"data"`
	}
	if err := c.doGetInto(ctx, "/api/analytics/aggregate", params.toMap(), &resp); err != nil {
		return nil, fmt.Errorf("analytics spend: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("analytics spend: %s", resp.Message)
	}
	return resp.Data, nil
}

// CostByModel is a convenience wrapper over Spend that groups by model only,
// giving a per-model cost and request breakdown for the period.
func (c *Client) CostByModel(ctx context.Context, params AnalyticsParams) ([]SpendRow, error) {
	params.GroupBy = []string{"model"}
	return c.Spend(ctx, params)
}

// DailyTrend is a convenience wrapper over Spend that groups by day only,
// giving one row per calendar day in the period.
func (c *Client) DailyTrend(ctx context.Context, params AnalyticsParams) ([]SpendRow, error) {
	params.GroupBy = []string{"day"}
	return c.Spend(ctx, params)
}

// QualityRow is one (model, principal) bucket of mined quality signals.
//
// Rates are fractions in 0..1, not percentages.
type QualityRow struct {
	Model          string  `json:"model"`
	PrincipalType  string  `json:"principal_type"`
	Requests       int64   `json:"requests"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	CacheTokens    int64   `json:"cache_tokens"`
	TokenCacheRate float64 `json:"token_cache_rate"`
	ErrorRate      float64 `json:"error_rate"`
	ErrorRequests  int64   `json:"error_requests"`
	// AvgFrtMs is the mean time to first response token, in milliseconds.
	AvgFrtMs float64 `json:"avg_frt_ms"`
}

// QualityMetrics returns per-request quality signals mined from consume and
// marketplace logs, one row per (model, principal).
//
// GET /api/analytics/quality — accepts API tokens and x402 callers.
func (c *Client) QualityMetrics(ctx context.Context, params AnalyticsParams) ([]QualityRow, error) {
	var resp struct {
		Success bool         `json:"success"`
		Message string       `json:"message"`
		Data    []QualityRow `json:"data"`
	}
	if err := c.doGetInto(ctx, "/api/analytics/quality", params.toMap(), &resp); err != nil {
		return nil, fmt.Errorf("analytics quality: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("analytics quality: %s", resp.Message)
	}
	return resp.Data, nil
}

// Insights returns the deep-scan summary: a single pass over consume and
// marketplace logs folding cache, latency, reliability, pricing and mapping
// signals into a global summary plus a per-(model, principal) breakdown.
//
// GET /api/analytics/insights — accepts API tokens and x402 callers.
func (c *Client) Insights(ctx context.Context, params AnalyticsParams) (map[string]any, error) {
	var resp struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := c.doGetInto(ctx, "/api/analytics/insights", params.toMap(), &resp); err != nil {
		return nil, fmt.Errorf("analytics insights: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("analytics insights: %s", resp.Message)
	}
	return resp.Data, nil
}
