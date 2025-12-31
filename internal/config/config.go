package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Nostr      NostrConfig      `mapstructure:"nostr"`
	Trust      TrustConfig      `mapstructure:"trust"`
	Enrichment EnrichmentConfig `mapstructure:"enrichment"`
	Indexer    IndexerConfig    `mapstructure:"indexer"`
	Curator    CuratorConfig    `mapstructure:"curator"`
	Relay      RelayServerConfig `mapstructure:"relay"`
}

type ServerConfig struct {
	Host   string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	APIKey string `mapstructure:"api_key"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type NostrConfig struct {
	Identity NostrIdentity  `mapstructure:"identity"`
	Relays   []RelayConfig  `mapstructure:"relays"`
}

type NostrIdentity struct {
	Npub string `mapstructure:"npub"`
	Nsec string `mapstructure:"nsec"`
}

type RelayConfig struct {
	URL     string `mapstructure:"url"`
	Name    string `mapstructure:"name"`
	Preset  string `mapstructure:"preset"`
	Enabled bool   `mapstructure:"enabled"`
}

type TrustConfig struct {
	Depth int `mapstructure:"depth"`
}

type EnrichmentConfig struct {
	TMDBAPIKey string `mapstructure:"tmdb_api_key"`
	OMDBAPIKey string `mapstructure:"omdb_api_key"`
	Enabled    bool   `mapstructure:"enabled"`
}

type IndexerConfig struct {
	// TagFilter contains tags that must be present for a torrent to be indexed.
	// If empty, all torrents are indexed. If set, only torrents with at least
	// one matching tag will be indexed.
	// Example: ["movie", "tv"] - only index movies and TV shows
	TagFilter        []string `mapstructure:"tag_filter"`
	TagFilterEnabled bool     `mapstructure:"tag_filter_enabled"`
}

type CuratorConfig struct {
	// Enabled enables the local curator module
	Enabled bool `mapstructure:"enabled"`
	// Mode: "local" (curate locally), "remote" (consume external decisions), "hybrid" (both)
	Mode string `mapstructure:"mode"`
	// AggregationMode: "any", "all", "quorum", "weighted"
	AggregationMode string `mapstructure:"aggregation_mode"`
	// QuorumRequired for quorum mode
	QuorumRequired int `mapstructure:"quorum_required"`
	// RejectThreshold for semantic rules (0.0-1.0)
	RejectThreshold float64 `mapstructure:"reject_threshold"`
}

type RelayServerConfig struct {
	// Enabled enables the relay server module
	Enabled bool `mapstructure:"enabled"`
	// Listen address for the relay WebSocket server
	Listen string `mapstructure:"listen"`
	// Mode: "public" or "community"
	Mode string `mapstructure:"mode"`
	// RequireCuration only accept curated content
	RequireCuration bool `mapstructure:"require_curation"`
	// SyncWith list of relays to sync with
	SyncWith []string `mapstructure:"sync_with"`
	// EnableDiscovery enable relay discovery via Nostr
	EnableDiscovery bool `mapstructure:"enable_discovery"`
}

var cfg *Config

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/lighthouse")
	viper.AddConfigPath("$HOME/.lighthouse")

	// Set defaults
	setDefaults()

	// Environment variable overrides
	viper.SetEnvPrefix("LIGHTHOUSE")
	viper.AutomaticEnv()

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Info().Msg("No config file found, using defaults")
			if err := createDefaultConfig(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	// Generate API key if not set
	if cfg.Server.APIKey == "" {
		cfg.Server.APIKey = generateAPIKey()
		viper.Set("server.api_key", cfg.Server.APIKey)
		if err := viper.WriteConfig(); err != nil {
			log.Warn().Err(err).Msg("Could not save generated API key to config")
		}
	}

	return cfg, nil
}

func Get() *Config {
	if cfg == nil {
		log.Fatal().Msg("Config not loaded")
	}
	return cfg
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 9999)
	viper.SetDefault("server.api_key", "")

	// Database defaults
	viper.SetDefault("database.path", "./data/lighthouse.db")

	// Nostr defaults
	viper.SetDefault("nostr.identity.npub", "")
	viper.SetDefault("nostr.identity.nsec", "")
	viper.SetDefault("nostr.relays", []RelayConfig{
		// Popular public relays
		{URL: "wss://relay.damus.io", Name: "Damus", Preset: "public", Enabled: true},
		{URL: "wss://nos.lol", Name: "nos.lol", Preset: "public", Enabled: true},
		{URL: "wss://relay.nostr.band", Name: "Nostr Band", Preset: "public", Enabled: true},
		{URL: "wss://relay.snort.social", Name: "Snort", Preset: "public", Enabled: true},
		{URL: "wss://relay.primal.net", Name: "Primal", Preset: "public", Enabled: true},
		{URL: "wss://nostr.wine", Name: "Nostr Wine", Preset: "public", Enabled: true},
		{URL: "wss://relay.nostr.org", Name: "Nostr.org", Preset: "public", Enabled: true},
		{URL: "wss://nostr-pub.wellorder.net", Name: "Wellorder", Preset: "public", Enabled: true},
		{URL: "wss://nostr.bitcoiner.social", Name: "Bitcoiner Social", Preset: "public", Enabled: true},
		{URL: "wss://nostr.mom", Name: "Nostr Mom", Preset: "public", Enabled: true},
		// Censorship-resistant relays
		{URL: "wss://nostr.mutinywallet.com", Name: "Mutiny", Preset: "censorship-resistant", Enabled: true},
		{URL: "wss://relay.nostr.bg", Name: "Nostr BG", Preset: "censorship-resistant", Enabled: true},
		{URL: "wss://nostr.oxtr.dev", Name: "Oxtr", Preset: "censorship-resistant", Enabled: true},
	})

	// Trust defaults
	viper.SetDefault("trust.depth", 1)

	// Enrichment defaults
	viper.SetDefault("enrichment.tmdb_api_key", "")
	viper.SetDefault("enrichment.omdb_api_key", "")
	viper.SetDefault("enrichment.enabled", true)

	// Indexer defaults
	viper.SetDefault("indexer.tag_filter", []string{})
	viper.SetDefault("indexer.tag_filter_enabled", false)

	// Curator defaults
	viper.SetDefault("curator.enabled", false)
	viper.SetDefault("curator.mode", "local")
	viper.SetDefault("curator.aggregation_mode", "any")
	viper.SetDefault("curator.quorum_required", 1)
	viper.SetDefault("curator.reject_threshold", 0.7)

	// Relay server defaults
	viper.SetDefault("relay.enabled", false)
	viper.SetDefault("relay.listen", "0.0.0.0:9998")
	viper.SetDefault("relay.mode", "community")
	viper.SetDefault("relay.require_curation", true)
	viper.SetDefault("relay.sync_with", []string{})
	viper.SetDefault("relay.enable_discovery", false)
}

func createDefaultConfig() error {
	configPath := "./config.yaml"

	// Ensure parent directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write default config
	return viper.SafeWriteConfigAs(configPath)
}

func generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal().Err(err).Msg("Failed to generate API key")
	}
	return hex.EncodeToString(bytes)
}

// Save writes the current configuration to the config file
func Save() error {
	return viper.WriteConfig()
}

// Update updates a configuration value and saves
func Update(key string, value interface{}) error {
	viper.Set(key, value)
	if err := Save(); err != nil {
		return err
	}
	// Reload the in-memory config to reflect the change
	return viper.Unmarshal(cfg)
}

// IsFirstRun returns true if this is the first run (no identity configured)
// Note: This only checks config. Use IsSetupCompleted() for full setup status.
func IsFirstRun() bool {
	return cfg.Nostr.Identity.Npub == "" || cfg.Nostr.Identity.Nsec == ""
}

// SetupCompletedChecker is a function type that checks if setup is complete
// This is set by the database package to avoid circular imports
var SetupCompletedChecker func() bool

// IsSetupCompleted returns true if the setup wizard has been completed
func IsSetupCompleted() bool {
	if SetupCompletedChecker != nil {
		return SetupCompletedChecker()
	}
	// Fallback to config check if database not initialized
	return !IsFirstRun()
}
