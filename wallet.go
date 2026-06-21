package jarvisclaw

import (
	"context"
	"fmt"
)

// WalletBalance represents the response from GET /v1/wallet/balance.
type WalletBalance struct {
	Quota    int    `json:"quota"`
	QuotaUSD string `json:"quota_usd"`
	HDWallet struct {
		BaseUSDC   string `json:"base_usdc"`
		SolanaUSDC string `json:"solana_usdc"`
	} `json:"hd_wallet"`
	Subscription struct {
		Active         bool  `json:"active"`
		RemainingQuota int64 `json:"remaining_quota"`
	} `json:"subscription"`
	TotalUSD string `json:"total_usd"`
}

// WalletLimits represents per-user spending limits.
type WalletLimits struct {
	DailyMaxUSD      float64 `json:"daily_max_usd"`
	PerRequestMaxUSD float64 `json:"per_request_max_usd"`
	MonthlyMaxUSD    float64 `json:"monthly_max_usd"`
	AutoPauseBelow   float64 `json:"auto_pause_below_usd"`
	// PoolAllocation is an optional JSON object mapping pool names to fractions (must sum to 1.0).
	// Example: `{"operations":0.60,"insurance":0.15,"savings":0.15,"dividends":0.10}`
	// Leave empty to keep the existing allocation unchanged.
	PoolAllocation string `json:"pool_allocation,omitempty"`
}

// WalletPools represents pool allocation percentages and current pool balances.
type WalletPools struct {
	Allocation   map[string]float64 `json:"allocation"`
	PoolBalances map[string]string  `json:"pool_balances"`
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

// SetWalletLimits updates the spending limits for the authenticated user.
// PUT /v1/wallet/limits — requires auth.
func (c *Client) SetWalletLimits(ctx context.Context, limits WalletLimits) error {
	if err := c.doJSON(ctx, "PUT", "/v1/wallet/limits", limits, nil); err != nil {
		return fmt.Errorf("set wallet limits: %w", err)
	}
	return nil
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
