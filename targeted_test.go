//go:build integration

package jarvisclaw_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk"
)

// Targeted tests for previously failing scenarios from the retest report.
// Run: cd sdk/go && go test -tags=integration -v -run TestTargeted -timeout=600s
//
// Categories tested:
// 1. Image X402 nil pointer (was: panic on nil result)
// 2. TTS / auto/tts (was: no channel for tts-1)
// 3. auto/search (was: endpoint type error)

var (
	targetedAPIKey = os.Getenv("JARVISCLAW_API_KEY")
	targetedWallet = os.Getenv("JARVISCLAW_WALLET_KEY")
)

func skipNoAPIKeyTargeted(t *testing.T) {
	t.Helper()
	if targetedAPIKey == "" {
		t.Skip("JARVISCLAW_API_KEY not set")
	}
}

func skipNoWalletTargeted(t *testing.T) {
	t.Helper()
	if targetedWallet == "" {
		t.Skip("JARVISCLAW_WALLET_KEY not set")
	}
}

// --- Image X402 nil pointer (was: panic on nil result) ---

func TestTargeted_ImageX402_NilSafe(t *testing.T) {
	skipNoWalletTargeted(t)

	ic, err := jarvisclaw.NewImageClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewImageClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	img, err := ic.Generate(ctx, "A blue square on white background", jarvisclaw.WithSize("1024x1024"))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if img == nil {
		t.Fatal("Expected non-nil image response")
	}
	if img.URL == "" {
		t.Fatal("Expected image URL, got empty string")
	}
	fmt.Printf("Image URL: %s\n", img.URL)
}

func TestTargeted_ImageAPIKey(t *testing.T) {
	skipNoAPIKeyTargeted(t)
	ic, err := jarvisclaw.NewImageClient(jarvisclaw.WithAPIKey(targetedAPIKey))
	if err != nil {
		t.Fatalf("NewImageClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	img, err := ic.Generate(ctx, "A red circle on black background", jarvisclaw.WithSize("1024x1024"))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if img == nil {
		t.Fatal("Expected non-nil image response")
	}
	if img.URL == "" {
		t.Fatal("Expected image URL, got empty string")
	}
	fmt.Printf("Image URL: %s\n", img.URL)
}

// --- TTS / auto/tts (was: no channel for tts-1) ---

func TestTargeted_Speech_APIKey(t *testing.T) {
	skipNoAPIKeyTargeted(t)
	c, err := jarvisclaw.NewClient(jarvisclaw.WithAPIKey(targetedAPIKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	data, err := c.AudioSpeech(ctx, "auto/tts", "Hello, this is a targeted test.", "alloy")
	if err != nil {
		t.Fatalf("AudioSpeech: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("Audio too small: %d bytes", len(data))
	}
	fmt.Printf("TTS APIKey: %d bytes\n", len(data))
}

func TestTargeted_Speech_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	c, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	data, err := c.AudioSpeech(ctx, "auto/tts", "Testing x402 speech output.", "alloy")
	if err != nil {
		t.Fatalf("AudioSpeech: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("Audio too small: %d bytes", len(data))
	}
	fmt.Printf("TTS X402: %d bytes\n", len(data))
}

// --- auto/search (was: endpoint type error) ---

func TestTargeted_Search_APIKey(t *testing.T) {
	skipNoAPIKeyTargeted(t)
	cc, err := jarvisclaw.NewChatClient(jarvisclaw.WithAPIKey(targetedAPIKey))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// auto/search goes through chat completions with model=auto/search
	resp, err := cc.Complete(ctx, "What is the latest AI news?", jarvisclaw.WithChatModel("auto/search"))
	if err != nil {
		t.Fatalf("Complete with auto/search: %v", err)
	}
	if resp == "" {
		t.Fatal("Expected non-empty search response")
	}
	fmt.Printf("Search result length: %d chars\n", len(resp))
}

func TestTargeted_Search_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	cc, err := jarvisclaw.NewChatClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := cc.Complete(ctx, "What is quantum computing?", jarvisclaw.WithChatModel("auto/search"))
	if err != nil {
		t.Fatalf("Complete with auto/search: %v", err)
	}
	if resp == "" {
		t.Fatal("Expected non-empty search response")
	}
	fmt.Printf("Search X402 result length: %d chars\n", len(resp))
}

// --- RPC (Multi-chain JSON-RPC) ---

func TestTargeted_RPC_EthBlockNumber_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.RPCCall(ctx, "ethereum", "eth_blockNumber", []any{})
	if err != nil {
		t.Fatalf("RPCCall: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if _, ok := result["result"]; !ok {
		t.Fatalf("Expected 'result' key in response, got: %v", result)
	}
	fmt.Printf("eth_blockNumber: %v\n", result["result"])
}

func TestTargeted_RPC_EthGasPrice_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.RPCCall(ctx, "ethereum", "eth_gasPrice", []any{})
	if err != nil {
		t.Fatalf("RPCCall: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("eth_gasPrice: %v\n", result["result"])
}

func TestTargeted_RPC_BaseChain_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.RPCCall(ctx, "base", "eth_blockNumber", []any{})
	if err != nil {
		t.Fatalf("RPCCall base: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("Base eth_blockNumber: %v\n", result["result"])
}

func TestTargeted_RPC_Batch_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := mc.RPCBatch(ctx, "ethereum", []jarvisclaw.RPCRequest{
		{Method: "eth_blockNumber", Params: []any{}},
		{Method: "eth_gasPrice", Params: []any{}},
	})
	if err != nil {
		t.Fatalf("RPCBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	fmt.Printf("RPCBatch: %d results\n", len(results))
}

func TestTargeted_RPC_APIKey(t *testing.T) {
	skipNoAPIKeyTargeted(t)
	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithAPIKey(targetedAPIKey))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.RPCCall(ctx, "ethereum", "eth_blockNumber", []any{})
	if err != nil {
		t.Fatalf("RPCCall APIKey: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("RPC APIKey eth_blockNumber: %v\n", result["result"])
}

// --- DeFi Data (DefiLlama) ---

func TestTargeted_Defi_Protocols_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.DefiProtocols(ctx)
	if err != nil {
		t.Fatalf("DefiProtocols: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("DefiProtocols: got response with %d keys\n", len(result))
}

func TestTargeted_Defi_Protocol_Aave_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.DefiProtocol(ctx, "aave")
	if err != nil {
		t.Fatalf("DefiProtocol aave: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("Aave protocol data: %d keys\n", len(result))
}

func TestTargeted_Defi_Yields_X402(t *testing.T) {
	skipNoWalletTargeted(t)

	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithPrivateKey(targetedWallet))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.DefiYields(ctx)
	if err != nil {
		t.Fatalf("DefiYields: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("DefiYields: got response\n")
}

func TestTargeted_Defi_Protocols_APIKey(t *testing.T) {
	skipNoAPIKeyTargeted(t)
	mc, err := jarvisclaw.NewMarketplaceClient(jarvisclaw.WithAPIKey(targetedAPIKey))
	if err != nil {
		t.Fatalf("NewMarketplaceClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mc.DefiProtocols(ctx)
	if err != nil {
		t.Fatalf("DefiProtocols APIKey: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	fmt.Printf("DefiProtocols APIKey: got response\n")
}
