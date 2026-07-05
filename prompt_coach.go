package jarvisclaw

import (
	"context"
	"fmt"
)

// PromptCoachRequest is the input for prompt optimization.
type PromptCoachRequest struct {
	// Prompt is the original prompt to optimize (required).
	Prompt string `json:"prompt"`
	// Context provides usage context (e.g., "technical blog for developers").
	Context string `json:"context,omitempty"`
	// Model is the target model the prompt will be used with.
	Model string `json:"model,omitempty"`
	// OptimizeFor sets the optimization strategy: "clarity", "technical", "creative".
	OptimizeFor string `json:"optimize_for,omitempty"`
}

// PromptCoachResponse is the result of a prompt optimization request.
type PromptCoachResponse struct {
	OptimizedPrompt string   `json:"optimized_prompt"`
	Suggestions     []string `json:"suggestions"`
	Score           float64  `json:"score"`
	ScoreBefore     float64  `json:"score_before"`
	ScoreAfter      float64  `json:"score_after"`
}

// PromptScoreRequest is the input for prompt scoring (without optimization).
type PromptScoreRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
}

// PromptScoreResponse is the result of a prompt scoring request.
type PromptScoreResponse struct {
	Score     float64            `json:"score"`
	Breakdown map[string]float64 `json:"breakdown"`
}

// PromptCoach optimizes a prompt and returns improvement suggestions.
// Fixed pricing: $0.002 USDC per request.
//
// POST /v1/prompt-coach/optimize — requires auth (API key or x402).
//
// Example:
//
//	result, err := c.PromptCoach(ctx, jarvisclaw.PromptCoachRequest{
//	    Prompt:  "make me a picture of a dog",
//	    Context: "high-quality image generation prompt",
//	})
//	fmt.Println(result.OptimizedPrompt)
func (c *Client) PromptCoach(ctx context.Context, req PromptCoachRequest) (*PromptCoachResponse, error) {
	var resp PromptCoachResponse
	if err := c.doJSON(ctx, "POST", "/v1/prompt-coach/optimize", req, &resp); err != nil {
		return nil, fmt.Errorf("prompt coach optimize: %w", err)
	}
	return &resp, nil
}

// PromptScore evaluates a prompt's quality without optimizing it.
// Fixed pricing: $0.002 USDC per request.
//
// POST /v1/prompt-coach/score — requires auth (API key or x402).
func (c *Client) PromptScore(ctx context.Context, req PromptScoreRequest) (*PromptScoreResponse, error) {
	var resp PromptScoreResponse
	if err := c.doJSON(ctx, "POST", "/v1/prompt-coach/score", req, &resp); err != nil {
		return nil, fmt.Errorf("prompt coach score: %w", err)
	}
	return &resp, nil
}
