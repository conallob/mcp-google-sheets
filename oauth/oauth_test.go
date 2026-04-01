package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestLoadConfig_FromEnvironmentVariables(t *testing.T) {
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")
	defer func() {
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.ClientID != "test-client-id" {
		t.Errorf("Expected ClientID 'test-client-id', got %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "test-client-secret" {
		t.Errorf("Expected ClientSecret 'test-client-secret', got %q", cfg.ClientSecret)
	}
	if cfg.RedirectURI == "" {
		t.Error("Expected RedirectURI to be set")
	}
}

func TestLoadConfig_FromFile_Installed(t *testing.T) {
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	credFile := filepath.Join(t.TempDir(), "oauth_credentials.json")
	data, _ := json.Marshal(map[string]interface{}{
		"installed": map[string]interface{}{
			"client_id":     "test-installed-client-id",
			"client_secret": "test-installed-client-secret",
			"redirect_uris": []string{"http://localhost:8080/oauth/callback"},
		},
	})
	if err := os.WriteFile(credFile, data, 0600); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.ClientID != "test-installed-client-id" {
		t.Errorf("Expected ClientID 'test-installed-client-id', got %q", cfg.ClientID)
	}
	if cfg.RedirectURI != "http://localhost:8080/oauth/callback" {
		t.Errorf("Unexpected RedirectURI: %q", cfg.RedirectURI)
	}
}

func TestLoadConfig_FromFile_Web(t *testing.T) {
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	credFile := filepath.Join(t.TempDir(), "oauth_credentials.json")
	data, _ := json.Marshal(map[string]interface{}{
		"web": map[string]interface{}{
			"client_id":     "test-web-client-id",
			"client_secret": "test-web-client-secret",
			"redirect_uris": []string{"http://localhost:9090/callback"},
		},
	})
	if err := os.WriteFile(credFile, data, 0600); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.ClientID != "test-web-client-id" {
		t.Errorf("Expected ClientID 'test-web-client-id', got %q", cfg.ClientID)
	}
	if cfg.RedirectURI != "http://localhost:9090/callback" {
		t.Errorf("Unexpected RedirectURI: %q", cfg.RedirectURI)
	}
}

func TestLoadConfig_FromFile_NoValidCredentials(t *testing.T) {
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	credFile := filepath.Join(t.TempDir(), "oauth_credentials.json")
	data, _ := json.Marshal(map[string]interface{}{"invalid": map[string]interface{}{}})
	os.WriteFile(credFile, data, 0600)

	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error when loading config with no valid credentials")
	}
	if !strings.Contains(err.Error(), "no valid OAuth credentials found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", "/nonexistent/path/oauth_credentials.json")
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error when credentials file doesn't exist")
	}
}

func TestGetRedirectURI(t *testing.T) {
	os.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "http://custom.redirect.uri")
	defer os.Unsetenv("GOOGLE_OAUTH_REDIRECT_URI")

	if got := redirectURI(); got != "http://custom.redirect.uri" {
		t.Errorf("Expected 'http://custom.redirect.uri', got %q", got)
	}

	os.Unsetenv("GOOGLE_OAUTH_REDIRECT_URI")
	if got := redirectURI(); got != RedirectURI {
		t.Errorf("Expected %q, got %q", RedirectURI, got)
	}
}

func TestGetTokenFilePath(t *testing.T) {
	os.Setenv("GOOGLE_OAUTH_TOKEN_FILE", "/custom/token/path.json")
	defer os.Unsetenv("GOOGLE_OAUTH_TOKEN_FILE")

	if got := tokenFilePath(); got != "/custom/token/path.json" {
		t.Errorf("Expected '/custom/token/path.json', got %q", got)
	}

	os.Unsetenv("GOOGLE_OAUTH_TOKEN_FILE")
	if got := tokenFilePath(); got == "" {
		t.Error("Expected non-empty token file path")
	}
}

func TestOAuthConfig_Scopes(t *testing.T) {
	cfg := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    "/tmp/token.json",
	}

	roCfg := cfg.oauthConfig(ScopeReadOnly)
	if len(roCfg.Scopes) == 0 {
		t.Error("Expected scopes to be set")
	}
	if roCfg.Scopes[0] != ScopeReadOnly {
		t.Errorf("Expected read-only scope, got %q", roCfg.Scopes[0])
	}

	rwCfg := cfg.oauthConfig(ScopeReadWrite)
	if rwCfg.Scopes[0] != ScopeReadWrite {
		t.Errorf("Expected read-write scope, got %q", rwCfg.Scopes[0])
	}
}

func TestOAuthConfig_AuthURLGeneration(t *testing.T) {
	cfg := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/oauth/callback",
		TokenFile:    filepath.Join(t.TempDir(), "token.json"),
	}
	oc := cfg.oauthConfig(ScopeReadOnly)
	authURL := oc.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	if authURL == "" {
		t.Error("Expected non-empty auth URL")
	}
	if !strings.Contains(authURL, "client_id=test-client-id") {
		t.Errorf("Auth URL missing client_id: %s", authURL)
	}
	if !strings.Contains(authURL, "access_type=offline") {
		t.Errorf("Auth URL missing access_type=offline: %s", authURL)
	}
}

func TestSaveAndLoadToken(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "test-token.json")
	cfg := &Config{
		ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8080/callback",
		TokenFile:   tokenFile,
	}

	original := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	if err := cfg.saveToken(original); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		t.Error("Token file was not created")
	}

	loaded, err := cfg.loadToken()
	if err != nil {
		t.Fatalf("loadToken failed: %v", err)
	}
	if loaded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken mismatch: want %q, got %q", original.AccessToken, loaded.AccessToken)
	}
	if loaded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken mismatch")
	}
}

func TestLoadToken_FileDoesNotExist(t *testing.T) {
	cfg := &Config{TokenFile: "/nonexistent/token.json"}
	_, err := cfg.loadToken()
	if err == nil {
		t.Error("Expected error for non-existent token file")
	}
}

func TestLoadToken_InvalidJSON(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(tokenFile, []byte("not valid json"), 0600)

	cfg := &Config{TokenFile: tokenFile}
	_, err := cfg.loadToken()
	if err == nil {
		t.Error("Expected error for invalid JSON token file")
	}
}

func TestGetClient_WithValidToken(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token.json")
	cfg := &Config{
		ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8080/callback",
		TokenFile:   tokenFile,
	}

	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	if err := cfg.saveToken(testToken); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	client, err := cfg.GetClient(context.Background(), false)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if client == nil {
		t.Error("Expected non-nil client")
	}
}

func TestTokenSecurity_FilePermissions(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token.json")
	cfg := &Config{
		ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8080/callback",
		TokenFile:   tokenFile,
	}
	tok := &oauth2.Token{AccessToken: "tok", RefreshToken: "ref", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}

	if err := cfg.saveToken(tok); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	info, err := os.Stat(tokenFile)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("Token file has insecure permissions: %o (want 0600)", info.Mode().Perm())
	}
}

func TestTokenSecurity_DirectoryPermissions(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "secure", "dir", "token.json")
	cfg := &Config{
		ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8080/callback",
		TokenFile:   tokenFile,
	}
	tok := &oauth2.Token{AccessToken: "tok", RefreshToken: "ref", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}

	if err := cfg.saveToken(tok); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	dir := filepath.Dir(tokenFile)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("Token directory has insecure permissions: %o (want 0700)", info.Mode().Perm())
	}
}

func TestSaveToken_DirectoryCreation(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "nested", "dir", "token.json")
	cfg := &Config{
		ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8080/callback",
		TokenFile:   tokenFile,
	}
	tok := &oauth2.Token{AccessToken: "tok", RefreshToken: "ref", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}

	if err := cfg.saveToken(tok); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		t.Error("Token file was not created")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")

	credFile := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(credFile, []byte("not valid json"), 0600)

	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error for invalid JSON credentials file")
	}
	if !strings.Contains(err.Error(), "unable to parse OAuth credentials") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestConfig_Constants(t *testing.T) {
	if TokenFileName == "" {
		t.Error("TokenFileName should not be empty")
	}
	if RedirectURI == "" {
		t.Error("RedirectURI should not be empty")
	}
	if !strings.Contains(RedirectURI, "localhost") {
		t.Errorf("Expected RedirectURI to contain 'localhost', got %q", RedirectURI)
	}
	if ScopeReadOnly == "" || ScopeReadWrite == "" {
		t.Error("Scope constants should not be empty")
	}
	if ScopeReadOnly == ScopeReadWrite {
		t.Error("Read-only and read-write scopes should differ")
	}
}

// Benchmarks

func BenchmarkLoadConfig(b *testing.B) {
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "bench-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "bench-secret")
	defer func() {
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := LoadConfig(); err != nil {
			b.Fatalf("LoadConfig failed: %v", err)
		}
	}
}

func BenchmarkSaveToken(b *testing.B) {
	tokenFile := filepath.Join(b.TempDir(), "bench-token.json")
	cfg := &Config{
		ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8080/callback",
		TokenFile:   tokenFile,
	}
	tok := &oauth2.Token{AccessToken: "tok", RefreshToken: "ref", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cfg.saveToken(tok); err != nil {
			b.Fatalf("saveToken failed: %v", err)
		}
	}
}

// Examples

func ExampleLoadConfig() {
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "your-client-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "your-client-secret")
	defer func() {
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}
	if cfg.ClientID != "" {
		fmt.Println("Client ID loaded successfully")
	}
	// Output: Client ID loaded successfully
}
