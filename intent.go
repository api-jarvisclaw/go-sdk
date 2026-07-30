package jarvisclaw

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	Pricing           Pricing `json:"pricing"`
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

// NaturalResolveRequest is the input for natural-language intent resolution.
type NaturalResolveRequest struct {
	// Query is the free-text request, e.g. "make me a 5 second clip of a cat".
	Query string `json:"query"`
	// SessionID carries a multi-turn clarification forward. Pass back the
	// SessionID from a previous "clarify" response.
	SessionID   string       `json:"session_id,omitempty"`
	Constraints *Constraints `json:"constraints,omitempty"`
	Preferences *Preferences `json:"preferences,omitempty"`
}

// NaturalResolveResponse is the result of natural-language resolution.
//
// Status is one of "resolved", "clarify", "budget_insufficient" or "no_match".
// On "clarify" the server needs more information: ask Clarify.Question and
// resolve again with the same SessionID.
type NaturalResolveResponse struct {
	Status     string          `json:"status"`
	SessionID  string          `json:"session_id,omitempty"`
	Intent     string          `json:"intent,omitempty"`
	Confidence float64         `json:"confidence,omitempty"`
	Matches    []NaturalMatch  `json:"matches,omitempty"`
	Clarify    *ClarifyPayload `json:"clarify,omitempty"`
	Message    string          `json:"message,omitempty"`
}

// NaturalMatch is a provider match from natural-language resolution. It differs
// from Match: providers are named rather than id'd, and price is already resolved.
type NaturalMatch struct {
	ProviderName string  `json:"provider_name"`
	Model        string  `json:"model,omitempty"`
	Intent       string  `json:"intent"`
	Score        float64 `json:"score"`
	PriceUSD     float64 `json:"price_usd,omitempty"`
	LatencyMs    int     `json:"latency_ms,omitempty"`
	Endpoint     string  `json:"endpoint,omitempty"`
}

// ClarifyPayload is a follow-up question the resolver needs answered.
type ClarifyPayload struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	// Round is the clarification round, 1 or 2.
	Round int `json:"round"`
}

// ResolveNatural resolves a free-text request to an intent and ranked providers,
// using embedding similarity with a keyword fallback.
//
// POST /v1/intent/resolve/natural — requires an API key or x402 payment
// (it consumes an embedding). Nothing is executed, so no provider is charged.
func (c *Client) ResolveNatural(ctx context.Context, req NaturalResolveRequest) (*NaturalResolveResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("resolve natural: query is required")
	}
	var resp NaturalResolveResponse
	if err := c.doJSON(ctx, "POST", "/v1/intent/resolve/natural", req, &resp); err != nil {
		return nil, fmt.Errorf("resolve natural: %w", err)
	}
	return &resp, nil
}

// NetworkStats summarises the size of the provider registry and federation.
type NetworkStats struct {
	TotalProviders int            `json:"total_providers"`
	BySource       map[string]int `json:"by_source"`
	// IntentTypes is a count, not a list.
	IntentTypes int `json:"intent_types"`
	// Federation is absent when the federation counts could not be read.
	Federation *struct {
		Servers        int `json:"servers"`
		HealthyServers int `json:"healthy_servers"`
		Resources      int `json:"resources"`
	} `json:"federation,omitempty"`
}

// NetworkStats returns provider and federation counts.
//
// GET /v1/network/stats — public, no auth required. Counts only; no provider
// entries are exposed.
func (c *Client) NetworkStats(ctx context.Context) (*NetworkStats, error) {
	var resp struct {
		Success bool         `json:"success"`
		Data    NetworkStats `json:"data"`
	}
	if err := c.doGetInto(ctx, "/v1/network/stats", nil, &resp); err != nil {
		return nil, fmt.Errorf("network stats: %w", err)
	}
	return &resp.Data, nil
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

// Provider is a registered provider entry as published by /v1/providers.
//
// This is the registry view, distinct from Match: it has no score or ranking
// reason, because nothing has been resolved yet.
type Provider struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	IntentTypes []string `json:"intent_types"`
	Pricing     Pricing  `json:"pricing"`
	Features    []string `json:"features"`
	Endpoint    string   `json:"endpoint"`
	// Source is "internal", "federation" or "marketplace".
	Source      string `json:"source"`
	ResourceID  int    `json:"resource_id"`
	ServerID    int    `json:"server_id"`
	Description string `json:"description,omitempty"`
}

// ListProviders returns every registered provider.
//
// GET /v1/providers — public, no auth required.
func (c *Client) ListProviders(ctx context.Context) ([]Provider, error) {
	var resp struct {
		Providers []Provider `json:"providers"`
		Total     int        `json:"total"`
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

// ── Discovery & Subscription ─────────────────────────────────────────────────

// DiscoverRequest is the input for capability discovery.
//
// All fields are optional: an empty request returns every intent and provider.
type DiscoverRequest struct {
	// Intent narrows results to one intent type. Empty means all.
	Intent string `json:"intent,omitempty"`
	// Features requires providers to support ALL listed features.
	Features []string `json:"features,omitempty"`
	// MaxPrice caps the estimated per-request price in USD. Zero means no cap.
	MaxPrice float64 `json:"max_price,omitempty"`
}

// DiscoverResponse is the result of a discovery query.
//
// Total counts Providers only; Intents and Federated are not included in it.
type DiscoverResponse struct {
	Intents   []DiscoveredIntent   `json:"intents"`
	Providers []DiscoveredProvider `json:"providers"`
	// Federated holds capability entries contributed by peer platforms. The
	// per-peer payload varies, so it stays untyped.
	Federated []map[string]any `json:"federated,omitempty"`
	Total     int              `json:"total"`
}

// DiscoveredIntent describes one supported intent type and how many providers
// can serve it.
type DiscoveredIntent struct {
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Features      []string `json:"features"`
	ProviderCount int      `json:"provider_count"`
}

// DiscoveredProvider represents a provider returned by discovery.
type DiscoveredProvider struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Intents  []string `json:"intents"`
	Features []string `json:"features"`
	Pricing  Pricing  `json:"pricing"`
	Endpoint string   `json:"endpoint"`
	// Source is "internal", "federation" or "marketplace".
	Source string `json:"source"`
}

// Pricing is a provider's published rate. Per-call and per-token pricing are
// mutually exclusive in practice: whichever applies is non-zero.
type Pricing struct {
	InputPerMillion  float64 `json:"input_per_million,omitempty"`
	OutputPerMillion float64 `json:"output_per_million,omitempty"`
	PerCall          float64 `json:"per_call,omitempty"`
}

// Discover lists the intents and providers this gateway and its federated peers
// can serve, optionally filtered.
//
// POST /v1/intent/discover — requires an API key or x402 payment.
// A free unauthenticated variant exists at GET /v1/intent/discover; see DiscoverPublic.
func (c *Client) Discover(ctx context.Context, req DiscoverRequest) (*DiscoverResponse, error) {
	var resp DiscoverResponse
	if err := c.doJSON(ctx, "POST", "/v1/intent/discover", req, &resp); err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}
	return &resp, nil
}

// DiscoverPublic is Discover over the free, unauthenticated GET route.
//
// GET /v1/intent/discover — no auth, rate-limited. Same response shape.
func (c *Client) DiscoverPublic(ctx context.Context, req DiscoverRequest) (*DiscoverResponse, error) {
	params := map[string]string{}
	if req.Intent != "" {
		params["intent"] = req.Intent
	}
	if len(req.Features) > 0 {
		params["features"] = strings.Join(req.Features, ",")
	}
	if req.MaxPrice > 0 {
		params["max_price"] = strconv.FormatFloat(req.MaxPrice, 'f', -1, 64)
	}
	var resp DiscoverResponse
	if err := c.doGetInto(ctx, "/v1/intent/discover", params, &resp); err != nil {
		return nil, fmt.Errorf("discover public: %w", err)
	}
	return &resp, nil
}

// SubscribeRequest is the input for streaming intent execution over SSE.
//
// Both Intent and Payload are required by the server. Payload is the provider
// request body (e.g. {"messages":[...]}); the server injects "stream":true and
// fills in "model" from the resolved provider if you omit it.
type SubscribeRequest struct {
	Intent      string         `json:"intent"`
	Payload     map[string]any `json:"payload"`
	Constraints *Constraints   `json:"constraints,omitempty"`
	Preferences *Preferences   `json:"preferences,omitempty"`
	// OptimizeFor is a shorthand for Preferences.OptimizeFor using the
	// subscribe-specific vocabulary: "speed", "cost" or "quality".
	OptimizeFor string `json:"optimize_for,omitempty"`
}

// SSEEvent represents a single Server-Sent Event from the subscribe stream.
//
// The first event is always "metadata" (provider, intent, model) and the last is
// "done". In between, upstream events are relayed verbatim, so for chat
// completions Data holds OpenAI-style chunk JSON — or the literal "[DONE]"
// sentinel, which is not JSON.
type SSEEvent struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

// SSEStream is a streaming reader for Server-Sent Events.
type SSEStream struct {
	resp    *http.Response
	scanner *bufio.Scanner
}

// Next reads the next SSE event from the stream.
// Returns io.EOF when the stream is exhausted.
//
// Per the SSE spec the field separator is "field:" with an optional single
// leading space, and repeated data: lines within one event are joined with
// newlines. Matching on "data: " with a hard-coded space would silently drop
// any upstream that writes "data:{...}".
func (s *SSEStream) Next() (*SSEEvent, error) {
	var event string
	var dataLines []string
	for s.scanner.Scan() {
		line := strings.TrimSuffix(s.scanner.Text(), "\r")
		if line == "" {
			// Empty line = event boundary. Dispatch only if we accumulated something.
			if event != "" || len(dataLines) > 0 {
				return &SSEEvent{Event: event, Data: strings.Join(dataLines, "\n")}, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // comment / keep-alive
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch name {
		case "event":
			event = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	// Flush a trailing event that the stream ended without a blank line after.
	if event != "" || len(dataLines) > 0 {
		return &SSEEvent{Event: event, Data: strings.Join(dataLines, "\n")}, nil
	}
	return nil, io.EOF
}

// Close releases the underlying HTTP response body.
func (s *SSEStream) Close() error {
	return s.resp.Body.Close()
}

// Subscribe opens a streaming SSE connection for real-time intent resolution events.
// POST /v1/intent/subscribe — requires auth.
// Caller must call stream.Close() when done.
func (c *Client) Subscribe(ctx context.Context, req SubscribeRequest) (*SSEStream, error) {
	if req.Payload == nil {
		return nil, fmt.Errorf("subscribe: payload is required")
	}
	resp, err := c.doPostRawCtx(ctx, "/v1/intent/subscribe", req)
	if err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}
	sc := bufio.NewScanner(resp.Body)
	// Default bufio.Scanner caps a token at 64 KiB; a single SSE data line can
	// exceed that (base64 images, long tool-call arguments), which would end the
	// stream mid-response with ErrTooLong. Match the gateway's own 1 MiB relay cap.
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	return &SSEStream{resp: resp, scanner: sc}, nil
}

// Subscription represents an active streaming subscription.
//
// Subscriptions are tracked in memory by the gateway, so the list resets when
// the server restarts and is not shared across instances.
type Subscription struct {
	ID          string   `json:"id"`
	UserID      int      `json:"user_id"`
	IntentTypes []string `json:"intent_types"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"created_at"`
}

// Unsubscribe cancels an active subscription by ID.
// DELETE /v1/intent/subscribe/:id
func (c *Client) Unsubscribe(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("unsubscribe: subscription id is required")
	}
	path := "/v1/intent/subscribe/" + url.PathEscape(subscriptionID)
	if err := c.doJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}
	return nil
}

// ListSubscriptions returns all active subscriptions for the authenticated user.
// GET /v1/intent/subscribe
func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	var resp struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/intent/subscribe", nil, &resp); err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return resp.Subscriptions, nil
}
