package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/conallob/mcp-google-sheets/sheets"
	"google.golang.org/api/option"
	sheetsapi "google.golang.org/api/sheets/v4"
)

const (
	serverName    = "mcp-google-sheets"
	serverVersion = "1.0.0"
)

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MCPServer struct {
	sheetsClient *sheets.Client
	ctx          context.Context
}

func NewMCPServer(ctx context.Context) (*MCPServer, error) {
	credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credPath == "" {
		return nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS environment variable not set")
	}

	srv, err := sheetsapi.NewService(ctx, option.WithCredentialsFile(credPath))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service: %v", err)
	}

	return &MCPServer{
		sheetsClient: sheets.NewClient(srv),
		ctx:          ctx,
	}, nil
}

func (s *MCPServer) handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (s *MCPServer) handleInitialize(req MCPRequest) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    serverName,
				"version": serverVersion,
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{},
			},
		},
	}
}

func (s *MCPServer) handleToolsList(req MCPRequest) MCPResponse {
	tools := []map[string]interface{}{
		{
			"name":        "read_sheet",
			"description": "Read data from a Google Sheet. Specify the spreadsheet ID and optional range (e.g., 'Sheet1!A1:D10'). If no range is provided, reads the entire first sheet.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet (from the URL)",
					},
					"range": map[string]interface{}{
						"type":        "string",
						"description": "The A1 notation range to read (e.g., 'Sheet1!A1:D10'). Optional - defaults to entire first sheet.",
					},
				},
				"required": []string{"spreadsheet_id"},
			},
		},
		{
			"name":        "write_sheet",
			"description": "Write data to a Google Sheet. Specify the spreadsheet ID, range, and data as a 2D array. Data overwrites existing content in the range.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet (from the URL)",
					},
					"range": map[string]interface{}{
						"type":        "string",
						"description": "The A1 notation range to write to (e.g., 'Sheet1!A1:D10')",
					},
					"values": map[string]interface{}{
						"type":        "array",
						"description": "2D array of values to write (array of rows, each row is an array of cell values)",
						"items": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"required": []string{"spreadsheet_id", "range", "values"},
			},
		},
		{
			"name":        "append_sheet",
			"description": "Append data to a Google Sheet. Adds new rows after the last row with data in the specified range.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet (from the URL)",
					},
					"range": map[string]interface{}{
						"type":        "string",
						"description": "The A1 notation range (e.g., 'Sheet1!A:D' or 'Sheet1')",
					},
					"values": map[string]interface{}{
						"type":        "array",
						"description": "2D array of values to append (array of rows)",
						"items": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"required": []string{"spreadsheet_id", "range", "values"},
			},
		},
		{
			"name":        "create_spreadsheet",
			"description": "Create a new Google Spreadsheet with the specified title and optional sheet names.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "The title of the new spreadsheet",
					},
					"sheets": map[string]interface{}{
						"type":        "array",
						"description": "Optional array of sheet names to create. If not provided, creates one default sheet.",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"title"},
			},
		},
		{
			"name":        "get_spreadsheet_info",
			"description": "Get metadata about a spreadsheet including title, sheets, and properties.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet (from the URL)",
					},
				},
				"required": []string{"spreadsheet_id"},
			},
		},
		{
			"name":        "add_sheet",
			"description": "Add a new sheet (tab) to an existing spreadsheet.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet",
					},
					"sheet_name": map[string]interface{}{
						"type":        "string",
						"description": "The name for the new sheet",
					},
				},
				"required": []string{"spreadsheet_id", "sheet_name"},
			},
		},
		{
			"name":        "clear_sheet",
			"description": "Clear all data in a specified range of a Google Sheet.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet",
					},
					"range": map[string]interface{}{
						"type":        "string",
						"description": "The A1 notation range to clear (e.g., 'Sheet1!A1:D10' or 'Sheet1')",
					},
				},
				"required": []string{"spreadsheet_id", "range"},
			},
		},
		{
			"name":        "batch_update",
			"description": "Perform multiple update operations on a spreadsheet in a single request. Supports formatting, adding sheets, and more complex operations.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the Google Spreadsheet",
					},
					"requests": map[string]interface{}{
						"type":        "array",
						"description": "Array of update request objects (see Google Sheets API documentation for request format)",
					},
				},
				"required": []string{"spreadsheet_id", "requests"},
			},
		},
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *MCPServer) handleToolsCall(req MCPRequest) MCPResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	var result interface{}
	var err error

	switch params.Name {
	case "read_sheet":
		result, err = s.handleReadSheet(params.Arguments)
	case "write_sheet":
		result, err = s.handleWriteSheet(params.Arguments)
	case "append_sheet":
		result, err = s.handleAppendSheet(params.Arguments)
	case "create_spreadsheet":
		result, err = s.handleCreateSpreadsheet(params.Arguments)
	case "get_spreadsheet_info":
		result, err = s.handleGetSpreadsheetInfo(params.Arguments)
	case "add_sheet":
		result, err = s.handleAddSheet(params.Arguments)
	case "clear_sheet":
		result, err = s.handleClearSheet(params.Arguments)
	case "batch_update":
		result, err = s.handleBatchUpdate(params.Arguments)
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Tool not found: %s", params.Name),
			},
		}
	}

	if err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}
}

func (s *MCPServer) handleReadSheet(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string `json:"spreadsheet_id"`
		Range         string `json:"range,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.ReadSheet(s.ctx, params.SpreadsheetID, params.Range)
}

func (s *MCPServer) handleWriteSheet(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string     `json:"spreadsheet_id"`
		Range         string     `json:"range"`
		Values        [][]string `json:"values"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.WriteSheet(s.ctx, params.SpreadsheetID, params.Range, params.Values)
}

func (s *MCPServer) handleAppendSheet(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string     `json:"spreadsheet_id"`
		Range         string     `json:"range"`
		Values        [][]string `json:"values"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.AppendSheet(s.ctx, params.SpreadsheetID, params.Range, params.Values)
}

func (s *MCPServer) handleCreateSpreadsheet(args json.RawMessage) (interface{}, error) {
	var params struct {
		Title  string   `json:"title"`
		Sheets []string `json:"sheets,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.CreateSpreadsheet(s.ctx, params.Title, params.Sheets)
}

func (s *MCPServer) handleGetSpreadsheetInfo(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string `json:"spreadsheet_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.GetSpreadsheetInfo(s.ctx, params.SpreadsheetID)
}

func (s *MCPServer) handleAddSheet(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string `json:"spreadsheet_id"`
		SheetName     string `json:"sheet_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.AddSheet(s.ctx, params.SpreadsheetID, params.SheetName)
}

func (s *MCPServer) handleClearSheet(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string `json:"spreadsheet_id"`
		Range         string `json:"range"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.ClearSheet(s.ctx, params.SpreadsheetID, params.Range)
}

func (s *MCPServer) handleBatchUpdate(args json.RawMessage) (interface{}, error) {
	var params struct {
		SpreadsheetID string                   `json:"spreadsheet_id"`
		Requests      []map[string]interface{} `json:"requests"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	return s.sheetsClient.BatchUpdate(s.ctx, params.SpreadsheetID, params.Requests)
}

func main() {
	// Parse command-line flags
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	// Handle --version flag
	if *versionFlag {
		fmt.Printf("%s version %s\n", serverName, serverVersion)
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx := context.Background()
	server, err := NewMCPServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		resp := server.handleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading from stdin: %v", err)
	}
}
