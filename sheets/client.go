package sheets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const productionBaseURL = "https://sheets.googleapis.com/v4/spreadsheets"

// maxResponseBytes caps how much of a single API response we read into memory.
// Spreadsheets with millions of cells can produce very large responses; this
// prevents a malformed or unexpectedly large response from exhausting RAM.
const maxResponseBytes = 50 * 1024 * 1024 // 50 MiB

// Client wraps an authenticated HTTP client for the Google Sheets REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string // defaults to productionBaseURL; overridable for tests
}

// NewClient creates a new Sheets client from an OAuth2-authenticated HTTP client.
func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient, baseURL: productionBaseURL}
}

// NewClientWithBaseURL creates a new Sheets client with a custom base URL.
// Intended for tests that need to point the client at an httptest.Server.
func NewClientWithBaseURL(httpClient *http.Client, baseURL string) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// ── API types ─────────────────────────────────────────────────────────────────

type valueRange struct {
	Range          string          `json:"range,omitempty"`
	MajorDimension string          `json:"majorDimension,omitempty"`
	Values         [][]interface{} `json:"values,omitempty"`
}

type spreadsheetProperties struct {
	Title    string `json:"title,omitempty"`
	Locale   string `json:"locale,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type gridProperties struct {
	RowCount          int64 `json:"rowCount,omitempty"`
	ColumnCount       int64 `json:"columnCount,omitempty"`
	FrozenRowCount    int64 `json:"frozenRowCount,omitempty"`
	FrozenColumnCount int64 `json:"frozenColumnCount,omitempty"`
}

type sheetProperties struct {
	SheetID        int64          `json:"sheetId,omitempty"`
	Title          string         `json:"title,omitempty"`
	Index          int64          `json:"index,omitempty"`
	SheetType      string         `json:"sheetType,omitempty"`
	GridProperties gridProperties `json:"gridProperties,omitempty"`
}

type sheet struct {
	Properties sheetProperties `json:"properties"`
}

type spreadsheet struct {
	SpreadsheetId  string                `json:"spreadsheetId"`
	SpreadsheetUrl string                `json:"spreadsheetUrl"`
	Properties     spreadsheetProperties `json:"properties"`
	Sheets         []sheet               `json:"sheets"`
}

type updateValuesResponse struct {
	SpreadsheetId  string `json:"spreadsheetId"`
	UpdatedRange   string `json:"updatedRange"`
	UpdatedRows    int64  `json:"updatedRows"`
	UpdatedColumns int64  `json:"updatedColumns"`
	UpdatedCells   int64  `json:"updatedCells"`
}

type appendValuesResponse struct {
	SpreadsheetId string `json:"spreadsheetId"`
	TableRange    string `json:"tableRange"`
	Updates       struct {
		UpdatedRange   string `json:"updatedRange"`
		UpdatedRows    int64  `json:"updatedRows"`
		UpdatedColumns int64  `json:"updatedColumns"`
		UpdatedCells   int64  `json:"updatedCells"`
	} `json:"updates"`
}

type clearValuesResponse struct {
	SpreadsheetId string `json:"spreadsheetId"`
	ClearedRange  string `json:"clearedRange"`
}

type batchUpdateRequest struct {
	Requests []map[string]interface{} `json:"requests"`
}

// ── Read ──────────────────────────────────────────────────────────────────────

// ReadSheet reads values from the given range (A1 notation). When range is
// empty, the first sheet tab's title is resolved via a metadata call and used
// as the range — this is necessary because the Sheets API requires an explicit
// range; there is no "values collection" endpoint. Supplying an explicit range
// is more efficient and is recommended when the target tab is known.
func (c *Client) ReadSheet(ctx context.Context, spreadsheetID, readRange string) (interface{}, error) {
	if readRange == "" {
		title, err := c.firstSheetTitle(ctx, spreadsheetID)
		if err != nil {
			return nil, fmt.Errorf("read sheet (resolving first sheet name): %v", err)
		}
		readRange = title
	}

	endpoint := fmt.Sprintf("%s/%s/values/%s", c.baseURL, url.PathEscape(spreadsheetID), url.PathEscape(readRange))
	var vr valueRange
	if err := c.get(ctx, endpoint, &vr); err != nil {
		return nil, fmt.Errorf("read sheet: %v", err)
	}

	if len(vr.Values) == 0 {
		return map[string]interface{}{
			"range":   vr.Range,
			"values":  [][]string{},
			"message": "No data found",
		}, nil
	}

	stringValues := toStringMatrix(vr.Values)
	colCount := 0
	for _, row := range stringValues {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	return map[string]interface{}{
		"range":     vr.Range,
		"values":    stringValues,
		"row_count": len(stringValues),
		"col_count": colCount,
	}, nil
}

// ── Write ─────────────────────────────────────────────────────────────────────

// WriteSheet overwrites the given range with values.
func (c *Client) WriteSheet(ctx context.Context, spreadsheetID, writeRange string, values [][]string) (interface{}, error) {
	body := valueRange{
		MajorDimension: "ROWS",
		Values:         toInterfaceMatrix(values),
	}

	endpoint := fmt.Sprintf("%s/%s/values/%s?valueInputOption=USER_ENTERED",
		c.baseURL, url.PathEscape(spreadsheetID), url.PathEscape(writeRange))

	var resp updateValuesResponse
	if err := c.put(ctx, endpoint, body, &resp); err != nil {
		return nil, fmt.Errorf("write sheet: %v", err)
	}

	return map[string]interface{}{
		"updated_range":   resp.UpdatedRange,
		"updated_rows":    resp.UpdatedRows,
		"updated_columns": resp.UpdatedColumns,
		"updated_cells":   resp.UpdatedCells,
		"message":         "Data written successfully",
	}, nil
}

// ── Append ────────────────────────────────────────────────────────────────────

// AppendSheet appends rows after the last row with data.
func (c *Client) AppendSheet(ctx context.Context, spreadsheetID, appendRange string, values [][]string) (interface{}, error) {
	body := valueRange{
		MajorDimension: "ROWS",
		Values:         toInterfaceMatrix(values),
	}

	endpoint := fmt.Sprintf("%s/%s/values/%s:append?valueInputOption=USER_ENTERED&insertDataOption=INSERT_ROWS",
		c.baseURL, url.PathEscape(spreadsheetID), url.PathEscape(appendRange))

	var resp appendValuesResponse
	if err := c.post(ctx, endpoint, body, &resp); err != nil {
		return nil, fmt.Errorf("append sheet: %v", err)
	}

	return map[string]interface{}{
		"updated_range":   resp.Updates.UpdatedRange,
		"updated_rows":    resp.Updates.UpdatedRows,
		"updated_columns": resp.Updates.UpdatedColumns,
		"updated_cells":   resp.Updates.UpdatedCells,
		"message":         "Data appended successfully",
	}, nil
}

// ── Clear ─────────────────────────────────────────────────────────────────────

// ClearSheet removes all values from a range (formatting is preserved).
func (c *Client) ClearSheet(ctx context.Context, spreadsheetID, clearRange string) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/%s/values/%s:clear",
		c.baseURL, url.PathEscape(spreadsheetID), url.PathEscape(clearRange))

	var resp clearValuesResponse
	if err := c.post(ctx, endpoint, struct{}{}, &resp); err != nil {
		return nil, fmt.Errorf("clear sheet: %v", err)
	}

	return map[string]interface{}{
		"cleared_range": resp.ClearedRange,
		"message":       "Range cleared successfully",
	}, nil
}

// ── Metadata ──────────────────────────────────────────────────────────────────

// GetSpreadsheetInfo returns metadata about the spreadsheet and its sheet tabs.
func (c *Client) GetSpreadsheetInfo(ctx context.Context, spreadsheetID string) (interface{}, error) {
	endpoint := fmt.Sprintf("%s/%s", c.baseURL, url.PathEscape(spreadsheetID))

	var ss spreadsheet
	if err := c.get(ctx, endpoint, &ss); err != nil {
		return nil, fmt.Errorf("get spreadsheet info: %v", err)
	}

	sheetInfo := make([]map[string]interface{}, len(ss.Sheets))
	for i, sh := range ss.Sheets {
		p := sh.Properties
		sheetInfo[i] = map[string]interface{}{
			"sheet_id":    p.SheetID,
			"title":       p.Title,
			"index":       p.Index,
			"sheet_type":  p.SheetType,
			"row_count":   p.GridProperties.RowCount,
			"col_count":   p.GridProperties.ColumnCount,
			"frozen_rows": p.GridProperties.FrozenRowCount,
			"frozen_cols": p.GridProperties.FrozenColumnCount,
		}
	}

	return map[string]interface{}{
		"spreadsheet_id":  ss.SpreadsheetId,
		"title":           ss.Properties.Title,
		"locale":          ss.Properties.Locale,
		"time_zone":       ss.Properties.TimeZone,
		"spreadsheet_url": ss.SpreadsheetUrl,
		"sheets":          sheetInfo,
	}, nil
}

// ── Create ────────────────────────────────────────────────────────────────────

// CreateSpreadsheet creates a new spreadsheet with optional named sheet tabs.
func (c *Client) CreateSpreadsheet(ctx context.Context, title string, sheetNames []string) (interface{}, error) {
	body := spreadsheet{
		Properties: spreadsheetProperties{Title: title},
	}
	if len(sheetNames) > 0 {
		body.Sheets = make([]sheet, len(sheetNames))
		for i, name := range sheetNames {
			body.Sheets[i] = sheet{Properties: sheetProperties{Title: name}}
		}
	}

	var resp spreadsheet
	if err := c.post(ctx, c.baseURL, body, &resp); err != nil {
		return nil, fmt.Errorf("create spreadsheet: %v", err)
	}

	titles := make([]string, len(resp.Sheets))
	for i, sh := range resp.Sheets {
		titles[i] = sh.Properties.Title
	}

	return map[string]interface{}{
		"spreadsheet_id":  resp.SpreadsheetId,
		"spreadsheet_url": resp.SpreadsheetUrl,
		"title":           resp.Properties.Title,
		"sheets":          titles,
		"message":         "Spreadsheet created successfully",
		"next_step":       "Add the spreadsheet_id above to your sheets.json config with the desired access level, then restart the server to use it with other tools.",
	}, nil
}

// ── Add sheet tab ─────────────────────────────────────────────────────────────

// AddSheet adds a new sheet tab to an existing spreadsheet.
func (c *Client) AddSheet(ctx context.Context, spreadsheetID, sheetName string) (interface{}, error) {
	req := batchUpdateRequest{
		Requests: []map[string]interface{}{
			{
				"addSheet": map[string]interface{}{
					"properties": map[string]interface{}{
						"title": sheetName,
					},
				},
			},
		},
	}

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", c.baseURL, url.PathEscape(spreadsheetID))
	var resp struct {
		SpreadsheetId string `json:"spreadsheetId"`
		Replies       []struct {
			AddSheet *struct {
				Properties sheetProperties `json:"properties"`
			} `json:"addSheet,omitempty"`
		} `json:"replies"`
	}

	if err := c.post(ctx, endpoint, req, &resp); err != nil {
		return nil, fmt.Errorf("add sheet: %v", err)
	}

	if len(resp.Replies) > 0 && resp.Replies[0].AddSheet != nil {
		p := resp.Replies[0].AddSheet.Properties
		return map[string]interface{}{
			"sheet_id": p.SheetID,
			"title":    p.Title,
			"index":    p.Index,
			"message":  "Sheet added successfully",
		}, nil
	}

	return map[string]interface{}{"message": "Sheet added successfully"}, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// firstSheetTitle fetches spreadsheet metadata and returns the title of the
// first sheet tab. Used by ReadSheet when no explicit range is supplied.
func (c *Client) firstSheetTitle(ctx context.Context, spreadsheetID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s?fields=sheets.properties.title", c.baseURL, url.PathEscape(spreadsheetID))
	var ss spreadsheet
	if err := c.get(ctx, endpoint, &ss); err != nil {
		return "", err
	}
	if len(ss.Sheets) == 0 {
		return "", fmt.Errorf("spreadsheet has no sheet tabs")
	}
	return ss.Sheets[0].Properties.Title, nil
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, endpoint string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) put(ctx context.Context, endpoint string, body, out interface{}) error {
	return c.doWithBody(ctx, http.MethodPut, endpoint, body, out)
}

func (c *Client) post(ctx context.Context, endpoint string, body, out interface{}) error {
	return c.doWithBody(ctx, http.MethodPost, endpoint, body, out)
}

func (c *Client) doWithBody(ctx context.Context, method, endpoint string, body, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP %s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read response body: %v", err)
	}
	// == is correct here (not >=): io.LimitReader returns at most maxResponseBytes
	// bytes, so len == limit is the precise signal that the body was truncated.
	if int64(len(bodyBytes)) == maxResponseBytes {
		return fmt.Errorf("response body exceeds %d MiB limit; use a smaller range", maxResponseBytes/(1024*1024))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to surface the API error message.
		var apiErr struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &apiErr) == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("API error %d (%s): %s", apiErr.Error.Code, apiErr.Error.Status, apiErr.Error.Message)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if out != nil {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return fmt.Errorf("decode response: %v", err)
		}
	}
	return nil
}

// ── Conversion helpers ────────────────────────────────────────────────────────

func toStringMatrix(in [][]interface{}) [][]string {
	out := make([][]string, len(in))
	for i, row := range in {
		stringRow := make([]string, len(row))
		for j, cell := range row {
			stringRow[j] = fmt.Sprintf("%v", cell)
		}
		out[i] = stringRow
	}
	return out
}

func toInterfaceMatrix(in [][]string) [][]interface{} {
	out := make([][]interface{}, len(in))
	for i, row := range in {
		interfaceRow := make([]interface{}, len(row))
		for j, cell := range row {
			interfaceRow[j] = cell
		}
		out[i] = interfaceRow
	}
	return out
}
