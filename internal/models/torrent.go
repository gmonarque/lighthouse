package models

import "time"

// Torrent represents a torrent in the database
type Torrent struct {
	ID          int64     `json:"id"`
	InfoHash    string    `json:"info_hash"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	Category    int       `json:"category"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	MagnetURI   string    `json:"magnet_uri"`
	Files       string    `json:"files,omitempty"`
	Title       string    `json:"title,omitempty"`
	Year        int       `json:"year,omitempty"`
	TmdbID      int       `json:"tmdb_id,omitempty"`
	ImdbID      string    `json:"imdb_id,omitempty"`
	PosterURL   string    `json:"poster_url,omitempty"`
	BackdropURL string    `json:"backdrop_url,omitempty"`
	Overview    string    `json:"overview,omitempty"`
	Genres      string    `json:"genres,omitempty"`
	Rating      float64   `json:"rating,omitempty"`
	TrustScore  int       `json:"trust_score"`
	UploadCount int       `json:"upload_count"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TorrentUpload represents an upload record
type TorrentUpload struct {
	ID           int64     `json:"id"`
	TorrentID    int64     `json:"torrent_id"`
	UploaderNpub string    `json:"uploader_npub"`
	NostrEventID string    `json:"nostr_event_id"`
	RelayURL     string    `json:"relay_url"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// TorrentWithUploaders represents a torrent with its uploaders
type TorrentWithUploaders struct {
	Torrent
	Uploaders []TorrentUpload `json:"uploaders"`
}

// TorrentSearchResult represents a search result
type TorrentSearchResult struct {
	ID          int64   `json:"id"`
	InfoHash    string  `json:"info_hash"`
	Name        string  `json:"name"`
	Size        int64   `json:"size"`
	Category    int     `json:"category"`
	Seeders     int     `json:"seeders"`
	Leechers    int     `json:"leechers"`
	Title       string  `json:"title,omitempty"`
	Year        int     `json:"year,omitempty"`
	PosterURL   string  `json:"poster_url,omitempty"`
	TrustScore  int     `json:"trust_score"`
	FirstSeenAt string  `json:"first_seen_at"`
}
