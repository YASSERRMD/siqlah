# In-Toto Attestation Support

siqlah can expose any receipt as an **in-toto v1 Statement**, turning AI usage receipts into machine-verifiable provenance artifacts that integrate with the broader SLSA and software supply chain tooling ecosystem.

---

## Background

[in-toto](https://in-toto.io) is a framework for supply chain integrity. An **in-toto Statement** is a JSON envelope that identifies *what* artifacts are being attested (subjects), *what kind* of claim is being made (predicate type), and *the claim itself* (predicate).

siqlah uses:

| Field | Value |
|---|---|
| `_type` | `https://in-toto.io/Statement/v1` |
| `predicateType` | `https://siqlah.dev/receipt/v1` |
| `subject[0]` | `{ name: "request:<id>", digest: { sha256: <request_hash> } }` |
| `subject[1]` | `{ name: "response:<id>", digest: { sha256: <response_hash> } }` |
| `predicate` | Full receipt JSON + optional model provenance + energy footprint |

The `subject` hashes are the same SHA-256 digests computed at ingest time and stored in the append-only Merkle log — the attestation is directly tied to the verifiable audit trail.

---

## API

### `GET /v1/receipts/{id}/attestation`

Returns an in-toto v1 Statement for the given receipt.

**Response**

```
HTTP/1.1 200 OK
Content-Type: application/vnd.in-toto+json
```

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "request:550e8400-e29b-41d4-a716-446655440000",
      "digest": { "sha256": "aabb...ccdd" }
    },
    {
      "name": "response:550e8400-e29b-41d4-a716-446655440000",
      "digest": { "sha256": "1122...33dd" }
    }
  ],
  "predicateType": "https://siqlah.dev/receipt/v1",
  "predicate": {
    "receipt": { ... },
    "model_provenance": {
      "model_name": "gpt-4o",
      "model_digest": "sha256:abc...",
      "signer_identity": "signer@example.com",
      "verified": true
    },
    "energy_footprint": {
      "joules": 3600.0,
      "carbon_gco2e": 0.2,
      "region": "us-east-1"
    }
  }
}
```

`model_provenance` is omitted when model identity verification is not configured.
`energy_footprint` is omitted when no energy estimate is available.

---

## Using Attestations

### Verify with `cosign`

```bash
# Fetch the attestation
curl http://localhost:8080/v1/receipts/550e8400.../attestation \
  -o receipt-attest.json

# Sign it with cosign (add your own identity layer)
cosign attest --type custom \
  --predicate receipt-attest.json \
  --key cosign.key \
  ghcr.io/yourorg/your-model-image:latest
```

### Ingest into GUAC

[GUAC](https://guac.sh) (Graph for Understanding Artifact Composition) can ingest in-toto attestations directly:

```bash
guacone collect files receipt-attest.json
```

GUAC will link the receipt subjects (request and response hashes) into its artifact graph, enabling policy queries like "which AI calls in this pipeline were verified against a keyless certificate?"

### Policy evaluation with `rego` / OPA

```rego
import future.keywords.if

deny if {
    input.predicate.model_provenance.verified == false
    input.predicate.receipt.input_tokens > 10000
}
```

---

## How Subjects Map to the Log

The `subject` hashes in the attestation are the same values stored in the siqlah Merkle log. You can cross-reference an attestation with the inclusion proof:

```bash
# Get the attestation
curl http://localhost:8080/v1/receipts/$ID/attestation | jq '.subject[0].digest.sha256'

# Get the Merkle inclusion proof for the same receipt
curl http://localhost:8080/v1/receipts/$ID/proof | jq '.proof'
```

This ties the SLSA provenance claim to a tamper-evident, append-only audit trail — an external verifier can confirm the receipt was included in the log *before* the attestation was issued.

---

## Predicate Schema Reference

The `https://siqlah.dev/receipt/v1` predicate has this structure:

```json
{
  "receipt": {
    "id": "...",
    "version": "1.0",
    "tenant": "...",
    "provider": "...",
    "model": "...",
    "input_tokens": 0,
    "output_tokens": 0,
    "request_hash": "...",
    "response_hash": "...",
    "timestamp": "...",
    "verified": true
  },
  "model_provenance": {
    "model_name": "...",
    "model_digest": "sha256:...",
    "signer_identity": "...",
    "verified": true
  },
  "energy_footprint": {
    "joules": 0.0,
    "carbon_gco2e": 0.0,
    "region": "..."
  }
}
```

All top-level fields except `receipt` are optional and omitted when not available.

---

## Related Documents

- [`docs/witness-protocol.md`](witness-protocol.md) — siqlah's C2SP witness protocol
- [`docs/witness-interop.md`](witness-interop.md) — connecting to external witnesses
- [in-toto specification v1](https://github.com/in-toto/attestation/tree/main/spec/v1)
- [SLSA provenance](https://slsa.dev/provenance)
- [GUAC documentation](https://guac.sh/docs)
