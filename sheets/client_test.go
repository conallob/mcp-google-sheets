package sheets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// newTestClient creates a Sheets client backed by a local httptest server.
// The client's baseURL is pointed at the test server so all API calls are
// intercepted without network access.
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := NewClientWithBaseURL(server.Client(), server.URL)
	return client, server
}

func TestNewClient(t *testing.T) {
	client := NewClient(&http.Client{})
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.httpClient == nil {
		t.Error("Client http should not be nil")
	}
	if client.baseURL != productionBaseURL {
		t.Errorf("Expected baseURL %q, got %q", productionBaseURL, client.baseURL)
	}
}

func TestReadSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"range":  "Sheet1!A1:B2",
			"values": [][]interface{}{{"Name", "Age"}, {"John", "30"}},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.ReadSheet(context.Background(), "test-id", "Sheet1!A1:B2")
	if err != nil {
		t.Fatalf("ReadSheet failed: %v", err)
	}

	m := result.(map[string]interface{})
	if m["range"] != "Sheet1!A1:B2" {
		t.Errorf("Expected range 'Sheet1!A1:B2', got %v", m["range"])
	}
	values := m["values"].([][]string)
	if len(values) != 2 || values[0][0] != "Name" {
		t.Errorf("Unexpected values: %v", values)
	}
}

func TestReadSheet_EmptyRange(t *testing.T) {
	// When range is empty the URL should end at /values (no range path segment),
	// allowing the Sheets API to return the first sheet regardless of its name.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if containsStr(r.URL.Path, "Sheet1") {
			t.Errorf("Should not include 'Sheet1' in path when range is empty; got: %s", r.URL.Path)
		}
		if !containsStr(r.URL.Path, "/values") {
			t.Errorf("Path should end with /values, got: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"range":  "FirstTab!A1:A1",
			"values": [][]interface{}{{"Data"}},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.ReadSheet(context.Background(), "test-id", "")
	if err != nil {
		t.Fatalf("ReadSheet with empty range failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestReadSheet_NoData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"range":  "Sheet1!A1:A1",
			"values": [][]interface{}{},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.ReadSheet(context.Background(), "test-id", "Sheet1!A1:A1")
	if err != nil {
		t.Fatalf("ReadSheet with no data failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["message"] != "No data found" {
		t.Errorf("Expected 'No data found', got %v", m["message"])
	}
	if len(m["values"].([][]string)) != 0 {
		t.Error("Expected empty values")
	}
}

func TestReadSheet_MultipleRows(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"range":  "Sheet1!A1:C3",
			"values": [][]interface{}{{"Name", "Age", "City"}, {"Alice", "28", "NYC"}, {"Bob", "35", "LA"}},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.ReadSheet(context.Background(), "test-id", "Sheet1!A1:C3")
	if err != nil {
		t.Fatalf("ReadSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	values := m["values"].([][]string)
	if len(values) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(values))
	}
	if m["row_count"].(int) != 3 {
		t.Errorf("Expected row_count 3, got %v", m["row_count"])
	}
	if m["col_count"].(int) != 3 {
		t.Errorf("Expected col_count 3, got %v", m["col_count"])
	}
}

func TestReadSheet_TypeConversion(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// JSON numbers / booleans come back as float64 / bool after json.Unmarshal.
		json.NewEncoder(w).Encode(map[string]interface{}{
			"range":  "Sheet1!A1:D2",
			"values": [][]interface{}{{"String", 123, 45.67, true}},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.ReadSheet(context.Background(), "test-id", "Sheet1!A1:D2")
	if err != nil {
		t.Fatalf("ReadSheet failed: %v", err)
	}
	values := result.(map[string]interface{})["values"].([][]string)
	if values[0][0] != "String" {
		t.Errorf("Expected 'String', got %q", values[0][0])
	}
	if values[0][1] != "123" {
		t.Errorf("Expected '123', got %q", values[0][1])
	}
}

func TestWriteSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updatedRange":   "Sheet1!A1:B2",
			"updatedRows":    2,
			"updatedColumns": 2,
			"updatedCells":   4,
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.WriteSheet(context.Background(), "test-id", "Sheet1!A1:B2", [][]string{{"Name", "Age"}, {"Jane", "25"}})
	if err != nil {
		t.Fatalf("WriteSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["updated_range"] != "Sheet1!A1:B2" {
		t.Errorf("Expected updated_range 'Sheet1!A1:B2', got %v", m["updated_range"])
	}
	if m["updated_rows"] != int64(2) {
		t.Errorf("Expected updated_rows 2, got %v", m["updated_rows"])
	}
}

func TestWriteSheet_EmptyValues(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updatedRange":   "Sheet1!A1:A1",
			"updatedRows":    0,
			"updatedColumns": 0,
			"updatedCells":   0,
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.WriteSheet(context.Background(), "test-id", "Sheet1!A1:A1", [][]string{})
	if err != nil {
		t.Fatalf("WriteSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["updated_cells"] != int64(0) {
		t.Errorf("Expected updated_cells 0, got %v", m["updated_cells"])
	}
}

func TestWriteSheet_ValueConversion(t *testing.T) {
	var received map[string]interface{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updatedRange": "Sheet1!A1:B1", "updatedRows": 1, "updatedColumns": 2, "updatedCells": 2,
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	_, err := client.WriteSheet(context.Background(), "test-id", "Sheet1!A1:B1", [][]string{{"a", "b"}})
	if err != nil {
		t.Fatalf("WriteSheet failed: %v", err)
	}
	vals := received["values"].([]interface{})
	if len(vals) != 1 || len(vals[0].([]interface{})) != 2 {
		t.Errorf("Unexpected received values: %v", received["values"])
	}
}

func TestAppendSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updates": map[string]interface{}{
				"updatedRange": "Sheet1!A3:B3", "updatedRows": 1, "updatedColumns": 2, "updatedCells": 2,
			},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.AppendSheet(context.Background(), "test-id", "Sheet1!A:B", [][]string{{"Bob", "35"}})
	if err != nil {
		t.Fatalf("AppendSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["updated_range"] != "Sheet1!A3:B3" {
		t.Errorf("Expected updated_range 'Sheet1!A3:B3', got %v", m["updated_range"])
	}
	if m["message"] != "Data appended successfully" {
		t.Errorf("Unexpected message: %v", m["message"])
	}
}

func TestAppendSheet_MultipleRows(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updates": map[string]interface{}{
				"updatedRange": "Sheet1!A10:C12", "updatedRows": 3, "updatedColumns": 3, "updatedCells": 9,
			},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	values := [][]string{{"Alice", "28", "NYC"}, {"Bob", "35", "LA"}, {"Carol", "42", "SF"}}
	result, err := client.AppendSheet(context.Background(), "test-id", "Sheet1!A:C", values)
	if err != nil {
		t.Fatalf("AppendSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["updated_rows"] != int64(3) {
		t.Errorf("Expected updated_rows 3, got %v", m["updated_rows"])
	}
	if m["updated_cells"] != int64(9) {
		t.Errorf("Expected updated_cells 9, got %v", m["updated_cells"])
	}
}

func TestClearSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId": "test-id",
			"clearedRange":  "Sheet1!A1:B10",
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.ClearSheet(context.Background(), "test-id", "Sheet1!A1:B10")
	if err != nil {
		t.Fatalf("ClearSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["cleared_range"] != "Sheet1!A1:B10" {
		t.Errorf("Expected cleared_range 'Sheet1!A1:B10', got %v", m["cleared_range"])
	}
	if m["message"] != "Range cleared successfully" {
		t.Errorf("Unexpected message: %v", m["message"])
	}
}

func TestGetSpreadsheetInfo_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId":  "test-id",
			"spreadsheetUrl": "https://docs.google.com/spreadsheets/d/test-id",
			"properties": map[string]interface{}{
				"title":    "Test Spreadsheet",
				"locale":   "en_US",
				"timeZone": "America/New_York",
			},
			"sheets": []interface{}{
				map[string]interface{}{
					"properties": map[string]interface{}{
						"sheetId":   0,
						"title":     "Sheet1",
						"index":     0,
						"sheetType": "GRID",
						"gridProperties": map[string]interface{}{
							"rowCount": 100, "columnCount": 26,
							"frozenRowCount": 1, "frozenColumnCount": 0,
						},
					},
				},
			},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.GetSpreadsheetInfo(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("GetSpreadsheetInfo failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["spreadsheet_id"] != "test-id" {
		t.Errorf("Expected spreadsheet_id 'test-id', got %v", m["spreadsheet_id"])
	}
	if m["title"] != "Test Spreadsheet" {
		t.Errorf("Expected title 'Test Spreadsheet', got %v", m["title"])
	}
	sheetList := m["sheets"].([]map[string]interface{})
	if len(sheetList) != 1 || sheetList[0]["title"] != "Sheet1" {
		t.Errorf("Unexpected sheets: %v", sheetList)
	}
}

func TestCreateSpreadsheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId":  "new-id",
			"spreadsheetUrl": "https://docs.google.com/spreadsheets/d/new-id",
			"properties":     map[string]interface{}{"title": "Test Spreadsheet"},
			"sheets": []interface{}{
				map[string]interface{}{"properties": map[string]interface{}{"title": "Sheet1"}},
			},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.CreateSpreadsheet(context.Background(), "Test Spreadsheet", []string{"Sheet1"})
	if err != nil {
		t.Fatalf("CreateSpreadsheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["spreadsheet_id"] != "new-id" {
		t.Errorf("Expected spreadsheet_id 'new-id', got %v", m["spreadsheet_id"])
	}
	sheets := m["sheets"].([]string)
	if len(sheets) != 1 || sheets[0] != "Sheet1" {
		t.Errorf("Expected sheets [Sheet1], got %v", sheets)
	}
}

func TestCreateSpreadsheet_NoSheetNames(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId":  "new-id",
			"spreadsheetUrl": "https://docs.google.com/spreadsheets/d/new-id",
			"properties":     map[string]interface{}{"title": "Test Spreadsheet"},
			"sheets":         []interface{}{},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.CreateSpreadsheet(context.Background(), "Test Spreadsheet", nil)
	if err != nil {
		t.Fatalf("CreateSpreadsheet failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestCreateSpreadsheet_MultipleSheets(t *testing.T) {
	sheetNames := []string{"Data", "Analysis", "Summary"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		sheets := make([]interface{}, len(sheetNames))
		for i, name := range sheetNames {
			sheets[i] = map[string]interface{}{"properties": map[string]interface{}{"title": name}}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId":  "multi-id",
			"spreadsheetUrl": "https://docs.google.com/spreadsheets/d/multi-id",
			"properties":     map[string]interface{}{"title": "Multi"},
			"sheets":         sheets,
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.CreateSpreadsheet(context.Background(), "Multi", sheetNames)
	if err != nil {
		t.Fatalf("CreateSpreadsheet failed: %v", err)
	}
	got := result.(map[string]interface{})["sheets"].([]string)
	if !reflect.DeepEqual(got, sheetNames) {
		t.Errorf("Expected %v, got %v", sheetNames, got)
	}
}

func TestAddSheet_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId": "test-id",
			"replies": []interface{}{
				map[string]interface{}{
					"addSheet": map[string]interface{}{
						"properties": map[string]interface{}{
							"sheetId": 123, "title": "NewSheet", "index": 1,
						},
					},
				},
			},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.AddSheet(context.Background(), "test-id", "NewSheet")
	if err != nil {
		t.Fatalf("AddSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["title"] != "NewSheet" {
		t.Errorf("Expected title 'NewSheet', got %v", m["title"])
	}
}

func TestAddSheet_NoReply(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"spreadsheetId": "test-id",
			"replies":       []interface{}{},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	result, err := client.AddSheet(context.Background(), "test-id", "NewSheet")
	if err != nil {
		t.Fatalf("AddSheet failed: %v", err)
	}
	m := result.(map[string]interface{})
	if m["message"] != "Sheet added successfully" {
		t.Errorf("Expected success message, got %v", m["message"])
	}
}

func TestClient_NilHTTPClient(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Error("NewClient should not return nil with nil http client")
	}
	if client.httpClient != nil {
		t.Error("Expected nil http client when initialized with nil")
	}
}

func TestAPIError_SurfacedCorrectly(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    403,
				"message": "The caller does not have permission",
				"status":  "PERMISSION_DENIED",
			},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	_, err := client.ReadSheet(context.Background(), "test-id", "Sheet1")
	if err == nil {
		t.Fatal("Expected error for 403 response")
	}
	if !containsStr(err.Error(), "PERMISSION_DENIED") && !containsStr(err.Error(), "403") {
		t.Errorf("Expected permission error, got: %v", err)
	}
}

// Helper

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || searchStr(s, substr))
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmarks

func BenchmarkReadSheet(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"range":  "Sheet1!A1:B2",
			"values": [][]interface{}{{"Name", "Age"}, {"John", "30"}},
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ReadSheet(context.Background(), "test-id", "Sheet1!A1:B2")
		if err != nil {
			b.Fatalf("ReadSheet failed: %v", err)
		}
	}
}

func BenchmarkWriteSheet(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"updatedRange": "Sheet1!A1:B2", "updatedRows": 2, "updatedColumns": 2, "updatedCells": 4,
		})
	})
	client, server := newTestClient(handler)
	defer server.Close()

	values := [][]string{{"Name", "Age"}, {"Jane", "25"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.WriteSheet(context.Background(), "test-id", "Sheet1!A1:B2", values)
		if err != nil {
			b.Fatalf("WriteSheet failed: %v", err)
		}
	}
}

// Examples

func ExampleNewClient() {
	client := NewClient(&http.Client{})
	if client != nil {
		fmt.Println("Client created successfully")
	}
	// Output: Client created successfully
}

func ExampleClient_ReadSheet() {
	fmt.Println("ReadSheet requires a spreadsheet ID and range")
	// Output: ReadSheet requires a spreadsheet ID and range
}
