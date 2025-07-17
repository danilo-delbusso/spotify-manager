package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
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
// authenticated Spotify client.
func (a *Authenticator) GetClient(ctx context.Context) (*spotify.Client, error) {
	clientChan := make(chan *spotify.Client)
	errChan := make(chan error, 1)

	server := a.startServer(clientChan, errChan)

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
			errChan <- fmt.Errorf("could not get token: %w", err)
			return
		}

		client := spotify.New(a.auth.Client(r.Context(), token))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err = fmt.Fprintln(w, "<html><body><h1>Login Completed!</h1><p>You can close this window now.</p></body></html>")
		if err != nil {
			log.Printf("Error writing response: %v", err)
			return
		}
		clientChan <- client
	})

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("server failed: %w", err)
		}
	}()

	return server
}
