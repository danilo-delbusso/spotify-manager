package processor

import (
	"context"
	"fmt"
	"log"

	"github.com/zmb3/spotify/v2"
)

type artistTrackRemover struct {
	client  SpotifyClient
	artists map[string]struct{}
	logger  *log.Logger
}

// NewArtistTrackRemover is a constructor that takes interfaces as dependencies
// and returns a Processor interface, hiding the concrete implementation.
func NewArtistTrackRemover(client SpotifyClient, artistsToRemove []string, logger *log.Logger) Processor {
	artistSet := make(map[string]struct{}, len(artistsToRemove))
	for _, artist := range artistsToRemove {
		artistSet[artist] = struct{}{}
	}

	return &artistTrackRemover{
		client:  client,
		artists: artistSet,
		logger:  logger,
	}
}

// Run contains the full logic for paging through liked songs and removing
// tracks by the specified artists.
func (p *artistTrackRemover) Run(ctx context.Context) error {
	p.logger.Println("Starting artist track removal process...")

	limit := 50
	offset := 0
	total := -1 // Sentinel value for total tracks

	for {
		p.logger.Printf("Fetching liked songs page (offset: %d)...", offset)
		page, err := p.client.CurrentUsersTracks(ctx, spotify.Offset(offset), spotify.Limit(limit))
		if err != nil {
			return fmt.Errorf("couldn't get liked songs page: %w", err)
		}

		if total == -1 {
			total = int(page.Total)
			p.logger.Printf("Found %d total liked songs to process.", total)
		}

		if len(page.Tracks) == 0 {
			p.logger.Println("No more liked songs found. Task complete.")
			break
		}

		tracksToRemove := p.findTracksToRemove(page.Tracks)

		if len(tracksToRemove) > 0 {
			p.logger.Printf("Attempting to remove %d track(s) from this page.", len(tracksToRemove))
			if err := p.client.RemoveTracksFromLibrary(ctx, tracksToRemove...); err != nil {
				// Log the error but continue, as it might be a transient issue
				p.logger.Printf("❌ ERROR: Failed to remove a batch of tracks: %v", err)
			} else {
				p.logger.Printf("✅ Batch removal successful.")
			}
		} else {
			p.logger.Println("No tracks matching criteria on this page.")
		}

		offset += limit
		if total != -1 && offset >= total {
			p.logger.Println("All songs have been processed. Task complete.")
			break
		}
	}

	return nil
}

// findTracksToRemove iterates a page of tracks and returns a slice of IDs to be removed.
func (p *artistTrackRemover) findTracksToRemove(savedTracks []spotify.SavedTrack) []spotify.ID {
	var idsToRemove []spotify.ID

	for _, item := range savedTracks {
		for _, artist := range item.Artists {
			if _, found := p.artists[artist.Name]; found {
				p.logger.Printf("  [MARK] '%s' by %s", item.Name, artist.Name)
				idsToRemove = append(idsToRemove, item.ID)
				break // Move to the next track once one matching artist is found
			}
		}
	}
	return idsToRemove
}
