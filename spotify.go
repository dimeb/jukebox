package main

type spotifyImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type spotifyPlaylistTracks struct {
	Href  string `json:"href"`
	Total int    `json:"total"`
}

type spotifyPlaylist struct {
	Href   string                `json:"href"`
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Tracks spotifyPlaylistTracks `json:"tracks"`
	Images []spotifyImage        `json:"images"`
}

type spotifyPlaylists struct {
	Items []spotifyPlaylist `json:"items"`
	Total int               `json:"total"`
}

var (
	spotifyPlaylistsURL string = `https://api.spotify.com/v1/me/playlists`
	spotifyPlaylistURL  string = `https://api.spotify.com/v1/playlists`
)

