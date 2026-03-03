package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"spotify/internal/auth"
	"spotify/internal/generator"
	"spotify/internal/processor"
	"time"

	"github.com/joho/godotenv"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

func main() {
	// === 1. Configuration ===
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file")
	}

	authConfig := auth.Config{
		RedirectURL:  "http://127.0.0.1:8000/callback",
		ClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		ClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		Port:         "8000",
		// Scopes updated to allow for playlist creation and modification
		Scopes: []string{
			spotifyauth.ScopeUserLibraryRead,
			spotifyauth.ScopePlaylistModifyPublic,
			spotifyauth.ScopePlaylistModifyPrivate,
			spotifyauth.ScopeImageUpload,
			spotifyauth.ScopeUserLibraryModify,
		},
	}

	if authConfig.ClientID == "" || authConfig.ClientSecret == "" {
		log.Fatal("🚨 SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET must be set.")
	}

	authenticator := auth.New(authConfig)
	fmt.Println("👉 Please log in to Spotify by visiting this URL in your browser:")
	fmt.Println(authenticator.AuthURL())

	authCtx, cancelAuth := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelAuth()

	client, err := authenticator.GetClient(authCtx)
	if err != nil {
		log.Fatalf("❌ Authentication failed: %v", err)
	}

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		log.Fatalf("❌ Couldn't get current user: %v", err)
	}
	fmt.Printf("\n✅ Logged in as: %s\n\n", user.DisplayName)

	logger := log.New(os.Stdout, " ", log.LstdFlags)

	imageGenerator := generator.NewImageGenerator()
	sorterTask := processor.NewPlaylistSorter(client, logger, imageGenerator)

	taskCtx, cancelTask := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancelTask()

	fmt.Println("🚀 Starting processor...")
	if err := sorterTask.Run(taskCtx); err != nil {
		log.Fatalf("❌ Processor run failed: %v", err)
	}

	fmt.Println("\n🎉 Processor finished successfully!")
}
