.PHONY: build test clean validators testnet help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod

# Binary names
NODE_BINARY=bin/node
WALLET_BINARY=bin/wallet

# Build directories
BUILD_DIR=bin
DATA_DIR=data

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build node and wallet binaries
	@echo "Building binaries..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(NODE_BINARY) cmd/node/main.go
	$(GOBUILD) -o $(WALLET_BINARY) cmd/wallet/main.go
	@echo "✅ Build complete: $(NODE_BINARY), $(WALLET_BINARY)"

test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v ./crypto
	$(GOTEST) -v ./consensus
	$(GOTEST) -v ./ledger
	$(GOTEST) -v ./storage
	@echo "✅ All tests passed"

validators: build ## Generate validator keys
	@echo "Generating validator keys..."
	@./scripts/generate_validators.sh
	@echo "✅ Validators generated"

testnet: build validators ## Start local 3-node testnet
	@echo "Starting testnet..."
	@./scripts/run_testnet.sh

clean: ## Clean build artifacts and data
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DATA_DIR)
	@rm -f validator*.json
	@rm -f wallet.json
	@rm -f tx_*.json
	@rm -f staking_tx.json
	$(GOCLEAN)
	@echo "✅ Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ Dependencies ready"

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing binaries..."
	@cp $(NODE_BINARY) $(GOPATH)/bin/
	@cp $(WALLET_BINARY) $(GOPATH)/bin/
	@echo "✅ Installed to $(GOPATH)/bin"

dev: clean build validators ## Full development setup
	@echo "Development environment ready!"
	@echo "Run 'make testnet' to start the network"

.DEFAULT_GOAL := help