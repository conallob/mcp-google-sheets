package sheets

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/sheets/v4"
)

// Client wraps the Google Sheets API service
type Client struct {
	service *sheets.Service
}

// NewClient creates a new Sheets client
func NewClient(service *sheets.Service) *Client {
	return &Client{
		service: service,
	}
}

// ReadSheet reads data from a spreadsheet range
func (c *Client) ReadSheet(ctx context.Context, spreadsheetID, readRange string) (interface{}, error) {
	if readRange == "" {
		readRange = "Sheet1"
	}

	resp, err := c.service.Spreadsheets.Values.Get(spreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		return map[string]interface{}{
			"range":  resp.Range,
			"values": [][]string{},
			"message": "No data found",
		}, nil
	}

	// Convert interface{} values to strings for easier handling
	stringValues := make([][]string, len(resp.Values))
	for i, row := range resp.Values {
		stringRow := make([]string, len(row))
		for j, cell := range row {
			stringRow[j] = fmt.Sprintf("%v", cell)
		}
		stringValues[i] = stringRow
	}

	return map[string]interface{}{
		"range":      resp.Range,
		"values":     stringValues,
		"row_count":  len(stringValues),
		"col_count":  len(stringValues[0]),
	}, nil
}

// WriteSheet writes data to a spreadsheet range
func (c *Client) WriteSheet(ctx context.Context, spreadsheetID, writeRange string, values [][]string) (interface{}, error) {
	// Convert [][]string to [][]interface{} for the API
	interfaceValues := make([][]interface{}, len(values))
	for i, row := range values {
		interfaceRow := make([]interface{}, len(row))
		for j, cell := range row {
			interfaceRow[j] = cell
		}
		interfaceValues[i] = interfaceRow
	}

	valueRange := &sheets.ValueRange{
		Values: interfaceValues,
	}

	resp, err := c.service.Spreadsheets.Values.Update(
		spreadsheetID,
		writeRange,
		valueRange,
	).ValueInputOption("USER_ENTERED").Context(ctx).Do()

	if err != nil {
		return nil, fmt.Errorf("unable to write data to sheet: %v", err)
	}

	return map[string]interface{}{
		"updated_range":   resp.UpdatedRange,
		"updated_rows":    resp.UpdatedRows,
		"updated_columns": resp.UpdatedColumns,
		"updated_cells":   resp.UpdatedCells,
		"message":         "Data written successfully",
	}, nil
}

// AppendSheet appends data to a spreadsheet
func (c *Client) AppendSheet(ctx context.Context, spreadsheetID, appendRange string, values [][]string) (interface{}, error) {
	// Convert [][]string to [][]interface{} for the API
	interfaceValues := make([][]interface{}, len(values))
	for i, row := range values {
		interfaceRow := make([]interface{}, len(row))
		for j, cell := range row {
			interfaceRow[j] = cell
		}
		interfaceValues[i] = interfaceRow
	}

	valueRange := &sheets.ValueRange{
		Values: interfaceValues,
	}

	resp, err := c.service.Spreadsheets.Values.Append(
		spreadsheetID,
		appendRange,
		valueRange,
	).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()

	if err != nil {
		return nil, fmt.Errorf("unable to append data to sheet: %v", err)
	}

	updates := resp.Updates
	return map[string]interface{}{
		"updated_range":   updates.UpdatedRange,
		"updated_rows":    updates.UpdatedRows,
		"updated_columns": updates.UpdatedColumns,
		"updated_cells":   updates.UpdatedCells,
		"message":         "Data appended successfully",
	}, nil
}

// CreateSpreadsheet creates a new spreadsheet
func (c *Client) CreateSpreadsheet(ctx context.Context, title string, sheetNames []string) (interface{}, error) {
	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
	}

	// Add sheets if specified
	if len(sheetNames) > 0 {
		spreadsheet.Sheets = make([]*sheets.Sheet, len(sheetNames))
		for i, name := range sheetNames {
			spreadsheet.Sheets[i] = &sheets.Sheet{
				Properties: &sheets.SheetProperties{
					Title: name,
				},
			}
		}
	}

	resp, err := c.service.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to create spreadsheet: %v", err)
	}

	sheetTitles := make([]string, len(resp.Sheets))
	for i, sheet := range resp.Sheets {
		sheetTitles[i] = sheet.Properties.Title
	}

	return map[string]interface{}{
		"spreadsheet_id":  resp.SpreadsheetId,
		"spreadsheet_url": resp.SpreadsheetUrl,
		"title":           resp.Properties.Title,
		"sheets":          sheetTitles,
		"message":         "Spreadsheet created successfully",
	}, nil
}

// GetSpreadsheetInfo retrieves metadata about a spreadsheet
func (c *Client) GetSpreadsheetInfo(ctx context.Context, spreadsheetID string) (interface{}, error) {
	resp, err := c.service.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve spreadsheet info: %v", err)
	}

	sheetInfo := make([]map[string]interface{}, len(resp.Sheets))
	for i, sheet := range resp.Sheets {
		props := sheet.Properties
		sheetInfo[i] = map[string]interface{}{
			"sheet_id":    props.SheetId,
			"title":       props.Title,
			"index":       props.Index,
			"sheet_type":  props.SheetType,
			"row_count":   props.GridProperties.RowCount,
			"col_count":   props.GridProperties.ColumnCount,
			"frozen_rows": props.GridProperties.FrozenRowCount,
			"frozen_cols": props.GridProperties.FrozenColumnCount,
		}
	}

	return map[string]interface{}{
		"spreadsheet_id":  resp.SpreadsheetId,
		"title":           resp.Properties.Title,
		"locale":          resp.Properties.Locale,
		"time_zone":       resp.Properties.TimeZone,
		"spreadsheet_url": resp.SpreadsheetUrl,
		"sheets":          sheetInfo,
	}, nil
}

// AddSheet adds a new sheet to an existing spreadsheet
func (c *Client) AddSheet(ctx context.Context, spreadsheetID, sheetName string) (interface{}, error) {
	requests := []*sheets.Request{
		{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{
					Title: sheetName,
				},
			},
		},
	}

	batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}

	resp, err := c.service.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdateRequest).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to add sheet: %v", err)
	}

	if len(resp.Replies) > 0 && resp.Replies[0].AddSheet != nil {
		props := resp.Replies[0].AddSheet.Properties
		return map[string]interface{}{
			"sheet_id": props.SheetId,
			"title":    props.Title,
			"index":    props.Index,
			"message":  "Sheet added successfully",
		}, nil
	}

	return map[string]interface{}{
		"message": "Sheet added successfully",
	}, nil
}

// ClearSheet clears data in a specified range
func (c *Client) ClearSheet(ctx context.Context, spreadsheetID, clearRange string) (interface{}, error) {
	clearRequest := &sheets.ClearValuesRequest{}

	resp, err := c.service.Spreadsheets.Values.Clear(spreadsheetID, clearRange, clearRequest).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to clear sheet: %v", err)
	}

	return map[string]interface{}{
		"cleared_range": resp.ClearedRange,
		"message":       "Range cleared successfully",
	}, nil
}

// BatchUpdate performs multiple updates on a spreadsheet
func (c *Client) BatchUpdate(ctx context.Context, spreadsheetID string, requestsData []map[string]interface{}) (interface{}, error) {
	// Convert the generic map to JSON and back to sheets.Request
	requestsJSON, err := json.Marshal(map[string]interface{}{"requests": requestsData})
	if err != nil {
		return nil, fmt.Errorf("unable to marshal requests: %v", err)
	}

	var batchUpdateRequest sheets.BatchUpdateSpreadsheetRequest
	if err := json.Unmarshal(requestsJSON, &batchUpdateRequest); err != nil {
		return nil, fmt.Errorf("unable to unmarshal requests: %v", err)
	}

	resp, err := c.service.Spreadsheets.BatchUpdate(spreadsheetID, &batchUpdateRequest).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to batch update: %v", err)
	}

	return map[string]interface{}{
		"spreadsheet_id": resp.SpreadsheetId,
		"replies_count":  len(resp.Replies),
		"message":        "Batch update completed successfully",
	}, nil
}
