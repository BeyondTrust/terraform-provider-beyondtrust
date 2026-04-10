# BeyondTrust Terraform Provider Makefile

# ==========================================
# Variables and Configuration
# ==========================================

.PHONY: help build install test test-unit test-acc testacc test-coverage test-coverage-html clean fmt lint generate docs docs-validate tf-local tf-local-shell default
.PHONY: pre-commit pre-commit-quick ci-local check-tools install-tools gofumpt-fix tf-fmt-check tf-fmt-fix spell-check go-mod-tidy check-uncommitted install-git-hooks

BINARY_NAME := terraform-provider-beyondtrust
VERSION := dev
HOSTNAME := registry.terraform.io
NAMESPACE := beyondtrust
NAME := beyondtrust
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)

# ==========================================
# Default Target
# ==========================================

# Default target - run comprehensive pre-commit checks
default: pre-commit

# ==========================================
# Help System
# ==========================================

## help: Display this help message
help:
	@echo "BeyondTrust Terraform Provider - Make Targets"
	@echo ""
	@echo "Pre-Commit Targets (Run before committing):"
	@echo "  pre-commit        - Full pre-commit checks (~9s)"
	@echo "  pre-commit-quick  - Fast checks for iteration (~5s)"
	@echo "  ci-local          - Full CI simulation (~13s)"
	@echo ""
	@echo "Build and Installation:"
	@echo "  build             - Build provider binary"
	@echo "  install           - Install to local terraform plugins"
	@echo "  tf-local          - Export TF_CLI_CONFIG_FILE for local dev"
	@echo "  tf-local-shell    - Start shell with local provider"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt               - Format Go code with gofmt"
	@echo "  gofumpt-fix       - Apply gofumpt formatting"
	@echo "  lint              - Run golangci-lint and gofumpt check"
	@echo "  tf-fmt-check      - Check Terraform formatting in examples/"
	@echo "  tf-fmt-fix        - Fix Terraform formatting in examples/"
	@echo "  spell-check       - Check spelling in documentation files"
	@echo ""
	@echo "Testing:"
	@echo "  test              - Run all tests (unit + acceptance)"
	@echo "  test-unit         - Run unit tests only"
	@echo "  test-acc          - Run acceptance tests (requires SMOP instance)"
	@echo "  test-coverage     - Generate coverage report"
	@echo "  test-coverage-html - Generate HTML coverage report"
	@echo ""
	@echo "Documentation:"
	@echo "  generate          - Generate provider documentation"
	@echo "  docs              - Alias for generate"
	@echo "  docs-validate     - Validate generated documentation"
	@echo ""
	@echo "Utility Targets:"
	@echo "  check-tools       - Verify required tool versions"
	@echo "  install-tools     - Install required development tools"
	@echo "  install-git-hooks - Install git pre-commit hooks (optional)"
	@echo "  go-mod-tidy       - Verify go.mod and go.sum are clean"
	@echo "  check-uncommitted - Verify no uncommitted changes"
	@echo "  clean             - Remove build artifacts"
	@echo ""
	@echo "For more details, see DEVELOPMENT.md"

# ==========================================
# Build and Installation
# ==========================================

## build: Build the provider binary
build:
	@echo "Building provider..."
	@go build -o $(BINARY_NAME) -ldflags="-X 'main.version=$(VERSION)'"
	@echo "✅ Build complete"

## install: Install the provider locally for development
install: build
	@echo "Installing provider locally..."
	@mkdir -p ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)
	@cp $(BINARY_NAME) ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)/
	@echo "✅ Installation complete"

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

# ==========================================
# Code Quality
# ==========================================

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w -e .

## gofumpt-fix: Apply gofumpt formatting
gofumpt-fix:
	@echo "Applying gofumpt formatting..."
	@gofumpt -w .

## lint: Run linters
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run

	@echo "Running gofumpt..."
	@out="$$(gofumpt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		exit 1; \
	fi

## tf-fmt-check: Check Terraform formatting in examples/
tf-fmt-check:
	@echo "Checking Terraform formatting..."
	@terraform fmt -check -recursive examples/

## tf-fmt-fix: Fix Terraform formatting in examples/
tf-fmt-fix:
	@echo "Fixing Terraform formatting..."
	@terraform fmt -recursive examples/
	@echo "✅ Terraform formatting complete"

## spell-check: Check spelling in documentation files
spell-check:
	@echo "Checking spelling..."
	@npx --yes --quiet cspell --no-progress "**/*.md" || (echo "❌ Spelling errors found. Add unknown technical terms to .cspell.json" && exit 1)

# ==========================================
# Testing
# ==========================================

## test: Run all tests (unit and acceptance)
test:
	@echo "Running all tests..."
	$(MAKE) test-unit
	$(MAKE) test-acc

## test-unit: Run unit tests only (excludes acceptance tests)
test-unit:
	@echo "Running unit tests..."
	@go test -v -cover -timeout=120s -parallel=10 -coverprofile=coverage-unit.out -covermode=atomic ./internal/...

## test-acc: Run acceptance tests (requires SMOP staging instance)
test-acc:
	@echo "Running acceptance tests..."
	@echo "Note: Requires test.config.json or environment variables (BEYONDTRUST_API_URL, BEYONDTRUST_SITE_ID, BEYONDTRUST_ACCESS_TOKEN)"
	@echo "See test.config.json.example for configuration format"
	@TF_ACC=1 go test -v -tags=acceptance -timeout=120m -parallel=4 -coverprofile=coverage-acc.out -covermode=atomic ./...

## testacc: Alias for test-acc
testacc: test-acc

## test-coverage: Generate coverage report
test-coverage: test-unit
	@echo "Generating coverage report..."
	@go tool cover -func=coverage-unit.out

## test-coverage-html: Generate HTML coverage report
test-coverage-html: test-unit
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage-unit.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# ==========================================
# Documentation
# ==========================================

## generate: Run code generation tools (docs)
generate:
	@echo "Running code generation..."
	@cd tools && go generate ./...

## docs: Generate documentation (alias for generate)
docs: generate

## docs-validate: Validate documentation
docs-validate:
	@echo "Validating documentation..."
	@tfplugindocs validate

# ==========================================
# Pre-Commit Targets
# ==========================================

## pre-commit: Run all pre-commit checks (recommended before pushing)
pre-commit: check-tools
	@echo "=================================================="
	@echo "Running pre-commit checks..."
	@echo "=================================================="
	@echo ""

	@echo "Step 1/10: Formatting Go code..."
	@$(MAKE) --no-print-directory fmt || (echo "❌ Format failed" && exit 1)
	@echo "✅ Format complete"
	@echo ""

	@echo "Step 2/10: Applying gofumpt..."
	@$(MAKE) --no-print-directory gofumpt-fix || (echo "❌ Gofumpt failed" && exit 1)
	@echo "✅ Gofumpt complete"
	@echo ""

	@echo "Step 3/10: Running linters..."
	@$(MAKE) --no-print-directory lint || (echo "❌ Lint failed" && exit 1)
	@echo "✅ Lint complete"
	@echo ""

	@echo "Step 4/10: Running unit tests..."
	@$(MAKE) --no-print-directory test-unit || (echo "❌ Tests failed" && exit 1)
	@echo "✅ Tests complete"
	@echo ""

	@echo "Step 5/10: Building provider..."
	@$(MAKE) --no-print-directory build || (echo "❌ Build failed" && exit 1)
	@echo ""

	@echo "Step 6/10: Generating documentation..."
	@$(MAKE) --no-print-directory generate || (echo "❌ Generate failed" && exit 1)
	@echo "✅ Generate complete"
	@echo ""

	@echo "Step 7/10: Validating documentation..."
	@$(MAKE) --no-print-directory docs-validate || (echo "❌ Docs validation failed" && exit 1)
	@echo "✅ Docs validation complete"
	@echo ""

	@echo "Step 8/11: Checking Terraform formatting..."
	@$(MAKE) --no-print-directory tf-fmt-check || (echo "❌ Terraform format check failed. Run 'make tf-fmt-fix' to fix." && exit 1)
	@echo "✅ Terraform format check complete"
	@echo ""

	@echo "Step 9/11: Checking spelling..."
	@$(MAKE) --no-print-directory spell-check || (echo "❌ Spelling check failed. Add technical terms to .cspell.json" && exit 1)
	@echo "✅ Spelling check complete"
	@echo ""

	@echo "Step 10/11: Tidying go.mod..."
	@$(MAKE) --no-print-directory go-mod-tidy || (echo "❌ go mod tidy failed" && exit 1)
	@echo "✅ go.mod tidy complete"
	@echo ""

	@echo "Step 11/11: Final verification..."
	@echo "✅ Final checks complete"
	@echo ""

	@echo "=================================================="
	@echo "✅ All pre-commit checks passed!"
	@echo "=================================================="

## pre-commit-quick: Fast checks for rapid iteration
pre-commit-quick:
	@echo "=================================================="
	@echo "Running quick pre-commit checks..."
	@echo "=================================================="
	@echo ""

	@echo "Step 1/4: Formatting code..."
	@$(MAKE) --no-print-directory fmt gofumpt-fix || (echo "❌ Format failed" && exit 1)
	@echo "✅ Format complete"
	@echo ""

	@echo "Step 2/4: Running linters..."
	@$(MAKE) --no-print-directory lint || (echo "❌ Lint failed" && exit 1)
	@echo "✅ Lint complete"
	@echo ""

	@echo "Step 3/4: Running unit tests..."
	@$(MAKE) --no-print-directory test-unit || (echo "❌ Tests failed" && exit 1)
	@echo "✅ Tests complete"
	@echo ""

	@echo "Step 4/5: Checking Terraform formatting..."
	@$(MAKE) --no-print-directory tf-fmt-check || (echo "❌ Terraform format check failed. Run 'make tf-fmt-fix' to fix." && exit 1)
	@echo "✅ Terraform format check complete"
	@echo ""

	@echo "Step 5/5: Checking spelling..."
	@$(MAKE) --no-print-directory spell-check || (echo "❌ Spelling check failed. Add technical terms to .cspell.json" && exit 1)
	@echo "✅ Spelling check complete"
	@echo ""

	@echo "=================================================="
	@echo "✅ Quick checks passed!"
	@echo "=================================================="

## ci-local: Full CI simulation (run before opening PR)
ci-local: clean
	@echo "=================================================="
	@echo "Running full CI simulation..."
	@echo "=================================================="
	@echo ""

	@$(MAKE) --no-print-directory pre-commit || exit 1

	@echo ""
	@echo "Checking for uncommitted changes..."
	@$(MAKE) --no-print-directory check-uncommitted || (echo "❌ Uncommitted changes detected after generation" && exit 1)
	@echo "✅ No uncommitted changes"
	@echo ""

	@echo "=================================================="
	@echo "✅ CI simulation passed!"
	@echo "Ready to open a PR."
	@echo "=================================================="

# ==========================================
# Utility Targets
# ==========================================

## check-tools: Verify required tools are installed with correct versions
check-tools:
	@echo "Checking required tools..."
	@which go >/dev/null 2>&1 || (echo "❌ go not found. Install from https://go.dev" && exit 1)
	@which golangci-lint >/dev/null 2>&1 || (echo "❌ golangci-lint not found. Run: make install-tools" && exit 1)
	@which gofumpt >/dev/null 2>&1 || (echo "❌ gofumpt not found. Run: make install-tools" && exit 1)
	@which terraform >/dev/null 2>&1 || (echo "❌ terraform not found. Install from https://terraform.io" && exit 1)
	@which tfplugindocs >/dev/null 2>&1 || (echo "❌ tfplugindocs not found. Run: make install-tools" && exit 1)
	@golangci-lint --version 2>&1 | grep -q "2.11.4" || echo "⚠️  Warning: golangci-lint version mismatch (expected v2.11.4)"
	@gofumpt --version 2>&1 | grep -q "v0.8.0" || echo "⚠️  Warning: gofumpt version mismatch (expected v0.8.0)"
	@echo "✅ All tools available"

## install-tools: Install required development tools
install-tools:
	@echo "Installing required tools..."
	@echo "Installing golangci-lint v2.11.4..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
	@echo "Installing gofumpt v0.8.0..."
	@go install mvdan.cc/gofumpt@v0.8.0
	@echo "Installing tfplugindocs..."
	@go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	@echo "✅ All tools installed"
	@echo ""
	@echo "Note: Ensure $(shell go env GOPATH)/bin is in your PATH"

## go-mod-tidy: Verify go.mod and go.sum are clean
go-mod-tidy:
	@echo "Tidying go.mod..."
	@go mod tidy
	@git diff --exit-code go.mod go.sum || (echo "❌ go.mod or go.sum has uncommitted changes" && exit 1)

## check-uncommitted: Verify no uncommitted changes
check-uncommitted:
	@git diff --exit-code || (echo "❌ Uncommitted changes detected" && exit 1)
	@git diff --cached --exit-code || (echo "❌ Staged changes detected" && exit 1)

## install-git-hooks: Install git pre-commit hooks (optional)
install-git-hooks:
	@echo "Installing git pre-commit hook..."
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make pre-commit-quick' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✅ Git pre-commit hook installed"
	@echo "The hook will run 'make pre-commit-quick' on every commit"
	@echo "To skip the hook temporarily, use: git commit --no-verify"

# ==========================================
# Cleanup
# ==========================================

## clean: Remove build artifacts and test coverage files
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage-unit.out coverage-acc.out coverage.html
	@echo "✅ Clean complete"
