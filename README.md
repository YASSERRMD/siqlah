# siqlah (سِقلة)

**Verifiable Usage Receipts for AI API Calls**

*siqlah* (Arabic: سِقلة, "ledger stone" / "record") is an open-source Verifiable Usage Receipt (VUR) system for AI token usage metering. It produces cryptographically verifiable, witness-cosigned, tamper-evident receipts for every AI API call — solving the fundamental trust gap where all current AI billing tools simply re-emit provider-reported token counts without any independent verification.

---

## Problem Statement

Every major AI provider — OpenAI, Anthropic, Google, Cohere — reports token counts that directly determine billing. Yet no tool today independently verifies those counts before the invoice is generated. Operators trust a number printed in a JSON response. siqlah closes this gap: it intercepts AI API responses, locally re-tokenizes the text using the same tokenizer the provider uses, signs a cryptographic receipt over both the provider-reported and locally-verified counts, batches receipts into a Merkle log, and allows external witnesses to cosign each checkpoint — producing a publicly auditable, append-only ledger of AI usage that any party can independently verify.

---

## Architecture

siqlah is built in four layers:

| Layer | What it does |
|-------|--------------|
| **Receipts** | Per-call signed records containing provider-reported counts, locally-verified counts, request/response hashes, and a Merkle root over token boundary byte offsets |
| **Log** | Append-only SQLite (or ClickHouse/TimescaleDB) store; receipts are batched into Merkle checkpoints with Ed25519 operator signatures |
| **Witnesses** | Independent cosigners that verify the operator signature and append-only property before adding their own cosignature, providing k-of-n attestation |
| **Monitor** | Background daemon that re-tokenizes stored receipts and fires alerts when provider-reported counts diverge from locally-verified counts beyond a configurable threshold |

---

## Quick Start (Docker)

```bash
git clone https://github.com/yasserrmd/siqlah
cd siqlah/deployments

# Start the main service + two auto-cosigning witnesses
docker compose up -d

# Ingest a usage event
curl -X POST http://localhost:8080/v1/receipts \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "tenant": "my-org",
    "model": "gpt-4o",
    "response_body": "{\"usage\":{\"prompt_tokens\":150,\"completion_tokens\":45}}"
  }'

# Trigger a checkpoint
curl -X POST http://localhost:8080/v1/checkpoints/build

# Verify the checkpoint
curl http://localhost:8080/v1/checkpoints/1/verify

# Check stats
curl http://localhost:8080/v1/stats
```

---

## Building from Source

**Requirements:** Go 1.22+, Rust 1.75+ (for tokenizer engine), Docker (optional)

```bash
# Build the Rust tokenizer engine
make build-tokenizer

# Build all Go binaries
make build

# Run tests
make test

# Lint
make lint
```

Binaries produced:
- `bin/siqlah` — main service
- `bin/witness` — witness CLI (keygen, cosign, verify, watch)
- `bin/verifier` — client-side verifier CLI

---

## Provider Adapters

| Provider | Adapter | Token Fields |
|----------|---------|--------------|
| OpenAI | `openai` | `prompt_tokens`, `completion_tokens`, `reasoning_tokens` |
| Anthropic | `anthropic` | `input_tokens`, `output_tokens`, cache fields |
| OpenAI-compatible | `generic` | Works with Ollama, vLLM, llama.cpp, LiteLLM |

---

## How Verification Works

1. **Per-call**: siqlah calls the Rust tokenizer (HuggingFace `tokenizers` crate) to re-count tokens in the request/response text, recording boundary byte offsets into a Merkle tree. The operator signs a receipt containing both the provider-reported count and the locally-verified count.

2. **Checkpoint**: Every N receipts (default: 1000) or every T seconds (default: 30s), siqlah builds a Merkle root over all receipt canonical bytes, signs a checkpoint payload, and persists it. The chain of checkpoints forms an append-only log: each checkpoint embeds the previous root.

3. **Witness cosigning**: External witnesses fetch each checkpoint, verify the operator signature, verify that the new root is consistent with the old one (using a Merkle consistency proof), and add their own Ed25519 cosignature. A checkpoint is considered fully attested when k-of-n configured witnesses have cosigned.

4. **Client verification**: The `verifier` CLI can fetch an inclusion proof for any receipt, verify it against the checkpoint Merkle root, and independently re-run the tokenizer to confirm the count recorded in the receipt.

---

## License

Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

Copyright 2026 Mohamed Yasser (YASSERRMD)
