package apikeys

import (
	"strings"
	"testing"
	"time"
)

func TestNewAPIKey(t *testing.T) {
	perms := []Permission{PermissionRead, PermissionTorznab}
	key, plaintext := NewAPIKey("Test Key", perms, "test-user")

	// Check key is not nil
	if key == nil {
		t.Fatal("Expected key to be created")
	}

	// Check plaintext key format
	if !strings.HasPrefix(plaintext, "lh_") {
		t.Errorf("Expected plaintext to start with 'lh_', got %s", plaintext)
	}

	// Check key prefix matches
	if !strings.HasPrefix(plaintext, key.KeyPrefix) {
		t.Errorf("Key prefix %s doesn't match start of plaintext", key.KeyPrefix)
	}

	// Check fields are set correctly
	if key.Name != "Test Key" {
		t.Errorf("Expected name 'Test Key', got %s", key.Name)
	}

	if key.CreatedBy != "test-user" {
		t.Errorf("Expected created_by 'test-user', got %s", key.CreatedBy)
	}

	if !key.Enabled {
		t.Error("Expected key to be enabled by default")
	}

	if len(key.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(key.Permissions))
	}

	// Check ID is generated
	if key.ID == "" {
		t.Error("Expected ID to be generated")
	}

	// Check hash is generated
	if key.KeyHash == "" {
		t.Error("Expected hash to be generated")
	}
}

func TestHashKey(t *testing.T) {
	key1 := HashKey("test-key-1")
	key2 := HashKey("test-key-2")
	key1Again := HashKey("test-key-1")

	// Same input should produce same hash
	if key1 != key1Again {
		t.Error("Same input should produce same hash")
	}

	// Different input should produce different hash
	if key1 == key2 {
		t.Error("Different input should produce different hash")
	}

	// Hash should be 64 characters (SHA256 hex)
	if len(key1) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(key1))
	}
}

func TestAPIKey_HasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []Permission
		check       Permission
		expected    bool
	}{
		{
			name:        "has read permission",
			permissions: []Permission{PermissionRead, PermissionWrite},
			check:       PermissionRead,
			expected:    true,
		},
		{
			name:        "missing permission",
			permissions: []Permission{PermissionRead},
			check:       PermissionWrite,
			expected:    false,
		},
		{
			name:        "admin has all permissions",
			permissions: []Permission{PermissionAdmin},
			check:       PermissionCurator,
			expected:    true,
		},
		{
			name:        "empty permissions",
			permissions: []Permission{},
			check:       PermissionRead,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{Permissions: tt.permissions}
			result := key.HasPermission(tt.check)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAPIKey_HasAnyPermission(t *testing.T) {
	key := &APIKey{Permissions: []Permission{PermissionRead, PermissionTorznab}}

	// Should have one of these
	if !key.HasAnyPermission(PermissionRead, PermissionWrite) {
		t.Error("Expected to have at least read permission")
	}

	// Should not have any of these
	if key.HasAnyPermission(PermissionAdmin, PermissionCurator) {
		t.Error("Expected not to have admin or curator permission")
	}
}

func TestAPIKey_IsValid(t *testing.T) {
	now := time.Now()
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name      string
		enabled   bool
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "enabled, no expiry",
			enabled:   true,
			expiresAt: nil,
			expected:  true,
		},
		{
			name:      "disabled",
			enabled:   false,
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "expired",
			enabled:   true,
			expiresAt: &past,
			expected:  false,
		},
		{
			name:      "enabled, not expired",
			enabled:   true,
			expiresAt: &future,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{Enabled: tt.enabled, ExpiresAt: tt.expiresAt}
			result := key.IsValid()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAPIKey_UpdateLastUsed(t *testing.T) {
	key := &APIKey{}

	if key.LastUsedAt != nil {
		t.Error("Expected LastUsedAt to be nil initially")
	}

	key.UpdateLastUsed()

	if key.LastUsedAt == nil {
		t.Error("Expected LastUsedAt to be set")
	}

	// Should be within last second
	if time.Since(*key.LastUsedAt) > time.Second {
		t.Error("LastUsedAt should be recent")
	}
}

func TestAPIKey_SetExpiry(t *testing.T) {
	key := &APIKey{}
	expiry := time.Now().Add(7 * 24 * time.Hour)

	key.SetExpiry(expiry)

	if key.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set")
	}

	if !key.ExpiresAt.Equal(expiry) {
		t.Errorf("Expected expiry %v, got %v", expiry, *key.ExpiresAt)
	}
}

func TestAPIKey_EnableDisable(t *testing.T) {
	key := &APIKey{Enabled: true}

	key.Disable()
	if key.Enabled {
		t.Error("Expected key to be disabled")
	}

	key.Enable()
	if !key.Enabled {
		t.Error("Expected key to be enabled")
	}
}

func TestDefaultPermissions(t *testing.T) {
	perms := DefaultPermissions()

	if len(perms) != 2 {
		t.Errorf("Expected 2 default permissions, got %d", len(perms))
	}

	hasRead := false
	hasTorznab := false
	for _, p := range perms {
		if p == PermissionRead {
			hasRead = true
		}
		if p == PermissionTorznab {
			hasTorznab = true
		}
	}

	if !hasRead {
		t.Error("Expected default permissions to include read")
	}
	if !hasTorznab {
		t.Error("Expected default permissions to include torznab")
	}
}

func TestAllPermissions(t *testing.T) {
	perms := AllPermissions()

	// Should have all 8 permissions
	if len(perms) != 8 {
		t.Errorf("Expected 8 permissions, got %d", len(perms))
	}
}
