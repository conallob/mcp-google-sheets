#!/bin/bash

# Setup script for MCP Google Sheets Server

set -e

echo "==================================="
echo "MCP Google Sheets Server Setup"
echo "==================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21 or higher."
    echo "Visit: https://golang.org/doc/install"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Found Go version: $GO_VERSION"
echo ""

# Install dependencies
echo "Installing Go dependencies..."
go mod download
go mod tidy
echo "Dependencies installed successfully!"
echo ""

# Check for credentials file
if [ ! -f "credentials.json" ]; then
    echo "Warning: credentials.json not found!"
    echo ""
    echo "To set up Google Sheets API access:"
    echo "1. Go to https://console.cloud.google.com/"
    echo "2. Create a new project or select existing one"
    echo "3. Enable Google Sheets API"
    echo "4. Create a Service Account and download the JSON key"
    echo "5. Save the key as 'credentials.json' in this directory"
    echo ""
    echo "See credentials.example.json for the expected format."
    echo ""
else
    echo "Found credentials.json"
    echo ""
fi

# Build the server
echo "Building the MCP server..."
go build -o mcp-google-sheets .
echo "Build complete! Binary created: mcp-google-sheets"
echo ""

# Set up environment variable
echo "Setting up environment..."
CREDS_PATH="$(pwd)/credentials.json"
echo "export GOOGLE_APPLICATION_CREDENTIALS=\"$CREDS_PATH\"" > .env

echo "Environment configuration saved to .env"
echo ""
echo "To use the environment variables, run:"
echo "  source .env"
echo ""

echo "==================================="
echo "Setup Complete!"
echo "==================================="
echo ""
echo "Next steps:"
echo "1. Ensure you have credentials.json in this directory"
echo "2. Run: source .env"
echo "3. Test the server: ./mcp-google-sheets"
echo ""
echo "For Claude Code integration, add this to your MCP settings:"
echo ""
echo '{
  "mcpServers": {
    "google-sheets": {
      "command": "'$(pwd)'/mcp-google-sheets",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "'$CREDS_PATH'"
      }
    }
  }
}'
echo ""
