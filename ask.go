package jarvisclaw

import (
	"context"
	"fmt"
)

// AskOptions configures the high-level Ask method.
type AskOptions struct {
	// Budget is the maximum cost in USD for a single call. Defaults to 0.05.
	Budget float64
	// Optimize controls the resolution preference: "cost", "quality", or "latency". Defaults to "cost".
	Optimize string
	// Model bypasses intent resolution and uses this model directly.
	Model string
}

// Ask resolves the best model within budget and executes a chat completion.
// This is the one-call experience for agents: resolve → chat in one step.
//
// Example:
//
//	answer, err := c.Ask(ctx, "Summarize the Go spec in 3 bullet points")
//	answer, err := c.Ask(ctx, "...", AskOptions{Budget: 0.10, Optimize: "quality"})
func (c *Client) Ask(ctx context.Context, prompt string, opts ...AskOptions) (string, error) {
	var opt AskOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Budget == 0 {
		opt.Budget = 0.05
	}
	if opt.Optimize == "" {
		opt.Optimize = "cost"
	}

	model := opt.Model
	if model == "" {
		// Resolve the best provider within budget.
		maxPrice := opt.Budget
		resp, err := c.Resolve(ctx, ResolveRequest{
			Intent:      "chat_completion",
			Constraints: Constraints{MaxPriceUSD: &maxPrice},
			Preferences: Preferences{OptimizeFor: opt.Optimize},
		})
		if err != nil {
			return "", fmt.Errorf("ask resolve: %w", err)
		}
		if len(resp.Matches) == 0 {
			return "", fmt.Errorf("no provider found within budget $%.4f", opt.Budget)
		}
		model = resp.Matches[0].Model
	}

	text, err := c.Chat(ctx, model, prompt)
	if err != nil {
		return "", fmt.Errorf("ask chat: %w", err)
	}
	return text, nil
}
