package moderation

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// Storage handles report persistence
type Storage struct{}

// NewStorage creates a new moderation storage
func NewStorage() *Storage {
	return &Storage{}
}

// Save saves a report to the database
func (s *Storage) Save(r *Report) error {
	db := database.Get()

	content, err := r.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	var acknowledgedAt, resolvedAt *string
	if r.AcknowledgedAt != nil {
		t := r.AcknowledgedAt.Format(time.RFC3339)
		acknowledgedAt = &t
	}
	if r.ResolvedAt != nil {
		t := r.ResolvedAt.Format(time.RFC3339)
		resolvedAt = &t
	}

	_, err = db.Exec(`
		INSERT INTO reports (
			report_id, kind, target_event_id, target_infohash, category,
			evidence, scope, jurisdiction, reporter_pubkey, reporter_contact,
			signature, status, resolution, created_at, acknowledged_at,
			resolved_at, resolved_by, content
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(report_id) DO UPDATE SET
			status = excluded.status,
			resolution = excluded.resolution,
			acknowledged_at = excluded.acknowledged_at,
			resolved_at = excluded.resolved_at,
			resolved_by = excluded.resolved_by,
			content = excluded.content
	`, r.ReportID, string(r.Kind), r.TargetEventID, r.TargetInfohash, string(r.Category),
		r.Evidence, r.Scope, r.Jurisdiction, r.ReporterPubkey, r.ReporterContact,
		r.Signature, string(r.Status), r.Resolution, r.CreatedAt.Format(time.RFC3339),
		acknowledgedAt, resolvedAt, r.ResolvedBy, string(content))

	if err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}

	log.Debug().
		Str("report_id", r.ReportID).
		Str("kind", string(r.Kind)).
		Str("status", string(r.Status)).
		Msg("Saved report")

	return nil
}

// GetByID retrieves a report by ID
func (s *Storage) GetByID(reportID string) (*Report, error) {
	db := database.Get()

	var content string
	err := db.QueryRow(`
		SELECT content FROM reports WHERE report_id = ?
	`, reportID).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	return ReportFromJSON([]byte(content))
}

// GetByInfohash retrieves all reports for an infohash
func (s *Storage) GetByInfohash(infohash string) ([]*Report, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT content FROM reports WHERE target_infohash = ? ORDER BY created_at DESC
	`, infohash)
	if err != nil {
		return nil, fmt.Errorf("failed to query reports: %w", err)
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		r, err := ReportFromJSON([]byte(content))
		if err != nil {
			continue
		}
		reports = append(reports, r)
	}

	return reports, nil
}

// GetByEventID retrieves all reports for an event ID
func (s *Storage) GetByEventID(eventID string) ([]*Report, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT content FROM reports WHERE target_event_id = ? ORDER BY created_at DESC
	`, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query reports: %w", err)
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		r, err := ReportFromJSON([]byte(content))
		if err != nil {
			continue
		}
		reports = append(reports, r)
	}

	return reports, nil
}

// Query reports with filter
func (s *Storage) Query(filter *ReportFilter) ([]*Report, error) {
	db := database.Get()

	query := "SELECT content FROM reports WHERE 1=1"
	args := []interface{}{}

	if filter.Kind != "" {
		query += " AND kind = ?"
		args = append(args, string(filter.Kind))
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	if filter.Category != "" {
		query += " AND category = ?"
		args = append(args, string(filter.Category))
	}
	if filter.TargetInfohash != "" {
		query += " AND target_infohash = ?"
		args = append(args, filter.TargetInfohash)
	}
	if filter.ReporterPubkey != "" {
		query += " AND reporter_pubkey = ?"
		args = append(args, filter.ReporterPubkey)
	}
	if filter.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, filter.Since.Format(time.RFC3339))
	}
	if filter.Until != nil {
		query += " AND created_at <= ?"
		args = append(args, filter.Until.Format(time.RFC3339))
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query reports: %w", err)
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		r, err := ReportFromJSON([]byte(content))
		if err != nil {
			continue
		}
		reports = append(reports, r)
	}

	return reports, nil
}

// GetPending returns all pending reports
func (s *Storage) GetPending() ([]*Report, error) {
	return s.Query(&ReportFilter{Status: StatusPending})
}

// GetOpen returns all open reports (pending, acknowledged, investigating)
func (s *Storage) GetOpen() ([]*Report, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT content FROM reports
		WHERE status IN ('pending', 'acknowledged', 'investigating')
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query reports: %w", err)
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		r, err := ReportFromJSON([]byte(content))
		if err != nil {
			continue
		}
		reports = append(reports, r)
	}

	return reports, nil
}

// GetNeedingAcknowledgment returns reports that need acknowledgment (>24h old)
func (s *Storage) GetNeedingAcknowledgment(deadline time.Duration) ([]*Report, error) {
	db := database.Get()

	cutoff := time.Now().UTC().Add(-deadline).Format(time.RFC3339)

	rows, err := db.Query(`
		SELECT content FROM reports
		WHERE status = 'pending' AND created_at < ?
		ORDER BY created_at ASC
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query reports: %w", err)
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		r, err := ReportFromJSON([]byte(content))
		if err != nil {
			continue
		}
		reports = append(reports, r)
	}

	return reports, nil
}

// UpdateStatus updates a report's status
func (s *Storage) UpdateStatus(reportID string, status ReportStatus, resolution, resolvedBy string) error {
	r, err := s.GetByID(reportID)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("report not found: %s", reportID)
	}

	switch status {
	case StatusAcknowledged:
		r.Acknowledge()
	case StatusInvestigating:
		r.StartInvestigation()
	case StatusResolved:
		r.Resolve(resolution, resolvedBy)
	case StatusRejected:
		r.Reject(resolution, resolvedBy)
	default:
		r.Status = status
	}

	return s.Save(r)
}

// GetStats returns report statistics
func (s *Storage) GetStats() (*ReportStats, error) {
	db := database.Get()

	stats := &ReportStats{}

	// Count by status
	err := db.QueryRow(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'acknowledged' THEN 1 ELSE 0 END) as acknowledged,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved,
			SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END) as rejected,
			SUM(CASE WHEN kind = 'appeal' THEN 1 ELSE 0 END) as appeals
		FROM reports
	`).Scan(&stats.TotalReports, &stats.PendingReports, &stats.AcknowledgedCount,
		&stats.ResolvedCount, &stats.RejectedCount, &stats.TotalAppeals)

	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Calculate average resolution time for resolved reports
	var avgSeconds sql.NullFloat64
	err = db.QueryRow(`
		SELECT AVG(
			CAST((julianday(resolved_at) - julianday(created_at)) * 86400 AS REAL)
		) FROM reports
		WHERE status IN ('resolved', 'rejected') AND resolved_at IS NOT NULL
	`).Scan(&avgSeconds)

	if err == nil && avgSeconds.Valid {
		duration := time.Duration(avgSeconds.Float64) * time.Second
		stats.AvgResolutionTime = duration.String()
	}

	return stats, nil
}

// Delete removes a report
func (s *Storage) Delete(reportID string) error {
	db := database.Get()

	result, err := db.Exec("DELETE FROM reports WHERE report_id = ?", reportID)
	if err != nil {
		return fmt.Errorf("failed to delete report: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("report not found: %s", reportID)
	}

	return nil
}

// HasOpenReport checks if there's an open report for an infohash
func (s *Storage) HasOpenReport(infohash string) (bool, error) {
	db := database.Get()

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM reports
		WHERE target_infohash = ? AND status IN ('pending', 'acknowledged', 'investigating')
	`, infohash).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateAppeal creates an appeal for a rejected report
func (s *Storage) CreateAppeal(originalReportID string, evidence string, reporterPubkey string) (*Report, error) {
	// Get original report
	original, err := s.GetByID(originalReportID)
	if err != nil {
		return nil, err
	}
	if original == nil {
		return nil, fmt.Errorf("original report not found: %s", originalReportID)
	}
	if original.Status != StatusResolved && original.Status != StatusRejected {
		return nil, fmt.Errorf("can only appeal resolved or rejected reports")
	}

	// Create appeal
	appeal := NewReport(ReportKindAppeal, original.Category, reporterPubkey)
	appeal.TargetEventID = original.TargetEventID
	appeal.TargetInfohash = original.TargetInfohash
	appeal.Evidence = evidence
	appeal.Scope = fmt.Sprintf("Appeal of report %s", originalReportID)

	if err := s.Save(appeal); err != nil {
		return nil, err
	}

	log.Info().
		Str("appeal_id", appeal.ReportID).
		Str("original_id", originalReportID).
		Msg("Created appeal")

	return appeal, nil
}
