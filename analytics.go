package jarvisclaw

import (
	"context"
	"fmt"
	"strconv"
)

// AnalyticsParams configures a time-ranged analytics query.
type AnalyticsParams struct {
	Start int64  // Unix timestamp, default: 24h ago
	End   int64  // Unix timestamp, default: now
	TopN  int    // Number of top entries (default: 10)
	Scope string // "self" or "global" (admin)
}

func (p AnalyticsParams) toMap() map[string]string {
	m := make(map[string]string)
	if p.Start > 0 {
		m["start"] = strconv.FormatInt(p.Start, 10)
	}
	if p.End > 0 {
		m["end"] = strconv.FormatInt(p.End, 10)
	}
	if p.TopN > 0 {
		m["top_n"] = strconv.Itoa(p.TopN)
	}
	if p.Scope != "" {
		m["scope"] = p.Scope
	}
	return m
}

// CostSummary returns aggregated cost data for a time range.
// GET /v1/aip/analytics/cost_summary
func (c *Client) CostSummary(ctx context.Context, params AnalyticsParams) (map[string]any, error) {
	resp, err := c.doGetCtx(ctx, "/v1/aip/analytics/cost_summary", params.toMap())
	if err != nil {
		return nil, fmt.Errorf("analytics cost_summary: %w", err)
	}
	return resp, nil
}

// CostTrend returns cost trend data points over a time range.
// GET /v1/aip/analytics/cost_trend
func (c *Client) CostTrend(ctx context.Context, params AnalyticsParams) (map[string]any, error) {
	resp, err := c.doGetCtx(ctx, "/v1/aip/analytics/cost_trend", params.toMap())
	if err != nil {
		return nil, fmt.Errorf("analytics cost_trend: %w", err)
	}
	return resp, nil
}

// BudgetStatus returns current budget utilization and limits.
// GET /v1/aip/analytics/budget_status
func (c *Client) BudgetStatus(ctx context.Context, params AnalyticsParams) (map[string]any, error) {
	resp, err := c.doGetCtx(ctx, "/v1/aip/analytics/budget_status", params.toMap())
	if err != nil {
		return nil, fmt.Errorf("analytics budget_status: %w", err)
	}
	return resp, nil
}

// ModelBreakdown returns per-model usage breakdown (tokens, cost, requests).
// GET /v1/aip/analytics/model_breakdown
func (c *Client) ModelBreakdown(ctx context.Context, params AnalyticsParams) (map[string]any, error) {
	resp, err := c.doGetCtx(ctx, "/v1/aip/analytics/model_breakdown", params.toMap())
	if err != nil {
		return nil, fmt.Errorf("analytics model_breakdown: %w", err)
	}
	return resp, nil
}

// ROI returns per-model return-on-investment metrics (tokens_per_dollar).
// GET /v1/aip/analytics/roi
func (c *Client) ROI(ctx context.Context, params AnalyticsParams) (map[string]any, error) {
	resp, err := c.doGetCtx(ctx, "/v1/aip/analytics/roi", params.toMap())
	if err != nil {
		return nil, fmt.Errorf("analytics roi: %w", err)
	}
	return resp, nil
}
