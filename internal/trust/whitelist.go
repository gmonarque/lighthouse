package trust

import (
	"github.com/lighthouse-client/lighthouse/internal/database"
	"github.com/lighthouse-client/lighthouse/internal/models"
)

// Whitelist manages the trust whitelist
type Whitelist struct{}

// NewWhitelist creates a new Whitelist manager
func NewWhitelist() *Whitelist {
	return &Whitelist{}
}

// Add adds an npub to the whitelist
func (w *Whitelist) Add(npub, alias, notes string) error {
	db := database.Get()
	_, err := db.Exec(`
		INSERT INTO trust_whitelist (npub, alias, notes)
		VALUES (?, ?, ?)
		ON CONFLICT(npub) DO UPDATE SET
			alias = excluded.alias,
			notes = excluded.notes
	`, npub, alias, notes)

	if err == nil {
		database.LogActivity(models.ActivityWhitelistAdd, npub)
	}
	return err
}

// Remove removes an npub from the whitelist
func (w *Whitelist) Remove(npub string) error {
	db := database.Get()
	_, err := db.Exec(`DELETE FROM trust_whitelist WHERE npub = ?`, npub)

	if err == nil {
		database.LogActivity(models.ActivityWhitelistRemove, npub)
	}
	return err
}

// Contains checks if an npub is in the whitelist
func (w *Whitelist) Contains(npub string) bool {
	db := database.Get()
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM trust_whitelist WHERE npub = ?`, npub).Scan(&count)
	return err == nil && count > 0
}

// GetAll returns all whitelist entries
func (w *Whitelist) GetAll() ([]models.TrustWhitelistEntry, error) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT id, npub, alias, notes, added_at
		FROM trust_whitelist
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.TrustWhitelistEntry
	for rows.Next() {
		var entry models.TrustWhitelistEntry
		var alias, notes *string
		if err := rows.Scan(&entry.ID, &entry.Npub, &alias, &notes, &entry.AddedAt); err != nil {
			continue
		}
		if alias != nil {
			entry.Alias = *alias
		}
		if notes != nil {
			entry.Notes = *notes
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Count returns the number of whitelist entries
func (w *Whitelist) Count() int {
	db := database.Get()
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM trust_whitelist`).Scan(&count)
	return count
}
