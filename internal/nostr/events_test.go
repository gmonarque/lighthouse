package nostr

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestParseTorrentEvent(t *testing.T) {
	event := &nostr.Event{
		ID:        "test123",
		PubKey:    "pubkey123",
		Kind:      KindTorrent,
		CreatedAt: 1700000000,
		Content:   "magnet:?xt=urn:btih:abc123def456&dn=Test+Torrent",
		Tags: nostr.Tags{
			{"title", "Test Movie"},
			{"x", "abc123def456"},
			{"size", "1234567890"},
			{"category", "movie"},
		},
	}

	result, err := ParseTorrentEvent(event)
	if err != nil {
		t.Fatalf("ParseTorrentEvent failed: %v", err)
	}

	if result.EventID != "test123" {
		t.Errorf("EventID = %q, want %q", result.EventID, "test123")
	}

	if result.Pubkey != "pubkey123" {
		t.Errorf("Pubkey = %q, want %q", result.Pubkey, "pubkey123")
	}

	if result.Name != "Test Movie" {
		t.Errorf("Name = %q, want %q", result.Name, "Test Movie")
	}

	if result.InfoHash != "abc123def456" {
		t.Errorf("InfoHash = %q, want %q", result.InfoHash, "abc123def456")
	}

	if result.Size != 1234567890 {
		t.Errorf("Size = %d, want %d", result.Size, 1234567890)
	}

	if result.Category != "movie" {
		t.Errorf("Category = %q, want %q", result.Category, "movie")
	}
}

func TestParseTorrentEvent_WrongKind(t *testing.T) {
	event := &nostr.Event{
		Kind: 1, // Text note, not torrent
	}

	result, err := ParseTorrentEvent(event)
	if err != nil {
		t.Fatalf("ParseTorrentEvent failed: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for wrong event kind")
	}
}

func TestExtractInfoHash(t *testing.T) {
	tests := []struct {
		magnet   string
		expected string
	}{
		{
			"magnet:?xt=urn:btih:abc123def456789012345678901234567890abcd&dn=Test",
			"abc123def456789012345678901234567890abcd",
		},
		{
			"magnet:?xt=urn:btih:ABC123DEF456789012345678901234567890ABCD&dn=Test",
			"abc123def456789012345678901234567890abcd",
		},
		{
			"invalid",
			"",
		},
		{
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.magnet, func(t *testing.T) {
			result := extractInfoHash(tt.magnet)
			if result != tt.expected {
				t.Errorf("extractInfoHash(%q) = %q, want %q", tt.magnet, result, tt.expected)
			}
		})
	}
}

func TestExtractNameFromMagnet(t *testing.T) {
	tests := []struct {
		magnet   string
		expected string
	}{
		{
			"magnet:?xt=urn:btih:abc123&dn=Test+Torrent+Name",
			"Test Torrent Name",
		},
		{
			"magnet:?xt=urn:btih:abc123&dn=Test%20Torrent",
			"Test Torrent",
		},
		{
			"magnet:?xt=urn:btih:abc123&dn=Test.Torrent.2024",
			"Test Torrent 2024",
		},
		{
			"magnet:?xt=urn:btih:abc123",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.magnet, func(t *testing.T) {
			result := extractNameFromMagnet(tt.magnet)
			if result != tt.expected {
				t.Errorf("extractNameFromMagnet(%q) = %q, want %q", tt.magnet, result, tt.expected)
			}
		})
	}
}

func TestCategoryFromNostrTags(t *testing.T) {
	tests := []struct {
		tags     []string
		expected int
	}{
		{[]string{"movie"}, 2000},
		{[]string{"tv"}, 5000},
		{[]string{"music"}, 3000},
		{[]string{"game"}, 4050},
		{[]string{"software"}, 4000},
		{[]string{"book"}, 7020},
		{[]string{"xxx"}, 6000},
		{[]string{"unknown"}, 8000},
		// Hierarchical tests
		{[]string{"movie", "4k"}, 2045},
		{[]string{"movie", "hd"}, 2040},
		{[]string{"tv", "anime"}, 5070},
		{[]string{"music", "flac"}, 3040},
	}

	for _, tt := range tests {
		t.Run(tt.tags[0], func(t *testing.T) {
			result := CategoryFromNostrTags(tt.tags)
			if result != tt.expected {
				t.Errorf("CategoryFromNostrTags(%v) = %d, want %d", tt.tags, result, tt.expected)
			}
		})
	}
}

func TestParseContactList(t *testing.T) {
	event := &nostr.Event{
		Kind: KindContactList,
		Tags: nostr.Tags{
			{"p", "pubkey1", "wss://relay1.com"},
			{"p", "pubkey2", "wss://relay2.com"},
			{"p", "pubkey3"},
			{"e", "event1"}, // Not a contact
		},
	}

	result := ParseContactList(event)
	if len(result) != 3 {
		t.Errorf("ParseContactList returned %d contacts, want 3", len(result))
	}

	expected := []string{"pubkey1", "pubkey2", "pubkey3"}
	for i, pk := range expected {
		if result[i] != pk {
			t.Errorf("ParseContactList()[%d] = %q, want %q", i, result[i], pk)
		}
	}
}
