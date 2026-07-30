// Marketplace services — DeFi data, blockchain RPC, and arbitrary services.
//
// The marketplace proxies third-party services through the gateway, so one API
// key reaches all of them.
//
// Billing note: marketplace services settle on-chain over x402 rather than
// debiting account quota, so they need a funded HD wallet even with API-key
// auth. On an empty wallet every call answers 403 "insufficient HD wallet
// balance". Run ./examples/wallet to check your balance first.
//
// Run: go run ./examples/marketplace
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk/v2"
)

// show runs one marketplace call and reports an unfunded wallet as such rather
// than aborting, so the example stays readable on an empty account.
func show(label string, fn func() (map[string]any, error)) map[string]any {
	out, err := fn()
	if err == nil {
		return out
	}
	var apiErr *jarvisclaw.APIError
	if errors.As(err, &apiErr) && (apiErr.StatusCode == 402 || apiErr.StatusCode == 403) {
		fmt.Printf("%s: needs a funded wallet (%d)\n", label, apiErr.StatusCode)
		return nil
	}
	fmt.Printf("%s: failed: %v\n", label, err)
	return nil
}

func main() {
	ctx := context.Background()

	mp, err := jarvisclaw.NewMarketplaceClient(
		jarvisclaw.WithAPIKey(os.Getenv("JARVISCLAW_API_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- DeFi -------------------------------------------------------------
	if protocols := show("defi/protocols", func() (map[string]any, error) {
		return mp.DefiProtocols(ctx)
	}); protocols != nil {
		fmt.Printf("Protocols payload keys: %d\n", len(protocols))
	}

	if aave := show("defi/aave", func() (map[string]any, error) {
		return mp.DefiProtocol(ctx, "aave")
	}); aave != nil {
		fmt.Printf("aave: %v\n", aave["name"])
	}

	if tvl := show("defi/tvl", func() (map[string]any, error) {
		return mp.DefiTVL(ctx)
	}); tvl != nil {
		fmt.Printf("Total TVL: %v\n", tvl)
	}

	// --- Blockchain RPC ---------------------------------------------------
	if block := show("rpc eth_blockNumber", func() (map[string]any, error) {
		return mp.RPCCall(ctx, "base", "eth_blockNumber", []any{})
	}); block != nil {
		fmt.Printf("Base block number: %v\n", block["result"])
	}

	// RPCBatch sends several JSON-RPC calls in one request. It returns a slice,
	// so it does not fit the show() helper above.
	batch, err := mp.RPCBatch(ctx, "base", []jarvisclaw.RPCRequest{
		{Method: "eth_blockNumber"},
		{Method: "eth_gasPrice"},
	})
	if err != nil {
		fmt.Println("rpc batch:", err)
	} else {
		fmt.Printf("Batched %d responses\n", len(batch))
	}

	// --- Any service by name ----------------------------------------------
	// Call reaches any marketplace service and path, so you are not limited to
	// the convenience wrappers above.
	if price := show("surf/exchange/price", func() (map[string]any, error) {
		return mp.Call(ctx, "surf", "exchange/price",
			jarvisclaw.WithParams(map[string]string{"symbol": "BTC"}))
	}); price != nil {
		fmt.Printf("BTC price: %v\n", price)
	}
}
