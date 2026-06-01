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
    "fmt"
    "os"

    jc "github.com/api-jarvisclaw/go-sdk"
)

func main() {
    client, err := jc.NewClient(
        jc.WithPrivateKey(os.Getenv("WALLET_KEY")),
    )
    if err != nil {
        panic(err)
    }

    fmt.Println("Address:", client.Address())

    // Prediction market data
    markets, err := client.Get("/v1/prediction/polymarket/markets", map[string]string{"limit": "10"})
    if err != nil {
        panic(err)
    }
    fmt.Println(markets)

    // AI model call
    resp, err := client.Post("/v1/chat/completions", map[string]any{
        "model":    "openai/gpt-5.4-nano",
        "messages": []map[string]string{{"role": "user", "content": "Hello!"}},
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(resp)
}
```

## Requirements

- Go >= 1.22
- Wallet with USDC on Base chain (Chain ID 8453)
- No ETH needed (facilitator pays gas)
