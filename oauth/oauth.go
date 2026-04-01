package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// TokenFileName is the default name for the stored token file.
	TokenFileName = "token.json"
	// RedirectURI is the default OAuth callback address.
	RedirectURI = "http://localhost:8080/oauth/callback"

	// ScopeReadOnly grants read-only access to spreadsheets.
	ScopeReadOnly = "https://www.googleapis.com/auth/spreadsheets.readonly"
	// ScopeReadWrite grants full read/write access to spreadsheets.
	ScopeReadWrite = "https://www.googleapis.com/auth/spreadsheets"
)

// Config holds OAuth client credentials and runtime paths.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenFile    string
}

// LoadConfig loads OAuth client credentials from environment variables or a
// credentials file (oauth_credentials.json / GOOGLE_OAUTH_CREDENTIALS).
func LoadConfig() (*Config, error) {
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")

	if clientID != "" && clientSecret != "" {
		return &Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  redirectURI(),
			TokenFile:    tokenFilePath(),
		}, nil
	}

	credPath := os.Getenv("GOOGLE_OAUTH_CREDENTIALS")
	if credPath == "" {
		homeDir, _ := os.UserHomeDir()
		candidate := filepath.Join(homeDir, ".config", "mcp-google-sheets", "oauth_credentials.json")
		if _, err := os.Stat(candidate); err == nil {
			credPath = candidate
		} else {
			credPath = "oauth_credentials.json"
		}
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf(
			"OAuth credentials not found: set GOOGLE_OAUTH_CLIENT_ID / GOOGLE_OAUTH_CLIENT_SECRET, "+
				"or place oauth_credentials.json at %s (err: %v)", credPath, err)
	}

	var raw struct {
		Installed struct {
			ClientID     string   `json:"client_id"`
			ClientSecret string   `json:"client_secret"`
			RedirectURIs []string `json:"redirect_uris"`
		} `json:"installed"`
		Web struct {
			ClientID     string   `json:"client_id"`
			ClientSecret string   `json:"client_secret"`
			RedirectURIs []string `json:"redirect_uris"`
		} `json:"web"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unable to parse OAuth credentials file: %v", err)
	}

	var id, secret string
	var uris []string
	switch {
	case raw.Installed.ClientID != "":
		id, secret, uris = raw.Installed.ClientID, raw.Installed.ClientSecret, raw.Installed.RedirectURIs
	case raw.Web.ClientID != "":
		id, secret, uris = raw.Web.ClientID, raw.Web.ClientSecret, raw.Web.RedirectURIs
	default:
		return nil, fmt.Errorf("no valid OAuth credentials found in %s", credPath)
	}

	redirect := RedirectURI
	if len(uris) > 0 && uris[0] != "" {
		redirect = uris[0]
	}

	return &Config{
		ClientID:     id,
		ClientSecret: secret,
		RedirectURI:  redirect,
		TokenFile:    tokenFilePath(),
	}, nil
}

// GetClient returns an authenticated HTTP client. If writeAccess is true the
// read+write spreadsheets scope is requested; otherwise read-only is used.
// On first run (no cached token) the user is prompted to complete a browser-based
// OAuth flow; the resulting token is cached for subsequent runs.
func (c *Config) GetClient(ctx context.Context, writeAccess bool) (*http.Client, error) {
	scope := ScopeReadOnly
	if writeAccess {
		scope = ScopeReadWrite
	}
	cfg := c.oauthConfig(scope)

	token, err := c.loadToken()
	if err == nil {
		// Cached token exists. If the scope changed (e.g. config now needs
		// write access but token was read-only) we must re-authorise.
		if writeAccess && !tokenHasWriteScope(token) {
			log.Println("Config requires write access but cached token is read-only. Re-authorising...")
			return c.authorise(ctx, cfg)
		}
		return cfg.Client(ctx, token), nil
	}

	log.Println("No cached token found. Starting OAuth flow...")
	return c.authorise(ctx, cfg)
}

// authorise runs the browser OAuth flow, saves the token, and returns the client.
func (c *Config) authorise(ctx context.Context, cfg *oauth2.Config) (*http.Client, error) {
	token, err := c.getTokenFromWeb(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("OAuth flow failed: %v", err)
	}
	if err := c.saveToken(token); err != nil {
		log.Printf("Warning: unable to cache token: %v", err)
	}
	return cfg.Client(ctx, token), nil
}

// oauthConfig builds an oauth2.Config for the given scope.
func (c *Config) oauthConfig(scope string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURI,
		Scopes:       []string{scope},
		Endpoint:     google.Endpoint,
	}
}

// loadToken reads a cached token from disk.
func (c *Config) loadToken() (*oauth2.Token, error) {
	f, err := os.Open(c.TokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t := &oauth2.Token{}
	return t, json.NewDecoder(f).Decode(t)
}

// saveToken persists a token to disk with restricted permissions.
func (c *Config) saveToken(token *oauth2.Token) error {
	dir := filepath.Dir(c.TokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("unable to create token directory: %v", err)
	}
	f, err := os.OpenFile(c.TokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open token file for writing: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// getTokenFromWeb runs a local HTTP server to receive the OAuth callback and
// exchanges the authorization code for a token.
func (c *Config) getTokenFromWeb(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code in callback")
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `<html><head><title>Authorized</title></head><body>
<h1>Authorization successful!</h1>
<p>You may close this window and return to the terminal.</p>
<script>window.close();</script>
</body></html>`)
		codeCh <- code
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("OAuth callback server error: %v", err)
		}
	}()
	defer srv.Shutdown(ctx) //nolint:errcheck

	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("GOOGLE OAUTH AUTHORIZATION REQUIRED")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("\nOpen the following URL in your browser to authorize access:\n\n  %s\n\n", authURL)
	fmt.Println("Waiting for authorization...")
	fmt.Println(strings.Repeat("=", 72))

	select {
	case code := <-codeCh:
		token, err := cfg.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("unable to exchange authorization code: %v", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("authorization cancelled")
	}
}

// tokenHasWriteScope returns true if the cached token includes the read+write scope.
func tokenHasWriteScope(t *oauth2.Token) bool {
	// The scope is stored in the Extra field when saved from an exchange.
	scope, _ := t.Extra("scope").(string)
	return strings.Contains(scope, ScopeReadWrite)
}

func redirectURI() string {
	if v := os.Getenv("GOOGLE_OAUTH_REDIRECT_URI"); v != "" {
		return v
	}
	return RedirectURI
}

func tokenFilePath() string {
	if p := os.Getenv("GOOGLE_OAUTH_TOKEN_FILE"); p != "" {
		return p
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return TokenFileName
	}
	dir := filepath.Join(homeDir, ".config", "mcp-google-sheets")
	os.MkdirAll(dir, 0700) //nolint:errcheck
	return filepath.Join(dir, TokenFileName)
}
