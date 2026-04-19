# VUR Receipt Specification

**Version:** 1.0.0  
**Status:** Draft

A **Verifiable Usage Receipt (VUR)** is a cryptographically signed record that proves the token counts billed for a single AI API call. This document is the normative specification for the receipt format, canonical serialization, and signature scheme.

---

## 1. Receipt Fields

A receipt contains exactly the following fields. All fields are always present in the canonical representation; absent optional data uses zero values (`""`, `0`, `false`, `0.0`).

| Field | JSON key | Type | Description |
|---|---|---|---|
| ID | `id` | string (UUID v4) | Globally unique receipt identifier |
| Version | `version` | string | Spec version, currently `"1.0.0"` |
| Tenant | `tenant` | string | Operator-assigned tenant identifier |
| Provider | `provider` | string | Provider name: `openai`, `anthropic`, or `generic` |
| Model | `model` | string | Model name as reported by the provider |
| Model Digest | `model_digest` | string | SHA-256 of model weights, if available; empty otherwise |
| Tokenizer ID | `tokenizer_id` | string | HuggingFace tokenizer identifier used for local verification |
| Tokenizer Hash | `tokenizer_hash` | string | SHA-256 hex of the tokenizer JSON file |
| Input Tokens | `input_tokens` | integer (int64) | Provider-reported input token count |
| Output Tokens | `output_tokens` | integer (int64) | Provider-reported output token count |
| Reasoning Tokens | `reasoning_tokens` | integer (int64) | Provider-reported reasoning/thinking tokens (OpenAI o-series) |
| Verified Input Tokens | `verified_input_tokens` | integer (int64) | Locally-verified input count (Rust tokenizer); 0 if not run |
| Verified Output Tokens | `verified_output_tokens` | integer (int64) | Locally-verified output count (Rust tokenizer); 0 if not run |
| Request Hash | `request_hash` | string (hex) | SHA-256 of the raw request bytes; empty if not provided |
| Response Hash | `response_hash` | string (hex) | SHA-256 of the raw response bytes |
| Token Boundary Root | `token_boundary_root` | string (hex) | Merkle root over token boundary byte offsets; empty if Rust tokenizer unavailable |
| Request ID | `request_id` | string | Provider-assigned request ID |
| Timestamp | `timestamp` | string (RFC3339Nano UTC) | Receipt creation time |
| Signer Identity | `signer_identity` | string (hex) | Operator Ed25519 public key |
| Signature | `signature_hex` | string (hex) | Ed25519 signature over canonical bytes (excluded from canonical bytes during signing) |
| Verified | `verified` | boolean | Whether local tokenizer verification was performed |
| Discrepancy Pct | `discrepancy_pct` | float64 | `(verified - provider) / provider * 100`; 0 if not verified |

---

## 2. Canonical Serialization

Canonical bytes are the authoritative bytes signed by the operator. They must be byte-for-byte reproducible across any conforming implementation.

### Rules

1. **Field order** — fields are serialized in alphabetical order by JSON key name.
2. **No whitespace** — compact JSON with no spaces or newlines.
3. **Timestamp format** — `r.Timestamp` is formatted as `"2006-01-02T15:04:05.999999999Z"` (RFC3339Nano, trailing zeros stripped, always UTC, always `Z` suffix).
4. **Signature excluded** — the `signature_hex` field is **not present** in canonical bytes. Set `signature_hex = ""` before serializing.
5. **All fields present** — all 21 non-signature fields appear even when zero/empty.

### Canonical field order

```
discrepancy_pct
id
input_tokens
model
model_digest
output_tokens
provider
reasoning_tokens
request_hash
request_id
response_hash
signer_identity
tenant
timestamp
token_boundary_root
tokenizer_hash
tokenizer_id
verified
verified_input_tokens
verified_output_tokens
version
```

### Example

```json
{"discrepancy_pct":0,"id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","input_tokens":150,"model":"gpt-4o","model_digest":"","output_tokens":75,"provider":"openai","reasoning_tokens":0,"request_hash":"","request_id":"chatcmpl-abc123","response_hash":"a1b2c3...","signer_identity":"pubkeyhex...","tenant":"acme","timestamp":"2024-01-15T10:30:00Z","token_boundary_root":"","tokenizer_hash":"","tokenizer_id":"","verified":true,"verified_input_tokens":0,"verified_output_tokens":0,"version":"1.0.0"}
```

---

## 3. Signature Scheme

- **Algorithm:** Ed25519 (RFC 8032)
- **Input:** canonical bytes as specified in Section 2
- **Output:** 64-byte signature, stored as lowercase hex in `signature_hex`

### Signing procedure

```
1. Set receipt.signature_hex = ""
2. canonical_bytes = CanonicalBytes(receipt)
3. receipt.signature_hex = hex(ed25519_sign(operator_private_key, canonical_bytes))
```

### Verification procedure

```
1. saved_sig = receipt.signature_hex
2. receipt.signature_hex = ""
3. canonical_bytes = CanonicalBytes(receipt)
4. receipt.signature_hex = saved_sig
5. assert ed25519_verify(operator_public_key, canonical_bytes, decode_hex(saved_sig))
```

The signer identity (`signer_identity`) is the operator's Ed25519 public key encoded as lowercase hex. Verifiers **must** check that `signer_identity` matches the public key used for verification; failure to do so allows signature substitution attacks.

---

## 4. Merkle Inclusion

Receipts are committed into RFC 6962 Merkle trees via the Checkpoint engine (see `docs/architecture.md`, Layer 3). The leaf hash for receipt `r` is:

```
leaf_hash = SHA256(0x00 || canonical_bytes(r))
```

The `0x00` domain separator distinguishes leaf hashes from internal node hashes (`0x01`), preventing second-preimage attacks.

A receipt can be proven included in a checkpoint by presenting:
- The receipt's leaf index within the checkpoint's tree
- The Merkle inclusion proof (sibling hashes from leaf to root)
- The checkpoint's signed root

Use `GET /v1/receipts/{id}/proof` to obtain these values.

---

## 5. Versioning Policy

The `version` field identifies the receipt format version. The current version is `"1.0.0"`.

| Version | Breaking changes |
|---|---|
| 1.0.0 | Initial specification |

**Backward compatibility:** A future `1.x.0` version may add new fields with zero defaults. Verifiers MUST ignore unknown fields during signature verification (by zeroing them before computing canonical bytes). A `2.x.0` version may change the canonical field order or algorithm and is not backward-compatible.

Operators must not mix receipts of different versions within the same checkpoint batch.

---

## 6. Implementation Notes

- The Go reference implementation uses a shadow struct (`canonicalReceipt`) with alphabetically-ordered JSON struct tags to guarantee field order independent of the Go runtime's map iteration order.
- The `timestamp` field in the canonical bytes uses trailing-zero-stripped RFC3339Nano (`"2006-01-02T15:04:05.999999999Z"` Go format). A timestamp like `10:30:00.000000000Z` serializes as `10:30:00Z`.
- SQLite stores timestamps as Unix integer seconds. The builder truncates `time.Now()` to whole seconds before signing to ensure the stored and reconstructed timestamps produce identical canonical bytes.
- The `verified` boolean is included in canonical bytes and must be set to its final value **before** signing. The server sets `verified = true` after parsing provider usage.
