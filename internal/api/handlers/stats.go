package handlers

import (
	"net/http"
	"strconv"

	"github.com/lighthouse-client/lighthouse/internal/database"
)

// GetStats returns dashboard statistics
func GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := database.GetStats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	// Get recent torrents
	recent, err := database.GetRecentTorrents(10)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get recent torrents")
		return
	}
	stats["recent_torrents"] = recent

	respondJSON(w, http.StatusOK, stats)
}

// GetStatsChart returns chart data for the dashboard
func GetStatsChart(w http.ResponseWriter, r *http.Request) {
	daysParam := r.URL.Query().Get("days")
	days := 7
	if daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}

	data, err := database.GetTorrentsPerDay(days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get chart data")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"days": days,
		"data": data,
	})
}
