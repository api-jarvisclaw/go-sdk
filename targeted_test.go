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
	targetedAPIKey = envOrTargeted("JARVISCLAW_API_KEY", "sk-OtqnrUGuNoROqKbJR9IlUFbQclLSH2vFWsvjMnR5744ZHMF0")
	targetedWallet = os.Getenv("JARVISCLAW_WALLET_KEY")
)

func envOrTargeted(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
