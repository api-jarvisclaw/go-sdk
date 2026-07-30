// Analytics — spend aggregation, quality metrics, and insights.
//
// All of these read /api/analytics/aggregate with a different grouping. Scope is
// enforced server-side from your auth context, so you only see your own figures.
//
// Run: go run ./examples/analytics
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

	client, err := jarvisclaw.NewClient(
		jarvisclaw.WithAPIKey(os.Getenv("JARVISCLAW_API_KEY")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// --- Spend, grouped however you need ----------------------------------
	spend, err := client.Spend(ctx, jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Spend rows (7d): %d\n", len(spend))
	for i, row := range spend {
		if i >= 5 {
			break
		}
		fmt.Printf("  %-40s $%v\n", row.Model, row.RevenueUSD)
	}

	// CostByModel and DailyTrend are the same endpoint with group_by preset.
	byModel, err := client.CostByModel(ctx, jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nBy model: %d rows\n", len(byModel))

	trend, err := client.DailyTrend(ctx, jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Daily trend: %d days\n", len(trend))

	// --- Quality metrics --------------------------------------------------
	// This returns one row per model, not a single summary object.
	quality, err := client.QualityMetrics(ctx, jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nQuality rows: %d\n", len(quality))
	for i, q := range quality {
		if i >= 5 {
			break
		}
		fmt.Printf("  %-38s %6d req  err %.1f%%  frt %.0fms  cache %.1f%%\n",
			q.Model, q.Requests, q.ErrorRate*100, q.AvgFrtMs, q.CacheHitRate*100)
	}

	// --- Insights ---------------------------------------------------------
	// Returned untyped: the payload is a rollup whose shape the gateway evolves,
	// so pinning it to a struct would guarantee drift.
	insights, err := client.Insights(ctx, jarvisclaw.AnalyticsParams{Period: "7d"})
	if err != nil {
		log.Fatal(err)
	}
	keys := make([]string, 0, len(insights))
	for k := range insights {
		keys = append(keys, k)
	}
	fmt.Printf("\nInsights keys: %v\n", keys)

	// --- Audit trail ------------------------------------------------------
	audit, err := client.Audit(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nAudit entries: %d\n", audit.Count)
	for i, e := range audit.Entries {
		if i >= 3 {
			break
		}
		fmt.Printf("  %s  %-22s %s\n", e.Timestamp, e.EventType, e.RequestID)
	}
}
