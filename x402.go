package jarvisclaw

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
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
	PayTo             string `json:"payTo"`
	Amount            string `json:"amount"`
	Network           string `json:"network"`
	MaxTimeoutSeconds int    `json:"maxTimeoutSeconds"`
}

func parsePaymentRequired(body []byte) (*paymentInfo, error) {
	var resp struct {
		Payments []paymentInfo `json:"payments"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Payments) == 0 {
		return nil, fmt.Errorf("no payment options in 402 response")
	}
	return &resp.Payments[0], nil
}

func (c *Client) signPayment(payment *paymentInfo, resourceURL string) (string, error) {
	nonce := make([]byte, 32)
	rand.Read(nonce)
	nonceHex := "0x" + hex.EncodeToString(nonce)

	now := time.Now().Unix()
	validAfter := now - 60
	validBefore := now + int64(payment.MaxTimeoutSeconds)

	amount, _ := new(big.Int).SetString(payment.Amount, 10)

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
			ChainId:           (*math.HexOrDecimal256)(big.NewInt(8453)),
			VerifyingContract: USDCContract,
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

	// Build payload
	payload := map[string]any{
		"x402Version": 1,
		"scheme":      "exact",
		"network":     payment.Network,
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
		"resource": resourceURL,
	}

	payloadJSON, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(payloadJSON), nil
}
