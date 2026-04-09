# BeyondTrust Terraform Provider Makefile

.PHONY: help build install test test-unit test-acc testacc test-coverage test-coverage-html clean fmt lint generate docs docs-validate tf-local tf-local-shell default

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

## docs-validate: Validate documentation
docs-validate:
	@echo "Validating documentation..."
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate

## test: Run all tests (unit and acceptance)
test:
	@echo "Running all tests..."
	$(MAKE) test-unit
	$(MAKE) test-acc

## test-unit: Run unit tests only (excludes acceptance tests)
test-unit:
	@echo "Running unit tests..."
	go test -v -cover -timeout=120s -parallel=10 -coverprofile=coverage-unit.out -covermode=atomic ./internal/...

## test-acc: Run acceptance tests (requires SMOP instance)
test-acc:
	@echo "Running acceptance tests..."
	@echo "Note: Set TF_ACC=1 and required environment variables"
	TF_ACC=1 go test -v -cover -timeout=120m -parallel=4 -coverprofile=coverage-acc.out -covermode=atomic ./...

## testacc: Alias for test-acc
testacc: test-acc

## test-coverage: Generate coverage report
test-coverage: test-unit
	@echo "Generating coverage report..."
	go tool cover -func=coverage-unit.out

## test-coverage-html: Generate HTML coverage report
test-coverage-html: test-unit
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage-unit.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## clean: Remove build artifacts and test coverage files
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f coverage-unit.out coverage-acc.out coverage.html

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	gofmt -s -w -e .

## lint: Run linters
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

	@echo "Running gofumpt..."
	@out="$$(gofumpt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		exit 1; \
	fi

## tf-local: Export TF_CLI_CONFIG_FILE to use local .terraformrc (run with: eval $(make tf-local))
tf-local: .terraformrc
	@echo "export TF_CLI_CONFIG_FILE=$(PWD)/.terraformrc"

## tf-local-shell: Start a new shell with local Terraform provider config enabled
tf-local-shell: .terraformrc
	@echo "Starting shell with local Terraform provider development overrides..."
	@echo "TF_CLI_CONFIG_FILE is set to: $(PWD)/.terraformrc"
	@echo "Provider override: beyondtrust/beyondtrust -> $(PWD)"
	@echo "Run 'exit' to return to your normal shell"
	@TF_CLI_CONFIG_FILE=$(PWD)/.terraformrc $(SHELL)

# Generate .terraformrc with current working directory
.terraformrc:
	@echo "Generating .terraformrc with provider override..."
	@echo 'provider_installation {' > .terraformrc
	@echo '  dev_overrides {' >> .terraformrc
	@echo '    "registry.terraform.io/beyondtrust/beyondtrust" = "$(PWD)"' >> .terraformrc
	@echo '  }' >> .terraformrc
	@echo '' >> .terraformrc
	@echo '  # For all other providers, use the default registry' >> .terraformrc
	@echo '  direct {}' >> .terraformrc
	@echo '}' >> .terraformrc
	@echo "Generated .terraformrc"
