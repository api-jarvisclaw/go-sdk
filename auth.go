package jarvisclaw

import "net/http"

// Auth applies authentication to an outgoing HTTP request.
type Auth interface {
	Apply(req *http.Request)
}

// APIKeyAuth authenticates using a bearer token API key.
type APIKeyAuth struct {
	Key string
}

func (a *APIKeyAuth) Apply(req *http.Request) {
	if a.Key != "" {
		req.Header.Set("Authorization", "Bearer "+a.Key)
	}
}

// X402Auth handles x402 payment authentication.
// It is used internally by Client.doRequest when a 402 response is received.
type X402Auth struct {
	client *Client
}

func (a *X402Auth) Apply(req *http.Request) {
	// X402Auth does not add headers upfront; payment is triggered reactively
	// when the server returns 402. See Client.doRequest for the retry logic.
}
