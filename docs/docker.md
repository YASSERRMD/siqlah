# Docker Deployment Guide

## Quick Start

```bash
# Build the image
docker compose -f deployments/docker-compose.yml build

# Start the ledger (single node, no witnesses)
docker compose -f deployments/docker-compose.yml up siqlah

# Verify it's running
curl http://localhost:8080/v1/health
```

## Starting with Witnesses

Witnesses need the operator's public key. Get it after the first start:

```bash
# Start the ledger
docker compose -f deployments/docker-compose.yml up -d siqlah

# Retrieve the operator public key from logs
docker logs siqlah 2>&1 | grep "operator public key"
# Output: operator public key: aabb...ff

# Set the env var and start witnesses
export SIQLAH_OPERATOR_PUB=aabb...ff
docker compose -f deployments/docker-compose.yml up -d
```

## Configuration via Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SIQLAH_ADDR` | `:8080` | HTTP listen address |
| `SIQLAH_DB` | `/data/siqlah.db` | SQLite database path |
| `SIQLAH_BATCH_INTERVAL` | `30s` | Checkpoint build interval |
| `SIQLAH_MAX_BATCH` | `1000` | Max receipts per checkpoint |
| `SIQLAH_MONITOR` | `` | Set to any value to enable monitor |
| `SIQLAH_DISCREPANCY_THRESHOLD` | `5.0` | Alert threshold percent |
| `SIQLAH_ALERT_WEBHOOK` | `` | Webhook URL for discrepancy alerts |
| `SIQLAH_WITNESSES` | `` | Comma-separated `id=pubhex` pairs |

## Example: Ingest a Receipt

```bash
curl -X POST http://localhost:8080/v1/receipts \
  -H "Content-Type: application/json" \
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

## Example: Build a Checkpoint

```bash
curl -X POST http://localhost:8080/v1/checkpoints/build
```

## Example: Verify a Receipt Inclusion Proof

```bash
# Get inclusion proof for a receipt
RECEIPT_ID="<uuid from ingest>"
curl http://localhost:8080/v1/receipts/$RECEIPT_ID/proof

# Verify locally with the CLI
./bin/verifier check-proof --receipt receipt.json --ledger http://localhost:8080
```

## Production Considerations

1. **Persist the operator key**: Use `--operator-key` or `OPERATOR_KEY` to provide a stable key across restarts. If the key changes, all existing signatures become unverifiable.

2. **SQLite path**: Mount a persistent volume for `/data`. Consider switching to a networked database for horizontal scaling.

3. **TLS**: Run siqlah behind a reverse proxy (nginx, Caddy) with TLS termination.

4. **Witness redundancy**: Run at least 2 independent witnesses from different operators to ensure meaningful cosigning.

5. **Monitoring**: Enable `SIQLAH_MONITOR=true` with a webhook to receive alerts when provider token counts differ from locally re-verified counts.

## Building from Source

```bash
# Build Rust tokenizer first
make build-tokenizer

# Build all Go binaries
make build

# Run tests
make test
```
