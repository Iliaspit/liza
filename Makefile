.PHONY: build test test-e2e clean install lint check-testhelpers check-embedded release package build-all tidy run coverage capsule-image capsule-image-smoke help

# Binary name
BINARY_NAME=liza
CAPSULE_IMAGE?=liza-capsule:latest

# Build variables
VERSION?=0.2.0
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X 'github.com/liza-mas/liza/internal/embedded.Version=$(VERSION)' \
	-X 'github.com/liza-mas/liza/internal/embedded.GitCommit=$(GIT_COMMIT)' \
	-X 'github.com/liza-mas/liza/internal/embedded.BuildDate=$(BUILD_DATE)' \
	-X 'main.Version=$(VERSION)' -X 'main.GitCommit=$(GIT_COMMIT)' -X 'main.BuildDate=$(BUILD_DATE)'"

# Sync embedded files from project root
.PHONY: sync-embedded
sync-embedded:
	@echo "Syncing files to internal/embedded/..."
	@rm -rf internal/embedded/contracts internal/embedded/skills internal/embedded/support-docs internal/embedded/docs internal/embedded/specs
	@mkdir -p internal/embedded/contracts internal/embedded/skills internal/embedded/support-docs
	@cp contracts/*.md internal/embedded/contracts/
	@cp -r skills/* internal/embedded/skills/
	@cp support-docs/*.md internal/embedded/support-docs/
	@find internal/embedded/skills/ -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
	@echo "Files synced successfully"

# Build the binaries
build: sync-embedded
	@echo "Building $(BINARY_NAME) (version=$(VERSION), commit=$(GIT_COMMIT), date=$(BUILD_DATE))"
	@go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/liza

# Run tests
# IMPORTANT: Always use `make test`, not bare `go test ./...`.
# The sync-embedded step copies contracts/ and skills/ into internal/embedded/ for go:embed.
# claude-settings.json and hooks/ are mastered directly in internal/embedded/.
test: sync-embedded check-testhelpers
	go test -race -coverprofile=coverage.out ./...

# Run e2e tests (full sprint sequence with mock CLI — ~40s)
test-e2e: sync-embedded check-testhelpers
	go test -race -tags e2e -run TestFullSprintSequence ./internal/integration/ -count=1

# Run tests with coverage report
coverage: test
	go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out
	rm -rf dist
	rm -rf internal/embedded/contracts internal/embedded/skills internal/embedded/support-docs internal/embedded/docs internal/embedded/specs
	go clean

# Install the binaries
# Prefer INSTALL_DIR env var, then ~/.local/bin (same as install.sh)
INSTALL_DIR ?= $(HOME)/.local/bin
SUDO := $(shell test -w $(INSTALL_DIR) && echo "" || echo "sudo")
install: build
	@mkdir -p $(INSTALL_DIR)
	$(SUDO) install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@if [ "$(INSTALL_DIR)" != "/usr/local/bin" ] && [ -f /usr/local/bin/liza ]; then \
		echo "Warning: old liza binary found in /usr/local/bin — run 'sudo rm /usr/local/bin/liza' to avoid shadowing"; \
	fi

# Check that testhelpers package is not imported in production code
# This prevents test utilities from leaking into production binaries and
# ensures clear separation between test and production code. Test helpers
# should only be used in *_test.go files.
check-testhelpers:
	@echo "Checking for testhelpers in production code..."
	@if find . -name "*.go" ! -name "*_test.go" -type f -exec grep -l "internal/testhelpers" {} \; | grep -q .; then \
		echo "ERROR: testhelpers package imported in production code:"; \
		find . -name "*.go" ! -name "*_test.go" -type f -exec grep -l "internal/testhelpers" {} \;; \
		echo ""; \
		echo "The testhelpers package should only be imported in test files (*_test.go)."; \
		echo "This ensures test utilities don't leak into production binaries."; \
		exit 1; \
	fi
	@echo "✓ No testhelpers in production code"

# Check that embedded copies match repo master files
check-embedded:
	@echo "Checking embedded artifact consistency..."
	@go test ./internal/embedded/ -run TestArtifactConsistency -count=1
	@echo "✓ Embedded artifacts are consistent with masters"

# Run linters
lint: sync-embedded check-testhelpers check-embedded
	go fmt ./...
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy

# Run the binary
run: build
	./$(BINARY_NAME)

# Build the local capsule image used by Docker capsules and Daytona snapshot sources.
capsule-image:
	docker build -f packaging/capsule/Dockerfile -t $(CAPSULE_IMAGE) .

capsule-image-smoke: capsule-image
	docker run --rm $(CAPSULE_IMAGE) sh -lc 'liza version && opencode --version && codex --version && claude --version && go version && stacklit --version && scip-search --version && scip-go --version && scip-typescript --version && scip-python --version && semble --help && ast-grep --version && rg --version && fd --version && jq --version && yq --version && gh --version'

# Build for multiple platforms
build-all: sync-embedded
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/liza
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 ./cmd/liza
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 ./cmd/liza

# Create release artifacts
release: clean lint test
	@echo "Building release artifacts..."
	@mkdir -p dist
	@# Build liza for all platforms
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/liza
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/liza
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/liza
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/liza
	@# Create checksums
	@cd dist && sha256sum * > checksums.txt
	@echo "✓ Release artifacts created in dist/"
	@echo ""
	@echo "Artifacts:"
	@ls -lh dist/
	@echo ""
	@echo "Checksums:"
	@cat dist/checksums.txt

# Create distribution packages (tarballs)
package: release
	@echo "Creating distribution packages..."
	@cd dist && \
		tar -czf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 && \
		tar -czf $(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64 && \
		tar -czf $(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 && \
		tar -czf $(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@echo "✓ Distribution packages created"
	@ls -lh dist/*.tar.gz

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build liza binary"
	@echo "  test               - Run tests (includes testhelpers check)"
	@echo "  test-e2e           - Run e2e full sprint test (~40s, requires -tags e2e)"
	@echo "  coverage           - Run tests with coverage report"
	@echo "  clean              - Clean build artifacts"
	@echo "  install            - Install liza binary"
	@echo "  lint               - Run linters (includes testhelpers check)"
	@echo "  check-testhelpers  - Verify testhelpers not in production code"
	@echo "  check-embedded     - Verify embedded copies match repo masters"
	@echo "  tidy               - Tidy dependencies"
	@echo "  run                - Build and run the liza binary"
	@echo "  capsule-image      - Build local capsule image (CAPSULE_IMAGE=...)"
	@echo "  capsule-image-smoke - Build and verify capsule image tool binaries"
	@echo "  build-all          - Build liza for multiple platforms"
	@echo "  release            - Create release artifacts (run tests, build all platforms, create checksums)"
	@echo "  package            - Create distribution packages (tarballs)"
