package jarvisclaw

import (
	"context"
	"encoding/json"
	"fmt"
)

// ResolveRequest is the input for intent resolution.
type ResolveRequest struct {
	Intent      string      `json:"intent"`
	Constraints Constraints `json:"constraints,omitempty"`
	Preferences Preferences `json:"preferences,omitempty"`
}

// Constraints limits the set of providers considered during resolution.
type Constraints struct {
	MaxPriceUSD  *float64 `json:"max_price_usd,omitempty"`
	MaxLatencyMS *int     `json:"max_latency_ms,omitempty"`
	Features     []string `json:"features,omitempty"`
}

// Preferences express soft optimization goals for resolution.
type Preferences struct {
	OptimizeFor string `json:"optimize_for,omitempty"` // "cost", "quality", "latency"
	Limit       int    `json:"limit,omitempty"`
}

// ResolveResponse is the result of an intent resolution request.
type ResolveResponse struct {
	Matches        []Match `json:"matches"`
	IntentType     string  `json:"intent_type"`
	TotalAvailable int     `json:"total_available"`
}

// Match represents a single provider candidate returned by intent resolution.
type Match struct {
	ProviderID        string  `json:"provider_id"`
	Score             float64 `json:"score"`
	EstimatedPriceUSD float64 `json:"estimated_price_usd"`
	Endpoint          string  `json:"endpoint"`
	Model             string  `json:"model"`
	Reason            string  `json:"reason"`
}

// ExecuteRequest is the request body for POST /v1/intent/execute.
type ExecuteRequest struct {
	Intent      string         `json:"intent"`
	Constraints *Constraints   `json:"constraints,omitempty"`
	Preferences *Preferences   `json:"preferences,omitempty"`
	Payload     map[string]any `json:"payload"`
}

// ExecuteBudgetRequest is the request body for POST /v1/intent/execute-budget.
type ExecuteBudgetRequest struct {
	Intent  string         `json:"intent"`
	Budget  Budget         `json:"budget"`
	Payload map[string]any `json:"payload"`
}

// Budget defines spending constraints for budget-controlled execution.
type Budget struct {
	MaxTotalUSD            float64 `json:"max_total_usd"`
	PreferredPaymentMethod string  `json:"preferred_payment_method,omitempty"`
	AllowOverdraft         bool    `json:"allow_overdraft,omitempty"`
}

// BudgetResult is the response from POST /v1/intent/execute-budget.
type BudgetResult struct {
	RequestID     string      `json:"request_id"`
	Status        string      `json:"status"` // "success", "rejected", "error"
	Provider      string      `json:"provider,omitempty"`
	Model         string      `json:"model,omitempty"`
	Result        any         `json:"result,omitempty"`
	ActualCostUSD *float64    `json:"actual_cost_usd,omitempty"`
	Settlement    *Settlement `json:"settlement,omitempty"`
	RiskLevel     string      `json:"risk_level,omitempty"`
	DurationMS    int         `json:"duration_ms"`
	Reason        string      `json:"reason,omitempty"`
}

// Settlement contains payment settlement details.
type Settlement struct {
	ID            string             `json:"id"`
	RequestID     string             `json:"request_id"`
	UserID        int                `json:"user_id"`
	PayerAddress  string             `json:"payer_address"`
	Decision      SettlementDecision `json:"decision"`
	ActualCostUSD float64            `json:"actual_cost_usd"`
	Status        string             `json:"status"`
	CreatedAt     string             `json:"created_at"`
	ConfirmedAt   string             `json:"confirmed_at"`
}

// SettlementDecision describes how payment was routed.
type SettlementDecision struct {
	Method        string `json:"method"`
	QuotaToDeduct int    `json:"quota_to_deduct"`
	Reason        string `json:"reason"`
}

// AuditResponse is the response from GET /v1/intent/audit.
type AuditResponse struct {
	Entries []AuditEntry `json:"entries"`
	Count   int          `json:"count"`
}

// AuditEntry represents a single audit trail event.
type AuditEntry struct {
	Timestamp string         `json:"timestamp"`
	RequestID string         `json:"request_id"`
	UserID    int            `json:"user_id"`
	EventType string         `json:"event_type"`
	Details   map[string]any `json:"details"`
}

// Resolve finds the optimal provider for a given intent.
// POST /v1/intent/resolve — free endpoint, auth optional but accepted.
func (c *Client) Resolve(ctx context.Context, req ResolveRequest) (*ResolveResponse, error) {
	var resp ResolveResponse
	if err := c.doJSON(ctx, "POST", "/v1/intent/resolve", req, &resp); err != nil {
		return nil, fmt.Errorf("resolve intent: %w", err)
	}
	return &resp, nil
}

// Execute resolves an intent and forwards the payload to the best provider.
// POST /v1/intent/execute — requires auth.
// Returns the raw upstream provider response.
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (json.RawMessage, error) {
	raw, err := c.doRawJSON(ctx, "POST", "/v1/intent/execute", req)
	if err != nil {
		return nil, fmt.Errorf("execute intent: %w", err)
	}
	return raw, nil
}

// ExecuteBudget resolves, pays, and executes with budget control.
// POST /v1/intent/execute-budget — requires auth.
func (c *Client) ExecuteBudget(ctx context.Context, req ExecuteBudgetRequest) (*BudgetResult, error) {
	var resp BudgetResult
	if err := c.doJSON(ctx, "POST", "/v1/intent/execute-budget", req, &resp); err != nil {
		return nil, fmt.Errorf("execute budget: %w", err)
	}
	return &resp, nil
}

// Audit returns the audit trail for recent requests.
// GET /v1/intent/audit — requires auth.
func (c *Client) Audit(ctx context.Context) (*AuditResponse, error) {
	var resp AuditResponse
	if err := c.doJSON(ctx, "GET", "/v1/intent/audit", nil, &resp); err != nil {
		return nil, fmt.Errorf("intent audit: %w", err)
	}
	return &resp, nil
}

// ListProviders returns all available providers.
// GET /v1/providers
func (c *Client) ListProviders(ctx context.Context) ([]Match, error) {
	var resp struct {
		Providers []Match `json:"providers"`
		Total     int     `json:"total"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/providers", nil, &resp); err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	return resp.Providers, nil
}

// ListIntentTypes returns the supported intent type identifiers.
// GET /v1/intent/types
func (c *Client) ListIntentTypes(ctx context.Context) ([]string, error) {
	var resp struct {
		IntentTypes []string `json:"intent_types"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/intent/types", nil, &resp); err != nil {
		return nil, fmt.Errorf("list intent types: %w", err)
	}
	return resp.IntentTypes, nil
}
