package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/gmonarque/lighthouse/internal/nostr"
	"github.com/gmonarque/lighthouse/internal/trust"
)

// statsCache caches the expensive stats response (60s TTL)
var statsCache struct {
	mu     sync.RWMutex
	data   []byte
	expiry time.Time
}

// GetStats returns dashboard statistics (filtered by trust), cached for 60s
func GetStats(w http.ResponseWriter, r *http.Request) {
	// Serve from cache if fresh
	statsCache.mu.RLock()
	if time.Now().Before(statsCache.expiry) && statsCache.data != nil {
		data := statsCache.data
		statsCache.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}
	statsCache.mu.RUnlock()
	db := database.Get()

	// Get trusted uploaders for filtering
	wot := trust.NewWebOfTrust()
	trustedUploaders, err := wot.GetTrustedUploaders()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get trusted uploaders")
		return
	}

	// Convert npubs to hex pubkeys for matching (uploads are stored as hex)
	var trustedHexPubkeys []string
	for _, npubOrHex := range trustedUploaders {
		if hexPk, err := nostr.NpubToHex(npubOrHex); err == nil {
			trustedHexPubkeys = append(trustedHexPubkeys, hexPk)
		} else {
			trustedHexPubkeys = append(trustedHexPubkeys, npubOrHex)
		}
	}

	stats := make(map[string]interface{})

	// If no trusted uploaders, return zero stats for torrent-related fields
	if len(trustedHexPubkeys) == 0 {
		stats["total_torrents"] = 0
		stats["total_size"] = 0
		stats["categories"] = make(map[int]int64)
		stats["recent_torrents"] = []interface{}{}
	} else {
		// Build trust EXISTS subquery (avoids JOIN + DISTINCT overhead)
		trustPlaceholders := "("
		trustArgs := make([]interface{}, len(trustedHexPubkeys))
		for i, u := range trustedHexPubkeys {
			if i > 0 {
				trustPlaceholders += ","
			}
			trustPlaceholders += "?"
			trustArgs[i] = u
		}
		trustPlaceholders += ")"

		// Drive queries from torrent_uploads (much smaller than torrents).
		// torrent_uploads is filtered by uploader via index, then JOINed to torrents by PK.
		// Use COUNT(DISTINCT) to avoid overcounting torrents with multiple uploads.

		// Count distinct torrents + total size in a single query
		var totalTorrents int64
		var totalSize sql.NullInt64
		summaryQuery := `SELECT COUNT(DISTINCT tu.torrent_id), COALESCE(SUM(t.size), 0)
			FROM torrents t INNER JOIN torrent_uploads tu ON t.id = tu.torrent_id
			WHERE tu.uploader_npub IN ` + trustPlaceholders
		if err := db.QueryRow(summaryQuery, trustArgs...).Scan(&totalTorrents, &totalSize); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get stats")
			return
		}
		stats["total_torrents"] = totalTorrents
		stats["total_size"] = totalSize.Int64

		// Categories: same approach, drive from torrent_uploads
		catQuery := `SELECT t.category, COUNT(DISTINCT tu.torrent_id) as count
			FROM torrents t INNER JOIN torrent_uploads tu ON t.id = tu.torrent_id
			WHERE tu.uploader_npub IN ` + trustPlaceholders + `
			GROUP BY t.category`
		rows, err := db.Query(catQuery, trustArgs...)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get stats")
			return
		}
		defer rows.Close()

		categories := make(map[int]int64)
		for rows.Next() {
			var category int
			var count int64
			if err := rows.Scan(&category, &count); err != nil {
				continue
			}
			categories[category] = count
		}
		stats["categories"] = categories

		// Recent 10: EXISTS with first_seen_at index is fast for small LIMIT
		trustExistsClause := `EXISTS (
			SELECT 1 FROM torrent_uploads tu
			WHERE tu.torrent_id = t.id
			AND tu.uploader_npub IN ` + trustPlaceholders + `
		)`
		recentQuery := `SELECT t.id, t.info_hash, t.name, t.size, t.category, t.seeders, t.leechers,
			t.title, t.year, t.poster_url, t.trust_score, t.first_seen_at
			FROM torrents t
			WHERE ` + trustExistsClause + `
			ORDER BY t.first_seen_at DESC LIMIT 10`
		recentRows, err := db.Query(recentQuery, trustArgs...)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get recent torrents")
			return
		}
		defer recentRows.Close()

		recent := make([]map[string]interface{}, 0)
		for recentRows.Next() {
			var id int64
			var infoHash, name string
			var size, category, seeders, leechers sql.NullInt64
			var title, posterURL sql.NullString
			var year sql.NullInt64
			var trustScore int64
			var firstSeenAt string

			if err := recentRows.Scan(&id, &infoHash, &name, &size, &category, &seeders, &leechers,
				&title, &year, &posterURL, &trustScore, &firstSeenAt); err != nil {
				continue
			}

			recent = append(recent, map[string]interface{}{
				"id":            id,
				"info_hash":     infoHash,
				"name":          name,
				"size":          size.Int64,
				"category":      category.Int64,
				"seeders":       seeders.Int64,
				"leechers":      leechers.Int64,
				"title":         title.String,
				"year":          year.Int64,
				"poster_url":    posterURL.String,
				"trust_score":   trustScore,
				"first_seen_at": firstSeenAt,
			})
		}
		stats["recent_torrents"] = recent
	}

	// Non-torrent stats (not filtered by trust)
	var connectedRelays int64
	if err := db.QueryRow("SELECT COUNT(*) FROM relays WHERE status = 'connected'").Scan(&connectedRelays); err != nil {
		connectedRelays = 0
	}
	stats["connected_relays"] = connectedRelays

	var whitelistCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM trust_whitelist").Scan(&whitelistCount); err != nil {
		whitelistCount = 0
	}
	stats["whitelist_count"] = whitelistCount

	var blacklistCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM trust_blacklist").Scan(&blacklistCount); err != nil {
		blacklistCount = 0
	}
	stats["blacklist_count"] = blacklistCount

	// Unique uploaders (just count trusted pubkeys directly — no DB query needed)
	stats["unique_uploaders"] = int64(len(trustedHexPubkeys))

	// Cache the result for 60 seconds
	if jsonData, err := json.Marshal(stats); err == nil {
		statsCache.mu.Lock()
		statsCache.data = jsonData
		statsCache.expiry = time.Now().Add(60 * time.Second)
		statsCache.mu.Unlock()
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetStatsChart returns chart data for the dashboard (filtered by trust)
func GetStatsChart(w http.ResponseWriter, r *http.Request) {
	daysParam := r.URL.Query().Get("days")
	days := 7
	if daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}

	db := database.Get()

	// Get trusted uploaders for filtering
	wot := trust.NewWebOfTrust()
	trustedUploaders, err := wot.GetTrustedUploaders()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get trusted uploaders")
		return
	}

	// Convert npubs to hex pubkeys for matching
	var trustedHexPubkeys []string
	for _, npubOrHex := range trustedUploaders {
		if hexPk, err := nostr.NpubToHex(npubOrHex); err == nil {
			trustedHexPubkeys = append(trustedHexPubkeys, hexPk)
		} else {
			trustedHexPubkeys = append(trustedHexPubkeys, npubOrHex)
		}
	}

	// If no trusted uploaders, return empty chart data
	if len(trustedHexPubkeys) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"days": days,
			"data": []interface{}{},
		})
		return
	}

	// Build trust EXISTS subquery
	trustPlaceholders := "("
	trustArgs := make([]interface{}, len(trustedHexPubkeys)+1)
	trustArgs[0] = days
	for i, u := range trustedHexPubkeys {
		if i > 0 {
			trustPlaceholders += ","
		}
		trustPlaceholders += "?"
		trustArgs[i+1] = u
	}
	trustPlaceholders += ")"

	// Drive from torrent_uploads, join torrents only for first_seen_at
	query := `SELECT DATE(t.first_seen_at) as date, COUNT(*) as count
		FROM torrents t INNER JOIN torrent_uploads tu ON t.id = tu.torrent_id
		WHERE t.first_seen_at >= DATE('now', '-' || ? || ' days')
		AND tu.uploader_npub IN ` + trustPlaceholders + `
		GROUP BY DATE(t.first_seen_at)
		ORDER BY date ASC`

	rows, err := db.Query(query, trustArgs...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get chart data")
		return
	}
	defer rows.Close()

	data := make([]map[string]interface{}, 0)
	for rows.Next() {
		var date string
		var count int64
		if err := rows.Scan(&date, &count); err != nil {
			continue
		}
		data = append(data, map[string]interface{}{
			"date":  date,
			"count": count,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"days": days,
		"data": data,
	})
}
