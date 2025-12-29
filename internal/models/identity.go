package models

import "time"

// Identity represents a Nostr identity
type Identity struct {
	ID        int64     `json:"id"`
	Npub      string    `json:"npub"`
	Nsec      string    `json:"nsec,omitempty"` // Only populated for own identity
	Name      string    `json:"name,omitempty"`
	IsOwn     bool      `json:"is_own"`
	CreatedAt time.Time `json:"created_at"`
}

// TrustWhitelistEntry represents a whitelisted user
type TrustWhitelistEntry struct {
	ID      int64     `json:"id"`
	Npub    string    `json:"npub"`
	Alias   string    `json:"alias,omitempty"`
	Notes   string    `json:"notes,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

// TrustBlacklistEntry represents a blacklisted user
type TrustBlacklistEntry struct {
	ID      int64     `json:"id"`
	Npub    string    `json:"npub"`
	Reason  string    `json:"reason,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

// TrustFollow represents a follow relationship
type TrustFollow struct {
	ID           int64     `json:"id"`
	FollowerNpub string    `json:"follower_npub"`
	FollowedNpub string    `json:"followed_npub"`
	Depth        int       `json:"depth"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// TrustLevel constants
const (
	TrustLevelWhitelistOnly = 0 // Only show content from whitelist
	TrustLevelFriends       = 1 // Show content from direct follows
	TrustLevelExtended      = 2 // Show content from friends of friends
)

// TrustLevelDescription returns a human-readable description
func TrustLevelDescription(level int) string {
	switch level {
	case TrustLevelWhitelistOnly:
		return "Whitelist Only - Only show content from explicitly trusted users"
	case TrustLevelFriends:
		return "Friends - Show content from your Nostr follows"
	case TrustLevelExtended:
		return "Extended Network - Show content from friends of friends (use with caution)"
	default:
		return "Unknown trust level"
	}
}
