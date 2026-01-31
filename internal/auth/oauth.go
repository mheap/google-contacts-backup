// Package auth provides OAuth2 authentication for Google APIs.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
)

const (
	// tokenDir is the directory name for storing tokens
	tokenDir = ".google-contacts-backup"
	// tokenFile is the filename for the cached token
	tokenFile = "token.json"
)

// Authenticator handles OAuth2 authentication with Google.
type Authenticator struct {
	credentialsFile string
	config          *oauth2.Config
}

// NewAuthenticator creates a new Authenticator with the given credentials file.
func NewAuthenticator(credentialsFile string) *Authenticator {
	return &Authenticator{
		credentialsFile: credentialsFile,
	}
}

// GetClient returns an authenticated HTTP client for Google APIs.
func (a *Authenticator) GetClient(ctx context.Context) (*http.Client, error) {
	// Load credentials
	config, err := a.loadCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	a.config = config

	// Try to load cached token
	token, err := a.loadToken()
	if err == nil && token.Valid() {
		return config.Client(ctx, token), nil
	}

	// If token exists but expired, try to refresh
	if token != nil && token.RefreshToken != "" {
		tokenSource := config.TokenSource(ctx, token)
		newToken, err := tokenSource.Token()
		if err == nil {
			if err := a.saveToken(newToken); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token: %v\n", err)
			}
			return config.Client(ctx, newToken), nil
		}
	}

	// Need to do full OAuth flow
	token, err = a.doOAuthFlow(ctx)
	if err != nil {
		return nil, fmt.Errorf("OAuth flow failed: %w", err)
	}

	// Save token for future use
	if err := a.saveToken(token); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", err)
	}

	return config.Client(ctx, token), nil
}

// loadCredentials loads OAuth2 credentials from the credentials file.
func (a *Authenticator) loadCredentials() (*oauth2.Config, error) {
	data, err := os.ReadFile(a.credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file %s: %w", a.credentialsFile, err)
	}

	// Parse the credentials file (supports both "installed" and "web" application types)
	var creds struct {
		Installed *struct {
			ClientID     string   `json:"client_id"`
			ClientSecret string   `json:"client_secret"`
			AuthURI      string   `json:"auth_uri"`
			TokenURI     string   `json:"token_uri"`
			RedirectURIs []string `json:"redirect_uris"`
		} `json:"installed"`
		Web *struct {
			ClientID     string   `json:"client_id"`
			ClientSecret string   `json:"client_secret"`
			AuthURI      string   `json:"auth_uri"`
			TokenURI     string   `json:"token_uri"`
			RedirectURIs []string `json:"redirect_uris"`
		} `json:"web"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("unable to parse credentials file: %w", err)
	}

	var clientID, clientSecret string
	if creds.Installed != nil {
		clientID = creds.Installed.ClientID
		clientSecret = creds.Installed.ClientSecret
	} else if creds.Web != nil {
		clientID = creds.Web.ClientID
		clientSecret = creds.Web.ClientSecret
	} else {
		return nil, fmt.Errorf("credentials file must contain 'installed' or 'web' application credentials")
	}

	// We'll set redirect URI dynamically when we start the server
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{people.ContactsScope},
		Endpoint:     google.Endpoint,
	}

	return config, nil
}

// doOAuthFlow performs the OAuth2 authorization flow using a local server.
func (a *Authenticator) doOAuthFlow(ctx context.Context) (*oauth2.Token, error) {
	// Start local server on a random port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)
	a.config.RedirectURL = redirectURL

	// Channel to receive the authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start HTTP server to handle callback
	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
	}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "no authorization code received"
			}
			errChan <- fmt.Errorf("authorization failed: %s", errMsg)
			fmt.Fprintf(w, "<html><body><h1>Authorization Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", errMsg)
			return
		}

		codeChan <- code
		fmt.Fprintf(w, "<html><body><h1>Authorization Successful!</h1><p>You can close this window and return to the terminal.</p></body></html>")
	})

	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Generate authorization URL with state
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	authURL := a.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("\nOpening browser for Google authorization...")
	fmt.Println("If the browser doesn't open automatically, please visit:")
	fmt.Println(authURL)
	fmt.Println()

	// Try to open browser
	if err := openBrowser(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: couldn't open browser automatically: %v\n", err)
	}

	// Wait for authorization code or error
	var code string
	select {
	case code = <-codeChan:
		// Got the code
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authorization timed out after 5 minutes")
	}

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)

	// Exchange code for token
	token, err := a.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	return token, nil
}

// tokenPath returns the path to the token file.
func (a *Authenticator) tokenPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, tokenDir, tokenFile), nil
}

// loadToken loads a token from the cache file.
func (a *Authenticator) loadToken() (*oauth2.Token, error) {
	path, err := a.tokenPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// saveToken saves a token to the cache file.
func (a *Authenticator) saveToken(token *oauth2.Token) error {
	path, err := a.tokenPath()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	// Write with restricted permissions
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
