.PHONY: all build build-ui build-bin test test-coverage bench clean run

APP_NAME := imgscale
BIN_DIR := bin
CMD_DIR := cmd/imgscale

all: build

build: build-ui build-bin

build-ui:
	@echo "Building React 19 visual studio..."
	@cd web && npm install && npm run build

build-bin:
	@echo "Compiling Go binary..."
	@go build -v -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME) ./$(CMD_DIR)

test:
	@echo "Running tests..."
	@go test -v -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

bench:
	@echo "Running benchmarks..."
	@go test -benchmem -bench=. ./internal/engine/...

run: build-bin
	@./$(BIN_DIR)/$(APP_NAME) serve --port 8080 --storage-dir ./testdata

clean:
	@rm -rf $(BIN_DIR) web/dist coverage.out coverage.html
