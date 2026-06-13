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

// GetBalance returns the current balance in USD.
// x402 mode: queries on-chain USDC balance via public Base RPC.
// API Key mode: queries account quota from the server.
func (c *Client) GetBalance(ctx context.Context) (float64, error) {
	if c.privateKey != nil {
		return c.queryOnchainBalance(ctx)
	}
	// API Key mode — query server
	raw, err := c.doGetCtx(ctx, "/api/user/self", nil)
	if err != nil {
		return 0, err
	}
	data, _ := raw["data"].(map[string]any)
	if data == nil {
		return 0, nil
	}
	quota, _ := data["quota"].(float64)
	return quota / 500000.0, nil
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
