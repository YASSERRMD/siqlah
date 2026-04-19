# Changelog

All notable changes to siqlah are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). siqlah uses [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added

**Phase 15 — Documentation & CI**
- Architecture document (`docs/architecture.md`) with four-layer diagram, data flow, threat model, and comparison with related approaches
- API reference (`docs/api.md`) with every endpoint, request/response schemas, error codes, and curl examples
- Receipt specification (`docs/receipt-spec.md`) — normative format, canonical serialization rules, signing procedure, versioning policy
- Witness protocol specification (`docs/witness-protocol.md`) — verification procedure, k-of-n trust policy, chain integrity, C2SP compatibility notes
- Quickstart guide (`docs/quickstart.md`) — 5-minute tutorial from `git clone` to verified cosigned receipt
- `CONTRIBUTING.md` — development setup, conventions, test instructions, PR guidelines
- GitHub Actions CI workflow (`.github/workflows/ci.yml`) — test, lint, go vet, go mod tidy check, and benchmark smoke jobs

**Phase 14 — Test Suite**
- End-to-end integration tests (`test/e2e_test.go`): 100-receipt ingest → checkpoint → inclusion proof → witness cosign → consistency proof
- Discrepancy detection test (`test/discrepancy_test.go`): verifies monitor alerts on inflated token counts
- Concurrent stress test (`test/stress_test.go`): 10 goroutines × 100 receipts, parallel ingest under lock
- Exhaustive Merkle proof tests (`test/proof_test.go`): all tree sizes 1–20, tamper detection, consistency proofs
- Fuzz tests (`test/fuzz_test.go`): `FuzzOpenAIAdapter`, `FuzzAnthropicAdapter`, `FuzzGenericAdapter`
- Benchmark suite (`test/bench_test.go`): canonical bytes, Ed25519 sign/verify, Merkle at 1k/10k/100k leaves, SQLite insert, full ingest flow

**Phase 13 — Docker Deployment**
- Multi-stage Dockerfile: Rust build → Go CGo build → `debian:bookworm-slim` runtime
- `docker-compose.yml` with `siqlah` + `witness-1` + `witness-2` services
- `entrypoint.sh` supporting `server` and `witness` modes with automatic key generation
- Docker deployment guide (`docs/docker.md`)

**Phase 12 — Main Service Entry Point**
- `cmd/siqlah/main.go` with flags: `--addr`, `--db`, `--operator-key`, `--batch-interval`, `--max-batch`, `--witnesses`, `--monitor`, `--discrepancy-threshold`, `--alert-webhook`
- JSON configuration file support (`internal/config/config.go`)
- Periodic checkpoint batcher goroutine
- Graceful shutdown on SIGINT/SIGTERM
- Build-time version and commit SHA injection via `go build -ldflags`

**Phase 11 — Discrepancy Monitor**
- `internal/monitor/monitor.go` — polling daemon comparing provider vs. locally-verified token counts
- `internal/monitor/alert.go` — `LogAlerter`, `WebhookAlerter` (POST JSON), `MultiAlerter`, `MemoryAlerter`

**Phase 10 — Client-Side Verifier**
- `cmd/verifier/main.go` — `verify-receipt`, `verify-tokens`, `check-proof`, `reconcile` subcommands
- Local Merkle inclusion proof verification without trusting the server

**Phase 9 — Witness CLI**
- `cmd/witness/main.go` — `keygen`, `cosign`, `verify`, `watch` subcommands
- Watch mode: continuous polling and cosigning of new checkpoints

**Phase 8 — HTTP API**
- `POST /v1/receipts` — single receipt ingest
- `POST /v1/receipts/batch` — batch ingest
- `GET /v1/receipts/{id}` — receipt fetch by UUID
- `GET /v1/receipts/{id}/proof` — Merkle inclusion proof
- `POST /v1/checkpoints/build` — manual checkpoint trigger
- `GET /v1/checkpoints` — paginated checkpoint list
- `GET /v1/checkpoints/{id}` — checkpoint fetch
- `GET /v1/checkpoints/{id}/verify` — operator sig + witness cosig verification
- `POST /v1/checkpoints/{id}/witness` — witness cosignature submission
- `GET /v1/checkpoints/{id}/consistency/{old_id}` — consistency proof between checkpoints
- `GET /v1/health`, `GET /v1/stats`

**Phase 7 — Rust Tokenizer**
- `tokenizer-rs/` — `siqlah-tokenizer` Rust crate using HuggingFace `tokenizers`
- C FFI exports (`siqlah_tokenize`, `siqlah_free`)
- Go CGo wrapper (`internal/tokenizer/`) with graceful degradation when shared library unavailable

**Phase 6 — Checkpoint Engine**
- `internal/checkpoint/builder.go` — `BuildAndSign()`: fetch unbatched receipts, build Merkle root, sign `SignedPayload`, mark receipts as batched
- `internal/checkpoint/payload.go` — `SignedPayload` with deterministic JSON serialization
- Chain integrity via `PreviousRootHex` linking

**Phase 5 — Provider Adapters**
- `internal/provider/openai.go` — OpenAI adapter with `o1`/`o3` reasoning token support
- `internal/provider/anthropic.go` — Anthropic adapter with cache token fields
- `internal/provider/generic.go` — Generic OpenAI-compatible adapter (Ollama, vLLM, LiteLLM)
- `internal/provider/registry.go` — provider registry

**Phase 4 — SQLite Store**
- `internal/store/sqlite.go` — `modernc.org/sqlite` (pure Go, no CGo dependency for storage)
- Schema with `receipts` and `checkpoints` tables plus `witness_sigs`
- `AppendReceipt`, `GetReceiptByID`, `GetReceiptsByRange`, `FetchUnbatched`, `MarkBatched`
- `SaveCheckpoint`, `GetCheckpoint`, `LatestCheckpoint`, `ListCheckpoints`
- `AddWitnessSignature`, `WitnessSignatures`
- `Stats` returning aggregate counts

**Phase 3 — Merkle Tree**
- `internal/merkle/hash.go` — domain-separated `HashLeaf` (prefix `0x00`) and `HashNode` (prefix `0x01`)
- `internal/merkle/tree.go` — `BuildRoot`, `InclusionProof`, `VerifyInclusion`, `ConsistencyProof`, `VerifyConsistency`
- RFC 6962-compatible power-of-2 subtree construction

**Phase 2 — VUR Core Types**
- `pkg/vur/receipt.go` — `Receipt` struct
- `pkg/vur/canonical.go` — alphabetically-ordered canonical serialization via shadow struct
- `pkg/vur/signing.go` — `SignReceipt` / `VerifyReceipt` (Ed25519)

**Phase 1 — Project Scaffold**
- Go module (`github.com/yasserrmd/siqlah`)
- Directory structure, `Makefile`, Apache 2.0 license, `README.md`

---

## Conventions

- **BREAKING** — changes to canonical receipt serialization, the witness signed payload, or the Merkle leaf hash are always breaking and require a version bump in the relevant spec document.
- Additions to the HTTP API that are backward-compatible (new endpoints, new optional response fields) are not breaking.
- Bug fixes that change observable behavior (e.g., signature verification outcomes) are noted explicitly.
