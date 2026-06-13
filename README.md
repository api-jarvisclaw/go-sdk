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
    chat, _ := jc.NewChatClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))
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
img, _ := jc.NewImageClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))

result, _ := img.Generate(ctx, "A cat in space",
    jc.WithSize("1024x1024"),
    jc.WithImageModel("auto/image"),
)
fmt.Println(result.URL)
```

### VideoClient

```go
vid, _ := jc.NewVideoClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))

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

### MarketplaceClient (RPC, DeFi)

```go
mp, _ := jc.NewMarketplaceClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))

// JSON-RPC to any chain
result, _ := mp.RPCCall(ctx, "ethereum", "eth_blockNumber", []any{})
fmt.Println(result["result"])

// Batch RPC
results, _ := mp.RPCBatch(ctx, "base", []jc.RPCRequest{
    {Method: "eth_blockNumber", Params: []any{}},
    {Method: "eth_gasPrice", Params: []any{}},
})

// DeFi data
protocols, _ := mp.DefiProtocols(ctx)
yields, _ := mp.DefiYields(ctx)
```

## Balance

```go
client, _ := jc.NewClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))

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
