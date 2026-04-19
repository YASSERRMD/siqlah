# Witness Interoperability Guide

This document explains how to connect siqlah to external witnesses in the broader transparency ecosystem — including [Sigsum](https://sigsum.org) witnesses, the [Armored Witness](https://github.com/transparency-dev/armored-witness) hardware device, and any other server implementing the [C2SP tlog-witness](https://github.com/C2SP/C2SP/blob/main/tlog-witness.md) protocol.

---

## How the Witness Protocol Works

siqlah publishes checkpoints in the [C2SP signed note](https://github.com/C2SP/C2SP/blob/main/signed-note.md) format:

```
<origin>
<tree_size>:<base64-root-hash>

— <operator-name> <base64-operator-sig>
```

External witnesses verify the operator signature, check that the tree size has only grown (never decreased), and append their own cosignature. The result is a **cosigned note** — the same bytes with one or more additional signature lines.

---

## The WitnessFeeder

`internal/witness.WitnessFeeder` implements the push side of the protocol. It submits siqlah checkpoints to external witnesses using the [transparency-dev/witness](https://github.com/transparency-dev/witness) client library.

### Usage (Go API)

```go
import (
    "context"
    "github.com/yasserrmd/siqlah/internal/witness"
)

feeder := witness.NewWitnessFeeder([]witness.ExternalWitness{
    {
        Name:     "my-witness",
        URL:      "https://witness.example.com",
        Verifier: verifier, // note.Verifier for the witness public key
    },
})

// rawCP is the C2SP signed note bytes from GET /v1/witness/checkpoint
results := feeder.Feed(ctx, rawCP, consistencyProof)
for _, r := range results {
    if r.Err != nil {
        log.Printf("witness %s error: %v", r.WitnessName, r.Err)
        continue
    }
    // r.Cosigned contains the cosigned note — submit back to siqlah via POST /v1/witness/cosign
}
```

---

## Connecting to Sigsum Witnesses

[Sigsum](https://sigsum.org) runs public witnesses compatible with the C2SP tlog-witness protocol.

### Step 1: Obtain the witness verifier key

Sigsum witnesses publish their key IDs. Example:

```
sigsum.org/v1/witness+4d96ff4c+AcEELJPTFuEFzQ42P4hVWqFrFP2bNYiR8W9I2hE4T1M7
```

### Step 2: Configure the feeder

```go
vkey := "sigsum.org/v1/witness+4d96ff4c+AcEELJPTFuEFzQ42P4hVWqFrFP2bNYiR8W9I2hE4T1M7"
verifier, err := note.NewVerifier(vkey)
if err != nil {
    log.Fatal(err)
}

feeder := witness.NewWitnessFeeder([]witness.ExternalWitness{
    {
        Name:     "sigsum-witness",
        URL:      "https://witness.sigsum.org",
        Verifier: verifier,
    },
})
```

### Step 3: Feed periodically

After each new checkpoint is built, call `feeder.Feed(ctx, rawCP, proof)`. For a fresh witness relationship (oldSize == 0), pass an empty proof:

```go
feeder.Feed(ctx, rawCP, nil)
```

For subsequent feeds, you need the Merkle consistency proof from the last acknowledged tree size to the current one:

```go
// from internal/merkle
proof, err := merkle.ConsistencyProof(leaves, lastSize, currentSize)
feeder.Feed(ctx, rawCP, proof)
```

---

## Connecting to Armored Witness

[Armored Witness](https://github.com/transparency-dev/armored-witness) is an open-source hardware witness device. It exposes the same HTTP API.

```go
feeder := witness.NewWitnessFeeder([]witness.ExternalWitness{
    {
        Name:     "armored-witness-1",
        URL:      "http://192.168.1.10",  // device IP
        Verifier: armoredWitnessVerifier, // from device provisioning
    },
})
```

See the [Armored Witness documentation](https://github.com/transparency-dev/armored-witness) for provisioning and key extraction.

---

## Verifying Cosignatures with Standard Tools

Cosigned checkpoints from siqlah can be verified with any C2SP-compliant tool:

```bash
# Fetch the cosigned checkpoint
curl http://localhost:8080/v1/witness/cosigned-checkpoint

# Verify with the Go note tool
go run golang.org/x/mod/cmd/modfmt@latest -v
# or use: https://pkg.go.dev/golang.org/x/mod/sumdb/note
```

### Verify with `note` package directly

```go
import "golang.org/x/mod/sumdb/note"

rawCosigned := // bytes from GET /v1/witness/cosigned-checkpoint

msg, err := note.Open(rawCosigned, note.VerifierList(
    operatorVerifier, // operator's note.Verifier
    witnessVerifier,  // at least one witness verifier
))
if err != nil {
    log.Fatalf("verification failed: %v", err)
}
fmt.Printf("checkpoint verified: %s", msg.Text)
fmt.Printf("sigs: %d\n", len(msg.Sigs))
```

---

## Running a Local Witness for Testing

You can run a local witness using the `transparency-dev/witness` omniwitness binary for integration testing:

```bash
go install github.com/transparency-dev/witness/cmd/omniwitness@latest

# Generate a witness key
go run github.com/transparency-dev/witness/cmd/generate_keys@latest \
  --key_name my-local-witness \
  --out witness.key

# Start the witness (listens on :2024 by default)
omniwitness --key_path witness.key
```

The local witness accepts `POST /add-checkpoint` requests and returns cosigned notes.

---

## The tlog-witness HTTP Protocol

For reference, the protocol used by the feeder:

**Request**
```
POST /add-checkpoint
Content-Type: text/plain

old <old_tree_size>
<base64-proof-hash-1>
<base64-proof-hash-2>
...

<raw signed note>
```

**Response** (200 OK)
```
<cosigned note bytes>
```

**Response** (409 Conflict, when the witness has a newer checkpoint)
```
Content-Type: text/x.tlog.size

<current_tree_size>
```

The feeder handles the 409 case automatically — it updates its cached old size and the next `Feed()` call will retry with the correct proof.

---

## Security Considerations

- The feeder does **not** verify the witness's cosignature before storing it. Callers should verify the returned `Cosigned` bytes using `note.Open` before submitting to siqlah's `POST /v1/witness/cosign`.
- Each external witness you trust must have its verifier key obtained out-of-band (not from the witness itself).
- The feeder uses a 30-second HTTP timeout per witness. Long timeouts in a `Feed()` call will delay the entire batch — consider calling each witness concurrently for high-throughput scenarios.

---

## Related Documents

- [`docs/witness-protocol.md`](witness-protocol.md) — siqlah's internal witness protocol
- [`docs/interop.md`](interop.md) — C2SP, Sigstore, Rekor, and x402 interoperability reference
- [C2SP tlog-witness spec](https://github.com/C2SP/C2SP/blob/main/tlog-witness.md)
- [transparency-dev/witness library](https://github.com/transparency-dev/witness)
