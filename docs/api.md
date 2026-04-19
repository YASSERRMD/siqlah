# siqlah API Reference

Base URL: `http://localhost:8080`

All request and response bodies are `application/json`. All timestamps are RFC 3339 (UTC).

---

## Receipts

### POST /v1/receipts

Ingest a single AI API response and create a signed VUR receipt.

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `provider` | string | yes | Provider name: `openai`, `anthropic`, or `generic` |
| `tenant` | string | yes | Tenant identifier for multi-tenant deployments |
| `model` | string | yes | Model name (e.g. `gpt-4o`, `claude-3-5-sonnet-20241022`) |
| `response_body` | object | yes | Raw provider API response JSON |
| `request_body` | object | no | Raw provider API request JSON (hashed for tamper detection) |
| `request_id` | string | no | Provider request ID (extracted from response if omitted) |

**Response — 201 Created**

Returns the full [Receipt object](#receipt-object).

**Errors**

| Code | Condition |
|---|---|
| 400 | Invalid JSON, unknown provider, or adapter parse error |
| 500 | Storage failure |

**Example**

```bash
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
```

---

### POST /v1/receipts/batch

Ingest multiple AI API responses in a single request.

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `items` | array | yes | Array of [ingest request objects](#post-v1receipts) (min 1) |

**Response — 201 Created**

```json
{
  "count": 3,
  "receipts": [ ... ]
}
```

**Errors**

| Code | Condition |
|---|---|
| 400 | Empty `items`, invalid JSON, or any item fails parsing |
| 500 | Storage failure |

**Example**

```bash
curl -X POST http://localhost:8080/v1/receipts/batch \
  -H 'Content-Type: application/json' \
  -d '{
    "items": [
      {
        "provider": "anthropic",
        "tenant": "acme",
        "model": "claude-3-5-sonnet-20241022",
        "response_body": {
          "usage": {"input_tokens": 100, "output_tokens": 50}
        }
      },
      {
        "provider": "openai",
        "tenant": "acme",
        "model": "gpt-4o-mini",
        "response_body": {
          "usage": {"prompt_tokens": 200, "completion_tokens": 80}
        }
      }
    ]
  }'
```

---

### GET /v1/receipts/{id}

Fetch a receipt by its UUID.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | string (UUID) | Receipt UUID |

**Response — 200 OK**

Returns the full [Receipt object](#receipt-object).

**Errors**

| Code | Condition |
|---|---|
| 404 | Receipt not found |
| 500 | Storage failure |

**Example**

```bash
curl http://localhost:8080/v1/receipts/f47ac10b-58cc-4372-a567-0e02b2c3d479
```

---

### GET /v1/receipts/{id}/proof

Get an RFC 6962 Merkle inclusion proof for a receipt.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | string (UUID) | Receipt UUID |

**Response — 200 OK**

```json
{
  "receipt_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "checkpoint_id": 1,
  "leaf_index": 3,
  "tree_size": 10,
  "root_hex": "a3f2b1...",
  "proof": [
    "c4d5e6...",
    "1a2b3c..."
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `receipt_id` | string | Receipt UUID |
| `checkpoint_id` | integer | ID of the checkpoint containing this receipt |
| `leaf_index` | integer | 0-based index of this receipt in the checkpoint's Merkle tree |
| `tree_size` | integer | Total number of leaves in the checkpoint's Merkle tree |
| `root_hex` | string | Merkle root hex (64 chars) |
| `proof` | array of strings | Sibling hashes from leaf to root |

**Errors**

| Code | Condition |
|---|---|
| 404 | Receipt not found, or not yet batched into a checkpoint |
| 500 | Storage or proof generation failure |

**Example**

```bash
curl http://localhost:8080/v1/receipts/f47ac10b-58cc-4372-a567-0e02b2c3d479/proof
```

To verify locally:

```bash
# Use cmd/verifier
siqlah-verifier check-proof \
  --receipt-id f47ac10b-58cc-4372-a567-0e02b2c3d479 \
  --server http://localhost:8080
```

---

## Checkpoints

### POST /v1/checkpoints/build

Trigger a manual checkpoint build. Batches all unbatched receipts into a new Merkle checkpoint, signs it with the operator key, and stores it.

**Request body** — none

**Response — 201 Created**

Returns a [Checkpoint object](#checkpoint-object).

If there are no unbatched receipts:

```json
{"message": "no unbatched receipts"}
```
with status **200 OK**.

**Errors**

| Code | Condition |
|---|---|
| 500 | Build or storage failure |

**Example**

```bash
curl -X POST http://localhost:8080/v1/checkpoints/build
```

---

### GET /v1/checkpoints

List checkpoints with pagination.

**Query parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `offset` | integer | 0 | Number of checkpoints to skip |
| `limit` | integer | 20 | Max checkpoints to return (capped at 100) |

**Response — 200 OK**

```json
{
  "checkpoints": [ ... ],
  "offset": 0,
  "limit": 20
}
```

**Example**

```bash
curl 'http://localhost:8080/v1/checkpoints?offset=0&limit=10'
```

---

### GET /v1/checkpoints/{id}

Fetch a checkpoint by its integer ID.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | integer | Checkpoint ID |

**Response — 200 OK**

Returns a [Checkpoint object](#checkpoint-object).

**Errors**

| Code | Condition |
|---|---|
| 400 | `id` is not a positive integer |
| 404 | Checkpoint not found |
| 500 | Storage failure |

**Example**

```bash
curl http://localhost:8080/v1/checkpoints/1
```

---

### GET /v1/checkpoints/{id}/verify

Verify the operator's Ed25519 signature on a checkpoint and list all witness cosignatures.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | integer | Checkpoint ID |

**Response — 200 OK**

```json
{
  "operator_valid": true,
  "witnesses": {
    "abc123pubkey...": "defsig..."
  }
}
```

| Field | Type | Description |
|---|---|---|
| `operator_valid` | boolean | Whether the operator Ed25519 signature is valid |
| `operator_error` | string | Present only when `operator_valid` is false |
| `witnesses` | object | Map of witness public key hex → cosignature hex |

**Errors**

| Code | Condition |
|---|---|
| 400 | `id` is not a positive integer |
| 404 | Checkpoint not found |
| 500 | Storage failure |

**Example**

```bash
curl http://localhost:8080/v1/checkpoints/1/verify
```

---

### POST /v1/checkpoints/{id}/witness

Submit a witness cosignature for a checkpoint.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | integer | Checkpoint ID |

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `witness_id` | string | yes | Witness Ed25519 public key hex |
| `sig_hex` | string | yes | Ed25519 cosignature hex over the checkpoint's `SignedPayload` bytes |

**Response — 201 Created**

```json
{"status": "accepted"}
```

**Errors**

| Code | Condition |
|---|---|
| 400 | Invalid JSON or missing fields |
| 404 | Checkpoint not found |
| 500 | Storage failure |

**Example**

```bash
# Using the witness CLI:
siqlah-witness cosign \
  --server http://localhost:8080 \
  --checkpoint-id 1 \
  --key /etc/siqlah/witness.key

# Or raw:
curl -X POST http://localhost:8080/v1/checkpoints/1/witness \
  -H 'Content-Type: application/json' \
  -d '{
    "witness_id": "a1b2c3...",
    "sig_hex": "d4e5f6..."
  }'
```

---

### GET /v1/checkpoints/{id}/consistency/{old_id}

Generate an RFC 6962 consistency proof showing that the log at `old_id` is a prefix of the log at `id`.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | integer | Newer checkpoint ID |
| `old_id` | integer | Older checkpoint ID |

**Response — 200 OK**

```json
{
  "old_checkpoint_id": 1,
  "new_checkpoint_id": 2,
  "old_size": 10,
  "new_size": 25,
  "old_root_hex": "a1b2c3...",
  "new_root_hex": "d4e5f6...",
  "proof": [
    "1a2b3c...",
    "4d5e6f..."
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `old_checkpoint_id` | integer | ID of the older checkpoint |
| `new_checkpoint_id` | integer | ID of the newer checkpoint |
| `old_size` | integer | Cumulative receipt count at the old checkpoint |
| `new_size` | integer | Cumulative receipt count at the new checkpoint |
| `old_root_hex` | string | Cumulative Merkle root at old size |
| `new_root_hex` | string | Cumulative Merkle root at new size |
| `proof` | array of strings | Consistency proof hashes |

**Errors**

| Code | Condition |
|---|---|
| 400 | Non-integer ID, or old checkpoint does not have fewer cumulative receipts than new |
| 404 | Either checkpoint not found |
| 500 | Storage or proof generation failure |

**Example**

```bash
curl http://localhost:8080/v1/checkpoints/2/consistency/1
```

---

## Utility

### GET /v1/health

Liveness and readiness probe.

**Response — 200 OK**

```json
{"status": "ok", "version": "v0.1.0"}
```

Returns **503 Service Unavailable** with `"status": "degraded"` if the database is unreachable.

**Example**

```bash
curl http://localhost:8080/v1/health
```

---

### GET /v1/stats

Aggregate usage statistics.

**Response — 200 OK**

```json
{
  "total_receipts": 1042,
  "total_checkpoints": 8,
  "pending_batch": 17,
  "total_witness_sigs": 24
}
```

| Field | Type | Description |
|---|---|---|
| `total_receipts` | integer | Total receipts ever stored |
| `total_checkpoints` | integer | Total checkpoints ever built |
| `pending_batch` | integer | Receipts not yet included in any checkpoint |
| `total_witness_sigs` | integer | Total witness cosignatures across all checkpoints |

**Example**

```bash
curl http://localhost:8080/v1/stats
```

---

## Data Schemas

### Receipt object

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "version": "1.0.0",
  "tenant": "acme",
  "provider": "openai",
  "model": "gpt-4o",
  "model_digest": "",
  "tokenizer_id": "",
  "tokenizer_hash": "",
  "input_tokens": 150,
  "output_tokens": 75,
  "reasoning_tokens": 0,
  "verified_input_tokens": 0,
  "verified_output_tokens": 0,
  "request_hash": "sha256hex...",
  "response_hash": "sha256hex...",
  "token_boundary_root": "",
  "request_id": "chatcmpl-abc123",
  "timestamp": "2024-01-15T10:30:00Z",
  "signer_identity": "ed25519pubkeyhex...",
  "signature_hex": "ed25519sighex...",
  "verified": true,
  "discrepancy_pct": 0.0
}
```

| Field | Type | Description |
|---|---|---|
| `id` | string (UUID) | Unique receipt identifier |
| `version` | string | Receipt format version (`1.0.0`) |
| `tenant` | string | Tenant identifier |
| `provider` | string | Provider name |
| `model` | string | Model name |
| `model_digest` | string | SHA-256 of model weights (if available) |
| `tokenizer_id` | string | Tokenizer identifier used for local verification |
| `tokenizer_hash` | string | SHA-256 of tokenizer file |
| `input_tokens` | integer | Provider-reported input token count |
| `output_tokens` | integer | Provider-reported output token count |
| `reasoning_tokens` | integer | Provider-reported reasoning/thinking token count |
| `verified_input_tokens` | integer | Locally-verified input token count (Rust tokenizer) |
| `verified_output_tokens` | integer | Locally-verified output token count (Rust tokenizer) |
| `request_hash` | string | SHA-256 hex of raw request bytes |
| `response_hash` | string | SHA-256 hex of raw response bytes |
| `token_boundary_root` | string | Merkle root over token boundary byte offsets |
| `request_id` | string | Provider-assigned request ID |
| `timestamp` | string (RFC3339) | Receipt creation time (UTC) |
| `signer_identity` | string | Operator Ed25519 public key hex |
| `signature_hex` | string | Ed25519 signature over canonical receipt bytes |
| `verified` | boolean | Whether local tokenizer verification was performed |
| `discrepancy_pct` | float | Percentage difference between provider and verified counts |

### Checkpoint object

```json
{
  "id": 1,
  "batch_start": 1,
  "batch_end": 50,
  "tree_size": 50,
  "root_hex": "a3b4c5...",
  "previous_root_hex": "",
  "issued_at": "2024-01-15T10:35:00Z",
  "operator_sig_hex": "ed25519sighex..."
}
```

| Field | Type | Description |
|---|---|---|
| `id` | integer | Checkpoint ID (auto-incremented) |
| `batch_start` | integer | Row ID of the first receipt in this batch |
| `batch_end` | integer | Row ID of the last receipt in this batch |
| `tree_size` | integer | Number of receipts in this checkpoint's Merkle tree |
| `root_hex` | string | Merkle root hex over this batch's receipts |
| `previous_root_hex` | string | Root hex of the immediately preceding checkpoint |
| `issued_at` | string (RFC3339) | Checkpoint creation time (UTC, whole seconds) |
| `operator_sig_hex` | string | Ed25519 signature over the `SignedPayload` bytes |

### Error object

```json
{"error": "human-readable error message"}
```

---

## Supported Providers

| Name | `provider` value | Notes |
|---|---|---|
| OpenAI | `openai` | Supports `o1`/`o3` reasoning tokens via `completion_tokens_details.reasoning_tokens` |
| Anthropic | `anthropic` | Supports cache tokens (`cache_creation_input_tokens`, `cache_read_input_tokens`) |
| Generic (OpenAI-compatible) | `generic` | Ollama, vLLM, LiteLLM, and any OpenAI-schema-compatible endpoint |
