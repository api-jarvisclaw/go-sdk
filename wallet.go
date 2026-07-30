package jarvisclaw

import (
	"context"
	"fmt"
	"strconv"
)

// ChainWallet is one chain's HD deposit address and its on-chain USDC balance.
// USDC is a decimal string with 6 places (e.g. "5.960000"), as returned by the
// server; use ParseFloat if you need a number.
type ChainWallet struct {
	USDC    string `json:"usdc"`
	Address string `json:"address"`
}

// WalletBalance represents the response from GET /v1/wallet/balance.
//
// The balance is the caller's HD wallet on-chain USDC across Base and Solana.
// It deliberately does NOT include the users.quota column: x402 settles against
// the wallet and never debits quota, so reporting quota here overstated the
// spendable balance by the caller's lifetime deposits.
type WalletBalance struct {
	// BalanceUSD is the sum of Base and Solana USDC, as a decimal string.
	BalanceUSD string `json:"balance_usd"`
	Wallets    struct {
		Base   ChainWallet `json:"base"`
		Solana ChainWallet `json:"solana"`
	} `json:"wallets"`
}

// TotalUSD returns BalanceUSD parsed as a float. It returns 0 if the field is
// empty or unparseable, which is what an unauthenticated or errored response
// looks like.
func (w WalletBalance) TotalUSD() float64 {
	v, err := strconv.ParseFloat(w.BalanceUSD, 64)
	if err != nil {
		return 0
	}
	return v
}

// WalletLimits represents per-user spending limits.
//
// IMPORTANT: SetWalletLimits replaces the whole record — the server persists it
// with a full-row write, so any field left at its zero value is stored as zero,
// not left alone. To change one limit, read the current values with WalletLimits
// first, mutate, then write back. UpdateWalletLimit does that for you.
type WalletLimits struct {
	// UserId is assigned by the server from the auth context. It is ignored on
	// write; whatever you send is overwritten with the caller's own id.
	UserId           int     `json:"user_id,omitempty"`
	DailyMaxUSD      float64 `json:"daily_max_usd"`
	PerRequestMaxUSD float64 `json:"per_request_max_usd"`
	MonthlyMaxUSD    float64 `json:"monthly_max_usd"`
	AutoPauseBelow   float64 `json:"auto_pause_below_usd"`
	// PoolAllocation is a JSON object mapping pool names to fractions summing to
	// 1.0, e.g. `{"operations":0.60,"insurance":0.15,"savings":0.15,"dividends":0.10}`.
	// An empty string is accepted (the server skips validation) but clears the
	// stored allocation, which makes GetPools fall back to its defaults.
	PoolAllocation string `json:"pool_allocation,omitempty"`
	// UpdatedAt is set by the server. Read-only.
	UpdatedAt int64 `json:"updated_at,omitempty"`
}

// PoolAllocation is the four-way split of the wallet balance. Values are
// fractions that sum to 1.0.
type PoolAllocation struct {
	Operations float64 `json:"operations"`
	Insurance  float64 `json:"insurance"`
	Savings    float64 `json:"savings"`
	Dividends  float64 `json:"dividends"`
}

// PoolBalances is each pool's share of the on-chain balance, as decimal strings
// with 4 places.
type PoolBalances struct {
	Operations string `json:"operations"`
	Insurance  string `json:"insurance"`
	Savings    string `json:"savings"`
	Dividends  string `json:"dividends"`
}

// WalletPools represents pool allocation ratios and the resulting balances.
//
// Pools are slices of the same on-chain balance WalletBalance reports, not
// separate accounts.
type WalletPools struct {
	Allocation   PoolAllocation `json:"allocation"`
	PoolBalances PoolBalances   `json:"pool_balances"`
}

// WalletBalance retrieves the current wallet balance for the authenticated user.
// GET /v1/wallet/balance — requires auth.
func (c *Client) WalletBalance(ctx context.Context) (*WalletBalance, error) {
	var resp WalletBalance
	if err := c.doJSON(ctx, "GET", "/v1/wallet/balance", nil, &resp); err != nil {
		return nil, fmt.Errorf("wallet balance: %w", err)
	}
	return &resp, nil
}

// WalletLimits retrieves the current spending limits for the authenticated user.
// GET /v1/wallet/limits — requires auth.
func (c *Client) WalletLimits(ctx context.Context) (*WalletLimits, error) {
	var resp WalletLimits
	if err := c.doJSON(ctx, "GET", "/v1/wallet/limits", nil, &resp); err != nil {
		return nil, fmt.Errorf("wallet limits: %w", err)
	}
	return &resp, nil
}

// SetWalletLimits replaces the spending limits for the authenticated user.
// PUT /v1/wallet/limits — requires auth.
//
// This is a full replacement, not a patch: fields left at zero are stored as
// zero. Use UpdateWalletLimit to change a single limit safely.
func (c *Client) SetWalletLimits(ctx context.Context, limits WalletLimits) error {
	if err := c.doJSON(ctx, "PUT", "/v1/wallet/limits", limits, nil); err != nil {
		return fmt.Errorf("set wallet limits: %w", err)
	}
	return nil
}

// UpdateWalletLimit reads the current limits, applies mutate, and writes the
// result back — the read-modify-write the replacing PUT requires.
//
// Example:
//
//	err := c.UpdateWalletLimit(ctx, func(l *jarvisclaw.WalletLimits) {
//	    l.DailyMaxUSD = 30
//	})
func (c *Client) UpdateWalletLimit(ctx context.Context, mutate func(*WalletLimits)) error {
	current, err := c.WalletLimits(ctx)
	if err != nil {
		return fmt.Errorf("update wallet limit: %w", err)
	}
	mutate(current)
	return c.SetWalletLimits(ctx, *current)
}

// WalletPools retrieves pool allocation and current balances for the authenticated user.
// GET /v1/wallet/pools — requires auth.
func (c *Client) WalletPools(ctx context.Context) (*WalletPools, error) {
	var resp WalletPools
	if err := c.doJSON(ctx, "GET", "/v1/wallet/pools", nil, &resp); err != nil {
		return nil, fmt.Errorf("wallet pools: %w", err)
	}
	return &resp, nil
}

// TransactionHistory represents paginated transaction history.
type TransactionHistory struct {
	Transactions []Transaction `json:"transactions"`
	Total        int           `json:"total"`
	Page         int           `json:"page"`
}

// Transaction represents a single billing transaction.
type Transaction struct {
	ID             int    `json:"id"`
	AmountQuota    int    `json:"amount_quota"`
	Category       string `json:"category"`
	Model          string `json:"model,omitempty"`
	UseTimeSeconds int    `json:"use_time_seconds,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

// WalletHistory retrieves paginated transaction history.
// GET /v1/wallet/history — requires auth.
func (c *Client) WalletHistory(ctx context.Context, page, pageSize int) (*TransactionHistory, error) {
	var resp TransactionHistory
	path := fmt.Sprintf("/v1/wallet/history?page=%d&page_size=%d", page, pageSize)
	if err := c.doJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("wallet history: %w", err)
	}
	return &resp, nil
}
