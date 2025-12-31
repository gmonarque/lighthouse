package apikeys

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// Storage handles API key persistence
type Storage struct{}

// NewStorage creates a new API key storage
func NewStorage() *Storage {
	return &Storage{}
}

// Init initializes the API keys table
func (s *Storage) Init() error {
	db := database.Get()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			key_prefix TEXT NOT NULL,
			permissions TEXT NOT NULL,
			rate_limit INTEGER DEFAULT 0,
			created_by TEXT,
			created_at TEXT NOT NULL,
			last_used_at TEXT,
			expires_at TEXT,
			enabled INTEGER DEFAULT 1,
			notes TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create api_keys table: %w", err)
	}

	// Create index on key_hash for fast lookups
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash)`)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	log.Debug().Msg("API keys table initialized")
	return nil
}

// Save saves an API key
func (s *Storage) Save(key *APIKey) error {
	db := database.Get()

	permsJSON, err := json.Marshal(key.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	var lastUsed, expires *string
	if key.LastUsedAt != nil {
		t := key.LastUsedAt.Format(time.RFC3339)
		lastUsed = &t
	}
	if key.ExpiresAt != nil {
		t := key.ExpiresAt.Format(time.RFC3339)
		expires = &t
	}

	enabled := 0
	if key.Enabled {
		enabled = 1
	}

	_, err = db.Exec(`
		INSERT INTO api_keys (
			id, name, key_hash, key_prefix, permissions, rate_limit,
			created_by, created_at, last_used_at, expires_at, enabled, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			permissions = excluded.permissions,
			rate_limit = excluded.rate_limit,
			last_used_at = excluded.last_used_at,
			expires_at = excluded.expires_at,
			enabled = excluded.enabled,
			notes = excluded.notes
	`, key.ID, key.Name, key.KeyHash, key.KeyPrefix, string(permsJSON),
		key.RateLimit, key.CreatedBy, key.CreatedAt.Format(time.RFC3339),
		lastUsed, expires, enabled, key.Notes)

	if err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	log.Debug().Str("id", key.ID).Str("name", key.Name).Msg("Saved API key")
	return nil
}

// GetByID retrieves an API key by ID
func (s *Storage) GetByID(id string) (*APIKey, error) {
	db := database.Get()

	var key APIKey
	var permsJSON string
	var lastUsed, expires sql.NullString
	var enabled int

	err := db.QueryRow(`
		SELECT id, name, key_hash, key_prefix, permissions, rate_limit,
		       created_by, created_at, last_used_at, expires_at, enabled, notes
		FROM api_keys WHERE id = ?
	`, id).Scan(
		&key.ID, &key.Name, &key.KeyHash, &key.KeyPrefix, &permsJSON,
		&key.RateLimit, &key.CreatedBy, &key.CreatedAt, &lastUsed, &expires,
		&enabled, &key.Notes,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	json.Unmarshal([]byte(permsJSON), &key.Permissions)

	if lastUsed.Valid {
		t, _ := time.Parse(time.RFC3339, lastUsed.String)
		key.LastUsedAt = &t
	}
	if expires.Valid {
		t, _ := time.Parse(time.RFC3339, expires.String)
		key.ExpiresAt = &t
	}
	key.Enabled = enabled == 1

	return &key, nil
}

// GetByHash retrieves an API key by its hash (for authentication)
func (s *Storage) GetByHash(keyHash string) (*APIKey, error) {
	db := database.Get()

	var key APIKey
	var permsJSON string
	var lastUsed, expires sql.NullString
	var enabled int
	var createdAt string

	err := db.QueryRow(`
		SELECT id, name, key_hash, key_prefix, permissions, rate_limit,
		       created_by, created_at, last_used_at, expires_at, enabled, notes
		FROM api_keys WHERE key_hash = ?
	`, keyHash).Scan(
		&key.ID, &key.Name, &key.KeyHash, &key.KeyPrefix, &permsJSON,
		&key.RateLimit, &key.CreatedBy, &createdAt, &lastUsed, &expires,
		&enabled, &key.Notes,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	json.Unmarshal([]byte(permsJSON), &key.Permissions)

	key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastUsed.Valid {
		t, _ := time.Parse(time.RFC3339, lastUsed.String)
		key.LastUsedAt = &t
	}
	if expires.Valid {
		t, _ := time.Parse(time.RFC3339, expires.String)
		key.ExpiresAt = &t
	}
	key.Enabled = enabled == 1

	return &key, nil
}

// ValidateKey validates a plaintext key and returns the key if valid
func (s *Storage) ValidateKey(plaintextKey string) (*APIKey, error) {
	keyHash := HashKey(plaintextKey)
	key, err := s.GetByHash(keyHash)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}
	if !key.IsValid() {
		return nil, nil
	}

	// Update last used
	key.UpdateLastUsed()
	go s.updateLastUsed(key.ID) // Async update

	return key, nil
}

// updateLastUsed updates the last used timestamp
func (s *Storage) updateLastUsed(id string) {
	db := database.Get()
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec("UPDATE api_keys SET last_used_at = ? WHERE id = ?", now, id)
}

// List returns all API keys (without sensitive data)
func (s *Storage) List() ([]*APIKey, error) {
	db := database.Get()

	rows, err := db.Query(`
		SELECT id, name, key_hash, key_prefix, permissions, rate_limit,
		       created_by, created_at, last_used_at, expires_at, enabled, notes
		FROM api_keys ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		var permsJSON string
		var lastUsed, expires sql.NullString
		var enabled int
		var createdAt string

		err := rows.Scan(
			&key.ID, &key.Name, &key.KeyHash, &key.KeyPrefix, &permsJSON,
			&key.RateLimit, &key.CreatedBy, &createdAt, &lastUsed, &expires,
			&enabled, &key.Notes,
		)
		if err != nil {
			continue
		}

		json.Unmarshal([]byte(permsJSON), &key.Permissions)
		key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if lastUsed.Valid {
			t, _ := time.Parse(time.RFC3339, lastUsed.String)
			key.LastUsedAt = &t
		}
		if expires.Valid {
			t, _ := time.Parse(time.RFC3339, expires.String)
			key.ExpiresAt = &t
		}
		key.Enabled = enabled == 1

		// Clear hash before returning
		key.KeyHash = ""
		keys = append(keys, &key)
	}

	return keys, nil
}

// Delete removes an API key
func (s *Storage) Delete(id string) error {
	db := database.Get()

	result, err := db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API key not found: %s", id)
	}

	log.Info().Str("id", id).Msg("Deleted API key")
	return nil
}

// Disable disables an API key
func (s *Storage) Disable(id string) error {
	db := database.Get()

	result, err := db.Exec("UPDATE api_keys SET enabled = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to disable API key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API key not found: %s", id)
	}

	return nil
}

// Enable enables an API key
func (s *Storage) Enable(id string) error {
	db := database.Get()

	result, err := db.Exec("UPDATE api_keys SET enabled = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to enable API key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API key not found: %s", id)
	}

	return nil
}

// Count returns the number of API keys
func (s *Storage) Count() (int, error) {
	db := database.Get()

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// HasKeys checks if any API keys exist
func (s *Storage) HasKeys() (bool, error) {
	count, err := s.Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
