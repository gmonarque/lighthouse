package ruleset

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// Storage handles ruleset persistence
type Storage struct{}

// NewStorage creates a new ruleset storage
func NewStorage() *Storage {
	return &Storage{}
}

// Save saves a ruleset to the database
func (s *Storage) Save(r *Ruleset) error {
	db := database.Get()

	content, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal ruleset: %w", err)
	}

	// Compute hash if not set
	hash := r.Hash
	if hash == "" {
		hash = r.ComputeHash()
	}

	_, err = db.Exec(`
		INSERT INTO rulesets (ruleset_id, type, version, hash, content, source, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(ruleset_id) DO UPDATE SET
			version = excluded.version,
			hash = excluded.hash,
			content = excluded.content,
			source = excluded.source
	`, r.ID, r.Type, r.Version, hash, string(content), "", false)

	if err != nil {
		return fmt.Errorf("failed to save ruleset: %w", err)
	}

	log.Debug().
		Str("id", r.ID).
		Str("type", string(r.Type)).
		Str("version", r.Version).
		Msg("Saved ruleset to database")

	return nil
}

// GetByID retrieves a ruleset by ID
func (s *Storage) GetByID(id string) (*Ruleset, error) {
	db := database.Get()

	var content string
	err := db.QueryRow(`
		SELECT content FROM rulesets WHERE ruleset_id = ?
	`, id).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ruleset: %w", err)
	}

	var ruleset Ruleset
	if err := json.Unmarshal([]byte(content), &ruleset); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ruleset: %w", err)
	}

	return &ruleset, nil
}

// GetByHash retrieves a ruleset by hash
func (s *Storage) GetByHash(hash string) (*Ruleset, error) {
	db := database.Get()

	var content string
	err := db.QueryRow(`
		SELECT content FROM rulesets WHERE hash = ?
	`, hash).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ruleset: %w", err)
	}

	var ruleset Ruleset
	if err := json.Unmarshal([]byte(content), &ruleset); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ruleset: %w", err)
	}

	return &ruleset, nil
}

// GetActive retrieves the active ruleset of a given type
func (s *Storage) GetActive(rulesetType RulesetType) (*Ruleset, error) {
	db := database.Get()

	var content string
	err := db.QueryRow(`
		SELECT content FROM rulesets WHERE type = ? AND is_active = 1
	`, rulesetType).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active ruleset: %w", err)
	}

	var ruleset Ruleset
	if err := json.Unmarshal([]byte(content), &ruleset); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ruleset: %w", err)
	}

	return &ruleset, nil
}

// SetActive sets a ruleset as active (deactivates others of the same type)
func (s *Storage) SetActive(id string) error {
	db := database.Get()

	// Get the ruleset type
	var rulesetType string
	err := db.QueryRow(`SELECT type FROM rulesets WHERE ruleset_id = ?`, id).Scan(&rulesetType)
	if err == sql.ErrNoRows {
		return fmt.Errorf("ruleset not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("failed to get ruleset type: %w", err)
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all rulesets of the same type
	_, err = tx.Exec(`UPDATE rulesets SET is_active = 0 WHERE type = ?`, rulesetType)
	if err != nil {
		return fmt.Errorf("failed to deactivate rulesets: %w", err)
	}

	// Activate the specified ruleset
	_, err = tx.Exec(`UPDATE rulesets SET is_active = 1 WHERE ruleset_id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to activate ruleset: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("id", id).
		Str("type", rulesetType).
		Msg("Activated ruleset")

	return nil
}

// Deprecate marks a ruleset as deprecated
func (s *Storage) Deprecate(id string) error {
	db := database.Get()

	result, err := db.Exec(`
		UPDATE rulesets SET deprecated_at = CURRENT_TIMESTAMP, is_active = 0
		WHERE ruleset_id = ?
	`, id)

	if err != nil {
		return fmt.Errorf("failed to deprecate ruleset: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("ruleset not found: %s", id)
	}

	log.Info().Str("id", id).Msg("Deprecated ruleset")
	return nil
}

// Delete removes a ruleset from the database
func (s *Storage) Delete(id string) error {
	db := database.Get()

	result, err := db.Exec(`DELETE FROM rulesets WHERE ruleset_id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete ruleset: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("ruleset not found: %s", id)
	}

	log.Info().Str("id", id).Msg("Deleted ruleset")
	return nil
}

// List returns all rulesets
func (s *Storage) List() ([]RulesetDescriptor, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT ruleset_id, type, version, hash, source, is_active, created_at
		FROM rulesets
		ORDER BY type, created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list rulesets: %w", err)
	}
	defer rows.Close()

	var descriptors []RulesetDescriptor
	for rows.Next() {
		var d RulesetDescriptor
		var source sql.NullString
		var createdAt string

		err := rows.Scan(&d.RulesetID, &d.Type, &d.Version, &d.Hash, &source, &d.IsActive, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		d.Source = source.String
		d.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		descriptors = append(descriptors, d)
	}

	return descriptors, nil
}

// ListByType returns all rulesets of a given type
func (s *Storage) ListByType(rulesetType RulesetType) ([]RulesetDescriptor, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT ruleset_id, type, version, hash, source, is_active, created_at
		FROM rulesets
		WHERE type = ?
		ORDER BY created_at DESC
	`, rulesetType)
	if err != nil {
		return nil, fmt.Errorf("failed to list rulesets: %w", err)
	}
	defer rows.Close()

	var descriptors []RulesetDescriptor
	for rows.Next() {
		var d RulesetDescriptor
		var source sql.NullString
		var createdAt string

		err := rows.Scan(&d.RulesetID, &d.Type, &d.Version, &d.Hash, &source, &d.IsActive, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		d.Source = source.String
		d.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		descriptors = append(descriptors, d)
	}

	return descriptors, nil
}

// IsHashApproved checks if a ruleset hash is in the list of approved hashes
func (s *Storage) IsHashApproved(hash string) (bool, error) {
	db := database.Get()

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM rulesets WHERE hash = ? AND deprecated_at IS NULL
	`, hash).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check hash: %w", err)
	}

	return count > 0, nil
}

// InitDefaults initializes default rulesets if none exist
func (s *Storage) InitDefaults() error {
	// Check if any rulesets exist
	descriptors, err := s.List()
	if err != nil {
		return err
	}

	if len(descriptors) > 0 {
		return nil // Already have rulesets
	}

	// Create default censoring ruleset
	censoring := CreateDefaultCensoringRuleset()
	if err := s.Save(censoring); err != nil {
		return fmt.Errorf("failed to save default censoring ruleset: %w", err)
	}
	if err := s.SetActive(censoring.ID); err != nil {
		return fmt.Errorf("failed to activate default censoring ruleset: %w", err)
	}

	// Create default semantic ruleset
	semantic := CreateDefaultSemanticRuleset()
	if err := s.Save(semantic); err != nil {
		return fmt.Errorf("failed to save default semantic ruleset: %w", err)
	}
	if err := s.SetActive(semantic.ID); err != nil {
		return fmt.Errorf("failed to activate default semantic ruleset: %w", err)
	}

	log.Info().Msg("Initialized default rulesets")
	return nil
}
