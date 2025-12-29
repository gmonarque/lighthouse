package models

import "time"

// Relay represents a Nostr relay configuration
type Relay struct {
	ID              int64      `json:"id"`
	URL             string     `json:"url"`
	Name            string     `json:"name,omitempty"`
	Preset          string     `json:"preset,omitempty"`
	Enabled         bool       `json:"enabled"`
	Status          string     `json:"status"`
	LastConnectedAt *time.Time `json:"last_connected_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// RelayPreset represents a preset configuration
type RelayPreset struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Relays      []string `json:"relays"`
}

// DefaultRelayPresets returns the default relay presets
func DefaultRelayPresets() []RelayPreset {
	return []RelayPreset{
		{
			Name:        "public",
			Description: "Popular public relays",
			Relays: []string{
				"wss://relay.damus.io",
				"wss://nos.lol",
				"wss://relay.nostr.band",
				"wss://relay.snort.social",
			},
		},
		{
			Name:        "minimal",
			Description: "Minimal relay set for low bandwidth",
			Relays: []string{
				"wss://relay.damus.io",
				"wss://nos.lol",
			},
		},
		{
			Name:        "censorship-resistant",
			Description: "Relays with minimal moderation",
			Relays: []string{
				"wss://nostr.bitcoiner.social",
				"wss://nostr.oxtr.dev",
			},
		},
	}
}

// RelayStatus constants
const (
	RelayStatusConnected    = "connected"
	RelayStatusConnecting   = "connecting"
	RelayStatusDisconnected = "disconnected"
	RelayStatusError        = "error"
)
