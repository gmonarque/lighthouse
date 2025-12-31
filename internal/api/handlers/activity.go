package handlers

import (
	"net/http"
	"strconv"

	"github.com/gmonarque/lighthouse/internal/database"
)

// GetActivity returns recent activity logs
func GetActivity(w http.ResponseWriter, r *http.Request) {
	limitParam := r.URL.Query().Get("limit")
	limit := 50
	if limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	offsetParam := r.URL.Query().Get("offset")
	offset := 0
	if offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	eventType := r.URL.Query().Get("type")

	db := database.Get()

	query := `
		SELECT id, event_type, details, created_at
		FROM activity_log
	`
	args := []interface{}{}

	if eventType != "" {
		query += " WHERE event_type = ?"
		args = append(args, eventType)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get activity")
		return
	}
	defer rows.Close()

	activities := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var eventType, createdAt string
		var details *string

		if err := rows.Scan(&id, &eventType, &details, &createdAt); err != nil {
			continue
		}

		activity := map[string]interface{}{
			"id":         id,
			"event_type": eventType,
			"created_at": createdAt,
		}
		if details != nil {
			activity["details"] = *details
		}

		activities = append(activities, activity)
	}

	respondJSON(w, http.StatusOK, activities)
}

// GetLogs returns system logs (placeholder for now)
func GetLogs(w http.ResponseWriter, r *http.Request) {
	// For now, return recent activity as logs
	// In production, this would read from actual log files or a log buffer
	GetActivity(w, r)
}
