package trust

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// PolicyStorage handles trust policy persistence
type PolicyStorage struct{}

// NewPolicyStorage creates a new policy storage
func NewPolicyStorage() *PolicyStorage {
	return &PolicyStorage{}
}

// Save saves a trust policy to the database
func (s *PolicyStorage) Save(p *TrustPolicy) error {
	db := database.Get()

	content, err := p.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	// Compute hash if not set
	hash := p.Hash
	if hash == "" {
		hash = p.ComputeHash()
	}

	var expiresAt *string
	if p.ExpiresAt != nil {
		exp := p.ExpiresAt.Format(time.RFC3339)
		expiresAt = &exp
	}

	_, err = db.Exec(`
		INSERT INTO trust_policies (
			policy_id, version, hash, content, admin_pubkey, signature,
			effective_at, expires_at, is_current
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(policy_id) DO UPDATE SET
			version = excluded.version,
			hash = excluded.hash,
			content = excluded.content,
			signature = excluded.signature,
			effective_at = excluded.effective_at,
			expires_at = excluded.expires_at
	`, p.PolicyID, p.Version, hash, string(content), p.AdminPubkey, p.Signature,
		p.EffectiveAt.Format(time.RFC3339), expiresAt)

	if err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	log.Debug().
		Str("policy_id", p.PolicyID).
		Str("version", p.Version).
		Int("curators", len(p.Allowlist)).
		Msg("Saved trust policy")

	return nil
}

// SetCurrent sets a policy as the current active policy
func (s *PolicyStorage) SetCurrent(policyID string) error {
	db := database.Get()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear current flag from all policies
	_, err = tx.Exec("UPDATE trust_policies SET is_current = 0")
	if err != nil {
		return fmt.Errorf("failed to clear current flag: %w", err)
	}

	// Set new current policy
	result, err := tx.Exec("UPDATE trust_policies SET is_current = 1 WHERE policy_id = ?", policyID)
	if err != nil {
		return fmt.Errorf("failed to set current policy: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Sync curators to curator_trust table
	if err := s.syncCurators(policyID); err != nil {
		log.Warn().Err(err).Msg("Failed to sync curators")
	}

	log.Info().Str("policy_id", policyID).Msg("Set current trust policy")
	return nil
}

// GetCurrent returns the current active policy
func (s *PolicyStorage) GetCurrent() (*TrustPolicy, error) {
	db := database.Get()

	var content string
	err := db.QueryRow(`
		SELECT content FROM trust_policies WHERE is_current = 1
	`).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get current policy: %w", err)
	}

	return TrustPolicyFromJSON([]byte(content))
}

// GetByID retrieves a policy by ID
func (s *PolicyStorage) GetByID(policyID string) (*TrustPolicy, error) {
	db := database.Get()

	var content string
	err := db.QueryRow(`
		SELECT content FROM trust_policies WHERE policy_id = ?
	`, policyID).Scan(&content)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	return TrustPolicyFromJSON([]byte(content))
}

// List returns all policies
func (s *PolicyStorage) List() ([]*TrustPolicy, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT content FROM trust_policies ORDER BY effective_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	defer rows.Close()

	var policies []*TrustPolicy
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		p, err := TrustPolicyFromJSON([]byte(content))
		if err != nil {
			continue
		}
		policies = append(policies, p)
	}

	return policies, nil
}

// Delete removes a policy
func (s *PolicyStorage) Delete(policyID string) error {
	db := database.Get()

	result, err := db.Exec("DELETE FROM trust_policies WHERE policy_id = ?", policyID)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	return nil
}

// syncCurators syncs the policy's curators to the curator_trust table
func (s *PolicyStorage) syncCurators(policyID string) error {
	policy, err := s.GetByID(policyID)
	if err != nil {
		return err
	}
	if policy == nil {
		return fmt.Errorf("policy not found")
	}

	db := database.Get()

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark all curators as pending (will be updated below)
	_, err = tx.Exec("UPDATE curator_trust SET status = 'pending'")
	if err != nil {
		return err
	}

	// Update/insert approved curators
	for _, curator := range policy.Allowlist {
		rulesets, _ := json.Marshal(curator.ApprovedRulesets)
		_, err = tx.Exec(`
			INSERT INTO curator_trust (curator_pubkey, alias, weight, status, approved_rulesets, approved_at)
			VALUES (?, ?, ?, 'approved', ?, ?)
			ON CONFLICT(curator_pubkey) DO UPDATE SET
				alias = excluded.alias,
				weight = excluded.weight,
				status = 'approved',
				approved_rulesets = excluded.approved_rulesets,
				approved_at = excluded.approved_at
		`, curator.Pubkey, curator.Alias, curator.Weight, string(rulesets), curator.AddedAt)
		if err != nil {
			return err
		}
	}

	// Mark revoked curators
	for _, revoked := range policy.Revoked {
		_, err = tx.Exec(`
			UPDATE curator_trust SET status = 'revoked', revoked_at = ?, revoke_reason = ?
			WHERE curator_pubkey = ?
		`, revoked.RevokedAt, revoked.Reason, revoked.Pubkey)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CuratorInfo represents curator info from the database
type CuratorInfo struct {
	Pubkey           string
	Alias            string
	Weight           int
	Status           string
	ApprovedRulesets []string
	ApprovedAt       *time.Time
	RevokedAt        *time.Time
	RevokeReason     string
}

// GetCurator returns curator info
func (s *PolicyStorage) GetCurator(pubkey string) (*CuratorInfo, error) {
	db := database.Get()

	var info CuratorInfo
	var approvedRulesets sql.NullString
	var approvedAt, revokedAt sql.NullString
	var revokeReason sql.NullString

	err := db.QueryRow(`
		SELECT curator_pubkey, alias, weight, status, approved_rulesets,
			   approved_at, revoked_at, revoke_reason
		FROM curator_trust WHERE curator_pubkey = ?
	`, pubkey).Scan(&info.Pubkey, &info.Alias, &info.Weight, &info.Status,
		&approvedRulesets, &approvedAt, &revokedAt, &revokeReason)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if approvedRulesets.Valid {
		json.Unmarshal([]byte(approvedRulesets.String), &info.ApprovedRulesets)
	}
	if approvedAt.Valid {
		t, _ := time.Parse(time.RFC3339, approvedAt.String)
		info.ApprovedAt = &t
	}
	if revokedAt.Valid {
		t, _ := time.Parse(time.RFC3339, revokedAt.String)
		info.RevokedAt = &t
	}
	if revokeReason.Valid {
		info.RevokeReason = revokeReason.String
	}

	return &info, nil
}

// ListCurators returns all curators
func (s *PolicyStorage) ListCurators() ([]*CuratorInfo, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT curator_pubkey, alias, weight, status, approved_rulesets,
			   approved_at, revoked_at, revoke_reason
		FROM curator_trust ORDER BY status, alias
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var curators []*CuratorInfo
	for rows.Next() {
		var info CuratorInfo
		var approvedRulesets sql.NullString
		var approvedAt, revokedAt sql.NullString
		var revokeReason sql.NullString

		err := rows.Scan(&info.Pubkey, &info.Alias, &info.Weight, &info.Status,
			&approvedRulesets, &approvedAt, &revokedAt, &revokeReason)
		if err != nil {
			return nil, err
		}

		if approvedRulesets.Valid {
			json.Unmarshal([]byte(approvedRulesets.String), &info.ApprovedRulesets)
		}
		if approvedAt.Valid {
			t, _ := time.Parse(time.RFC3339, approvedAt.String)
			info.ApprovedAt = &t
		}
		if revokedAt.Valid {
			t, _ := time.Parse(time.RFC3339, revokedAt.String)
			info.RevokedAt = &t
		}
		if revokeReason.Valid {
			info.RevokeReason = revokeReason.String
		}

		curators = append(curators, &info)
	}

	return curators, nil
}

// IsCuratorApproved checks if a curator is approved
func (s *PolicyStorage) IsCuratorApproved(pubkey string) (bool, error) {
	db := database.Get()

	var status string
	err := db.QueryRow(`
		SELECT status FROM curator_trust WHERE curator_pubkey = ?
	`, pubkey).Scan(&status)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return status == "approved", nil
}

// GetApprovedCurators returns all approved curators
func (s *PolicyStorage) GetApprovedCurators() ([]string, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT curator_pubkey FROM curator_trust WHERE status = 'approved'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var curators []string
	for rows.Next() {
		var pubkey string
		if err := rows.Scan(&pubkey); err != nil {
			return nil, err
		}
		curators = append(curators, pubkey)
	}

	return curators, nil
}
