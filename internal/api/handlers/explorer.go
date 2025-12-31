package handlers

import (
	"net/http"

	"github.com/gmonarque/lighthouse/internal/database"
)

// GetExplorerStats returns explorer/indexer statistics
func GetExplorerStats(w http.ResponseWriter, r *http.Request) {
	db := database.Get()

	// Get various stats
	stats := make(map[string]interface{})

	// Events discovered (from activity log)
	var eventsDiscovered int64
	db.QueryRow("SELECT COUNT(*) FROM activity_log WHERE event_type = 'torrent_indexed'").Scan(&eventsDiscovered)
	stats["events_discovered"] = eventsDiscovered

	// Events in last hour
	var eventsLastHour int64
	db.QueryRow("SELECT COUNT(*) FROM activity_log WHERE event_type = 'torrent_indexed' AND created_at > datetime('now', '-1 hour')").Scan(&eventsLastHour)
	stats["events_last_hour"] = eventsLastHour

	// Events in last 24 hours
	var eventsLast24h int64
	db.QueryRow("SELECT COUNT(*) FROM activity_log WHERE event_type = 'torrent_indexed' AND created_at > datetime('now', '-24 hours')").Scan(&eventsLast24h)
	stats["events_last_24h"] = eventsLast24h

	// Total torrents
	var totalTorrents int64
	db.QueryRow("SELECT COUNT(*) FROM torrents").Scan(&totalTorrents)
	stats["total_torrents"] = totalTorrents

	// Connected relays
	var connectedRelays int64
	db.QueryRow("SELECT COUNT(*) FROM relays WHERE enabled = 1").Scan(&connectedRelays)
	stats["connected_relays"] = connectedRelays

	// Unique uploaders
	var uniqueUploaders int64
	db.QueryRow("SELECT COUNT(DISTINCT pubkey) FROM uploads").Scan(&uniqueUploaders)
	stats["unique_uploaders"] = uniqueUploaders

	// Database size
	var dbSize int64
	db.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&dbSize)
	stats["database_size"] = dbSize

	// Events by type (last 7 days)
	rows, err := db.Query(`
		SELECT event_type, COUNT(*) as count
		FROM activity_log
		WHERE created_at > datetime('now', '-7 days')
		GROUP BY event_type
		ORDER BY count DESC
		LIMIT 10
	`)
	if err == nil {
		eventTypes := make([]map[string]interface{}, 0)
		for rows.Next() {
			var eventType string
			var count int64
			if rows.Scan(&eventType, &count) == nil {
				eventTypes = append(eventTypes, map[string]interface{}{
					"type":  eventType,
					"count": count,
				})
			}
		}
		rows.Close()
		stats["event_types"] = eventTypes
	}

	// Hourly activity chart (last 24 hours)
	hourlyRows, err := db.Query(`
		SELECT strftime('%Y-%m-%d %H:00', created_at) as hour, COUNT(*) as count
		FROM activity_log
		WHERE created_at > datetime('now', '-24 hours')
		GROUP BY hour
		ORDER BY hour
	`)
	if err == nil {
		hourlyData := make([]map[string]interface{}, 0)
		for hourlyRows.Next() {
			var hour string
			var count int64
			if hourlyRows.Scan(&hour, &count) == nil {
				hourlyData = append(hourlyData, map[string]interface{}{
					"hour":  hour,
					"count": count,
				})
			}
		}
		hourlyRows.Close()
		stats["hourly_activity"] = hourlyData
	}

	// Queue stats (placeholder - would come from actual explorer instance)
	stats["queue_length"] = 0
	stats["events_dropped"] = 0

	respondJSON(w, http.StatusOK, stats)
}
