.PHONY: build test lint docker-build docker-up clean build-tokenizer help

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

build: $(BINARY_DIR) ## Build all Go binaries into bin/
	$(GO) build $(LDFLAGS) -o $(SIQLAH_BIN) ./cmd/siqlah
	$(GO) build -o $(WITNESS_BIN) ./cmd/witness
	$(GO) build -o $(VERIFIER_BIN) ./cmd/verifier

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

build-tokenizer: ## Build the Rust tokenizer static library
	cd $(TOKENIZER_DIR) && $(CARGO) build --release

test: ## Run the full Go test suite
	$(GO) test ./...

lint: ## Run go vet and golangci-lint (if installed)
	$(GO) vet ./...
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

docker-build: ## Build the siqlah Docker image
	$(DOCKER) build -f deployments/Dockerfile -t siqlah:latest .

docker-up: ## Start siqlah + witness containers via docker compose
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml up -d

clean: ## Remove build artifacts (bin/ and Rust target/)
	rm -rf $(BINARY_DIR)
	cd $(TOKENIZER_DIR) && $(CARGO) clean

build-legacy: ## Build with SQLite-only mode (no Tessera)
	go build -tags sqlite_legacy -o bin/ ./cmd/...

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Targets without descriptions:"
	@grep -E '^[a-zA-Z_-]+:' $(MAKEFILE_LIST) | grep -v '##' | awk -F: '{print "  " $$1}'
