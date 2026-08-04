package jarvisclaw_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	jarvisclaw "github.com/api-jarvisclaw/go-sdk/v2"
)

// This SDK could only pay with an EVM key: WithPrivateKey parsed secp256k1 hex only, so a
// base58 Solana key failed at construction with "invalid hex character". The Python SDK
// has accepted both rails for some time, so "wallet mode" meant different things in the
// two SDKs, and a Solana wallet holder could not use this one at all.

// solanaTestKey is a deterministic throwaway keypair. It holds nothing; the tests only
// need a key that parses and signs.
func solanaTestKey(t *testing.T) (base58Key string, pub solana.PublicKey) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return base58.Encode(priv), solana.PublicKeyFromBytes(priv.Public().(ed25519.PublicKey))
}

func TestWithPrivateKeyAcceptsASolanaKey(t *testing.T) {
	key, pub := solanaTestKey(t)

	c, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(key))
	if err != nil {
		t.Fatalf("a base58 Solana key must be accepted: %v", err)
	}
	if got := c.Address(); got != pub.String() {
		t.Errorf("Address() = %q, want the base58 pubkey %q", got, pub.String())
	}
}

func TestWithPrivateKeyStillAcceptsAnEVMKey(t *testing.T) {
	// Detection must not regress the existing rail. A 64-char hex string is also valid
	// base58, so hex has to win — otherwise every EVM key would be read as Solana.
	const evmKey = "4c0883a69102937d6231471b5dbb6204fe512961708279f2e3f1a1a1f1f1f1f1"
	for _, k := range []string{evmKey, "0x" + evmKey} {
		c, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(k))
		if err != nil {
			t.Fatalf("EVM key %q rejected: %v", k, err)
		}
		if !strings.HasPrefix(c.Address(), "0x") {
			t.Errorf("Address() = %q, want a 0x-prefixed EVM address", c.Address())
		}
	}
}

func TestWithPrivateKeyRejectsGarbage(t *testing.T) {
	for _, k := range []string{"", "   ", "not-a-key", "zzzz"} {
		if _, err := jarvisclaw.NewClient(jarvisclaw.WithPrivateKey(k)); err == nil {
			t.Errorf("NewClient(WithPrivateKey(%q)): expected an error", k)
		}
	}
}

// solanaStub is a gateway that answers 402 with a Solana challenge and then accepts the
// signed retry, plus the RPC endpoint the signer needs for a blockhash.
type solanaStub struct {
	t          *testing.T
	calls      int
	seenSig    string
	offerRails []string
}

func (s *solanaStub) start() *httptest.Server {
	mux := http.NewServeMux()

	// Solana RPC: only getLatestBlockhash is needed.
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// A real base58 blockhash, so HashFromBase58 succeeds.
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{` +
			`"blockhash":"9zqCPWSHY9BLPjcPFhvXsjMDW1vZVJ5nZ5nZ5nZ5nZ5n","lastValidBlockHeight":1}}}`))
	})

	mux.HandleFunc("/v1/marketplace/api/476", func(w http.ResponseWriter, r *http.Request) {
		s.calls++
		if sig := r.Header.Get("PAYMENT-SIGNATURE"); sig != "" {
			s.seenSig = sig
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"decoded":["ok"]}`))
			return
		}

		accepts := make([]string, 0, len(s.offerRails))
		for _, rail := range s.offerRails {
			switch rail {
			case "solana":
				accepts = append(accepts, `{"scheme":"exact",`+
					`"network":"solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",`+
					`"asset":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",`+
					`"amount":"5750",`+
					`"payTo":"3xxDCjN8s6MgNHwdRExRLa6gHmmRTWPnUdzkbKfEgNAF",`+
					`"maxTimeoutSeconds":300,`+
					`"extra":{"feePayer":"6WrbjPvE6BiabTHhNz9U4XZFTZs2sYnfNAGjr5AzRy1c"}}`)
			case "base":
				accepts = append(accepts, `{"scheme":"exact","network":"eip155:8453",`+
					`"asset":"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",`+
					`"maxAmountRequired":"5750",`+
					`"payTo":"0xDC59fa7b64988B846e76eC9849bb68f889071506",`+
					`"maxTimeoutSeconds":300,"extra":{"name":"USD Coin","version":"2"}}`)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"x402Version":2,"accepts":[` + strings.Join(accepts, ",") + `]}`))
	})

	srv := httptest.NewServer(mux)
	s.t.Cleanup(srv.Close)
	return srv
}

func TestSolanaWalletPaysA402Challenge(t *testing.T) {
	stub := &solanaStub{t: t, offerRails: []string{"base", "solana"}}
	srv := stub.start()
	key, pub := solanaTestKey(t)

	c, err := jarvisclaw.NewClient(
		jarvisclaw.WithSolanaRPC(srv.URL+"/rpc"),
		jarvisclaw.WithPrivateKey(key),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	raw, err := c.InvokeAPI(context.Background(), 476, map[string]any{"url": "x"})
	if err != nil {
		t.Fatalf("InvokeAPI with a Solana wallet: %v", err)
	}
	if !strings.Contains(string(raw), "ok") {
		t.Errorf("body = %q", raw)
	}
	if stub.calls != 2 {
		t.Fatalf("calls = %d, want a 402 then a paid retry", stub.calls)
	}

	// The header carries a base64 x402 v2 payload with a partially-signed transaction.
	payloadJSON, err := base64.StdEncoding.DecodeString(stub.seenSig)
	if err != nil {
		t.Fatalf("payment header is not base64: %v", err)
	}
	var payload struct {
		X402Version int `json:"x402Version"`
		Accepted    struct {
			Network string `json:"network"`
			Amount  string `json:"amount"`
			Asset   string `json:"asset"`
		} `json:"accepted"`
		Payload struct {
			Transaction string `json:"transaction"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("payment payload is not JSON: %v", err)
	}
	if payload.X402Version != 2 {
		t.Errorf("x402Version = %d, want 2", payload.X402Version)
	}
	// The Solana option must have been chosen even though Base was offered first: an
	// ed25519 key cannot produce an EVM signature.
	if !strings.HasPrefix(payload.Accepted.Network, "solana:") {
		t.Errorf("network = %q, want the Solana option", payload.Accepted.Network)
	}
	if payload.Accepted.Amount != "5750" {
		t.Errorf("amount = %q, want 5750", payload.Accepted.Amount)
	}

	// The transaction must be a V0 transaction carrying our signature in our own slot,
	// with the fee payer's slot left empty for the facilitator.
	txBytes, err := base64.StdEncoding.DecodeString(payload.Payload.Transaction)
	if err != nil {
		t.Fatalf("transaction is not base64: %v", err)
	}
	tx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		t.Fatalf("transaction does not decode: %v", err)
	}

	ourIndex := -1
	for i, k := range tx.Message.AccountKeys {
		if k.Equals(pub) {
			ourIndex = i
			break
		}
	}
	if ourIndex < 0 {
		t.Fatal("our pubkey is absent from the transaction account keys")
	}
	if ourIndex >= len(tx.Signatures) {
		t.Fatalf("no signature slot at our index %d (have %d)", ourIndex, len(tx.Signatures))
	}
	if tx.Signatures[ourIndex].IsZero() {
		t.Error("our signature slot is empty — the transaction was not signed by us")
	}
	// Fee payer is account key 0, and the facilitator fills that slot.
	if !tx.Signatures[0].IsZero() && ourIndex != 0 {
		t.Error("the fee payer's slot must be left empty for the facilitator")
	}

	// The instruction list is what CDP's simulation expects; a different shape is
	// rejected. See the comment in x402_solana.go.
	if len(tx.Message.Instructions) != 4 {
		t.Errorf("instructions = %d, want 4 (CU limit, CU price, TransferChecked, Memo)",
			len(tx.Message.Instructions))
	}
}

// The gateway omits the Solana option whenever its facilitator fee payer is unknown, and
// that is the state of production right now. A Solana client must say so plainly rather
// than trying to sign the Base option with an ed25519 key.
func TestSolanaWalletReportsAMissingSolanaOption(t *testing.T) {
	stub := &solanaStub{t: t, offerRails: []string{"base"}}
	srv := stub.start()
	key, _ := solanaTestKey(t)

	c, err := jarvisclaw.NewClient(
		jarvisclaw.WithSolanaRPC(srv.URL+"/rpc"),
		jarvisclaw.WithPrivateKey(key),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = c.InvokeAPI(context.Background(), 476, map[string]any{"url": "x"})
	if err == nil {
		t.Fatal("expected an error when no Solana option is offered")
	}
	if !strings.Contains(err.Error(), "no Solana payment option") {
		t.Errorf("error = %q, want it to name the missing Solana option", err)
	}
	// The offered rails belong in the message: without them the operator cannot tell a
	// misconfigured client from a gateway that has not discovered its fee payer.
	if !strings.Contains(err.Error(), "eip155:8453") {
		t.Errorf("error = %q, want it to list what was offered", err)
	}
}

// An EVM wallet must keep working against a challenge that offers both rails.
func TestEVMWalletStillPicksBase(t *testing.T) {
	stub := &solanaStub{t: t, offerRails: []string{"solana", "base"}}
	srv := stub.start()

	c, err := jarvisclaw.NewClient(
		jarvisclaw.WithPrivateKey("4c0883a69102937d6231471b5dbb6204fe512961708279f2e3f1a1a1f1f1f1f1"),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.InvokeAPI(context.Background(), 476, map[string]any{"url": "x"}); err != nil {
		t.Fatalf("InvokeAPI with an EVM wallet: %v", err)
	}
	if stub.calls != 2 {
		t.Fatalf("calls = %d, want a 402 then a paid retry", stub.calls)
	}
	if stub.seenSig == "" {
		t.Fatal("no payment header was sent")
	}
}

// Every constructor must accept either credential on either rail.
//
// The specialized clients embed *Client, so this holds structurally — the test exists so
// that a future client which does NOT embed it, and therefore silently supports only one
// credential, fails here rather than in someone's production agent.
func TestEveryClientAcceptsBothCredentials(t *testing.T) {
	solKey, solPub := solanaTestKey(t)
	const evmKey = "4c0883a69102937d6231471b5dbb6204fe512961708279f2e3f1a1a1f1f1f1f1"

	for _, cred := range []struct {
		name       string
		opt        jarvisclaw.Option
		wantAddr   string
		wantPrefix string
	}{
		{"api key", jarvisclaw.WithAPIKey("sk-test"), "", ""},
		{"evm wallet", jarvisclaw.WithPrivateKey(evmKey), "", "0x"},
		{"solana wallet", jarvisclaw.WithPrivateKey(solKey), solPub.String(), ""},
	} {
		t.Run(cred.name, func(t *testing.T) {
			// Each constructor takes the same Option list and delegates to NewClient.
			base, err := jarvisclaw.NewClient(cred.opt)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			chat, err := jarvisclaw.NewChatClient(cred.opt)
			if err != nil {
				t.Fatalf("NewChatClient: %v", err)
			}
			mp, err := jarvisclaw.NewMarketplaceClient(cred.opt)
			if err != nil {
				t.Fatalf("NewMarketplaceClient: %v", err)
			}
			if _, err := jarvisclaw.NewImageClient(cred.opt); err != nil {
				t.Fatalf("NewImageClient: %v", err)
			}
			if _, err := jarvisclaw.NewVideoClient(cred.opt); err != nil {
				t.Fatalf("NewVideoClient: %v", err)
			}
			if _, err := jarvisclaw.NewAudioClient(cred.opt); err != nil {
				t.Fatalf("NewAudioClient: %v", err)
			}
			if _, err := jarvisclaw.NewSearchClient(cred.opt); err != nil {
				t.Fatalf("NewSearchClient: %v", err)
			}

			// The wallet address must agree across constructors: they are the same
			// credential, so a difference would mean one of them re-derived it.
			for name, got := range map[string]string{
				"Client": base.Address(), "ChatClient": chat.Address(), "MarketplaceClient": mp.Address(),
			} {
				if cred.wantAddr != "" && got != cred.wantAddr {
					t.Errorf("%s.Address() = %q, want %q", name, got, cred.wantAddr)
				}
				if cred.wantPrefix != "" && !strings.HasPrefix(got, cred.wantPrefix) {
					t.Errorf("%s.Address() = %q, want prefix %q", name, got, cred.wantPrefix)
				}
				if cred.wantAddr == "" && cred.wantPrefix == "" && got != "" {
					t.Errorf("%s.Address() = %q, want empty in API-key mode", name, got)
				}
			}
		})
	}
}

// An API-key client must not attempt to sign, and must say what is missing. Read paths
// still work in either mode — only settlement needs a wallet.
func TestAPIKeyClientCannotPayA402(t *testing.T) {
	stub := &solanaStub{t: t, offerRails: []string{"base", "solana"}}
	srv := stub.start()

	c, err := jarvisclaw.NewClient(
		jarvisclaw.WithAPIKey("sk-test"),
		jarvisclaw.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = c.InvokeAPI(context.Background(), 476, map[string]any{"url": "x"})
	if err == nil {
		t.Fatal("an API-key client has no key to sign with; expected an error")
	}
	if !strings.Contains(err.Error(), "WithPrivateKey") {
		t.Errorf("error = %q, want it to name the missing credential", err)
	}
}
