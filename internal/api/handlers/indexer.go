package handlers

import (
	"context"
	"net/http"

	"github.com/lighthouse-client/lighthouse/internal/database"
)

// IndexerController interface for controlling the indexer
type IndexerController interface {
	Start(ctx context.Context) error
	Stop()
	IsRunning() bool
}

// RelayLoader interface for loading relays from database
type RelayLoader interface {
	LoadRelaysFromDB() error
}

// Global indexer and relay manager references (set by main.go)
var (
	indexerController IndexerController
	relayLoader       RelayLoader
)

// SetIndexerController sets the indexer controller reference
func SetIndexerController(idx IndexerController) {
	indexerController = idx
}

// SetRelayLoader sets the relay loader reference
func SetRelayLoader(rl RelayLoader) {
	relayLoader = rl
}

// StartIndexer starts the indexer
func StartIndexer(w http.ResponseWriter, r *http.Request) {
	if indexerController == nil {
		respondError(w, http.StatusInternalServerError, "Indexer not initialized")
		return
	}

	if indexerController.IsRunning() {
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "running",
			"message": "Indexer is already running",
		})
		return
	}

	// Load relays from database before starting
	if relayLoader != nil {
		if err := relayLoader.LoadRelaysFromDB(); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to load relays: "+err.Error())
			return
		}
	}

	// Start the indexer
	if err := indexerController.Start(context.Background()); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start indexer: "+err.Error())
		return
	}

	database.SetSetting("indexer_enabled", "true")

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "started",
		"message": "Indexer started successfully",
	})
}

// StopIndexer stops the indexer
func StopIndexer(w http.ResponseWriter, r *http.Request) {
	if indexerController == nil {
		respondError(w, http.StatusInternalServerError, "Indexer not initialized")
		return
	}

	if !indexerController.IsRunning() {
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "stopped",
			"message": "Indexer is already stopped",
		})
		return
	}

	indexerController.Stop()
	database.SetSetting("indexer_enabled", "false")

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "stopped",
		"message": "Indexer stopped successfully",
	})
}

// GetIndexerStatus returns the current indexer status
func GetIndexerStatus(w http.ResponseWriter, r *http.Request) {
	running := false
	if indexerController != nil {
		running = indexerController.IsRunning()
	}

	enabled, _ := database.GetSetting("indexer_enabled")

	// Get indexing stats
	stats, _ := database.GetStats()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"running":          running,
		"enabled":          enabled == "true",
		"total_torrents":   stats["total_torrents"],
		"connected_relays": stats["connected_relays"],
	})
}
