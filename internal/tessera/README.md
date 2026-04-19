# Tessera Integration Notes

## What is Tessera?

Trillian Tessera (`github.com/transparency-dev/tessera`) is Google's production-grade successor to Trillian, designed for append-only transparency logs using the [tlog-tiles](https://c2sp.org/tlog-tiles) C2SP specification.

## Key Concepts

### Driver / Storage Backend

Tessera separates the log logic from storage. Storage backends implement the `tessera.Driver` interface:
- `posix` — files on local filesystem (development/single-node)
- `gcp` — Google Cloud Storage + Spanner
- `aws` — S3 + DynamoDB
- `mysql` — MySQL

Usage:
```go
driver, err := posix.New(ctx, posix.Config{Path: "/var/lib/siqlah/tessera"})
```

### Appender Lifecycle

`tessera.NewAppender(ctx, driver, opts)` returns:
- `*Appender` with an `Add(ctx, *Entry) IndexFuture` method
- A shutdown function `func(ctx) error` to call on graceful stop
- A `LogReader` for reading tiles, checkpoints, entry bundles

```go
appender, shutdown, reader, err := tessera.NewAppender(ctx, driver,
    tessera.NewAppendOptions().WithCheckpointSigner(noteSigner))

future := appender.Add(ctx, tessera.NewEntry(data))
index, err := future()  // blocks until index is assigned
```

### Entries

Each entry in the log is raw bytes. Tessera computes the RFC 6962 leaf hash internally using `rfc6962.DefaultHasher.HashLeaf(data)`.

```go
entry := tessera.NewEntry(canonicalReceiptBytes)
```

### Checkpoints

Tessera signs checkpoints using the C2SP [signed note](https://pkg.go.dev/golang.org/x/mod/sumdb/note) format:
```
<origin>
<tree_size>
<root_hash_base64>

— <signer_name> <key_hint_base64> <sig_base64>
```

The checkpoint signer is a `note.Signer` created from an Ed25519 private key.

### Proof Generation

Proofs are generated via `client.NewProofBuilder`:
```go
pb, err := client.NewProofBuilder(ctx, treeSize, reader.ReadTile)
inclProof, err := pb.InclusionProof(ctx, leafIndex)
conProof, err := pb.ConsistencyProof(ctx, oldSize, newSize)
```

### Static Tile Files

The log is stored as static files following the tlog-tiles spec:
- `tile/2/<level>/<index>` — Merkle tree tiles
- `tile/entries/<index>` — entry data bundles
- `checkpoint` — the latest signed checkpoint
- `.state/` — internal state (not public)

## Tessera vs Custom Merkle

| Feature | Custom (`internal/merkle`) | Tessera |
|---|---|---|
| Tree structure | RFC 6962, in-memory | RFC 6962 tlog-tiles, disk tiles |
| Storage | SQLite `receipts` table | POSIX/GCS/S3/MySQL tile files |
| Proofs | Computed on demand from DB | Served from static tile reads |
| Checkpoints | Custom JSON + Ed25519 | C2SP signed note format |
| Scale | Thousands of entries | Millions+ entries |
| Witness support | Custom protocol | Built-in C2SP witness protocol |

## Deprecation Note

`internal/merkle/` is retained for the SQLite (legacy) backend. New deployments should use Tessera. See the `--log-backend` flag.
