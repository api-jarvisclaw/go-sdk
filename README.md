# JarvisClaw Go SDK

Go SDK for JarvisClaw AI & Prediction Market APIs with x402 machine payments.

## Install

```bash
go get github.com/api-jarvisclaw/go-sdk
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

    // Chat
    chat, _ := jc.NewChatClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))
    text, _ := chat.Complete(ctx, "What is quantum computing?")
    fmt.Println(text)

    // Streaming
    stream, _ := chat.Stream(ctx, "Tell me a joke")
    for chunk := range stream.Channel() {
        fmt.Print(chunk)
    }

    // Image generation
    image, _ := jc.NewImageClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))
    result, _ := image.Generate(ctx, "A cat in space")
    fmt.Println(result.URL)

    // Search
    search, _ := jc.NewSearchClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))
    results, _ := search.Query(ctx, "latest AI news")
    for _, r := range results {
        fmt.Println(r.Title, r.URL)
    }

    // Prediction Market
    mp, _ := jc.NewMarketplaceClient(jc.WithPrivateKey(os.Getenv("WALLET_KEY")))
    markets, _ := mp.Call(ctx, "polymarket", "markets?limit=10")
    fmt.Println(markets)
}
```

## Authentication

```go
// API Key mode
client, _ := jc.NewChatClient(jc.WithAPIKey("sk-your-key"))

// x402 wallet (EVM / Base chain)
client, _ := jc.NewChatClient(jc.WithPrivateKey("0x<hex-private-key>"))

// x402 wallet (Solana)
client, _ := jc.NewChatClient(jc.WithPrivateKey("<base58-solana-keypair>"))

// Environment variable: JARVISCLAW_API_KEY or JARVISCLAW_WALLET_KEY
client, _ := jc.NewChatClient()
```

## Requirements

- Go >= 1.22
- Wallet with USDC on Base chain (Chain ID 8453) or Solana
- No ETH/SOL needed for gas (facilitator pays)
