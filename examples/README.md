# Examples

Runnable examples for the JarvisClaw Go SDK. Each directory is its own `main`
package, and every one was executed against the live gateway.

## Setup

```bash
go get github.com/api-jarvisclaw/go-sdk/v2
export JARVISCLAW_API_KEY=sk-...
```

Every example reads `JARVISCLAW_API_KEY` from the environment. Nothing here
hardcodes a credential.

## Running

```bash
go run ./examples/chat
```

| Example | What it shows | Needs a funded wallet |
|---|---|---|
| [chat](chat/) | Completions, system prompts, metadata, streaming | no |
| [wallet](wallet/) | On-chain balance, history, limits, pools | no |
| [intent](intent/) | Intent types, discovery, ranking, natural language | no |
| [embeddings](embeddings/) | Embeddings and batch embedding | no |
| [analytics](analytics/) | Spend aggregation, quality metrics, insights | no |
| [federation](federation/) | Peer discovery and federated resource search | no |
| [marketplace](marketplace/) | DeFi data, JSON-RPC, arbitrary services | **yes** |

## Two billing paths

The gateway settles a request one of two ways:

- **Account quota** — you name a model explicitly (`WithChatModel("openai/gpt-4o-mini")`).
- **x402 on-chain USDC** — anything routed through `auto/*` (the default when no
  model is given) and every marketplace service, settled against your HD wallet.

The second path needs USDC in the wallet even with API-key auth. Without it you
get a settlement failure after a slow retry, or a 403 `insufficient HD wallet
balance`. Run `go run ./examples/wallet` to see your balance and addresses.

That is why these examples name their models explicitly and pass
`WithMaxTokens`: the gateway reserves against the model's full output allowance
up front, so an uncapped request can be refused on a low balance even when the
reply would have been cheap.

## x402 mode

Every constructor also accepts a private key instead of an API key, settling
each request on-chain:

```go
client, err := jarvisclaw.NewClient(
    jarvisclaw.WithPrivateKey(os.Getenv("JARVISCLAW_WALLET_KEY")),
)

// Solana instead of Base:
client, err := jarvisclaw.NewClient(
    jarvisclaw.WithPrivateKey(key),
    jarvisclaw.WithNetwork("solana"),
)
```

The examples use API-key auth because x402 spends real funds on every call.
