package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/gmonarque/lighthouse/internal/nostr"
	"github.com/gmonarque/lighthouse/internal/trust"
)

// GetStats returns dashboard statistics (filtered by trust)
func GetStats(w http.ResponseWriter, r *http.Request) {
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
		// Build trust filter subquery
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

		// Total trusted torrents
		var totalTorrents int64
		countQuery := `SELECT COUNT(DISTINCT t.id) FROM torrents t
			JOIN torrent_uploads tu ON t.id = tu.torrent_id
			WHERE tu.uploader_npub IN ` + trustPlaceholders
		if err := db.QueryRow(countQuery, trustArgs...).Scan(&totalTorrents); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get stats")
			return
		}
		stats["total_torrents"] = totalTorrents

		// Total size (bytes) for trusted torrents
		var totalSize sql.NullInt64
		sizeQuery := `SELECT SUM(t.size) FROM torrents t
			JOIN torrent_uploads tu ON t.id = tu.torrent_id
			WHERE tu.uploader_npub IN ` + trustPlaceholders
		if err := db.QueryRow(sizeQuery, trustArgs...).Scan(&totalSize); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get stats")
			return
		}
		stats["total_size"] = totalSize.Int64

		// Torrents by category (trusted only)
		catQuery := `SELECT t.category, COUNT(DISTINCT t.id) as count FROM torrents t
			JOIN torrent_uploads tu ON t.id = tu.torrent_id
			WHERE tu.uploader_npub IN ` + trustPlaceholders + ` GROUP BY t.category`
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

		// Recent trusted torrents
		recentQuery := `SELECT DISTINCT t.id, t.info_hash, t.name, t.size, t.category, t.seeders, t.leechers,
			t.title, t.year, t.poster_url, t.trust_score, t.first_seen_at
			FROM torrents t
			JOIN torrent_uploads tu ON t.id = tu.torrent_id
			WHERE tu.uploader_npub IN ` + trustPlaceholders + `
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

	// Unique uploaders should also be filtered by trust
	var uniqueUploaders int64
	if len(trustedHexPubkeys) > 0 {
		// Build placeholder string for IN clause
		uploaderPlaceholders := "("
		uploaderArgs := make([]interface{}, len(trustedHexPubkeys))
		for i, u := range trustedHexPubkeys {
			if i > 0 {
				uploaderPlaceholders += ","
			}
			uploaderPlaceholders += "?"
			uploaderArgs[i] = u
		}
		uploaderPlaceholders += ")"

		uploaderQuery := `SELECT COUNT(DISTINCT uploader_npub) FROM torrent_uploads WHERE uploader_npub IN ` + uploaderPlaceholders
		if err := db.QueryRow(uploaderQuery, uploaderArgs...).Scan(&uniqueUploaders); err != nil {
			uniqueUploaders = 0
		}
	} else {
		uniqueUploaders = 0
	}
	stats["unique_uploaders"] = uniqueUploaders

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

	// Build trust filter
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

	query := `SELECT DATE(t.first_seen_at) as date, COUNT(DISTINCT t.id) as count
		FROM torrents t
		JOIN torrent_uploads tu ON t.id = tu.torrent_id
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
