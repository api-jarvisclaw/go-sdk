package jarvisclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

const (
	baseRPCURL   = "https://mainnet.base.org"
	usdcContract = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
)

// GetBalance returns the current spendable balance in USD.
//
// x402 mode: queries the wallet's on-chain USDC balance on Base via public RPC.
// Only Base is checked — this SDK signs Base payments only.
//
// API Key mode: reads the OpenAI-compatible billing endpoint
// GET /v1/dashboard/billing/subscription. When the account has an HD deposit
// wallet the gateway reports the real on-chain balance there; otherwise it
// reports the ledger quota converted to USD.
//
// Note this is NOT /api/user/self: that route requires a dashboard session or
// access token plus a New-Api-User header, which an API key cannot satisfy.
func (c *Client) GetBalance(ctx context.Context) (float64, error) {
	if c.privateKey != nil {
		return c.queryOnchainBalance(ctx)
	}
	var resp struct {
		HardLimitUSD float64 `json:"hard_limit_usd"`
		Error        *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	// This endpoint answers 200 with an {"error":{...}} body on failure instead of
	// a 4xx, so the status check in executeRaw cannot catch it.
	if err := c.doGetInto(ctx, "/v1/dashboard/billing/subscription", nil, &resp); err != nil {
		return 0, err
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return 0, &APIError{StatusCode: 200, Message: resp.Error.Message}
	}
	return resp.HardLimitUSD, nil
}

func (c *Client) queryOnchainBalance(ctx context.Context) (float64, error) {
	// balanceOf(address) = 0x70a08231 + address padded to 32 bytes
	addr := strings.ToLower(c.address.Hex()[2:])
	callData := "0x70a08231" + fmt.Sprintf("%064s", addr)

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_call",
		"params":  []any{map[string]string{"to": usdcContract, "data": callData}, "latest"},
		"id":      1,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseRPCURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	result := strings.TrimPrefix(rpcResp.Result, "0x")
	if result == "" {
		return 0, nil
	}
	balance := new(big.Int)
	if _, ok := balance.SetString(result, 16); !ok {
		return 0, fmt.Errorf("invalid balance hex from RPC: %q", rpcResp.Result)
	}

	// USDC has 6 decimals
	f := new(big.Float).SetInt(balance)
	divisor := new(big.Float).SetInt64(1_000_000)
	f.Quo(f, divisor)
	usd, _ := f.Float64()
	return usd, nil
}
