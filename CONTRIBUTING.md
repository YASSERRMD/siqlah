# Contributing to siqlah

## Getting started

```bash
git clone https://github.com/yasserrmd/siqlah
cd siqlah
make build   # builds Go binaries and Rust tokenizer
make test    # runs all tests
```

## Development requirements

- Go 1.21+
- Rust 1.70+ (for `tokenizer-rs/`)
- `gcc` or `clang` (CGo for tokenizer FFI)
- `sqlite3` (optional; tests use in-memory SQLite)

## Repository layout

```
cmd/              - Main binaries (siqlah, witness, verifier)
internal/         - Private packages
  api/            - HTTP handlers
  checkpoint/     - Merkle checkpoint builder
  merkle/         - RFC 6962 tree implementation
  monitor/        - Discrepancy monitoring
  provider/       - Provider adapters (OpenAI, Anthropic, Generic)
  store/          - SQLite storage
  tokenizer/      - Go CGo wrapper for Rust tokenizer
pkg/vur/          - Public VUR types and signing
tokenizer-rs/     - Rust cdylib tokenizer
test/             - Integration, E2E, stress, fuzz, and bench tests
docs/             - Specifications and guides
deployments/      - Dockerfile, docker-compose, entrypoint
```

## Making changes

1. Fork the repo and create a branch: `git checkout -b feature/my-change`
2. Write code and tests
3. Run `make test` — all tests must pass
4. Run `go vet ./...` — must be clean
5. Run `go mod tidy` — `go.mod` and `go.sum` must be unmodified
6. Commit with a descriptive message and open a pull request against `main`

## Code conventions

- No unnecessary comments — code should be self-documenting
- No half-finished implementations or TODO stubs in submitted code
- Error handling at system boundaries only; trust internal guarantees
- No backwards-compatibility shims for code that has no existing callers

## Testing

- Unit tests live beside their package (`_test.go` in the same directory)
- Integration and E2E tests live in `test/`
- Use in-memory SQLite (`store.Open(":memory:")`) for fast, isolated tests
- The CGo tokenizer tests require the compiled shared library; CI builds it automatically

Run specific packages:

```bash
go test ./pkg/... ./internal/...      # all unit tests
go test ./test/... -run TestE2E       # E2E only
go test ./test/... -bench=. -benchtime=1x  # benchmarks (smoke)
```

## Cryptography

siqlah uses Ed25519 (RFC 8032) exclusively. Do not add other signature schemes without updating the receipt spec and witness protocol docs.

The canonical receipt serialization (field order, timestamp format) is a compatibility surface — any change is a breaking change requiring a version bump in the receipt spec.

## Opening a pull request

Include in the PR description:
- What problem this solves or what feature this adds
- How to test it manually
- Any spec changes (receipt, witness protocol, API)

The CI pipeline runs tests, `go vet`, `go mod tidy`, and benchmark smoke tests. All checks must pass before merge.
