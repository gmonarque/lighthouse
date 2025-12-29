package indexer

import (
	"database/sql"
	"encoding/json"

	"github.com/lighthouse-client/lighthouse/internal/database"
	"github.com/lighthouse-client/lighthouse/internal/nostr"
	"github.com/rs/zerolog/log"
)

// Deduplicator handles deduplication of torrents by info hash
type Deduplicator struct{}

// NewDeduplicator creates a new Deduplicator
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{}
}

// determineCategoryCode determines the Torznab category code from event data
// It combines Category field and ContentTags for hierarchical matching
func determineCategoryCode(event *nostr.TorrentEvent) int {
	// Combine all tags for hierarchical category detection
	allTags := make([]string, 0, len(event.ContentTags)+1)

	// Add explicit category tag if present
	if event.Category != "" {
		allTags = append(allTags, event.Category)
	}

	// Add all content tags (t tags from NIP-35)
	allTags = append(allTags, event.ContentTags...)

	// Use the new hierarchical category detection
	return nostr.CategoryFromNostrTags(allTags)
}

// Process processes a torrent event, deduplicating by info hash
// Returns true if this is a new torrent, false if it's a duplicate
func (d *Deduplicator) Process(event *nostr.TorrentEvent, relayURL string) (bool, error) {
	db := database.Get()

	// Check if torrent already exists
	var torrentID int64
	err := db.QueryRow("SELECT id FROM torrents WHERE info_hash = ?", event.InfoHash).Scan(&torrentID)

	if err == sql.ErrNoRows {
		// New torrent - insert it
		// Determine category from Category field or ContentTags
		category := determineCategoryCode(event)

		// Serialize files to JSON
		var filesJSON string
		if len(event.Files) > 0 {
			if data, err := json.Marshal(event.Files); err == nil {
				filesJSON = string(data)
			}
		}

		log.Debug().
			Str("info_hash", event.InfoHash).
			Str("category_tag", event.Category).
			Strs("content_tags", event.ContentTags).
			Int("category_code", category).
			Int("file_count", len(event.Files)).
			Msg("Categorizing new torrent")

		result, err := db.Exec(`
			INSERT OR IGNORE INTO torrents (info_hash, name, size, category, magnet_uri, files, trust_score, upload_count)
			VALUES (?, ?, ?, ?, ?, ?, 10, 1)
		`, event.InfoHash, event.Name, event.Size, category, event.MagnetURI, filesJSON)

		if err != nil {
			return false, err
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			// Race condition: another goroutine inserted this torrent first
			// Fetch the existing torrent ID and record this upload
			err = db.QueryRow("SELECT id FROM torrents WHERE info_hash = ?", event.InfoHash).Scan(&torrentID)
			if err != nil {
				return false, nil // Silently skip
			}
			// Record this upload for the existing torrent
			db.Exec(`
				INSERT OR IGNORE INTO torrent_uploads (torrent_id, uploader_npub, nostr_event_id, relay_url)
				VALUES (?, ?, ?, ?)
			`, torrentID, event.Pubkey, event.EventID, relayURL)
			return false, nil
		}

		torrentID, _ = result.LastInsertId()

		// Record the upload
		_, err = db.Exec(`
			INSERT OR IGNORE INTO torrent_uploads (torrent_id, uploader_npub, nostr_event_id, relay_url)
			VALUES (?, ?, ?, ?)
		`, torrentID, event.Pubkey, event.EventID, relayURL)

		if err != nil {
			log.Error().Err(err).Msg("Failed to record upload")
		}

		log.Debug().
			Str("info_hash", event.InfoHash).
			Str("name", event.Name).
			Msg("New torrent indexed")

		return true, nil
	} else if err != nil {
		// Actual database error (not ErrNoRows)
		return false, err
	}

	// Existing torrent - check if this is a new upload
	var uploadExists int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM torrent_uploads
		WHERE torrent_id = ? AND nostr_event_id = ?
	`, torrentID, event.EventID).Scan(&uploadExists)

	if err != nil || uploadExists > 0 {
		// Already seen this exact event
		return false, nil
	}

	// New upload of existing torrent
	_, err = db.Exec(`
		INSERT INTO torrent_uploads (torrent_id, uploader_npub, nostr_event_id, relay_url)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(nostr_event_id) DO NOTHING
	`, torrentID, event.Pubkey, event.EventID, relayURL)

	if err != nil {
		log.Error().Err(err).Msg("Failed to record upload")
	}

	// Update torrent stats
	_, err = db.Exec(`
		UPDATE torrents SET
			upload_count = (SELECT COUNT(DISTINCT uploader_npub) FROM torrent_uploads WHERE torrent_id = ?),
			trust_score = trust_score + 10,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, torrentID, torrentID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update torrent stats")
	}

	log.Debug().
		Str("info_hash", event.InfoHash).
		Int64("torrent_id", torrentID).
		Msg("Duplicate torrent, updated trust score")

	return false, nil
}

// CalculateTrustScore calculates the trust score for a torrent
func (d *Deduplicator) CalculateTrustScore(torrentID int64, userNpub string, trustDepth int) (int, error) {
	db := database.Get()

	// Base score
	var baseScore int
	err := db.QueryRow("SELECT trust_score FROM torrents WHERE id = ?", torrentID).Scan(&baseScore)
	if err != nil {
		return 0, err
	}

	if trustDepth == 0 {
		// Only count whitelisted uploaders
		var trustedCount int
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT tu.uploader_npub)
			FROM torrent_uploads tu
			JOIN trust_whitelist tw ON tu.uploader_npub = tw.npub
			WHERE tu.torrent_id = ?
		`, torrentID).Scan(&trustedCount)

		if err != nil {
			return baseScore, nil
		}

		return baseScore + (trustedCount * 50), nil
	}

	if trustDepth >= 1 {
		// Count uploaders who are followed by the user or in whitelist
		var trustedCount int
		err = db.QueryRow(`
			SELECT COUNT(DISTINCT tu.uploader_npub)
			FROM torrent_uploads tu
			WHERE tu.torrent_id = ?
			AND (
				tu.uploader_npub IN (SELECT npub FROM trust_whitelist)
				OR tu.uploader_npub IN (
					SELECT followed_npub FROM trust_follows
					WHERE follower_npub = ? AND depth <= ?
				)
			)
		`, torrentID, userNpub, trustDepth).Scan(&trustedCount)

		if err != nil {
			return baseScore, nil
		}

		return baseScore + (trustedCount * 50), nil
	}

	return baseScore, nil
}

// RecalculateAllTrustScores recalculates trust scores for all torrents
func (d *Deduplicator) RecalculateAllTrustScores() error {
	db := database.Get()

	// Update all torrent upload counts
	_, err := db.Exec(`
		UPDATE torrents SET
			upload_count = (SELECT COUNT(DISTINCT uploader_npub) FROM torrent_uploads WHERE torrent_id = torrents.id)
	`)
	if err != nil {
		return err
	}

	// Recalculate base trust scores
	_, err = db.Exec(`
		UPDATE torrents SET
			trust_score = 10 + (upload_count * 10)
	`)

	return err
}

// PurgeTorrentsFromUploader removes all torrents that only have uploads from a specific uploader
func (d *Deduplicator) PurgeTorrentsFromUploader(npub string) (int64, error) {
	db := database.Get()

	// Find torrents that only have this uploader
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

	// Also remove this uploader's records from shared torrents
	db.Exec("DELETE FROM torrent_uploads WHERE uploader_npub = ?", npub)

	return deleted, nil
}
