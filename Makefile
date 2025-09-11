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
	go test -v ./...

.PHONY: lint
lint: ## Run linter
	golangci-lint run

.PHONY: clean
clean: ## Clean build artifacts
	rm -f monolithic-builder

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
check: fmt vet lint test ## Run all checks

.PHONY: all
all: clean mod-tidy check build docker-build ## Build everything
