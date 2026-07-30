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
		fmt.Printf("  %-40s %v %s\n", truncate(h.Name, 40), h.SellPrice, h.Currency)
	}

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
