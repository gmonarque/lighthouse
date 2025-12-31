package trust

import (
	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/gmonarque/lighthouse/internal/models"
	"github.com/rs/zerolog/log"
)

// Blacklist manages the trust blacklist
type Blacklist struct{}

// NewBlacklist creates a new Blacklist manager
func NewBlacklist() *Blacklist {
	return &Blacklist{}
}

// Add adds an npub to the blacklist and removes all their content
func (b *Blacklist) Add(npub, reason string) error {
	db := database.Get()

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Add to blacklist
	_, err = tx.Exec(`
		INSERT INTO trust_blacklist (npub, reason)
		VALUES (?, ?)
		ON CONFLICT(npub) DO UPDATE SET reason = excluded.reason
	`, npub, reason)
	if err != nil {
		return err
	}

	// Remove from whitelist if present
	tx.Exec(`DELETE FROM trust_whitelist WHERE npub = ?`, npub)

	// Delete torrents that only have this uploader
	result, err := tx.Exec(`
		DELETE FROM torrents
		WHERE id IN (
			SELECT torrent_id FROM torrent_uploads
			GROUP BY torrent_id
			HAVING COUNT(DISTINCT uploader_npub) = 1
			AND MAX(uploader_npub) = ?
		)
	`, npub)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete torrents from blacklisted user")
	} else {
		deleted, _ := result.RowsAffected()
		if deleted > 0 {
			log.Info().Int64("count", deleted).Str("npub", npub).Msg("Deleted torrents from blacklisted user")
		}
	}

	// Remove upload records
	tx.Exec(`DELETE FROM torrent_uploads WHERE uploader_npub = ?`, npub)

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	database.LogActivity(models.ActivityBlacklistAdd, npub)
	return nil
}

// Remove removes an npub from the blacklist
func (b *Blacklist) Remove(npub string) error {
	db := database.Get()
	_, err := db.Exec(`DELETE FROM trust_blacklist WHERE npub = ?`, npub)

	if err == nil {
		database.LogActivity(models.ActivityBlacklistRemove, npub)
	}
	return err
}

// Contains checks if an npub is in the blacklist
func (b *Blacklist) Contains(npub string) bool {
	db := database.Get()
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM trust_blacklist WHERE npub = ?`, npub).Scan(&count)
	return err == nil && count > 0
}

// GetAll returns all blacklist entries
func (b *Blacklist) GetAll() ([]models.TrustBlacklistEntry, error) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT id, npub, reason, added_at
		FROM trust_blacklist
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.TrustBlacklistEntry
	for rows.Next() {
		var entry models.TrustBlacklistEntry
		var reason *string
		if err := rows.Scan(&entry.ID, &entry.Npub, &reason, &entry.AddedAt); err != nil {
			continue
		}
		if reason != nil {
			entry.Reason = *reason
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Count returns the number of blacklist entries
func (b *Blacklist) Count() int {
	db := database.Get()
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM trust_blacklist`).Scan(&count)
	return count
}

// PurgeContent removes all content from a blacklisted user
func (b *Blacklist) PurgeContent(npub string) (int64, error) {
	db := database.Get()

	// Delete torrents that only have this uploader
	result, err := db.Exec(`
		DELETE FROM torrents
		WHERE id IN (
			SELECT torrent_id FROM torrent_uploads
			GROUP BY torrent_id
			HAVING COUNT(DISTINCT uploader_npub) = 1
			AND MAX(uploader_npub) = ?
		)
	`, npub)
	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()

	// Remove upload records
	db.Exec(`DELETE FROM torrent_uploads WHERE uploader_npub = ?`, npub)

	return deleted, nil
}
