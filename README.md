# JarvisClaw Go SDK

Go SDK for [JarvisClaw AI](https://jarvisclaw.ai) — intent-based AI routing with x402 USDC micropayments.

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
text, _ := client.Ask(ctx, "Write a haiku about Go",
    jc.AskOptions{Budget: 0.02, Optimize: "quality"})
fmt.Println(text)
```

### Streaming

```go
stream, _ := client.Stream(ctx, jc.StreamRequest{
    Intent:  "chat_completion",
    Payload: map[string]any{"messages": []map[string]string{{"role": "user", "content": "Count to 10"}}},
    Budget:  jc.Budget{MaxTotalUSD: 0.01},
})
for chunk := range stream.Channel() {
    fmt.Print(chunk)
}
```

### Wallet

```go
balance, _ := client.GetBalance(ctx)
fmt.Printf("$%.2f USDC\n", balance)

// Wallet pools & limits
pools, _ := client.WalletPools(ctx)
limits, _ := client.WalletLimits(ctx)
client.SetWalletLimits(ctx, jc.WalletLimits{DailyMaxUSD: 30.0})
```

### Prompt Coach

```go
result, _ := client.PromptCoach(ctx, jc.PromptCoachRequest{
    Prompt:  "write code that does the thing",
    Context: "technical blog for developers",
})
fmt.Println(result.OptimizedPrompt)
fmt.Println(result.Suggestions)

score, _ := client.PromptScore(ctx, jc.PromptScoreRequest{
    Prompt: "Explain quantum entanglement using analogies for a physics undergrad",
})
fmt.Printf("Score: %.1f/10\n", score.Score)
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

result, _ := img.Generate(ctx, "A cat in space",
    jc.WithSize("1024x1024"),
    jc.WithImageModel("auto/image"),
)
fmt.Println(result.URL)
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

## Requirements

- Go >= 1.22

## Links

- [AIP Protocol Spec](https://docs.jarvisclaw.ai/aip)
- [SDK Reference](https://docs.jarvisclaw.ai/sdk)
- [Telegram](https://t.me/JarvisClawai)

## License

MIT
