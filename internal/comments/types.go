// Package comments handles torrent comments (NIP-35 Kind 2004)
package comments

import (
	"encoding/json"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// Comment represents a torrent comment
type Comment struct {
	ID            string    `json:"id"`
	EventID       string    `json:"event_id"`
	Infohash      string    `json:"infohash"`
	TorrentEventID string   `json:"torrent_event_id,omitempty"`
	AuthorPubkey  string    `json:"author_pubkey"`
	Content       string    `json:"content"`
	Rating        int       `json:"rating,omitempty"` // 1-5 stars
	ParentID      string    `json:"parent_id,omitempty"` // For replies
	RootID        string    `json:"root_id,omitempty"`   // Root comment in thread
	Mentions      []string  `json:"mentions,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	Signature     string    `json:"signature,omitempty"`
}

// KindTorrentComment is the Nostr event kind for torrent comments
const KindTorrentComment = 2004

// NewComment creates a new comment
func NewComment(infohash, content, authorPubkey string) *Comment {
	return &Comment{
		Infohash:     infohash,
		Content:      content,
		AuthorPubkey: authorPubkey,
		CreatedAt:    time.Now().UTC(),
	}
}

// SetTorrentEvent sets the torrent event reference
func (c *Comment) SetTorrentEvent(eventID string) {
	c.TorrentEventID = eventID
}

// SetRating sets the rating (1-5)
func (c *Comment) SetRating(rating int) {
	if rating < 1 {
		rating = 1
	}
	if rating > 5 {
		rating = 5
	}
	c.Rating = rating
}

// SetParent sets the parent comment (for replies)
func (c *Comment) SetParent(parentID, rootID string) {
	c.ParentID = parentID
	c.RootID = rootID
	if c.RootID == "" {
		c.RootID = parentID
	}
}

// IsReply returns true if this is a reply to another comment
func (c *Comment) IsReply() bool {
	return c.ParentID != ""
}

// ToNostrEvent converts the comment to a Nostr event
func (c *Comment) ToNostrEvent(privateKey string) (*nostr.Event, error) {
	event := &nostr.Event{
		Kind:      KindTorrentComment,
		Content:   c.Content,
		CreatedAt: nostr.Timestamp(c.CreatedAt.Unix()),
		Tags: nostr.Tags{
			{"x", c.Infohash}, // Infohash reference
		},
	}

	// Add torrent event reference
	if c.TorrentEventID != "" {
		event.Tags = append(event.Tags, nostr.Tag{"e", c.TorrentEventID, "", "root"})
	}

	// Add reply tags
	if c.ParentID != "" {
		event.Tags = append(event.Tags, nostr.Tag{"e", c.ParentID, "", "reply"})
	}
	if c.RootID != "" && c.RootID != c.ParentID {
		event.Tags = append(event.Tags, nostr.Tag{"e", c.RootID, "", "root"})
	}

	// Add rating tag
	if c.Rating > 0 {
		event.Tags = append(event.Tags, nostr.Tag{"rating", ratingToString(c.Rating)})
	}

	// Add mentions
	for _, pubkey := range c.Mentions {
		event.Tags = append(event.Tags, nostr.Tag{"p", pubkey})
	}

	// Sign the event
	if err := event.Sign(privateKey); err != nil {
		return nil, err
	}

	c.EventID = event.ID
	c.Signature = event.Sig

	return event, nil
}

// FromNostrEvent creates a Comment from a Nostr event
func FromNostrEvent(event *nostr.Event) (*Comment, error) {
	if event.Kind != KindTorrentComment {
		return nil, &InvalidKindError{Expected: KindTorrentComment, Got: event.Kind}
	}

	c := &Comment{
		EventID:      event.ID,
		AuthorPubkey: event.PubKey,
		Content:      event.Content,
		CreatedAt:    time.Unix(int64(event.CreatedAt), 0).UTC(),
		Signature:    event.Sig,
	}

	// Parse tags
	for _, tag := range event.Tags {
		if len(tag) < 2 {
			continue
		}

		switch tag[0] {
		case "x", "btih", "infohash":
			c.Infohash = tag[1]
		case "e":
			// Event reference
			if len(tag) >= 4 {
				switch tag[3] {
				case "root":
					if c.TorrentEventID == "" {
						c.TorrentEventID = tag[1]
					}
					c.RootID = tag[1]
				case "reply":
					c.ParentID = tag[1]
				default:
					// Generic event reference
					if c.TorrentEventID == "" {
						c.TorrentEventID = tag[1]
					}
				}
			} else {
				// No marker, treat as torrent event reference
				if c.TorrentEventID == "" {
					c.TorrentEventID = tag[1]
				}
			}
		case "p":
			c.Mentions = append(c.Mentions, tag[1])
		case "rating":
			c.Rating = stringToRating(tag[1])
		}
	}

	return c, nil
}

// ratingToString converts rating to string
func ratingToString(rating int) string {
	stars := ""
	for i := 0; i < rating; i++ {
		stars += "⭐"
	}
	return stars
}

// stringToRating converts string to rating
func stringToRating(s string) int {
	// Count stars
	count := 0
	for _, r := range s {
		if r == '⭐' {
			count++
		}
	}
	if count > 0 {
		return count
	}

	// Try numeric
	var rating int
	if err := json.Unmarshal([]byte(s), &rating); err == nil {
		return rating
	}

	return 0
}

// InvalidKindError indicates an invalid event kind
type InvalidKindError struct {
	Expected int
	Got      int
}

func (e *InvalidKindError) Error() string {
	return "invalid event kind"
}

// CommentThread represents a thread of comments
type CommentThread struct {
	Root     *Comment
	Replies  []*Comment
	Depth    int
	Children []*CommentThread
}

// CommentFilter for querying comments
type CommentFilter struct {
	Infohash     string
	TorrentEvent string
	AuthorPubkey string
	ParentID     string
	HasRating    bool
	MinRating    int
	Since        *time.Time
	Until        *time.Time
	Limit        int
	Offset       int
}

// CommentStats contains comment statistics for a torrent
type CommentStats struct {
	TotalComments int     `json:"total_comments"`
	TotalRatings  int     `json:"total_ratings"`
	AverageRating float64 `json:"average_rating,omitempty"`
	RatingCounts  map[int]int `json:"rating_counts,omitempty"`
}
