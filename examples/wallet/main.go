// Wallet — on-chain balance, transaction history, and spending limits.
//
// Read-only. The limit-changing section is commented out.
//
// Run: go run ./examples/wallet
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

	// --- Balance ----------------------------------------------------------
	// This is the HD wallet's on-chain USDC across Base and Solana. It
	// deliberately excludes account quota: x402 settles against the wallet and
	// never debits quota, so folding quota in would overstate what is spendable.
	bal, err := client.WalletBalance(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Total USDC:", bal.BalanceUSD)
	fmt.Printf("  base    %12s  %s\n", bal.Wallets.Base.USDC, bal.Wallets.Base.Address)
	fmt.Printf("  solana  %12s  %s\n", bal.Wallets.Solana.USDC, bal.Wallets.Solana.Address)

	// TotalUSD parses BalanceUSD to a float for arithmetic. The wire format is a
	// decimal string, so this cannot be a plain float64 field without losing
	// precision on the way in.
	fmt.Printf("As float: %.6f\n", bal.TotalUSD())

	// --- History ----------------------------------------------------------
	// Page is 1-based; page size caps at 100.
	hist, err := client.WalletHistory(ctx, 1, 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n%d transactions total. Most recent %d:\n",
		hist.Total, len(hist.Transactions))
	for _, tx := range hist.Transactions {
		model := tx.Model
		if model == "" {
			model = "-"
		}
		// AmountQuota is negated spend, so it is normally negative.
		fmt.Printf("  #%-6d %-12s %10d  %s\n", tx.ID, tx.Category, tx.AmountQuota, model)
	}

	// --- Limits -----------------------------------------------------------
	limits, err := client.WalletLimits(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nLimits: daily $%.2f | per-request $%.2f | monthly $%.2f | auto-pause below $%.2f\n",
		limits.DailyMaxUSD, limits.PerRequestMaxUSD, limits.MonthlyMaxUSD, limits.AutoPauseBelow)

	// --- Treasury pools ---------------------------------------------------
	pools, err := client.WalletPools(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nPool allocation: %+v\n", pools.Allocation)
	fmt.Printf("Pool balances:   %+v\n", pools.PoolBalances)

	// --- Changing limits (mutates state — uncomment to run) ---------------
	//
	// The endpoint replaces the whole record, so UpdateWalletLimit reads the
	// current limits, hands them to your callback, and writes the result back.
	// Calling SetWalletLimits with one field set would zero the others.
	//
	// updated, err := client.UpdateWalletLimit(ctx, func(l *jarvisclaw.WalletLimits) {
	//     l.DailyMaxUSD = 25.0
	// })
}
