# siqlah Architecture

## Overview

siqlah is a Verifiable Usage Receipt (VUR) system that brings cryptographic accountability to AI API billing. It answers the question: *"Can I prove what tokens I was billed for, to anyone, without trusting any single party?"*

## The Four-Layer Architecture

```
┌──────────────────────────────────────────────────────┐
│  Layer 4: Witness Network                             │
│  Independent parties co-sign checkpoints             │
│  (Ed25519, C2SP-compatible)                          │
├──────────────────────────────────────────────────────┤
│  Layer 3: Checkpoint Log                             │
│  RFC 6962 Merkle tree over receipt batches           │
│  Append-only, consistency-provable                   │
├──────────────────────────────────────────────────────┤
│  Layer 2: Receipt Store                              │
│  Signed, canonical VUR records per API call          │
│  SQLite (dev) / ClickHouse/TimescaleDB (prod)        │
├──────────────────────────────────────────────────────┤
│  Layer 1: Provider Adapters                          │
│  OpenAI, Anthropic, Generic (OpenAI-compatible)      │
│  Parse raw provider responses into normalized usage  │
└──────────────────────────────────────────────────────┘
```

### Layer 1: Provider Adapters

Each provider adapter (`internal/provider/`) implements `vur.ProviderAdapter`:
- `ParseUsage(responseBody []byte) (*ProviderUsage, error)` — extracts token counts from provider JSON
- Supported: OpenAI (including `o1`/`o3` reasoning tokens), Anthropic (including cache tokens), Generic (OpenAI-compatible: Ollama, vLLM, LiteLLM)

### Layer 2: Receipt Store

A **VUR Receipt** (`pkg/vur/Receipt`) is the atomic unit:
- UUID, version, tenant, provider, model, token counts (provider-reported and locally-verified)
- `RequestHash`, `ResponseHash` — SHA-256 of raw bytes for tamper detection
- `TokenBoundaryRoot` — Merkle root over token boundary byte offsets (from Rust tokenizer)
- `SignerIdentity` — operator public key hex
- `SignatureHex` — Ed25519 signature over canonical JSON (fields alphabetically sorted, timestamp as RFC3339Nano UTC)

**Canonical serialization** uses a shadow struct with alphabetically-ordered JSON tags, ensuring byte-for-byte reproducibility across implementations.

### Layer 3: Checkpoint Log

The checkpoint engine (`internal/checkpoint/`) batches unbatched receipts into an RFC 6962 Merkle tree:
1. Fetch unbatched receipts from the store
2. Compute `HashLeaf(CanonicalBytes(r))` for each receipt
3. Build the Merkle root via `BuildRoot(leaves)` (domain-separated, power-of-2 subtrees)
4. Sign a `SignedPayload` (batch bounds, tree size, root hex, previous root hex, timestamp) with the operator Ed25519 key
5. Store the checkpoint, mark receipts as batched

**Chain integrity**: each checkpoint records `PreviousRootHex`, forming an append-only chain detectable by `VerifyChainConsistency`.

### Layer 4: Witness Network

Witnesses are independent parties that:
1. Fetch a checkpoint from the ledger
2. Verify the operator's Ed25519 signature on the `SignedPayload`
3. Produce their own Ed25519 cosignature over the same payload
4. Submit it via `POST /v1/checkpoints/{id}/witness`

Witnesses are identified by their public key hex. Multiple witnesses provide k-of-n cosigning with no on-chain coordination required.

## Data Flow

```
Client                     siqlah                      Witnesses
  │                           │                             │
  │  POST /v1/receipts        │                             │
  │──────────────────────────>│                             │
  │  {provider, response_body}│                             │
  │                           │  ParseUsage(body)           │
  │                           │  SignReceipt(priv)          │
  │                           │  AppendReceipt(store)       │
  │<──────────────────────────│                             │
  │  {receipt, sig_hex}       │                             │
  │                           │                             │
  │  POST /v1/checkpoints/build                             │
  │──────────────────────────>│                             │
  │                           │  BuildAndSign()             │
  │<──────────────────────────│                             │
  │  {checkpoint, root_hex}   │                             │
  │                           │  GET /v1/checkpoints/{id}   │
  │                           │<────────────────────────────│
  │                           │──────────────────────────── │
  │                           │  VerifyOperatorSig()        │
  │                           │  CoSign(witnessPriv)        │
  │                           │  POST /v1/checkpoints/{id}/witness
  │                           │<────────────────────────────│
```

## Threat Model

| Threat | Mitigation |
|---|---|
| Operator inflates token counts | Ed25519 signature binds token counts at signing time; client verifies |
| Provider inflates counts | Local tokenizer re-verification (Rust FFI); discrepancy monitor alerts |
| Log tampering after the fact | RFC 6962 Merkle inclusion and consistency proofs; witness cosignatures |
| Single-operator trust | Witness network; k-of-n cosigning; public audit log |
| Replay attacks | Unique receipt ID (UUID); timestamp in signed payload |
| Key compromise | Operator key rotation (checkpoint chain breaks; detectable); witness network provides redundancy |
| Fake witnesses | Witnesses are identified by public key; clients configure trusted witness set |

## Rust Tokenizer Engine

The Rust `siqlah-tokenizer` crate (`tokenizer-rs/`) provides independent token count verification:
- Accepts tokenizer JSON directly (from HuggingFace `tokenizers` crate)
- Computes token count and boundary Merkle root over byte offsets
- Exposed via C FFI (`siqlah_tokenize` / `siqlah_free`)
- Go wrapper (`internal/tokenizer/`) degrades gracefully if the shared library is unavailable

## Comparison with Related Approaches

| Approach | Trust Model | Verifiability | Token-Level |
|---|---|---|---|
| **siqlah** | Multi-party witness, Ed25519 | Cryptographic inclusion proofs | Yes (Rust FFI) |
| Hyperledger Fabric | Permissioned blockchain | On-chain audit | No |
| x402 (HTTP payment) | Blockchain settlement | Payment proof only | No |
| ZKML | ZK proofs of inference | Strong but expensive | Yes (heavy) |
| Provider billing APIs | Trust provider | None | No |

siqlah occupies the pragmatic middle ground: cryptographically strong, operationally simple, no blockchain required.
