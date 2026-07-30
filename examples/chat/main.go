// Chat completions — smart routing, explicit models, and streaming.
//
// Run: go run ./examples/chat
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk/v2"
)

func main() {
	ctx := context.Background()

	chat, err := jarvisclaw.NewChatClient(
		jarvisclaw.WithAPIKey(os.Getenv("JARVISCLAW_API_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- Simplest form: one string in, one string out ---------------------
	answer, err := chat.Complete(ctx,
		"Explain the x402 payment protocol in one sentence.",
		jarvisclaw.WithChatModel("openai/gpt-4o-mini"),
		jarvisclaw.WithMaxTokens(60),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Direct:", answer)

	// Omitting WithChatModel selects smart routing ("auto"), where the gateway
	// picks a provider. Note that auto settles on-chain over x402 rather than
	// debiting quota, so it needs a funded wallet.

	// --- A system prompt and sampling controls ----------------------------
	answer, err = chat.Complete(ctx,
		"Write a haiku about settlement latency.",
		jarvisclaw.WithChatModel("openai/gpt-4o-mini"),
		jarvisclaw.WithSystem("You are a terse poet who likes financial plumbing."),
		jarvisclaw.WithTemperature(0.9),
		jarvisclaw.WithMaxTokens(60),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nHaiku:\n%s\n", answer)

	// --- Full message list, when you want the response metadata -----------
	resp, err := chat.Completion(ctx,
		[]jarvisclaw.Message{
			{Role: "system", Content: "Answer in exactly one word."},
			{Role: "user", Content: "Name the default settlement chain here."},
		},
		jarvisclaw.WithChatModel("openai/gpt-4o-mini"),
		jarvisclaw.WithMaxTokens(5),
		// Temperature 0 is honoured rather than treated as unset, so this is
		// genuinely deterministic.
		jarvisclaw.WithTemperature(0),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nContent: %s\nModel:   %s\nUsage:   %+v\n",
		resp.Content, resp.Model, resp.Usage)

	// --- Streaming --------------------------------------------------------
	stream, err := chat.Stream(ctx,
		"Count from one to five, words only.",
		jarvisclaw.WithChatModel("openai/gpt-4o-mini"),
		jarvisclaw.WithMaxTokens(30),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print("\nStreaming: ")
	for chunk := range stream.Channel() {
		fmt.Print(chunk)
	}
	fmt.Println()
}
