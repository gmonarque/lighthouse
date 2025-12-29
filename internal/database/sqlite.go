package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lighthouse-client/lighthouse/internal/config"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var db *sql.DB

// Init initializes the SQLite database connection and runs migrations
func Init(dbPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite handles concurrency via WAL
	db.SetMaxIdleConns(1)

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Register setup checker with config package
	config.SetupCompletedChecker = IsSetupCompleted

	log.Info().Str("path", dbPath).Msg("Database initialized")
	return nil
}

// IsSetupCompleted checks if the setup wizard has been completed
func IsSetupCompleted() bool {
	if db == nil {
		return false
	}
	value, err := GetSetting("setup_completed")
	if err != nil {
		return false
	}
	return value == "true"
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

// runMigrations executes all SQL migration files in order
func runMigrations() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migrations by filename
	var migrationFiles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Execute each migration
	for _, filename := range migrationFiles {
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		log.Debug().Str("file", filename).Msg("Running migration")

		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}
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
	defaultEntries := []WhitelistEntry{
		{
			Npub:  "npub1j4ppgput8ss89v9xvsv7pww0nxdc4wxk4edt69zhakluljcnwgvq8mf8uy",
			Alias: "nmirror",
			Notes: "https://nostrudel.ninja/u/npub1j4ppgput8ss89v9xvsv7pww0nxdc4wxk4edt69zhakluljcnwgvq8mf8uy",
		},
	}

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
