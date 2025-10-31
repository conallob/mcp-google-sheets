# MCP Google Sheets Server - Detailed Setup Guide

This guide provides step-by-step instructions for setting up the MCP Google Sheets Server with OAuth 2.0 authentication.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Google Cloud Setup](#google-cloud-setup)
3. [OAuth 2.0 Configuration](#oauth-20-configuration)
4. [Local Installation](#local-installation)
5. [OAuth Authentication](#oauth-authentication)
6. [MCP Client Configuration](#mcp-client-configuration)
7. [Testing the Connection](#testing-the-connection)
8. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Go**: Version 1.21 or higher
  - Download from: https://golang.org/doc/install
  - Verify installation: `go version`

- **Git**: For cloning the repository
  - Download from: https://git-scm.com/downloads

- **Google Account**: Any Google account (no special setup required)

## Google Cloud Setup

### Step 1: Create a Google Cloud Project

1. Navigate to [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown at the top of the page
3. Click "New Project"
4. Enter a project name (e.g., "MCP Google Sheets")
5. **Note**: No billing account is required for personal use!
6. Click "Create"
7. Wait for the project to be created and select it

### Step 2: Enable Google Sheets API

1. In your Google Cloud project, go to the navigation menu (☰)
2. Navigate to "APIs & Services" → "Library"
3. In the search bar, type "Google Sheets API"
4. Click on "Google Sheets API" in the results
5. Click the "Enable" button
6. Wait for the API to be enabled

## OAuth 2.0 Configuration

### Step 3: Configure OAuth Consent Screen

1. Navigate to "APIs & Services" → "OAuth consent screen"
2. Select **"External"** user type (unless you have a Google Workspace organization)
   - External allows any Google account user to authenticate
   - You can keep the app in testing mode for personal use
3. Click "Create"
4. Fill in the required information:
   - **App name**: `MCP Google Sheets` (or your preferred name)
   - **User support email**: Your email address
   - **Developer contact information**: Your email address
5. Click "Save and Continue"

### Step 4: Add Scopes

1. On the "Scopes" page, click "Add or Remove Scopes"
2. In the filter box, search for "Google Sheets API"
3. Select the following scope:
   - `https://www.googleapis.com/auth/spreadsheets` (full access to Google Sheets)
4. Click "Update"
5. Verify the scope appears in your list
6. Click "Save and Continue"

### Step 5: Add Test Users

**Note**: This step is only needed if your app is in testing mode (which is fine for personal use!)

1. On the "Test users" page, click "Add Users"
2. Enter your Google account email address (the one you'll use to authenticate)
3. Click "Add"
4. Click "Save and Continue"

### Step 6: Review and Complete

1. Review your OAuth consent screen configuration
2. Click "Back to Dashboard"

### Step 7: Create OAuth 2.0 Client ID

1. Navigate to "APIs & Services" → "Credentials"
2. Click "Create Credentials" at the top
3. Select "OAuth client ID"
4. For "Application type", select **"Desktop app"**
5. Give it a name (e.g., "MCP Google Sheets Desktop Client")
6. Click "Create"
7. A popup will appear with your client ID and secret
8. Click "Download JSON" to download the credentials file
9. **Important**: Save this file as `oauth_credentials.json` in your project directory

The downloaded file should look like:
```json
{
  "installed": {
    "client_id": "xxxxx.apps.googleusercontent.com",
    "project_id": "your-project-id",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_secret": "your-client-secret",
    "redirect_uris": ["http://localhost"]
  }
}
```

## Local Installation

### Step 8: Clone and Build the Server

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

### Step 9: Configure OAuth Credentials

1. Rename your downloaded OAuth credentials file to `oauth_credentials.json`
2. Move it to the project directory:
   ```bash
   mv ~/Downloads/client_secret_*.json ./oauth_credentials.json
   ```
3. Secure the file permissions:
   ```bash
   chmod 600 oauth_credentials.json
   ```

Alternatively, you can place the file in:
```bash
mkdir -p ~/.config/mcp-google-sheets
mv ~/Downloads/client_secret_*.json ~/.config/mcp-google-sheets/oauth_credentials.json
chmod 600 ~/.config/mcp-google-sheets/oauth_credentials.json
```

### Step 10: Set Environment Variable (Optional)

If you placed the credentials file in a custom location:

```bash
# Option 1: Export for current session
export GOOGLE_OAUTH_CREDENTIALS="$(pwd)/oauth_credentials.json"

# Option 2: Add to your shell profile for persistence
echo 'export GOOGLE_OAUTH_CREDENTIALS="/absolute/path/to/oauth_credentials.json"' >> ~/.bashrc
source ~/.bashrc

# Option 3: Use individual environment variables
export GOOGLE_OAUTH_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GOOGLE_OAUTH_CLIENT_SECRET="your-client-secret"
```

## OAuth Authentication

### Step 11: First Run - Authenticate with Google

Before using the MCP server, you need to authenticate:

```bash
# Run the server for the first time
./mcp-google-sheets
```

The server will:
1. Detect that no OAuth token exists
2. Display a URL for authorization
3. Wait for you to complete the OAuth flow

**In your browser:**
1. Visit the displayed URL (or it may open automatically)
2. Sign in with your Google account (the one you added as a test user)
3. Review the permissions requested
4. Click "Allow" to grant access
5. You'll see a success message

**Back in the terminal:**
- The server will receive the authorization code
- Exchange it for an access token and refresh token
- Save the tokens to `~/.config/mcp-google-sheets/token.json`
- You're now authenticated!

**Important Notes:**
- You only need to do this once
- The access token will be automatically refreshed when it expires
- The refresh token is long-lived
- If you need to re-authenticate, just delete `token.json` and run the server again

## MCP Client Configuration

### For Claude Code

#### Method 1: Using Claude Code Settings UI

1. Open Claude Code
2. Go to Settings → MCP Servers
3. Add a new server with:
   - **Name**: `google-sheets`
   - **Command**: `/absolute/path/to/mcp-google-sheets/mcp-google-sheets`
   - **Environment Variables** (optional):
     - `GOOGLE_OAUTH_CREDENTIALS`: `/absolute/path/to/oauth_credentials.json`

#### Method 2: Manual Configuration File

Edit or create `~/.claude/mcp_settings.json`:

```json
{
  "mcpServers": {
    "google-sheets": {
      "command": "/absolute/path/to/mcp-google-sheets/mcp-google-sheets",
      "env": {
        "GOOGLE_OAUTH_CREDENTIALS": "/absolute/path/to/oauth_credentials.json"
      }
    }
  }
}
```

**Note**: If you don't set the environment variable, the server will automatically look for:
1. `oauth_credentials.json` in the current directory
2. `~/.config/mcp-google-sheets/oauth_credentials.json`

**Replace** `/absolute/path/to/` with your actual paths.

### For Other MCP Clients

Configure the MCP client to:
- **Protocol**: stdio (standard input/output)
- **Command**: Path to the `mcp-google-sheets` binary
- **Environment** (optional):
  - `GOOGLE_OAUTH_CREDENTIALS` = path to `oauth_credentials.json`

## Testing the Connection

### Step 12: Test with Your Spreadsheets

**No sharing required!** You can now access any Google Sheet that your authenticated Google account has access to.

### Step 13: Test with Claude Code

1. Open Claude Code
2. Start a conversation and try commands like:

```
"Read data from my spreadsheet 1abc123def456"

"Create a new spreadsheet called 'Test Sheet'"

"Get information about spreadsheet 1abc123def456"
```

Replace `1abc123def456` with your actual spreadsheet ID (from the URL).

### Step 14: Manual Testing (Optional)

You can test the server directly:

```bash
# Start the server
./mcp-google-sheets

# In another terminal, send a test request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./mcp-google-sheets
```

## Troubleshooting

### Error: "unable to load OAuth configuration"

**Solution**: Ensure OAuth credentials are available:
```bash
# Check if file exists
ls oauth_credentials.json
# OR
ls ~/.config/mcp-google-sheets/oauth_credentials.json

# OR set environment variables
export GOOGLE_OAUTH_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GOOGLE_OAUTH_CLIENT_SECRET="your-client-secret"
```

### Error: "unable to get OAuth client" or Authorization Failed

**Possible causes**:
1. OAuth consent screen not configured
2. User not added as test user (if app is in testing mode)
3. Required scopes not added

**Solution**:
- Verify OAuth consent screen is configured in Google Cloud Console
- Add your email as a test user
- Ensure `https://www.googleapis.com/auth/spreadsheets` scope is added
- Try re-authenticating: `rm ~/.config/mcp-google-sheets/token.json && ./mcp-google-sheets`

### Error: "Permission denied" or "403 Forbidden"

**Solution**:
- Ensure your Google account has access to the spreadsheet
- You don't need to share with anyone else - just make sure you're authenticated with the right account
- If accessing someone else's sheet, ask them to share it with your Google account

### Error: "Spreadsheet not found" or "404"

**Possible causes**:
1. Incorrect spreadsheet ID
2. Spreadsheet not accessible by your Google account
3. Spreadsheet deleted

**Solution**:
- Verify the spreadsheet ID from the URL
- Check that your Google account can access the sheet
- Ensure spreadsheet exists

### Re-authenticating with a Different Account

To switch Google accounts:
```bash
# Remove the stored token
rm ~/.config/mcp-google-sheets/token.json

# Run the server again
./mcp-google-sheets

# Complete the OAuth flow with the new account
```

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

### Using Multiple Google Accounts

You can create multiple instances of the server with different OAuth credentials and tokens:

```json
{
  "mcpServers": {
    "google-sheets-personal": {
      "command": "/path/to/mcp-google-sheets",
      "env": {
        "GOOGLE_OAUTH_CREDENTIALS": "/path/to/personal-oauth.json",
        "GOOGLE_OAUTH_TOKEN_FILE": "/path/to/personal-token.json"
      }
    },
    "google-sheets-work": {
      "command": "/path/to/mcp-google-sheets",
      "env": {
        "GOOGLE_OAUTH_CREDENTIALS": "/path/to/work-oauth.json",
        "GOOGLE_OAUTH_TOKEN_FILE": "/path/to/work-token.json"
      }
    }
  }
}
```

### Publishing Your OAuth App (Optional)

If you want to share this with others or use it without test user limitations:

1. Go to "OAuth consent screen" in Google Cloud Console
2. Click "Publish App"
3. Submit for verification (may require domain verification)
4. Once verified, any Google user can authenticate

For personal use, keeping the app in testing mode is perfectly fine!

### Production Deployment

For production use:

1. **Use environment variables** for OAuth credentials instead of files
2. **Consider implementing token encryption** at rest
3. **Monitor API usage** in Google Cloud Console
4. **Set up logging and alerting**
5. **Regularly review authorized applications** in Google Account settings

## Security Best Practices

1. **Never commit `oauth_credentials.json` or `token.json` to version control**
   - Already in .gitignore, but verify
2. **Restrict file permissions**:
   ```bash
   chmod 600 oauth_credentials.json
   chmod 600 ~/.config/mcp-google-sheets/token.json
   ```
3. **Review authorized applications** periodically:
   - Visit https://myaccount.google.com/permissions
   - Revoke access for apps you no longer use
4. **Use separate OAuth clients** for different environments (dev, prod)
5. **Monitor API usage** in Google Cloud Console
6. **Keep your OAuth client secret secure** - treat it like a password

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
