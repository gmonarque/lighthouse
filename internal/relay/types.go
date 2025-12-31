package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/nbd-wtf/go-nostr"
)

// Event represents a Nostr event
type Event struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// Filter represents a Nostr subscription filter
type Filter struct {
	IDs     []string            `json:"ids,omitempty"`
	Authors []string            `json:"authors,omitempty"`
	Kinds   []int               `json:"kinds,omitempty"`
	Tags    map[string][]string `json:"-"` // Parsed from #e, #p, etc.
	Since   int64               `json:"since,omitempty"`
	Until   int64               `json:"until,omitempty"`
	Limit   int                 `json:"limit,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for Filter
func (f *Filter) UnmarshalJSON(data []byte) error {
	// First unmarshal to a generic map
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Parse known fields
	if v, ok := raw["ids"]; ok {
		json.Unmarshal(v, &f.IDs)
	}
	if v, ok := raw["authors"]; ok {
		json.Unmarshal(v, &f.Authors)
	}
	if v, ok := raw["kinds"]; ok {
		json.Unmarshal(v, &f.Kinds)
	}
	if v, ok := raw["since"]; ok {
		json.Unmarshal(v, &f.Since)
	}
	if v, ok := raw["until"]; ok {
		json.Unmarshal(v, &f.Until)
	}
	if v, ok := raw["limit"]; ok {
		json.Unmarshal(v, &f.Limit)
	}

	// Parse tag filters (#e, #p, #t, etc.)
	f.Tags = make(map[string][]string)
	for key, value := range raw {
		if len(key) == 2 && key[0] == '#' {
			var tagValues []string
			if err := json.Unmarshal(value, &tagValues); err == nil {
				f.Tags[string(key[1])] = tagValues
			}
		}
	}

	return nil
}

// GetID computes the event ID
func (e *Event) GetID() string {
	// Serialize for hashing
	serialized := e.Serialize()
	hash := sha256.Sum256(serialized)
	return hex.EncodeToString(hash[:])
}

// Serialize serializes the event for hashing
func (e *Event) Serialize() []byte {
	// Format: [0, pubkey, created_at, kind, tags, content]
	data := []interface{}{
		0,
		e.PubKey,
		e.CreatedAt,
		e.Kind,
		e.Tags,
		e.Content,
	}
	serialized, _ := json.Marshal(data)
	return serialized
}

// VerifySignature verifies the event signature
func (e *Event) VerifySignature() bool {
	// Convert to go-nostr event and verify
	event := &nostr.Event{
		ID:        e.ID,
		PubKey:    e.PubKey,
		CreatedAt: nostr.Timestamp(e.CreatedAt),
		Kind:      e.Kind,
		Content:   e.Content,
		Sig:       e.Sig,
	}

	// Convert tags
	for _, tag := range e.Tags {
		event.Tags = append(event.Tags, nostr.Tag(tag))
	}

	valid, err := event.CheckSignature()
	return err == nil && valid
}

// ToNostrEvent converts to a go-nostr Event
func (e *Event) ToNostrEvent() *nostr.Event {
	event := &nostr.Event{
		ID:        e.ID,
		PubKey:    e.PubKey,
		CreatedAt: nostr.Timestamp(e.CreatedAt),
		Kind:      e.Kind,
		Content:   e.Content,
		Sig:       e.Sig,
	}

	for _, tag := range e.Tags {
		event.Tags = append(event.Tags, nostr.Tag(tag))
	}

	return event
}

// FromNostrEvent creates an Event from a go-nostr Event
func FromNostrEvent(event *nostr.Event) *Event {
	e := &Event{
		ID:        event.ID,
		PubKey:    event.PubKey,
		CreatedAt: int64(event.CreatedAt),
		Kind:      event.Kind,
		Content:   event.Content,
		Sig:       event.Sig,
	}

	for _, tag := range event.Tags {
		e.Tags = append(e.Tags, []string(tag))
	}

	return e
}

// GetTagValue returns the first value for a tag
func (e *Event) GetTagValue(tagName string) string {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == tagName {
			return tag[1]
		}
	}
	return ""
}

// GetTagValues returns all values for a tag
func (e *Event) GetTagValues(tagName string) []string {
	var values []string
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == tagName {
			values = append(values, tag[1])
		}
	}
	return values
}

// IsTorrentEvent checks if this is a torrent event (kind 2003)
func (e *Event) IsTorrentEvent() bool {
	return e.Kind == 2003
}

// IsCommentEvent checks if this is a comment event (kind 2004)
func (e *Event) IsCommentEvent() bool {
	return e.Kind == 2004
}

// IsDecisionEvent checks if this is a decision event (kind 30175)
func (e *Event) IsDecisionEvent() bool {
	return e.Kind == 30175
}

// GetInfohash returns the infohash from a torrent event
func (e *Event) GetInfohash() string {
	// Try common tag names
	for _, tagName := range []string{"x", "btih", "infohash"} {
		if v := e.GetTagValue(tagName); v != "" {
			return v
		}
	}
	return ""
}

// RelayInfo represents relay information (NIP-11)
type RelayInfo struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Pubkey        string   `json:"pubkey,omitempty"`
	Contact       string   `json:"contact,omitempty"`
	SupportedNIPs []int    `json:"supported_nips,omitempty"`
	Software      string   `json:"software,omitempty"`
	Version       string   `json:"version,omitempty"`
	Limitation    *Limits  `json:"limitation,omitempty"`
}

// Limits represents relay limitations
type Limits struct {
	MaxMessageLength   int   `json:"max_message_length,omitempty"`
	MaxSubscriptions   int   `json:"max_subscriptions,omitempty"`
	MaxFilters         int   `json:"max_filters,omitempty"`
	MaxEventTags       int   `json:"max_event_tags,omitempty"`
	MaxContentLength   int   `json:"max_content_length,omitempty"`
	MinPowDifficulty   int   `json:"min_pow_difficulty,omitempty"`
	AuthRequired       bool  `json:"auth_required,omitempty"`
	PaymentRequired    bool  `json:"payment_required,omitempty"`
	CreatedAtLowerLimit int64 `json:"created_at_lower_limit,omitempty"`
	CreatedAtUpperLimit int64 `json:"created_at_upper_limit,omitempty"`
}

// SortEventsByCreatedAt sorts events by created_at descending
func SortEventsByCreatedAt(events []*Event) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})
}
