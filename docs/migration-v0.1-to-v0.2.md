# Migration Guide: v0.1 → v0.2

This guide explains how to upgrade an existing siqlah v0.1 deployment to v0.2.

---

## What Changed in v0.2

### New Features
| Feature | Phase | Impact |
|---|---|---|
| Tessera append-only log backend | 16 | Optional; SQLite still works |
| Fulcio keyless signing | 17 | Optional; Ed25519 still default |
| C2SP witness protocol | 18 | Replaces custom witness format |
| OMS model identity verification | 19 | Optional; receipt schema extended |
| Rekor transparency log anchoring | 20 | Optional; operator-configured |
| Energy-per-token reporting | 21 | Automatic for known models |
| x402 payment bridge | 22 | New optional endpoints |

### Breaking Changes
- None. All v0.1 receipt JSON deserializes cleanly under v0.2.
- The witness signature format has changed. Witnesses using the pre-18 custom format must migrate to C2SP cosigning (see below).

### Schema Changes
New optional fields on `Receipt` (all `omitempty` — absent from v0.1 receipts):
- `signer_type` — `"ed25519"` (default) or `"fulcio"`
- `certificate_pem` — leaf certificate for Fulcio-signed receipts
- `rekor_log_index` — Rekor transparency log entry index
- `energy_estimate_joules` — estimated inference energy
- `energy_source` — `"model-benchmark"` or `"none"`
- `carbon_intensity_gco2e_per_kwh` — grid carbon intensity
- `inference_region` — cloud region used for inference

---

## Upgrade Steps

### 1. Update the Binary

```bash
go install github.com/yasserrmd/siqlah/cmd/siqlah@v0.2.0
```

### 2. Run Database Migrations

The v0.2 SQLite migrations run automatically on first start. They are idempotent — safe to run against an existing v0.1 database:

```bash
siqlah --db=siqlah.db
```

The migration adds:
- `rekor_log_index` column to `checkpoints` table (default `-1`)

No data is lost or modified.

### 3. (Optional) Migrate to Tessera Backend

If you want to move from SQLite-only to the Tessera append-only log:

```bash
siqlah migrate \
  --src=siqlah.db \
  --dst=siqlah-v2.db \
  --tessera-storage-path=./tessera-data/ \
  --tessera-log-name=your.log.domain/log \
  --operator-key=<hex-encoded Ed25519 private key>
```

Dry-run first to verify the receipt count:

```bash
siqlah migrate --src=siqlah.db --dry-run
```

After migration, start the server with the Tessera backend:

```bash
siqlah \
  --log-backend=tessera \
  --db=siqlah-v2.db \
  --tessera-storage-path=./tessera-data/ \
  --tessera-log-name=your.log.domain/log \
  --operator-key=<hex>
```

### 4. (Optional) Enable Keyless Signing

Fulcio-based keyless signing requires an OIDC identity provider. Configure it with:

```bash
siqlah \
  --signing-backend=fulcio \
  --fulcio-url=https://fulcio.sigstore.dev \
  --oidc-issuer=https://accounts.google.com \
  --oidc-client-id=<your-client-id>
```

### 5. (Optional) Enable Carbon Reporting

Pass the cloud region where your inference runs:

```bash
siqlah --inference-region=us-east-1
```

Supported regions include all major AWS, GCP, and Azure regions. Unknown regions fall back to the global average (436 gCO₂e/kWh).

### 6. Update Witnesses

Witnesses using the pre-v0.2 custom cosigning format must switch to the C2SP witness protocol. See [docs/witness-protocol.md](witness-protocol.md) for the updated API.

The new C2SP endpoints are:
- `GET /v1/witness/checkpoint` — fetch the latest signed note
- `POST /v1/witness/cosign` — submit a cosignature
- `GET /v1/witness/cosigned-checkpoint` — fetch the fully cosigned note

---

## Configuration Reference

| Flag | Default | Description |
|---|---|---|
| `--log-backend` | `sqlite` | `sqlite` or `tessera` |
| `--tessera-storage-path` | `./tessera-data/` | POSIX tile storage path |
| `--tessera-log-name` | `siqlah.dev/log` | C2SP log origin string |
| `--signing-backend` | `ed25519` | `ed25519` or `fulcio` |
| `--fulcio-url` | `https://fulcio.sigstore.dev` | Fulcio CA endpoint |
| `--oidc-issuer` | `https://accounts.google.com` | OIDC provider |
| `--rekor-url` | _(empty)_ | Rekor endpoint; empty disables |
| `--inference-region` | _(empty)_ | Cloud region for carbon lookup |

---

## Rollback

v0.2 is fully backward-compatible with v0.1 clients. To roll back:

1. Stop the v0.2 server.
2. Start the v0.1 binary against the same SQLite database.
3. The extra columns added by v0.2 migrations are ignored by v0.1.

If you migrated to Tessera, the original SQLite database remains intact and unchanged.
