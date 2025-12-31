package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/gmonarque/lighthouse/internal/nostr"
	gonostr "github.com/nbd-wtf/go-nostr"
)

// Publisher interface for publishing events to relays
type Publisher interface {
	PublishToRelays(ctx context.Context, event *gonostr.Event, relayIDs []int) []nostr.PublishResult
}

// Global publisher reference (set by main.go)
var publisher Publisher

// SetPublisher sets the publisher (RelayManager) reference
func SetPublisher(p Publisher) {
	publisher = p
}

// ParseTorrentFile handles parsing of uploaded .torrent files
func ParseTorrentFile(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form with 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	// Get the file from the form
	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	// Parse the torrent file
	info, err := nostr.ParseTorrentReader(file)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to parse torrent file: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, info)
}

// PublishTorrentRequest is the request body for publishing a torrent
type PublishTorrentRequest struct {
	InfoHash    string              `json:"info_hash"`
	Name        string              `json:"name"`
	Size        int64               `json:"size"`
	Category    int                 `json:"category"`
	Files       []nostr.TorrentFile `json:"files"`
	Trackers    []string            `json:"trackers"`
	Tags        []string            `json:"tags"`
	Description string              `json:"description"`
	ImdbID      string              `json:"imdb_id"`
	TmdbID      string              `json:"tmdb_id"`
	RelayIDs    []int               `json:"relay_ids"`
}

// PublishTorrentResponse is the response after publishing
type PublishTorrentResponse struct {
	EventID string                `json:"event_id"`
	Results []nostr.PublishResult `json:"results"`
}

// PublishTorrent publishes a torrent event to Nostr relays
func PublishTorrent(w http.ResponseWriter, r *http.Request) {
	if publisher == nil {
		respondError(w, http.StatusInternalServerError, "Publisher not initialized")
		return
	}

	// Parse request body
	var req PublishTorrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate required fields
	if req.InfoHash == "" {
		respondError(w, http.StatusBadRequest, "info_hash is required")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Size <= 0 {
		respondError(w, http.StatusBadRequest, "size must be positive")
		return
	}

	// Validate info hash format (40 hex chars)
	infoHashRegex := regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	if !infoHashRegex.MatchString(req.InfoHash) {
		respondError(w, http.StatusBadRequest, "info_hash must be 40 hex characters")
		return
	}

	// Get user's nsec from config
	cfg := config.Get()
	if cfg.Nostr.Identity.Nsec == "" {
		respondError(w, http.StatusBadRequest, "No identity configured. Generate or import an nsec in settings.")
		return
	}

	// Create the event
	eventReq := nostr.PublishTorrentRequest{
		InfoHash:    req.InfoHash,
		Name:        req.Name,
		Size:        req.Size,
		Category:    req.Category,
		Files:       req.Files,
		Trackers:    req.Trackers,
		Tags:        req.Tags,
		Description: req.Description,
		ImdbID:      req.ImdbID,
		TmdbID:      req.TmdbID,
	}

	event := nostr.CreateFullTorrentEvent(eventReq)

	// Sign the event
	if err := nostr.SignEvent(event, cfg.Nostr.Identity.Nsec); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to sign event: "+err.Error())
		return
	}

	// Publish to relays
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results := publisher.PublishToRelays(ctx, event, req.RelayIDs)

	// Log the publish activity
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}
	database.LogActivity("torrent_published", req.Name)

	respondJSON(w, http.StatusOK, PublishTorrentResponse{
		EventID: event.ID,
		Results: results,
	})
}
