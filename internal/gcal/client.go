package gcal

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
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// TokenPath returns the path where the OAuth2 token is stored.
func TokenPath(dataDir string) string {
	return filepath.Join(dataDir, "gcal_token.json")
}

// IsConnected returns true if a token file exists.
func IsConnected(dataDir string) bool {
	_, err := os.Stat(TokenPath(dataDir))
	return err == nil
}

// Connect runs the OAuth2 loopback authorization flow and saves the token.
// clientID and clientSecret come from a Google Cloud "Desktop app" OAuth client.
func Connect(clientID, clientSecret, dataDir string) error {
	cfg := oauthConfig(clientID, clientSecret, "") // redirect filled in below

	// Start a local listener on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local server: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	cfg.RedirectURL = redirectURL

	codeCh := make(chan string, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			if code != "" {
				codeCh <- code
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, `<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem">
<h2>✓ btrack connected to Google Calendar</h2>
<p>Authorization successful. You can close this tab.</p>
</body></html>`)
			} else {
				http.Error(w, "missing code", http.StatusBadRequest)
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	authURL := cfg.AuthCodeURL("btrack-state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println()
	fmt.Println("  Opening browser for Google authorization...")
	fmt.Printf("  If it doesn't open, visit:\n\n  %s\n\n", authURL)
	openBrowser(authURL)
	fmt.Println("  Waiting for authorization...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case <-ctx.Done():
		return fmt.Errorf("authorization timed out (5 min) — try again")
	}

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchange auth code: %w", err)
	}

	return saveToken(token, dataDir)
}

// NewService creates a Google Calendar service using stored credentials.
func NewService(clientID, clientSecret, dataDir string) (*calendar.Service, error) {
	token, err := loadToken(dataDir)
	if err != nil {
		return nil, fmt.Errorf("not connected — run: btrack gcal connect")
	}

	cfg := oauthConfig(clientID, clientSecret, "")
	src := cfg.TokenSource(context.Background(), token)

	// Persist a refreshed token if it changed.
	if newTok, err := src.Token(); err == nil && newTok.AccessToken != token.AccessToken {
		_ = saveToken(newTok, dataDir)
	}

	svc, err := calendar.NewService(context.Background(), option.WithTokenSource(src))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}
	return svc, nil
}

// PushSession creates a Google Calendar event for a completed session.
// calendarID defaults to "primary" when empty.
func PushSession(svc *calendar.Service, calendarID, taskName, project string, start, end time.Time) (string, error) {
	title := taskName
	if project != "" {
		title = "[" + project + "] " + taskName
	}
	if calendarID == "" {
		calendarID = "primary"
	}

	event := &calendar.Event{
		Summary: title,
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: start.Location().String(),
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: end.Location().String(),
		},
		Source: &calendar.EventSource{
			Title: "btrack",
			Url:   "https://github.com/tolgazorlu/btrack",
		},
	}

	created, err := svc.Events.Insert(calendarID, event).Do()
	if err != nil {
		return "", fmt.Errorf("insert event: %w", err)
	}
	return created.HtmlLink, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func oauthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{calendar.CalendarEventsScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
	}
}

func saveToken(token *oauth2.Token, dataDir string) error {
	path := TokenPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(path), data, 0600)
}

func loadToken(dataDir string) (*oauth2.Token, error) {
	data, err := os.ReadFile(TokenPath(dataDir))
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
