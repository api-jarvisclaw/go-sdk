# JarvisClaw Go SDK

Go SDK for [JarvisClaw AI](https://jarvisclaw.ai) — intent-based AI routing with x402 USDC micropayments.

## Install

```bash
go get github.com/api-jarvisclaw/go-sdk/v2@latest
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"

    jc "github.com/api-jarvisclaw/go-sdk/v2"
)

func main() {
    ctx := context.Background()

    // x402 wallet mode (pay per request, no API key needed)
    client, _ := jc.NewClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

    // Intent resolution + execution in one call
    text, _ := client.Ask(ctx, "Explain quantum computing",
        jc.AskOptions{Budget: 0.01, Optimize: "cost"})
    fmt.Println(text)
}
```

## Authentication

Two modes — pick one:

```go
// x402 wallet (USDC on Base, no gas needed) — recommended
client, _ := jc.NewClient(jc.WithPrivateKey("0x<hex-private-key>"))

// API key (bearer token)
client, _ := jc.NewClient(jc.WithAPIKey("sk-your-key"))

// Auto-detect from environment: JARVISCLAW_WALLET_KEY or JARVISCLAW_API_KEY
client, _ := jc.NewClient()
```

Environment variables:
- `JARVISCLAW_WALLET_KEY` — EVM private key (hex, with or without 0x prefix)
- `JARVISCLAW_API_KEY` — API key
- `JARVISCLAW_BASE_URL` — Custom base URL

---

## Unified Client (AIP)

The unified `Client` handles intent resolution, execution, streaming, wallet, and federation.

### Resolve (Find Best Provider)

```go
resp, _ := client.Resolve(ctx, jc.ResolveRequest{
    Intent:      "chat_completion",
    Constraints: jc.Constraints{MaxPriceUSD: jc.Float64Ptr(0.01)},
    Preferences: jc.Preferences{OptimizeFor: "cost"},
})
fmt.Printf("Best: %s at $%.6f/req\n", resp.Matches[0].Model, resp.Matches[0].EstimatedPriceUSD)
```

### Execute (Resolve + Call in One Step)

```go
// One-shot: resolve the cheapest provider within budget, then chat.
text, _ := client.Ask(ctx, "Write a haiku about Go",
    jc.AskOptions{Budget: 0.02, Optimize: "quality"})
fmt.Println(text)

// Full control: forward an arbitrary payload to the resolved provider.
raw, _ := client.Execute(ctx, jc.ExecuteRequest{
    Intent:  "chat_completion",
    Payload: map[string]any{"messages": []map[string]string{{"role": "user", "content": "Hi"}}},
})

// With a hard spend cap and settlement details in the response.
result, _ := client.ExecuteBudget(ctx, jc.ExecuteBudgetRequest{
    Intent:  "chat_completion",
    Budget:  jc.Budget{MaxTotalUSD: 0.01},
    Payload: map[string]any{"messages": []map[string]string{{"role": "user", "content": "Hi"}}},
})
fmt.Println(result.Status, *result.ActualCostUSD)
```

### Streaming

Two options. `ChatStream` streams text deltas from a model you name; `Subscribe`
resolves the provider first and gives you the raw SSE events.

```go
// Text deltas, model chosen by you (or "auto").
chunks, _ := client.ChatStream(ctx, "auto", "Count to 10")
for chunk := range chunks {
    fmt.Print(chunk)
}

// Intent-routed SSE. The first event is "metadata", the last is "done".
stream, _ := client.Subscribe(ctx, jc.SubscribeRequest{
    Intent:      "chat_completion",
    Payload:     map[string]any{"messages": []map[string]string{{"role": "user", "content": "Count to 10"}}},
    OptimizeFor: "speed",
})
defer stream.Close()
for {
    ev, err := stream.Next()
    if err != nil {
        break // io.EOF at end of stream
    }
    fmt.Printf("[%s] %s\n", ev.Event, ev.Data)
}
```

### Wallet

```go
// Spendable balance in USD. x402 mode reads USDC on Base directly from chain.
balance, _ := client.GetBalance(ctx)
fmt.Printf("$%.2f USDC\n", balance)

// Per-chain detail
wb, _ := client.WalletBalance(ctx)
fmt.Println(wb.Wallets.Base.USDC, wb.Wallets.Solana.USDC, wb.TotalUSD())

pools, _ := client.WalletPools(ctx)
fmt.Println(pools.Allocation.Operations, pools.PoolBalances.Operations)

// Limits: PUT replaces the whole record, so change one field via
// UpdateWalletLimit rather than SetWalletLimits — otherwise the fields you
// leave unset are stored as zero.
client.UpdateWalletLimit(ctx, func(l *jc.WalletLimits) {
    l.DailyMaxUSD = 30.0
})

history, _ := client.WalletHistory(ctx, 1, 20)
fmt.Println(history.Total)
```

### Prompt Coach

```go
result, _ := client.PromptCoach(ctx, jc.PromptCoachRequest{
    Prompt:  "write code that does the thing",
    Context: "technical blog for developers",
})
fmt.Println(result.OptimizedPrompt)
fmt.Println(result.Suggestions)
// Scores are integers on a 1-100 scale. There is no score-only endpoint;
// read ScoreBefore to grade a prompt as-is.
fmt.Printf("%d → %d\n", result.ScoreBefore, result.ScoreAfter)
```

### Analytics

```go
// Spend and settlement, aggregated. A non-admin caller always sees only their
// own data — the scope is enforced server-side, not by these parameters.
rows, _ := client.Spend(ctx, jc.AnalyticsParams{
    Period:  "30d",
    GroupBy: []string{"day", "model"},
})
for _, r := range rows {
    fmt.Printf("%s %s $%.4f (%d reqs)\n", r.Day, r.Model, r.TotalCostUSD, r.TotalReqs)
}

byModel, _ := client.CostByModel(ctx, jc.AnalyticsParams{Period: "7d"})
trend, _ := client.DailyTrend(ctx, jc.AnalyticsParams{Period: "30d"})
```

### Discovery

```go
// What this gateway and its peers can do.
d, _ := client.Discover(ctx, jc.DiscoverRequest{Intent: "web_search", MaxPrice: 0.02})
fmt.Println(d.Total, len(d.Federated))

// Free, unauthenticated variant.
d2, _ := client.DiscoverPublic(ctx, jc.DiscoverRequest{})

// Natural-language routing. Status may be "clarify", in which case ask
// Clarify.Question and retry with the same SessionID.
nr, _ := client.ResolveNatural(ctx, jc.NaturalResolveRequest{
    Query: "find me recent papers about MoE routing",
})
if nr.Status == "clarify" {
    fmt.Println(nr.Clarify.Question, nr.Clarify.Options)
}

stats, _ := client.NetworkStats(ctx)
fmt.Println(stats.TotalProviders, stats.IntentTypes)
```

### Embeddings, Rerank, Moderation

```go
vec, _ := client.Embed(ctx, "text-embedding-3-small", "hello world")

ranked, _ := client.RerankTexts(ctx, "rerank-v1", "cats",
    []string{"dogs are loyal", "cats are independent", "birds sing"})
fmt.Println(ranked[0].Index, ranked[0].RelevanceScore)

flags, _ := client.Moderate(ctx, "", "some text to classify")
```

### Community APIs (UAPI)

```go
apis, total, _ := client.ListUserAPIs(ctx, jc.UserAPIListParams{Category: "data", PageSize: 10})
fmt.Println(total, apis[0].Slug, apis[0].PricePerCall)

// Invoke one — the gateway pays the provider on your behalf.
body, _ := client.CallUserAPI(ctx, "POST", "weather", "forecast", map[string]any{"city": "Tokyo"})
fmt.Println(string(body))
```

### Federation

```go
// Public registry — no admin rights needed.
resources, _ := client.SearchFederation(ctx, jc.FederationSearchParams{Query: "price", Limit: 10})
servers, total, _ := client.ListFederationServers(ctx, 1, 20)

// Peer management requires a dashboard session or access token, not an API key.
peers, _ := client.FederationPeers(ctx)
client.AddFederationPeer(ctx, "peer.example.com")
client.RemoveFederationPeer(ctx, "peer.example.com") // by domain, not id
```

---

## Specialized Clients

For direct access to specific modalities:

### ChatClient

```go
chat, _ := jc.NewChatClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// Simple completion (model defaults to "auto" — smart-routed)
text, _ := chat.Complete(ctx, "Hello")

// Specify model
text, _ = chat.Complete(ctx, "Hello", jc.WithChatModel("openai/gpt-4o"))

// Full message array
resp, _ := chat.Completion(ctx, []jc.Message{
    {Role: "system", Content: "You are helpful."},
    {Role: "user", Content: "Hi"},
})
fmt.Println(resp.Content, resp.Model, resp.Usage)

// Streaming
sr, _ := chat.Stream(ctx, "Tell me a joke")
for chunk := range sr.Channel() {
    fmt.Print(chunk)
}
```

### ImageClient

```go
img, _ := jc.NewImageClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// Blocking. Fast models answer inline; slower ones return a job that Generate
// polls to completion for you.
result, _ := img.Generate(ctx, "A cat in space",
    jc.WithSize("1024x1024"),
    jc.WithImageModel("auto/image"),
)
fmt.Println(result.URL)

// Non-blocking
job, _ := img.Generate(ctx, "A cat in space", jc.WithImageWait(false))
status, _ := img.Status(ctx, job.ID)
fmt.Println(status.Done(), status.URL)
```

### VideoClient

```go
vid, _ := jc.NewVideoClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// Blocking — waits until video is ready
job, _ := vid.Generate(ctx, "A ball bouncing", jc.WithDuration(5))
fmt.Println(job.URL)

// Non-blocking
job, _ = vid.Generate(ctx, "Clouds moving", jc.WithWait(false))
status, _ := vid.Status(ctx, job.ID)
```

### AudioClient

```go
audio, _ := jc.NewAudioClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// Text-to-speech
resp, _ := audio.Speech(ctx, "Hello world", jc.WithVoice("alloy"))
os.WriteFile("output.mp3", resp.Data, 0644)

// Music generation
music, _ := audio.Music(ctx, "a calm lo-fi beat", jc.WithInstrumental(true))
fmt.Println(music.URL)

// Speech-to-text. Pass a seekable reader: with x402 the request is replayed
// after payment, so the audio has to be readable twice.
f, _ := os.Open("meeting.mp3")
defer f.Close()
tr, _ := audio.Transcribe(ctx, jc.TranscriptionRequest{
    Model:    "whisper-1",
    Filename: "meeting.mp3",
    Audio:    f,
    Language: "en",
})
fmt.Println(tr.Text)
```

### SearchClient

```go
sc, _ := jc.NewSearchClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

results, _ := sc.Query(ctx, "latest AI news", jc.WithNumResults(5))
similar, _ := sc.FindSimilar(ctx, "https://arxiv.org/abs/2301.00001", jc.WithNumResults(5))
contents, _ := sc.Contents(ctx, []string{"https://example.com/article"})
answer, _ := sc.Answer(ctx, "What are the latest advances in AI?")
```

---

## Marketplace (80+ Endpoints)

Access crypto data, blockchain RPC, DeFi, prediction markets, and web search via x402 micropayments.

```go
mp, _ := jc.NewMarketplaceClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))
```

### Crypto Data (Surf)

```go
// Exchange prices (16 CEXes)
price, _ := mp.Call(ctx, "surf", "/exchange/price",
    jc.WithParams(map[string]string{"pair": "BTC-USDT"}))

// Market overview
rankings, _ := mp.Call(ctx, "surf", "/market/ranking",
    jc.WithParams(map[string]string{"limit": "10"}))
fearGreed, _ := mp.Call(ctx, "surf", "/market/fear-greed")

// Social / CT intelligence
tweets, _ := mp.Call(ctx, "surf", "/social/user/posts",
    jc.WithParams(map[string]string{"username": "VitalikButerin", "limit": "5"}))

// Wallet intelligence (100M+ labeled wallets)
wallet, _ := mp.Call(ctx, "surf", "/wallet/detail",
    jc.WithParams(map[string]string{"address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"}))

// Token analytics
holders, _ := mp.Call(ctx, "surf", "/token/holders",
    jc.WithParams(map[string]string{"symbol": "UNI", "limit": "10"}))

// On-chain SQL (80+ ClickHouse tables)
result, _ := mp.Post(ctx, "surf", "/onchain/sql", map[string]any{
    "sql": "SELECT from_address, SUM(value/1e18) as eth FROM ethereum.transactions WHERE block_time > now() - interval '1 hour' GROUP BY from_address ORDER BY eth DESC LIMIT 5",
})
```

### Prediction Markets

```go
markets, _ := mp.Call(ctx, "prediction", "/polymarket/markets",
    jc.WithParams(map[string]string{"limit": "5", "category": "politics"}))
kalshi, _ := mp.Call(ctx, "prediction", "/kalshi/markets",
    jc.WithParams(map[string]string{"limit": "5"}))
```

### DEX Trading (0x)

```go
quote, _ := mp.Call(ctx, "dex", "/price",
    jc.WithParams(map[string]string{
        "sellToken":  "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",
        "buyToken":   "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
        "sellAmount": "100000000000000000",
        "chainId":    "8453",
    }))
```

### Blockchain RPC (40+ Chains)

```go
block, _ := mp.RPCCall(ctx, "eth", "eth_blockNumber", []any{})
slot, _ := mp.RPCCall(ctx, "sol", "getSlot", []any{})

// Batch RPC
batch, _ := mp.RPCBatch(ctx, "ethereum", []jc.RPCRequest{
    {Method: "eth_blockNumber", Params: []any{}},
    {Method: "eth_gasPrice", Params: []any{}},
})
```

### DeFi (DefiLlama)

```go
protocols, _ := mp.DefiProtocols(ctx)
aave, _ := mp.DefiProtocol(ctx, "aave-v3")
yields, _ := mp.DefiYields(ctx)
```

> All marketplace endpoints use x402 micropayments (USDC on Base). Standard calls: $0.0075, premium SQL: $0.02.

---

## OpenAI / Anthropic SDK Compatibility

Use official Go SDKs directly against JarvisClaw by changing the base URL:

### OpenAI Go SDK

```go
import (
    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
)

client := openai.NewClient(
    option.WithAPIKey("sk-your-jarvisclaw-key"),
    option.WithBaseURL("https://api.jarvisclaw.ai/v1"),
)
resp, _ := client.Responses.New(ctx, openai.ResponseNewParams{
    Model: "anthropic/claude-sonnet-4-20250514",
    Input: openai.ResponseNewParamsInputUnionString("Explain quantum computing"),
})
fmt.Println(resp.OutputText)
```

### Anthropic Go SDK

```go
import (
    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

client := anthropic.NewClient(
    option.WithAPIKey("sk-your-jarvisclaw-key"),
    option.WithBaseURL("https://api.jarvisclaw.ai"),
)
message, _ := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     "claude-sonnet-4-20250514",
    MaxTokens: 1024,
    Messages: []anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Explain quantum computing")),
    },
})
fmt.Println(message.Content[0].Text)
```

> **When to use which?**
> - `go-sdk` (this package) — x402 wallet payments, intent routing, budget control, marketplace
> - `openai-go` — Responses API features, drop-in for existing OpenAI code
> - `anthropic-sdk-go` — Claude-native features (prompt caching, extended thinking)

---

## Error Handling

```go
text, err := client.Ask(ctx, "Hello", jc.AskOptions{})
if err != nil {
    switch e := err.(type) {
    case *jc.AuthenticationError:
        // 401 — bad API key or wallet key
    case *jc.RateLimitError:
        // 429 — auto-retried up to 3x
    case *jc.InsufficientBalanceError:
        // 402 — insufficient USDC
    case *jc.PaymentError:
        // x402 signing/settlement failure
    case *jc.APIError:
        fmt.Println(e.StatusCode, e.Message)
    }
}
```

## Configuration

```go
jc.NewClient(
    jc.WithPrivateKey("0x..."),          // x402 wallet auth
    jc.WithAPIKey("sk-..."),             // API key auth
    jc.WithBaseURL("https://..."),       // Custom endpoint
    jc.WithTimeout(120 * time.Second),   // HTTP timeout
    jc.WithNetwork("eip155:8453"),       // Payment network (Base)
)
```

## Migration to v2

v2 fixes a build failure and removes methods that called gateway routes which no
longer exist.

**Import path.** Go requires a new module path for v2, so update your imports:

```go
// before
import jc "github.com/api-jarvisclaw/go-sdk"

// after
import jc "github.com/api-jarvisclaw/go-sdk/v2"
```

```bash
go get github.com/api-jarvisclaw/go-sdk/v2@latest
```

`go get -u` will not carry you across this boundary — that is deliberate, since
everything below is breaking.

**Build.** v1.2 and v1.3 did not compile at all: `go-ethereum v1.14.12` pulled in
a `go-kzg-4844` incompatible with the `gnark-crypto` version MVS selected, so
`go build` failed for the package and for anything importing it. Fixed by moving
to `go-ethereum v1.17.0`, which raises the minimum Go to 1.24.

**Analytics.** `/v1/aip/analytics/*` was consolidated into
`/api/analytics/aggregate`, so these were 404ing:

| Removed | Use instead |
|---|---|
| `CostSummary` | `Spend` |
| `CostTrend` | `DailyTrend` |
| `ModelBreakdown` | `CostByModel` |
| `ROI` | `Spend` and compute from `TotalQuota` / `TotalCostUSD` |
| `BudgetStatus` | `Spend` and compare against your own limits |

`AnalyticsParams` changed with them: `Period` and `GroupBy` replace
`Start`/`End`/`TopN`/`Scope`. Scope is enforced from your credentials.

**Wallet.** `WalletBalance` matched a response shape the gateway had already
dropped, so it decoded to an all-zero struct without erroring. `Quota`,
`QuotaUSD`, `HDWallet` and `Subscription` are gone; read `BalanceUSD`,
`Wallets.Base` / `Wallets.Solana`, or `TotalUSD()`.

`WalletPools` fields are now structs (`PoolAllocation`, `PoolBalances`) instead of
maps. `SetWalletLimits` replaces the whole record — use `UpdateWalletLimit` for
single-field changes.

**Balance.** `GetBalance` in API-key mode read `/api/user/self`, which requires a
dashboard session an API key cannot provide. It now uses
`/v1/dashboard/billing/subscription`.

**Federation.** `DeleteFederationPeer(id)` became `RemoveFederationPeer(domain)` —
the server identifies peers by domain in the body, not by id in the path.
`FederationCrawl` no longer takes a `CrawlRequest`; the server crawls every
registered peer. `FederationPeers` returns `[]FederationPeer` directly, with
camelCase-tagged fields matching what the server actually sends.

**Prompt coach.** `PromptScore` and its types are gone: `/v1/prompt-coach/score`
never existed. `PromptCoachResponse` now matches the real payload — integer
`ScoreBefore`/`ScoreAfter` on a 1-100 scale, plus `Explanation` and `ModelUsed`.

**Prediction.** `Prediction` takes a `body` argument and prefixes `/v1/prediction`
for you. The route is singular; the old doc comment said `/v1/predictions/`.

**Discover.** `DiscoverRequest` and `DiscoverResponse` were both wrong. The
request takes `Intent`, `Features` and `MaxPrice`; the response carries `Intents`,
`Providers`, `Federated` and `Total`.

**ListProviders** returns `[]Provider` (registry entries) rather than `[]Match`,
which is the ranked-resolution type.

**New.** `Embeddings`, `Embed`, `Rerank`, `RerankTexts`, `Moderate`, `Responses`,
`AudioClient.Transcribe`, `ImageClient.Status` with async polling,
`ResolveNatural`, `NetworkStats`, `DiscoverPublic`, `SearchFederation`,
`ListFederationServers`, `ListFederationResources`, `FederationExecute`,
`ListUserAPIs`, `GetUserAPI`, `CallUserAPI`, and the `Float64Ptr`/`IntPtr`/
`BoolPtr`/`StringPtr` helpers.

## Requirements

- Go >= 1.24 (set by `go.mod`; the `go-ethereum` dependency used for x402
  EIP-712 signing requires it)

## Links

- [AIP Protocol Spec](https://docs.jarvisclaw.ai/aip)
- [SDK Reference](https://docs.jarvisclaw.ai/sdk)
- [Telegram](https://t.me/JarvisClawai)

## License

MIT
