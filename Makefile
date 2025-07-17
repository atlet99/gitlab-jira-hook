# GitLab-Jira Hook Makefile

# Project-specific variables
PROJECT_NAME := gitlab-jira-hook
OUTPUT_DIR := build
PKG_DIR := internal

TAG_NAME ?= $(shell head -n 1 .release-version 2>/dev/null | sed 's/^/v/' || echo "v0.1.0")
VERSION ?= $(shell head -n 1 .release-version 2>/dev/null || echo "0.1.0")
BUILD_INFO ?= $(shell date +%s)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GO_VERSION := $(shell cat .go-version 2>/dev/null || echo "1.21")
GO_FILES := $(wildcard $(PKG_DIR)/**/*.go)
GOPATH ?= $(shell go env GOPATH)
ifeq ($(GOPATH),)
GOPATH = $(HOME)/go
endif
GOLANGCI_LINT = $(GOPATH)/bin/golangci-lint
STATICCHECK = $(GOPATH)/bin/staticcheck
GOIMPORTS = $(GOPATH)/bin/goimports
GOSEC = $(GOPATH)/bin/gosec
ERRCHECK = $(GOPATH)/bin/errcheck

# Security scanning constants
GOSEC_VERSION := v2.22.5
GOSEC_OUTPUT_FORMAT := sarif
GOSEC_REPORT_FILE := gosec-report.sarif
GOSEC_JSON_REPORT := gosec-report.json
GOSEC_SEVERITY := medium

# Vulnerability checking constants
GOVULNCHECK_VERSION := v1.1.4
GOVULNCHECK = $(GOPATH)/bin/govulncheck
VULNCHECK_OUTPUT_FORMAT := json
VULNCHECK_REPORT_FILE := vulncheck-report.json

# Error checking constants
ERRCHECK_VERSION := v1.9.0

# SBOM generation constants
SYFT_VERSION := 1.28.0
SYFT = $(GOPATH)/bin/syft
SYFT_OUTPUT_FORMAT := syft-json
SYFT_SBOM_FILE := sbom.syft.json
SYFT_SPDX_FILE := sbom.spdx.json
SYFT_CYCLONEDX_FILE := sbom.cyclonedx.json

# Build flags
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
BUILT_BY ?= $(shell git remote get-url origin 2>/dev/null | sed -n 's/.*[:/]\([^/]*\)\/[^/]*\.git.*/\1/p' || git config user.name 2>/dev/null | tr ' ' '_' || echo "local")

# Build flags for Go
BUILD_FLAGS=-buildvcs=false
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(DATE)"

# Matrix testing constants
MATRIX_MIN_GO_VERSION := 1.21
MATRIX_STABLE_GO_VERSION := 1.21
MATRIX_LATEST_GO_VERSION := 1.21
MATRIX_TEST_TIMEOUT := 10m
MATRIX_COVERAGE_THRESHOLD := 80

# Docker constants
DOCKER_IMAGE = atlet99/gitlab-jira-hook
DOCKER_TAG = $(VERSION)

# Ensure the output directory exists
.PHONY: ensure-output-dir
ensure-output-dir:
	@mkdir -p $(OUTPUT_DIR)

# Default target
.PHONY: default
default: fmt vet imports lint staticcheck test quicktest

# Display help information
.PHONY: help
help:
	@echo "GitLab-Jira Hook - Webhook Server"
	@echo ""
	@echo "Available targets:"
	@echo "  Building and Testing:"
	@echo "  ===================="
	@echo "  default         - Run formatting, vetting, linting, staticcheck, and tests"
	@echo "  build           - Build the application"
	@echo "  test            - Run all tests with standard coverage"
	@echo "  test-with-race  - Run all tests with race detection and coverage"
	@echo "  quicktest       - Run quick tests without additional checks"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  test-race       - Run tests with race detection"
	@echo "  test-all        - Run all tests and benchmarks"
	@echo ""
	@echo "  Development:"
	@echo "  ============"
	@echo "  run             - Build and run the application"
	@echo "  dev             - Run in development mode"
	@echo "  docker-build    - Build Docker image"
	@echo "  docker-run      - Run Docker container"
	@echo "  docker-push     - Push Docker image"
	@echo ""
	@echo "  Code Quality:"
	@echo "  ============"
	@echo "  fmt             - Check and format Go code"
	@echo "  vet             - Analyze code with go vet"
	@echo "  imports         - Format imports with goimports"
	@echo "  lint            - Run golangci-lint"
	@echo "  lint-fix        - Run linters with auto-fix"
	@echo "  staticcheck     - Run staticcheck static analyzer"
	@echo "  staticcheck-only - Run staticcheck with enhanced configuration"
	@echo "  errcheck        - Check for unchecked errors in Go code"
	@echo "  security-scan   - Run gosec security scanner (SARIF output)"
	@echo "  security-scan-json - Run gosec security scanner (JSON output)"
	@echo "  security-scan-html - Run gosec security scanner (HTML output)"
	@echo "  security-scan-ci - Run gosec security scanner for CI (no-fail mode)"
	@echo "  vuln-check      - Run govulncheck vulnerability scanner"
	@echo "  vuln-check-json - Run govulncheck vulnerability scanner (JSON output)"
	@echo "  vuln-check-ci   - Run govulncheck vulnerability scanner for CI"
	@echo "  sbom-generate   - Generate Software Bill of Materials (SBOM) with Syft"
	@echo "  sbom-syft       - Generate SBOM in Syft JSON format (alias for sbom-generate)"
	@echo "  sbom-spdx       - Generate SBOM in SPDX JSON format"
	@echo "  sbom-cyclonedx  - Generate SBOM in CycloneDX JSON format"
	@echo "  sbom-all        - Generate SBOM in all supported formats"
	@echo "  sbom-ci         - Generate SBOM for CI pipeline (quiet mode)"
	@echo "  check-all       - Run all code quality checks including error checking, security, vulnerability checks and SBOM generation"
	@echo ""
	@echo "  Dependencies:"
	@echo "  ============="
	@echo "  deps            - Install project dependencies"
	@echo "  install-deps    - Install project dependencies (alias for deps)"
	@echo "  upgrade-deps    - Upgrade all dependencies to latest versions"
	@echo "  clean-deps      - Clean up dependencies"
	@echo "  install-tools   - Install development tools"
	@echo ""
	@echo "  Version Management:"
	@echo "  =================="
	@echo "  version         - Show current version information"
	@echo "  bump-patch      - Bump patch version"
	@echo "  bump-minor      - Bump minor version"
	@echo "  bump-major      - Bump major version"
	@echo "  release         - Build release version with all optimizations"
	@echo ""
	@echo "  Documentation:"
	@echo "  =============="
	@echo "  docs            - Generate documentation"
	@echo "  docs-api        - Generate API documentation"
	@echo ""
	@echo "  CI/CD Support:"
	@echo "  =============="
	@echo "  ci-lint         - Run CI linting checks"
	@echo "  ci-test         - Run CI tests"
	@echo "  ci-build        - Run CI build"
	@echo "  ci-release      - Complete CI release pipeline"
	@echo "  matrix-test-local - Run matrix tests locally with multiple Go versions"
	@echo "  matrix-info     - Show matrix testing configuration and features"
	@echo "  test-multi-go   - Test Go version compatibility"
	@echo "  test-go-versions - Check current Go version against requirements"
	@echo ""
	@echo "  Cleanup:"
	@echo "  ========"
	@echo "  clean           - Clean build artifacts"
	@echo "  clean-coverage  - Clean coverage and benchmark files"
	@echo "  clean-all       - Clean everything including dependencies"
	@echo ""
	@echo "Examples:"
	@echo "  make build                     - Build the application"
	@echo "  make test                      - Run all tests"
	@echo "  make run                       - Build and run"
	@echo "  make check-all                 - Run all quality checks"
	@echo "  make bump-patch                - Bump patch version"

# Dependencies
.PHONY: deps install-deps upgrade-deps clean-deps install-tools
deps: install-deps

install-deps:
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed successfully"

upgrade-deps:
	@echo "Upgrading all dependencies to latest versions..."
	go get -u ./...
	go mod tidy
	@echo "Dependencies upgraded. Please test thoroughly before committing!"

clean-deps:
	@echo "Cleaning up dependencies..."
	rm -rf vendor

install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	go install github.com/kisielk/errcheck@$(ERRCHECK_VERSION)
	@echo "Installing Syft SBOM generator..."
	@if ! command -v $(SYFT) >/dev/null 2>&1; then \
		curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b $(GOPATH)/bin; \
	else \
		echo "Syft is already installed at $(SYFT)"; \
	fi
	@echo "Development tools installed successfully"

# Building
.PHONY: build release-build

build: ensure-output-dir
	@echo "Building $(PROJECT_NAME) v$(VERSION)..."
	go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(PROJECT_NAME) ./cmd/server
	@echo "Build complete: $(OUTPUT_DIR)/$(PROJECT_NAME)"

release-build: test lint staticcheck
	@echo "Building release version $(VERSION)..."
	@mkdir -p $(OUTPUT_DIR)
	go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(PROJECT_NAME) ./cmd/server
	@echo "Release build completed for $(PROJECT_NAME) v$(VERSION)"

# Testing
.PHONY: test test-with-race quicktest test-coverage test-race test-all

test:
	@echo "Running Go tests..."
	go test -v ./... -cover

test-with-race:
	@echo "Running all tests with race detection and coverage..."
	go test -v -race -cover ./...

quicktest:
	@echo "Running quick tests..."
	go test ./...

test-coverage:
	@echo "Running tests with coverage report..."
	go test -v -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

test-all: test-coverage test-race
	@echo "All tests completed"

# Development
.PHONY: run dev

run: build
	@echo "Running $(PROJECT_NAME)..."
	./$(OUTPUT_DIR)/$(PROJECT_NAME)

dev:
	@echo "Running in development mode..."
	go run ./cmd/server

# Docker
.PHONY: docker-build docker-run docker-push

docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Docker image built"

docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file config.env $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest
	@echo "Docker image pushed"

# Code quality
.PHONY: fmt vet imports lint lint-fix staticcheck errcheck security-scan vuln-check check-all

fmt:
	@echo "Checking and formatting code..."
	@go fmt ./...
	@echo "Code formatting completed"

vet:
	@echo "Running go vet..."
	go vet ./...

# Run goimports
.PHONY: imports
imports:
	@if command -v $(GOIMPORTS) >/dev/null 2>&1; then \
		echo "Running goimports..."; \
		$(GOIMPORTS) -local github.com/atlet99/gitlab-jira-hook -w $(GO_FILES); \
		echo "Imports formatting completed!"; \
	else \
		echo "goimports is not installed. Installing..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
		echo "Running goimports..."; \
		$(GOIMPORTS) -local github.com/atlet99/gitlab-jira-hook -w $(GO_FILES); \
		echo "Imports formatting completed!"; \
	fi

# Run linter
.PHONY: lint
lint:
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		echo "Running linter..."; \
		$(GOLANGCI_LINT) run --timeout=5m; \
		echo "Linter completed!"; \
	else \
		echo "golangci-lint is not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
		echo "Running linter..."; \
		$(GOLANGCI_LINT) run --timeout=5m; \
		echo "Linter completed!"; \
	fi

# Run staticcheck tool
.PHONY: staticcheck
staticcheck:
	@if command -v $(STATICCHECK) >/dev/null 2>&1; then \
		echo "Running staticcheck..."; \
		$(STATICCHECK) ./...; \
		echo "Staticcheck completed!"; \
	else \
		echo "staticcheck is not installed. Installing..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
		echo "Running staticcheck..."; \
		$(STATICCHECK) ./...; \
		echo "Staticcheck completed!"; \
	fi

# Run errcheck tool to find unchecked errors
.PHONY: errcheck errcheck-install
errcheck-install:
	@if ! command -v $(ERRCHECK) >/dev/null 2>&1; then \
		echo "errcheck is not installed. Installing errcheck $(ERRCHECK_VERSION)..."; \
		go install github.com/kisielk/errcheck@$(ERRCHECK_VERSION); \
		echo "errcheck installed successfully!"; \
	else \
		echo "errcheck is already installed"; \
	fi

errcheck: errcheck-install
	@echo "Running errcheck to find unchecked errors..."
	@if [ -f .errcheck_excludes.txt ]; then \
		$(ERRCHECK) -exclude .errcheck_excludes.txt ./...; \
	else \
		$(ERRCHECK) ./...; \
	fi
	@echo "errcheck completed!"

.PHONY: lint-fix
lint-fix:
	@echo "Running linters with auto-fix..."
	@$(GOLANGCI_LINT) run --fix
	@echo "Auto-fix completed"

# Run staticcheck with enhanced configuration
.PHONY: staticcheck-only
staticcheck-only:
	@echo "Running staticcheck with enhanced configuration..."
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		$(GOLANGCI_LINT) run --enable-only=staticcheck ./...; \
		echo "staticcheck completed!"; \
	else \
		echo "golangci-lint is not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
		echo "Running staticcheck..."; \
		$(GOLANGCI_LINT) run --enable-only=staticcheck ./...; \
		echo "staticcheck completed!"; \
	fi

# Security scanning with gosec
.PHONY: security-scan security-scan-json security-scan-html security-install-gosec

security-install-gosec:
	@if ! command -v $(GOSEC) >/dev/null 2>&1; then \
		echo "gosec is not installed. Installing gosec $(GOSEC_VERSION)..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION); \
		echo "gosec installed successfully!"; \
	else \
		echo "gosec is already installed"; \
	fi

security-scan: security-install-gosec
	@echo "Running gosec security scan..."
	@if [ -f .gosec.json ]; then \
		$(GOSEC) -quiet -conf .gosec.json -fmt $(GOSEC_OUTPUT_FORMAT) -out $(GOSEC_REPORT_FILE) -severity $(GOSEC_SEVERITY) ./...; \
	else \
		$(GOSEC) -quiet -fmt $(GOSEC_OUTPUT_FORMAT) -out $(GOSEC_REPORT_FILE) -severity $(GOSEC_SEVERITY) ./...; \
	fi
	@echo "Security scan completed. Report saved to $(GOSEC_REPORT_FILE)"
	@echo "To view issues: cat $(GOSEC_REPORT_FILE)"

security-scan-json: security-install-gosec
	@echo "Running gosec security scan with JSON output..."
	@if [ -f .gosec.json ]; then \
		$(GOSEC) -quiet -conf .gosec.json -fmt json -out $(GOSEC_JSON_REPORT) -severity $(GOSEC_SEVERITY) ./...; \
	else \
		$(GOSEC) -quiet -fmt json -out $(GOSEC_JSON_REPORT) -severity $(GOSEC_SEVERITY) ./...; \
	fi
	@echo "Security scan completed. JSON report saved to $(GOSEC_JSON_REPORT)"

security-scan-html: security-install-gosec
	@echo "Running gosec security scan with HTML output..."
	@if [ -f .gosec.json ]; then \
		$(GOSEC) -quiet -conf .gosec.json -fmt html -out gosec-report.html -severity $(GOSEC_SEVERITY) ./...; \
	else \
		$(GOSEC) -quiet -fmt html -out gosec-report.html -severity $(GOSEC_SEVERITY) ./...; \
	fi
	@echo "Security scan completed. HTML report saved to gosec-report.html"

security-scan-ci: security-install-gosec
	@echo "Running gosec security scan for CI..."
	@if [ -f .gosec.json ]; then \
		$(GOSEC) -quiet -conf .gosec.json -fmt $(GOSEC_OUTPUT_FORMAT) -out $(GOSEC_REPORT_FILE) -no-fail -quiet ./...; \
	else \
		$(GOSEC) -quiet -fmt $(GOSEC_OUTPUT_FORMAT) -out $(GOSEC_REPORT_FILE) -no-fail -quiet ./...; \
	fi
	@echo "CI security scan completed"

# Vulnerability checking with govulncheck
.PHONY: vuln-check vuln-check-json vuln-install-govulncheck vuln-check-ci

vuln-install-govulncheck:
	@if ! command -v $(GOVULNCHECK) >/dev/null 2>&1; then \
		echo "govulncheck is not installed. Installing govulncheck $(GOVULNCHECK_VERSION)..."; \
		go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
		echo "govulncheck installed successfully!"; \
	else \
		echo "govulncheck is already installed"; \
	fi

vuln-check: vuln-install-govulncheck
	@echo "Running govulncheck vulnerability scan..."
	@$(GOVULNCHECK) ./...
	@echo "Vulnerability scan completed successfully"

vuln-check-json: vuln-install-govulncheck
	@echo "Running govulncheck vulnerability scan with JSON output..."
	@$(GOVULNCHECK) -json ./... > $(VULNCHECK_REPORT_FILE)
	@echo "Vulnerability scan completed. JSON report saved to $(VULNCHECK_REPORT_FILE)"
	@echo "To view results: cat $(VULNCHECK_REPORT_FILE)"

vuln-check-ci: vuln-install-govulncheck
	@echo "Running govulncheck vulnerability scan for CI..."
	@$(GOVULNCHECK) -json ./... > $(VULNCHECK_REPORT_FILE) || echo "Vulnerabilities found, check report"
	@echo "CI vulnerability scan completed. Report saved to $(VULNCHECK_REPORT_FILE)"

# SBOM generation with Syft
.PHONY: sbom-generate sbom-syft sbom-spdx sbom-cyclonedx sbom-install-syft sbom-all sbom-ci

sbom-install-syft:
	@if ! command -v $(SYFT) >/dev/null 2>&1; then \
		echo "Syft is not installed. Installing Syft $(SYFT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b $(GOPATH)/bin; \
		echo "Syft installed successfully!"; \
	else \
		echo "Syft is already installed"; \
	fi

sbom-generate: sbom-install-syft
	@echo "Generating SBOM with Syft (JSON format)..."
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o $(SYFT_OUTPUT_FORMAT)=$(SYFT_SBOM_FILE)
	@echo "SBOM generated successfully: $(SYFT_SBOM_FILE)"
	@echo "To view SBOM: cat $(SYFT_SBOM_FILE)"

sbom-syft: sbom-generate

sbom-spdx: sbom-install-syft
	@echo "Generating SBOM with Syft (SPDX JSON format)..."
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o spdx-json=$(SYFT_SPDX_FILE)
	@echo "SPDX SBOM generated successfully: $(SYFT_SPDX_FILE)"

sbom-cyclonedx: sbom-install-syft
	@echo "Generating SBOM with Syft (CycloneDX JSON format)..."
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o cyclonedx-json=$(SYFT_CYCLONEDX_FILE)
	@echo "CycloneDX SBOM generated successfully: $(SYFT_CYCLONEDX_FILE)"

sbom-all: sbom-install-syft
	@echo "Generating SBOM in all supported formats..."
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o $(SYFT_OUTPUT_FORMAT)=$(SYFT_SBOM_FILE)
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o spdx-json=$(SYFT_SPDX_FILE)
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o cyclonedx-json=$(SYFT_CYCLONEDX_FILE)
	@echo "All SBOM formats generated successfully:"
	@echo "  - Syft JSON: $(SYFT_SBOM_FILE)"
	@echo "  - SPDX JSON: $(SYFT_SPDX_FILE)"
	@echo "  - CycloneDX JSON: $(SYFT_CYCLONEDX_FILE)"

sbom-ci: sbom-install-syft
	@echo "Generating SBOM for CI pipeline..."
	@$(SYFT) . --source-name $(PROJECT_NAME) --source-version $(VERSION) -o $(SYFT_OUTPUT_FORMAT)=$(SYFT_SBOM_FILE) --quiet
	@echo "CI SBOM generation completed. Report saved to $(SYFT_SBOM_FILE)"

check-all: fmt vet imports lint staticcheck errcheck security-scan vuln-check sbom-generate
	@echo "All code quality checks and SBOM generation completed"

# Version management
.PHONY: version bump-patch bump-minor bump-major release

version:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Go version: $(GO_VERSION)"
	@echo "Release version: $(VERSION)"
	@echo "Tag name: $(TAG_NAME)"
	@echo "Build target: $(GOOS)/$(GOARCH)"
	@echo "Commit: $(COMMIT)"
	@echo "Built by: $(BUILT_BY)"
	@echo "Build info: $(BUILD_INFO)"

bump-patch:
	@if [ ! -f .release-version ]; then echo "0.1.0" > .release-version; fi
	@current=$$(cat .release-version); \
	new=$$(echo $$current | awk -F. '{$$3=$$3+1; print $$1"."$$2"."$$3}'); \
	echo $$new > .release-version; \
	echo "Version bumped from $$current to $$new"

bump-minor:
	@if [ ! -f .release-version ]; then echo "0.1.0" > .release-version; fi
	@current=$$(cat .release-version); \
	new=$$(echo $$current | awk -F. '{$$2=$$2+1; $$3=0; print $$1"."$$2"."$$3}'); \
	echo $$new > .release-version; \
	echo "Version bumped from $$current to $$new"

bump-major:
	@if [ ! -f .release-version ]; then echo "0.1.0" > .release-version; fi
	@current=$$(cat .release-version); \
	new=$$(echo $$current | awk -F. '{$$1=$$1+1; $$2=0; $$3=0; print $$1"."$$2"."$$3}'); \
	echo $$new > .release-version; \
	echo "Version bumped from $$current to $$new"

release: release-build
	@echo "Release build completed for $(PROJECT_NAME) v$(VERSION)"

# Documentation
.PHONY: docs docs-api

docs:
	@echo "Generating documentation..."
	@echo "Documentation is available in README.md"

docs-api:
	@echo "Generating API documentation..."
	@echo "API documentation is available in the source code comments"
	@echo "Run 'go doc ./internal/...' for detailed API documentation"

# CI/CD Support
.PHONY: ci-lint ci-test ci-build ci-release matrix-test-local matrix-info test-multi-go test-go-versions test-integration test-data-check

ci-lint:
	@echo "Running CI linting checks..."
	@$(GOLANGCI_LINT) run --timeout=5m --out-format=line-number

ci-test:
	@echo "Running CI tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

ci-build:
	@echo "Running CI build..."
	@go build ./cmd/server

ci-release: ci-lint ci-test ci-build
	@echo "CI release pipeline completed"

test-integration:
	@echo "Running integration tests..."
	@go test -v -tags=integration ./...

test-data-check:
	@echo "Running data validation tests..."
	@go test -v ./internal/gitlab/ -run TestDataValidation

matrix-test-local:
	@echo "Running matrix tests locally..."
	@echo "Testing with Go $(MATRIX_STABLE_GO_VERSION)..."
	@go test -v -race -cover ./...

matrix-info:
	@echo "Matrix testing configuration:"
	@echo "  Minimum Go version: $(MATRIX_MIN_GO_VERSION)"
	@echo "  Stable Go version: $(MATRIX_STABLE_GO_VERSION)"
	@echo "  Latest Go version: $(MATRIX_LATEST_GO_VERSION)"
	@echo "  Test timeout: $(MATRIX_TEST_TIMEOUT)"
	@echo "  Coverage threshold: $(MATRIX_COVERAGE_THRESHOLD)%"

test-multi-go:
	@echo "Testing Go version compatibility..."
	@go version
	@go test -v ./...

test-go-versions:
	@echo "Checking current Go version against requirements..."
	@current_version=$$(go version | cut -d' ' -f3 | sed 's/go//'); \
	echo "Current Go version: $$current_version"; \
	echo "Required Go version: $(GO_VERSION)"; \
	if [ "$$current_version" = "$(GO_VERSION)" ]; then \
		echo "✓ Go version matches requirements"; \
	else \
		echo "⚠ Go version mismatch. Expected: $(GO_VERSION), Got: $$current_version"; \
	fi

# Cleanup
.PHONY: clean clean-coverage clean-all

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(OUTPUT_DIR)
	rm -f coverage.out coverage.html
	rm -f $(GOSEC_REPORT_FILE) $(GOSEC_JSON_REPORT) gosec-report.html
	rm -f $(VULNCHECK_REPORT_FILE)
	rm -f $(SYFT_SBOM_FILE) $(SYFT_SPDX_FILE) $(SYFT_CYCLONEDX_FILE)
	go clean -cache
	@echo "Cleanup completed"

clean-coverage:
	@echo "Cleaning coverage files..."
	rm -f coverage.out coverage.html
	@echo "Coverage files cleaned"

clean-all: clean clean-deps
	@echo "Deep cleaning everything including dependencies..."
	go clean -modcache
	@echo "Deep cleanup completed" 