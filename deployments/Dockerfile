# Stage 1: Build the Rust tokenizer CDylib
FROM rust:1.80-slim AS rust-builder
WORKDIR /build
COPY tokenizer-rs/ tokenizer-rs/
RUN cd tokenizer-rs && cargo build --release

# Stage 2: Build Go binaries with CGo linking to the Rust library
FROM golang:1.25-bookworm AS go-builder
WORKDIR /build
RUN apt-get update && apt-get install -y gcc libc6-dev && rm -rf /var/lib/apt/lists/*

# Copy the Rust shared library so CGo can link against it
COPY --from=rust-builder /build/tokenizer-rs/target/release/libsiqlah_tokenizer.a \
     /build/tokenizer-rs/target/release/libsiqlah_tokenizer.a

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
RUN go build \
    -ldflags "-X main.version=${VERSION} -X main.commitSHA=${COMMIT}" \
    -o /bin/siqlah ./cmd/siqlah && \
    go build -o /bin/witness ./cmd/witness && \
    go build -o /bin/verifier ./cmd/verifier

# Stage 3: Minimal runtime image
FROM debian:bookworm-slim AS runtime

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user for the service
RUN useradd -r -u 1001 -g root siqlah && \
    mkdir -p /data && \
    chown siqlah:root /data

COPY --from=go-builder /bin/siqlah   /usr/local/bin/siqlah
COPY --from=go-builder /bin/witness  /usr/local/bin/witness
COPY --from=go-builder /bin/verifier /usr/local/bin/verifier
COPY deployments/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER siqlah
WORKDIR /data
VOLUME ["/data"]

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/v1/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["server"]
