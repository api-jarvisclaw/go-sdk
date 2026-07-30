package jarvisclaw

import (
	"context"
	"fmt"
)

// EmbeddingRequest is the input for POST /v1/embeddings.
//
// Input accepts a string, a []string, or a token-id array, matching the OpenAI
// embeddings API.
type EmbeddingRequest struct {
	Model          string `json:"model"`
	Input          any    `json:"input"`
	EncodingFormat string `json:"encoding_format,omitempty"` // "float" (default) or "base64"
	// Dimensions truncates the output vector. Pointer so an explicit 0 is
	// distinguishable from "not set"; only some models support it.
	Dimensions *int   `json:"dimensions,omitempty"`
	User       string `json:"user,omitempty"`
}

// Embedding is one vector in an embeddings response.
type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// EmbeddingResponse is the result of POST /v1/embeddings.
type EmbeddingResponse struct {
	Object string         `json:"object"`
	Data   []Embedding    `json:"data"`
	Model  string         `json:"model"`
	Usage  map[string]any `json:"usage"`
}

// Embeddings creates embedding vectors for the given input.
//
// POST /v1/embeddings — requires an API key or x402 payment.
//
// Note EncodingFormat "base64" makes the upstream return strings rather than
// float arrays, which will not decode into Embedding.Embedding. Leave it empty
// unless you are handling the raw response yourself.
func (c *Client) Embeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("embeddings: model is required")
	}
	if req.Input == nil {
		return nil, fmt.Errorf("embeddings: input is required")
	}
	var resp EmbeddingResponse
	if err := c.doJSON(ctx, "POST", "/v1/embeddings", req, &resp); err != nil {
		return nil, fmt.Errorf("embeddings: %w", err)
	}
	return &resp, nil
}

// Embed is a convenience wrapper returning a single vector for one string.
func (c *Client) Embed(ctx context.Context, model, text string) ([]float64, error) {
	resp, err := c.Embeddings(ctx, EmbeddingRequest{Model: model, Input: text})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("embed: server returned no vectors")
	}
	return resp.Data[0].Embedding, nil
}

// RerankRequest is the input for POST /v1/rerank.
//
// Documents accepts plain strings or objects, depending on what the chosen
// rerank model expects.
type RerankRequest struct {
	Model     string `json:"model"`
	Query     string `json:"query"`
	Documents []any  `json:"documents"`
	// TopN limits how many ranked results come back. Pointer so 0 is not
	// silently dropped as "unset".
	TopN            *int  `json:"top_n,omitempty"`
	ReturnDocuments *bool `json:"return_documents,omitempty"`
	MaxChunkPerDoc  *int  `json:"max_chunk_per_doc,omitempty"`
	OverlapTokens   *int  `json:"overlap_tokens,omitempty"`
}

// RerankResult is one scored document from a rerank response.
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	// Document is present only when ReturnDocuments was true. Its shape follows
	// what was submitted, so it stays untyped.
	Document any `json:"document,omitempty"`
}

// RerankResponse is the result of POST /v1/rerank.
type RerankResponse struct {
	Results []RerankResult `json:"results"`
	Model   string         `json:"model,omitempty"`
	Usage   map[string]any `json:"usage,omitempty"`
}

// Rerank reorders documents by relevance to a query.
//
// POST /v1/rerank — requires an API key or x402 payment.
func (c *Client) Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("rerank: model is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("rerank: query is required")
	}
	if len(req.Documents) == 0 {
		return nil, fmt.Errorf("rerank: documents is required")
	}
	var resp RerankResponse
	if err := c.doJSON(ctx, "POST", "/v1/rerank", req, &resp); err != nil {
		return nil, fmt.Errorf("rerank: %w", err)
	}
	return &resp, nil
}

// RerankTexts is a convenience wrapper over Rerank for plain-string documents.
func (c *Client) RerankTexts(ctx context.Context, model, query string, docs []string) ([]RerankResult, error) {
	anyDocs := make([]any, len(docs))
	for i, d := range docs {
		anyDocs[i] = d
	}
	resp, err := c.Rerank(ctx, RerankRequest{Model: model, Query: query, Documents: anyDocs})
	if err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// Moderate classifies text against the content policy.
//
// POST /v1/moderations — requires an API key or x402 payment.
//
// Input accepts a string or []string. The result shape is provider-specific, so
// it is returned decoded but untyped.
func (c *Client) Moderate(ctx context.Context, model string, input any) (map[string]any, error) {
	if input == nil {
		return nil, fmt.Errorf("moderations: input is required")
	}
	body := map[string]any{"input": input}
	if model != "" {
		body["model"] = model
	}
	return c.doPostCtx(ctx, "/v1/moderations", body)
}

// Responses calls the OpenAI-compatible Responses API for multi-turn agent runs.
//
// POST /v1/responses — requires an API key or x402 payment.
//
// The request and response schemas track OpenAI's and change often, so both are
// passed through untyped rather than pinned to a struct that would drift.
func (c *Client) Responses(ctx context.Context, req map[string]any) (map[string]any, error) {
	if req == nil {
		return nil, fmt.Errorf("responses: request is required")
	}
	return c.doPostCtx(ctx, "/v1/responses", req)
}

// EmbedBatch returns one vector per input string, in input order.
//
// Results are sorted by the response's index field rather than trusting array
// order, which the API does not guarantee.
func (c *Client) EmbedBatch(ctx context.Context, model string, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("embed batch: texts is required")
	}
	resp, err := c.Embeddings(ctx, EmbeddingRequest{Model: model, Input: texts})
	if err != nil {
		return nil, err
	}
	out := make([][]float64, len(resp.Data))
	for _, item := range resp.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, fmt.Errorf("embed batch: response index %d out of range for %d inputs",
				item.Index, len(out))
		}
		out[item.Index] = item.Embedding
	}
	return out, nil
}
