// Package apikeys implements multi-user API key management
package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// APIKey represents an API key for authentication
type APIKey struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	KeyHash     string       `json:"-"`          // SHA256 hash of the actual key, never exposed
	KeyPrefix   string       `json:"key_prefix"` // First 8 chars for identification
	Permissions []Permission `json:"permissions"`
	RateLimit   int          `json:"rate_limit,omitempty"` // Custom rate limit (0 = default)
	CreatedBy   string       `json:"created_by"`
	CreatedAt   time.Time    `json:"created_at"`
	LastUsedAt  *time.Time   `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Enabled     bool         `json:"enabled"`
	Notes       string       `json:"notes,omitempty"`
}

// Permission represents an API permission
type Permission string

const (
	PermissionRead       Permission = "read"       // Search, view torrents
	PermissionWrite      Permission = "write"      // Publish, modify
	PermissionAdmin      Permission = "admin"      // Full access
	PermissionTorznab    Permission = "torznab"    // Torznab API access
	PermissionCurator    Permission = "curator"    // Curation decisions
	PermissionModeration Permission = "moderation" // Handle reports
	PermissionSettings   Permission = "settings"   // Modify settings
	PermissionRelays     Permission = "relays"     // Manage relays
)

// NewAPIKey creates a new API key and returns the plaintext key (only shown once)
func NewAPIKey(name string, permissions []Permission, createdBy string) (*APIKey, string) {
	// Generate a secure random key
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	plaintextKey := "lh_" + hex.EncodeToString(keyBytes) // Prefix for easy identification

	// Hash the key for storage
	hash := sha256.Sum256([]byte(plaintextKey))
	keyHash := hex.EncodeToString(hash[:])

	key := &APIKey{
		ID:          generateKeyID(),
		Name:        name,
		KeyHash:     keyHash,
		KeyPrefix:   plaintextKey[:11], // "lh_" + first 8 hex chars
		Permissions: permissions,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
		Enabled:     true,
	}

	return key, plaintextKey
}

// generateKeyID creates a unique key ID
func generateKeyID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// HashKey returns the SHA256 hash of a plaintext key
func HashKey(plaintextKey string) string {
	hash := sha256.Sum256([]byte(plaintextKey))
	return hex.EncodeToString(hash[:])
}

// HasPermission checks if the key has a specific permission
func (k *APIKey) HasPermission(perm Permission) bool {
	// Admin has all permissions
	for _, p := range k.Permissions {
		if p == PermissionAdmin || p == perm {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if the key has any of the specified permissions
func (k *APIKey) HasAnyPermission(perms ...Permission) bool {
	for _, perm := range perms {
		if k.HasPermission(perm) {
			return true
		}
	}
	return false
}

// IsValid checks if the key is valid (enabled and not expired)
func (k *APIKey) IsValid() bool {
	if !k.Enabled {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}

// UpdateLastUsed updates the last used timestamp
func (k *APIKey) UpdateLastUsed() {
	now := time.Now().UTC()
	k.LastUsedAt = &now
}

// SetExpiry sets the expiry time
func (k *APIKey) SetExpiry(expiresAt time.Time) {
	k.ExpiresAt = &expiresAt
}

// Disable disables the key
func (k *APIKey) Disable() {
	k.Enabled = false
}

// Enable enables the key
func (k *APIKey) Enable() {
	k.Enabled = true
}

// DefaultPermissions returns default permissions for new users
func DefaultPermissions() []Permission {
	return []Permission{PermissionRead, PermissionTorznab}
}

// AdminPermissions returns all permissions
func AdminPermissions() []Permission {
	return []Permission{PermissionAdmin}
}

// AllPermissions returns list of all available permissions
func AllPermissions() []Permission {
	return []Permission{
		PermissionRead,
		PermissionWrite,
		PermissionAdmin,
		PermissionTorznab,
		PermissionCurator,
		PermissionModeration,
		PermissionSettings,
		PermissionRelays,
	}
}
