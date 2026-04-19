# siqlah &nbsp;·&nbsp; سِقلة

<p align="center">
  <strong>Cryptographic accountability for AI API billing</strong><br>
  Verifiable Usage Receipts · Ed25519 Signatures · RFC 6962 Merkle Proofs · Witness Cosigning
</p>

<p align="center">
  <a href="https://github.com/YASSERRMD/siqlah/actions/workflows/ci.yml">
    <img alt="CI" src="https://github.com/YASSERRMD/siqlah/actions/workflows/ci.yml/badge.svg">
  </a>
  <a href="https://pkg.go.dev/github.com/yasserrmd/siqlah">
    <img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/yasserrmd/siqlah.svg">
  </a>
  <a href="LICENSE">
    <img alt="License" src="https://img.shields.io/badge/license-Apache%202.0-blue.svg">
  </a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white">
  <img alt="Rust" src="https://img.shields.io/badge/Rust-1.70+-orange?logo=rust&logoColor=white">
</p>

---

> **siqlah** (سِقلة) — Arabic: *"polish"* or *"refinement"*. The tool that brings clarity and proof to what was previously opaque: AI token billing.

---

## The Problem

Every major AI provider — OpenAI, Anthropic, Google — prints a token count in their API response. That number directly determines your invoice. Yet **no independent party verifies it**. You trust a JSON field.

siqlah closes this gap:

| Today | With siqlah |
|---|---|
| Provider reports tokens; you pay | Provider reports tokens; siqlah re-tokenizes locally and signs both |
| No audit trail | Append-only Merkle log with inclusion proofs |
| Single point of trust | k-of-n witness cosigning; any party can verify |
| Billing disputes require provider cooperation | Cryptographic receipts verifiable offline |

---

## Architecture

<!-- ARCHITECTURE DIAGRAM -->
<!-- Replace this block with the generated diagram image. See docs/gemini-diagram-prompt.md for the prompt to use with Google AI Studio / Gemini to generate a professional architecture diagram for this system. -->

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
│  SQLite (dev) / ClickHouse / TimescaleDB (prod)      │
├──────────────────────────────────────────────────────┤
│  Layer 1: Provider Adapters                          │
│  OpenAI · Anthropic · Generic (OpenAI-compatible)    │
│  Parse raw provider responses into normalized usage  │
└──────────────────────────────────────────────────────┘
```

> **Generate a visual diagram:** Use the prompt in [`docs/gemini-diagram-prompt.md`](docs/gemini-diagram-prompt.md) with [Google AI Studio](https://aistudio.google.com) to produce a publication-quality architecture diagram. Replace the ASCII block above with the result.

See [`docs/architecture.md`](docs/architecture.md) for the full four-layer design, data flow, and threat model.

---

## How It Works

### 1 · Ingest

When your application calls an AI API, forward the raw response body to siqlah:

```
POST /v1/receipts   { provider, tenant, model, response_body }
```

siqlah parses token counts, re-tokenizes locally via the Rust engine, hashes the raw bytes, and returns a signed **VUR Receipt** — an Ed25519-signed canonical JSON record containing both the provider-reported and locally-verified counts.

### 2 · Checkpoint

Every N receipts (or on a timer), siqlah builds a Merkle root over all canonical receipt bytes, signs a `SignedPayload`, and persists a **Checkpoint**. Each checkpoint embeds the previous root, forming an append-only hash chain.

### 3 · Witness

Independent witnesses fetch each checkpoint, verify the operator signature, and co-sign with their own Ed25519 key. Clients configure which witnesses they trust and how many cosignatures are required (k-of-n).

### 4 · Verify

Anyone with the operator's public key can:
- Verify any receipt's Ed25519 signature offline
- Obtain a Merkle inclusion proof (`GET /v1/receipts/{id}/proof`) and verify it without trusting the server
- Obtain a consistency proof between two checkpoints to prove the log has not been rewritten

---

## Quick Start

### Docker (Recommended)

```bash
git clone https://github.com/YASSERRMD/siqlah
cd siqlah/deployments

# Start main service + two auto-cosigning witnesses
docker compose up -d

# Health check
curl http://localhost:8080/v1/health
```

### Build from Source

**Requirements:** Go 1.21+, Rust 1.70+, `gcc`/`clang` (CGo for tokenizer FFI)

```bash
git clone https://github.com/YASSERRMD/siqlah
cd siqlah

make build        # build all binaries (Rust tokenizer + Go)
make test         # run full test suite
```

Binaries produced in `bin/`:

| Binary | Purpose |
|---|---|
| `siqlah` | Main API server |
| `siqlah-witness` | Witness CLI: `keygen`, `cosign`, `verify`, `watch` |
| `siqlah-verifier` | Client verifier: receipt verify, inclusion proof, reconcile |

### First Receipt in 60 Seconds

```bash
# 1. Generate operator key
./bin/siqlah-witness keygen --out operator.key

# 2. Start server
./bin/siqlah --operator-key ./operator.key --db ./siqlah.db --addr :8080

# 3. Ingest an OpenAI response
curl -X POST http://localhost:8080/v1/receipts \
  -H 'Content-Type: application/json' \
  -d '{
    "provider": "openai",
    "tenant": "acme",
    "model": "gpt-4o",
    "response_body": {
      "id": "chatcmpl-abc123",
      "usage": {"prompt_tokens": 150, "completion_tokens": 75}
    }
  }'

# 4. Build a checkpoint
curl -X POST http://localhost:8080/v1/checkpoints/build

# 5. Verify
curl http://localhost:8080/v1/checkpoints/1/verify
```

See [`docs/quickstart.md`](docs/quickstart.md) for the full 5-minute tutorial including witness setup and local proof verification.

---

## Supported Providers

| Provider | `provider` value | Notes |
|---|---|---|
| OpenAI | `openai` | `o1`/`o3` reasoning tokens via `completion_tokens_details` |
| Anthropic | `anthropic` | Cache creation and cache read token fields |
| OpenAI-compatible | `generic` | Ollama, vLLM, LiteLLM, llama.cpp |

---

## API Overview

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/receipts` | Ingest a single API response |
| `POST` | `/v1/receipts/batch` | Ingest multiple responses |
| `GET` | `/v1/receipts/{id}` | Fetch a receipt by UUID |
| `GET` | `/v1/receipts/{id}/proof` | Merkle inclusion proof |
| `POST` | `/v1/checkpoints/build` | Build and sign a checkpoint |
| `GET` | `/v1/checkpoints` | List checkpoints (paginated) |
| `GET` | `/v1/checkpoints/{id}/verify` | Verify operator sig + witness cosigs |
| `POST` | `/v1/checkpoints/{id}/witness` | Submit a witness cosignature |
| `GET` | `/v1/checkpoints/{id}/consistency/{old_id}` | Consistency proof between checkpoints |
| `GET` | `/v1/health` | Liveness probe |
| `GET` | `/v1/stats` | Aggregate counts |

Full reference: [`docs/api.md`](docs/api.md)

---

## Security & Threat Model

| Threat | Mitigation |
|---|---|
| Provider inflates token counts | Rust tokenizer re-verifies locally; discrepancy monitor alerts |
| Operator inflates token counts | Ed25519 signature binds counts at signing time; client verifies |
| Log tampered after the fact | RFC 6962 Merkle inclusion and consistency proofs; witness cosignatures |
| Single-operator trust | Witness network; k-of-n cosigning; public audit log |
| Replay attacks | Unique receipt UUID; timestamp in signed payload |
| Key compromise | Checkpoint chain breaks; detectable; witness network provides redundancy |

---

## Documentation

| Document | Description |
|---|---|
| [`docs/quickstart.md`](docs/quickstart.md) | 5-minute tutorial |
| [`docs/api.md`](docs/api.md) | Full API reference |
| [`docs/architecture.md`](docs/architecture.md) | Four-layer design, data flow, threat model |
| [`docs/receipt-spec.md`](docs/receipt-spec.md) | VUR receipt format and canonical serialization spec |
| [`docs/witness-protocol.md`](docs/witness-protocol.md) | Witness verification and cosigning protocol |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Development guide |
| [`CHANGELOG.md`](CHANGELOG.md) | Release history |

---

## Comparison

| Approach | Trust Model | Verifiability | Token-Level |
|---|---|---|---|
| **siqlah** | Multi-party witness, Ed25519 | Cryptographic inclusion proofs | Yes (Rust FFI) |
| Hyperledger Fabric | Permissioned blockchain | On-chain audit | No |
| x402 (HTTP payment) | Blockchain settlement | Payment proof only | No |
| ZKML | ZK proofs of inference | Strong but expensive | Yes (heavy) |
| Provider billing APIs | Trust provider | None | No |

siqlah occupies the pragmatic middle ground: cryptographically strong, operationally simple, no blockchain required.

---

## License

Apache License, Version 2.0. See [LICENSE](LICENSE).

Copyright 2026 [YASSERRMD](https://github.com/YASSERRMD)
