# JarvisClaw Go SDK

Go SDK for the JarvisClaw AI API with x402 USDC micropayment support.

## Install

```bash
go get github.com/api-jarvisclaw/go-sdk@latest
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"

    jc "github.com/api-jarvisclaw/go-sdk"
)

func main() {
    ctx := context.Background()

    // Chat (smart-routed — model defaults to "auto")
    chat, _ := jc.NewChatClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))
    text, _ := chat.Complete(ctx, "What is quantum computing?")
    fmt.Println(text)
}
```

## Authentication

Two modes — pick one:

```go
// API Key (bearer token)
client, _ := jc.NewChatClient(jc.WithAPIKey("sk-your-key"))

// x402 wallet (USDC on Base, no gas needed)
client, _ := jc.NewChatClient(jc.WithPrivateKey("0x<hex-private-key>"))

// Auto-detect from environment: JARVISCLAW_API_KEY or JARVISCLAW_WALLET_KEY
client, _ := jc.NewChatClient()
```

---

## Agent Economy (AIP + Treasury) ⚡ NEW

Resolve optimal providers, manage wallet, and execute — all from the same client.

```go
c, _ := jc.NewClient(jc.WithAPIKey("sk-YOUR-KEY"))
ctx := context.Background()

// One line: find cheapest model + call it
text, _ := c.Ask(ctx, "Explain quantum computing",
    jc.AskOptions{Budget: 0.01, Optimize: "cost"})
fmt.Println(text)
```

### Intent Resolution (Free)

```go
resp, _ := c.Resolve(ctx, jc.ResolveRequest{
    Intent:      "chat_completion",
    Constraints: jc.Constraints{MaxPriceUSD: jc.Float64Ptr(0.01)},
    Preferences: jc.Preferences{OptimizeFor: "cost"},
})
fmt.Printf("Best: %s at $%.6f\n", resp.Matches[0].Model, resp.Matches[0].EstimatedPriceUSD)
```

### Wallet API

```go
bal, _ := c.WalletBalance(ctx)
fmt.Printf("Total: %s USD\n", bal.TotalUSD)

pools, _ := c.WalletPools(ctx)
limits, _ := c.WalletLimits(ctx)
c.SetWalletLimits(ctx, jc.WalletLimits{DailyMaxUSD: 30.0})
```

### Advanced: Go Treasury SDK

For full financial autonomy (multi-pool, rules, circuit breakers):

```bash
go get github.com/api-jarvisclaw/agent-treasury-go
```

## Clients

### ChatClient

```go
chat, _ := jc.NewChatClient(jc.WithAPIKey("sk-..."))

// Simple completion
text, _ := chat.Complete(ctx, "Hello", jc.WithChatModel("openai/gpt-4o"))

// Full message array
resp, _ := chat.Completion(ctx, []jc.Message{
    {Role: "system", Content: "You are helpful."},
    {Role: "user", Content: "Hi"},
})
fmt.Println(resp.Content, resp.Model, resp.Usage)

// Streaming
sr, _ := chat.Stream(ctx, "Tell me a joke", jc.WithSystem("Be funny"))
for chunk := range sr.Channel() {
    fmt.Print(chunk)
}
```

### ImageClient

```go
img, _ := jc.NewImageClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

result, _ := img.Generate(ctx, "A cat in space",
    jc.WithSize("1024x1024"),
    jc.WithImageModel("auto/image"),
)
fmt.Println(result.URL)
```

### VideoClient

```go
vid, _ := jc.NewVideoClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// Blocking — waits until video is ready (default)
job, _ := vid.Generate(ctx, "A ball bouncing", jc.WithDuration(5))
fmt.Println(job.URL)

// Non-blocking — returns immediately after submission
job, _ = vid.Generate(ctx, "Clouds moving", jc.WithWait(false))
fmt.Println(job.ID, job.Status)

// Poll status manually
status, _ := vid.Status(ctx, job.ID)
```

### AudioClient

```go
audio, _ := jc.NewAudioClient(jc.WithAPIKey("sk-..."))

// Text-to-speech (returns raw audio bytes)
resp, _ := audio.Speech(ctx, "Hello world", jc.WithVoice("alloy"))
os.WriteFile("output.mp3", resp.Data, 0644)

// Music generation
music, _ := audio.Music(ctx, "a calm lo-fi beat", jc.WithInstrumental(true))
fmt.Println(music.URL)
```

### SearchClient

```go
sc, _ := jc.NewSearchClient(jc.WithAPIKey("sk-..."))
results, _ := sc.Query(ctx, "latest AI news", jc.WithNumResults(5))
for _, r := range results {
    fmt.Printf("%s — %s\n", r.Title, r.URL)
}
```

### MarketplaceClient (83+ Endpoints)

Access crypto data, blockchain RPC, DeFi, prediction markets, web search, and more. All marketplace endpoints require x402 private key authentication (no API key support).

```go
mp, _ := jc.NewMarketplaceClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// ─── Crypto Data (Surf) ───

// Exchange data (16 CEXes supported)
price, _ := mp.Call(ctx, "surf", "/exchange/price",
    jc.WithParams(map[string]string{"pair": "BTC-USDT"}))
fmt.Printf("BTC: $%v\n", price["price"])

klines, _ := mp.Call(ctx, "surf", "/exchange/klines",
    jc.WithParams(map[string]string{"pair": "ETH-USDT", "interval": "1h", "limit": "24"}))

funding, _ := mp.Call(ctx, "surf", "/exchange/funding-history",
    jc.WithParams(map[string]string{"pair": "BTC-USDT-PERP"}))

// Market overview
rankings, _ := mp.Call(ctx, "surf", "/market/ranking",
    jc.WithParams(map[string]string{"limit": "10"}))

fearGreed, _ := mp.Call(ctx, "surf", "/market/fear-greed", )

etf, _ := mp.Call(ctx, "surf", "/market/etf", )

indicators, _ := mp.Call(ctx, "surf", "/market/price-indicator",
    jc.WithParams(map[string]string{"symbol": "BTC", "indicator": "rsi"}))

// Social / CT intelligence
social, _ := mp.Call(ctx, "surf", "/social/ranking",
    jc.WithParams(map[string]string{"limit": "10"}))

tweets, _ := mp.Call(ctx, "surf", "/social/user/posts",
    jc.WithParams(map[string]string{"username": "VitalikButerin", "limit": "5"}))

// Wallet intelligence (100M+ labeled wallets)
wallet, _ := mp.Call(ctx, "surf", "/wallet/detail",
    jc.WithParams(map[string]string{"address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"}))

netWorth, _ := mp.Call(ctx, "surf", "/wallet/net-worth",
    jc.WithParams(map[string]string{"address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"}))

// Token analytics
holders, _ := mp.Call(ctx, "surf", "/token/holders",
    jc.WithParams(map[string]string{"symbol": "UNI", "limit": "10"}))

tokenomics, _ := mp.Call(ctx, "surf", "/token/tokenomics",
    jc.WithParams(map[string]string{"symbol": "ARB"}))

// News
news, _ := mp.Call(ctx, "surf", "/news/feed",
    jc.WithParams(map[string]string{"limit": "5"}))

// On-chain SQL (80+ ClickHouse tables)
result, _ := mp.Post(ctx, "surf", "/onchain/sql", map[string]any{
    "sql": "SELECT from_address, SUM(value/1e18) as eth FROM ethereum.transactions WHERE block_time > now() - interval '1 hour' GROUP BY from_address ORDER BY eth DESC LIMIT 5",
})

// VC Fund intelligence
funds, _ := mp.Call(ctx, "surf", "/fund/ranking",
    jc.WithParams(map[string]string{"limit": "10"}))

// Unified search
results, _ := mp.Call(ctx, "surf", "/search/web",
    jc.WithParams(map[string]string{"q": "bitcoin etf approval"}))

// ─── Prediction Markets ───
markets, _ := mp.Call(ctx, "prediction", "/polymarket/markets",
    jc.WithParams(map[string]string{"limit": "5", "category": "politics"}))

kalshi, _ := mp.Call(ctx, "prediction", "/kalshi/markets",
    jc.WithParams(map[string]string{"limit": "5"}))

search, _ := mp.Call(ctx, "prediction", "/markets/search",
    jc.WithParams(map[string]string{"q": "bitcoin 2026", "limit": "5"}))

// ─── DEX Trading (0x) ───
quote, _ := mp.Call(ctx, "dex", "/price",
    jc.WithParams(map[string]string{
        "sellToken":  "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",
        "buyToken":   "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
        "sellAmount": "100000000000000000",
        "chainId":    "8453",
    }))

// ─── Web Search (Exa) ───
exa, _ := mp.Post(ctx, "exa", "/search", map[string]any{
    "query": "latest AI research papers", "numResults": 5,
})

// Or use SearchClient convenience methods:
sc, _ := jc.NewSearchClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))
similar, _ := sc.FindSimilar(ctx, "https://arxiv.org/abs/2301.00001", jc.WithNumResults(5))
contents, _ := sc.Contents(ctx, []string{"https://example.com/article"})
answer, _ := sc.Answer(ctx, "What are the latest advances in AI?")

// ─── Blockchain RPC (40+ chains) ───
block, _ := mp.RPCCall(ctx, "eth", "eth_blockNumber", []any{})
slot, _ := mp.RPCCall(ctx, "sol", "getSlot", []any{})
baseBlock, _ := mp.RPCCall(ctx, "base", "eth_blockNumber", []any{})

// Batch RPC
batch, _ := mp.RPCBatch(ctx, "ethereum", []jc.RPCRequest{
    {Method: "eth_blockNumber", Params: []any{}},
    {Method: "eth_gasPrice", Params: []any{}},
})

// ─── DeFi (DefiLlama) ───
protocols, _ := mp.DefiProtocols(ctx)
aave, _ := mp.DefiProtocol(ctx, "aave-v3")
yields, _ := mp.DefiYields(ctx)
```

> **Note:** All marketplace endpoints use x402 micropayments (USDC on Base). Standard calls cost $0.0075, premium SQL costs $0.02. No API key authentication is supported for marketplace services.

## Balance

```go
client, _ := jc.NewClient(jc.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")))

// x402 mode: returns on-chain USDC balance
// API key mode: returns server quota in USD
balance, _ := client.GetBalance(ctx)
fmt.Printf("$%.2f\n", balance)

// Wallet address (x402 mode only)
fmt.Println(client.Address()) // "0x..."
```

## Error Handling

```go
text, err := chat.Complete(ctx, "Hello")
if err != nil {
    switch e := err.(type) {
    case *jc.AuthenticationError:
        // 401 — bad API key
    case *jc.RateLimitError:
        // 429 — too many requests (auto-retried up to 3x)
    case *jc.InsufficientBalanceError:
        // 402 — no wallet key for x402, or insufficient USDC
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
    jc.WithAPIKey("sk-..."),             // API key auth
    jc.WithPrivateKey("0x..."),          // x402 wallet auth
    jc.WithBaseURL("https://..."),       // Custom endpoint
    jc.WithTimeout(120 * time.Second),   // HTTP timeout
    jc.WithNetwork("eip155:8453"),       // Payment network (Base)
)
```

Environment variables (auto-detected):
- `JARVISCLAW_API_KEY` — API key
- `JARVISCLAW_WALLET_KEY` — EVM private key (hex, with or without 0x prefix)
- `JARVISCLAW_BASE_URL` — Custom base URL

## Requirements

- Go >= 1.22
- For x402: wallet with USDC on Base (Chain ID 8453). No ETH needed for gas — the facilitator covers it.
