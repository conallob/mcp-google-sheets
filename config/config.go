package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Access defines the permission level for a spreadsheet.
type Access string

const (
	// AccessRead grants read-only access to a spreadsheet.
	AccessRead Access = "read"
	// AccessReadWrite grants read and write access to a spreadsheet.
	AccessReadWrite Access = "readwrite"
)

// SpreadsheetPermission defines access control for a single spreadsheet.
type SpreadsheetPermission struct {
	// ID is the Google Spreadsheet ID (the long string in the sheet's URL).
	ID string `json:"id"`
	// Name is a human-readable label; optional, used only for display.
	Name string `json:"name,omitempty"`
	// Access is "read" or "readwrite".
	Access Access `json:"access"`
}

// Config holds the set of spreadsheets this MCP server is permitted to access.
type Config struct {
	Sheets []SpreadsheetPermission `json:"sheets"`
}

// Load reads the sheets permissions config from the path specified by
// GOOGLE_SHEETS_CONFIG, or from ~/.config/mcp-google-sheets/sheets.json.
// If the file does not exist, an empty config is returned (no sheets allowed).
func Load() (*Config, error) {
	configPath := os.Getenv("GOOGLE_SHEETS_CONFIG")
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("unable to determine home directory: %v", err)
		}
		configPath = filepath.Join(homeDir, ".config", "mcp-google-sheets", "sheets.json")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Sheets: []SpreadsheetPermission{}}, nil
		}
		return nil, fmt.Errorf("unable to read sheets config at %s: %v", configPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to parse sheets config: %v", err)
	}

	seen := make(map[string]struct{}, len(cfg.Sheets))
	for _, sheet := range cfg.Sheets {
		if sheet.ID == "" {
			return nil, fmt.Errorf("sheets config contains an entry with an empty id")
		}
		if sheet.Access != AccessRead && sheet.Access != AccessReadWrite {
			return nil, fmt.Errorf("invalid access %q for sheet %q: must be %q or %q",
				sheet.Access, sheet.ID, AccessRead, AccessReadWrite)
		}
		if _, dup := seen[sheet.ID]; dup {
			return nil, fmt.Errorf("sheets config contains duplicate id %q", sheet.ID)
		}
		seen[sheet.ID] = struct{}{}
	}

	return &cfg, nil
}

// CanRead returns true if the spreadsheet is in the allowlist (any access level).
func (c *Config) CanRead(spreadsheetID string) bool {
	_, ok := c.find(spreadsheetID)
	return ok
}

// CanWrite returns true if the spreadsheet is in the allowlist with readwrite access.
func (c *Config) CanWrite(spreadsheetID string) bool {
	s, ok := c.find(spreadsheetID)
	return ok && s.Access == AccessReadWrite
}

// NeedsWriteScope returns true if any configured sheet requires write access,
// which determines whether the broader spreadsheets scope must be requested.
func (c *Config) NeedsWriteScope() bool {
	for _, s := range c.Sheets {
		if s.Access == AccessReadWrite {
			return true
		}
	}
	return false
}

// AllowedSheets returns the full list of configured sheet permissions.
func (c *Config) AllowedSheets() []SpreadsheetPermission {
	return c.Sheets
}

func (c *Config) find(spreadsheetID string) (SpreadsheetPermission, bool) {
	for _, s := range c.Sheets {
		if s.ID == spreadsheetID {
			return s, true
		}
	}
	return SpreadsheetPermission{}, false
}
