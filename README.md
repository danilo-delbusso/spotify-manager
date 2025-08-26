## Spotify Manager
A Go application with several tools to automatically manage your Spotify library.
This includes a sorter to organize your "Liked Songs" by year and a 
remover to block specific artists from your playlists.

## Features

This project contains multiple tools.

### Playlist Sorter Features

- **Fetches All Liked Songs**: Pages through your entire "Liked Songs" library.

- **Groups by Year Added**: Sorts tracks into playlists based on the year you added them, not the track's release year (e.g., "Liked Songs (2025)").

- **Smart Playlist Updates**: If a yearly playlist already exists, the tool intelligently clears and repopulates it, preserving the playlist's URL and followers.

- **Custom Cover Art**: Includes a pluggable interface to generate and upload custom cover art for each yearly playlist.

### Artist Remover Features

- **Scans All Playlists**: Checks all playlists you own for tracks by specific artists.

- **Removes Blocked Artists**: Automatically removes any tracks from a configurable list of blocked artists.

- **Automated Cleanup**: Ideal for running periodically to ensure your playlists stay free of artists you don't want to hear.

### Requirements

- Go (version 1.21 or later)

- A Spotify account

- A registered application on the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)

### Setup

#### 1. Create a Spotify Application

- Go to the Spotify Developer Dashboard and log in.

- Click "Create app" and give it a name and description.

- Note your Client ID and Client Secret.

- Go to the app's "Settings".

- In the Redirect URIs field, add http://localhost:3000/callback. This must match exactly.

- Click "Save".

#### 2. Configure Credentials

- Clone this repository to your local machine.

- Create a file named `.env` in the root of the project.

- Add your Spotify credentials to the `.env` file like this:
```bash
SPOTIFY_CLIENT_ID=your_client_id
SPOTIFY_CLIENT_SECRET=your_client_secret
```

#### 3. Install Dependencies

Navigate to the project directory in your terminal and run:

```bash
go mod tidy
```

### Usage

This project contains several tools, or "processors." You can choose which one to run by editing the main.go file.

#### 1. Select a Processor

Open `main.go` and comment/uncomment the desired processor.

##### To run the Playlist Sorter:

Make sure the NewPlaylistSorter line is active and the NewArtistRemover line is commented out.

```go
// main.go (example)
func main() {
    // ... setup code ...

    // Activate the Playlist Sorter
    processor := processor.NewPlaylistSorter(client, logger, imgGen)

    // Make sure the Artist Remover is commented out
    // processor := processor.NewArtistRemover(client, logger, []string{"artist_id_1"})
    
    processor.Run(context.Background())
}
```

##### To run the Artist Remover:

Comment out the sorter and activate the NewArtistRemover, adding the Spotify IDs of the artists you want to block.
```go

// main.go (example)
func main() {
    // ... setup code ...

    // Make sure the Playlist Sorter is commented out
    // processor := processor.NewPlaylistSorter(client, logger, imgGen)

    // Activate the Artist Remover with a list of artist IDs
    blockedArtists := []string{"spotify_artist_id_to_block"}
    processor := processor.NewArtistRemover(client, logger, blockedArtists)

    processor.Run(context.Background())
}
```

#### 2. Run the Application

You'll need to authorize the application.
- Run the app from your terminal: `go run cmd/main.go`. 
- The console will print a URL. Copy it into your browser.
- Log in to Spotify and click "Agree" to grant permissions.

The application will save an authentication token so you don't have to log in again.
