# MCP Google Sheets Server - Detailed Setup Guide

This guide provides step-by-step instructions for setting up the MCP Google Sheets Server.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Google Cloud Setup](#google-cloud-setup)
3. [Service Account Configuration](#service-account-configuration)
4. [Local Installation](#local-installation)
5. [MCP Client Configuration](#mcp-client-configuration)
6. [Testing the Connection](#testing-the-connection)
7. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Go**: Version 1.21 or higher
  - Download from: https://golang.org/doc/install
  - Verify installation: `go version`

- **Git**: For cloning the repository
  - Download from: https://git-scm.com/downloads

- **Google Account**: To access Google Cloud Console

## Google Cloud Setup

### Step 1: Create a Google Cloud Project

1. Navigate to [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown at the top of the page
3. Click "New Project"
4. Enter a project name (e.g., "MCP Google Sheets")
5. Select a billing account (if required)
6. Click "Create"
7. Wait for the project to be created and select it

### Step 2: Enable Google Sheets API

1. In your Google Cloud project, go to the navigation menu (☰)
2. Navigate to "APIs & Services" → "Library"
3. In the search bar, type "Google Sheets API"
4. Click on "Google Sheets API" in the results
5. Click the "Enable" button
6. Wait for the API to be enabled

## Service Account Configuration

### Step 3: Create a Service Account

1. Navigate to "APIs & Services" → "Credentials"
2. Click "Create Credentials" at the top
3. Select "Service Account"
4. Fill in the service account details:
   - **Service account name**: `mcp-sheets-service`
   - **Service account ID**: (auto-generated)
   - **Description**: "Service account for MCP Google Sheets server"
5. Click "Create and Continue"
6. (Optional) Grant roles if needed:
   - For accessing organization-wide sheets, you might need specific roles
   - For personal sheets, you can skip this step
7. Click "Continue" and then "Done"

### Step 4: Create and Download Service Account Key

1. On the "Credentials" page, find your newly created service account
2. Click on the service account email
3. Go to the "Keys" tab
4. Click "Add Key" → "Create new key"
5. Select "JSON" as the key type
6. Click "Create"
7. The JSON key file will be automatically downloaded
8. **Important**: Save this file securely - you cannot download it again

### Step 5: Note the Service Account Email

In the downloaded JSON file, find the `client_email` field. It will look like:
```
mcp-sheets-service@your-project-id.iam.gserviceaccount.com
```

You'll need this email to share Google Sheets with the service account.

## Local Installation

### Step 6: Clone and Build the Server

```bash
# Clone the repository
git clone https://github.com/conallob/mcp-google-sheets.git
cd mcp-google-sheets

# Run the automated setup script
./setup.sh
```

Or manually:

```bash
# Install dependencies
go mod download
go mod tidy

# Build the server
go build -o mcp-google-sheets

# Or use the Makefile
make install
make build
```

### Step 7: Configure Credentials

1. Rename your downloaded service account key to `credentials.json`
2. Move it to the project directory:
   ```bash
   mv ~/Downloads/your-project-xyz123.json ./credentials.json
   ```
3. Secure the file permissions:
   ```bash
   chmod 600 credentials.json
   ```

### Step 8: Set Environment Variable

```bash
# Option 1: Export for current session
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/credentials.json"

# Option 2: Add to your shell profile for persistence
echo 'export GOOGLE_APPLICATION_CREDENTIALS="/absolute/path/to/mcp-google-sheets/credentials.json"' >> ~/.bashrc
source ~/.bashrc

# Option 3: Use the generated .env file
source .env
```

## MCP Client Configuration

### For Claude Code

#### Method 1: Using Claude Code Settings UI

1. Open Claude Code
2. Go to Settings → MCP Servers
3. Add a new server with:
   - **Name**: `google-sheets`
   - **Command**: `/absolute/path/to/mcp-google-sheets/mcp-google-sheets`
   - **Environment Variables**:
     - `GOOGLE_APPLICATION_CREDENTIALS`: `/absolute/path/to/credentials.json`

#### Method 2: Manual Configuration File

Edit or create `~/.claude/mcp_settings.json`:

```json
{
  "mcpServers": {
    "google-sheets": {
      "command": "/absolute/path/to/mcp-google-sheets/mcp-google-sheets",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "/absolute/path/to/credentials.json"
      }
    }
  }
}
```

**Replace** `/absolute/path/to/` with your actual paths.

### For Other MCP Clients

Configure the MCP client to:
- **Protocol**: stdio (standard input/output)
- **Command**: Path to the `mcp-google-sheets` binary
- **Environment**:
  - `GOOGLE_APPLICATION_CREDENTIALS` = path to `credentials.json`

## Testing the Connection

### Step 9: Share a Test Spreadsheet

1. Create or open a Google Sheet
2. Click the "Share" button
3. In the "Add people and groups" field, paste your service account email:
   ```
   mcp-sheets-service@your-project-id.iam.gserviceaccount.com
   ```
4. Set appropriate permissions (Editor recommended for testing)
5. Uncheck "Notify people" (the service account won't read emails)
6. Click "Share"

### Step 10: Test with Claude Code

1. Open Claude Code
2. Start a conversation and try commands like:

```
"Read data from my spreadsheet 1abc123def456"

"Create a new spreadsheet called 'Test Sheet'"

"Get information about spreadsheet 1abc123def456"
```

Replace `1abc123def456` with your actual spreadsheet ID (from the URL).

### Step 11: Manual Testing (Optional)

You can test the server directly:

```bash
# Start the server
./mcp-google-sheets

# In another terminal, send a test request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./mcp-google-sheets
```

## Troubleshooting

### Error: "GOOGLE_APPLICATION_CREDENTIALS environment variable not set"

**Solution**: Ensure the environment variable is set:
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials.json"
```

### Error: "Unable to create sheets service"

**Possible causes**:
1. Invalid credentials.json file
2. Incorrect file path
3. Missing Google Sheets API enablement

**Solution**:
- Verify the credentials file exists and is valid JSON
- Check the file path is absolute
- Re-enable the Google Sheets API in Google Cloud Console

### Error: "Permission denied" or "403 Forbidden"

**Solution**: Share the spreadsheet with your service account email

### Error: "Spreadsheet not found" or "404"

**Possible causes**:
1. Incorrect spreadsheet ID
2. Spreadsheet not shared with service account
3. Spreadsheet deleted

**Solution**:
- Verify the spreadsheet ID from the URL
- Check sharing settings
- Ensure spreadsheet exists

### Build Errors

If you encounter build errors:

```bash
# Clear the Go cache
go clean -modcache

# Re-download dependencies
go mod download

# Rebuild
go build -o mcp-google-sheets
```

### Connection Issues

If the MCP client can't connect to the server:

1. Verify the binary path is correct
2. Check the binary has execute permissions: `chmod +x mcp-google-sheets`
3. Test the server runs standalone
4. Check MCP client logs for detailed errors

## Advanced Configuration

### Using Multiple Service Accounts

You can create multiple instances of the server with different credentials:

```json
{
  "mcpServers": {
    "google-sheets-personal": {
      "command": "/path/to/mcp-google-sheets",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "/path/to/personal-credentials.json"
      }
    },
    "google-sheets-work": {
      "command": "/path/to/mcp-google-sheets",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "/path/to/work-credentials.json"
      }
    }
  }
}
```

### Production Deployment

For production use:

1. **Use Google Cloud Secret Manager** instead of local credential files
2. **Implement credential rotation** policies
3. **Use service accounts with minimal necessary permissions**
4. **Monitor API usage** in Google Cloud Console
5. **Set up logging and alerting**

## Security Best Practices

1. **Never commit credentials.json to version control**
   - Already in .gitignore, but verify
2. **Restrict file permissions**: `chmod 600 credentials.json`
3. **Regularly rotate service account keys** (every 90 days recommended)
4. **Use separate service accounts** for different environments (dev, staging, prod)
5. **Monitor service account activity** in Google Cloud Console
6. **Revoke unused service account keys**

## Getting Help

- **Documentation**: See [README.md](README.md)
- **Google Sheets API**: https://developers.google.com/sheets/api
- **MCP Protocol**: https://modelcontextprotocol.io/
- **Issues**: https://github.com/conallob/mcp-google-sheets/issues

## Next Steps

Once setup is complete:

1. Read the [README.md](README.md) for usage examples
2. Explore the available tools and their parameters
3. Try creating, reading, and updating spreadsheets
4. Integrate with your workflows using Claude Code or other MCP clients

Happy spreadsheeting!
