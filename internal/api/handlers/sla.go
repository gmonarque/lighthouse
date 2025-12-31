package handlers

import (
	"net/http"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
)

// GetSLAStatus returns SLA compliance status
func GetSLAStatus(w http.ResponseWriter, r *http.Request) {
	db := database.Get()

	status := make(map[string]interface{})

	// Report SLA metrics
	reportMetrics := make(map[string]interface{})

	// Total reports
	var totalReports int64
	db.QueryRow("SELECT COUNT(*) FROM reports").Scan(&totalReports)
	reportMetrics["total"] = totalReports

	// Pending reports (not yet acknowledged)
	var pendingReports int64
	db.QueryRow("SELECT COUNT(*) FROM reports WHERE status = 'pending'").Scan(&pendingReports)
	reportMetrics["pending"] = pendingReports

	// Reports acknowledged within 24h
	var acknowledgedIn24h int64
	db.QueryRow(`
		SELECT COUNT(*) FROM reports
		WHERE acknowledged_at IS NOT NULL
		AND (julianday(acknowledged_at) - julianday(created_at)) * 24 <= 24
	`).Scan(&acknowledgedIn24h)
	reportMetrics["acknowledged_within_sla"] = acknowledgedIn24h

	// Reports past 24h SLA
	var pastSLA int64
	db.QueryRow(`
		SELECT COUNT(*) FROM reports
		WHERE status = 'pending'
		AND (julianday('now') - julianday(created_at)) * 24 > 24
	`).Scan(&pastSLA)
	reportMetrics["past_sla"] = pastSLA

	// Average acknowledgment time (hours)
	var avgAckTime float64
	db.QueryRow(`
		SELECT COALESCE(AVG((julianday(acknowledged_at) - julianday(created_at)) * 24), 0)
		FROM reports
		WHERE acknowledged_at IS NOT NULL
	`).Scan(&avgAckTime)
	reportMetrics["avg_ack_time_hours"] = avgAckTime

	// SLA compliance rate
	if totalReports > 0 {
		reportMetrics["sla_compliance_rate"] = float64(acknowledgedIn24h) / float64(totalReports) * 100
	} else {
		reportMetrics["sla_compliance_rate"] = 100.0
	}

	status["reports"] = reportMetrics

	// Resolution metrics
	resolutionMetrics := make(map[string]interface{})

	// Resolved reports
	var resolvedReports int64
	db.QueryRow("SELECT COUNT(*) FROM reports WHERE status IN ('resolved', 'rejected')").Scan(&resolvedReports)
	resolutionMetrics["total_resolved"] = resolvedReports

	// Average resolution time (hours)
	var avgResTime float64
	db.QueryRow(`
		SELECT COALESCE(AVG((julianday(resolved_at) - julianday(created_at)) * 24), 0)
		FROM reports
		WHERE resolved_at IS NOT NULL
	`).Scan(&avgResTime)
	resolutionMetrics["avg_resolution_time_hours"] = avgResTime

	// Reports by status
	statusRows, err := db.Query(`
		SELECT status, COUNT(*) as count
		FROM reports
		GROUP BY status
	`)
	if err == nil {
		byStatus := make(map[string]int64)
		for statusRows.Next() {
			var s string
			var count int64
			if statusRows.Scan(&s, &count) == nil {
				byStatus[s] = count
			}
		}
		statusRows.Close()
		resolutionMetrics["by_status"] = byStatus
	}

	status["resolution"] = resolutionMetrics

	// System health
	systemHealth := make(map[string]interface{})

	// Uptime (placeholder - would need actual tracking)
	systemHealth["uptime_percent"] = 99.9
	systemHealth["last_check"] = time.Now().UTC().Format(time.RFC3339)

	// Recent errors (from activity log)
	var recentErrors int64
	db.QueryRow(`
		SELECT COUNT(*) FROM activity_log
		WHERE event_type LIKE '%error%'
		AND created_at > datetime('now', '-24 hours')
	`).Scan(&recentErrors)
	systemHealth["recent_errors"] = recentErrors

	status["system"] = systemHealth

	// SLA thresholds
	status["thresholds"] = map[string]interface{}{
		"acknowledgment_hours": 24,
		"resolution_hours":     168, // 7 days
		"uptime_percent":       99.0,
	}

	// Overall compliance
	overallCompliant := pastSLA == 0 && recentErrors < 10
	status["overall_compliant"] = overallCompliant
	if overallCompliant {
		status["status"] = "healthy"
	} else if pastSLA > 0 {
		status["status"] = "warning"
	} else {
		status["status"] = "degraded"
	}

	respondJSON(w, http.StatusOK, status)
}

// GetSLAHistory returns historical SLA data
func GetSLAHistory(w http.ResponseWriter, r *http.Request) {
	db := database.Get()

	// Daily report stats for the last 30 days
	rows, err := db.Query(`
		SELECT
			date(created_at) as day,
			COUNT(*) as total,
			SUM(CASE WHEN acknowledged_at IS NOT NULL AND (julianday(acknowledged_at) - julianday(created_at)) * 24 <= 24 THEN 1 ELSE 0 END) as within_sla
		FROM reports
		WHERE created_at > datetime('now', '-30 days')
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get SLA history")
		return
	}
	defer rows.Close()

	history := make([]map[string]interface{}, 0)
	for rows.Next() {
		var day string
		var total, withinSLA int64
		if rows.Scan(&day, &total, &withinSLA) == nil {
			compliance := 100.0
			if total > 0 {
				compliance = float64(withinSLA) / float64(total) * 100
			}
			history = append(history, map[string]interface{}{
				"date":            day,
				"total_reports":   total,
				"within_sla":      withinSLA,
				"compliance_rate": compliance,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"period":  "30 days",
	})
}
