package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestLoadConfig_FromEnvironmentVariables(t *testing.T) {
	// Set up environment variables
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")
	defer func() {
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	}()

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.ClientID != "test-client-id" {
		t.Errorf("Expected ClientID 'test-client-id', got '%s'", config.ClientID)
	}

	if config.ClientSecret != "test-client-secret" {
		t.Errorf("Expected ClientSecret 'test-client-secret', got '%s'", config.ClientSecret)
	}

	if config.RedirectURI == "" {
		t.Error("Expected RedirectURI to be set")
	}
}

func TestLoadConfig_FromFile_Installed(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	// Create a temporary credentials file
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "oauth_credentials.json")

	credentials := map[string]interface{}{
		"installed": map[string]interface{}{
			"client_id":     "test-installed-client-id",
			"client_secret": "test-installed-client-secret",
			"redirect_uris": []string{"http://localhost:8080/oauth/callback"},
		},
	}

	data, err := json.Marshal(credentials)
	if err != nil {
		t.Fatalf("Failed to marshal credentials: %v", err)
	}

	if err := os.WriteFile(credFile, data, 0600); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	// Set environment to point to our test file
	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.ClientID != "test-installed-client-id" {
		t.Errorf("Expected ClientID 'test-installed-client-id', got '%s'", config.ClientID)
	}

	if config.ClientSecret != "test-installed-client-secret" {
		t.Errorf("Expected ClientSecret 'test-installed-client-secret', got '%s'", config.ClientSecret)
	}

	if config.RedirectURI != "http://localhost:8080/oauth/callback" {
		t.Errorf("Expected RedirectURI 'http://localhost:8080/oauth/callback', got '%s'", config.RedirectURI)
	}
}

func TestLoadConfig_FromFile_Web(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	// Create a temporary credentials file with web credentials
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "oauth_credentials.json")

	credentials := map[string]interface{}{
		"web": map[string]interface{}{
			"client_id":     "test-web-client-id",
			"client_secret": "test-web-client-secret",
			"redirect_uris": []string{"http://localhost:9090/callback"},
		},
	}

	data, err := json.Marshal(credentials)
	if err != nil {
		t.Fatalf("Failed to marshal credentials: %v", err)
	}

	if err := os.WriteFile(credFile, data, 0600); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	// Set environment to point to our test file
	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.ClientID != "test-web-client-id" {
		t.Errorf("Expected ClientID 'test-web-client-id', got '%s'", config.ClientID)
	}

	if config.ClientSecret != "test-web-client-secret" {
		t.Errorf("Expected ClientSecret 'test-web-client-secret', got '%s'", config.ClientSecret)
	}

	if config.RedirectURI != "http://localhost:9090/callback" {
		t.Errorf("Expected RedirectURI 'http://localhost:9090/callback', got '%s'", config.RedirectURI)
	}
}

func TestLoadConfig_FromFile_NoValidCredentials(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	// Create a temporary credentials file with invalid structure
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "oauth_credentials.json")

	credentials := map[string]interface{}{
		"invalid": map[string]interface{}{},
	}

	data, err := json.Marshal(credentials)
	if err != nil {
		t.Fatalf("Failed to marshal credentials: %v", err)
	}

	if err := os.WriteFile(credFile, data, 0600); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	// Set environment to point to our test file
	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	_, err = LoadConfig()
	if err == nil {
		t.Error("Expected error when loading config with no valid credentials")
	}

	if !strings.Contains(err.Error(), "no valid OAuth credentials found") {
		t.Errorf("Expected error message about no valid credentials, got: %v", err)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	// Clear all OAuth-related environment variables
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	// Point to a non-existent file
	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", "/nonexistent/path/oauth_credentials.json")
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error when credentials file doesn't exist")
	}
}

func TestGetRedirectURI(t *testing.T) {
	// Test with environment variable set
	os.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "http://custom.redirect.uri")
	defer os.Unsetenv("GOOGLE_OAUTH_REDIRECT_URI")

	uri := getRedirectURI()
	if uri != "http://custom.redirect.uri" {
		t.Errorf("Expected redirect URI 'http://custom.redirect.uri', got '%s'", uri)
	}

	// Test without environment variable
	os.Unsetenv("GOOGLE_OAUTH_REDIRECT_URI")
	uri = getRedirectURI()
	if uri != RedirectURI {
		t.Errorf("Expected redirect URI '%s', got '%s'", RedirectURI, uri)
	}
}

func TestGetTokenFilePath(t *testing.T) {
	// Test with environment variable set
	os.Setenv("GOOGLE_OAUTH_TOKEN_FILE", "/custom/token/path.json")
	defer os.Unsetenv("GOOGLE_OAUTH_TOKEN_FILE")

	path := getTokenFilePath()
	if path != "/custom/token/path.json" {
		t.Errorf("Expected token file path '/custom/token/path.json', got '%s'", path)
	}

	// Test without environment variable
	os.Unsetenv("GOOGLE_OAUTH_TOKEN_FILE")
	path = getTokenFilePath()
	if path == "" {
		t.Error("Expected token file path to be set")
	}
}

func TestGetOAuthConfig(t *testing.T) {
	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    "/tmp/token.json",
	}

	oauthConfig := config.GetOAuthConfig()

	if oauthConfig.ClientID != "test-client-id" {
		t.Errorf("Expected ClientID 'test-client-id', got '%s'", oauthConfig.ClientID)
	}

	if oauthConfig.ClientSecret != "test-client-secret" {
		t.Errorf("Expected ClientSecret 'test-client-secret', got '%s'", oauthConfig.ClientSecret)
	}

	if oauthConfig.RedirectURL != "http://localhost:8080/callback" {
		t.Errorf("Expected RedirectURL 'http://localhost:8080/callback', got '%s'", oauthConfig.RedirectURL)
	}

	if len(oauthConfig.Scopes) == 0 {
		t.Error("Expected OAuth config to have scopes")
	}
}

func TestSaveAndLoadToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "test-token.json")

	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	// Create a test token
	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Save the token
	err := config.saveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		t.Error("Token file was not created")
	}

	// Load the token
	loadedToken, err := config.loadToken()
	if err != nil {
		t.Fatalf("Failed to load token: %v", err)
	}

	// Verify token contents
	if loadedToken.AccessToken != testToken.AccessToken {
		t.Errorf("Expected AccessToken '%s', got '%s'", testToken.AccessToken, loadedToken.AccessToken)
	}

	if loadedToken.RefreshToken != testToken.RefreshToken {
		t.Errorf("Expected RefreshToken '%s', got '%s'", testToken.RefreshToken, loadedToken.RefreshToken)
	}

	if loadedToken.TokenType != testToken.TokenType {
		t.Errorf("Expected TokenType '%s', got '%s'", testToken.TokenType, loadedToken.TokenType)
	}
}

func TestLoadToken_FileDoesNotExist(t *testing.T) {
	config := &Config{
		TokenFile: "/nonexistent/token.json",
	}

	_, err := config.loadToken()
	if err == nil {
		t.Error("Expected error when loading non-existent token file")
	}
}

func TestGetClient_WithValidToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "test-token.json")

	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	// Create a test token that's not expired
	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Save the token
	if err := config.saveToken(testToken); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Get client with valid token
	ctx := context.Background()
	client, err := config.GetClient(ctx)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	if client == nil {
		t.Error("Expected client to be non-nil")
	}
}

func TestOAuthConfig_AuthURLGeneration(t *testing.T) {
	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/oauth/callback",
		TokenFile:    filepath.Join(t.TempDir(), "token.json"),
	}

	// Test that the OAuth config is set up correctly
	oauthConfig := config.GetOAuthConfig()
	if oauthConfig == nil {
		t.Error("Expected OAuth config to be non-nil")
	}

	// Test auth URL generation
	authURL := oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	if authURL == "" {
		t.Error("Expected auth URL to be non-empty")
	}

	if !strings.Contains(authURL, "client_id=test-client-id") {
		t.Errorf("Expected auth URL to contain client_id, got: %s", authURL)
	}

	if !strings.Contains(authURL, "access_type=offline") {
		t.Errorf("Expected auth URL to contain access_type=offline, got: %s", authURL)
	}
}

func TestTokenSecurity_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.json")

	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Save the token
	err := config.saveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Check file permissions
	fileInfo, err := os.Stat(tokenFile)
	if err != nil {
		t.Fatalf("Failed to stat token file: %v", err)
	}

	// On Unix-like systems, token file should be 0600 (read/write for owner only)
	mode := fileInfo.Mode()
	perm := mode.Perm()

	// Check that file is not world-readable or group-readable
	if perm&0077 != 0 {
		t.Errorf("Token file has insecure permissions: %o (should be 0600)", perm)
	}
}

func TestTokenSecurity_DirectoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "secure", "dir", "token.json")

	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Save the token (should create directories)
	err := config.saveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Check directory permissions
	dir := filepath.Dir(tokenFile)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Failed to stat token directory: %v", err)
	}

	mode := dirInfo.Mode()
	perm := mode.Perm()

	// Directory should be 0700 (rwx for owner only)
	if perm&0077 != 0 {
		t.Errorf("Token directory has insecure permissions: %o (should be 0700)", perm)
	}
}

func TestSaveToken_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a path with nested directories
	tokenFile := filepath.Join(tmpDir, "nested", "dir", "token.json")

	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Save the token - should create directories
	err := config.saveToken(testToken)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Verify directories were created
	dir := filepath.Dir(tokenFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}

	// Verify token file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		t.Error("Token file was not created")
	}
}

func TestConfig_Constants(t *testing.T) {
	if TokenFileName == "" {
		t.Error("TokenFileName constant should not be empty")
	}

	if RedirectURI == "" {
		t.Error("RedirectURI constant should not be empty")
	}

	if !strings.Contains(RedirectURI, "localhost") {
		t.Errorf("Expected RedirectURI to contain 'localhost', got: %s", RedirectURI)
	}
}

func TestLoadToken_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "invalid-token.json")

	// Write invalid JSON
	if err := os.WriteFile(tokenFile, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("Failed to write invalid token file: %v", err)
	}

	config := &Config{
		TokenFile: tokenFile,
	}

	_, err := config.loadToken()
	if err == nil {
		t.Error("Expected error when loading invalid JSON token file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
	os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")

	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "invalid_credentials.json")

	// Write invalid JSON
	if err := os.WriteFile(credFile, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("Failed to write invalid credentials file: %v", err)
	}

	os.Setenv("GOOGLE_OAUTH_CREDENTIALS", credFile)
	defer os.Unsetenv("GOOGLE_OAUTH_CREDENTIALS")

	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error when loading invalid JSON credentials file")
	}

	if !strings.Contains(err.Error(), "unable to parse OAuth credentials") {
		t.Errorf("Expected error message about parsing credentials, got: %v", err)
	}
}

func TestSaveTokenIfRefreshed(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.json")

	config := &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Create a mock client
	client := &http.Client{}

	ctx := context.Background()

	// Call saveTokenIfRefreshed - it should not panic
	config.saveTokenIfRefreshed(ctx, client, testToken)

	// This is a stub implementation in the actual code, so we just verify it doesn't crash
}

func BenchmarkLoadConfig(b *testing.B) {
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "benchmark-client-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "benchmark-client-secret")
	defer func() {
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadConfig()
		if err != nil {
			b.Fatalf("LoadConfig failed: %v", err)
		}
	}
}

func BenchmarkSaveToken(b *testing.B) {
	tmpDir := b.TempDir()
	tokenFile := filepath.Join(tmpDir, "bench-token.json")

	config := &Config{
		ClientID:     "bench-client-id",
		ClientSecret: "bench-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    tokenFile,
	}

	testToken := &oauth2.Token{
		AccessToken:  "bench-access-token",
		RefreshToken: "bench-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := config.saveToken(testToken); err != nil {
			b.Fatalf("saveToken failed: %v", err)
		}
	}
}

func ExampleLoadConfig() {
	// Set up environment variables
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "your-client-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "your-client-secret")
	defer func() {
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
	}()

	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	if config.ClientID != "" {
		fmt.Println("Client ID loaded successfully")
	}
	// Output: Client ID loaded successfully
}
