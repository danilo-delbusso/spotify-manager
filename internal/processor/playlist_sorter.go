package processor

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/zmb3/spotify/v2"
)

type playlistSorter struct {
	client SpotifyClient
	logger *log.Logger
	imgGen ImageGenerator
}

func NewPlaylistSorter(client SpotifyClient, logger *log.Logger, imgGen ImageGenerator) *playlistSorter {
	return &playlistSorter{
		client: client,
		logger: logger,
		imgGen: imgGen,
	}
}

// Run fetches liked songs, groups them by the year they were added, and creates or updates
// a playlist for each year. If a playlist for a year already exists, it will be cleared
// and repopulated with the correct tracks.
func (p *playlistSorter) Run(ctx context.Context) error {
	p.logger.Println("Starting liked songs sorter...")
	allTracks, err := p.fetchAllLikedTracks(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch liked tracks: %w", err)
	}
	if len(allTracks) == 0 {
		p.logger.Println("No liked tracks found. Nothing to do.")
		return nil
	}
	tracksByYear := p.groupTracksByYear(allTracks)
	user, err := p.client.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	years := make([]int, 0, len(tracksByYear))
	for year := range tracksByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	p.logger.Printf("Found songs spanning %d years: %v", len(years), years)

	for _, year := range years {
		playlistName := fmt.Sprintf("Liked Songs (%d)", year)
		trackIDs := tracksByYear[year]
		p.logger.Printf("--- Processing year %d (%d tracks) ---", year, len(trackIDs))

		var playlistID spotify.ID

		existingPlaylist, err := p.findExistingPlaylist(ctx, user.ID, playlistName)
		if err != nil {
			return err
		}

		if existingPlaylist != nil {
			playlistID = existingPlaylist.ID
			p.logger.Printf("Found existing playlist: '%s'. Clearing it now.", existingPlaylist.Name)

			tracksToRemove, err := p.fetchAllPlaylistTracks(ctx, playlistID)
			if err != nil {
				return fmt.Errorf("could not fetch tracks from existing playlist '%s': %w", playlistName, err)
			}
			if err := p.removeTracksInBatches(ctx, playlistID, tracksToRemove); err != nil {
				return fmt.Errorf("could not clear existing playlist '%s': %w", playlistName, err)
			}
		} else {
			description := fmt.Sprintf("All songs I liked that were added in %d.", year)
			newPlaylist, err := p.client.CreatePlaylistForUser(ctx, user.ID, playlistName, description, false, false)
			if err != nil {
				return fmt.Errorf("failed to create playlist for year %d: %w", year, err)
			}
			playlistID = newPlaylist.ID
			p.logger.Printf("✅ Created new playlist: '%s'", newPlaylist.Name)
		}

		p.logger.Println("Generating custom cover image...")
		imageReader, err := p.imgGen.GenerateForPlaylist(playlistName)
		if err != nil {
			p.logger.Printf("⚠️  Could not generate image for '%s': %v", playlistName, err)
		} else {
			if err := p.client.SetPlaylistImage(ctx, playlistID, imageReader); err != nil {
				p.logger.Printf("⚠️  Could not upload cover image for '%s': %v", playlistName, err)
			} else {
				p.logger.Println("✅ Custom cover image uploaded.")
			}
		}

		if err := p.addTracksInBatches(ctx, playlistID, trackIDs); err != nil {
			return err
		}
	}
	return nil
}

// fetchAllLikedTracks pages through the entire "Liked Songs" library using manual pagination.
func (p *playlistSorter) fetchAllLikedTracks(ctx context.Context) ([]spotify.SavedTrack, error) {
	var allTracks []spotify.SavedTrack
	limit := 50
	offset := 0

	for {
		page, err := p.client.CurrentUsersTracks(ctx, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return nil, err
		}
		if len(page.Tracks) == 0 {
			break
		}
		allTracks = append(allTracks, page.Tracks...)
		p.logger.Printf("Fetched %d/%d liked songs...", len(allTracks), page.Total)
		offset += len(page.Tracks)
	}
	p.logger.Printf("Total liked songs fetched: %d", len(allTracks))
	return allTracks, nil
}

// groupTracksByYear categorizes tracks into a map where the key is the year.
func (p *playlistSorter) groupTracksByYear(tracks []spotify.SavedTrack) map[int][]spotify.ID {
	grouped := make(map[int][]spotify.ID)
	for _, item := range tracks {
		t, err := time.Parse(time.RFC3339, item.AddedAt)
		if err != nil {
			p.logger.Printf("Error parsing track date for '%s': %v", item.Name, err)
			continue
		}
		year := t.Year()
		grouped[year] = append(grouped[year], item.ID)
	}
	return grouped
}

// findExistingPlaylist searches for a playlist by name using manual pagination.
func (p *playlistSorter) findExistingPlaylist(ctx context.Context, userID, name string) (*spotify.SimplePlaylist, error) {
	p.logger.Printf("Searching for existing playlist named '%s'...", name)
	limit := 50
	offset := 0

	for {
		page, err := p.client.GetPlaylistsForUser(ctx, userID, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return nil, fmt.Errorf("failed to get user playlists: %w", err)
		}

		for _, pl := range page.Playlists {
			if pl.Name == name && pl.Owner.ID == userID {
				p.logger.Printf("Found existing playlist: '%s' (ID: %s)", pl.Name, pl.ID)
				found := pl // Create a new variable to ensure we don't return a pointer to the loop variable.
				return &found, nil
			}
		}
		if len(page.Playlists) == 0 {
			break
		}
		offset += len(page.Playlists)
	}

	p.logger.Println("No existing playlist found.")
	return nil, nil
}

// fetchAllPlaylistTracks pages through a playlist's items using manual pagination.
func (p *playlistSorter) fetchAllPlaylistTracks(ctx context.Context, playlistID spotify.ID) ([]spotify.ID, error) {
	var allTrackIDs []spotify.ID
	limit := 100
	offset := 0

	for {
		page, err := p.client.GetPlaylistTracks(ctx, playlistID, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return nil, err
		}
		if len(page.Tracks) == 0 {
			break
		}
		for _, item := range page.Tracks {
			// A track might be unavailable in the user's region or deleted, so we check for a valid ID.
			if item.Track.ID != "" {
				allTrackIDs = append(allTrackIDs, item.Track.ID)
			}
		}
		p.logger.Printf("Fetched %d/%d existing tracks from playlist...", len(allTrackIDs), page.Total)
		offset += len(page.Tracks)
	}
	return allTrackIDs, nil
}

// removeTracksInBatches removes tracks from a playlist in batches of 100.
func (p *playlistSorter) removeTracksInBatches(ctx context.Context, playlistID spotify.ID, trackIDs []spotify.ID) error {
	if len(trackIDs) == 0 {
		p.logger.Println("Playlist is already empty. No tracks to remove.")
		return nil
	}

	batchSize := 100
	for i := 0; i < len(trackIDs); i += batchSize {
		end := i + batchSize
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		batch := trackIDs[i:end]
		p.logger.Printf("  Removing batch of %d tracks...", len(batch))
		if _, err := p.client.RemoveTracksFromPlaylist(ctx, playlistID, batch...); err != nil {
			return fmt.Errorf("failed to remove tracks from playlist: %w", err)
		}
	}
	p.logger.Printf("✅ Finished removing all %d old tracks.", len(trackIDs))
	return nil
}

// addTracksInBatches adds tracks to a playlist in batches of 100.
func (p *playlistSorter) addTracksInBatches(ctx context.Context, playlistID spotify.ID, trackIDs []spotify.ID) error {
	batchSize := 100
	for i := 0; i < len(trackIDs); i += batchSize {
		end := i + batchSize
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		batch := trackIDs[i:end]
		p.logger.Printf("  Adding batch of %d tracks...", len(batch))
		if _, err := p.client.AddTracksToPlaylist(ctx, playlistID, batch...); err != nil {
			return fmt.Errorf("failed to add tracks to playlist: %w", err)
		}
	}
	p.logger.Printf("✅ Finished adding all %d tracks.", len(trackIDs))
	return nil
}
