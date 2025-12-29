package indexer

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/lighthouse-client/lighthouse/internal/database"
	"github.com/lighthouse-client/lighthouse/internal/nostr"
	gonostr "github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

// Indexer handles the indexing of torrents from Nostr relays
type Indexer struct {
	relayManager *nostr.RelayManager
	enricher     *Enricher
	deduplicator *Deduplicator
	running      bool
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	stats        IndexerStats
}

// IndexerStats tracks indexer statistics
type IndexerStats struct {
	TorrentsProcessed int64
	TorrentsAdded     int64
	TorrentsDuplicate int64
	EventsReceived    int64
	LastEventAt       time.Time
	StartedAt         time.Time
}

// New creates a new Indexer
func New(relayManager *nostr.RelayManager) *Indexer {
	return &Indexer{
		relayManager: relayManager,
		enricher:     NewEnricher(),
		deduplicator: NewDeduplicator(),
	}
}

// Start begins the indexing process
func (idx *Indexer) Start(ctx context.Context) error {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return nil
	}
	idx.ctx, idx.cancel = context.WithCancel(ctx)
	idx.running = true
	idx.stats.StartedAt = time.Now()
	idx.mu.Unlock()

	log.Info().Msg("Starting indexer")

	// Start relay manager
	if err := idx.relayManager.Start(idx.ctx); err != nil {
		log.Error().Err(err).Msg("Failed to start relay manager")
		return err
	}

	// Subscribe to torrent events
	since := time.Now().Add(-24 * time.Hour) // Start with last 24 hours
	err := idx.relayManager.SubscribeTorrents(idx.ctx, since, func(event *gonostr.Event, relayURL string) {
		idx.processEvent(event, relayURL)
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to subscribe to torrents")
		return err
	}

	// Start background tasks
	go idx.runBackgroundTasks()

	log.Info().Msg("Indexer started")
	database.LogActivity("indexer_started", "")

	return nil
}

// Stop stops the indexing process
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if !idx.running {
		return
	}

	log.Info().Msg("Stopping indexer")

	if idx.cancel != nil {
		idx.cancel()
	}

	idx.relayManager.Stop()
	idx.running = false

	database.LogActivity("indexer_stopped", "")
	log.Info().Msg("Indexer stopped")
}

// IsRunning returns whether the indexer is running
func (idx *Indexer) IsRunning() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.running
}

// GetStats returns indexer statistics
func (idx *Indexer) GetStats() IndexerStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.stats
}

// processEvent handles a single torrent event
func (idx *Indexer) processEvent(event *gonostr.Event, relayURL string) {
	idx.mu.Lock()
	idx.stats.EventsReceived++
	idx.stats.LastEventAt = time.Now()
	idx.mu.Unlock()

	// Parse the event
	torrentEvent, err := nostr.ParseTorrentEvent(event)
	if err != nil || torrentEvent == nil {
		return
	}

	// Skip if no info hash
	if torrentEvent.InfoHash == "" {
		log.Debug().Str("event_id", event.ID).Msg("Skipping event without info hash")
		return
	}

	// Check if uploader is blacklisted
	if idx.isBlacklisted(torrentEvent.Pubkey) {
		log.Debug().Str("pubkey", torrentEvent.Pubkey).Msg("Skipping blacklisted uploader")
		return
	}

	// Check tag filter
	if !idx.matchesTagFilter(torrentEvent) {
		log.Debug().
			Str("info_hash", torrentEvent.InfoHash).
			Strs("tags", torrentEvent.ContentTags).
			Msg("Skipping torrent that doesn't match tag filter")
		return
	}

	// Process with deduplicator
	isNew, err := idx.deduplicator.Process(torrentEvent, relayURL)
	if err != nil {
		log.Error().Err(err).Str("info_hash", torrentEvent.InfoHash).Msg("Failed to process torrent")
		return
	}

	idx.mu.Lock()
	idx.stats.TorrentsProcessed++
	if isNew {
		idx.stats.TorrentsAdded++
	} else {
		idx.stats.TorrentsDuplicate++
	}
	idx.mu.Unlock()

	// Enrich metadata for new torrents
	if isNew {
		go idx.enricher.EnrichTorrent(torrentEvent.InfoHash)
	}
}

// isBlacklisted checks if a pubkey is blacklisted
func (idx *Indexer) isBlacklisted(pubkey string) bool {
	db := database.Get()
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM trust_blacklist WHERE npub = ?", pubkey).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// matchesTagFilter checks if a torrent matches the configured tag filter
func (idx *Indexer) matchesTagFilter(torrentEvent *nostr.TorrentEvent) bool {
	cfg := config.Get()

	// If tag filtering is disabled or no tags configured, allow all
	if !cfg.Indexer.TagFilterEnabled || len(cfg.Indexer.TagFilter) == 0 {
		return true
	}

	// Build a set of tags to check (content tags + category)
	eventTags := make(map[string]bool)
	for _, tag := range torrentEvent.ContentTags {
		eventTags[strings.ToLower(tag)] = true
	}
	// Also check category field as a tag
	if torrentEvent.Category != "" {
		eventTags[strings.ToLower(torrentEvent.Category)] = true
	}

	// Check if any configured filter tag matches
	for _, filterTag := range cfg.Indexer.TagFilter {
		if eventTags[strings.ToLower(filterTag)] {
			return true
		}
	}

	return false
}

// runBackgroundTasks runs periodic background tasks
func (idx *Indexer) runBackgroundTasks() {
	// Reconnect ticker
	reconnectTicker := time.NewTicker(5 * time.Minute)
	defer reconnectTicker.Stop()

	// Stats ticker
	statsTicker := time.NewTicker(1 * time.Minute)
	defer statsTicker.Stop()

	// Enrichment ticker
	enrichTicker := time.NewTicker(10 * time.Minute)
	defer enrichTicker.Stop()

	for {
		select {
		case <-idx.ctx.Done():
			return

		case <-reconnectTicker.C:
			// Try to reconnect disconnected relays
			idx.relayManager.ReconnectAll()

		case <-statsTicker.C:
			// Log stats
			stats := idx.GetStats()
			log.Info().
				Int64("processed", stats.TorrentsProcessed).
				Int64("added", stats.TorrentsAdded).
				Int64("duplicate", stats.TorrentsDuplicate).
				Int("relays", idx.relayManager.ConnectedCount()).
				Msg("Indexer stats")

		case <-enrichTicker.C:
			// Enrich pending torrents
			go idx.enrichPendingTorrents()
		}
	}
}

// enrichPendingTorrents finds and enriches torrents without metadata
func (idx *Indexer) enrichPendingTorrents() {
	db := database.Get()
	rows, err := db.Query(`
		SELECT info_hash FROM torrents
		WHERE title IS NULL OR title = ''
		ORDER BY first_seen_at DESC
		LIMIT 100
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var infoHash string
		if err := rows.Scan(&infoHash); err != nil {
			continue
		}
		idx.enricher.EnrichTorrent(infoHash)
	}
}

// FetchHistorical fetches historical torrents from relays
func (idx *Indexer) FetchHistorical(days int) error {
	if !idx.IsRunning() {
		return nil
	}

	since := time.Now().AddDate(0, 0, -days)
	log.Info().Int("days", days).Msg("Fetching historical torrents")

	return idx.relayManager.SubscribeTorrents(idx.ctx, since, func(event *gonostr.Event, relayURL string) {
		idx.processEvent(event, relayURL)
	})
}

// ImportContactList imports follows from a contact list
func (idx *Indexer) ImportContactList(npub string) error {
	// Convert npub to hex pubkey
	pubkey, err := nostr.NpubToHex(npub)
	if err != nil {
		return err
	}

	// Fetch contact list
	event, err := idx.relayManager.FetchContactList(idx.ctx, pubkey)
	if err != nil {
		return err
	}

	// Parse contacts
	contacts := nostr.ParseContactList(event)
	if len(contacts) == 0 {
		return nil
	}

	// Store follows in database
	db := database.Get()
	for _, contact := range contacts {
		contactNpub, err := nostr.HexToNpub(contact)
		if err != nil {
			continue
		}

		_, err = db.Exec(`
			INSERT INTO trust_follows (follower_npub, followed_npub, depth)
			VALUES (?, ?, 1)
			ON CONFLICT(follower_npub, followed_npub) DO NOTHING
		`, npub, contactNpub)
		if err != nil {
			log.Error().Err(err).Msg("Failed to store follow")
		}
	}

	log.Info().Int("contacts", len(contacts)).Str("npub", npub).Msg("Imported contact list")
	database.LogActivity("contacts_imported", npub)

	return nil
}
