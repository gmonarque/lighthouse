package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gmonarque/lighthouse/internal/config"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

//go:embed schema.sql
var schemaSQL string

var db *sql.DB

// Init initializes the SQLite database connection and runs schema
func Init(dbPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	var err error
	// WAL mode allows concurrent reads during writes
	// Increase busy timeout to 30 seconds to prevent blocking
	// cache=shared enables shared cache for better concurrency
	db, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=30000&cache=shared&_synchronous=NORMAL")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Allow multiple connections for concurrent reads
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Run schema
	if err := runSchema(); err != nil {
		return fmt.Errorf("failed to run schema: %w", err)
	}

	// Register setup checker with config package
	config.SetupCompletedChecker = IsSetupCompleted

	// Auto-detect setup completion from config.yaml identity
	if err := autoDetectSetupCompletion(); err != nil {
		log.Warn().Err(err).Msg("Failed to auto-detect setup completion")
	}

	log.Info().Str("path", dbPath).Msg("Database initialized")
	return nil
}

// autoDetectSetupCompletion checks if config.yaml has identity configured
// and marks setup as complete if so (for users with existing configs)
func autoDetectSetupCompletion() error {
	// Check if setup is already marked complete
	value, err := GetSetting("setup_completed")
	if err != nil {
		return err
	}
	if value == "true" {
		return nil // Already complete
	}

	// Check if config has identity configured
	cfg := config.Get()
	if cfg.Nostr.Identity.Npub != "" && cfg.Nostr.Identity.Nsec != "" {
		// Config has identity, mark setup as complete
		log.Info().Msg("Detected existing identity in config.yaml, marking setup as complete")
		return SetSetting("setup_completed", "true")
	}

	return nil
}

// IsSetupCompleted checks if the setup wizard has been completed
func IsSetupCompleted() bool {
	if db == nil {
		return false
	}

	// First check database flag
	value, err := GetSetting("setup_completed")
	if err == nil && value == "true" {
		return true
	}

	// Also check if config has identity (for existing users)
	cfg := config.Get()
	if cfg.Nostr.Identity.Npub != "" && cfg.Nostr.Identity.Nsec != "" {
		return true
	}

	return false
}

// Get returns the database connection
func Get() *sql.DB {
	if db == nil {
		log.Fatal().Msg("Database not initialized")
	}
	return db
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// runSchema executes the database schema
func runSchema() error {
	log.Debug().Msg("Running database schema")
	_, err := db.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	return nil
}

// GetSetting retrieves a setting value by key
func GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting sets a setting value
func SetSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}

// GetStats returns database statistics
func GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total torrents
	var totalTorrents int64
	if err := db.QueryRow("SELECT COUNT(*) FROM torrents").Scan(&totalTorrents); err != nil {
		return nil, err
	}
	stats["total_torrents"] = totalTorrents

	// Total size (bytes)
	var totalSize sql.NullInt64
	if err := db.QueryRow("SELECT SUM(size) FROM torrents").Scan(&totalSize); err != nil {
		return nil, err
	}
	stats["total_size"] = totalSize.Int64

	// Torrents by category
	rows, err := db.Query("SELECT category, COUNT(*) as count FROM torrents GROUP BY category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make(map[int]int64)
	for rows.Next() {
		var category int
		var count int64
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		categories[category] = count
	}
	stats["categories"] = categories

	// Connected relays
	var connectedRelays int64
	if err := db.QueryRow("SELECT COUNT(*) FROM relays WHERE status = 'connected'").Scan(&connectedRelays); err != nil {
		return nil, err
	}
	stats["connected_relays"] = connectedRelays

	// Whitelist count
	var whitelistCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM trust_whitelist").Scan(&whitelistCount); err != nil {
		return nil, err
	}
	stats["whitelist_count"] = whitelistCount

	// Blacklist count
	var blacklistCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM trust_blacklist").Scan(&blacklistCount); err != nil {
		return nil, err
	}
	stats["blacklist_count"] = blacklistCount

	// Unique uploaders
	var uniqueUploaders int64
	if err := db.QueryRow("SELECT COUNT(DISTINCT uploader_npub) FROM torrent_uploads").Scan(&uniqueUploaders); err != nil {
		return nil, err
	}
	stats["unique_uploaders"] = uniqueUploaders

	// Database file size
	var pageCount, pageSize int64
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return nil, err
	}
	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	stats["database_size"] = pageCount * pageSize

	return stats, nil
}

// GetRecentTorrents returns the most recent torrents
func GetRecentTorrents(limit int) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT id, info_hash, name, size, category, seeders, leechers,
			   title, year, poster_url, trust_score, first_seen_at
		FROM torrents
		ORDER BY first_seen_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	torrents := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var infoHash, name string
		var size, category, seeders, leechers sql.NullInt64
		var title, posterURL sql.NullString
		var year sql.NullInt64
		var trustScore int64
		var firstSeenAt string

		if err := rows.Scan(&id, &infoHash, &name, &size, &category, &seeders, &leechers,
			&title, &year, &posterURL, &trustScore, &firstSeenAt); err != nil {
			return nil, err
		}

		torrent := map[string]interface{}{
			"id":            id,
			"info_hash":     infoHash,
			"name":          name,
			"size":          size.Int64,
			"category":      category.Int64,
			"seeders":       seeders.Int64,
			"leechers":      leechers.Int64,
			"title":         title.String,
			"year":          year.Int64,
			"poster_url":    posterURL.String,
			"trust_score":   trustScore,
			"first_seen_at": firstSeenAt,
		}
		torrents = append(torrents, torrent)
	}

	return torrents, nil
}

// GetTorrentsPerDay returns torrent counts for the last N days
func GetTorrentsPerDay(days int) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT DATE(first_seen_at) as date, COUNT(*) as count
		FROM torrents
		WHERE first_seen_at >= DATE('now', '-' || ? || ' days')
		GROUP BY DATE(first_seen_at)
		ORDER BY date ASC
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]map[string]interface{}, 0)
	for rows.Next() {
		var date string
		var count int64
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		stats = append(stats, map[string]interface{}{
			"date":  date,
			"count": count,
		})
	}

	return stats, nil
}

// LogActivity logs an activity event
func LogActivity(eventType string, details string) error {
	_, err := db.Exec(`
		INSERT INTO activity_log (event_type, details, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, eventType, details)
	return err
}

// RelayConfig represents a relay configuration for seeding
type RelayConfig struct {
	URL     string
	Name    string
	Preset  string
	Enabled bool
}

// SeedDefaultRelays inserts default relays if the relays table is empty
func SeedDefaultRelays(relays []RelayConfig) error {
	// Check if relays table is empty
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM relays").Scan(&count); err != nil {
		return fmt.Errorf("failed to check relays count: %w", err)
	}

	// Only seed if table is empty
	if count > 0 {
		log.Debug().Int("count", count).Msg("Relays already exist, skipping seed")
		return nil
	}

	// Insert default relays
	stmt, err := db.Prepare(`
		INSERT INTO relays (url, name, preset, enabled, status)
		VALUES (?, ?, ?, ?, 'disconnected')
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare relay insert: %w", err)
	}
	defer stmt.Close()

	for _, relay := range relays {
		_, err := stmt.Exec(relay.URL, relay.Name, relay.Preset, relay.Enabled)
		if err != nil {
			log.Warn().Err(err).Str("url", relay.URL).Msg("Failed to insert default relay")
			continue
		}
	}

	log.Info().Int("count", len(relays)).Msg("Seeded default relays")
	return nil
}

// WhitelistEntry represents a default whitelist entry
type WhitelistEntry struct {
	Npub  string
	Alias string
	Notes string
}

// SeedDefaultWhitelist adds default trusted users to the whitelist
func SeedDefaultWhitelist() error {
	// No default entries - users must add their own trusted sources
	defaultEntries := []WhitelistEntry{}

	// Check if whitelist already has entries
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM trust_whitelist").Scan(&count); err != nil {
		return fmt.Errorf("failed to check whitelist count: %w", err)
	}

	// Only seed if table is empty
	if count > 0 {
		log.Debug().Int("count", count).Msg("Whitelist already has entries, skipping seed")
		return nil
	}

	// Insert default whitelist entries
	stmt, err := db.Prepare(`
		INSERT INTO trust_whitelist (npub, alias, notes)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare whitelist insert: %w", err)
	}
	defer stmt.Close()

	for _, entry := range defaultEntries {
		_, err := stmt.Exec(entry.Npub, entry.Alias, entry.Notes)
		if err != nil {
			log.Warn().Err(err).Str("npub", entry.Npub).Msg("Failed to insert default whitelist entry")
			continue
		}
	}

	log.Info().Int("count", len(defaultEntries)).Msg("Seeded default whitelist")
	return nil
}
