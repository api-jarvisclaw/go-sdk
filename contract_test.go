package jarvisclaw_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk/v2"
)

// Contract tests: no network, no credentials. Each one pins a decoder against
// the exact JSON the gateway emits, so a server-side shape change surfaces here
// instead of as a silently zero-valued struct at runtime.
//
// Every fixture below is copied from the handler that produces it, cited in the
// comment. Run with: go test -run TestContract ./...

// newStub returns a client pointed at a test server that answers every request
// with the given status and body, plus the path it last saw.
func newStub(t *testing.T, status int, body string) (*jarvisclaw.Client, *struct {
	Method string
	Path   string
	Query  string
	Body   string
}) {
	t.Helper()
	seen := &struct {
		Method string
		Path   string
		Query  string
		Body   string
	}{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Method = r.Method
		seen.Path = r.URL.Path
		seen.Query = r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		seen.Body = string(raw)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c, err := jarvisclaw.NewClient(
		jarvisclaw.WithAPIKey("sk-test"),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, seen
}

// controller/wallet/balance.go GetBalance
func TestContractWalletBalance(t *testing.T) {
	const body = `{
	  "balance_usd": "5.960000",
	  "wallets": {
	    "base":   {"usdc": "5.910000", "address": "0xabc0000000000000000000000000000000000001"},
	    "solana": {"usdc": "0.050000", "address": "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU"}
	  }
	}`
	c, seen := newStub(t, 200, body)

	got, err := c.WalletBalance(context.Background())
	if err != nil {
		t.Fatalf("WalletBalance: %v", err)
	}
	if seen.Path != "/v1/wallet/balance" {
		t.Errorf("path = %q", seen.Path)
	}
	if got.BalanceUSD != "5.960000" {
		t.Errorf("BalanceUSD = %q, want 5.960000", got.BalanceUSD)
	}
	if got.TotalUSD() != 5.96 {
		t.Errorf("TotalUSD() = %v, want 5.96", got.TotalUSD())
	}
	if got.Wallets.Base.USDC != "5.910000" {
		t.Errorf("base usdc = %q", got.Wallets.Base.USDC)
	}
	if got.Wallets.Solana.Address == "" {
		t.Error("solana address not decoded")
	}
}

// controller/wallet/pools.go GetPools
func TestContractWalletPools(t *testing.T) {
	const body = `{
	  "allocation": {"operations":0.6,"insurance":0.15,"savings":0.15,"dividends":0.1},
	  "pool_balances": {"operations":"3.5760","insurance":"0.8940","savings":"0.8940","dividends":"0.5960"}
	}`
	c, _ := newStub(t, 200, body)

	got, err := c.WalletPools(context.Background())
	if err != nil {
		t.Fatalf("WalletPools: %v", err)
	}
	if got.Allocation.Operations != 0.6 {
		t.Errorf("operations allocation = %v", got.Allocation.Operations)
	}
	if got.PoolBalances.Dividends != "0.5960" {
		t.Errorf("dividends balance = %q", got.PoolBalances.Dividends)
	}
}

// controller/wallet/history.go GetHistory
func TestContractWalletHistory(t *testing.T) {
	const body = `{
	  "transactions": [
	    {"id":9,"amount_quota":-5000,"category":"inference","model":"gpt-4o","use_time_seconds":3,"created_at":1751000000}
	  ],
	  "total": 1,
	  "page": 1
	}`
	c, seen := newStub(t, 200, body)

	got, err := c.WalletHistory(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("WalletHistory: %v", err)
	}
	if !strings.Contains(seen.Query, "page=1") || !strings.Contains(seen.Query, "page_size=20") {
		t.Errorf("query = %q", seen.Query)
	}
	if len(got.Transactions) != 1 || got.Transactions[0].Category != "inference" {
		t.Fatalf("transactions = %+v", got.Transactions)
	}
	// amount_quota is negated spend, so it is legitimately negative.
	if got.Transactions[0].AmountQuota != -5000 {
		t.Errorf("amount_quota = %d", got.Transactions[0].AmountQuota)
	}
}

// controller/wallet/limits.go — a read-modify-write must preserve untouched fields,
// because PUT replaces the whole row (model.UpsertUserWalletLimits uses DB.Save).
func TestContractUpdateWalletLimitPreservesOtherFields(t *testing.T) {
	const body = `{
	  "user_id": 42, "daily_max_usd": 50, "per_request_max_usd": 1,
	  "monthly_max_usd": 500, "auto_pause_below_usd": 2,
	  "pool_allocation": "{\"operations\":0.6,\"insurance\":0.15,\"savings\":0.15,\"dividends\":0.1}",
	  "updated_at": 1751000000
	}`
	c, seen := newStub(t, 200, body)

	err := c.UpdateWalletLimit(context.Background(), func(l *jarvisclaw.WalletLimits) {
		l.DailyMaxUSD = 30
	})
	if err != nil {
		t.Fatalf("UpdateWalletLimit: %v", err)
	}
	if seen.Method != http.MethodPut {
		t.Errorf("method = %s", seen.Method)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(seen.Body), &sent); err != nil {
		t.Fatalf("sent body not JSON: %v (%s)", err, seen.Body)
	}
	if sent["daily_max_usd"] != 30.0 {
		t.Errorf("daily_max_usd = %v, want 30", sent["daily_max_usd"])
	}
	// The whole point: these must survive the replacing PUT.
	if sent["monthly_max_usd"] != 500.0 {
		t.Errorf("monthly_max_usd = %v, want 500 preserved", sent["monthly_max_usd"])
	}
	if sent["per_request_max_usd"] != 1.0 {
		t.Errorf("per_request_max_usd = %v, want 1 preserved", sent["per_request_max_usd"])
	}
	if _, ok := sent["pool_allocation"]; !ok {
		t.Error("pool_allocation dropped; GetPools would fall back to defaults")
	}
}

// controller/aip/federation.go FederationStatus — {success,data} with camelCase.
func TestContractNetworkPeers(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": [
	    {"id":1,"name":"peer-a","url":"https://a.example","status":"online",
	     "lastSeen":"2026-07-30T00:00:00Z","resourceCount":12,"latencyMs":85,
	     "aipVersion":"1.0","discoverUrl":"https://a.example/.well-known/aip.json"}
	  ]
	}`
	c, _ := newStub(t, 200, body)

	peers, err := c.NetworkPeers(context.Background())
	if err != nil {
		t.Fatalf("NetworkPeers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("peers = %+v", peers)
	}
	p := peers[0]
	if p.Name != "peer-a" || p.URL != "https://a.example" {
		t.Errorf("peer = %+v", p)
	}
	if p.ResourceCount != 12 || p.LatencyMs != 85 {
		t.Errorf("camelCase fields not decoded: %+v", p)
	}
	if !p.Healthy() {
		t.Error("Healthy() = false for status=online")
	}
}

// Both of these pin the error envelope, not the happy path. The gateway reports
// auth failures as HTTP 200 with success=false and names the reason "message";
// these two decoders used to read only "error" and gate on `resp.Error != ""`,
// so an unauthorised caller silently got an empty list instead of an error.
// Caught by an end-to-end run against production, not by the suite — hence these.
func TestContractNetworkPeersSurfacesSuccessFalse(t *testing.T) {
	c, _ := newStub(t, 200, `{"success":false,"message":"Unauthorized, invalid access token"}`)

	peers, err := c.NetworkPeers(context.Background())
	if err == nil {
		t.Fatalf("expected error when success=false on a 200, got peers=%+v", peers)
	}
	if !strings.Contains(err.Error(), "Unauthorized, invalid access token") {
		t.Errorf("error must carry the message field, got %q", err)
	}
}

func TestContractListAPIsSurfacesSuccessFalse(t *testing.T) {
	c, _ := newStub(t, 200, `{"success":false,"message":"Unauthorized, invalid access token"}`)

	page, err := c.ListAPIs(context.Background(), jarvisclaw.CatalogueParams{})
	if err == nil {
		t.Fatalf("expected error when success=false on a 200, got page=%+v", page)
	}
	if !strings.Contains(err.Error(), "Unauthorized, invalid access token") {
		t.Errorf("error must carry the message field, got %q", err)
	}
}

// controller/aip/federation.go FederationRemovePeer — domain in the body, no path id.
func TestContractRemoveNetworkPeerSendsDomain(t *testing.T) {
	c, seen := newStub(t, 200, `{"message":"peer removed","domain":"a.example"}`)

	if err := c.RemoveNetworkPeer(context.Background(), "a.example"); err != nil {
		t.Fatalf("RemoveNetworkPeer: %v", err)
	}
	if seen.Method != http.MethodDelete {
		t.Errorf("method = %s", seen.Method)
	}
	if seen.Path != "/v1/aip/federation/peers" {
		t.Errorf("path = %q, want no trailing id", seen.Path)
	}
	var sent map[string]string
	if err := json.Unmarshal([]byte(seen.Body), &sent); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, seen.Body)
	}
	if sent["domain"] != "a.example" {
		t.Errorf("body = %v, want domain", sent)
	}
}

func TestContractRemoveNetworkPeerRejectsEmpty(t *testing.T) {
	c, _ := newStub(t, 200, `{}`)
	if err := c.RemoveNetworkPeer(context.Background(), ""); err == nil {
		t.Error("expected error for empty domain")
	}
}

// controller/analytics/analytics.go GetAggregate — jsonSuccess wraps in {success,data}.
func TestContractSpend(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": [
	    {"day":"2026-07-29","model":"gpt-4o","api_source":"aip","total_quota":25000,
	     "total_reqs":4,"total_cost_usd":0.05,"revenue_usd":0.06,"settle_done":4,
	     "settle_failed":0,"delivered":4,"undelivered":0,"loss_usd":0}
	  ]
	}`
	c, seen := newStub(t, 200, body)

	rows, err := c.Spend(context.Background(), jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if seen.Path != "/api/analytics/aggregate" {
		t.Errorf("path = %q", seen.Path)
	}
	if !strings.Contains(seen.Query, "period=7d") {
		t.Errorf("query = %q", seen.Query)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Model != "gpt-4o" || rows[0].TotalCostUSD != 0.05 || rows[0].TotalReqs != 4 {
		t.Errorf("row = %+v", rows[0])
	}
	if rows[0].APISource != "aip" {
		t.Errorf("api_source = %q; AIP spend must be visible here", rows[0].APISource)
	}
}

func TestContractSpendPropagatesServerFailure(t *testing.T) {
	c, _ := newStub(t, 200, `{"success":false,"message":"unauthorized"}`)
	if _, err := c.Spend(context.Background(), jarvisclaw.AnalyticsParams{}); err == nil {
		t.Error("expected error when success=false")
	}
}

// GET /api/analytics/quality returns data as an ARRAY of per-(model, principal)
// rows, not an object. Typing it as a map made the decode fail outright.
func TestContractQualityMetrics(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": [
	    {"model":"text-embedding-3-small","principal_type":"user","requests":12,
	     "cache_hit_rate":0.25,"cache_tokens":40,"token_cache_rate":0.1,
	     "error_rate":0,"error_requests":0,"avg_frt_ms":180.5}
	  ]
	}`
	c, seen := newStub(t, 200, body)

	rows, err := c.QualityMetrics(context.Background(), jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		t.Fatalf("QualityMetrics: %v", err)
	}
	if seen.Path != "/api/analytics/quality" {
		t.Errorf("path = %q", seen.Path)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[0]
	if r.Model != "text-embedding-3-small" || r.PrincipalType != "user" || r.Requests != 12 {
		t.Errorf("row = %+v", r)
	}
	// Rates are fractions, not percentages.
	if r.CacheHitRate != 0.25 || r.AvgFrtMs != 180.5 {
		t.Errorf("metrics not decoded: %+v", r)
	}
}

// GET /api/analytics/insights returns data as an OBJECT (summary + breakdown),
// unlike /quality. The two are deliberately typed differently.
func TestContractInsights(t *testing.T) {
	const body = `{"success":true,"data":{"period":"7d","summary":{"requests":9},"breakdown":[]}}`
	c, seen := newStub(t, 200, body)

	got, err := c.Insights(context.Background(), jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		t.Fatalf("Insights: %v", err)
	}
	if seen.Path != "/api/analytics/insights" {
		t.Errorf("path = %q", seen.Path)
	}
	for _, k := range []string{"period", "summary", "breakdown"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %v", k, got)
		}
	}
}

func TestContractCostByModelForcesGrouping(t *testing.T) {
	c, seen := newStub(t, 200, `{"success":true,"data":[]}`)
	if _, err := c.CostByModel(context.Background(), jarvisclaw.AnalyticsParams{GroupBy: []string{"day"}}); err != nil {
		t.Fatalf("CostByModel: %v", err)
	}
	if !strings.Contains(seen.Query, "group_by=model") {
		t.Errorf("query = %q, want group_by=model to override caller's GroupBy", seen.Query)
	}
}

// controller/prompt_coach_x402.go PostPromptCoachOptimize — {success,data} envelope,
// 1-100 integer scores.
func TestContractPromptCoach(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": {
	    "original_prompt":"make a site","optimized_prompt":"Build a portfolio site...",
	    "explanation":"Added scope and audience.","score_before":35,"score_after":88,
	    "suggestions":["state the audience","name the stack"],
	    "model_used":"deepseek/deepseek-chat"
	  }
	}`
	c, seen := newStub(t, 200, body)

	got, err := c.PromptCoach(context.Background(), jarvisclaw.PromptCoachRequest{Prompt: "make a site"})
	if err != nil {
		t.Fatalf("PromptCoach: %v", err)
	}
	if seen.Path != "/v1/prompt-coach/optimize" {
		t.Errorf("path = %q", seen.Path)
	}
	if got.ScoreBefore != 35 || got.ScoreAfter != 88 {
		t.Errorf("scores = %d/%d, want 35/88 on the 1-100 scale", got.ScoreBefore, got.ScoreAfter)
	}
	if got.OptimizedPrompt == "" || got.ModelUsed == "" || len(got.Suggestions) != 2 {
		t.Errorf("data not unwrapped from envelope: %+v", got)
	}
}

// controller/aip/discover.go Discover — {intents,providers,federated,total}.
func TestContractDiscover(t *testing.T) {
	const body = `{
	  "intents": [{"type":"chat_completion","description":"Text generation","features":["streaming"],"provider_count":3}],
	  "providers": [{"id":"p1","name":"Provider One","intents":["chat_completion"],
	    "features":["streaming"],"pricing":{"input_per_million":0.5,"output_per_million":1.5},
	    "endpoint":"/v1/chat/completions","source":"internal"}],
	  "federated": [{"source":"peer-a"}],
	  "total": 1
	}`
	c, _ := newStub(t, 200, body)

	got, err := c.Discover(context.Background(), jarvisclaw.DiscoverRequest{Intent: "chat_completion"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Intents) != 1 || got.Intents[0].ProviderCount != 3 {
		t.Errorf("intents = %+v", got.Intents)
	}
	if len(got.Providers) != 1 || got.Providers[0].ID != "p1" {
		t.Fatalf("providers = %+v", got.Providers)
	}
	if got.Providers[0].Pricing.InputPerMillion != 0.5 {
		t.Errorf("pricing not decoded: %+v", got.Providers[0].Pricing)
	}
	if got.Providers[0].Source != "internal" {
		t.Errorf("source = %q", got.Providers[0].Source)
	}
	if len(got.Federated) != 1 {
		t.Errorf("federated = %+v", got.Federated)
	}
}

func TestContractDiscoverPublicUsesGET(t *testing.T) {
	c, seen := newStub(t, 200, `{"intents":[],"providers":[],"total":0}`)
	_, err := c.DiscoverPublic(context.Background(), jarvisclaw.DiscoverRequest{
		Intent:   "web_search",
		Features: []string{"citations", "fresh"},
		MaxPrice: 0.02,
	})
	if err != nil {
		t.Fatalf("DiscoverPublic: %v", err)
	}
	if seen.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", seen.Method)
	}
	if !strings.Contains(seen.Query, "features=citations%2Cfresh") {
		t.Errorf("query = %q, want comma-joined features", seen.Query)
	}
	if !strings.Contains(seen.Query, "max_price=0.02") {
		t.Errorf("query = %q", seen.Query)
	}
}

// controller/aip/provider.go ListProviders — registry entries, not resolve matches.
func TestContractListProviders(t *testing.T) {
	const body = `{
	  "providers": [{"id":"internal:gpt-4o","name":"gpt-4o","intent_types":["chat_completion"],
	    "pricing":{"input_per_million":2.5,"output_per_million":10},"features":["tools"],
	    "endpoint":"/v1/chat/completions","source":"internal","resource_id":0,"server_id":0}],
	  "total": 1
	}`
	c, _ := newStub(t, 200, body)

	got, err := c.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("providers = %+v", got)
	}
	if got[0].ID != "internal:gpt-4o" || len(got[0].IntentTypes) != 1 {
		t.Errorf("provider = %+v", got[0])
	}
	if got[0].Pricing.OutputPerMillion != 10 {
		t.Errorf("pricing = %+v", got[0].Pricing)
	}
}

// service/aip/types.go ResolveResponse
func TestContractResolve(t *testing.T) {
	const body = `{
	  "matches":[{"provider_id":"p1","score":0.91,"estimated_price_usd":0.0012,
	    "pricing":{"input_per_million":0.5,"output_per_million":1.5},
	    "endpoint":"/v1/chat/completions","model":"gpt-4o-mini","reason":"cheapest"}],
	  "intent_type":"chat_completion","total_available":7
	}`
	c, _ := newStub(t, 200, body)

	got, err := c.Resolve(context.Background(), jarvisclaw.ResolveRequest{Intent: "chat_completion"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.TotalAvailable != 7 || got.IntentType != "chat_completion" {
		t.Errorf("resp = %+v", got)
	}
	if len(got.Matches) != 1 || got.Matches[0].Model != "gpt-4o-mini" {
		t.Fatalf("matches = %+v", got.Matches)
	}
	if got.Matches[0].Pricing.InputPerMillion != 0.5 {
		t.Errorf("match pricing not decoded: %+v", got.Matches[0].Pricing)
	}
}

// service/aip/types.go NaturalResolveResponse — the clarify branch.
func TestContractResolveNaturalClarify(t *testing.T) {
	const body = `{
	  "status":"clarify","session_id":"s-1",
	  "clarify":{"question":"How long should the clip be?","options":["5s","10s"],"round":1}
	}`
	c, seen := newStub(t, 200, body)

	got, err := c.ResolveNatural(context.Background(), jarvisclaw.NaturalResolveRequest{Query: "make a cat video"})
	if err != nil {
		t.Fatalf("ResolveNatural: %v", err)
	}
	if seen.Path != "/v1/intent/resolve/natural" {
		t.Errorf("path = %q", seen.Path)
	}
	if got.Status != "clarify" || got.SessionID != "s-1" {
		t.Errorf("resp = %+v", got)
	}
	if got.Clarify == nil || got.Clarify.Round != 1 || len(got.Clarify.Options) != 2 {
		t.Fatalf("clarify = %+v", got.Clarify)
	}
}

func TestContractResolveNaturalRejectsBlankQuery(t *testing.T) {
	c, _ := newStub(t, 200, `{}`)
	if _, err := c.ResolveNatural(context.Background(), jarvisclaw.NaturalResolveRequest{Query: "   "}); err == nil {
		t.Error("expected error for blank query")
	}
}

// controller/aip/provider.go NetworkStats — {success,data}, intent_types is a count.
func TestContractNetworkStats(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": {"total_providers":42,"by_source":{"internal":30,"federation":12},
	    "intent_types":13,"federation":{"servers":4,"healthy_servers":3,"resources":88}}
	}`
	c, _ := newStub(t, 200, body)

	got, err := c.NetworkStats(context.Background())
	if err != nil {
		t.Fatalf("NetworkStats: %v", err)
	}
	if got.TotalProviders != 42 || got.IntentTypes != 13 {
		t.Errorf("stats = %+v", got)
	}
	if got.BySource["federation"] != 12 {
		t.Errorf("by_source = %v", got.BySource)
	}
	if got.Federation == nil || got.Federation.HealthyServers != 3 {
		t.Fatalf("federation = %+v", got.Federation)
	}
}

// NetworkStats omits "federation" when the DB read fails; that must not error.
func TestContractNetworkStatsWithoutFederation(t *testing.T) {
	c, _ := newStub(t, 200, `{"success":true,"data":{"total_providers":5,"by_source":{},"intent_types":13}}`)
	got, err := c.NetworkStats(context.Background())
	if err != nil {
		t.Fatalf("NetworkStats: %v", err)
	}
	if got.Federation != nil {
		t.Errorf("Federation = %+v, want nil when absent", got.Federation)
	}
}

// controller/billing.go GetSubscription — OpenAI-compatible billing shape.
func TestContractGetBalanceAPIKeyMode(t *testing.T) {
	const body = `{"object":"billing_subscription","has_payment_method":true,
	  "soft_limit_usd":5.96,"hard_limit_usd":5.96,"system_hard_limit_usd":5.96,"access_until":0}`
	c, seen := newStub(t, 200, body)

	got, err := c.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	// Not /api/user/self: that route needs a dashboard session, which an API key
	// cannot provide.
	if seen.Path != "/v1/dashboard/billing/subscription" {
		t.Errorf("path = %q", seen.Path)
	}
	if got != 5.96 {
		t.Errorf("balance = %v, want 5.96", got)
	}
}

// GetSubscription answers 200 with an {"error":...} body instead of a 4xx.
func TestContractGetBalanceSurfaces200Error(t *testing.T) {
	c, _ := newStub(t, 200, `{"error":{"message":"token not found","type":"upstream_error"}}`)
	if _, err := c.GetBalance(context.Background()); err == nil {
		t.Error("expected error for 200-with-error-body")
	}
}

// relay/channel/blockrun/handler_image.go — a slow model answers with a job,
// not an image.
func TestContractImageAsyncJobNonBlocking(t *testing.T) {
	const body = `{"id":"img_abc123","status":"queued","poll_url":"/v1/images/generations/img_abc123"}`
	c, _ := newStub(t, 200, body)

	ic := jarvisclaw.ImageClient{Client: c}
	got, err := ic.Generate(context.Background(), "a cat", jarvisclaw.WithImageWait(false))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.ID != "img_abc123" || got.Status != "queued" {
		t.Errorf("job = %+v", got)
	}
	if got.Done() {
		t.Error("Done() = true for a queued job")
	}
}

func TestContractImageInline(t *testing.T) {
	const body = `{"data":[{"url":"https://cdn.example/i.png","revised_prompt":"a fluffy cat"}]}`
	c, _ := newStub(t, 200, body)

	ic := jarvisclaw.ImageClient{Client: c}
	got, err := ic.Generate(context.Background(), "a cat")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.URL != "https://cdn.example/i.png" || !got.Done() {
		t.Errorf("image = %+v", got)
	}
	if got.RevisedPrompt != "a fluffy cat" {
		t.Errorf("revised_prompt = %q", got.RevisedPrompt)
	}
}

// The /v1/prediction prefix must be added once, never twice.
func TestContractPredictionPathPrefixing(t *testing.T) {
	cases := []struct{ in, want string }{
		{"markets", "/v1/prediction/markets"},
		{"/markets", "/v1/prediction/markets"},
		{"/v1/prediction/markets", "/v1/prediction/markets"},
		{"", "/v1/prediction/"},
	}
	for _, tc := range cases {
		c, seen := newStub(t, 200, `{}`)
		if _, err := c.Prediction(context.Background(), "GET", tc.in, nil); err != nil {
			t.Fatalf("Prediction(%q): %v", tc.in, err)
		}
		if seen.Path != tc.want {
			t.Errorf("Prediction(%q) path = %q, want %q", tc.in, seen.Path, tc.want)
		}
	}
}

func TestContractPredictionRejectsBadMethod(t *testing.T) {
	c, _ := newStub(t, 200, `{}`)
	if _, err := c.Prediction(context.Background(), "DELETE", "markets", nil); err == nil {
		t.Error("expected error for unsupported method")
	}
}

// controller/user_api_public.go GetUserAPIList — 200 with success:false on failure.
func TestContractListUserAPIs(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": [{"id":3,"slug":"weather","name":"Weather","description":"forecasts",
	    "price_per_call":0.013,"max_rpm":60,"category":"data","status":1,
	    "avg_rating":4.5,"total_calls":900,"created_at":1751000000}],
	  "total": 1, "page": 1, "page_size": 20
	}`
	c, seen := newStub(t, 200, body)

	apis, total, err := c.ListUserAPIs(context.Background(), jarvisclaw.UserAPIListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserAPIs: %v", err)
	}
	if seen.Path != "/api/user-api/list" {
		t.Errorf("path = %q", seen.Path)
	}
	if total != 1 || len(apis) != 1 {
		t.Fatalf("apis = %+v total = %d", apis, total)
	}
	if apis[0].Slug != "weather" || apis[0].PricePerCall != 0.013 {
		t.Errorf("api = %+v", apis[0])
	}
}

func TestContractListUserAPIsSurfacesSuccessFalse(t *testing.T) {
	c, _ := newStub(t, 200, `{"success":false,"message":"db down"}`)
	if _, _, err := c.ListUserAPIs(context.Background(), jarvisclaw.UserAPIListParams{}); err == nil {
		t.Error("expected error when success=false on a 200")
	}
}

func TestContractCallUserAPIPath(t *testing.T) {
	c, seen := newStub(t, 200, `{"ok":true}`)
	if _, err := c.CallUserAPI(context.Background(), "POST", "weather", "forecast", map[string]any{"city": "Tokyo"}); err != nil {
		t.Fatalf("CallUserAPI: %v", err)
	}
	if seen.Path != "/v1/uapi/weather/forecast" {
		t.Errorf("path = %q", seen.Path)
	}
	if !strings.Contains(seen.Body, "Tokyo") {
		t.Errorf("body = %q", seen.Body)
	}
}

// controller/federation.go SearchFederationResources — public projection.
//
// The stub body carries resource_id because the live endpoint does. It did not,
// and neither did NetworkAPI, so this test passed while the SDK dropped
// the one field that makes a search result callable — the implementation and the
// fixture were wrong together, which is exactly what a contract test cannot catch
// when its fixture is written from the implementation instead of the server.
func TestContractSearchFederation(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": [{"resource_id":476,"name":"price","path":"/price","method":"GET","description":"spot price",
	    "category":"crypto","tags":"defi","price_input":0,"price_output":0,
	    "sell_price":0.002,"price_unit":"call","currency":"USDC","network":"base",
	    "popular":true,"call_count":120,"server_name":"peer-a","updated_at":1751000000}],
	  "count": 1
	}`
	c, seen := newStub(t, 200, body)

	got, err := c.SearchNetwork(context.Background(), jarvisclaw.NetworkSearchParams{Query: "price", Limit: 5})
	if err != nil {
		t.Fatalf("SearchFederation: %v", err)
	}
	if !strings.Contains(seen.Query, "q=price") || !strings.Contains(seen.Query, "limit=5") {
		t.Errorf("query = %q", seen.Query)
	}
	if len(got) != 1 || got[0].SellPrice != 0.002 || got[0].ServerName != "peer-a" {
		t.Fatalf("resources = %+v", got)
	}
	if got[0].ResourceID != 476 {
		t.Errorf("ResourceID = %d, want 476 — without it a hit cannot be passed to CallAPI", got[0].ResourceID)
	}
}

// ListNetworkAPIs must also surface the handle, for the same reason.
func TestContractListFederationResourcesCarriesResourceID(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": [{"resource_id":64,"name":"Summarize Text","path":"/summarize-text","method":"POST",
	    "sell_price":0.002875,"price_unit":"call","currency":"USDC","network":"base"}],
	  "total": 2658, "page": 1, "page_size": 1
	}`
	c, _ := newStub(t, 200, body)

	got, total, err := c.ListNetworkAPIs(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("ListFederationResources: %v", err)
	}
	if total != 2658 {
		t.Errorf("total = %d, want 2658", total)
	}
	if len(got) != 1 || got[0].ResourceID != 64 {
		t.Fatalf("resources = %+v, want ResourceID 64", got)
	}
}

// controller/aip/federation_marketplace.go FederationMarketplaceCatalogue —
// {success, data:{items,total,page,page_size,categories}}, one nesting level
// deeper than the other federation listings.
func TestContractListAPIs(t *testing.T) {
	const body = `{
	  "success": true,
	  "data": {
	    "items": [{"service_id":"federation/64","name":"Summarize Text","category":"ai tools",
	      "price_unit":"call","display_price":0.002875,"resource_id":64,"server_name":"",
	      "method":"POST","description":"Summarize long text.","tags":"AI,Text","source":"federation"}],
	    "total": 2720, "page": 1, "page_size": 1,
	    "categories": [{"category":"ai tools","count":312}]
	  }
	}`
	c, seen := newStub(t, 200, body)

	page, err := c.ListAPIs(context.Background(), jarvisclaw.CatalogueParams{
		Page: 1, PageSize: 1, Category: "ai tools", Keyword: "summar",
	})
	if err != nil {
		t.Fatalf("ListAPIs: %v", err)
	}
	if seen.Path != "/api/marketplace/apis" {
		t.Errorf("path = %q", seen.Path)
	}
	// The keyword parameter is named q, not keyword or search.
	if !strings.Contains(seen.Query, "q=summar") {
		t.Errorf("query = %q, want q=summar", seen.Query)
	}
	if page.Total != 2720 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].ResourceID != 64 || page.Items[0].ServiceID != "federation/64" {
		t.Errorf("item = %+v", page.Items[0])
	}
	if page.Items[0].DisplayPrice != 0.002875 {
		t.Errorf("DisplayPrice = %v, want 0.002875", page.Items[0].DisplayPrice)
	}
	if len(page.Categories) != 1 || page.Categories[0].Count != 312 {
		t.Errorf("categories = %+v", page.Categories)
	}
}

// CallAPI must send resource_id (not id) and omit payload entirely when nil,
// since the execute path only forwards a body when one is present.
func TestContractCallAPIBody(t *testing.T) {
	t.Run("with payload", func(t *testing.T) {
		c, seen := newStub(t, 200, `{"success":true,"status_code":200}`)
		if _, err := c.CallAPI(context.Background(), 476, map[string]any{"url": "x"}); err != nil {
			t.Fatalf("CallAPI: %v", err)
		}
		if seen.Path != "/v1/federation/execute" {
			t.Errorf("path = %q", seen.Path)
		}
		if !strings.Contains(seen.Body, `"resource_id":476`) {
			t.Errorf("body = %q, want resource_id:476", seen.Body)
		}
		if !strings.Contains(seen.Body, `"url":"x"`) {
			t.Errorf("body = %q, want the payload forwarded", seen.Body)
		}
	})

	t.Run("nil payload sends no payload key", func(t *testing.T) {
		c, seen := newStub(t, 200, `{"success":true}`)
		if _, err := c.CallAPI(context.Background(), 476, nil); err != nil {
			t.Fatalf("CallAPI: %v", err)
		}
		if strings.Contains(seen.Body, "payload") {
			t.Errorf("body = %q, want no payload key for a nil payload", seen.Body)
		}
	})

	t.Run("rejects non-positive id", func(t *testing.T) {
		c, _ := newStub(t, 200, `{}`)
		for _, id := range []int{0, -1} {
			if _, err := c.CallAPI(context.Background(), id, nil); err == nil {
				t.Errorf("CallAPI(%d): expected an error", id)
			}
		}
	})
}

// InvokeAPI targets the marketplace URL shape and returns the upstream body
// unwrapped, so nothing here should look for an envelope.
func TestContractInvokeAPI(t *testing.T) {
	c, seen := newStub(t, 200, `{"decoded":["https://example.com"]}`)

	raw, err := c.InvokeAPI(context.Background(), 476, map[string]any{"url": "x"})
	if err != nil {
		t.Fatalf("InvokeAPI: %v", err)
	}
	if seen.Path != "/v1/marketplace/api/476" {
		t.Errorf("path = %q, want /v1/marketplace/api/476", seen.Path)
	}
	if seen.Method != "POST" {
		t.Errorf("method = %q, want POST", seen.Method)
	}
	if !strings.Contains(string(raw), "example.com") {
		t.Errorf("raw = %q, want the upstream body verbatim", raw)
	}

	if _, err := c.InvokeAPI(context.Background(), 0, nil); err == nil {
		t.Error("InvokeAPI(0): expected an error")
	}
}

// SSE framing: "data:" without a space, multi-line data, comments, and a final
// event with no trailing blank line.
func TestContractSSEFraming(t *testing.T) {
	stream := strings.Join([]string{
		": keep-alive",
		"event: metadata",
		`data: {"provider":"p1"}`,
		"",
		"event:chunk",
		`data:{"a":1}`,
		`data:{"b":2}`,
		"",
		"event: done",
		"data: {}",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer srv.Close()

	c, err := jarvisclaw.NewClient(jarvisclaw.WithAPIKey("sk-test"), jarvisclaw.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Subscribe(context.Background(), jarvisclaw.SubscribeRequest{
		Intent:  "chat_completion",
		Payload: map[string]any{"messages": []map[string]string{{"role": "user", "content": "hi"}}},
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer s.Close()

	var events []jarvisclaw.SSEEvent
	for {
		ev, err := s.Next()
		if err != nil {
			break
		}
		events = append(events, *ev)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(events), events)
	}
	if events[0].Event != "metadata" || !strings.Contains(events[0].Data, "p1") {
		t.Errorf("event 0 = %+v", events[0])
	}
	// "event:chunk" with no space must parse, and both data lines must be kept.
	if events[1].Event != "chunk" {
		t.Errorf("event 1 name = %q, want chunk (no-space form)", events[1].Event)
	}
	if events[1].Data != "{\"a\":1}\n{\"b\":2}" {
		t.Errorf("event 1 data = %q, want both lines joined", events[1].Data)
	}
	// The trailing event has no blank line after it and must still be delivered.
	if events[2].Event != "done" {
		t.Errorf("event 2 = %+v", events[2])
	}
}

func TestContractSubscribeRequiresPayload(t *testing.T) {
	c, _ := newStub(t, 200, `{}`)
	if _, err := c.Subscribe(context.Background(), jarvisclaw.SubscribeRequest{Intent: "chat_completion"}); err == nil {
		t.Error("expected error for missing payload")
	}
}

// Argument validation on the new endpoints, so a typo fails locally rather than
// as a confusing upstream 400.
func TestContractInputValidation(t *testing.T) {
	c, _ := newStub(t, 200, `{}`)
	ctx := context.Background()

	if _, err := c.Embeddings(ctx, jarvisclaw.EmbeddingRequest{Input: "x"}); err == nil {
		t.Error("Embeddings: expected error for missing model")
	}
	if _, err := c.Embeddings(ctx, jarvisclaw.EmbeddingRequest{Model: "m"}); err == nil {
		t.Error("Embeddings: expected error for missing input")
	}
	if _, err := c.Rerank(ctx, jarvisclaw.RerankRequest{Model: "m", Query: "q"}); err == nil {
		t.Error("Rerank: expected error for missing documents")
	}
	if _, err := c.Moderate(ctx, "m", nil); err == nil {
		t.Error("Moderate: expected error for nil input")
	}
	if err := c.AddNetworkPeer(ctx, ""); err == nil {
		t.Error("AddFederationPeer: expected error for empty domain")
	}
	if err := c.Unsubscribe(ctx, ""); err == nil {
		t.Error("Unsubscribe: expected error for empty id")
	}
	if _, err := c.CallUserAPI(ctx, "GET", "", "x", nil); err == nil {
		t.Error("CallUserAPI: expected error for empty slug")
	}
}

// Embeddings / rerank happy paths.
func TestContractEmbeddings(t *testing.T) {
	const body = `{"object":"list","model":"text-embedding-3-small",
	  "data":[{"object":"embedding","index":0,"embedding":[0.1,-0.2,0.3]}],
	  "usage":{"prompt_tokens":3,"total_tokens":3}}`
	c, seen := newStub(t, 200, body)

	vec, err := c.Embed(context.Background(), "text-embedding-3-small", "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if seen.Path != "/v1/embeddings" {
		t.Errorf("path = %q", seen.Path)
	}
	if len(vec) != 3 || vec[1] != -0.2 {
		t.Errorf("vector = %v", vec)
	}
}

func TestContractRerank(t *testing.T) {
	const body = `{"results":[{"index":2,"relevance_score":0.87},{"index":0,"relevance_score":0.41}],
	  "model":"rerank-v1"}`
	c, seen := newStub(t, 200, body)

	res, err := c.RerankTexts(context.Background(), "rerank-v1", "cats", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("RerankTexts: %v", err)
	}
	if seen.Path != "/v1/rerank" {
		t.Errorf("path = %q", seen.Path)
	}
	if len(res) != 2 || res[0].Index != 2 || res[0].RelevanceScore != 0.87 {
		t.Errorf("results = %+v", res)
	}
}

// Error bodies must surface the server's message, not a generic string.
func TestContractErrorMessageExtraction(t *testing.T) {
	c, _ := newStub(t, 400, `{"error":{"message":"model not found","type":"invalid_request_error"}}`)
	_, err := c.Resolve(context.Background(), jarvisclaw.ResolveRequest{Intent: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("err = %v, want the server message", err)
	}
}

func TestContractUnauthorizedIsTyped(t *testing.T) {
	c, _ := newStub(t, 401, `{"message":"Invalid token"}`)
	_, err := c.WalletBalance(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Invalid token") {
		t.Errorf("err = %v", err)
	}
}

// ─── Regressions found by running the examples against the live gateway ──────

// newMarketplaceStub is the MarketplaceClient counterpart to newStub.
func newMarketplaceStub(t *testing.T, status int, body string) (*jarvisclaw.MarketplaceClient, *struct {
	Method string
	Path   string
	Query  string
}) {
	t.Helper()
	seen := &struct {
		Method string
		Path   string
		Query  string
	}{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Method = r.Method
		seen.Path = r.URL.Path
		seen.Query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	mc, err := jarvisclaw.NewMarketplaceClient(
		jarvisclaw.WithAPIKey("sk-test"),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("new marketplace client: %v", err)
	}
	return mc, seen
}

// Call joined service and path with no separator, so a path without a leading
// slash produced "/v1/marketplace/surfexchange/price" and the gateway answered
// 404 "service 'surfexchange' not found".
func TestContractMarketplaceCallJoinsPath(t *testing.T) {
	for _, tc := range []struct {
		name, service, path, want string
	}{
		{"no leading slash", "surf", "exchange/price", "/v1/marketplace/surf/exchange/price"},
		{"leading slash", "defi", "/protocols", "/v1/marketplace/defi/protocols"},
		{"trailing slash on service", "surf/", "exchange/price", "/v1/marketplace/surf/exchange/price"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mc, seen := newMarketplaceStub(t, 200, `{"ok":true}`)
			if _, err := mc.Call(context.Background(), tc.service, tc.path); err != nil {
				t.Fatalf("call: %v", err)
			}
			if seen.Path != tc.want {
				t.Errorf("path = %q, want %q", seen.Path, tc.want)
			}
		})
	}
}

func TestContractMarketplaceCallForwardsParams(t *testing.T) {
	mc, seen := newMarketplaceStub(t, 200, `{"ok":true}`)

	_, err := mc.Call(context.Background(), "surf", "exchange/price",
		jarvisclaw.WithParams(map[string]string{"symbol": "BTC"}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if seen.Query != "symbol=BTC" {
		t.Errorf("query = %q, want symbol=BTC", seen.Query)
	}
}

// Chat had no way to set max_tokens, and temperature used a plain float64 with a
// non-zero check — so WithTemperature(0), meaning deterministic output, was
// indistinguishable from unset and silently dropped.
func TestContractChatSendsExplicitZeroAndMaxTokens(t *testing.T) {
	seen := &struct{ Body string }{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen.Body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	t.Cleanup(srv.Close)

	cc, err := jarvisclaw.NewChatClient(
		jarvisclaw.WithAPIKey("sk-test"),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("new chat client: %v", err)
	}

	_, err = cc.Complete(context.Background(), "hi",
		jarvisclaw.WithChatModel("openai/gpt-4o-mini"),
		jarvisclaw.WithTemperature(0),
		jarvisclaw.WithMaxTokens(5),
		jarvisclaw.WithSeed(7),
	)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(seen.Body), &body); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	temp, ok := body["temperature"]
	if !ok {
		t.Error("temperature absent: an explicit 0 must still be sent")
	} else if temp.(float64) != 0 {
		t.Errorf("temperature = %v, want 0", temp)
	}
	if body["max_tokens"] != float64(5) {
		t.Errorf("max_tokens = %v, want 5", body["max_tokens"])
	}
	if body["seed"] != float64(7) {
		t.Errorf("seed = %v, want 7", body["seed"])
	}
}


// The federation→network rename kept the old Go symbols as deprecated aliases.
// These assert the aliases still reach the gateway on the unchanged wire paths:
// the rename was a Go-level rename, not an HTTP contract change.
func TestContractDeprecatedAliasesHitUnchangedPaths(t *testing.T) {
	t.Run("FederationPeers", func(t *testing.T) {
		c, seen := newStub(t, 200, `{"success":true,"data":[]}`)
		if _, err := c.FederationPeers(context.Background()); err != nil {
			t.Fatalf("FederationPeers: %v", err)
		}
		if seen.Path != "/v1/aip/federation/peers" {
			t.Errorf("path = %q, want unchanged federation path", seen.Path)
		}
	})

	t.Run("SearchFederation", func(t *testing.T) {
		c, seen := newStub(t, 200, `{"success":true,"data":[]}`)
		if _, err := c.SearchFederation(context.Background(), jarvisclaw.FederationSearchParams{Query: "x"}); err != nil {
			t.Fatalf("SearchFederation: %v", err)
		}
		if seen.Path != "/v1/federation/search" {
			t.Errorf("path = %q", seen.Path)
		}
	})

	t.Run("ListFederationResources", func(t *testing.T) {
		c, seen := newStub(t, 200, `{"success":true,"data":[],"total":0}`)
		if _, _, err := c.ListFederationResources(context.Background(), 1, 10); err != nil {
			t.Fatalf("ListFederationResources: %v", err)
		}
		// Note: the server renamed this route to /v1/network/apis, but production
		// still only serves the federation spelling, so the SDK must not move yet.
		if seen.Path != "/v1/federation/resources" {
			t.Errorf("path = %q", seen.Path)
		}
	})

	t.Run("FederationHealth", func(t *testing.T) {
		c, seen := newStub(t, 200, `{"status":"ok"}`)
		if _, err := c.FederationHealth(context.Background()); err != nil {
			t.Fatalf("FederationHealth: %v", err)
		}
		if seen.Path != "/v1/federation/health" {
			t.Errorf("path = %q", seen.Path)
		}
	})
}

// The aliases must be type-identical to the new names, not merely convertible,
// so existing callers can pass them into new-named APIs interchangeably.
func TestDeprecatedAliasesAreTypeIdentical(t *testing.T) {
	var p jarvisclaw.FederationPeer
	var np jarvisclaw.NetworkPeer = p // compiles only if aliased, not a distinct type
	_ = np

	var r jarvisclaw.FederationResource
	var api jarvisclaw.NetworkAPI = r
	_ = api

	var s jarvisclaw.FederationServer
	var ns jarvisclaw.NetworkServer = s
	_ = ns
}
