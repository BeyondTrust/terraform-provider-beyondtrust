# BeyondTrust Terraform Provider Makefile

.PHONY: help build install test testacc clean fmt lint generate docs default

BINARY_NAME := terraform-provider-beyondtrust
VERSION := dev
HOSTNAME := registry.terraform.io
NAMESPACE := beyondtrust
NAME := beyondtrust
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)

# Default target - align with scaffolding
default: fmt lint install generate

## help: Display this help message
help:
	@echo "BeyondTrust Terraform Provider - Make Targets"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: Build the provider binary
build:
	@echo "Building provider..."
	go build -o $(BINARY_NAME) -ldflags="-X 'main.version=$(VERSION)'"

## install: Install the provider locally for development
install: build
	@echo "Installing provider locally..."
	mkdir -p ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)
	cp $(BINARY_NAME) ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)/

## generate: Run code generation tools (docs)
generate:
	@echo "Running code generation..."
	cd tools && go generate ./...

## docs: Generate documentation (alias for generate)
docs: generate

## test: Run unit tests
test:
	@echo "Running unit tests..."
	go test -v -cover -timeout=120s -parallel=10 ./...

## testacc: Run acceptance tests (requires SMOP instance)
testacc:
	@echo "Running acceptance tests..."
	TF_ACC=1 go test -v -cover -timeout 120m ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	gofmt -s -w -e .

## lint: Run linters
lint:
	@echo "Running linters..."
	golangci-lint run
