package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

// Config holds the necessary configuration for the authenticator.
type Config struct {
	RedirectURL  string
	ClientID     string
	ClientSecret string
	Port         string
	Scopes       []string
}

// Authenticator handles the OAuth2 flow for a CLI application.
type Authenticator struct {
	config Config
	auth   *spotifyauth.Authenticator
	state  string
}

// New creates a new Authenticator ready for use.
func New(config Config) *Authenticator {
	return &Authenticator{
		config: config,
		auth: spotifyauth.New(
			spotifyauth.WithRedirectURL(config.RedirectURL),
			spotifyauth.WithClientID(config.ClientID),
			spotifyauth.WithClientSecret(config.ClientSecret),
			spotifyauth.WithScopes(config.Scopes...),
		),
		state: uuid.New().String(),
	}
}

// AuthURL returns the URL the user must visit to grant permissions.
func (a *Authenticator) AuthURL() string {
	return a.auth.AuthURL(a.state)
}

// GetClient starts a local server to handle the auth callback and returns an
// authenticated Spotify client. If input is non-nil, the user may alternatively
// paste the callback URL (or just the code) into it, which is useful when the
// browser can't reach the local server (e.g. over SSH).
func (a *Authenticator) GetClient(ctx context.Context, input io.Reader) (*spotify.Client, error) {
	clientChan := make(chan *spotify.Client, 1)
	errChan := make(chan error, 1)

	server := a.startServer(clientChan, errChan)
	if input != nil {
		go a.readPasted(ctx, input, clientChan)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Failed to gracefully shut down server: %v", err)
		}
	}()

	select {
	case client := <-clientChan:
		return client, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// readPasted reads lines from input until one yields a valid token.
func (a *Authenticator) readPasted(ctx context.Context, input io.Reader, clientChan chan *spotify.Client) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		code, err := a.parseCode(line)
		if err != nil {
			log.Printf("❌ %v — try again", err)
			continue
		}
		token, err := a.auth.Exchange(ctx, code)
		if err != nil {
			log.Printf("❌ Could not exchange code for token: %v — try again", err)
			continue
		}
		select {
		case clientChan <- spotify.New(a.auth.Client(ctx, token)):
		default:
		}
		return
	}
}

// parseCode extracts the authorization code from a pasted callback URL, query
// string, or bare code.
func (a *Authenticator) parseCode(input string) (string, error) {
	if !strings.Contains(input, "=") {
		return input, nil
	}
	query := input
	if i := strings.Index(input, "?"); i >= 0 {
		query = input[i+1:]
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("could not parse input: %w", err)
	}
	if e := values.Get("error"); e != "" {
		return "", fmt.Errorf("spotify returned error: %s", e)
	}
	if st := values.Get("state"); st != "" && st != a.state {
		return "", errors.New("state mismatch")
	}
	code := values.Get("code")
	if code == "" {
		return "", errors.New("no code found in input")
	}
	return code, nil
}

// startServer configures and launches the HTTP server in a goroutine.
func (a *Authenticator) startServer(clientChan chan *spotify.Client, errChan chan error) *http.Server {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":" + a.config.Port,
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token, err := a.auth.Token(r.Context(), a.state, r)
		if err != nil {
			http.Error(w, "Couldn't get token", http.StatusForbidden)
			select {
			case errChan <- fmt.Errorf("could not get token: %w", err):
			default:
			}
			return
		}

		client := spotify.New(a.auth.Client(r.Context(), token))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err = fmt.Fprintln(w, "<html><body><h1>Login Completed!</h1><p>You can close this window now.</p></body></html>")
		if err != nil {
			log.Printf("Error writing response: %v", err)
			return
		}
		select {
		case clientChan <- client:
		default:
		}
	})

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			select {
			case errChan <- fmt.Errorf("server failed: %w", err):
			default:
			}
		}
	}()

	return server
}
