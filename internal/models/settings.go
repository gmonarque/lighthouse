package models

// Setting represents a key-value setting
type Setting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// AppSettings represents the full application settings
type AppSettings struct {
	Server     ServerSettings     `json:"server"`
	Database   DatabaseSettings   `json:"database"`
	Nostr      NostrSettings      `json:"nostr"`
	Trust      TrustSettings      `json:"trust"`
	Enrichment EnrichmentSettings `json:"enrichment"`
}

// ServerSettings represents server configuration
type ServerSettings struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	APIKey string `json:"api_key"`
}

// DatabaseSettings represents database configuration
type DatabaseSettings struct {
	Path string `json:"path"`
}

// NostrSettings represents Nostr configuration
type NostrSettings struct {
	Identity NostrIdentity `json:"identity"`
	Relays   []Relay       `json:"relays"`
}

// NostrIdentity represents Nostr identity settings
type NostrIdentity struct {
	Npub string `json:"npub"`
	Nsec string `json:"nsec,omitempty"`
}

// TrustSettings represents trust configuration
type TrustSettings struct {
	Depth int `json:"depth"`
}

// EnrichmentSettings represents metadata enrichment configuration
type EnrichmentSettings struct {
	Enabled    bool   `json:"enabled"`
	TMDBAPIKey string `json:"tmdb_api_key,omitempty"`
	OMDBAPIKey string `json:"omdb_api_key,omitempty"`
}

// SetupStatus represents the setup wizard status
type SetupStatus struct {
	Completed         bool `json:"completed"`
	HasIdentity       bool `json:"has_identity"`
	HasRelays         bool `json:"has_relays"`
	HasTMDBKey        bool `json:"has_tmdb_key"`
	EnrichmentEnabled bool `json:"enrichment_enabled"`
}

// IndexerStatus represents the indexer status
type IndexerStatus struct {
	Running         bool  `json:"running"`
	Enabled         bool  `json:"enabled"`
	TotalTorrents   int64 `json:"total_torrents"`
	ConnectedRelays int   `json:"connected_relays"`
}

// ActivityLog represents an activity log entry
type ActivityLog struct {
	ID        int64  `json:"id"`
	EventType string `json:"event_type"`
	Details   string `json:"details,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Activity event types
const (
	ActivityTorrentAdded     = "torrent_added"
	ActivityRelayConnected   = "relay_connected"
	ActivityRelayError       = "relay_error"
	ActivityTrustUpdated     = "trust_updated"
	ActivityWhitelistAdd     = "whitelist_add"
	ActivityWhitelistRemove  = "whitelist_remove"
	ActivityBlacklistAdd     = "blacklist_add"
	ActivityBlacklistRemove  = "blacklist_remove"
	ActivityIdentityGenerate = "identity_generated"
	ActivityIdentityImport   = "identity_imported"
	ActivityIndexerStarted   = "indexer_started"
	ActivityIndexerStopped   = "indexer_stopped"
	ActivitySetupCompleted   = "setup_completed"
	ActivityConfigImported   = "config_imported"
	ActivityContactsImported = "contacts_imported"
)
