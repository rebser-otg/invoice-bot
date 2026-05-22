package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var scopes = []string{
	gmailv1.GmailReadonlyScope,
	gmailv1.GmailSendScope,
}

// Client wraps the Gmail API service.
type Client struct {
	svc *gmailv1.Service
}

// NewClient loads OAuth2 credentials and returns an authenticated Client.
// If token.json is missing or expired, it prints a URL for the consent flow,
// reads the auth code from stdin, and saves the resulting token to tokenPath.
func NewClient(credentialsPath, tokenPath string) (*Client, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf(
				"credentials.json not found at %q\n"+
					"Download it from https://console.cloud.google.com/apis/credentials\n"+
					"(Create OAuth 2.0 Client ID → Desktop App, then download JSON)",
				credentialsPath,
			)
		}
		return nil, fmt.Errorf("reading credentials: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	tok, err := loadToken(tokenPath)
	if err != nil {
		tok, err = runBrowserFlow(oauthCfg)
		if err != nil {
			return nil, fmt.Errorf("OAuth flow: %w", err)
		}
		if err := saveToken(tokenPath, tok); err != nil {
			return nil, fmt.Errorf("saving token: %w", err)
		}
	}

	httpClient := oauthCfg.Client(context.Background(), tok)
	svc, err := gmailv1.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating Gmail service: %w", err)
	}
	return &Client{svc: svc}, nil
}

func loadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveToken(path string, tok *oauth2.Token) error {
	data, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// runBrowserFlow starts a local HTTP server on a random port, opens the OAuth
// consent URL in the user's browser, waits for the redirect callback, and
// exchanges the received code for a token.
func runBrowserFlow(cfg *oauth2.Config) (*oauth2.Token, error) {
	// Bind to a random free port on loopback.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", port)
	cfg.RedirectURL = redirectURL

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback: %s", r.URL.RawQuery)
			http.Error(w, "Authorization failed — no code received.", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "<html><body><p>Authorization successful — you can close this tab.</p></body></html>")
		codeCh <- code
	})

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Close()

	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Opening browser for Gmail authorization...\n%s\n\n(Waiting for redirect on %s)\n", authURL, redirectURL)

	// Best-effort browser open; user can always paste the URL manually.
	_ = openBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return nil, fmt.Errorf("OAuth callback: %w", err)
	}

	tok, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}
	return tok, nil
}

// openBrowser attempts to open url in the default browser. Errors are ignored
// because the user can always paste the URL manually.
func openBrowser(url string) error {
	// Try common Linux launchers, then macOS, then Windows.
	for _, cmd := range []string{"xdg-open", "open", "start"} {
		if err := exec.Command(cmd, url).Start(); err == nil {
			return nil
		}
	}
	return nil
}

// BuildQuery constructs a Gmail search query matching any of the given sender addresses.
// Returns an empty string if senders is empty.
func BuildQuery(senders []string) string {
	if len(senders) == 0 {
		return ""
	}
	return "from:(" + strings.Join(senders, " OR ") + ")"
}

// Search returns all Gmail message IDs matching the configured senders.
// Handles pagination automatically.
func (c *Client) Search(senders []string) ([]string, error) {
	query := BuildQuery(senders)
	var ids []string
	pageToken := ""
	for {
		call := c.svc.Users.Messages.List("me").Q(query)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("gmail search: %w", err)
		}
		for _, m := range r.Messages {
			ids = append(ids, m.Id)
		}
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return ids, nil
}

// Send encodes raw RFC 2822 bytes as base64url and sends them via the Gmail API.
func (c *Client) Send(raw []byte) error {
	encoded := base64.URLEncoding.EncodeToString(raw)
	msg := &gmailv1.Message{Raw: encoded}
	_, err := c.svc.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	return nil
}

// FetchRaw retrieves the raw RFC 2822 bytes for the given Gmail message ID.
func (c *Client) FetchRaw(id string) ([]byte, error) {
	msg, err := c.svc.Users.Messages.Get("me", id).Format("raw").Do()
	if err != nil {
		return nil, fmt.Errorf("fetching message %s: %w", id, err)
	}
	// Gmail returns unpadded base64url (RFC 4648 §5); fall back to padded just in case.
	raw, err := base64.RawURLEncoding.DecodeString(msg.Raw)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(msg.Raw)
		if err != nil {
			return nil, fmt.Errorf("decoding message %s: %w", id, err)
		}
	}
	return raw, nil
}
