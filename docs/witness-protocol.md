# Witness Protocol Specification

**Version:** 1.0.0  
**Status:** Draft

Witnesses are independent parties that independently verify and cosign siqlah checkpoints. Multiple witnesses provide k-of-n trust without requiring blockchain coordination.

---

## 1. Overview

After the operator builds and signs a checkpoint, any party who trusts neither the operator alone nor the storage layer can become a witness:

1. Fetch the checkpoint from the ledger
2. Verify the operator's Ed25519 signature
3. Produce an independent Ed25519 cosignature over the same payload
4. Submit the cosignature back to the ledger

Clients configure which witnesses they trust. A checkpoint is considered witnessed when it carries cosignatures from at least `k` trusted witnesses.

---

## 2. Signed Payload

Both the operator signature and all witness cosignatures cover the same byte sequence: the JSON serialization of `SignedPayload`.

```json
{
  "batch_start": 1,
  "batch_end": 50,
  "tree_size": 50,
  "root_hex": "a3b4c5...",
  "previous_root_hex": "",
  "issued_at": "2024-01-15T10:35:00Z"
}
```

### Fields

| Field | Type | Description |
|---|---|---|
| `batch_start` | integer | Row ID of the first receipt in this batch |
| `batch_end` | integer | Row ID of the last receipt in this batch |
| `tree_size` | integer | Number of receipts in this checkpoint's Merkle tree |
| `root_hex` | string | Merkle root hex over this batch's receipts |
| `previous_root_hex` | string | Root hex of the preceding checkpoint; empty for the first |
| `issued_at` | string (RFC3339Nano UTC) | Checkpoint issuance time (whole seconds — no sub-second precision) |

### Serialization rules

- Standard `json.Marshal` field order (declaration order: `batch_start`, `batch_end`, `tree_size`, `root_hex`, `previous_root_hex`, `issued_at`)
- Compact JSON, no whitespace
- `issued_at` uses Go format `"2006-01-02T15:04:05.999999999Z"` — trailing zeros stripped, always `Z`

---

## 3. Witness Verification Procedure

A witness MUST perform these steps before cosigning:

1. **Fetch the checkpoint** via `GET /v1/checkpoints/{id}` and record the full checkpoint object.

2. **Reconstruct the signed payload** from the checkpoint fields.

3. **Verify the operator signature:**
   ```
   payload_bytes = json_compact(SignedPayload)
   assert ed25519_verify(operator_public_key, payload_bytes, decode_hex(checkpoint.operator_sig_hex))
   ```
   The operator's public key is obtained out-of-band (e.g., from a trusted public key registry or the siqlah deployment's `/v1/health` response). A witness MUST reject checkpoints with invalid operator signatures.

4. **Optionally verify receipt integrity** (recommended):
   - Fetch receipts in the batch via the store or re-fetch from the originating provider
   - Recompute `leaf_hash = SHA256(0x00 || canonical_bytes(receipt))` for each receipt
   - Rebuild the Merkle root and compare against `checkpoint.root_hex`

5. **Produce cosignature:**
   ```
   cosig = ed25519_sign(witness_private_key, payload_bytes)
   ```

6. **Submit the cosignature** via `POST /v1/checkpoints/{id}/witness`:
   ```json
   {
     "witness_id": "witness_ed25519_pubkey_hex",
     "sig_hex": "cosig_hex"
   }
   ```

---

## 4. Witness Identity

A witness is identified by its Ed25519 public key, encoded as lowercase hex. The `witness_id` field in the submission is this public key.

Witness key pairs are generated with:

```bash
siqlah-witness keygen --out /etc/siqlah/witness.key
```

The public key is printed to stdout and must be registered with clients that will trust this witness.

---

## 5. Client Trust Policy (k-of-n)

Clients configure a trusted witness set. A checkpoint is considered sufficiently witnessed when at least `k` signatures from the trusted set are present and valid.

### Verification

```
trusted_witnesses = { "pubkey1hex", "pubkey2hex", ... }
k = 2

valid_cosigs = 0
for witness_id, sig_hex in checkpoint.witnesses:
    if witness_id in trusted_witnesses:
        payload_bytes = json_compact(SignedPayload from checkpoint)
        if ed25519_verify(decode_hex(witness_id), payload_bytes, decode_hex(sig_hex)):
            valid_cosigs += 1

assert valid_cosigs >= k
```

The server does **not** enforce the k-of-n policy — it accepts any cosignature from any witness. Policy enforcement is a client-side responsibility.

---

## 6. Chain Integrity

Each checkpoint records `previous_root_hex`, the Merkle root of the immediately preceding checkpoint. This forms an append-only hash chain. Clients can detect log tampering by:

1. Fetching all checkpoints in order
2. Verifying that `checkpoints[i].previous_root_hex == checkpoints[i-1].root_hex`
3. For any two checkpoints, verifying a consistency proof via `GET /v1/checkpoints/{new_id}/consistency/{old_id}`

The consistency proof uses cumulative tree sizes (all receipts from row 1 to each checkpoint's `batch_end`), not per-batch sizes.

---

## 7. Automated Witness Operation

The `siqlah-witness` CLI supports continuous cosigning:

```bash
# Watch for new checkpoints and cosign each one
siqlah-witness watch \
  --server http://siqlah.example.com:8080 \
  --key /etc/siqlah/witness.key \
  --operator-key /etc/siqlah/operator.pub \
  --interval 30s
```

In watch mode the witness:
1. Lists all checkpoints via `GET /v1/checkpoints`
2. Skips any already cosigned by this witness
3. Verifies and cosigns each new checkpoint
4. Sleeps for `--interval` then repeats

---

## 8. C2SP Compatibility

The witness protocol is designed to be compatible with the [C2SP transparency log witness protocol](https://c2sp.org/tlog-witness). Key similarities:

- Ed25519 signatures
- Cosignature covers the same signed tree head bytes as the operator
- Witnesses are identified by public key

Current differences from C2SP:
- siqlah uses compact JSON encoding for the signed payload rather than a structured binary format
- The note format is not yet implemented (planned for v2.0.0)

---

## 9. Security Considerations

**Witness key compromise:** If a witness key is compromised, remove it from client trusted sets. Past cosignatures on checkpoints cannot be revoked, but clients can re-verify with the remaining trusted witnesses. The witness network provides redundancy: compromise of one witness does not invalidate properly k-of-n-witnessed checkpoints.

**Sybil witnesses:** All cosignatures on a checkpoint are public. Clients should use a diverse witness set (geographically and organizationally) to reduce correlated failure risk.

**Payload replay:** The `issued_at` timestamp and `previous_root_hex` chain ensure that a valid cosignature from a past checkpoint cannot be replayed against a future checkpoint with a different root.
