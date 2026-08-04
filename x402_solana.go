package jarvisclaw

// x402 payment signing on Solana.
//
// This SDK could only pay with an EVM key, so a caller holding a Solana wallet had no
// way to spend it — WithPrivateKey parsed secp256k1 only, and a base58 Solana key was
// rejected as an invalid hex key at construction time. The Python SDK has supported both
// rails for some time, so the two SDKs disagreed about what "wallet mode" means.
//
// The transaction shape mirrors the gateway's own signer (service/hd_wallet_x402_solana.go)
// and the official x402 SVM SDK: a partially-signed V0 transaction whose fee payer is the
// facilitator, carrying
//
//	[SetComputeUnitLimit, SetComputeUnitPrice, TransferChecked, Memo]
//
// We sign our slot only; the facilitator signs as fee payer when it verifies. Deviating
// from that instruction list makes CDP's simulation reject the payment, so it is copied
// deliberately rather than reinvented.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/mr-tron/base58"
)

const (
	// SolanaNetwork is the CAIP-2 identifier for Solana mainnet, as the gateway
	// advertises it in a 402 challenge.
	SolanaNetwork = "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"
	// SolanaUSDCMint is the only asset this SDK will pay with on Solana.
	SolanaUSDCMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	// solanaUSDCDecimals is fixed by the mint; TransferChecked verifies it on-chain.
	solanaUSDCDecimals = 6
	// solanaFallbackRPC is used when no RPC endpoint is configured.
	solanaFallbackRPC = "https://api.mainnet-beta.solana.com"
	// solanaMaxPaymentMicroUSDC caps a single signed payment at 100 USDC.
	//
	// A signed transaction is irreversible, and the amount comes from a server response.
	// The cap is a blast-radius limit on a wrong or hostile challenge, matching the
	// Python SDK's own guard.
	solanaMaxPaymentMicroUSDC = 100_000_000
)

// memoProgramID is the SPL Memo Program v2.
var memoProgramID = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")

// solanaSigner holds a Solana keypair for x402 payments.
type solanaSigner struct {
	priv ed25519.PrivateKey
	pub  solana.PublicKey
	// rpcURL is where a recent blockhash is fetched from.
	rpcURL string
}

// newSolanaSigner parses a base58 Solana private key: either a 64-byte keypair or a
// 32-byte seed, which is what every Solana wallet exports.
func newSolanaSigner(base58Key, rpcURL string) (*solanaSigner, error) {
	decoded, err := base58.Decode(strings.TrimSpace(base58Key))
	if err != nil {
		return nil, fmt.Errorf("invalid base58 Solana key: %w", err)
	}

	var priv ed25519.PrivateKey
	switch len(decoded) {
	case ed25519.PrivateKeySize: // 64 — full keypair
		priv = ed25519.PrivateKey(decoded)
	case ed25519.SeedSize: // 32 — seed only
		priv = ed25519.NewKeyFromSeed(decoded)
	default:
		return nil, fmt.Errorf("invalid Solana key length %d bytes (expected 32 or 64)", len(decoded))
	}

	if rpcURL == "" {
		rpcURL = solanaFallbackRPC
	}
	return &solanaSigner{
		priv:   priv,
		pub:    solana.PublicKeyFromBytes(priv.Public().(ed25519.PublicKey)),
		rpcURL: rpcURL,
	}, nil
}

// Address returns the signer's base58 public key.
func (s *solanaSigner) Address() string { return s.pub.String() }

// isSolanaKey reports whether a string looks like a base58-encoded Solana key rather
// than a hex EVM key.
//
// Checked in this order because the two encodings overlap: a 64-character hex string is
// also valid base58, so hex must win. An EVM key is 64 hex chars (optionally 0x-prefixed);
// a Solana key decodes from base58 to exactly 32 or 64 bytes.
func isSolanaKey(key string) bool {
	k := strings.TrimSpace(key)
	if k == "" {
		return false
	}
	if strings.HasPrefix(k, "0x") || strings.HasPrefix(k, "0X") {
		return false
	}
	if len(k) == 64 && isHexString(k) {
		return false
	}
	decoded, err := base58.Decode(k)
	if err != nil {
		return false
	}
	return len(decoded) == ed25519.SeedSize || len(decoded) == ed25519.PrivateKeySize
}

func isHexString(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// selectSolanaPayment picks the Solana option out of a 402 challenge.
//
// Returns a clear error rather than silently falling back to an EVM option: a
// Solana-only client cannot sign an EVM payment, and reporting "no Solana option" names
// the actual problem — the gateway omits the Solana entry whenever its facilitator fee
// payer is not yet known.
func selectSolanaPayment(body []byte) (*paymentInfo, error) {
	var resp struct {
		Accepts  []paymentInfo `json:"accepts"`
		Payments []paymentInfo `json:"payments"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse 402 body: %w", err)
	}
	options := resp.Accepts
	if len(options) == 0 {
		options = resp.Payments
	}
	for i := range options {
		if strings.HasPrefix(options[i].Network, "solana:") {
			return &options[i], nil
		}
	}
	networks := make([]string, 0, len(options))
	for i := range options {
		networks = append(networks, options[i].Network)
	}
	return nil, fmt.Errorf(
		"402 response offers no Solana payment option (networks offered: %s) — "+
			"the gateway omits Solana until its facilitator fee payer is known",
		strings.Join(networks, ", "))
}

// signSolanaPayment builds and partially signs the payment transaction, returning the
// base64 x402 payload for the PAYMENT-SIGNATURE header.
func (s *solanaSigner) signSolanaPayment(ctx context.Context, payment *paymentInfo, resourceURL string) (string, error) {
	if payment.Asset != "" && payment.Asset != SolanaUSDCMint {
		return "", fmt.Errorf("unexpected Solana asset %q, expected USDC (%s)", payment.Asset, SolanaUSDCMint)
	}
	if payment.PayTo == "" {
		return "", fmt.Errorf("402 response has an empty payTo for Solana")
	}

	amount, err := solanaPaymentAmount(payment)
	if err != nil {
		return "", err
	}

	// The fee payer is the facilitator's, discovered from CDP and rotated by it, so it
	// only ever comes from the challenge. Without it the transaction cannot be compiled
	// at all: it is account key 0.
	feePayerStr, _ := payment.Extra["feePayer"].(string)
	if feePayerStr == "" {
		return "", fmt.Errorf("402 response did not provide a Solana feePayer")
	}
	feePayer, err := solana.PublicKeyFromBase58(feePayerStr)
	if err != nil {
		return "", fmt.Errorf("invalid Solana feePayer %q: %w", feePayerStr, err)
	}

	recipient, err := solana.PublicKeyFromBase58(payment.PayTo)
	if err != nil {
		return "", fmt.Errorf("invalid Solana payTo %q: %w", payment.PayTo, err)
	}
	mint := solana.MustPublicKeyFromBase58(SolanaUSDCMint)

	senderATA, _, err := solana.FindAssociatedTokenAddress(s.pub, mint)
	if err != nil {
		return "", fmt.Errorf("derive sender token account: %w", err)
	}
	recipientATA, _, err := solana.FindAssociatedTokenAddress(recipient, mint)
	if err != nil {
		return "", fmt.Errorf("derive recipient token account: %w", err)
	}

	transferIx, err := token.NewTransferCheckedInstructionBuilder().
		SetAmount(amount).
		SetDecimals(solanaUSDCDecimals).
		SetSourceAccount(senderATA).
		SetMintAccount(mint).
		SetDestinationAccount(recipientATA).
		SetOwnerAccount(s.pub).
		ValidateAndBuild()
	if err != nil {
		return "", fmt.Errorf("build TransferChecked: %w", err)
	}

	// A random nonce makes each transaction unique, which the facilitator requires:
	// two identical payments would otherwise share a signature and be replayable.
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("generate memo nonce: %w", err)
	}

	blockhash, err := s.recentBlockhash(ctx)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			// 20000 CU and 1 microlamport, matching the gateway and the official SVM
			// SDK. The facilitator simulates against these values.
			computebudget.NewSetComputeUnitLimitInstruction(20_000).Build(),
			computebudget.NewSetComputeUnitPriceInstruction(1).Build(),
			transferIx,
			&solanaMemoInstruction{data: []byte(hex.EncodeToString(nonceBytes))},
		},
		blockhash,
		solana.TransactionPayer(feePayer),
	)
	if err != nil {
		return "", fmt.Errorf("build Solana transaction: %w", err)
	}
	// V0 is what the facilitator accepts; a legacy transaction is rejected.
	tx.Message.SetVersion(solana.MessageVersionV0)

	messageBytes, err := tx.Message.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("serialize Solana message: %w", err)
	}

	// Our signature goes in the slot matching our position in the account keys, with
	// the fee payer's slot left empty for the facilitator to fill. Signing into the
	// wrong slot produces a transaction that verifies as neither party's.
	index := -1
	for i, key := range tx.Message.AccountKeys {
		if key.Equals(s.pub) {
			index = i
			break
		}
	}
	if index < 0 {
		return "", fmt.Errorf("signer key absent from the compiled transaction")
	}
	if len(tx.Signatures) <= index {
		sigs := make([]solana.Signature, index+1)
		copy(sigs, tx.Signatures)
		tx.Signatures = sigs
	}
	copy(tx.Signatures[index][:], ed25519.Sign(s.priv, messageBytes))

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("serialize Solana transaction: %w", err)
	}

	network := payment.Network
	if network == "" {
		network = SolanaNetwork
	}
	maxTimeout := payment.MaxTimeoutSeconds
	if maxTimeout <= 0 {
		maxTimeout = 300
	}

	payload := map[string]any{
		"x402Version": 2,
		"resource": map[string]any{
			"url":         resourceURL,
			"description": "API request",
			"mimeType":    "application/json",
		},
		"accepted": map[string]any{
			"scheme":            "exact",
			"network":           network,
			"amount":            strconv.FormatUint(amount, 10),
			"asset":             SolanaUSDCMint,
			"payTo":             payment.PayTo,
			"maxTimeoutSeconds": maxTimeout,
			"extra":             map[string]any{"feePayer": feePayerStr},
		},
		"payload": map[string]any{
			"transaction": base64.StdEncoding.EncodeToString(txBytes),
		},
		"extensions": map[string]any{},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal Solana payment payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(payloadJSON), nil
}

// solanaPaymentAmount reads the amount from a challenge, accepting both field names the
// protocol has used, and refuses anything outside the safety cap.
func solanaPaymentAmount(payment *paymentInfo) (uint64, error) {
	raw := payment.amountValue()
	if raw == "" {
		return 0, fmt.Errorf("402 response carries no payment amount")
	}
	amount, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Solana payment amount %q: %w", raw, err)
	}
	if amount == 0 {
		return 0, fmt.Errorf("Solana payment amount must be positive")
	}
	if amount > solanaMaxPaymentMicroUSDC {
		return 0, fmt.Errorf(
			"Solana payment amount %d micro-USDC exceeds the SDK safety cap of %d (100 USDC)",
			amount, solanaMaxPaymentMicroUSDC)
	}
	return amount, nil
}

// recentBlockhash fetches a finalized blockhash.
//
// Not cached: a stale blockhash makes the facilitator's simulation fail, and the failure
// surfaces as a rejected payment rather than a retryable error.
//
// Hand-rolled JSON-RPC rather than solana-go/rpc. That package pulls in a MongoDB driver
// and zap through its own transitive dependencies — 14 extra modules for one method call,
// in a library other people's builds have to carry. The request is four lines; the
// dependency is not worth it.
func (s *solanaSigner) recentBlockhash(ctx context.Context) (solana.Hash, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getLatestBlockhash",
		"params":  []any{map[string]string{"commitment": "finalized"}},
	})
	if err != nil {
		return solana.Hash{}, fmt.Errorf("build blockhash request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, s.rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return solana.Hash{}, fmt.Errorf("build blockhash request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return solana.Hash{}, fmt.Errorf("get Solana blockhash from %s: %w", s.rpcURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return solana.Hash{}, fmt.Errorf("Solana RPC %s answered HTTP %d", s.rpcURL, resp.StatusCode)
	}

	var parsed struct {
		Result struct {
			Value struct {
				Blockhash string `json:"blockhash"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return solana.Hash{}, fmt.Errorf("parse blockhash response: %w", err)
	}
	if parsed.Error != nil {
		return solana.Hash{}, fmt.Errorf("Solana RPC error: %s", parsed.Error.Message)
	}
	if parsed.Result.Value.Blockhash == "" {
		return solana.Hash{}, fmt.Errorf("Solana RPC returned no blockhash")
	}
	return solana.HashFromBase58(parsed.Result.Value.Blockhash)
}

// solanaMemoInstruction is an SPL Memo v2 instruction carrying the replay nonce.
type solanaMemoInstruction struct{ data []byte }

func (ix *solanaMemoInstruction) ProgramID() solana.PublicKey { return memoProgramID }

// Accounts is deliberately empty: Memo v2 verifies the signatures of every account it
// lists, which fails during the facilitator's simulation because our signature is not
// recognised at that point. With no accounts the memo is still recorded.
func (ix *solanaMemoInstruction) Accounts() []*solana.AccountMeta {
	return []*solana.AccountMeta{}
}

func (ix *solanaMemoInstruction) Data() ([]byte, error) { return ix.data, nil }
