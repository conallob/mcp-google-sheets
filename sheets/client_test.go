package sheets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// mockSheetsService creates a mock Google Sheets service for testing
func mockSheetsService(t *testing.T, handler http.HandlerFunc) (*sheets.Service, *httptest.Server) {
	server := httptest.NewServer(handler)
	service, err := sheets.NewService(context.Background(), option.WithHTTPClient(server.Client()), option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatalf("Failed to create mock sheets service: %v", err)
		server.Close()
		return nil, nil
	}
	return service, server
}

func TestNewClient(t *testing.T) {
	service, server := mockSheetsService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client := NewClient(service)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.service == nil {
		t.Error("Client service should not be nil")
	}
}

func TestReadSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.ValueRange{
			Range: "Sheet1!A1:B2",
			Values: [][]interface{}{
				{"Name", "Age"},
				{"John", "30"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.ReadSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:B2")
	if err != nil {
		t.Fatalf("ReadSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["range"] != "Sheet1!A1:B2" {
		t.Errorf("Expected range 'Sheet1!A1:B2', got %v", resultMap["range"])
	}

	values, ok := resultMap["values"].([][]string)
	if !ok {
		t.Fatal("Expected values to be [][]string")
	}

	if len(values) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(values))
	}

	if values[0][0] != "Name" {
		t.Errorf("Expected first cell to be 'Name', got '%s'", values[0][0])
	}
}

func TestReadSheet_EmptyRange(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that when no range is provided, it defaults to Sheet1
		if !contains(r.URL.Path, "Sheet1") {
			t.Errorf("Expected default range 'Sheet1', got path: %s", r.URL.Path)
		}
		response := &sheets.ValueRange{
			Range:  "Sheet1!A1:A1",
			Values: [][]interface{}{{"Data"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.ReadSheet(ctx, "test-spreadsheet-id", "")
	if err != nil {
		t.Fatalf("ReadSheet with empty range failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result to be non-nil")
	}
}

func TestReadSheet_NoData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.ValueRange{
			Range:  "Sheet1!A1:A1",
			Values: [][]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.ReadSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:A1")
	if err != nil {
		t.Fatalf("ReadSheet with no data failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	message, ok := resultMap["message"].(string)
	if !ok || message != "No data found" {
		t.Errorf("Expected 'No data found' message, got %v", resultMap["message"])
	}

	values, ok := resultMap["values"].([][]string)
	if !ok {
		t.Fatal("Expected values to be [][]string")
	}

	if len(values) != 0 {
		t.Errorf("Expected empty values array, got %d rows", len(values))
	}
}

func TestWriteSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT request, got %s", r.Method)
		}

		response := &sheets.UpdateValuesResponse{
			UpdatedRange:   "Sheet1!A1:B2",
			UpdatedRows:    2,
			UpdatedColumns: 2,
			UpdatedCells:   4,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	values := [][]string{
		{"Name", "Age"},
		{"Jane", "25"},
	}

	result, err := client.WriteSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:B2", values)
	if err != nil {
		t.Fatalf("WriteSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["updated_range"] != "Sheet1!A1:B2" {
		t.Errorf("Expected updated_range 'Sheet1!A1:B2', got %v", resultMap["updated_range"])
	}

	if resultMap["updated_rows"] != int64(2) {
		t.Errorf("Expected updated_rows 2, got %v", resultMap["updated_rows"])
	}
}

func TestAppendSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		response := &sheets.AppendValuesResponse{
			Updates: &sheets.UpdateValuesResponse{
				UpdatedRange:   "Sheet1!A3:B3",
				UpdatedRows:    1,
				UpdatedColumns: 2,
				UpdatedCells:   2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	values := [][]string{
		{"Bob", "35"},
	}

	result, err := client.AppendSheet(ctx, "test-spreadsheet-id", "Sheet1!A:B", values)
	if err != nil {
		t.Fatalf("AppendSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["updated_range"] != "Sheet1!A3:B3" {
		t.Errorf("Expected updated_range 'Sheet1!A3:B3', got %v", resultMap["updated_range"])
	}

	message, ok := resultMap["message"].(string)
	if !ok || message != "Data appended successfully" {
		t.Errorf("Expected success message, got %v", resultMap["message"])
	}
}

func TestCreateSpreadsheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		response := &sheets.Spreadsheet{
			SpreadsheetId:  "new-spreadsheet-id",
			SpreadsheetUrl: "https://docs.google.com/spreadsheets/d/new-spreadsheet-id",
			Properties: &sheets.SpreadsheetProperties{
				Title: "Test Spreadsheet",
			},
			Sheets: []*sheets.Sheet{
				{
					Properties: &sheets.SheetProperties{
						Title: "Sheet1",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.CreateSpreadsheet(ctx, "Test Spreadsheet", []string{"Sheet1"})
	if err != nil {
		t.Fatalf("CreateSpreadsheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["spreadsheet_id"] != "new-spreadsheet-id" {
		t.Errorf("Expected spreadsheet_id 'new-spreadsheet-id', got %v", resultMap["spreadsheet_id"])
	}

	if resultMap["title"] != "Test Spreadsheet" {
		t.Errorf("Expected title 'Test Spreadsheet', got %v", resultMap["title"])
	}

	sheets, ok := resultMap["sheets"].([]string)
	if !ok {
		t.Fatal("Expected sheets to be []string")
	}

	if len(sheets) != 1 || sheets[0] != "Sheet1" {
		t.Errorf("Expected sheets to contain 'Sheet1', got %v", sheets)
	}
}

func TestCreateSpreadsheet_NoSheetNames(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.Spreadsheet{
			SpreadsheetId:  "new-spreadsheet-id",
			SpreadsheetUrl: "https://docs.google.com/spreadsheets/d/new-spreadsheet-id",
			Properties: &sheets.SpreadsheetProperties{
				Title: "Test Spreadsheet",
			},
			Sheets: []*sheets.Sheet{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.CreateSpreadsheet(ctx, "Test Spreadsheet", nil)
	if err != nil {
		t.Fatalf("CreateSpreadsheet with no sheet names failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result to be non-nil")
	}
}

func TestGetSpreadsheetInfo_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		response := &sheets.Spreadsheet{
			SpreadsheetId:  "test-spreadsheet-id",
			SpreadsheetUrl: "https://docs.google.com/spreadsheets/d/test-spreadsheet-id",
			Properties: &sheets.SpreadsheetProperties{
				Title:    "Test Spreadsheet",
				Locale:   "en_US",
				TimeZone: "America/New_York",
			},
			Sheets: []*sheets.Sheet{
				{
					Properties: &sheets.SheetProperties{
						SheetId:   0,
						Title:     "Sheet1",
						Index:     0,
						SheetType: "GRID",
						GridProperties: &sheets.GridProperties{
							RowCount:         100,
							ColumnCount:      26,
							FrozenRowCount:   1,
							FrozenColumnCount: 0,
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.GetSpreadsheetInfo(ctx, "test-spreadsheet-id")
	if err != nil {
		t.Fatalf("GetSpreadsheetInfo failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["spreadsheet_id"] != "test-spreadsheet-id" {
		t.Errorf("Expected spreadsheet_id 'test-spreadsheet-id', got %v", resultMap["spreadsheet_id"])
	}

	if resultMap["title"] != "Test Spreadsheet" {
		t.Errorf("Expected title 'Test Spreadsheet', got %v", resultMap["title"])
	}

	sheets, ok := resultMap["sheets"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected sheets to be []map[string]interface{}")
	}

	if len(sheets) != 1 {
		t.Errorf("Expected 1 sheet, got %d", len(sheets))
	}

	if sheets[0]["title"] != "Sheet1" {
		t.Errorf("Expected sheet title 'Sheet1', got %v", sheets[0]["title"])
	}
}

func TestAddSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		response := &sheets.BatchUpdateSpreadsheetResponse{
			Replies: []*sheets.Response{
				{
					AddSheet: &sheets.AddSheetResponse{
						Properties: &sheets.SheetProperties{
							SheetId: 123,
							Title:   "NewSheet",
							Index:   1,
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.AddSheet(ctx, "test-spreadsheet-id", "NewSheet")
	if err != nil {
		t.Fatalf("AddSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["title"] != "NewSheet" {
		t.Errorf("Expected title 'NewSheet', got %v", resultMap["title"])
	}

	if resultMap["sheet_id"] != int64(123) {
		t.Errorf("Expected sheet_id 123, got %v", resultMap["sheet_id"])
	}
}

func TestAddSheet_NoReply(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.BatchUpdateSpreadsheetResponse{
			Replies: []*sheets.Response{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.AddSheet(ctx, "test-spreadsheet-id", "NewSheet")
	if err != nil {
		t.Fatalf("AddSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	message, ok := resultMap["message"].(string)
	if !ok || message != "Sheet added successfully" {
		t.Errorf("Expected success message, got %v", resultMap["message"])
	}
}

func TestClearSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		response := &sheets.ClearValuesResponse{
			ClearedRange:   "Sheet1!A1:B10",
			SpreadsheetId:  "test-spreadsheet-id",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.ClearSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:B10")
	if err != nil {
		t.Fatalf("ClearSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["cleared_range"] != "Sheet1!A1:B10" {
		t.Errorf("Expected cleared_range 'Sheet1!A1:B10', got %v", resultMap["cleared_range"])
	}

	message, ok := resultMap["message"].(string)
	if !ok || message != "Range cleared successfully" {
		t.Errorf("Expected success message, got %v", resultMap["message"])
	}
}

func TestBatchUpdate_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		response := &sheets.BatchUpdateSpreadsheetResponse{
			SpreadsheetId: "test-spreadsheet-id",
			Replies: []*sheets.Response{
				{},
				{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	requests := []map[string]interface{}{
		{
			"updateCells": map[string]interface{}{
				"range": map[string]interface{}{
					"sheetId": 0,
				},
			},
		},
	}

	result, err := client.BatchUpdate(ctx, "test-spreadsheet-id", requests)
	if err != nil {
		t.Fatalf("BatchUpdate failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["spreadsheet_id"] != "test-spreadsheet-id" {
		t.Errorf("Expected spreadsheet_id 'test-spreadsheet-id', got %v", resultMap["spreadsheet_id"])
	}

	if resultMap["replies_count"] != 2 {
		t.Errorf("Expected replies_count 2, got %v", resultMap["replies_count"])
	}
}

func TestBatchUpdate_InvalidJSON(t *testing.T) {
	service, server := mockSheetsService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	// Create requests that will fail JSON marshaling by including unsupported types
	requests := []map[string]interface{}{
		{
			"invalid": make(chan int), // channels can't be marshaled to JSON
		},
	}

	_, err := client.BatchUpdate(ctx, "test-spreadsheet-id", requests)
	if err == nil {
		t.Error("Expected error when marshaling invalid requests")
	}
}

func TestReadSheet_MultipleRows(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.ValueRange{
			Range: "Sheet1!A1:C3",
			Values: [][]interface{}{
				{"Name", "Age", "City"},
				{"Alice", "28", "NYC"},
				{"Bob", "35", "LA"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.ReadSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:C3")
	if err != nil {
		t.Fatalf("ReadSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	values, ok := resultMap["values"].([][]string)
	if !ok {
		t.Fatal("Expected values to be [][]string")
	}

	if len(values) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(values))
	}

	if len(values[0]) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(values[0]))
	}

	rowCount, ok := resultMap["row_count"].(int)
	if !ok || rowCount != 3 {
		t.Errorf("Expected row_count 3, got %v", resultMap["row_count"])
	}

	colCount, ok := resultMap["col_count"].(int)
	if !ok || colCount != 3 {
		t.Errorf("Expected col_count 3, got %v", resultMap["col_count"])
	}
}

func TestWriteSheet_EmptyValues(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.UpdateValuesResponse{
			UpdatedRange:   "Sheet1!A1:A1",
			UpdatedRows:    0,
			UpdatedColumns: 0,
			UpdatedCells:   0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	values := [][]string{}

	result, err := client.WriteSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:A1", values)
	if err != nil {
		t.Fatalf("WriteSheet with empty values failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["updated_cells"] != int64(0) {
		t.Errorf("Expected updated_cells 0, got %v", resultMap["updated_cells"])
	}
}

func TestClient_NilService(t *testing.T) {
	// Test that NewClient handles nil service gracefully
	client := NewClient(nil)
	if client == nil {
		t.Error("NewClient should not return nil even with nil service")
	}

	if client.service != nil {
		t.Error("Client service should be nil when initialized with nil")
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkReadSheet(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.ValueRange{
			Range: "Sheet1!A1:B2",
			Values: [][]interface{}{
				{"Name", "Age"},
				{"John", "30"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(b, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ReadSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:B2")
		if err != nil {
			b.Fatalf("ReadSheet failed: %v", err)
		}
	}
}

func BenchmarkWriteSheet(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.UpdateValuesResponse{
			UpdatedRange:   "Sheet1!A1:B2",
			UpdatedRows:    2,
			UpdatedColumns: 2,
			UpdatedCells:   4,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(b, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	values := [][]string{
		{"Name", "Age"},
		{"Jane", "25"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.WriteSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:B2", values)
		if err != nil {
			b.Fatalf("WriteSheet failed: %v", err)
		}
	}
}

func TestReadSheet_TypeConversion(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.ValueRange{
			Range: "Sheet1!A1:D2",
			Values: [][]interface{}{
				{"String", 123, 45.67, true},
				{"Another", 456, 78.90, false},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	result, err := client.ReadSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:D2")
	if err != nil {
		t.Fatalf("ReadSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	values, ok := resultMap["values"].([][]string)
	if !ok {
		t.Fatal("Expected values to be [][]string")
	}

	// Verify type conversion to strings
	if values[0][0] != "String" {
		t.Errorf("Expected 'String', got '%s'", values[0][0])
	}

	if values[0][1] != "123" {
		t.Errorf("Expected '123', got '%s'", values[0][1])
	}

	if values[0][2] != "45.67" {
		t.Errorf("Expected '45.67', got '%s'", values[0][2])
	}

	if values[0][3] != "true" {
		t.Errorf("Expected 'true', got '%s'", values[0][3])
	}
}

func TestWriteSheet_ValueConversion(t *testing.T) {
	var receivedValues [][]interface{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Values [][]interface{} `json:"values"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		receivedValues = body.Values

		response := &sheets.UpdateValuesResponse{
			UpdatedRange:   "Sheet1!A1:B1",
			UpdatedRows:    1,
			UpdatedColumns: 2,
			UpdatedCells:   2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	values := [][]string{
		{"test1", "test2"},
	}

	_, err := client.WriteSheet(ctx, "test-spreadsheet-id", "Sheet1!A1:B1", values)
	if err != nil {
		t.Fatalf("WriteSheet failed: %v", err)
	}

	if len(receivedValues) != 1 || len(receivedValues[0]) != 2 {
		t.Errorf("Expected 1 row with 2 columns, got %d rows", len(receivedValues))
	}
}

func TestAppendSheet_MultipleRows(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.AppendValuesResponse{
			Updates: &sheets.UpdateValuesResponse{
				UpdatedRange:   "Sheet1!A10:C12",
				UpdatedRows:    3,
				UpdatedColumns: 3,
				UpdatedCells:   9,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	values := [][]string{
		{"Alice", "28", "NYC"},
		{"Bob", "35", "LA"},
		{"Carol", "42", "SF"},
	}

	result, err := client.AppendSheet(ctx, "test-spreadsheet-id", "Sheet1!A:C", values)
	if err != nil {
		t.Fatalf("AppendSheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["updated_rows"] != int64(3) {
		t.Errorf("Expected updated_rows 3, got %v", resultMap["updated_rows"])
	}

	if resultMap["updated_cells"] != int64(9) {
		t.Errorf("Expected updated_cells 9, got %v", resultMap["updated_cells"])
	}
}

func TestCreateSpreadsheet_MultipleSheets(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &sheets.Spreadsheet{
			SpreadsheetId:  "multi-sheet-id",
			SpreadsheetUrl: "https://docs.google.com/spreadsheets/d/multi-sheet-id",
			Properties: &sheets.SpreadsheetProperties{
				Title: "Multi Sheet Spreadsheet",
			},
			Sheets: []*sheets.Sheet{
				{Properties: &sheets.SheetProperties{Title: "Data"}},
				{Properties: &sheets.SheetProperties{Title: "Analysis"}},
				{Properties: &sheets.SheetProperties{Title: "Summary"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	service, server := mockSheetsService(t, handler)
	defer server.Close()

	client := NewClient(service)
	ctx := context.Background()

	sheetNames := []string{"Data", "Analysis", "Summary"}
	result, err := client.CreateSpreadsheet(ctx, "Multi Sheet Spreadsheet", sheetNames)
	if err != nil {
		t.Fatalf("CreateSpreadsheet failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	sheets, ok := resultMap["sheets"].([]string)
	if !ok {
		t.Fatal("Expected sheets to be []string")
	}

	if !reflect.DeepEqual(sheets, sheetNames) {
		t.Errorf("Expected sheets %v, got %v", sheetNames, sheets)
	}
}

func ExampleNewClient() {
	// This example shows how to create a new Sheets client
	// In a real scenario, you would get the service from OAuth authentication
	service, _ := sheets.NewService(context.Background())
	client := NewClient(service)
	_ = client // Use the client for operations
}

func ExampleClient_ReadSheet() {
	// This example shows how to read data from a sheet
	service, _ := sheets.NewService(context.Background())
	client := NewClient(service)

	ctx := context.Background()
	result, err := client.ReadSheet(ctx, "spreadsheet-id", "Sheet1!A1:B2")
	if err != nil {
		// Handle error
		return
	}
	_ = result // Process the result
}
