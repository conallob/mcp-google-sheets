package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, cfg Config) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "sheets.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoad_FileNotExist_ReturnsEmpty(t *testing.T) {
	t.Setenv("GOOGLE_SHEETS_CONFIG", "/nonexistent/sheets.json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected empty config, got error: %v", err)
	}
	if len(cfg.Sheets) != 0 {
		t.Errorf("Expected empty config, got %d sheets", len(cfg.Sheets))
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeConfig(t, Config{Sheets: []SpreadsheetPermission{}})
	t.Setenv("GOOGLE_SHEETS_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Sheets) != 0 {
		t.Errorf("Expected empty sheets, got %d", len(cfg.Sheets))
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, Config{Sheets: []SpreadsheetPermission{
		{ID: "id1", Name: "Budget", Access: AccessRead},
		{ID: "id2", Access: AccessReadWrite},
	}})
	t.Setenv("GOOGLE_SHEETS_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Sheets) != 2 {
		t.Fatalf("Expected 2 sheets, got %d", len(cfg.Sheets))
	}
	if cfg.Sheets[0].Name != "Budget" {
		t.Errorf("Expected Name 'Budget', got %q", cfg.Sheets[0].Name)
	}
}

func TestLoad_InvalidAccess(t *testing.T) {
	data := []byte(`{"sheets":[{"id":"x","access":"admin"}]}`)
	path := filepath.Join(t.TempDir(), "sheets.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GOOGLE_SHEETS_CONFIG", path)

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for invalid access level")
	}
}

func TestLoad_EmptyID(t *testing.T) {
	data := []byte(`{"sheets":[{"id":"","access":"read"}]}`)
	path := filepath.Join(t.TempDir(), "sheets.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GOOGLE_SHEETS_CONFIG", path)

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
}

func TestLoad_NoConfigFile_DefaultsToEmpty(t *testing.T) {
	// Point to a non-existent default path by unsetting the env var and
	// temporarily setting HOME to a temp dir with no config file.
	t.Setenv("GOOGLE_SHEETS_CONFIG", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should return empty config when file missing: %v", err)
	}
	if len(cfg.Sheets) != 0 {
		t.Errorf("Expected empty config, got %d sheets", len(cfg.Sheets))
	}
}

func TestCanRead(t *testing.T) {
	cfg := &Config{Sheets: []SpreadsheetPermission{
		{ID: "id1", Access: AccessRead},
		{ID: "id2", Access: AccessReadWrite},
	}}

	if !cfg.CanRead("id1") {
		t.Error("Expected CanRead('id1') == true")
	}
	if !cfg.CanRead("id2") {
		t.Error("Expected CanRead('id2') == true")
	}
	if cfg.CanRead("unknown") {
		t.Error("Expected CanRead('unknown') == false")
	}
}

func TestCanWrite(t *testing.T) {
	cfg := &Config{Sheets: []SpreadsheetPermission{
		{ID: "id1", Access: AccessRead},
		{ID: "id2", Access: AccessReadWrite},
	}}

	if cfg.CanWrite("id1") {
		t.Error("Expected CanWrite('id1') == false")
	}
	if !cfg.CanWrite("id2") {
		t.Error("Expected CanWrite('id2') == true")
	}
	if cfg.CanWrite("unknown") {
		t.Error("Expected CanWrite('unknown') == false")
	}
}

func TestNeedsWriteScope(t *testing.T) {
	readOnly := &Config{Sheets: []SpreadsheetPermission{{ID: "id1", Access: AccessRead}}}
	if readOnly.NeedsWriteScope() {
		t.Error("Expected NeedsWriteScope == false for read-only config")
	}

	mixed := &Config{Sheets: []SpreadsheetPermission{
		{ID: "id1", Access: AccessRead},
		{ID: "id2", Access: AccessReadWrite},
	}}
	if !mixed.NeedsWriteScope() {
		t.Error("Expected NeedsWriteScope == true when any sheet has readwrite")
	}

	empty := &Config{}
	if empty.NeedsWriteScope() {
		t.Error("Expected NeedsWriteScope == false for empty config")
	}
}

func TestAllowedSheets(t *testing.T) {
	perms := []SpreadsheetPermission{
		{ID: "id1", Access: AccessRead},
		{ID: "id2", Access: AccessReadWrite},
	}
	cfg := &Config{Sheets: perms}
	got := cfg.AllowedSheets()
	if len(got) != 2 {
		t.Errorf("Expected 2 sheets, got %d", len(got))
	}
}
