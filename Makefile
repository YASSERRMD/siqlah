.PHONY: build test lint docker-build docker-up clean build-tokenizer

BINARY_DIR := bin
SIQLAH_BIN := $(BINARY_DIR)/siqlah
WITNESS_BIN := $(BINARY_DIR)/witness
VERIFIER_BIN := $(BINARY_DIR)/verifier
TOKENIZER_DIR := tokenizer-rs
TOKENIZER_LIB := $(TOKENIZER_DIR)/target/release/libsiqlah_tokenizer.a

GO := go
CARGO := cargo
DOCKER := docker
DOCKER_COMPOSE := docker compose

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commitSHA=$(COMMIT)"

build: $(BINARY_DIR)
	$(GO) build $(LDFLAGS) -o $(SIQLAH_BIN) ./cmd/siqlah
	$(GO) build -o $(WITNESS_BIN) ./cmd/witness
	$(GO) build -o $(VERIFIER_BIN) ./cmd/verifier

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

build-tokenizer:
	cd $(TOKENIZER_DIR) && $(CARGO) build --release

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

docker-build:
	$(DOCKER) build -f deployments/Dockerfile -t siqlah:latest .

docker-up:
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml up -d

clean:
	rm -rf $(BINARY_DIR)
	cd $(TOKENIZER_DIR) && $(CARGO) clean
