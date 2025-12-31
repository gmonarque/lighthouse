package indexer

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/gmonarque/lighthouse/internal/nostr"
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

// DuplicateType represents the type of duplicate detection
type DuplicateType string

const (
	DuplicateExact    DuplicateType = "exact"    // Same infohash
	DuplicateProbable DuplicateType = "probable" // Same file tree hash
	DuplicateSemantic DuplicateType = "semantic" // Same IMDB/TMDB ID
)

// DuplicateResult contains duplicate detection results
type DuplicateResult struct {
	IsDuplicate   bool
	DuplicateType DuplicateType
	GroupID       string
	MatchedWith   string // Event ID or infohash of the matched torrent
	Confidence    float64
	Reason        string
}

// AdvancedDeduplicator handles advanced duplicate detection with multiple strategies
type AdvancedDeduplicator struct {
	enabled bool
}

// NewAdvancedDeduplicator creates a new advanced deduplicator
func NewAdvancedDeduplicator() *AdvancedDeduplicator {
	return &AdvancedDeduplicator{
		enabled: true,
	}
}

// SetEnabled enables or disables advanced deduplication
func (d *AdvancedDeduplicator) SetEnabled(enabled bool) {
	d.enabled = enabled
}

// FileEntry represents a single file in a torrent for dedup purposes
type FileEntry struct {
	Path string
	Size int64
}

// TorrentFilesInfo contains file information for deduplication
type TorrentFilesInfo struct {
	InfoHash string
	EventID  string
	Name     string
	Size     int64
	Files    []FileEntry
	ImdbID   string
	TmdbID   int
	Category int
}

// CheckAdvancedDuplicate checks if a torrent is a duplicate using multiple strategies
func (d *AdvancedDeduplicator) CheckAdvancedDuplicate(torrent *TorrentFilesInfo) (*DuplicateResult, error) {
	if !d.enabled {
		return &DuplicateResult{IsDuplicate: false}, nil
	}

	db := database.Get()

	// 1. Check exact duplicate (same infohash)
	var existingEventID string
	var groupID sql.NullString
	err := db.QueryRow(`
		SELECT event_id, dedup_group_id FROM torrents WHERE info_hash = ?
	`, torrent.InfoHash).Scan(&existingEventID, &groupID)

	if err == nil {
		result := &DuplicateResult{
			IsDuplicate:   true,
			DuplicateType: DuplicateExact,
			MatchedWith:   existingEventID,
			Confidence:    1.0,
			Reason:        "Same infohash",
		}
		if groupID.Valid {
			result.GroupID = groupID.String
		}
		return result, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	// 2. Check probable duplicate (file tree hash)
	if len(torrent.Files) > 0 {
		treeHash := d.computeFileTreeHash(torrent.Files)

		var matchedInfohash, matchedGroupID string
		err = db.QueryRow(`
			SELECT t.info_hash, dg.group_id
			FROM torrents t
			JOIN dedup_groups dg ON t.dedup_group_id = dg.group_id
			WHERE dg.file_tree_hash = ? AND t.info_hash != ?
			LIMIT 1
		`, treeHash, torrent.InfoHash).Scan(&matchedInfohash, &matchedGroupID)

		if err == nil {
			return &DuplicateResult{
				IsDuplicate:   true,
				DuplicateType: DuplicateProbable,
				GroupID:       matchedGroupID,
				MatchedWith:   matchedInfohash,
				Confidence:    0.95,
				Reason:        "Same file tree structure",
			}, nil
		} else if err != sql.ErrNoRows {
			log.Debug().Err(err).Msg("Error checking file tree duplicate")
		}
	}

	// 3. Check semantic duplicate (same IMDB/TMDB)
	if torrent.ImdbID != "" {
		var matchedInfohash, matchedEventID string
		err = db.QueryRow(`
			SELECT info_hash, event_id FROM torrents
			WHERE imdb_id = ? AND info_hash != ?
			ORDER BY created_at ASC LIMIT 1
		`, torrent.ImdbID, torrent.InfoHash).Scan(&matchedInfohash, &matchedEventID)

		if err == nil && d.isSimilarRelease(torrent.Name, matchedInfohash) {
			return &DuplicateResult{
				IsDuplicate:   true,
				DuplicateType: DuplicateSemantic,
				MatchedWith:   matchedEventID,
				Confidence:    0.85,
				Reason:        "Same IMDB ID and similar release",
			}, nil
		}
	}

	if torrent.TmdbID > 0 {
		var matchedInfohash, matchedEventID string
		err = db.QueryRow(`
			SELECT info_hash, event_id FROM torrents
			WHERE tmdb_id = ? AND info_hash != ?
			ORDER BY created_at ASC LIMIT 1
		`, torrent.TmdbID, torrent.InfoHash).Scan(&matchedInfohash, &matchedEventID)

		if err == nil && d.isSimilarRelease(torrent.Name, matchedInfohash) {
			return &DuplicateResult{
				IsDuplicate:   true,
				DuplicateType: DuplicateSemantic,
				MatchedWith:   matchedEventID,
				Confidence:    0.85,
				Reason:        "Same TMDB ID and similar release",
			}, nil
		}
	}

	return &DuplicateResult{IsDuplicate: false}, nil
}

// computeFileTreeHash computes a hash of the file tree structure
func (d *AdvancedDeduplicator) computeFileTreeHash(files []FileEntry) string {
	// Sort files by path for consistent hashing
	sorted := make([]FileEntry, len(files))
	copy(sorted, files)

	// Sort by path
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Path > sorted[j].Path {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Build canonical representation
	var builder string
	for _, f := range sorted {
		builder += fmt.Sprintf("%s:%d\n", strings.ToLower(f.Path), f.Size)
	}

	hash := sha256.Sum256([]byte(builder))
	return hex.EncodeToString(hash[:])
}

// isSimilarRelease checks if two releases are similar (same quality/group)
func (d *AdvancedDeduplicator) isSimilarRelease(newName, existingInfohash string) bool {
	db := database.Get()

	var existingName string
	var existingSize int64
	err := db.QueryRow(`
		SELECT name, size FROM torrents WHERE info_hash = ?
	`, existingInfohash).Scan(&existingName, &existingSize)

	if err != nil {
		return false
	}

	// Extract release info from names
	newInfo := extractReleaseInfo(newName)
	existingInfo := extractReleaseInfo(existingName)

	// Check if same quality and release group
	if newInfo.Quality != "" && existingInfo.Quality != "" {
		if newInfo.Quality != existingInfo.Quality {
			return false // Different quality
		}
	}

	if newInfo.Group != "" && existingInfo.Group != "" {
		if newInfo.Group == existingInfo.Group {
			return true // Same group = likely same release
		}
	}

	return true
}

// ReleaseInfo contains parsed release information
type ReleaseInfo struct {
	Title   string
	Year    int
	Quality string
	Source  string
	Group   string
}

// extractReleaseInfo extracts release information from a torrent name
func extractReleaseInfo(name string) ReleaseInfo {
	info := ReleaseInfo{}
	nameLower := strings.ToLower(name)

	// Quality detection
	if strings.Contains(nameLower, "2160p") || strings.Contains(nameLower, "4k") || strings.Contains(nameLower, "uhd") {
		info.Quality = "2160p"
	} else if strings.Contains(nameLower, "1080p") {
		info.Quality = "1080p"
	} else if strings.Contains(nameLower, "720p") {
		info.Quality = "720p"
	} else if strings.Contains(nameLower, "480p") || strings.Contains(nameLower, "dvd") {
		info.Quality = "480p"
	}

	// Source detection
	if strings.Contains(nameLower, "bluray") || strings.Contains(nameLower, "blu-ray") || strings.Contains(nameLower, "bdremux") {
		info.Source = "bluray"
	} else if strings.Contains(nameLower, "web-dl") || strings.Contains(nameLower, "webdl") || strings.Contains(nameLower, "webrip") {
		info.Source = "web"
	} else if strings.Contains(nameLower, "hdtv") {
		info.Source = "hdtv"
	}

	// Release group (usually at the end after a dash)
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// Remove file extension if present
		if idx := strings.LastIndex(lastPart, "."); idx > 0 {
			lastPart = lastPart[:idx]
		}
		info.Group = strings.ToUpper(strings.TrimSpace(lastPart))
	}

	return info
}

// CreateDedupGroup creates a new deduplication group
func (d *AdvancedDeduplicator) CreateDedupGroup(canonicalInfohash string, treeHash string) (string, error) {
	db := database.Get()

	groupID := fmt.Sprintf("dg_%s", canonicalInfohash[:16])

	_, err := db.Exec(`
		INSERT INTO dedup_groups (group_id, canonical_infohash, file_tree_hash, member_count, created_at)
		VALUES (?, ?, ?, 1, datetime('now'))
		ON CONFLICT(group_id) DO NOTHING
	`, groupID, canonicalInfohash, treeHash)
	if err != nil {
		return "", fmt.Errorf("failed to create dedup group: %w", err)
	}

	return groupID, nil
}

// AddToGroup adds a torrent to an existing dedup group
func (d *AdvancedDeduplicator) AddToGroup(groupID, infohash string) error {
	db := database.Get()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update torrent
	_, err = tx.Exec(`UPDATE torrents SET dedup_group_id = ? WHERE info_hash = ?`, groupID, infohash)
	if err != nil {
		return fmt.Errorf("failed to update torrent: %w", err)
	}

	// Increment member count
	_, err = tx.Exec(`UPDATE dedup_groups SET member_count = member_count + 1 WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	return tx.Commit()
}

// GetGroupMembers returns all infohashes in a dedup group
func (d *AdvancedDeduplicator) GetGroupMembers(groupID string) ([]string, error) {
	db := database.Get()

	rows, err := db.Query(`SELECT info_hash FROM torrents WHERE dedup_group_id = ?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []string
	for rows.Next() {
		var infohash string
		if err := rows.Scan(&infohash); err != nil {
			return nil, err
		}
		members = append(members, infohash)
	}

	return members, nil
}

// DedupStats contains deduplication statistics
type DedupStats struct {
	TotalGroups  int
	TotalMembers int
	MaxGroupSize int
	AvgGroupSize float64
}

// GetDedupStats returns deduplication statistics
func (d *AdvancedDeduplicator) GetDedupStats() (*DedupStats, error) {
	db := database.Get()

	stats := &DedupStats{}

	err := db.QueryRow(`
		SELECT
			COUNT(DISTINCT group_id) as groups,
			COALESCE(SUM(member_count), 0) as total_members,
			COALESCE(MAX(member_count), 0) as max_members,
			COALESCE(AVG(member_count), 0) as avg_members
		FROM dedup_groups
	`).Scan(&stats.TotalGroups, &stats.TotalMembers, &stats.MaxGroupSize, &stats.AvgGroupSize)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return stats, nil
}
