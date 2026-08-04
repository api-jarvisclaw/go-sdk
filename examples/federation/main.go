// Federation — discover peer gateways and search their advertised resources.
//
// The public registry endpoints used here need no auth. Peer management
// (AddFederationPeer, RemoveFederationPeer, FederationCrawl) is admin-only and
// is left commented out.
//
// Run: go run ./examples/federation
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

	// --- Who is in the network? -------------------------------------------
	// Server and resource listings are paginated (page, pageSize) and also return
	// the total count, so you can tell how much more there is to fetch.
	servers, totalServers, err := client.ListFederationServers(ctx, 1, 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Federation servers (%d total, showing %d):\n", totalServers, len(servers))
	for _, s := range servers {
		health := "down"
		if s.Healthy {
			health = "up"
		}
		fmt.Printf("  %-34s %-5s %d resources\n", s.Name, health, s.ResourceCount)
	}

	// --- What can they do? ------------------------------------------------
	resources, totalResources, err := client.ListFederationResources(ctx, 1, 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nResources (%d total, showing %d):\n", totalResources, len(resources))
	for _, r := range resources {
		fmt.Printf("  %-40s %s %s\n", truncate(r.Name, 40), r.Method, truncate(r.Path, 30))
	}

	// --- Search across every peer at once ---------------------------------
	hits, err := client.SearchFederation(ctx, jarvisclaw.FederationSearchParams{
		Query: "video generation",
		Limit: 5,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nSearch hits for \"video generation\": %d\n", len(hits))
	for _, h := range hits {
		fmt.Printf("  #%-6d %-40s %v %s\n", h.ResourceID, truncate(h.Name, 40), h.SellPrice, h.Currency)
	}

	// --- The marketplace view of the same capacity ------------------------
	// Priced in marketplace terms, unpriced (therefore uncallable) rows excluded,
	// and it hands back the category counts so you can build a filter without
	// paging the whole catalogue.
	page, err := client.ListAPIs(ctx, jarvisclaw.CatalogueParams{PageSize: 5, Keyword: "qr"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nCatalogue matches for \"qr\": %d total\n", page.Total)
	for _, it := range page.Items {
		fmt.Printf("  #%-6d %-30s $%v/%s\n", it.ResourceID, truncate(it.Name, 30), it.DisplayPrice, it.PriceUnit)
	}

	// --- Discovery to invocation ------------------------------------------
	// ResourceID from any listing is the handle both call paths take. Both settle
	// on-chain, so they are commented out rather than run: they spend real USDC.
	//
	// CallAPI keeps execute's envelope, which is where tx_hash and cost_usd
	// live, and reports an upstream error as success=false rather than a Go error:
	//
	//   out, err := client.CallAPI(ctx, hits[0].ResourceID, map[string]any{"prompt": "a cat"})
	//   if out["success"] != true { /* charged, upstream refused */ }
	//
	// InvokeAPI hands back the upstream body with no envelope:
	//
	//   raw, err := client.InvokeAPI(ctx, hits[0].ResourceID, map[string]any{"prompt": "a cat"})

	// --- Admin-only operations (need an admin session — commented out) -----
	//
	// peers, err := client.FederationPeers(ctx)
	// added, err := client.AddFederationPeer(ctx, "https://peer.example.com")
	// err = client.RemoveFederationPeer(ctx, "peer.example.com")
	// result, err := client.FederationCrawl(ctx)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
