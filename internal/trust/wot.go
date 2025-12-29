package trust

import (
	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/lighthouse-client/lighthouse/internal/database"
)

// WebOfTrust manages the trust graph
type WebOfTrust struct{}

// NewWebOfTrust creates a new WebOfTrust manager
func NewWebOfTrust() *WebOfTrust {
	return &WebOfTrust{}
}

// IsTrusted checks if an npub is trusted based on the current trust depth setting
func (w *WebOfTrust) IsTrusted(npub string) bool {
	cfg := config.Get()
	db := database.Get()

	// Check blacklist first
	var blacklisted int
	err := db.QueryRow("SELECT COUNT(*) FROM trust_blacklist WHERE npub = ?", npub).Scan(&blacklisted)
	if err == nil && blacklisted > 0 {
		return false
	}

	// Depth 0: Whitelist only
	if cfg.Trust.Depth == 0 {
		var whitelisted int
		err := db.QueryRow("SELECT COUNT(*) FROM trust_whitelist WHERE npub = ?", npub).Scan(&whitelisted)
		return err == nil && whitelisted > 0
	}

	// Check whitelist (always trusted at any depth)
	var whitelisted int
	err = db.QueryRow("SELECT COUNT(*) FROM trust_whitelist WHERE npub = ?", npub).Scan(&whitelisted)
	if err == nil && whitelisted > 0 {
		return true
	}

	// Depth 1+: Check follows
	ownNpub := cfg.Nostr.Identity.Npub
	if ownNpub == "" {
		return false
	}

	var followDepth int
	err = db.QueryRow(`
		SELECT depth FROM trust_follows
		WHERE follower_npub = ? AND followed_npub = ? AND depth <= ?
	`, ownNpub, npub, cfg.Trust.Depth).Scan(&followDepth)

	return err == nil && followDepth <= cfg.Trust.Depth
}

// GetTrustScore calculates the trust score for an npub
func (w *WebOfTrust) GetTrustScore(npub string) int {
	cfg := config.Get()
	db := database.Get()
	score := 0

	// Blacklisted = -1000
	var blacklisted int
	err := db.QueryRow("SELECT COUNT(*) FROM trust_blacklist WHERE npub = ?", npub).Scan(&blacklisted)
	if err == nil && blacklisted > 0 {
		return -1000
	}

	// Whitelisted = +100
	var whitelisted int
	err = db.QueryRow("SELECT COUNT(*) FROM trust_whitelist WHERE npub = ?", npub).Scan(&whitelisted)
	if err == nil && whitelisted > 0 {
		score += 100
	}

	// Direct follow = +50
	ownNpub := cfg.Nostr.Identity.Npub
	if ownNpub != "" {
		var depth int
		err := db.QueryRow(`
			SELECT depth FROM trust_follows
			WHERE follower_npub = ? AND followed_npub = ?
		`, ownNpub, npub).Scan(&depth)
		if err == nil {
			switch depth {
			case 1:
				score += 50
			case 2:
				score += 20
			}
		}
	}

	return score
}

// GetTrustedUploaders returns all uploaders trusted at the current depth
func (w *WebOfTrust) GetTrustedUploaders() ([]string, error) {
	cfg := config.Get()
	db := database.Get()

	var uploaders []string

	// Always include whitelist
	rows, err := db.Query("SELECT npub FROM trust_whitelist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var npub string
		if err := rows.Scan(&npub); err != nil {
			continue
		}
		uploaders = append(uploaders, npub)
	}

	if cfg.Trust.Depth == 0 {
		return uploaders, nil
	}

	// Include follows at configured depth
	ownNpub := cfg.Nostr.Identity.Npub
	if ownNpub == "" {
		return uploaders, nil
	}

	rows, err = db.Query(`
		SELECT followed_npub FROM trust_follows
		WHERE follower_npub = ? AND depth <= ?
	`, ownNpub, cfg.Trust.Depth)
	if err != nil {
		return uploaders, nil
	}
	defer rows.Close()

	for rows.Next() {
		var npub string
		if err := rows.Scan(&npub); err != nil {
			continue
		}
		uploaders = append(uploaders, npub)
	}

	return uploaders, nil
}

// AddFollow adds a follow relationship
func (w *WebOfTrust) AddFollow(followerNpub, followedNpub string, depth int) error {
	db := database.Get()
	_, err := db.Exec(`
		INSERT INTO trust_follows (follower_npub, followed_npub, depth)
		VALUES (?, ?, ?)
		ON CONFLICT(follower_npub, followed_npub) DO UPDATE SET depth = excluded.depth
	`, followerNpub, followedNpub, depth)
	return err
}

// RemoveFollow removes a follow relationship
func (w *WebOfTrust) RemoveFollow(followerNpub, followedNpub string) error {
	db := database.Get()
	_, err := db.Exec(`
		DELETE FROM trust_follows
		WHERE follower_npub = ? AND followed_npub = ?
	`, followerNpub, followedNpub)
	return err
}

// GetFollowers returns all users who follow the given npub
func (w *WebOfTrust) GetFollowers(npub string) ([]string, error) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT follower_npub FROM trust_follows WHERE followed_npub = ?
	`, npub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followers []string
	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			continue
		}
		followers = append(followers, follower)
	}
	return followers, nil
}

// GetFollowing returns all users the given npub follows
func (w *WebOfTrust) GetFollowing(npub string) ([]string, error) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT followed_npub FROM trust_follows WHERE follower_npub = ?
	`, npub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var following []string
	for rows.Next() {
		var followed string
		if err := rows.Scan(&followed); err != nil {
			continue
		}
		following = append(following, followed)
	}
	return following, nil
}

// ClearFollows removes all follows for an npub
func (w *WebOfTrust) ClearFollows(npub string) error {
	db := database.Get()
	_, err := db.Exec(`DELETE FROM trust_follows WHERE follower_npub = ?`, npub)
	return err
}
