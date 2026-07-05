// Package jarvisclaw provides a Go client for the JarvisClaw AI API.
//
// It supports both API key authentication and x402 USDC micropayment authentication.
//
// Usage with API key:
//
//	c, _ := jarvisclaw.NewClient(jarvisclaw.WithAPIKey("sk-..."))
//	text, _ := c.Chat(ctx, "openai/gpt-4o-mini", "Hello")
//
// Usage with x402 wallet:
//
//	c, _ := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey("0x..."))
//	text, _ := c.Chat(ctx, "openai/gpt-4o-mini", "Hello")
package jarvisclaw

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	DefaultBaseURL = "https://api.jarvisclaw.ai"
	Version        = "0.10.0"

	maxRetries = 3
)

var retryStatusCodes = map[int]bool{
	429: true, 500: true, 502: true, 503: true, 504: true,
}

func sleepWithBackoff(attempt int) {
	delay := time.Duration(1<<uint(attempt)) * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	time.Sleep(delay)
}

// isRetryableNetworkError returns true for transient network errors worth retrying.
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// net/http wraps errors in *url.Error
	if ue, ok := err.(*url.Error); ok {
		err = ue.Err
	}
	// Retry on timeout or connection refused/reset
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "broken pipe")
}

// Client is the JarvisClaw API client.
type Client struct {
	apiKey     string
	privateKey *ecdsa.PrivateKey
	address    common.Address
	baseURL    string
	network    string
	httpClient *http.Client
	initErr    error
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key for bearer-token authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithPrivateKey sets the Ethereum private key for x402 payment authentication.
func WithPrivateKey(hexKey string) Option {
	return func(c *Client) {
		clean := strings.TrimPrefix(hexKey, "0x")
		key, err := crypto.HexToECDSA(clean)
		if err != nil {
			c.initErr = err
			return
		}
		c.privateKey = key
		c.address = crypto.PubkeyToAddress(key.PublicKey)
	}
}

// WithBaseURL overrides the default base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithNetwork sets the x402 payment network (default: "eip155:8453").
func WithNetwork(network string) Option {
	return func(c *Client) { c.network = network }
}

// NewClient creates a new JarvisClaw client.
// Falls back to JARVISCLAW_API_KEY, JARVISCLAW_WALLET_KEY, JARVISCLAW_BASE_URL env vars.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    DefaultBaseURL,
		network:    DefaultNetwork,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	// Apply env var defaults before options
	if v := os.Getenv("JARVISCLAW_API_KEY"); v != "" {
		c.apiKey = v
	}
	if v := os.Getenv("JARVISCLAW_WALLET_KEY"); v != "" {
		WithPrivateKey(v)(c)
	}
	if v := os.Getenv("JARVISCLAW_BASE_URL"); v != "" {
		c.baseURL = v
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.initErr != nil {
		return nil, fmt.Errorf("invalid private key: %w", c.initErr)
	}

	if c.apiKey == "" && c.privateKey == nil {
		return nil, fmt.Errorf("jarvisclaw: authentication required — provide WithAPIKey or WithPrivateKey (or set JARVISCLAW_API_KEY / JARVISCLAW_WALLET_KEY)")
	}
	return c, nil
}

// Address returns the Ethereum address derived from the private key, or empty string.
func (c *Client) Address() string {
	if c.privateKey == nil {
		return ""
	}
	return c.address.Hex()
}

// ── Internal request helpers ─────────────────────────────────────────────────

func (c *Client) applyAuth(req *http.Request) {
	req.Header.Set("User-Agent", "jarvisclaw-go/"+Version)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// doGetCtx performs a GET request with context and returns parsed JSON.
func (c *Client) doGetCtx(ctx context.Context, path string, params map[string]string) (map[string]any, error) {
	url := c.buildURL(path, params)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)
	return c.executeJSON(req, nil)
}

// doPostCtx performs a POST request with context and returns parsed JSON.
func (c *Client) doPostCtx(ctx context.Context, path string, body any) (map[string]any, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	u := c.buildURL(path, nil)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)
	return c.executeJSON(req, bodyBytes)
}

// doPostRawCtx performs a POST request with context and returns the raw HTTP response (caller must close Body).
func (c *Client) doPostRawCtx(ctx context.Context, path string, body any) (*http.Response, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	u := c.buildURL(path, nil)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)
	return c.executeRaw(req, bodyBytes)
}

// ── Execution core ───────────────────────────────────────────────────────────

// executeJSON runs the request, handles 402 x402 retry, retries on 429/5xx and network errors, and parses JSON.
func (c *Client) executeJSON(req *http.Request, bodyBytes []byte) (map[string]any, error) {
	var lastErr error
	lastStatusCode := 0
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			sleepWithBackoff(attempt)
			// Need a fresh request since body was consumed
			var err error
			req, err = c.cloneRequest(req, bodyBytes)
			if err != nil {
				return nil, err
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isRetryableNetworkError(err) && attempt < maxRetries {
				lastErr = err
				continue
			}
			return nil, err
		}

		if resp.StatusCode == http.StatusPaymentRequired {
			body402, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return c.handle402JSON(req, bodyBytes, body402)
		}

		if retryStatusCodes[resp.StatusCode] && attempt < maxRetries {
			lastStatusCode = resp.StatusCode
			resp.Body.Close()
			continue
		}

		defer resp.Body.Close()
		return c.parseJSONResponse(resp)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
	}
	return nil, &APIError{StatusCode: lastStatusCode, Message: fmt.Sprintf("request failed after %d retries (last status %d)", maxRetries, lastStatusCode)}
}

// executeRaw runs the request, handles 402 x402 retry, retries on 429/5xx and network errors, and returns raw response.
func (c *Client) executeRaw(req *http.Request, bodyBytes []byte) (*http.Response, error) {
	var lastErr error
	lastStatusCode := 0
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			sleepWithBackoff(attempt)
			var err error
			req, err = c.cloneRequest(req, bodyBytes)
			if err != nil {
				return nil, err
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isRetryableNetworkError(err) && attempt < maxRetries {
				lastErr = err
				continue
			}
			return nil, err
		}

		if resp.StatusCode == http.StatusPaymentRequired {
			body402, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return c.handle402Raw(req, bodyBytes, body402)
		}

		if retryStatusCodes[resp.StatusCode] && attempt < maxRetries {
			lastStatusCode = resp.StatusCode
			resp.Body.Close()
			continue
		}

		if resp.StatusCode >= 400 {
			defer resp.Body.Close()
			return nil, c.buildError(resp)
		}
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
	}
	return nil, &APIError{StatusCode: lastStatusCode, Message: fmt.Sprintf("request failed after %d retries (last status %d)", maxRetries, lastStatusCode)}
}

// handle402JSON handles a 402 Payment Required by signing an x402 payment and retrying.
func (c *Client) handle402JSON(orig *http.Request, bodyBytes []byte, body402 []byte) (map[string]any, error) {
	if c.privateKey == nil {
		return nil, &InsufficientBalanceError{APIError{StatusCode: 402, Message: "payment required — provide WithPrivateKey for x402 payments"}}
	}

	payment, err := parsePaymentRequired(body402, c.network)
	if err != nil {
		return nil, &PaymentError{JarvisClawError{Message: fmt.Sprintf("parse 402: %s", err)}}
	}

	sig, err := c.signPayment(payment, orig.URL.String())
	if err != nil {
		return nil, &PaymentError{JarvisClawError{Message: fmt.Sprintf("sign payment: %s", err)}}
	}

	retryReq, err := c.cloneRequest(orig, bodyBytes)
	if err != nil {
		return nil, err
	}
	retryReq.Header.Set("PAYMENT-SIGNATURE", sig)

	retryResp, err := c.httpClient.Do(retryReq)
	if err != nil {
		return nil, err
	}
	defer retryResp.Body.Close()
	return c.parseJSONResponse(retryResp)
}

// handle402Raw handles x402 for raw/streaming requests.
func (c *Client) handle402Raw(orig *http.Request, bodyBytes []byte, body402 []byte) (*http.Response, error) {
	if c.privateKey == nil {
		return nil, &InsufficientBalanceError{APIError{StatusCode: 402, Message: "payment required — provide WithPrivateKey for x402 payments"}}
	}

	payment, err := parsePaymentRequired(body402, c.network)
	if err != nil {
		return nil, &PaymentError{JarvisClawError{Message: fmt.Sprintf("parse 402: %s", err)}}
	}

	sig, err := c.signPayment(payment, orig.URL.String())
	if err != nil {
		return nil, &PaymentError{JarvisClawError{Message: fmt.Sprintf("sign payment: %s", err)}}
	}

	retryReq, err := c.cloneRequest(orig, bodyBytes)
	if err != nil {
		return nil, err
	}
	retryReq.Header.Set("PAYMENT-SIGNATURE", sig)

	retryResp, err := c.httpClient.Do(retryReq)
	if err != nil {
		return nil, err
	}
	if retryResp.StatusCode >= 400 {
		defer retryResp.Body.Close()
		return nil, c.buildError(retryResp)
	}
	return retryResp, nil
}

func (c *Client) cloneRequest(orig *http.Request, bodyBytes []byte) (*http.Request, error) {
	var body io.Reader
	if bodyBytes != nil {
		body = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(orig.Context(), orig.Method, orig.URL.String(), body)
	if err != nil {
		return nil, err
	}
	// Copy headers
	for k, vv := range orig.Header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	return req, nil
}

func (c *Client) parseJSONResponse(resp *http.Response) (map[string]any, error) {
	if resp.StatusCode >= 400 {
		return nil, c.buildError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result, nil
}

func (c *Client) buildError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	msg := string(body)

	var bodyMap map[string]any
	json.Unmarshal(body, &bodyMap) //nolint:errcheck

	// Try to extract message from JSON error body
	if bodyMap != nil {
		if errObj, ok := bodyMap["error"].(map[string]any); ok {
			if m, ok := errObj["message"].(string); ok {
				msg = m
			}
		} else if m, ok := bodyMap["message"].(string); ok {
			msg = m
		}
	}

	base := APIError{StatusCode: resp.StatusCode, Message: msg, Body: bodyMap}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthenticationError{base}
	case http.StatusPaymentRequired:
		return &InsufficientBalanceError{base}
	case http.StatusTooManyRequests:
		return &RateLimitError{base}
	default:
		return &base
	}
}

// doPostInto performs a POST with context and unmarshals the JSON response into dest.
// dest must be a pointer (e.g. *[]map[string]any).
func (c *Client) doPostInto(ctx context.Context, path string, body any, dest any) error {
	raw, err := c.doPostCtx(ctx, path, body)
	if err != nil {
		// doPostCtx already returns an error for non-2xx — propagate as-is.
		// For batch responses the server may return a JSON array, which
		// parseJSONResponse cannot decode into map[string]any. In that case
		// we need a lower-level path; see the note below.
		return err
	}
	// Fast path: dest is *map[string]any — just assign.
	if mp, ok := dest.(*map[string]any); ok {
		*mp = raw
		return nil
	}
	// General path: round-trip through JSON to decode into arbitrary dest.
	b, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("doPostInto: re-marshal: %w", err)
	}
	return json.Unmarshal(b, dest)
}

// doPostRawBytes performs a POST with context and returns the raw response body bytes.
// Used when the response is not a JSON object (e.g., a JSON array for batch RPC).
// Goes through the retry/x402 execution path for resilience.
func (c *Client) doPostRawBytes(ctx context.Context, path string, body any) ([]byte, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	u := c.buildURL(path, nil)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	resp, err := c.executeRaw(req, bodyBytes)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// doJSON performs a request with optional JSON body and unmarshals the JSON response into dest.
// method: "GET", "POST", "PUT", etc.
// body: marshaled as JSON request body (pass nil for GET).
// dest: pointer to target struct (pass nil to discard response body).
func (c *Client) doJSON(ctx context.Context, method, path string, body any, dest any) error {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
	}

	u := c.buildURL(path, nil)
	var reqBody io.Reader
	if bodyBytes != nil {
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return err
	}
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyAuth(req)

	resp, err := c.executeRaw(req, bodyBytes)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if dest == nil {
		return nil
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(respBytes, dest); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// doRawJSON is like doJSON but returns the raw response bytes instead of decoding.
func (c *Client) doRawJSON(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
	}

	u := c.buildURL(path, nil)
	var reqBody io.Reader
	if bodyBytes != nil {
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, err
	}
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyAuth(req)

	resp, err := c.executeRaw(req, bodyBytes)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return json.RawMessage(respBytes), nil
}

func (c *Client) buildURL(path string, params map[string]string) string {
	u := c.baseURL + path
	if len(params) == 0 {
		return u
	}
	v := url.Values{}
	for key, val := range params {
		v.Set(key, val)
	}
	return u + "?" + v.Encode()
}
