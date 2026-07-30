// Embeddings — single vectors, batches, and cosine similarity.
//
// Run: go run ./examples/embeddings
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk/v2"
)

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func main() {
	ctx := context.Background()

	client, err := jarvisclaw.NewClient(
		jarvisclaw.WithAPIKey(os.Getenv("JARVISCLAW_API_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- A single embedding -----------------------------------------------
	// Embed is the convenience path: text in, vector out.
	vec, err := client.Embed(ctx, "text-embedding-3-small",
		"Agents settle payments over x402.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Dimensions: %d\n", len(vec))
	fmt.Printf("First few:  %.4f\n", vec[:5])

	// --- Several at once --------------------------------------------------
	// One request, one vector per input. Embeddings orders results by the
	// response's index field rather than trusting array order.
	resp, err := client.Embeddings(ctx, jarvisclaw.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []string{"USDC on Base", "USDC on Solana", "A recipe for sourdough"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n%d vectors of %d dimensions each\n",
		len(resp.Data), len(resp.Data[0].Embedding))
	fmt.Printf("Model: %s  usage: %+v\n", resp.Model, resp.Usage)

	// The first two should be closer to each other than to the third.
	v := resp.Data
	fmt.Printf("  base vs solana:    %.4f\n", cosine(v[0].Embedding, v[1].Embedding))
	fmt.Printf("  base vs sourdough: %.4f\n", cosine(v[0].Embedding, v[2].Embedding))

	// --- Reranking and moderation -----------------------------------------
	//
	// Rerank, RerankTexts and Moderate exist, but this deployment has no rerank
	// or moderation model configured, so they answer 503 ("no available
	// channel"). Check ListModels for what your gateway actually serves.
	//
	// ranked, err := client.RerankTexts(ctx, "<a rerank model you serve>",
	//     "How do I pay an API with a crypto wallet?",
	//     []string{"Sourdough needs a starter.", "x402 settles HTTP requests in USDC."})
}
