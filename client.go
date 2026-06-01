// Package jarvisclaw provides an x402-enabled HTTP client for JarvisClaw APIs.
//
// Usage:
//
//	client, _ := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey("0x..."))
//	resp, _ := client.Post("/v1/chat/completions", map[string]any{
//	    "model": "openai/gpt-5.4-nano",
//	    "messages": []map[string]string{{"role": "user", "content": "Hello"}},
//	})
//	fmt.Println(resp)
package jarvisclaw

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	DefaultBaseURL = "https://api.jarvisclaw.ai"
	DefaultNetwork = "eip155:8453"
	USDCContract   = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	Version        = "0.1.1"
)

type Client struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
	baseURL    string
	network    string
	httpClient *http.Client
}

type Option func(*Client)

func WithPrivateKey(hexKey string) Option {
	return func(c *Client) {
		if len(hexKey) > 2 && hexKey[:2] == "0x" {
			hexKey = hexKey[2:]
		}
		key, err := crypto.HexToECDSA(hexKey)
		if err == nil {
			c.privateKey = key
			c.address = crypto.PubkeyToAddress(key.PublicKey)
		}
	}
}

func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

func WithNetwork(network string) Option {
	return func(c *Client) { c.network = network }
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    DefaultBaseURL,
		network:    DefaultNetwork,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.privateKey == nil {
		return nil, fmt.Errorf("private key is required")
	}
	return c, nil
}

func (c *Client) Address() string {
	return c.address.Hex()
}

func (c *Client) Get(path string, params map[string]string) (map[string]any, error) {
	return c.request("GET", path, params, nil)
}

func (c *Client) Post(path string, body any) (map[string]any, error) {
	return c.request("POST", path, nil, body)
}

func (c *Client) request(method, path string, params map[string]string, body any) (map[string]any, error) {
	url := c.baseURL + path
	if len(params) > 0 {
		url += "?"
		for k, v := range params {
			url += k + "=" + v + "&"
		}
		url = url[:len(url)-1]
	}

	var reqBody io.Reader
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		return c.parseResponse(resp)
	}

	// Handle x402 payment
	respBody, _ := io.ReadAll(resp.Body)
	paymentReq, err := c.parsePaymentRequired(respBody)
	if err != nil {
		return nil, fmt.Errorf("parse 402: %w", err)
	}

	signature, err := c.signPayment(paymentReq, url)
	if err != nil {
		return nil, fmt.Errorf("sign payment: %w", err)
	}

	// Retry with signature
	var retryBody io.Reader
	if bodyBytes != nil {
		retryBody = bytes.NewReader(bodyBytes)
	}
	retryReq, _ := http.NewRequest(method, url, retryBody)
	retryReq.Header.Set("Content-Type", "application/json")
	retryReq.Header.Set("PAYMENT-SIGNATURE", signature)

	retryResp, err := c.httpClient.Do(retryReq)
	if err != nil {
		return nil, err
	}
	defer retryResp.Body.Close()

	return c.parseResponse(retryResp)
}

func (c *Client) parseResponse(resp *http.Response) (map[string]any, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return result, nil
}

type paymentInfo struct {
	PayTo             string `json:"payTo"`
	Amount            string `json:"amount"`
	Network           string `json:"network"`
	MaxTimeoutSeconds int    `json:"maxTimeoutSeconds"`
}

func (c *Client) parsePaymentRequired(body []byte) (*paymentInfo, error) {
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
