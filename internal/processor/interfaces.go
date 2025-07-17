package processor

import (
	"context"
	"io"

	"github.com/zmb3/spotify/v2"
)

// SpotifyClient defines the subset of methods from spotify.Client that our processors need.
type SpotifyClient interface {
	CurrentUser(ctx context.Context) (*spotify.PrivateUser, error)
	CurrentUsersTracks(ctx context.Context, opts ...spotify.RequestOption) (*spotify.SavedTrackPage, error)
	RemoveTracksFromLibrary(ctx context.Context, ids ...spotify.ID) error
	Search(ctx context.Context, query string, t spotify.SearchType, opts ...spotify.RequestOption) (*spotify.SearchResult, error)
	UnfollowPlaylist(ctx context.Context, playlistID spotify.ID) error
	CreatePlaylistForUser(ctx context.Context, userID, playlistName, description string, public bool, collaborative bool) (*spotify.FullPlaylist, error)
	AddTracksToPlaylist(ctx context.Context, playlistID spotify.ID, trackIDs ...spotify.ID) (string, error)
	SetPlaylistImage(ctx context.Context, playlistID spotify.ID, img io.Reader) error
	GetPlaylistsForUser(ctx context.Context, userID string, opts ...spotify.RequestOption) (*spotify.SimplePlaylistPage, error)
	GetPlaylistTracks(context.Context, spotify.ID, ...spotify.RequestOption) (*spotify.PlaylistTrackPage, error)
	RemoveTracksFromPlaylist(context.Context, spotify.ID, ...spotify.ID) (string, error)
}

// Processor defines a generic task that can be executed.
type Processor interface {
	Run(ctx context.Context) error
}

// ImageGenerator defines a component that can generate an image.
type ImageGenerator interface {
	GenerateForPlaylist(name string) (io.Reader, error)
}
