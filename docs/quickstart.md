# siqlah Quickstart

Get siqlah running and issue your first verifiable receipt in 5 minutes.

## Prerequisites

- Go 1.21+
- Rust 1.70+ (for the tokenizer; optional but recommended)
- `make`

## 1. Build

```bash
git clone https://github.com/yasserrmd/siqlah
cd siqlah
make build
```

This produces three binaries in `bin/`:
- `siqlah` — the main API server
- `siqlah-witness` — the witness CLI
- `siqlah-verifier` — the client-side verifier

## 2. Generate an Operator Key

```bash
./bin/siqlah-witness keygen --out operator.key
# Operator public key: a1b2c3d4...
```

Keep `operator.key` secret. The public key is shown on stdout and embedded in every receipt.

## 3. Start the Server

```bash
./bin/siqlah \
  --operator-key ./operator.key \
  --db ./siqlah.db \
  --addr :8080
```

Check it's up:

```bash
curl http://localhost:8080/v1/health
# {"status":"ok","version":"dev"}
```

## 4. Ingest an API Response

Paste a raw OpenAI response body to create a signed receipt:

```bash
curl -X POST http://localhost:8080/v1/receipts \
  -H 'Content-Type: application/json' \
  -d '{
    "provider": "openai",
    "tenant": "my-app",
    "model": "gpt-4o",
    "response_body": {
      "id": "chatcmpl-abc123",
      "usage": {
        "prompt_tokens": 150,
        "completion_tokens": 75
      }
    }
  }'
```

The response is a signed receipt. Save the `id` field — you will need it for the proof:

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "input_tokens": 150,
  "output_tokens": 75,
  "signature_hex": "...",
  ...
}
```

Works the same for Anthropic responses:

```bash
curl -X POST http://localhost:8080/v1/receipts \
  -H 'Content-Type: application/json' \
  -d '{
    "provider": "anthropic",
    "tenant": "my-app",
    "model": "claude-3-5-sonnet-20241022",
    "response_body": {
      "usage": {
        "input_tokens": 200,
        "output_tokens": 100,
        "cache_creation_input_tokens": 50
      }
    }
  }'
```

## 5. Build a Checkpoint

A checkpoint commits a batch of receipts into a signed Merkle tree:

```bash
curl -X POST http://localhost:8080/v1/checkpoints/build
```

```json
{
  "id": 1,
  "tree_size": 2,
  "root_hex": "a3b4c5...",
  "operator_sig_hex": "..."
}
```

Checkpoints are also built automatically on the `--batch-interval` schedule (default: 60 seconds).

## 6. Get an Inclusion Proof

Prove that your receipt is in the checkpoint:

```bash
RECEIPT_ID="f47ac10b-58cc-4372-a567-0e02b2c3d479"
curl http://localhost:8080/v1/receipts/$RECEIPT_ID/proof
```

```json
{
  "receipt_id": "f47ac10b-...",
  "checkpoint_id": 1,
  "leaf_index": 0,
  "tree_size": 2,
  "root_hex": "a3b4c5...",
  "proof": ["d4e5f6..."]
}
```

Verify it locally without trusting the server:

```bash
./bin/siqlah-verifier check-proof \
  --server http://localhost:8080 \
  --receipt-id $RECEIPT_ID
```

## 7. Add a Witness

Start a second terminal and generate a witness key:

```bash
./bin/siqlah-witness keygen --out witness.key
# Witness public key: 9f8e7d...
```

Cosign checkpoint 1:

```bash
./bin/siqlah-witness cosign \
  --server http://localhost:8080 \
  --checkpoint-id 1 \
  --key ./witness.key \
  --operator-key ./operator.key
```

Verify the checkpoint now has a cosignature:

```bash
curl http://localhost:8080/v1/checkpoints/1/verify
```

```json
{
  "operator_valid": true,
  "witnesses": {
    "9f8e7d...": "cosig_hex..."
  }
}
```

## 8. Run Continuous Witnessing

For production, run the witness in watch mode:

```bash
./bin/siqlah-witness watch \
  --server http://localhost:8080 \
  --key ./witness.key \
  --operator-key ./operator.key \
  --interval 30s
```

## 9. Docker Compose (Optional)

Run siqlah with two witnesses using Docker:

```bash
cd deployments
docker compose up
```

Services:
- `siqlah` — API server on port 8080
- `witness-1`, `witness-2` — auto-cosigning witnesses

## Next Steps

- Read the [API Reference](api.md) for every endpoint
- Read the [Receipt Specification](receipt-spec.md) to understand the signature format
- Read the [Witness Protocol](witness-protocol.md) to run your own witness
- Read the [Architecture](architecture.md) for the full four-layer design
