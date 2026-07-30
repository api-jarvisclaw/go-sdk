// AIP intent protocol — describe what you want, let the gateway pick a provider.
//
// Read-only: resolves and discovers without executing a paid intent.
//
// Run: go run ./examples/intent
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk/v2"
)

func main() {
	ctx := context.Background()

	client, err := jarvisclaw.NewClient(
		jarvisclaw.WithAPIKey(os.Getenv("JARVISCLAW_API_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- What can I ask for? ----------------------------------------------
	types, err := client.ListIntentTypes(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d intent types available.\n", len(types))
	if len(types) > 8 {
		fmt.Println("Sample:", strings.Join(types[:8], ", "))
	}

	// --- Discovery: browse intents and their providers --------------------
	found, err := client.Discover(ctx, jarvisclaw.DiscoverRequest{
		Intent: "chat_completion",
	})
	if err != nil {
		log.Fatal(err)
	}
	for i, in := range found.Intents {
		if i >= 3 {
			break
		}
		fmt.Printf("\n%s: %s\n  providers: %d\n", in.Type, in.Description, in.ProviderCount)
	}

	// --- Resolve: rank providers for a specific need -----------------------
	// Constraints are hard filters; preferences are soft ranking hints.
	ranked, err := client.Resolve(ctx, jarvisclaw.ResolveRequest{
		Intent:      "chat_completion",
		Constraints: jarvisclaw.Constraints{MaxPriceUSD: jarvisclaw.Float64Ptr(0.01)},
		Preferences: jarvisclaw.Preferences{OptimizeFor: "cost"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nTop matches (%d available):\n", ranked.TotalAvailable)
	for i, m := range ranked.Matches {
		if i >= 5 {
			break
		}
		fmt.Printf("  %-40s $%-10v score=%.3f\n", m.ProviderID, m.EstimatedPriceUSD, m.Score)
	}

	// --- Natural language resolution --------------------------------------
	// Describe the goal in prose instead of naming an intent type.
	nat, err := client.ResolveNatural(ctx, jarvisclaw.NaturalResolveRequest{
		Query: "I need to turn a paragraph into a short video",
	})
	if err != nil {
		log.Fatal(err)
	}
	if nat.Status == "resolved" {
		fmt.Printf("\nRead as %q (confidence %.2f):\n", nat.Intent, nat.Confidence)
		for i, m := range nat.Matches {
			if i >= 3 {
				break
			}
			fmt.Printf("  %-40s score=%.3f\n", m.ProviderName, m.Score)
		}
	} else if nat.Clarify != nil {
		// Ambiguous phrasing comes back with a clarifying question instead.
		fmt.Println("\nNeeds clarification:", nat.Clarify.Question)
	}

	// --- Network size -----------------------------------------------------
	stats, err := client.NetworkStats(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// BySource is a map because the gateway adds source categories over time.
	fmt.Printf("\n%d providers (%d internal, %d federated) across %d intent types\n",
		stats.TotalProviders, stats.BySource["internal"], stats.BySource["federation"],
		stats.IntentTypes)
	if stats.Federation != nil {
		fmt.Printf("Federation: %d/%d servers healthy, %d resources\n",
			stats.Federation.HealthyServers, stats.Federation.Servers,
			stats.Federation.Resources)
	}

	// --- Executing an intent (spends — uncomment to run) ------------------
	//
	// result, err := client.Execute(ctx, "chat_completion", map[string]any{
	//     "messages":   []map[string]string{{"role": "user", "content": "Hello"}},
	//     "max_tokens": 10,
	// })
	//
	// ExecuteBudget caps total spend and returns settlement detail:
	//
	// result, err := client.ExecuteBudget(ctx, "chat_completion", payload,
	//     map[string]any{"max_total_usd": 0.01})
}
