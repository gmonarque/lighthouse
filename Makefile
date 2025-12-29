.PHONY: all build run dev clean test frontend backend docker copy-static

# Variables
BINARY_NAME=lighthouse
BUILD_DIR=./build
FRONTEND_DIR=./web
GO_FILES=$(shell find . -name '*.go' -not -path "./web/*")

# Default target
all: build

# Build everything
build: frontend copy-static backend

# Copy frontend build to static directory for embedding
copy-static:
	@echo "Copying frontend to static directory..."
	rm -rf ./internal/api/static
	cp -r ./web/build ./internal/api/static

# Build backend only
backend:
	@echo "Building backend..."
	CGO_ENABLED=1 go build -tags "fts5" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/lighthouse

# Build frontend only
frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && pnpm install && pnpm run build

# Run the application
run: build
	@echo "Starting Lighthouse..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Development mode - run backend with hot reload
dev:
	@echo "Starting development server..."
	@which air > /dev/null || go install github.com/air-verse/air@latest
	air

# Development mode - frontend only
dev-frontend:
	@echo "Starting frontend dev server..."
	cd $(FRONTEND_DIR) && npm run dev

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -rf $(FRONTEND_DIR)/build
	rm -rf $(FRONTEND_DIR)/node_modules
	rm -rf ./internal/api/static
	rm -rf ./data/*.db

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Initialize the database
init-db:
	@echo "Initializing database..."
	$(BUILD_DIR)/$(BINARY_NAME) --init-db

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t lighthouse:latest .

# Run with Docker Compose
docker-up:
	docker-compose up -d

# Stop Docker Compose
docker-down:
	docker-compose down

# Show logs
logs:
	docker-compose logs -f

# Generate API documentation
docs:
	@echo "Generating API documentation..."
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/lighthouse/main.go

# Lint the code
lint:
	@echo "Linting..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

# Format code
fmt:
	@echo "Formatting..."
	go fmt ./...
	cd $(FRONTEND_DIR) && npm run format 2>/dev/null || true

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	cd $(FRONTEND_DIR) && pnpm install

# Help
help:
	@echo "Lighthouse Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build frontend and backend"
	@echo "  make backend        Build backend only"
	@echo "  make frontend       Build frontend only"
	@echo "  make run            Build and run the application"
	@echo "  make dev            Run with hot reload (requires air)"
	@echo "  make dev-frontend   Run frontend dev server"
	@echo "  make clean          Clean build artifacts"
	@echo "  make test           Run tests"
	@echo "  make docker         Build Docker image"
	@echo "  make docker-up      Start with Docker Compose"
	@echo "  make docker-down    Stop Docker Compose"
	@echo "  make deps           Download dependencies"
	@echo "  make help           Show this help"
