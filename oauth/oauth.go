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
	"google.golang.org/api/sheets/v4"
)

const (
	// TokenFileName is the default name for the stored token file
	TokenFileName = "token.json"
	// Default redirect URI for OAuth flow
	RedirectURI = "http://localhost:8080/oauth/callback"
)

// Config holds OAuth configuration
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenFile    string
}

// LoadConfig loads OAuth configuration from environment variables or a config file
func LoadConfig() (*Config, error) {
	// Try to load from environment variables first
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")

	if clientID != "" && clientSecret != "" {
		return &Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  getRedirectURI(),
			TokenFile:    getTokenFilePath(),
		}, nil
	}

	// Try to load from oauth_credentials.json file
	credPath := os.Getenv("GOOGLE_OAUTH_CREDENTIALS")
	if credPath == "" {
		// Look for oauth_credentials.json in current directory
		homeDir, _ := os.UserHomeDir()
		credPath = filepath.Join(homeDir, ".config", "mcp-google-sheets", "oauth_credentials.json")
		if _, err := os.Stat(credPath); os.IsNotExist(err) {
			credPath = "oauth_credentials.json"
		}
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read OAuth credentials file: %v. Please set GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET environment variables or provide oauth_credentials.json", err)
	}

	var creds struct {
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

	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("unable to parse OAuth credentials: %v", err)
	}

	// Use installed app credentials if available, otherwise web credentials
	var clientIDVal, clientSecretVal string
	var redirectURIs []string

	if creds.Installed.ClientID != "" {
		clientIDVal = creds.Installed.ClientID
		clientSecretVal = creds.Installed.ClientSecret
		redirectURIs = creds.Installed.RedirectURIs
	} else if creds.Web.ClientID != "" {
		clientIDVal = creds.Web.ClientID
		clientSecretVal = creds.Web.ClientSecret
		redirectURIs = creds.Web.RedirectURIs
	} else {
		return nil, fmt.Errorf("no valid OAuth credentials found in file")
	}

	redirectURI := RedirectURI
	if len(redirectURIs) > 0 {
		redirectURI = redirectURIs[0]
	}

	return &Config{
		ClientID:     clientIDVal,
		ClientSecret: clientSecretVal,
		RedirectURI:  redirectURI,
		TokenFile:    getTokenFilePath(),
	}, nil
}

func getRedirectURI() string {
	if uri := os.Getenv("GOOGLE_OAUTH_REDIRECT_URI"); uri != "" {
		return uri
	}
	return RedirectURI
}

func getTokenFilePath() string {
	if path := os.Getenv("GOOGLE_OAUTH_TOKEN_FILE"); path != "" {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return TokenFileName
	}

	configDir := filepath.Join(homeDir, ".config", "mcp-google-sheets")
	os.MkdirAll(configDir, 0700)
	return filepath.Join(configDir, TokenFileName)
}

// GetOAuthConfig returns an OAuth2 config for Google Sheets
func (c *Config) GetOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURI,
		Scopes: []string{
			sheets.SpreadsheetsScope,
		},
		Endpoint: google.Endpoint,
	}
}

// GetClient retrieves a token from the token file, refreshes if needed, or initiates OAuth flow
func (c *Config) GetClient(ctx context.Context) (*http.Client, error) {
	config := c.GetOAuthConfig()

	// Try to load existing token
	token, err := c.loadToken()
	if err == nil {
		// Token exists, create client (will auto-refresh if needed)
		client := config.Client(ctx, token)

		// Save token if it was refreshed
		go c.saveTokenIfRefreshed(ctx, client, token)

		return client, nil
	}

	// No token exists, need to authenticate
	log.Println("No existing token found. Starting OAuth flow...")
	token, err = c.getTokenFromWeb(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to get token from web: %v", err)
	}

	// Save the token
	if err := c.saveToken(token); err != nil {
		log.Printf("Warning: unable to save token: %v", err)
	}

	return config.Client(ctx, token), nil
}

// loadToken loads a token from the token file
func (c *Config) loadToken() (*oauth2.Token, error) {
	f, err := os.Open(c.TokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves a token to the token file
func (c *Config) saveToken(token *oauth2.Token) error {
	// Ensure directory exists
	dir := filepath.Dir(c.TokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("unable to create token directory: %v", err)
	}

	f, err := os.OpenFile(c.TokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to create token file: %v", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

// saveTokenIfRefreshed checks if token was refreshed and saves it
func (c *Config) saveTokenIfRefreshed(ctx context.Context, client *http.Client, originalToken *oauth2.Token) {
	// This is a simple approach - in a more sophisticated implementation,
	// you might use a custom Transport to detect refreshes
	// For now, we'll periodically check and save if the token changed
}

// getTokenFromWeb initiates the OAuth flow and returns a token
func (c *Config) getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Create a channel to receive the authorization code
	codeChan := make(chan string)
	errChan := make(chan error)

	// Start local server to handle OAuth callback
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in OAuth callback")
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			return
		}

		fmt.Fprintf(w, `
			<html>
			<head><title>Authentication Successful</title></head>
			<body>
				<h1>Authentication Successful!</h1>
				<p>You can close this window and return to the application.</p>
				<script>window.close();</script>
			</body>
			</html>
		`)

		codeChan <- code
	})

	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start OAuth callback server: %v", err)
		}
	}()

	// Generate the authorization URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("GOOGLE OAUTH AUTHENTICATION REQUIRED")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\nPlease visit the following URL to authorize this application:\n\n%s\n\n", authURL)
	fmt.Println("Waiting for authorization...")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// Wait for either the code or an error
	var code string
	var token *oauth2.Token

	select {
	case code = <-codeChan:
		// Exchange code for token
		var err error
		token, err = config.Exchange(ctx, code)
		if err != nil {
			server.Shutdown(ctx)
			return nil, fmt.Errorf("unable to exchange code for token: %v", err)
		}
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return nil, fmt.Errorf("context cancelled")
	}

	// Shutdown the server
	server.Shutdown(ctx)

	return token, nil
}
