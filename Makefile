.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

# Variables
GO_VERSION := $(shell go version | cut -d' ' -f3)
DOCKER_COMPOSE := docker compose
HELM := helm
KUBECTL := kubectl
DATE := $(shell date +%Y-%m-%d)
VERSION ?= "test"


##@ Build
.PHONY: build run
build: check-tools ## Build the Go application.
	@echo "Building GSLB - Operator binary..."
	@echo "Version: $(VERSION)"
	@echo "Date: $(DATE)"
	@go build -ldflags "-s -w -X main.version=$(VERSION)  -X main.buildDate=$(DATE)" -o ./bin/ ./cmd/main.go

run:
	@echo "Running GSLB - Operator"
	@go run -ldflags "-X main.version=0.0.0-test -X main.buildDate=$(DATE)" ./cmd/main.go

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...
	@echo "Tests complete!"

deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@go mod tidy
	@echo "Dependencies updated!"

update-deps: ## Update dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "Dependencies updated!"

##@ Code Quality
.PHONY: lint format security-scan bench
lint: ## Run Go linters
	@echo "Running Go linters..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	@golangci-lint run ./...
	@echo "Linting complete!"

format: ## Format Go code
	@echo "Formatting Go code..."
	@go fmt ./...
	@echo "Code formatted!"

go-security-scan: ## Run security scan
	@echo "Running security scan..."
	@command -v govulncheck >/dev/null 2>&1 || { echo "Installing govulncheck..."; go install golang.org/x/vuln/cmd/govulncheck@latest; }
	@gosec ./...
	@echo "Security scan complete!"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...
	@echo "Benchmarks complete!"


##@ Tools
.PHONY: check-tools install-tools
# Check if required tools are installed
check-tools:
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting." >&2; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed. Aborting." >&2; exit 1; }
	@command -v $(DOCKER_COMPOSE) >/dev/null 2>&1 || { echo "Docker Compose is required but not installed. Aborting." >&2; exit 1; }

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest;
	@echo "Development tools installed!"