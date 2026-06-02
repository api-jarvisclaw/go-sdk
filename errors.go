package jarvisclaw

import "fmt"

// JarvisClawError is the base error type.
type JarvisClawError struct {
	Message string
}

func (e *JarvisClawError) Error() string { return e.Message }

// APIError represents an HTTP error from the API.
type APIError struct {
	StatusCode int
	Message    string
	Body       map[string]any
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%d] %s", e.StatusCode, e.Message)
}

// AuthenticationError is a 401 error.
type AuthenticationError struct{ APIError }

// RateLimitError is a 429 error.
type RateLimitError struct{ APIError }

// InsufficientBalanceError is a 402 error (API Key mode).
type InsufficientBalanceError struct{ APIError }

// PaymentError is an x402 signing/settlement failure.
type PaymentError struct{ JarvisClawError }
