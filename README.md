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
- Google Cloud Project with Sheets API enabled
- Service Account credentials (JSON key file)

## Quick Start

### 1. Set Up Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Sheets API:
   - Navigate to "APIs & Services" > "Library"
   - Search for "Google Sheets API"
   - Click "Enable"

### 2. Create Service Account

1. Go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "Service Account"
3. Fill in the service account details and click "Create"
4. Grant the service account appropriate roles (optional for private sheets)
5. Click "Done"
6. Click on the created service account
7. Go to "Keys" tab
8. Click "Add Key" > "Create New Key"
9. Select "JSON" and click "Create"
10. Save the downloaded JSON file as `credentials.json` in the project directory

### 3. Share Sheets with Service Account

For the service account to access your Google Sheets:
1. Open the Google Sheet you want to access
2. Click "Share"
3. Add the service account email (found in `credentials.json` as `client_email`)
4. Grant appropriate permissions (Viewer, Editor, etc.)

### 4. Install and Build

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

### 5. Configure Environment

Set the environment variable to point to your credentials:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/credentials.json"
```

Or add to your shell profile (~/.bashrc, ~/.zshrc, etc.):

```bash
echo 'export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/credentials.json"' >> ~/.bashrc
source ~/.bashrc
```

## MCP Client Configuration

### Claude Code Configuration

Add to your Claude Code MCP settings (typically in `~/.claude/mcp_settings.json` or through Claude Code settings):

```json
{
  "mcpServers": {
    "google-sheets": {
      "command": "/path/to/mcp-google-sheets/mcp-google-sheets",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "/path/to/your/credentials.json"
      }
    }
  }
}
```

### Other MCP Clients

For other MCP clients, configure the server with:
- **Command**: Path to the `mcp-google-sheets` binary
- **Environment**: Set `GOOGLE_APPLICATION_CREDENTIALS` to your credentials file path
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

- Ensure `GOOGLE_APPLICATION_CREDENTIALS` points to a valid JSON key file
- Verify the service account email has access to the spreadsheet
- Check that the Google Sheets API is enabled in your Google Cloud project

### Permission Errors

- Make sure you've shared the spreadsheet with the service account email
- Grant appropriate permissions (Editor for write operations, Viewer for read-only)

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

- **Never commit `credentials.json`** to version control
- Store credentials securely and restrict file permissions:
  ```bash
  chmod 600 credentials.json
  ```
- Use service accounts with minimal necessary permissions
- Regularly rotate service account keys
- Consider using Google Cloud Secret Manager for production deployments

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
