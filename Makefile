# Makefile for monolithic-builder

# Variables
REGISTRY ?= quay.io/yftacherzog-konflux/monolithic-builder
IMAGE ?= $(REGISTRY)
VERSION ?= latest

# Go variables
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target] ...'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the unified monolithic-builder binary
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -a -installsuffix cgo -o monolithic-builder ./cmd/monolithic-builder

.PHONY: test
test: ## Run tests
	go test -v -race ./...

.PHONY: test-ginkgo
test-ginkgo: ## Run tests with Ginkgo
	ginkgo run -r --randomize-all --randomize-suites --fail-on-pending --race --trace --json-report=test-results.json

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-all
test-all: test test-ginkgo test-coverage ## Run all test suites

.PHONY: lint
lint: ## Run linter
	golangci-lint run --timeout=5m

.PHONY: clean
clean: ## Clean build artifacts
	rm -f monolithic-builder *.out *.html test-results.json

.PHONY: docker-build
docker-build: ## Build the unified container image
	docker build -t $(IMAGE):$(VERSION) .

.PHONY: docker-push
docker-push: ## Push the unified container image
	docker push $(IMAGE):$(VERSION)

.PHONY: mod-tidy
mod-tidy: ## Tidy go modules
	go mod tidy

.PHONY: mod-verify
mod-verify: ## Verify go modules
	go mod verify

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: check
check: fmt vet lint test-all ## Run all checks

.PHONY: all
all: clean mod-tidy check build docker-build ## Build everything
