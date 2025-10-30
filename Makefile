.PHONY: build clean install test run

BINARY_NAME=mcp-google-sheets
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) .
	@echo "Build complete!"

clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(GOBIN)
	@go clean
	@echo "Clean complete!"

install:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed!"

test:
	@echo "Running tests..."
	@go test -v ./...
	@echo "Tests complete!"

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux .
	@echo "Linux build complete!"

build-darwin:
	@echo "Building for macOS..."
	@GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-mac .
	@echo "macOS build complete!"

build-windows:
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME).exe .
	@echo "Windows build complete!"

help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  clean       - Remove built binaries and clean cache"
	@echo "  install     - Install Go dependencies"
	@echo "  test        - Run tests"
	@echo "  run         - Build and run the server"
	@echo "  build-all   - Build for all platforms (Linux, macOS, Windows)"
	@echo "  help        - Show this help message"
