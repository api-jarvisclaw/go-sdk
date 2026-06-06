package jarvisclaw_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk"
)

// ─── Test helpers ──────────────────────────────────────────

var (
	apiKey    = os.Getenv("JARVISCLAW_API_KEY")
	walletKey = os.Getenv("JARVISCLAW_WALLET_KEY")
)

func skipNoAPIKey(t *testing.T) {
	t.Helper()
	if apiKey == "" {
		t.Skip("JARVISCLAW_API_KEY not set")
	}
}

func skipNoWallet(t *testing.T) {
	t.Helper()
	if walletKey == "" {
		t.Skip("JARVISCLAW_WALLET_KEY not set")
	}
}

func logResult(t *testing.T, name string, details map[string]interface{}) {
	t.Helper()
	t.Logf("\n────────────────────────────────────────────────────────────")
	t.Logf("  TEST: %s", name)
	t.Logf("────────────────────────────────────────────────────────────")
	for k, v := range details {
		t.Logf("  %s: %v", k, v)
	}
	t.Logf("────────────────────────────────────────────────────────────")
}

// ─── ChatClient Tests ──────────────────────────────────────

func TestChatComplete_APIKey(t *testing.T) {
	skipNoAPIKey(t)
	c, err := jarvisclaw.NewChatClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	start := time.Now()
	text, err := c.Complete(context.Background(), "Say 'hello' and nothing else")
	elapsed := time.Since(start)

	logResult(t, "Chat.Complete (APIKey)", map[string]interface{}{
		"response":   text,
		"error":      err,
		"latency_ms": elapsed.Milliseconds(),
		"auth":       "API Key",
	})

	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text == "" {
		t.Fatal("Expected non-empty response")
	}
}

func TestChatComplete_X402(t *testing.T) {
	skipNoWallet(t)
	c, err := jarvisclaw.NewChatClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	start := time.Now()
	text, err := c.Complete(context.Background(), "Say 'hello' and nothing else")
	elapsed := time.Since(start)

	logResult(t, "Chat.Complete (X402)", map[string]interface{}{
		"response":   text,
		"error":      err,
		"latency_ms": elapsed.Milliseconds(),
		"auth":       "x402 wallet",
	})

	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text == "" {
		t.Fatal("Expected non-empty response")
	}
}

func TestChatCompletion_APIKey(t *testing.T) {
	skipNoAPIKey(t)
	c, err := jarvisclaw.NewChatClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	start := time.Now()
	resp, err := c.Completion(context.Background(), []jarvisclaw.Message{
		{Role: "system", Content: "Reply with exactly one word."},
		{Role: "user", Content: "What color is the sky?"},
	})
	elapsed := time.Since(start)

	logResult(t, "Chat.Completion (APIKey)", map[string]interface{}{
		"content":    resp.Content,
		"model":      resp.Model,
		"usage":      resp.Usage,
		"latency_ms": elapsed.Milliseconds(),
	})

	if err != nil {
		t.Fatalf("Completion: %v", err)
	}
	if resp.Content == "" {
		t.Fatal("Expected non-empty content")
	}
}

func TestChatStream_APIKey(t *testing.T) {
	skipNoAPIKey(t)
	c, err := jarvisclaw.NewChatClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	start := time.Now()
	sr, err := c.Stream(context.Background(), "Count from 1 to 3")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []string
	for {
		chunk, ok := sr.Read()
		if !ok {
			break
		}
		chunks = append(chunks, chunk)
	}
	elapsed := time.Since(start)

	fullText := ""
	for _, ch := range chunks {
		fullText += ch
	}

	logResult(t, "Chat.Stream (APIKey)", map[string]interface{}{
		"chunk_count":   len(chunks),
		"full_response": fullText,
		"latency_ms":    elapsed.Milliseconds(),
	})

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}
}

func TestChatConcurrent_X402(t *testing.T) {
	skipNoWallet(t)

	chat, err := jarvisclaw.NewChatClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	start := time.Now()
	var wg sync.WaitGroup
	results := make([]string, 3)
	errors := make([]error, 3)
	prompts := []string{"Say 'one'", "Say 'two'", "Say 'three'"}

	for i, p := range prompts {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			text, err := chat.Complete(context.Background(), p)
			results[i] = text
			errors[i] = err
		}(i, p)
	}
	wg.Wait()
	elapsed := time.Since(start)

	logResult(t, "Chat.Concurrent x3 (X402)", map[string]interface{}{
		"results":    results,
		"errors":     errors,
		"latency_ms": elapsed.Milliseconds(),
	})

	for i, r := range results {
		if errors[i] != nil {
			t.Errorf("Concurrent[%d]: %v", i, errors[i])
		}
		if r == "" {
			t.Errorf("Result[%d] empty", i)
		}
	}
}

// ─── ImageClient Tests ─────────────────────────────────────

func TestImageGenerate_APIKey(t *testing.T) {
	skipNoAPIKey(t)

	ic, err := jarvisclaw.NewImageClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewImageClient: %v", err)
	}

	start := time.Now()
	img, err := ic.Generate(context.Background(), "A simple red circle on white background", jarvisclaw.WithSize("1024x1024"))
	elapsed := time.Since(start)

	imgURL := ""
	if img != nil {
		imgURL = img.URL
	}
	logResult(t, "Image.Generate (APIKey)", map[string]interface{}{
		"url":       imgURL,
		"error":     err,
		"latency_s": fmt.Sprintf("%.1f", elapsed.Seconds()),
	})

	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if img.URL == "" {
		t.Fatal("Expected image URL")
	}
}

func TestImageGenerate_X402(t *testing.T) {
	skipNoWallet(t)

	ic, err := jarvisclaw.NewImageClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewImageClient: %v", err)
	}

	start := time.Now()
	img, err := ic.Generate(context.Background(), "A blue square", jarvisclaw.WithSize("1024x1024"))
	elapsed := time.Since(start)

	imgURL := ""
	if img != nil {
		imgURL = img.URL
	}
	logResult(t, "Image.Generate (X402)", map[string]interface{}{
		"url":       imgURL,
		"error":     err,
		"latency_s": fmt.Sprintf("%.1f", elapsed.Seconds()),
	})

	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if img == nil {
		t.Fatal("Expected non-nil image response")
	}
	if img.URL == "" {
		t.Fatal("Expected image URL")
	}
}

// ─── VideoClient Tests ─────────────────────────────────────

func TestVideoGenerate_APIKey(t *testing.T) {
	skipNoAPIKey(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	vc, err := jarvisclaw.NewVideoClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewVideoClient: %v", err)
	}

	start := time.Now()
	job, err := vc.Generate(ctx, "A ball bouncing", jarvisclaw.WithDuration(5))
	elapsed := time.Since(start)

	var jobID, jobStatus, jobURL string
	if job != nil {
		jobID, jobStatus, jobURL = job.ID, job.Status, job.URL
	}
	logResult(t, "Video.Generate blocking (APIKey)", map[string]interface{}{
		"job_id":    jobID,
		"status":    jobStatus,
		"url":       jobURL,
		"error":     err,
		"latency_s": fmt.Sprintf("%.1f", elapsed.Seconds()),
	})

	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if job.ID == "" && job.URL == "" {
		t.Fatal("Expected job ID or URL")
	}
}

func TestVideoGenerate_X402(t *testing.T) {
	skipNoWallet(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	vc, err := jarvisclaw.NewVideoClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewVideoClient: %v", err)
	}

	start := time.Now()
	job, err := vc.Generate(ctx, "A leaf falling slowly", jarvisclaw.WithDuration(5))
	elapsed := time.Since(start)

	var jobID, jobStatus, jobURL string
	if job != nil {
		jobID, jobStatus, jobURL = job.ID, job.Status, job.URL
	}
	logResult(t, "Video.Generate blocking (X402)", map[string]interface{}{
		"job_id":    jobID,
		"status":    jobStatus,
		"url":       jobURL,
		"error":     err,
		"latency_s": fmt.Sprintf("%.1f", elapsed.Seconds()),
	})

	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

func TestVideoNonBlocking_X402(t *testing.T) {
	skipNoWallet(t)

	vc, err := jarvisclaw.NewVideoClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewVideoClient: %v", err)
	}

	start := time.Now()
	job, err := vc.Generate(context.Background(), "Clouds moving", jarvisclaw.WithDuration(5), jarvisclaw.WithWait(false))
	elapsed := time.Since(start)

	var jobID, jobStatus string
	if job != nil {
		jobID, jobStatus = job.ID, job.Status
	}
	logResult(t, "Video.Generate non-blocking (X402)", map[string]interface{}{
		"job_id":    jobID,
		"status":    jobStatus,
		"error":     err,
		"latency_ms": elapsed.Milliseconds(),
	})

	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if job.ID == "" {
		t.Fatal("Expected job ID")
	}

	// Check status
	status, err := vc.Status(context.Background(), job.ID)
	var statusStr, statusURL string
	if status != nil {
		statusStr, statusURL = status.Status, status.URL
	}
	logResult(t, "Video.Status (X402)", map[string]interface{}{
		"status": statusStr,
		"url":    statusURL,
		"error":  err,
	})
}

// ─── SearchClient Tests ────────────────────────────────────

func TestSearch_APIKey(t *testing.T) {
	skipNoAPIKey(t)

	sc, err := jarvisclaw.NewClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	results, err := sc.Search(context.Background(), "Python programming")
	elapsed := time.Since(start)

	logResult(t, "Search.Query (APIKey)", map[string]interface{}{
		"result_count": len(results),
		"error":        err,
		"latency_ms":   elapsed.Milliseconds(),
	})
	if len(results) > 0 {
		t.Logf("  first_title: %s", results[0].Title)
		t.Logf("  first_url: %s", results[0].URL)
	}

	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected search results")
	}
}

func TestSearch_X402(t *testing.T) {
	skipNoWallet(t)

	sc, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	results, err := sc.Search(context.Background(), "Bitcoin price")
	elapsed := time.Since(start)

	logResult(t, "Search.Query (X402)", map[string]interface{}{
		"result_count": len(results),
		"error":        err,
		"latency_ms":   elapsed.Milliseconds(),
	})

	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected search results")
	}
}

// ─── AudioClient Tests ─────────────────────────────────────

func TestAudioSpeech_APIKey(t *testing.T) {
	skipNoAPIKey(t)

	c, err := jarvisclaw.NewClient(jarvisclaw.WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	data, err := c.AudioSpeech(context.Background(), "elevenlabs/flash-v2.5", "Hello world, this is a test.", "alloy")
	elapsed := time.Since(start)

	logResult(t, "Audio.Speech (APIKey)", map[string]interface{}{
		"data_bytes": len(data),
		"error":      err,
		"latency_ms": elapsed.Milliseconds(),
	})

	if err != nil {
		t.Fatalf("AudioSpeech: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("Expected audio data >1KB, got %d bytes", len(data))
	}
}

func TestAudioSpeech_X402(t *testing.T) {
	skipNoWallet(t)

	c, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	data, err := c.AudioSpeech(context.Background(), "elevenlabs/flash-v2.5", "Test audio output", "nova")
	elapsed := time.Since(start)

	logResult(t, "Audio.Speech (X402)", map[string]interface{}{
		"data_bytes": len(data),
		"error":      err,
		"latency_ms": elapsed.Milliseconds(),
	})

	if err != nil {
		t.Fatalf("AudioSpeech: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("Expected audio data >1KB, got %d bytes", len(data))
	}
}

// ─── Balance & Utility Tests ───────────────────────────────

func TestGetBalance_X402(t *testing.T) {
	skipNoWallet(t)

	c, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	balance, err := c.GetBalance(context.Background())
	elapsed := time.Since(start)

	logResult(t, "Utility.GetBalance (X402)", map[string]interface{}{
		"balance_usd": fmt.Sprintf("$%.4f", balance),
		"error":       err,
		"latency_ms":  elapsed.Milliseconds(),
	})

	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if balance < 0 {
		t.Fatalf("Expected non-negative balance, got %f", balance)
	}
}

func TestWalletAddress_X402(t *testing.T) {
	skipNoWallet(t)

	c, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(walletKey))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	addr := c.Address()
	logResult(t, "Utility.WalletAddress (X402)", map[string]interface{}{
		"address": addr,
	})

	if addr == "" {
		t.Fatal("Expected wallet address")
	}
	if len(addr) != 42 || addr[:2] != "0x" {
		t.Fatalf("Invalid address format: %s", addr)
	}
}

// ─── Error Tests ───────────────────────────────────────────

func TestInvalidAPIKey(t *testing.T) {
	c, err := jarvisclaw.NewChatClient(jarvisclaw.WithAPIKey("sk-invalid-key-12345"))
	if err != nil {
		t.Fatalf("NewChatClient: %v", err)
	}

	start := time.Now()
	_, err = c.Complete(context.Background(), "Hello")
	elapsed := time.Since(start)

	logResult(t, "Error.InvalidAPIKey", map[string]interface{}{
		"error":      err,
		"latency_ms": elapsed.Milliseconds(),
	})

	if err == nil {
		t.Fatal("Expected error with invalid API key")
	}
}
