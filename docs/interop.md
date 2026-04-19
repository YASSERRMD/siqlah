# Interoperability

siqlah v0.2 implements several open standards to maximize interoperability.

---

## Standards Implemented

### C2SP Signed Note Format (Checkpoint)

Checkpoints are formatted as [C2SP signed notes](https://c2sp.org/signed-note):

```
<log-origin>
<tree-size>:<base64-root-hash>

<base64-signature>
```

The signature covers the note body using the operator's Ed25519 key. Consumers can verify using the standard Go `golang.org/x/mod/sumdb/note` package.

**API**: `GET /v1/checkpoints/{id}` with `Accept: text/plain` returns the raw signed note.

**API**: `GET /v1/witness/checkpoint` returns the latest signed note for C2SP witnesses.

### C2SP Witness Cosigning

The witness cosigning protocol follows the [C2SP witness spec](https://c2sp.org/tlog-witness):

- A witness fetches the latest checkpoint note via `GET /v1/witness/checkpoint`.
- The witness signs the note body with its Ed25519 key.
- The witness submits the cosignature via `POST /v1/witness/cosign`.

Cosigned checkpoints (with all witness cosignatures appended) are available at `GET /v1/witness/cosigned-checkpoint`.

### Sigstore Bundle v0.3

Model identity verification uses the [Sigstore bundle format](https://github.com/sigstore/protobuf-specs/blob/main/protos/sigstore_bundle.proto). Bundle JSON must include:

```json
{
  "mediaType": "application/vnd.dev.sigstore.bundle+json;version=0.3",
  "verificationMaterial": {
    "x509CertificateChain": {
      "certificates": [{ "rawBytes": "<base64-DER cert>" }]
    }
  },
  "messageSignature": {
    "messageDigest": { "algorithm": "SHA2_256", "digest": "<base64>" },
    "signature": "<base64 ECDSA P-256 signature>"
  }
}
```

The signer identity is extracted from the certificate's URI SAN (used for OIDC identity).

### Rekor v2 REST API

Checkpoint anchoring uses the [Rekor v2 REST API](https://rekor.sigstore.dev):

- **POST** `/api/v1/log/entries` — submits a `hashedrekord` entry for a checkpoint hash.
- **GET** `/api/v1/log/entries?logIndex=N` — retrieves and verifies an existing entry.

The anchoring uses an ephemeral ECDSA P-256 key per `RekorAnchor` instance.

### x402 Payment Protocol

The optional payment bridge implements the [HTTP 402 x402 protocol](https://x402.org):

- `X-Payment` header carries a base64-encoded `PaymentAuthorization` JSON object.
- Missing or invalid header → HTTP 402 with a `PaymentRequired` body listing accepted schemes.
- Currently supports `x402/evm-token` on EVM-compatible networks.

---

## Supported Providers

| Provider | Adapter | Token Fields |
|---|---|---|
| OpenAI | `openai` | `prompt_tokens`, `completion_tokens` |
| Anthropic | `anthropic` | `input_tokens`, `output_tokens` |
| Generic | `generic` | Any `input_tokens` / `output_tokens` |

Custom providers implement the `vur.ProviderAdapter` interface:

```go
type ProviderAdapter interface {
    ParseUsage(responseBody json.RawMessage) (*UsageData, error)
}
```

---

## Clients

Any HTTP client can interact with the siqlah API. Example with curl:

```bash
# Ingest a receipt
curl -X POST http://localhost:8080/v1/receipts \
  -H "Content-Type: application/json" \
  -d '{"provider":"openai","tenant":"my-tenant","model":"gpt-4o","response_body":{"usage":{"prompt_tokens":100,"completion_tokens":50}}}'

# Fetch the signed note checkpoint (C2SP format)
curl -H "Accept: text/plain" http://localhost:8080/v1/checkpoints/1

# Submit a cosignature
curl -X POST http://localhost:8080/v1/witness/cosign \
  -H "Content-Type: application/json" \
  -d '{"witness_id":"<hex-pub-key>","note":"<signed-note>","sig_hex":"<hex-sig>"}'
```

---

## Verification

Receipts can be verified independently of the siqlah server:

```go
import "github.com/yasserrmd/siqlah/pkg/vur"

pubKey := ed25519.PublicKey(mustDecodeHex(operatorPubHex))
if err := vur.VerifyReceipt(&receipt, pubKey); err != nil {
    log.Fatalf("receipt invalid: %v", err)
}
```

Checkpoints can be verified using the standard Go note verifier:

```go
import "golang.org/x/mod/sumdb/note"

verifier, _ := note.NewVerifier(operatorVerifierKey)
n, _ := note.Open(checkpointBytes, note.VerifierList(verifier))
```

---

## Known Limitations

- **Tessera Merkle proofs**: Inclusion and consistency proofs require the Tessera backend; the SQLite backend returns `ErrNotSupported`.
- **Rekor anchoring**: On-chain anchoring requires network access to a Rekor instance. Offline or air-gapped deployments should set `--rekor-url=""`.
- **x402 on-chain verification**: The payment bridge performs structural validation only. On-chain transaction verification requires an external EVM RPC provider.
- **Fulcio OIDC flow**: Keyless signing requires an interactive OIDC token flow; it is not suitable for fully automated server deployments without a credential helper.
