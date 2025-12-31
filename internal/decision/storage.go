package decision

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/gmonarque/lighthouse/internal/ruleset"
	"github.com/rs/zerolog/log"
)

// Storage handles verification decision persistence
type Storage struct{}

// NewStorage creates a new decision storage
func NewStorage() *Storage {
	return &Storage{}
}

// Save stores a verification decision
func (s *Storage) Save(d *VerificationDecision) error {
	db := database.Get()

	reasonCodes, err := json.Marshal(d.ReasonCodes)
	if err != nil {
		return fmt.Errorf("failed to marshal reason codes: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO verification_decisions (
			decision_id, target_event_id, target_infohash, decision,
			reason_codes, ruleset_type, ruleset_version, ruleset_hash,
			curator_pubkey, signature, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(decision_id) DO UPDATE SET
			processed_at = CURRENT_TIMESTAMP
	`, d.DecisionID, d.TargetEventID, d.TargetInfohash, d.Decision,
		string(reasonCodes), d.RulesetType, d.RulesetVersion, d.RulesetHash,
		d.CuratorPubkey, d.Signature, d.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save decision: %w", err)
	}

	log.Debug().
		Str("decision_id", d.DecisionID).
		Str("infohash", d.TargetInfohash).
		Str("decision", string(d.Decision)).
		Msg("Saved verification decision")

	return nil
}

// GetByID retrieves a decision by its ID
func (s *Storage) GetByID(decisionID string) (*VerificationDecision, error) {
	db := database.Get()

	var d VerificationDecision
	var reasonCodesJSON string
	var processedAt sql.NullString
	var aggregatedDecision sql.NullString

	err := db.QueryRow(`
		SELECT decision_id, target_event_id, target_infohash, decision,
			   reason_codes, ruleset_type, ruleset_version, ruleset_hash,
			   curator_pubkey, signature, created_at, processed_at, aggregated_decision
		FROM verification_decisions
		WHERE decision_id = ?
	`, decisionID).Scan(
		&d.DecisionID, &d.TargetEventID, &d.TargetInfohash, &d.Decision,
		&reasonCodesJSON, &d.RulesetType, &d.RulesetVersion, &d.RulesetHash,
		&d.CuratorPubkey, &d.Signature, &d.CreatedAt, &processedAt, &aggregatedDecision,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get decision: %w", err)
	}

	if reasonCodesJSON != "" {
		json.Unmarshal([]byte(reasonCodesJSON), &d.ReasonCodes)
	}

	if processedAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
		d.ProcessedAt = &t
	}

	if aggregatedDecision.Valid {
		dec := Decision(aggregatedDecision.String)
		d.AggregatedDecision = &dec
	}

	return &d, nil
}

// GetByInfohash retrieves all decisions for an infohash
func (s *Storage) GetByInfohash(infohash string) ([]*VerificationDecision, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT decision_id, target_event_id, target_infohash, decision,
			   reason_codes, ruleset_type, ruleset_version, ruleset_hash,
			   curator_pubkey, signature, created_at, processed_at, aggregated_decision
		FROM verification_decisions
		WHERE target_infohash = ?
		ORDER BY created_at DESC
	`, infohash)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer rows.Close()

	return s.scanDecisions(rows)
}

// GetByCurator retrieves all decisions by a curator
func (s *Storage) GetByCurator(curatorPubkey string, limit int) ([]*VerificationDecision, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT decision_id, target_event_id, target_infohash, decision,
			   reason_codes, ruleset_type, ruleset_version, ruleset_hash,
			   curator_pubkey, signature, created_at, processed_at, aggregated_decision
		FROM verification_decisions
		WHERE curator_pubkey = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, curatorPubkey, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer rows.Close()

	return s.scanDecisions(rows)
}

// Query retrieves decisions based on filter criteria
func (s *Storage) Query(filter DecisionFilter) ([]*VerificationDecision, error) {
	db := database.Get()

	query := `
		SELECT decision_id, target_event_id, target_infohash, decision,
			   reason_codes, ruleset_type, ruleset_version, ruleset_hash,
			   curator_pubkey, signature, created_at, processed_at, aggregated_decision
		FROM verification_decisions
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.TargetInfohash != "" {
		query += " AND target_infohash = ?"
		args = append(args, filter.TargetInfohash)
	}
	if filter.CuratorPubkey != "" {
		query += " AND curator_pubkey = ?"
		args = append(args, filter.CuratorPubkey)
	}
	if filter.Decision != "" {
		query += " AND decision = ?"
		args = append(args, filter.Decision)
	}
	if filter.RulesetHash != "" {
		query += " AND ruleset_hash = ?"
		args = append(args, filter.RulesetHash)
	}
	if filter.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, filter.Since)
	}
	if filter.Until != nil {
		query += " AND created_at <= ?"
		args = append(args, filter.Until)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer rows.Close()

	return s.scanDecisions(rows)
}

// scanDecisions scans rows into decision objects
func (s *Storage) scanDecisions(rows *sql.Rows) ([]*VerificationDecision, error) {
	var decisions []*VerificationDecision

	for rows.Next() {
		var d VerificationDecision
		var reasonCodesJSON string
		var processedAt sql.NullString
		var aggregatedDecision sql.NullString

		err := rows.Scan(
			&d.DecisionID, &d.TargetEventID, &d.TargetInfohash, &d.Decision,
			&reasonCodesJSON, &d.RulesetType, &d.RulesetVersion, &d.RulesetHash,
			&d.CuratorPubkey, &d.Signature, &d.CreatedAt, &processedAt, &aggregatedDecision,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if reasonCodesJSON != "" {
			json.Unmarshal([]byte(reasonCodesJSON), &d.ReasonCodes)
		}

		if processedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
			d.ProcessedAt = &t
		}

		if aggregatedDecision.Valid {
			dec := Decision(aggregatedDecision.String)
			d.AggregatedDecision = &dec
		}

		decisions = append(decisions, &d)
	}

	return decisions, nil
}

// GetSummary gets a summary of decisions for an infohash
func (s *Storage) GetSummary(infohash string) (*DecisionSummary, error) {
	decisions, err := s.GetByInfohash(infohash)
	if err != nil {
		return nil, err
	}

	summary := &DecisionSummary{
		Infohash:       infohash,
		TotalDecisions: len(decisions),
	}

	for _, d := range decisions {
		if d.Decision == DecisionAccept {
			summary.AcceptCount++
		} else {
			summary.RejectCount++
			if d.HasLegalCode() {
				summary.HasLegalReject = true
			}
		}
	}

	// Determine final decision
	if summary.HasLegalReject {
		summary.FinalDecision = DecisionReject
	} else if summary.AcceptCount > summary.RejectCount {
		summary.FinalDecision = DecisionAccept
	} else if summary.RejectCount > 0 {
		summary.FinalDecision = DecisionReject
	} else {
		summary.FinalDecision = DecisionAccept
	}

	return summary, nil
}

// UpdateAggregatedDecision updates the aggregated decision for an infohash
func (s *Storage) UpdateAggregatedDecision(infohash string, decision Decision) error {
	db := database.Get()

	_, err := db.Exec(`
		UPDATE verification_decisions
		SET aggregated_decision = ?, processed_at = CURRENT_TIMESTAMP
		WHERE target_infohash = ?
	`, decision, infohash)

	if err != nil {
		return fmt.Errorf("failed to update aggregated decision: %w", err)
	}

	return nil
}

// GetPendingAggregation returns infohashes that need aggregation
func (s *Storage) GetPendingAggregation(limit int) ([]string, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT DISTINCT target_infohash
		FROM verification_decisions
		WHERE aggregated_decision IS NULL
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending: %w", err)
	}
	defer rows.Close()

	var infohashes []string
	for rows.Next() {
		var infohash string
		if err := rows.Scan(&infohash); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		infohashes = append(infohashes, infohash)
	}

	return infohashes, nil
}

// GetStats returns decision statistics
func (s *Storage) GetStats() (map[string]interface{}, error) {
	db := database.Get()

	stats := make(map[string]interface{})

	// Total decisions
	var total int64
	db.QueryRow("SELECT COUNT(*) FROM verification_decisions").Scan(&total)
	stats["total_decisions"] = total

	// By decision type
	var accepts, rejects int64
	db.QueryRow("SELECT COUNT(*) FROM verification_decisions WHERE decision = 'accept'").Scan(&accepts)
	db.QueryRow("SELECT COUNT(*) FROM verification_decisions WHERE decision = 'reject'").Scan(&rejects)
	stats["accept_count"] = accepts
	stats["reject_count"] = rejects

	// Unique curators
	var curators int64
	db.QueryRow("SELECT COUNT(DISTINCT curator_pubkey) FROM verification_decisions").Scan(&curators)
	stats["unique_curators"] = curators

	// Decisions today
	var today int64
	db.QueryRow("SELECT COUNT(*) FROM verification_decisions WHERE DATE(created_at) = DATE('now')").Scan(&today)
	stats["decisions_today"] = today

	return stats, nil
}

// DeleteByCurator removes all decisions from a curator
func (s *Storage) DeleteByCurator(curatorPubkey string) (int64, error) {
	db := database.Get()

	result, err := db.Exec(`
		DELETE FROM verification_decisions WHERE curator_pubkey = ?
	`, curatorPubkey)
	if err != nil {
		return 0, fmt.Errorf("failed to delete decisions: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Info().
		Str("curator", curatorPubkey).
		Int64("deleted", rows).
		Msg("Deleted curator decisions")

	return rows, nil
}

// HasDecisionFrom checks if a curator has already made a decision on an infohash
func (s *Storage) HasDecisionFrom(curatorPubkey, infohash string) (bool, error) {
	db := database.Get()

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM verification_decisions
		WHERE curator_pubkey = ? AND target_infohash = ?
	`, curatorPubkey, infohash).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check decision: %w", err)
	}

	return count > 0, nil
}

// GetRecentByReason returns recent decisions with a specific reason code
func (s *Storage) GetRecentByReason(reason ruleset.ReasonCode, limit int) ([]*VerificationDecision, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT decision_id, target_event_id, target_infohash, decision,
			   reason_codes, ruleset_type, ruleset_version, ruleset_hash,
			   curator_pubkey, signature, created_at, processed_at, aggregated_decision
		FROM verification_decisions
		WHERE reason_codes LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`, "%"+string(reason)+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer rows.Close()

	return s.scanDecisions(rows)
}
