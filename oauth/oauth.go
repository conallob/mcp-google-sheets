package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Warning: unable to determine home directory (%v); falling back to current directory for OAuth credentials", err)
		}
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

	token, cachedScope, err := c.loadToken()
	if err == nil {
		// Cached token exists. If the scope changed (e.g. config now needs
		// write access but token was read-only) we must re-authorise.
		// Note: a token with ScopeReadWrite that is later configured for
		// read-only will continue to use the broader token — this is expected
		// behaviour since narrowing scope requires user interaction to revoke
		// the grant.
		if writeAccess && !strings.Contains(cachedScope, ScopeReadWrite) {
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
	// Store the granted scope alongside the token so it survives reload
	// (oauth2.Token.raw is unexported and not round-tripped by encoding/json).
	grantedScope := ""
	if len(cfg.Scopes) > 0 {
		grantedScope = cfg.Scopes[0]
	}
	if err := c.saveToken(token, grantedScope); err != nil {
		log.Printf("Warning: unable to cache token: %v", err)
	}
	return cfg.Client(ctx, token), nil
}

// persistedToken is the on-disk format for cached OAuth credentials.
// Wrapping oauth2.Token in a struct lets us store the granted scope explicitly,
// since oauth2.Token.raw is unexported and is not preserved through
// encoding/json round-trips.
type persistedToken struct {
	Token *oauth2.Token `json:"token"`
	Scope string        `json:"scope"`
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

// loadToken reads a cached token and its scope from disk.
// It supports both the current persistedToken format and the legacy bare
// oauth2.Token format (which returns an empty scope, triggering re-auth for
// write-access requests).
func (c *Config) loadToken() (*oauth2.Token, string, error) {
	data, err := os.ReadFile(c.TokenFile)
	if err != nil {
		return nil, "", err
	}

	// Try the current wrapper format first. A successful unmarshal into
	// persistedToken with a non-nil Token field means the file was written by
	// this version of the code. The unmarshal error itself is intentionally
	// ignored here: a bare oauth2.Token also unmarshals without error into
	// persistedToken (all fields are JSON-compatible), but pt.Token will be nil
	// in that case, so the nil check distinguishes the two formats.
	var pt persistedToken
	if err := json.Unmarshal(data, &pt); err == nil && pt.Token != nil {
		return pt.Token, pt.Scope, nil
	}

	// Fall back to the legacy bare-token format for tokens written by older
	// versions. Scope will be empty, so re-auth is triggered on the next
	// write-access request.
	var t oauth2.Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, "", fmt.Errorf("parse token file: %v", err)
	}
	if t.AccessToken == "" {
		return nil, "", fmt.Errorf("invalid token file: no access token")
	}
	return &t, "", nil
}

// saveToken persists a token and its granted scope to disk with restricted
// permissions. The directory is created if it does not exist. Pass an empty
// string for scope when the scope is unknown; GetClient will re-authorise on
// the next write-access request in that case.
func (c *Config) saveToken(token *oauth2.Token, scope string) error {
	dir := filepath.Dir(c.TokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("unable to create token directory: %v", err)
	}
	f, err := os.OpenFile(c.TokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open token file for writing: %v", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(persistedToken{Token: token, Scope: scope})
}

// newOAuthCallbackMux builds the HTTP mux for the OAuth redirect callback.
// It verifies the state parameter to prevent CSRF, then forwards the code.
// Separated from getTokenFromWeb so that the CSRF logic can be unit-tested
// without starting a real listener.
func newOAuthCallbackMux(state string, codeCh chan<- string, errCh chan<- error) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != state {
			errCh <- fmt.Errorf("OAuth state mismatch (possible CSRF): got %q", got)
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
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
	return mux
}

// getTokenFromWeb runs a local HTTP server to receive the OAuth callback and
// exchanges the authorization code for a token.
func (c *Config) getTokenFromWeb(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	// Generate a random state token to protect against CSRF attacks.
	// 16 bytes = 128 bits of entropy, the recommended minimum for CSRF tokens.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generate OAuth state token: %v", err)
	}
	state := hex.EncodeToString(stateBytes)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := newOAuthCallbackMux(state, codeCh, errCh)

	// Bind to loopback only — the callback must not be reachable from other
	// machines on the same network.
	srv := &http.Server{Addr: "127.0.0.1:8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("OAuth callback server error: %v", err)
		}
	}()
	defer srv.Shutdown(ctx) //nolint:errcheck

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
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
	// Directory creation is deferred to saveToken so that tokenFilePath
	// remains a pure path getter without filesystem side effects.
	return filepath.Join(homeDir, ".config", "mcp-google-sheets", TokenFileName)
}
