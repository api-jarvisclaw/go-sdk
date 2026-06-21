package jarvisclaw

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	DefaultNetwork = "eip155:8453"
	USDCContract   = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
)

type paymentInfo struct {
	PayTo             string         `json:"payTo"`
	Amount            string         `json:"amount"`
	Network           string         `json:"network"`
	Asset             string         `json:"asset"`
	MaxTimeoutSeconds int            `json:"maxTimeoutSeconds"`
	Extra             map[string]any `json:"extra"`
}

func parsePaymentRequired(body []byte, preferredNetwork string) (*paymentInfo, error) {
	var resp struct {
		Accepts  []paymentInfo `json:"accepts"`
		Payments []paymentInfo `json:"payments"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	// x402 v2 uses "accepts", v1 uses "payments"
	payments := resp.Accepts
	if len(payments) == 0 {
		payments = resp.Payments
	}
	if len(payments) == 0 {
		return nil, fmt.Errorf("no payment options in 402 response")
	}
	// Prefer the user's configured network if available
	if preferredNetwork != "" {
		for i := range payments {
			if payments[i].Network == preferredNetwork {
				return &payments[i], nil
			}
		}
	}
	// Fall back: prefer any EVM payment option
	for i := range payments {
		if strings.HasPrefix(payments[i].Network, "eip155:") {
			return &payments[i], nil
		}
	}
	return &payments[0], nil
}

func (c *Client) signPayment(payment *paymentInfo, resourceURL string) (string, error) {
	// Safety: this SDK only supports Base chain (eip155:8453) for x402 signing.
	// Reject if the selected payment option is for a different EVM chain to prevent
	// producing an invalid signature (wrong chainId/contract would burn USDC).
	if payment.Network != "" && payment.Network != DefaultNetwork {
		return "", fmt.Errorf("unsupported payment network %q: this SDK only supports %s", payment.Network, DefaultNetwork)
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	nonceHex := "0x" + hex.EncodeToString(nonce)

	now := time.Now().Unix()
	validAfter := now - 60
	validBefore := now + int64(payment.MaxTimeoutSeconds)

	amount, ok := new(big.Int).SetString(payment.Amount, 10)
	if !ok || amount == nil {
		return "", fmt.Errorf("invalid payment amount: %q", payment.Amount)
	}

	// Extract chainId from network string "eip155:<chainId>"
	chainId := int64(8453) // Default to Base
	if strings.HasPrefix(payment.Network, "eip155:") {
		cidStr := strings.TrimPrefix(payment.Network, "eip155:")
		if cid, cidOk := new(big.Int).SetString(cidStr, 10); cidOk {
			chainId = cid.Int64()
		}
	}

	// Use asset address from payment info as the verifying contract (USDC on the target chain)
	verifyingContract := USDCContract
	if payment.Asset != "" {
		verifyingContract = payment.Asset
	}

	// EIP-712 typed data for TransferWithAuthorization
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"TransferWithAuthorization": {
				{Name: "from", Type: "address"},
				{Name: "to", Type: "address"},
				{Name: "value", Type: "uint256"},
				{Name: "validAfter", Type: "uint256"},
				{Name: "validBefore", Type: "uint256"},
				{Name: "nonce", Type: "bytes32"},
			},
		},
		PrimaryType: "TransferWithAuthorization",
		Domain: apitypes.TypedDataDomain{
			Name:              "USD Coin",
			Version:           "2",
			ChainId:           (*math.HexOrDecimal256)(big.NewInt(chainId)),
			VerifyingContract: verifyingContract,
		},
		Message: apitypes.TypedDataMessage{
			"from":        c.address.Hex(),
			"to":          payment.PayTo,
			"value":       amount.String(),
			"validAfter":  fmt.Sprintf("%d", validAfter),
			"validBefore": fmt.Sprintf("%d", validBefore),
			"nonce":       nonce,
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return "", fmt.Errorf("hash domain: %w", err)
	}
	messageHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return "", fmt.Errorf("hash message: %w", err)
	}

	rawData := fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(messageHash))
	hash := crypto.Keccak256Hash([]byte(rawData))

	sig, err := crypto.Sign(hash.Bytes(), c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	sig[64] += 27 // EIP-155 recovery id

	// x402 v2 payload (matches BlockRun format)
	payload := map[string]any{
		"x402Version": 2,
		"resource": map[string]any{
			"url":         resourceURL,
			"description": "API request",
			"mimeType":    "application/json",
		},
		"accepted": map[string]any{
			"scheme":            "exact",
			"network":           payment.Network,
			"amount":            payment.Amount,
			"asset":             payment.Asset,
			"payTo":             payment.PayTo,
			"maxTimeoutSeconds": payment.MaxTimeoutSeconds,
			"extra":             payment.Extra,
		},
		"payload": map[string]any{
			"signature": "0x" + hex.EncodeToString(sig),
			"authorization": map[string]any{
				"from":        c.address.Hex(),
				"to":          payment.PayTo,
				"value":       payment.Amount,
				"validAfter":  fmt.Sprintf("%d", validAfter),
				"validBefore": fmt.Sprintf("%d", validBefore),
				"nonce":       nonceHex,
			},
		},
		"extensions": map[string]any{},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal x402 payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(payloadJSON), nil
}
