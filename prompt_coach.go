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
//
// ScoreBefore and ScoreAfter are integers on a 1-100 scale, not 0-10.
type PromptCoachResponse struct {
	OriginalPrompt  string   `json:"original_prompt"`
	OptimizedPrompt string   `json:"optimized_prompt"`
	Explanation     string   `json:"explanation"`
	ScoreBefore     int      `json:"score_before"`
	ScoreAfter      int      `json:"score_after"`
	Suggestions     []string `json:"suggestions"`
	ModelUsed       string   `json:"model_used"`
}

// PromptCoach optimizes a prompt and returns improvement suggestions plus a
// before/after quality score.
//
// POST /v1/prompt-coach/optimize — requires auth (API key or x402).
//
// The coaching model is chosen by the gateway; PromptCoachRequest.Model only
// tells the coach which model the prompt is *destined for*.
//
// Example:
//
//	result, err := c.PromptCoach(ctx, jarvisclaw.PromptCoachRequest{
//	    Prompt:  "make me a picture of a dog",
//	    Context: "high-quality image generation prompt",
//	})
//	fmt.Println(result.OptimizedPrompt)
//
// There is no separate score-only endpoint: call this and read ScoreBefore.
func (c *Client) PromptCoach(ctx context.Context, req PromptCoachRequest) (*PromptCoachResponse, error) {
	// The handler wraps its result as {"success":true,"data":{...}} rather than
	// returning the object at the top level.
	var envelope struct {
		Success bool                `json:"success"`
		Data    PromptCoachResponse `json:"data"`
	}
	if err := c.doJSON(ctx, "POST", "/v1/prompt-coach/optimize", req, &envelope); err != nil {
		return nil, fmt.Errorf("prompt coach optimize: %w", err)
	}
	if !envelope.Success {
		return nil, fmt.Errorf("prompt coach optimize: server reported failure")
	}
	return &envelope.Data, nil
}
