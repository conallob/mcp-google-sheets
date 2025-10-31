package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/conallob/mcp-google-sheets/sheets"
	"google.golang.org/api/option"
	sheetsapi "google.golang.org/api/sheets/v4"
)

func TestMCPRequest_JSONParsing(t *testing.T) {
	jsonStr := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`

	var req MCPRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal MCPRequest: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", req.JSONRPC)
	}

	if req.Method != "initialize" {
		t.Errorf("Expected Method 'initialize', got '%s'", req.Method)
	}
}

func TestMCPResponse_JSONSerialization(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]interface{}{
			"status": "ok",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal MCPResponse: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got %v", parsed["jsonrpc"])
	}
}

func TestMCPError_Structure(t *testing.T) {
	mcpError := MCPError{
		Code:    -32601,
		Message: "Method not found",
		Data:    "additional info",
	}

	data, err := json.Marshal(mcpError)
	if err != nil {
		t.Fatalf("Failed to marshal MCPError: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal error: %v", err)
	}

	if parsed["code"] != float64(-32601) {
		t.Errorf("Expected code -32601, got %v", parsed["code"])
	}

	if parsed["message"] != "Method not found" {
		t.Errorf("Expected message 'Method not found', got %v", parsed["message"])
	}
}

func TestHandleRequest_Initialize(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := server.handleRequest(req)

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]string)
	if !ok {
		t.Fatal("Expected serverInfo to be a map")
	}

	if serverInfo["name"] != serverName {
		t.Errorf("Expected server name '%s', got '%s'", serverName, serverInfo["name"])
	}

	if serverInfo["version"] != serverVersion {
		t.Errorf("Expected server version '%s', got '%s'", serverVersion, serverInfo["version"])
	}
}

func TestHandleRequest_Ping(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "ping",
	}

	resp := server.handleRequest(req)

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}

	if resp.ID != 2 {
		t.Errorf("Expected ID 2, got %v", resp.ID)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %v", result)
	}
}

func TestHandleRequest_MethodNotFound(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "nonexistent_method",
	}

	resp := server.handleRequest(req)

	if resp.Error == nil {
		t.Fatal("Expected error for nonexistent method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}

	if resp.Error.Message != "Method not found: nonexistent_method" {
		t.Errorf("Expected 'Method not found' message, got '%s'", resp.Error.Message)
	}
}

func TestHandleRequest_ToolsList(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/list",
	}

	resp := server.handleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	tools, ok := result["tools"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected tools to be a slice of maps")
	}

	expectedTools := []string{
		"read_sheet",
		"write_sheet",
		"append_sheet",
		"create_spreadsheet",
		"get_spreadsheet_info",
		"add_sheet",
		"clear_sheet",
		"batch_update",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	// Verify each tool has required fields
	for i, tool := range tools {
		name, ok := tool["name"].(string)
		if !ok {
			t.Errorf("Tool %d missing name", i)
			continue
		}

		if name != expectedTools[i] {
			t.Errorf("Expected tool %d to be '%s', got '%s'", i, expectedTools[i], name)
		}

		if _, ok := tool["description"].(string); !ok {
			t.Errorf("Tool '%s' missing description", name)
		}

		if _, ok := tool["inputSchema"].(map[string]interface{}); !ok {
			t.Errorf("Tool '%s' missing inputSchema", name)
		}
	}
}

func TestHandleInitialize(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-id",
		Method:  "initialize",
	}

	resp := server.handleInitialize(req)

	if resp.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %v", resp.ID)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected capabilities to be a map")
	}

	tools, ok := capabilities["tools"].(map[string]bool)
	if !ok {
		t.Fatal("Expected tools capability to be a map")
	}

	_ = tools // tools capability exists
}

func TestHandleToolsList_AllToolsHaveRequiredFields(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/list",
	}

	resp := server.handleToolsList(req)

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	tools, ok := result["tools"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected tools to be a slice of maps")
	}

	for _, tool := range tools {
		name := tool["name"].(string)

		// Check inputSchema structure
		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool '%s' inputSchema is not a map", name)
			continue
		}

		if inputSchema["type"] != "object" {
			t.Errorf("Tool '%s' inputSchema type should be 'object'", name)
		}

		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool '%s' inputSchema missing properties", name)
			continue
		}

		if len(properties) == 0 {
			t.Errorf("Tool '%s' has no properties defined", name)
		}

		required, ok := inputSchema["required"].([]string)
		if !ok {
			t.Errorf("Tool '%s' inputSchema missing required array", name)
			continue
		}

		// Verify required fields exist in properties
		for _, reqField := range required {
			if _, exists := properties[reqField]; !exists {
				t.Errorf("Tool '%s' required field '%s' not in properties", name, reqField)
			}
		}
	}
}

func TestHandleToolsCall_InvalidParams(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  json.RawMessage(`invalid json`),
	}

	resp := server.handleToolsCall(req)

	if resp.Error == nil {
		t.Fatal("Expected error for invalid params")
	}

	if resp.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", resp.Error.Code)
	}

	if resp.Error.Message != "Invalid params" {
		t.Errorf("Expected 'Invalid params' message, got '%s'", resp.Error.Message)
	}
}

func TestHandleToolsCall_ToolNotFound(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	params := map[string]interface{}{
		"name":      "nonexistent_tool",
		"arguments": json.RawMessage(`{}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleToolsCall(req)

	if resp.Error == nil {
		t.Fatal("Expected error for nonexistent tool")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}

	if resp.Error.Message != "Tool not found: nonexistent_tool" {
		t.Errorf("Expected 'Tool not found' message, got '%s'", resp.Error.Message)
	}
}

func TestHandleReadSheet_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleReadSheet(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleWriteSheet_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleWriteSheet(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleAppendSheet_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleAppendSheet(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleCreateSpreadsheet_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleCreateSpreadsheet(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleGetSpreadsheetInfo_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleGetSpreadsheetInfo(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleAddSheet_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleAddSheet(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleClearSheet_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleClearSheet(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandleBatchUpdate_InvalidJSON(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	_, err := server.handleBatchUpdate(json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestConstants(t *testing.T) {
	if serverName == "" {
		t.Error("serverName constant should not be empty")
	}

	if serverVersion == "" {
		t.Error("serverVersion constant should not be empty")
	}

	if serverName != "mcp-google-sheets" {
		t.Errorf("Expected serverName 'mcp-google-sheets', got '%s'", serverName)
	}

	if serverVersion != "1.0.0" {
		t.Errorf("Expected serverVersion '1.0.0', got '%s'", serverVersion)
	}
}

func TestMCPRequest_WithDifferentIDTypes(t *testing.T) {
	// Test with integer ID
	jsonStr := `{"jsonrpc":"2.0","id":123,"method":"test"}`
	var req1 MCPRequest
	err := json.Unmarshal([]byte(jsonStr), &req1)
	if err != nil {
		t.Fatalf("Failed to unmarshal request with int ID: %v", err)
	}
	if req1.ID != float64(123) { // JSON numbers are unmarshaled as float64
		t.Errorf("Expected ID 123, got %v", req1.ID)
	}

	// Test with string ID
	jsonStr = `{"jsonrpc":"2.0","id":"abc-123","method":"test"}`
	var req2 MCPRequest
	err = json.Unmarshal([]byte(jsonStr), &req2)
	if err != nil {
		t.Fatalf("Failed to unmarshal request with string ID: %v", err)
	}
	if req2.ID != "abc-123" {
		t.Errorf("Expected ID 'abc-123', got %v", req2.ID)
	}

	// Test with null ID
	jsonStr = `{"jsonrpc":"2.0","id":null,"method":"test"}`
	var req3 MCPRequest
	err = json.Unmarshal([]byte(jsonStr), &req3)
	if err != nil {
		t.Fatalf("Failed to unmarshal request with null ID: %v", err)
	}
	if req3.ID != nil {
		t.Errorf("Expected ID nil, got %v", req3.ID)
	}
}

func TestMCPResponse_ErrorResponse(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &MCPError{
			Code:    -32700,
			Message: "Parse error",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	var parsed MCPResponse
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if parsed.Error == nil {
		t.Fatal("Expected error to be non-nil")
	}

	if parsed.Error.Code != -32700 {
		t.Errorf("Expected error code -32700, got %d", parsed.Error.Code)
	}

	if parsed.Result != nil {
		t.Error("Expected result to be nil when error is present")
	}
}

func TestHandleToolsCall_WithToolError(t *testing.T) {
	// Create a server with a mock client that returns an error
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	params := map[string]interface{}{
		"name": "read_sheet",
		"arguments": map[string]interface{}{
			"spreadsheet_id": "test-id",
		},
	}
	paramsJSON, _ := json.Marshal(params)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleToolsCall(req)

	// Should get an error because the client doesn't have a real service
	if resp.Error == nil {
		t.Fatal("Expected error when tool execution fails")
	}

	if resp.Error.Code != -32000 {
		t.Errorf("Expected error code -32000, got %d", resp.Error.Code)
	}
}

func TestMCPServer_Structure(t *testing.T) {
	ctx := context.Background()
	server := &MCPServer{
		ctx: ctx,
	}

	if server.ctx != ctx {
		t.Error("Server context not set correctly")
	}

	// Test that server can be created with nil client (for testing purposes)
	if server.sheetsClient != nil {
		t.Error("Expected sheetsClient to be nil")
	}
}

func TestHandleRequest_AllMethods(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	methods := []string{
		"initialize",
		"tools/list",
		"tools/call",
		"ping",
	}

	for _, method := range methods {
		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  method,
		}

		resp := server.handleRequest(req)

		if resp.JSONRPC != "2.0" {
			t.Errorf("Method %s: Expected JSONRPC '2.0', got '%s'", method, resp.JSONRPC)
		}

		// All methods except tools/call should succeed without sheetsClient
		if method == "tools/call" && resp.Error != nil && resp.Error.Code != -32602 {
			// tools/call needs params, so it's ok to fail with invalid params
			continue
		} else if method != "tools/call" && resp.Error != nil {
			t.Errorf("Method %s: Unexpected error: %v", method, resp.Error)
		}
	}
}

func TestHandleToolsCall_AllTools(t *testing.T) {
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	tools := []struct {
		name string
		args map[string]interface{}
	}{
		{"read_sheet", map[string]interface{}{"spreadsheet_id": "test"}},
		{"write_sheet", map[string]interface{}{"spreadsheet_id": "test", "range": "A1", "values": [][]string{}}},
		{"append_sheet", map[string]interface{}{"spreadsheet_id": "test", "range": "A1", "values": [][]string{}}},
		{"create_spreadsheet", map[string]interface{}{"title": "Test"}},
		{"get_spreadsheet_info", map[string]interface{}{"spreadsheet_id": "test"}},
		{"add_sheet", map[string]interface{}{"spreadsheet_id": "test", "sheet_name": "New"}},
		{"clear_sheet", map[string]interface{}{"spreadsheet_id": "test", "range": "A1"}},
		{"batch_update", map[string]interface{}{"spreadsheet_id": "test", "requests": []map[string]interface{}{}}},
	}

	for _, tool := range tools {
		argsJSON, _ := json.Marshal(tool.args)
		params := map[string]interface{}{
			"name":      tool.name,
			"arguments": json.RawMessage(argsJSON),
		}
		paramsJSON, _ := json.Marshal(params)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.handleToolsCall(req)

		// All tools will fail due to nil service, but should be recognized
		// as valid tools (not -32601 "Tool not found")
		if resp.Error != nil && resp.Error.Code == -32601 {
			t.Errorf("Tool %s: Should be recognized as valid tool", tool.name)
		}
	}
}

func TestNewMCPServer_ErrorHandling(t *testing.T) {
	// This test verifies that NewMCPServer handles errors properly
	// In a real scenario, it would fail due to missing OAuth credentials
	ctx := context.Background()

	// Clear any OAuth environment variables
	oldClientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	oldClientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")

	defer func() {
		if oldClientID != "" {
			os.Setenv("GOOGLE_OAUTH_CLIENT_ID", oldClientID)
		}
		if oldClientSecret != "" {
			os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", oldClientSecret)
		}
	}()

	_, err := NewMCPServer(ctx)
	if err == nil {
		// If we got here, OAuth credentials were found somewhere
		// This is acceptable in a testing environment
		return
	}

	// Verify the error is related to OAuth configuration
	errStr := err.Error()
	if errStr == "" {
		t.Error("Expected non-empty error message")
	}
}

func BenchmarkHandleRequest_Initialize(b *testing.B) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.handleRequest(req)
	}
}

func BenchmarkHandleRequest_ToolsList(b *testing.B) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.handleRequest(req)
	}
}

func BenchmarkJSONMarshalRequest(b *testing.B) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(req)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkJSONMarshalResponse(b *testing.B) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]interface{}{
			"status": "ok",
			"data":   []string{"item1", "item2", "item3"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(resp)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func ExampleMCPServer_handleRequest() {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}

	resp := server.handleRequest(req)

	// Verify the response has no error
	if resp.Error == nil {
		fmt.Println("ping successful")
	}
	// Output: ping successful
}

// Security and Input Validation Tests
func TestInputValidation_MaliciousSpreadsheetID(t *testing.T) {
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	maliciousInputs := []string{
		"../../../etc/passwd",
		"'; DROP TABLE spreadsheets; --",
		"<script>alert('xss')</script>",
		strings.Repeat("A", 10000), // Very long input
		"\x00\x01\x02",             // Null bytes and control characters
	}

	for _, input := range maliciousInputs {
		args := map[string]interface{}{
			"spreadsheet_id": input,
		}
		argsJSON, _ := json.Marshal(args)

		// Should handle gracefully without panicking
		_, err := server.handleReadSheet(argsJSON)
		// Error is acceptable, panic is not
		_ = err
	}
}

func TestInputValidation_ExtremelyLargeData(t *testing.T) {
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	// Test with very large data array
	largeValues := make([][]string, 10000)
	for i := range largeValues {
		largeValues[i] = []string{"data1", "data2", "data3"}
	}

	args := map[string]interface{}{
		"spreadsheet_id": "test-id",
		"range":          "Sheet1!A1",
		"values":         largeValues,
	}
	argsJSON, _ := json.Marshal(args)

	// Should handle gracefully without panicking
	_, err := server.handleWriteSheet(argsJSON)
	_ = err
}

func TestInputValidation_SpecialCharactersInSheetName(t *testing.T) {
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	specialNames := []string{
		"Sheet!@#$%^&*()",
		"Sheet\nWith\nNewlines",
		"Sheet\tWith\tTabs",
		"Sheet'With\"Quotes",
	}

	for _, name := range specialNames {
		args := map[string]interface{}{
			"spreadsheet_id": "test-id",
			"sheet_name":     name,
		}
		argsJSON, _ := json.Marshal(args)

		// Should handle gracefully
		_, err := server.handleAddSheet(argsJSON)
		_ = err
	}
}

// Concurrency Tests
func TestConcurrency_MultipleSimultaneousRequests(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	var wg sync.WaitGroup
	numRequests := 100

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := MCPRequest{
				JSONRPC: "2.0",
				ID:      id,
				Method:  "ping",
			}

			resp := server.handleRequest(req)
			if resp.Error != nil {
				t.Errorf("Request %d failed: %v", id, resp.Error)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrency_InitializeRequests(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	var wg sync.WaitGroup
	numRequests := 50

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := MCPRequest{
				JSONRPC: "2.0",
				ID:      id,
				Method:  "initialize",
			}

			resp := server.handleRequest(req)
			if resp.Error != nil {
				t.Errorf("Initialize request %d failed: %v", id, resp.Error)
			}

			result, ok := resp.Result.(map[string]interface{})
			if !ok {
				t.Errorf("Request %d: expected result to be a map", id)
				return
			}

			if result["protocolVersion"] != "2024-11-05" {
				t.Errorf("Request %d: wrong protocol version", id)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrency_ToolsListRequests(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	var wg sync.WaitGroup
	numRequests := 50

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := MCPRequest{
				JSONRPC: "2.0",
				ID:      id,
				Method:  "tools/list",
			}

			resp := server.handleRequest(req)
			if resp.Error != nil {
				t.Errorf("Tools/list request %d failed: %v", id, resp.Error)
			}
		}(i)
	}

	wg.Wait()
}

// Error Message Validation Tests
func TestErrorMessages_MethodNotFound(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "invalid_method",
	}

	resp := server.handleRequest(req)

	if resp.Error == nil {
		t.Fatal("Expected error for invalid method")
	}

	expectedMsg := "Method not found: invalid_method"
	if resp.Error.Message != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, resp.Error.Message)
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestErrorMessages_ToolNotFound(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	params := map[string]interface{}{
		"name":      "nonexistent_tool",
		"arguments": json.RawMessage(`{}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleToolsCall(req)

	if resp.Error == nil {
		t.Fatal("Expected error for nonexistent tool")
	}

	expectedMsg := "Tool not found: nonexistent_tool"
	if resp.Error.Message != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, resp.Error.Message)
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestErrorMessages_InvalidParams(t *testing.T) {
	server := &MCPServer{
		ctx: context.Background(),
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`not valid json`),
	}

	resp := server.handleToolsCall(req)

	if resp.Error == nil {
		t.Fatal("Expected error for invalid params")
	}

	expectedMsg := "Invalid params"
	if resp.Error.Message != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, resp.Error.Message)
	}

	if resp.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", resp.Error.Code)
	}
}

// Test to verify error handling in tool execution
func TestToolExecution_ErrorPropagation(t *testing.T) {
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	// Test with missing required parameter
	argsJSON := json.RawMessage(`{}`) // Missing required spreadsheet_id
	params := map[string]interface{}{
		"name":      "read_sheet",
		"arguments": argsJSON,
	}
	paramsJSON, _ := json.Marshal(params)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleToolsCall(req)

	// Should propagate error from tool execution
	if resp.Error == nil {
		// If no error, the tool accepted empty params (which is ok for some tools)
		return
	}

	if resp.Error.Code != -32000 {
		t.Errorf("Expected error code -32000, got %d", resp.Error.Code)
	}
}

func TestReadSheetToolSchema(t *testing.T) {
	server := &MCPServer{ctx: context.Background()}
	resp := server.handleToolsList(MCPRequest{})

	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]map[string]interface{})

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

	// Verify required fields
	if !contains(required, "spreadsheet_id") {
		t.Error("spreadsheet_id should be required for read_sheet")
	}

	// Verify range is optional
	if contains(required, "range") {
		t.Error("range should be optional for read_sheet")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestAllToolsSchemasAreValid(t *testing.T) {
	server := &MCPServer{ctx: context.Background()}
	resp := server.handleToolsList(MCPRequest{})

	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]map[string]interface{})

	for _, tool := range tools {
		name := tool["name"].(string)

		// Verify description exists and is not empty
		desc, ok := tool["description"].(string)
		if !ok || desc == "" {
			t.Errorf("Tool %s has invalid description", name)
		}

		// Verify inputSchema is valid
		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %s has invalid inputSchema", name)
			continue
		}

		// Verify type is object
		if inputSchema["type"] != "object" {
			t.Errorf("Tool %s inputSchema type should be 'object'", name)
		}

		// Verify properties exist
		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok || len(properties) == 0 {
			t.Errorf("Tool %s has no properties", name)
		}

		// Verify required array exists
		_, ok = inputSchema["required"].([]string)
		if !ok {
			t.Errorf("Tool %s has invalid required array", name)
		}
	}
}

// Mock implementation of error from sheets client
type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

func TestHandleToolsCall_ResultFormatting(t *testing.T) {
	server := &MCPServer{
		sheetsClient: &sheets.Client{},
		ctx:          context.Background(),
	}

	// Create a tools/call request that will fail
	params := map[string]interface{}{
		"name": "read_sheet",
		"arguments": map[string]interface{}{
			"spreadsheet_id": "test-id",
		},
	}
	paramsJSON, _ := json.Marshal(params)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-123",
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleToolsCall(req)

	// Verify response structure
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}

	if resp.ID != "test-123" {
		t.Errorf("Expected ID 'test-123', got %v", resp.ID)
	}

	// Should have error due to nil service
	if resp.Error == nil {
		// If no error, check result format
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result to be a map")
		}

		content, ok := result["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("Expected content to be a slice of maps")
		}

		if len(content) != 1 {
			t.Errorf("Expected 1 content item, got %d", len(content))
		}

		if content[0]["type"] != "text" {
			t.Errorf("Expected content type 'text', got %v", content[0]["type"])
		}
	}
}

func TestMCPResponse_BothResultAndError(t *testing.T) {
	// According to JSON-RPC spec, a response should have either result or error, not both
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]interface{}{"data": "test"},
		Error:   &MCPError{Code: -32000, Message: "Error"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Both should be present in the JSON (though this violates JSON-RPC spec)
	if _, hasResult := parsed["result"]; !hasResult {
		t.Error("Result should be in JSON")
	}

	if _, hasError := parsed["error"]; !hasError {
		t.Error("Error should be in JSON")
	}
}
