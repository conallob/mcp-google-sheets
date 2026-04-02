package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/conallob/mcp-google-sheets/config"
	"github.com/conallob/mcp-google-sheets/oauth"
	"github.com/conallob/mcp-google-sheets/sheets"
)

const (
	serverName    = "mcp-google-sheets"
	serverVersion = "2.0.0"

	// scannerInitBytes is the initial buffer size for the stdin scanner.
	scannerInitBytes = 64 * 1024 // 64 KiB — sufficient for the vast majority of requests
	// scannerMaxBytes is the hard ceiling; large batch/write payloads can
	// exceed bufio's default 64 KiB buffer, so we allow growth up to 16 MiB.
	scannerMaxBytes = 16 * 1024 * 1024 // 16 MiB
)

// ── MCP protocol types ────────────────────────────────────────────────────────

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

// ── Server ────────────────────────────────────────────────────────────────────

type MCPServer struct {
	sheetsClient *sheets.Client
	cfg          *config.Config
	// ctx is stored here rather than threaded through every handler because the
	// MCP server's lifetime is tied to a single process invocation; there is no
	// need for per-request cancellation beyond what the process signal provides.
	ctx context.Context
}

func NewMCPServer(ctx context.Context) (*MCPServer, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("sheets config: %v", err)
	}

	oauthCfg, err := oauth.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("OAuth config: %v", err)
	}

	httpClient, err := oauthCfg.GetClient(ctx, cfg.NeedsWriteScope())
	if err != nil {
		return nil, fmt.Errorf("OAuth: %v", err)
	}

	return &MCPServer{
		sheetsClient: sheets.NewClient(httpClient),
		cfg:          cfg,
		ctx:          ctx,
	}, nil
}

// ── Request dispatch ──────────────────────────────────────────────────────────

func (s *MCPServer) handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	case "notifications/initialized":
		// MCP post-handshake notification — no response expected or sent.
		return MCPResponse{}
	default:
		return errResponse(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
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

// ── Tool definitions ──────────────────────────────────────────────────────────

func (s *MCPServer) handleToolsList(req MCPRequest) MCPResponse {
	tools := []map[string]interface{}{
		{
			"name":        "list_sheets",
			"description": "List all Google Sheets this MCP server has been granted access to, along with each sheet's permission level (read or readwrite).",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
		{
			"name":        "get_spreadsheet_info",
			"description": "Get metadata about a spreadsheet: title, locale, time zone, and the list of sheet tabs with their dimensions.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": prop("string", "The spreadsheet ID (from its URL)."),
				},
				"required": []string{"spreadsheet_id"},
			},
		},
		{
			"name":        "read_sheet",
			"description": "Read cell values from a Google Sheet. Returns the values as a 2-D array of strings together with row/column counts.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": prop("string", "The spreadsheet ID (from its URL)."),
					"range":          prop("string", "A1 notation range, e.g. 'Sheet1!A1:D10'. Defaults to the entire first sheet when omitted."),
				},
				"required": []string{"spreadsheet_id"},
			},
		},
		{
			"name":        "write_sheet",
			"description": "Overwrite a range of cells in a Google Sheet with the supplied values. Requires readwrite access on the sheet.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": prop("string", "The spreadsheet ID (from its URL)."),
					"range":          prop("string", "A1 notation range to write to, e.g. 'Sheet1!A1'."),
					"values": map[string]interface{}{
						"type":        "array",
						"description": "2-D array of values to write (array of rows; each row is an array of cell values).",
						"items": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
				},
				"required": []string{"spreadsheet_id", "range", "values"},
			},
		},
		{
			"name":        "append_sheet",
			"description": "Append rows after the last row that contains data in the given range. Requires readwrite access on the sheet.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": prop("string", "The spreadsheet ID (from its URL)."),
					"range":          prop("string", "A1 notation range, e.g. 'Sheet1!A:D' or simply 'Sheet1'."),
					"values": map[string]interface{}{
						"type":        "array",
						"description": "2-D array of rows to append.",
						"items": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
				},
				"required": []string{"spreadsheet_id", "range", "values"},
			},
		},
		{
			"name":        "clear_sheet",
			"description": "Clear all values from a range in a Google Sheet (formatting is preserved). Requires readwrite access.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": prop("string", "The spreadsheet ID (from its URL)."),
					"range":          prop("string", "A1 notation range to clear, e.g. 'Sheet1!A1:D10' or 'Sheet1'."),
				},
				"required": []string{"spreadsheet_id", "range"},
			},
		},
		{
			"name":        "add_sheet",
			"description": "Add a new sheet tab to an existing spreadsheet. Requires readwrite access.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"spreadsheet_id": prop("string", "The spreadsheet ID (from its URL)."),
					"sheet_name":     prop("string", "Name for the new sheet tab."),
				},
				"required": []string{"spreadsheet_id", "sheet_name"},
			},
		},
		{
			"name":        "create_spreadsheet",
			"description": "Create a new Google Spreadsheet. Requires at least one sheet configured with readwrite access. The returned spreadsheet ID must be added to the server's config before it can be accessed by other tools.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": prop("string", "Title of the new spreadsheet."),
					"sheets": map[string]interface{}{
						"type":        "array",
						"description": "Optional list of sheet tab names. A single default sheet is created when omitted.",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"title"},
			},
		},
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

// ── Tool dispatch ─────────────────────────────────────────────────────────────

func (s *MCPServer) handleToolsCall(req MCPRequest) MCPResponse {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return errResponse(req.ID, -32602, "invalid params: "+err.Error())
	}

	var (
		result interface{}
		err    error
	)
	switch p.Name {
	case "list_sheets":
		result, err = s.toolListSheets()
	case "get_spreadsheet_info":
		result, err = s.toolGetSpreadsheetInfo(p.Arguments)
	case "read_sheet":
		result, err = s.toolReadSheet(p.Arguments)
	case "write_sheet":
		result, err = s.toolWriteSheet(p.Arguments)
	case "append_sheet":
		result, err = s.toolAppendSheet(p.Arguments)
	case "clear_sheet":
		result, err = s.toolClearSheet(p.Arguments)
	case "add_sheet":
		result, err = s.toolAddSheet(p.Arguments)
	case "create_spreadsheet":
		result, err = s.toolCreateSpreadsheet(p.Arguments)
	default:
		return errResponse(req.ID, -32601, fmt.Sprintf("tool not found: %s", p.Name))
	}

	if err != nil {
		return errResponse(req.ID, -32000, err.Error())
	}

	text, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return errResponse(req.ID, -32000, fmt.Sprintf("failed to encode result: %v", jsonErr))
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": string(text)},
			},
		},
	}
}

// ── Tool implementations ──────────────────────────────────────────────────────

func (s *MCPServer) toolListSheets() (interface{}, error) {
	allowed := s.cfg.AllowedSheets()
	result := make([]map[string]interface{}, len(allowed))
	for i, sh := range allowed {
		entry := map[string]interface{}{
			"id":     sh.ID,
			"access": string(sh.Access),
		}
		if sh.Name != "" {
			entry["name"] = sh.Name
		}
		result[i] = entry
	}
	return map[string]interface{}{
		"sheets": result,
		"count":  len(result),
	}, nil
}

func (s *MCPServer) toolGetSpreadsheetInfo(args json.RawMessage) (interface{}, error) {
	var p struct {
		SpreadsheetID string `json:"spreadsheet_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	if !s.cfg.CanRead(p.SpreadsheetID) {
		return nil, permissionDenied(p.SpreadsheetID, "read")
	}
	return s.sheetsClient.GetSpreadsheetInfo(s.ctx, p.SpreadsheetID)
}

func (s *MCPServer) toolReadSheet(args json.RawMessage) (interface{}, error) {
	var p struct {
		SpreadsheetID string `json:"spreadsheet_id"`
		Range         string `json:"range,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	if !s.cfg.CanRead(p.SpreadsheetID) {
		return nil, permissionDenied(p.SpreadsheetID, "read")
	}
	return s.sheetsClient.ReadSheet(s.ctx, p.SpreadsheetID, p.Range)
}

func (s *MCPServer) toolWriteSheet(args json.RawMessage) (interface{}, error) {
	var p struct {
		SpreadsheetID string     `json:"spreadsheet_id"`
		Range         string     `json:"range"`
		Values        [][]string `json:"values"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	if !s.cfg.CanWrite(p.SpreadsheetID) {
		return nil, permissionDenied(p.SpreadsheetID, "readwrite")
	}
	return s.sheetsClient.WriteSheet(s.ctx, p.SpreadsheetID, p.Range, p.Values)
}

func (s *MCPServer) toolAppendSheet(args json.RawMessage) (interface{}, error) {
	var p struct {
		SpreadsheetID string     `json:"spreadsheet_id"`
		Range         string     `json:"range"`
		Values        [][]string `json:"values"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	if !s.cfg.CanWrite(p.SpreadsheetID) {
		return nil, permissionDenied(p.SpreadsheetID, "readwrite")
	}
	return s.sheetsClient.AppendSheet(s.ctx, p.SpreadsheetID, p.Range, p.Values)
}

func (s *MCPServer) toolClearSheet(args json.RawMessage) (interface{}, error) {
	var p struct {
		SpreadsheetID string `json:"spreadsheet_id"`
		Range         string `json:"range"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	if !s.cfg.CanWrite(p.SpreadsheetID) {
		return nil, permissionDenied(p.SpreadsheetID, "readwrite")
	}
	return s.sheetsClient.ClearSheet(s.ctx, p.SpreadsheetID, p.Range)
}

func (s *MCPServer) toolAddSheet(args json.RawMessage) (interface{}, error) {
	var p struct {
		SpreadsheetID string `json:"spreadsheet_id"`
		SheetName     string `json:"sheet_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	if !s.cfg.CanWrite(p.SpreadsheetID) {
		return nil, permissionDenied(p.SpreadsheetID, "readwrite")
	}
	return s.sheetsClient.AddSheet(s.ctx, p.SpreadsheetID, p.SheetName)
}

func (s *MCPServer) toolCreateSpreadsheet(args json.RawMessage) (interface{}, error) {
	var p struct {
		Title  string   `json:"title"`
		Sheets []string `json:"sheets,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, err
	}
	// create_spreadsheet is not tied to a pre-existing spreadsheet ID, so we
	// can't use CanWrite(id). Instead we gate on NeedsWriteScope(): if any
	// configured sheet is readwrite the OAuth token carries write permission,
	// which is sufficient to create a new spreadsheet.
	if !s.cfg.NeedsWriteScope() {
		return nil, fmt.Errorf("creating spreadsheets requires at least one sheet configured with readwrite access")
	}
	return s.sheetsClient.CreateSpreadsheet(s.ctx, p.Title, p.Sheets)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func prop(typ, description string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": description}
}

func errResponse(id interface{}, code int, msg string) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: msg},
	}
}

func permissionDenied(spreadsheetID, required string) error {
	return fmt.Errorf("spreadsheet %q is not in the allowed sheets config (required access: %s)", spreadsheetID, required)
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s %s\n", serverName, serverVersion)
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx := context.Background()
	server, err := NewMCPServer(ctx)
	if err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, scannerInitBytes), scannerMaxBytes)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("Failed to parse request: %v", err)
			continue
		}

		resp := server.handleRequest(req)
		// JSON-RPC 2.0: notifications (absent/null id) must not receive a response.
		if req.ID == nil {
			continue
		}
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin read error: %v", err)
	}
}
