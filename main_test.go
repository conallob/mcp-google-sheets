package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/conallob/mcp-google-sheets/config"
	"github.com/conallob/mcp-google-sheets/sheets"
)

// newTestServer returns an MCPServer with an empty config (no sheets allowed).
// The sheets client is nil; tests that exercise tool execution rely on the
// permission check returning early before any HTTP call is made.
func newTestServer() *MCPServer {
	return &MCPServer{
		ctx: context.Background(),
		cfg: &config.Config{},
	}
}

// newTestServerWithSheet returns an MCPServer that has one configured sheet
// with the given access level. The sheets client points at a local test server
// returning HTTP 500, so any API call that passes the permission check fails
// explicitly rather than panicking on a nil client.
func newTestServerWithSheet(t *testing.T, id string, access config.Access) *MCPServer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return &MCPServer{
		ctx: context.Background(),
		cfg: &config.Config{
			Sheets: []config.SpreadsheetPermission{{ID: id, Access: access}},
		},
		sheetsClient: sheets.NewClientWithBaseURL(srv.Client(), srv.URL),
	}
}

// ── JSON parsing / serialisation ───────────────────────────────────────────

func TestMCPRequest_JSONParsing(t *testing.T) {
	var req MCPRequest
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`), &req); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got %q", req.JSONRPC)
	}
	if req.Method != "initialize" {
		t.Errorf("Expected Method 'initialize', got %q", req.Method)
	}
}

func TestMCPResponse_JSONSerialization(t *testing.T) {
	resp := MCPResponse{JSONRPC: "2.0", ID: 1, Result: map[string]interface{}{"status": "ok"}}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got %v", parsed["jsonrpc"])
	}
}

func TestMCPError_Structure(t *testing.T) {
	e := MCPError{Code: -32601, Message: "Method not found", Data: "info"}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if parsed["code"] != float64(-32601) {
		t.Errorf("Expected code -32601, got %v", parsed["code"])
	}
}

func TestMCPRequest_WithDifferentIDTypes(t *testing.T) {
	var r1 MCPRequest
	json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":123,"method":"test"}`), &r1)
	if r1.ID != float64(123) {
		t.Errorf("Expected float64(123), got %v", r1.ID)
	}

	var r2 MCPRequest
	json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":"abc-123","method":"test"}`), &r2)
	if r2.ID != "abc-123" {
		t.Errorf("Expected 'abc-123', got %v", r2.ID)
	}

	var r3 MCPRequest
	json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":null,"method":"test"}`), &r3)
	if r3.ID != nil {
		t.Errorf("Expected nil, got %v", r3.ID)
	}
}

func TestMCPResponse_ErrorResponse(t *testing.T) {
	resp := MCPResponse{JSONRPC: "2.0", ID: 1, Error: &MCPError{Code: -32700, Message: "Parse error"}}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed MCPResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if parsed.Error == nil || parsed.Error.Code != -32700 {
		t.Errorf("Unexpected error: %v", parsed.Error)
	}
	if parsed.Result != nil {
		t.Error("Expected nil result when error is set")
	}
}

func TestMCPResponse_BothResultAndError(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0", ID: 1,
		Result: map[string]interface{}{"data": "test"},
		Error:  &MCPError{Code: -32000, Message: "Error"},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if _, ok := parsed["result"]; !ok {
		t.Error("Result should be in JSON")
	}
	if _, ok := parsed["error"]; !ok {
		t.Error("Error should be in JSON")
	}
}

// ── initialize ─────────────────────────────────────────────────────────────

func TestHandleRequest_Initialize(t *testing.T) {
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected '2.0', got %q", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("Unexpected protocolVersion: %v", result["protocolVersion"])
	}
	info := result["serverInfo"].(map[string]string)
	if info["name"] != serverName {
		t.Errorf("Expected name %q, got %q", serverName, info["name"])
	}
}

func TestHandleInitialize_IDPreserved(t *testing.T) {
	server := newTestServer()
	resp := server.handleInitialize(MCPRequest{JSONRPC: "2.0", ID: "test-id", Method: "initialize"})
	if resp.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %v", resp.ID)
	}
	result := resp.Result.(map[string]interface{})
	if _, ok := result["capabilities"].(map[string]interface{}); !ok {
		t.Error("Expected capabilities map")
	}
}

// ── notifications ──────────────────────────────────────────────────────────

func TestHandleRequest_NotificationsInitialized(t *testing.T) {
	// The server must handle notifications/initialized without error and return
	// an empty response (the caller must not send it over the wire).
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	if resp.Error != nil {
		t.Errorf("notifications/initialized should not produce an error: %v", resp.Error)
	}
}

// ── ping ───────────────────────────────────────────────────────────────────

func TestHandleRequest_Ping(t *testing.T) {
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 2, Method: "ping"})
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	if len(resp.Result.(map[string]interface{})) != 0 {
		t.Error("Expected empty result for ping")
	}
}

// ── method not found ────────────────────────────────────────────────────────

func TestHandleRequest_MethodNotFound(t *testing.T) {
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 3, Method: "nonexistent_method"})
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected -32601, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "nonexistent_method") {
		t.Errorf("Expected method name in message, got: %q", resp.Error.Message)
	}
}

func TestErrorMessages_MethodNotFound(t *testing.T) {
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: "invalid_method"})
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(resp.Error.Message, "method not found") {
		t.Errorf("Expected lowercase 'method not found' in %q", resp.Error.Message)
	}
	if !strings.Contains(resp.Error.Message, "invalid_method") {
		t.Errorf("Expected method name in message, got %q", resp.Error.Message)
	}
}

// ── tools/list ─────────────────────────────────────────────────────────────

func TestHandleRequest_ToolsList(t *testing.T) {
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 4, Method: "tools/list"})
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	tools := resp.Result.(map[string]interface{})["tools"].([]map[string]interface{})

	expected := []string{"list_sheets", "get_spreadsheet_info", "read_sheet", "write_sheet", "append_sheet", "clear_sheet", "add_sheet", "create_spreadsheet"}
	if len(tools) != len(expected) {
		t.Errorf("Expected %d tools, got %d", len(expected), len(tools))
	}
	for i, tool := range tools {
		if tool["name"] != expected[i] {
			t.Errorf("Tool %d: expected %q, got %q", i, expected[i], tool["name"])
		}
	}
}

func TestHandleToolsList_AllToolsHaveRequiredFields(t *testing.T) {
	server := newTestServer()
	resp := server.handleToolsList(MCPRequest{})
	tools := resp.Result.(map[string]interface{})["tools"].([]map[string]interface{})

	for _, tool := range tools {
		name := tool["name"].(string)
		if _, ok := tool["description"].(string); !ok {
			t.Errorf("Tool %q missing description", name)
		}
		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %q missing inputSchema", name)
			continue
		}
		if inputSchema["type"] != "object" {
			t.Errorf("Tool %q: inputSchema type should be 'object'", name)
		}
	}
}

func TestAllToolsSchemasAreValid(t *testing.T) {
	server := newTestServer()
	resp := server.handleToolsList(MCPRequest{})
	tools := resp.Result.(map[string]interface{})["tools"].([]map[string]interface{})

	for _, tool := range tools {
		name := tool["name"].(string)
		desc, ok := tool["description"].(string)
		if !ok || desc == "" {
			t.Errorf("Tool %q has invalid description", name)
		}
		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %q has invalid inputSchema", name)
			continue
		}
		if inputSchema["type"] != "object" {
			t.Errorf("Tool %q: type should be 'object'", name)
		}
		// All tools must declare a required array (may be empty for no-arg tools).
		if _, ok := inputSchema["required"].([]string); !ok {
			t.Errorf("Tool %q has invalid required array", name)
		}
		// Tools with arguments must have a non-empty properties map.
		if name != "list_sheets" {
			props, ok := inputSchema["properties"].(map[string]interface{})
			if !ok || len(props) == 0 {
				t.Errorf("Tool %q has no properties", name)
			}
		}
	}
}

func TestReadSheetToolSchema(t *testing.T) {
	server := newTestServer()
	resp := server.handleToolsList(MCPRequest{})
	tools := resp.Result.(map[string]interface{})["tools"].([]map[string]interface{})

	var readSheetTool map[string]interface{}
	for _, tool := range tools {
		if tool["name"] == "read_sheet" {
			readSheetTool = tool
			break
		}
	}
	if readSheetTool == nil {
		t.Fatal("read_sheet tool not found")
	}

	inputSchema := readSheetTool["inputSchema"].(map[string]interface{})
	required := inputSchema["required"].([]string)

	if !containsSlice(required, "spreadsheet_id") {
		t.Error("spreadsheet_id should be required for read_sheet")
	}
	if containsSlice(required, "range") {
		t.Error("range should be optional for read_sheet")
	}
}

// ── tools/call ─────────────────────────────────────────────────────────────

func TestHandleToolsCall_InvalidParams(t *testing.T) {
	server := newTestServer()
	resp := server.handleToolsCall(MCPRequest{JSONRPC: "2.0", ID: 6, Method: "tools/call", Params: json.RawMessage(`invalid json`)})
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("Expected -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "invalid params") {
		t.Errorf("Expected 'invalid params' in message, got: %q", resp.Error.Message)
	}
}

func TestHandleToolsCall_ToolNotFound(t *testing.T) {
	server := newTestServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "nonexistent_tool", "arguments": json.RawMessage(`{}`)})
	resp := server.handleToolsCall(MCPRequest{JSONRPC: "2.0", ID: 7, Params: params})
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected -32601, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "nonexistent_tool") {
		t.Errorf("Expected tool name in message, got: %q", resp.Error.Message)
	}
}

func TestErrorMessages_ToolNotFound(t *testing.T) {
	server := newTestServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "nonexistent_tool", "arguments": json.RawMessage(`{}`)})
	resp := server.handleToolsCall(MCPRequest{JSONRPC: "2.0", ID: 1, Params: params})
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(resp.Error.Message, "tool not found") {
		t.Errorf("Expected lowercase 'tool not found' in %q", resp.Error.Message)
	}
}

func TestErrorMessages_InvalidParams(t *testing.T) {
	server := newTestServer()
	resp := server.handleToolsCall(MCPRequest{JSONRPC: "2.0", ID: 1, Params: json.RawMessage(`not valid json`)})
	if resp.Error == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(resp.Error.Message, "invalid params") {
		t.Errorf("Expected 'invalid params' in %q", resp.Error.Message)
	}
	if resp.Error.Code != -32602 {
		t.Errorf("Expected -32602, got %d", resp.Error.Code)
	}
}

func TestHandleToolsCall_ResultFormatting(t *testing.T) {
	server := newTestServer()
	params, _ := json.Marshal(map[string]interface{}{
		"name":      "read_sheet",
		"arguments": map[string]interface{}{"spreadsheet_id": "test-id"},
	})
	resp := server.handleToolsCall(MCPRequest{JSONRPC: "2.0", ID: "test-123", Params: params})

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected '2.0', got %q", resp.JSONRPC)
	}
	if resp.ID != "test-123" {
		t.Errorf("Expected ID 'test-123', got %v", resp.ID)
	}
	// With empty config, we expect a permission-denied error (-32000).
	if resp.Error == nil {
		result := resp.Result.(map[string]interface{})
		content := result["content"].([]map[string]interface{})
		if len(content) != 1 || content[0]["type"] != "text" {
			t.Error("Unexpected content structure")
		}
	} else if resp.Error.Code != -32000 {
		t.Errorf("Expected -32000, got %d", resp.Error.Code)
	}
}

// ── Permission enforcement ──────────────────────────────────────────────────

func TestPermission_ReadDeniedForUnknownSheet(t *testing.T) {
	server := newTestServer() // empty config
	args, _ := json.Marshal(map[string]interface{}{"spreadsheet_id": "unknown-id"})
	_, err := server.toolReadSheet(args)
	if err == nil {
		t.Fatal("Expected permission error")
	}
	if !strings.Contains(err.Error(), "unknown-id") {
		t.Errorf("Expected spreadsheet ID in error: %v", err)
	}
}

func TestPermission_WriteDeniedForReadOnlySheet(t *testing.T) {
	server := newTestServerWithSheet(t, "read-only-id", config.AccessRead)
	args, _ := json.Marshal(map[string]interface{}{
		"spreadsheet_id": "read-only-id",
		"range":          "Sheet1!A1",
		"values":         [][]string{{"val"}},
	})
	_, err := server.toolWriteSheet(args)
	if err == nil {
		t.Fatal("Expected permission error")
	}
	if !strings.Contains(err.Error(), "readwrite") {
		t.Errorf("Expected 'readwrite' in error: %v", err)
	}
}

func TestPermission_ReadAllowedForConfiguredSheet(t *testing.T) {
	// A configured sheet should pass the config check. The test HTTP server
	// returns 403 to simulate a normal API error (NOT a config permission error).
	// NewClientWithBaseURL ensures requests go to the test server, not googleapis.com.
	srv := httpTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"code":403,"message":"forbidden","status":"PERMISSION_DENIED"}}`)
	})

	mcpServer := &MCPServer{
		ctx: context.Background(),
		cfg: &config.Config{
			Sheets: []config.SpreadsheetPermission{{ID: "allowed-id", Access: config.AccessRead}},
		},
		sheetsClient: sheets.NewClientWithBaseURL(srv.Client(), srv.URL),
	}
	args, _ := json.Marshal(map[string]interface{}{"spreadsheet_id": "allowed-id"})
	_, err := mcpServer.toolReadSheet(args)
	// The config check must have passed; error (if any) comes from the HTTP layer.
	if err != nil && strings.Contains(err.Error(), "not in the allowed sheets config") {
		t.Errorf("Should have passed config check: %v", err)
	}
}

func TestToolWriteSheet_EmptyValues(t *testing.T) {
	server := newTestServerWithSheet(t, "sheet-id", config.AccessReadWrite)
	args, _ := json.Marshal(map[string]interface{}{
		"spreadsheet_id": "sheet-id",
		"range":          "Sheet1!A1",
		"values":         nil,
	})
	_, err := server.toolWriteSheet(args)
	if err == nil {
		t.Fatal("Expected error for empty values")
	}
	if !strings.Contains(err.Error(), "values must not be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestToolAppendSheet_EmptyValues(t *testing.T) {
	server := newTestServerWithSheet(t, "sheet-id", config.AccessReadWrite)
	args, _ := json.Marshal(map[string]interface{}{
		"spreadsheet_id": "sheet-id",
		"range":          "Sheet1",
		"values":         nil,
	})
	_, err := server.toolAppendSheet(args)
	if err == nil {
		t.Fatal("Expected error for empty values")
	}
	if !strings.Contains(err.Error(), "values must not be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestHandleRequest_NotificationsInitialized_WithID_NoResponse(t *testing.T) {
	// Verify that a non-compliant client sending notifications/initialized with
	// a non-null id still does not produce a response (filtered in main loop by
	// method prefix, not just by nil id).
	server := newTestServer()
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: "notifications/initialized"})
	// The handler returns an empty struct; main() must suppress it.
	// At the handleRequest level we just confirm no error is set.
	if resp.Error != nil {
		t.Errorf("notifications/initialized should not produce an error: %v", resp.Error)
	}
}

func TestPermission_CreateRequiresWriteScope(t *testing.T) {
	server := newTestServer() // no sheets → no write scope
	args, _ := json.Marshal(map[string]interface{}{"title": "New Sheet"})
	_, err := server.toolCreateSpreadsheet(args)
	if err == nil {
		t.Fatal("Expected error when no write scope configured")
	}
	if !strings.Contains(err.Error(), "readwrite") {
		t.Errorf("Expected 'readwrite' in error: %v", err)
	}
}

// ── list_sheets tool ────────────────────────────────────────────────────────

func TestToolListSheets_EmptyConfig(t *testing.T) {
	server := newTestServer()
	result, err := server.toolListSheets()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["count"] != 0 {
		t.Errorf("Expected count 0, got %v", m["count"])
	}
}

func TestToolListSheets_WithSheets(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
		cfg: &config.Config{
			Sheets: []config.SpreadsheetPermission{
				{ID: "id1", Name: "Sheet One", Access: config.AccessRead},
				{ID: "id2", Access: config.AccessReadWrite},
			},
		},
	}
	result, err := server.toolListSheets()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["count"] != 2 {
		t.Errorf("Expected count 2, got %v", m["count"])
	}
	sheetList := m["sheets"].([]map[string]interface{})
	if sheetList[0]["id"] != "id1" || sheetList[0]["access"] != "read" {
		t.Errorf("Unexpected first sheet: %v", sheetList[0])
	}
	if _, hasName := sheetList[1]["name"]; hasName {
		t.Error("Sheet without name should not have name key")
	}
}

// ── Invalid JSON arguments ─────────────────────────────────────────────────

func TestToolReadSheet_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolReadSheet(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolWriteSheet_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolWriteSheet(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolAppendSheet_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolAppendSheet(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolCreateSpreadsheet_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolCreateSpreadsheet(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolGetSpreadsheetInfo_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolGetSpreadsheetInfo(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolAddSheet_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolAddSheet(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToolClearSheet_InvalidJSON(t *testing.T) {
	server := newTestServer()
	if _, err := server.toolClearSheet(json.RawMessage(`invalid`)); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// ── Constants ──────────────────────────────────────────────────────────────

func TestConstants(t *testing.T) {
	if serverName == "" {
		t.Error("serverName should not be empty")
	}
	if serverVersion == "" {
		t.Error("serverVersion should not be empty")
	}
	if serverName != "mcp-google-sheets" {
		t.Errorf("Expected 'mcp-google-sheets', got %q", serverName)
	}
	if serverVersion != "2.0.0" {
		t.Errorf("Expected '2.0.0', got %q", serverVersion)
	}
}

// ── MCPServer structure ────────────────────────────────────────────────────

func TestMCPServer_Structure(t *testing.T) {
	ctx := context.Background()
	server := &MCPServer{ctx: ctx, cfg: &config.Config{}}
	if server.ctx != ctx {
		t.Error("Server context not set correctly")
	}
	if server.sheetsClient != nil {
		t.Error("Expected nil sheetsClient")
	}
}

// ── NewMCPServer error handling ────────────────────────────────────────────

func TestNewMCPServer_ErrorHandling(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	_, err := NewMCPServer(context.Background())
	if err == nil {
		return // credentials found in environment — acceptable
	}
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// ── handleRequest all methods ──────────────────────────────────────────────

func TestHandleRequest_AllMethods(t *testing.T) {
	server := newTestServer()
	for _, method := range []string{"initialize", "tools/list", "tools/call", "ping"} {
		resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: method})
		if resp.JSONRPC != "2.0" {
			t.Errorf("Method %s: Expected '2.0', got %q", method, resp.JSONRPC)
		}
		if method != "tools/call" && resp.Error != nil {
			t.Errorf("Method %s: Unexpected error: %v", method, resp.Error)
		}
	}
}

// ── Input validation ───────────────────────────────────────────────────────

func TestInputValidation_MaliciousSpreadsheetID(t *testing.T) {
	server := newTestServer()
	malicious := []string{
		"../../../etc/passwd",
		"'; DROP TABLE spreadsheets; --",
		"<script>alert('xss')</script>",
		strings.Repeat("A", 10000),
		"\x00\x01\x02",
	}
	for _, input := range malicious {
		args, _ := json.Marshal(map[string]interface{}{"spreadsheet_id": input})
		_, _ = server.toolReadSheet(args) // must not panic
	}
}

func TestInputValidation_ExtremelyLargeData(t *testing.T) {
	server := newTestServer()
	large := make([][]string, 10000)
	for i := range large {
		large[i] = []string{"a", "b", "c"}
	}
	args, _ := json.Marshal(map[string]interface{}{"spreadsheet_id": "id", "range": "Sheet1", "values": large})
	_, _ = server.toolWriteSheet(args) // must not panic
}

func TestInputValidation_SpecialCharactersInSheetName(t *testing.T) {
	server := newTestServer()
	for _, name := range []string{"Sheet!@#", "Sheet\nNewlines", "Sheet'\"Quotes"} {
		args, _ := json.Marshal(map[string]interface{}{"spreadsheet_id": "id", "sheet_name": name})
		_, _ = server.toolAddSheet(args) // must not panic
	}
}

// ── Concurrency ────────────────────────────────────────────────────────────

func TestConcurrency_MultipleSimultaneousRequests(t *testing.T) {
	server := newTestServer()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: id, Method: "ping"})
			if resp.Error != nil {
				t.Errorf("Request %d failed: %v", id, resp.Error)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrency_InitializeRequests(t *testing.T) {
	server := newTestServer()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: id, Method: "initialize"})
			if resp.Error != nil {
				t.Errorf("Request %d failed: %v", id, resp.Error)
			}
			result := resp.Result.(map[string]interface{})
			if result["protocolVersion"] != "2024-11-05" {
				t.Errorf("Request %d: wrong protocolVersion", id)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrency_ToolsListRequests(t *testing.T) {
	server := newTestServer()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: id, Method: "tools/list"})
			if resp.Error != nil {
				t.Errorf("Request %d failed: %v", id, resp.Error)
			}
		}(i)
	}
	wg.Wait()
}

// ── Benchmarks ─────────────────────────────────────────────────────────────

func BenchmarkHandleRequest_Initialize(b *testing.B) {
	server := newTestServer()
	req := MCPRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.handleRequest(req)
	}
}

func BenchmarkHandleRequest_ToolsList(b *testing.B) {
	server := newTestServer()
	req := MCPRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.handleRequest(req)
	}
}

func BenchmarkJSONMarshalRequest(b *testing.B) {
	req := MCPRequest{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: json.RawMessage(`{}`)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(req); err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkJSONMarshalResponse(b *testing.B) {
	resp := MCPResponse{JSONRPC: "2.0", ID: 1, Result: map[string]interface{}{"status": "ok", "data": []string{"item1", "item2"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(resp); err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

// ── Examples ───────────────────────────────────────────────────────────────

func ExampleMCPServer_handleRequest() {
	server := &MCPServer{ctx: context.Background(), cfg: &config.Config{}}
	resp := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: "ping"})
	if resp.Error == nil {
		fmt.Println("ping successful")
	}
	// Output: ping successful
}

// ── Helpers ────────────────────────────────────────────────────────────────

func containsSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// httpTestServer starts a local HTTP test server and registers cleanup.
func httpTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}
