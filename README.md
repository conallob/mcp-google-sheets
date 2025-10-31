# MCP Google Sheets Server

A Model Context Protocol (MCP) server written in Go that provides native integration with Google Sheets API. This server enables Claude and other MCP clients to create, read, update, and manage Google Sheets without resorting to xlsx files or Python code.

## Features

- **Read Google Sheets**: Retrieve data from any sheet with flexible range selection
- **Write & Update**: Write data to specific ranges or update existing content
- **Append Data**: Add new rows to sheets without overwriting existing data
- **Create Spreadsheets**: Create new Google Sheets programmatically
- **Sheet Management**: Add new sheets (tabs), clear data, get spreadsheet metadata
- **Batch Operations**: Perform multiple updates in a single request for efficiency
- **Native Go Implementation**: Fast, lightweight, and efficient
- **MCP Protocol**: Full compatibility with Claude Code and other MCP clients

## Prerequisites

- Go 1.21 or higher
- Google Account (no GCP project required!)
- OAuth 2.0 Client credentials (easy to set up)

## Quick Start

### 1. Set Up OAuth 2.0 Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or use an existing one)
3. Enable the Google Sheets API:
   - Navigate to "APIs & Services" > "Library"
   - Search for "Google Sheets API"
   - Click "Enable"
4. Configure OAuth Consent Screen:
   - Go to "APIs & Services" > "OAuth consent screen"
   - Select "External" user type (unless you have a Google Workspace)
   - Fill in app name (e.g., "MCP Google Sheets")
   - Add your email as developer contact
   - Click "Save and Continue"
   - Add scope: `https://www.googleapis.com/auth/spreadsheets`
   - Click "Save and Continue"
   - Add your email as a test user
   - Click "Save and Continue"
5. Create OAuth 2.0 Client ID:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - Select "Desktop app" as application type
   - Give it a name (e.g., "MCP Google Sheets Desktop")
   - Click "Create"
   - Download the JSON file
   - Save it as `oauth_credentials.json` in the project directory

### 2. Install and Build

```bash
# Clone the repository
git clone https://github.com/conallob/mcp-google-sheets.git
cd mcp-google-sheets

# Download dependencies
go mod download

# Build the server
go build -o mcp-google-sheets

# Or run directly
go run main.go
```

### 3. First Run - OAuth Authentication

On the first run, you'll be prompted to authorize the application:

```bash
# Run the server (it will guide you through OAuth)
./mcp-google-sheets
```

The server will:
1. Display a URL for authorization
2. Open your browser automatically (or you can copy/paste the URL)
3. Ask you to sign in with your Google account
4. Request permission to access your Google Sheets
5. Save the access token for future use

After authorization, you can access any Google Sheets you own or have access to - no need to share with a service account!

## MCP Client Configuration

### Claude Code Configuration

Add to your Claude Code MCP settings (typically in `~/.claude/mcp_settings.json` or through Claude Code settings):

```json
{
  "mcpServers": {
    "google-sheets": {
      "command": "/path/to/mcp-google-sheets/mcp-google-sheets",
      "env": {
        "GOOGLE_OAUTH_CREDENTIALS": "/path/to/oauth_credentials.json"
      }
    }
  }
}
```

**Note**: If you don't set the `GOOGLE_OAUTH_CREDENTIALS` environment variable, the server will look for `oauth_credentials.json` in:
1. Current directory
2. `~/.config/mcp-google-sheets/oauth_credentials.json`

You can also use individual environment variables:
```json
{
  "mcpServers": {
    "google-sheets": {
      "command": "/path/to/mcp-google-sheets/mcp-google-sheets",
      "env": {
        "GOOGLE_OAUTH_CLIENT_ID": "your-client-id.apps.googleusercontent.com",
        "GOOGLE_OAUTH_CLIENT_SECRET": "your-client-secret"
      }
    }
  }
}
```

### Other MCP Clients

For other MCP clients, configure the server with:
- **Command**: Path to the `mcp-google-sheets` binary
- **Environment** (optional): Set `GOOGLE_OAUTH_CREDENTIALS` to your OAuth credentials file path
- **Protocol**: stdio (standard input/output)

## Available Tools

### read_sheet

Read data from a Google Sheet.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID from the URL
- `range` (optional): A1 notation range (e.g., "Sheet1!A1:D10"). Defaults to entire first sheet.

**Example:**
```json
{
  "spreadsheet_id": "1abc123def456",
  "range": "Sheet1!A1:C10"
}
```

### write_sheet

Write data to a Google Sheet, overwriting existing content.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID
- `range` (required): A1 notation range to write to
- `values` (required): 2D array of values (rows and columns)

**Example:**
```json
{
  "spreadsheet_id": "1abc123def456",
  "range": "Sheet1!A1:C2",
  "values": [
    ["Name", "Age", "City"],
    ["Alice", "30", "NYC"]
  ]
}
```

### append_sheet

Append data to a sheet after the last row with data.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID
- `range` (required): Range indicating which columns to append to
- `values` (required): 2D array of values to append

**Example:**
```json
{
  "spreadsheet_id": "1abc123def456",
  "range": "Sheet1!A:C",
  "values": [
    ["Bob", "25", "SF"],
    ["Charlie", "35", "LA"]
  ]
}
```

### create_spreadsheet

Create a new Google Spreadsheet.

**Parameters:**
- `title` (required): Title for the new spreadsheet
- `sheets` (optional): Array of sheet names to create

**Example:**
```json
{
  "title": "My New Spreadsheet",
  "sheets": ["Data", "Analysis", "Summary"]
}
```

**Returns:** Spreadsheet ID and URL

### get_spreadsheet_info

Get metadata about a spreadsheet.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID

**Returns:** Information about sheets, dimensions, properties, etc.

### add_sheet

Add a new sheet (tab) to an existing spreadsheet.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID
- `sheet_name` (required): Name for the new sheet

### clear_sheet

Clear all data in a specified range.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID
- `range` (required): A1 notation range to clear

### batch_update

Perform multiple operations in a single request. Supports complex operations like formatting, conditional formatting, adding/deleting rows, etc.

**Parameters:**
- `spreadsheet_id` (required): The spreadsheet ID
- `requests` (required): Array of request objects (see [Google Sheets API documentation](https://developers.google.com/sheets/api/reference/rest/v4/spreadsheets/request))

## Usage Examples

### With Claude Code

Once configured, you can ask Claude to interact with your sheets:

```
"Read the data from my spreadsheet 1abc123def456 in range Sheet1!A1:E100"

"Create a new spreadsheet called 'Sales Report 2024' with sheets: Q1, Q2, Q3, Q4"

"Append the following data to my sheet 1abc123def456 in the range Sheet1!A:D:
Name,Product,Quantity,Price
John,Widget,5,29.99
Jane,Gadget,3,49.99"

"Clear the range Sheet1!A1:Z1000 in spreadsheet 1abc123def456"
```

### Programmatic Usage

You can also integrate this server into your own MCP client applications. The server communicates via JSON-RPC 2.0 over stdio.

## Finding Spreadsheet IDs

The spreadsheet ID is in the URL of your Google Sheet:
```
https://docs.google.com/spreadsheets/d/1abc123def456ghi789/edit
                                      ^^^^^^^^^^^^^^^^
                                      This is the ID
```

## Troubleshooting

### Authentication Errors

- If you get "unable to load OAuth configuration", make sure:
  - `oauth_credentials.json` exists in the project directory, OR
  - `GOOGLE_OAUTH_CREDENTIALS` environment variable is set, OR
  - `GOOGLE_OAUTH_CLIENT_ID` and `GOOGLE_OAUTH_CLIENT_SECRET` environment variables are set
- Check that the Google Sheets API is enabled in your Google Cloud project
- Verify your OAuth consent screen is configured correctly

### Permission Errors

- The authenticated user must have access to the spreadsheet
- You can access any sheets your Google account can access
- No need to share sheets with a service account anymore!

### Re-authenticating

If you need to re-authenticate (e.g., revoked access or different account):
```bash
# Remove the stored token
rm ~/.config/mcp-google-sheets/token.json

# Or if using custom token location
rm /path/to/your/token.json

# Run the server again to re-authenticate
./mcp-google-sheets
```

### Connection Errors

- Verify your internet connection
- Check if Google APIs are accessible from your network
- Ensure firewall/proxy settings allow access to googleapis.com

## Development

### Project Structure

```
mcp-google-sheets/
├── main.go                      # MCP server implementation
├── sheets/
│   └── client.go               # Google Sheets API client
├── go.mod                      # Go module definition
├── credentials.example.json    # Example credentials file
└── README.md                   # This file
```

### Building from Source

```bash
# Install dependencies
go mod download

# Run tests (if available)
go test ./...

# Build
go build -o mcp-google-sheets

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o mcp-google-sheets-linux
GOOS=darwin GOARCH=amd64 go build -o mcp-google-sheets-mac
GOOS=windows GOARCH=amd64 go build -o mcp-google-sheets.exe
```

## Security Considerations

- **Never commit `oauth_credentials.json` or `token.json`** to version control (already in .gitignore)
- Store OAuth credentials securely and restrict file permissions:
  ```bash
  chmod 600 oauth_credentials.json
  chmod 600 ~/.config/mcp-google-sheets/token.json
  ```
- OAuth tokens expire and are automatically refreshed
- You can revoke access at any time from [Google Account Permissions](https://myaccount.google.com/permissions)
- For production deployments, consider using environment variables for OAuth credentials

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

See [LICENSE](LICENSE) file for details.

## Resources

- [Model Context Protocol Documentation](https://modelcontextprotocol.io/)
- [Google Sheets API Documentation](https://developers.google.com/sheets/api)
- [Google Cloud Authentication](https://cloud.google.com/docs/authentication)

## Support

For issues, questions, or contributions, please use the GitHub issue tracker.
