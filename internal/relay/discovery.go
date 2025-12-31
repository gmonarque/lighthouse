package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

// RelayDiscovery handles relay discovery via Nostr
type RelayDiscovery struct {
	mu sync.RWMutex

	// Our relay info
	relayURL  string
	relayInfo *RelayInfo

	// Discovered relays
	knownRelays map[string]*DiscoveredRelay

	// Connected relay pool
	pool *nostr.SimplePool

	// Configuration
	enabled       bool
	announceEvery time.Duration
	scanEvery     time.Duration

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// DiscoveredRelay represents a discovered relay
type DiscoveredRelay struct {
	URL          string
	Info         *RelayInfo
	DiscoveredAt time.Time
	LastSeen     time.Time
	Healthy      bool
	EventCount   int64
	Latency      time.Duration
}

// DiscoveryConfig holds relay discovery configuration
type DiscoveryConfig struct {
	RelayURL      string
	RelayInfo     *RelayInfo
	AnnounceEvery time.Duration
	ScanEvery     time.Duration
	BootstrapURLs []string
}

// NewRelayDiscovery creates a new relay discovery service
func NewRelayDiscovery(cfg DiscoveryConfig) *RelayDiscovery {
	if cfg.AnnounceEvery == 0 {
		cfg.AnnounceEvery = 1 * time.Hour
	}
	if cfg.ScanEvery == 0 {
		cfg.ScanEvery = 15 * time.Minute
	}

	d := &RelayDiscovery{
		relayURL:      cfg.RelayURL,
		relayInfo:     cfg.RelayInfo,
		knownRelays:   make(map[string]*DiscoveredRelay),
		enabled:       true,
		announceEvery: cfg.AnnounceEvery,
		scanEvery:     cfg.ScanEvery,
		pool:          nostr.NewSimplePool(context.Background()),
	}

	// Add bootstrap relays
	for _, url := range cfg.BootstrapURLs {
		d.knownRelays[url] = &DiscoveredRelay{
			URL:          url,
			DiscoveredAt: time.Now(),
			LastSeen:     time.Now(),
			Healthy:      true,
		}
	}

	return d
}

// Start starts the relay discovery service
func (d *RelayDiscovery) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("relay discovery already running")
	}
	d.running = true
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.mu.Unlock()

	log.Info().
		Str("relay_url", d.relayURL).
		Dur("announce_every", d.announceEvery).
		Dur("scan_every", d.scanEvery).
		Msg("Starting relay discovery")

	// Start background tasks
	go d.announceLoop()
	go d.scanLoop()

	return nil
}

// Stop stops the relay discovery service
func (d *RelayDiscovery) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	d.running = false
	if d.cancel != nil {
		d.cancel()
	}

	log.Info().Msg("Relay discovery stopped")
}

// announceLoop periodically announces our relay
func (d *RelayDiscovery) announceLoop() {
	ticker := time.NewTicker(d.announceEvery)
	defer ticker.Stop()

	// Initial announce
	d.announce()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.announce()
		}
	}
}

// scanLoop periodically scans for new relays
func (d *RelayDiscovery) scanLoop() {
	ticker := time.NewTicker(d.scanEvery)
	defer ticker.Stop()

	// Initial scan
	d.scan()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.scan()
		}
	}
}

// announce announces our relay to known relays
func (d *RelayDiscovery) announce() {
	if d.relayURL == "" || d.relayInfo == nil {
		return
	}

	log.Debug().Str("url", d.relayURL).Msg("Announcing relay")

	// Create announcement event
	// Kind 30166 = Relay announcement (inspired by NIP-65)
	event := &nostr.Event{
		Kind:      30166,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"d", d.relayURL},
			{"r", d.relayURL},
			{"name", d.relayInfo.Name},
		},
	}

	if d.relayInfo.Description != "" {
		event.Tags = append(event.Tags, nostr.Tag{"description", d.relayInfo.Description})
	}

	// Add supported NIPs
	for _, nip := range d.relayInfo.SupportedNIPs {
		event.Tags = append(event.Tags, nostr.Tag{"nip", fmt.Sprintf("%d", nip)})
	}

	// Content is relay info JSON
	content, _ := json.Marshal(d.relayInfo)
	event.Content = string(content)

	// Note: Event should be signed before publishing
	// For now, just log the intent
	log.Debug().
		Str("url", d.relayURL).
		Int("known_relays", len(d.knownRelays)).
		Msg("Would publish relay announcement")
}

// scan scans known relays for other relay announcements
func (d *RelayDiscovery) scan() {
	log.Debug().Int("known_relays", len(d.knownRelays)).Msg("Scanning for relays")

	d.mu.RLock()
	urls := make([]string, 0, len(d.knownRelays))
	for url := range d.knownRelays {
		urls = append(urls, url)
	}
	d.mu.RUnlock()

	// Query for relay announcements
	filters := []nostr.Filter{{
		Kinds: []int{30166},
		Limit: 100,
	}}

	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	for event := range d.pool.SubManyEose(ctx, urls, filters) {
		d.processRelayAnnouncement(event.Event)
	}
}

// processRelayAnnouncement processes a relay announcement event
func (d *RelayDiscovery) processRelayAnnouncement(event *nostr.Event) {
	// Extract relay URL from tags
	var relayURL string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "r" {
			relayURL = tag[1]
			break
		}
	}

	if relayURL == "" || relayURL == d.relayURL {
		return
	}

	// Parse relay info from content
	var info RelayInfo
	if err := json.Unmarshal([]byte(event.Content), &info); err != nil {
		// Try to extract basic info from tags
		for _, tag := range event.Tags {
			if len(tag) >= 2 {
				switch tag[0] {
				case "name":
					info.Name = tag[1]
				case "description":
					info.Description = tag[1]
				}
			}
		}
	}

	d.mu.Lock()
	if existing, ok := d.knownRelays[relayURL]; ok {
		existing.LastSeen = time.Now()
		existing.Info = &info
	} else {
		d.knownRelays[relayURL] = &DiscoveredRelay{
			URL:          relayURL,
			Info:         &info,
			DiscoveredAt: time.Now(),
			LastSeen:     time.Now(),
			Healthy:      true,
		}
		log.Info().Str("url", relayURL).Msg("Discovered new relay")
	}
	d.mu.Unlock()
}

// AddRelay manually adds a relay
func (d *RelayDiscovery) AddRelay(url string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.knownRelays[url]; !ok {
		d.knownRelays[url] = &DiscoveredRelay{
			URL:          url,
			DiscoveredAt: time.Now(),
			LastSeen:     time.Now(),
			Healthy:      true,
		}
	}
}

// RemoveRelay removes a relay
func (d *RelayDiscovery) RemoveRelay(url string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.knownRelays, url)
}

// GetRelays returns all known relays
func (d *RelayDiscovery) GetRelays() []*DiscoveredRelay {
	d.mu.RLock()
	defer d.mu.RUnlock()

	relays := make([]*DiscoveredRelay, 0, len(d.knownRelays))
	for _, relay := range d.knownRelays {
		relays = append(relays, relay)
	}
	return relays
}

// GetHealthyRelays returns healthy relays
func (d *RelayDiscovery) GetHealthyRelays() []*DiscoveredRelay {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var relays []*DiscoveredRelay
	for _, relay := range d.knownRelays {
		if relay.Healthy {
			relays = append(relays, relay)
		}
	}
	return relays
}

// CheckHealth checks the health of all known relays
func (d *RelayDiscovery) CheckHealth() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, relay := range d.knownRelays {
		// Check if relay has been seen recently
		if time.Since(relay.LastSeen) > 24*time.Hour {
			relay.Healthy = false
		}
	}
}

// PruneStale removes relays not seen in a while
func (d *RelayDiscovery) PruneStale(maxAge time.Duration) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	pruned := 0
	for url, relay := range d.knownRelays {
		if time.Since(relay.LastSeen) > maxAge {
			delete(d.knownRelays, url)
			pruned++
		}
	}

	if pruned > 0 {
		log.Info().Int("pruned", pruned).Msg("Pruned stale relays")
	}

	return pruned
}

// SyncWith syncs events with specific relays (inbound: pulls events FROM other relays)
func (d *RelayDiscovery) SyncWith(urls []string, kinds []int, since int64) error {
	log.Info().
		Strs("urls", urls).
		Ints("kinds", kinds).
		Int64("since", since).
		Msg("Syncing with relays (inbound)")

	sinceTs := nostr.Timestamp(since)
	filters := []nostr.Filter{{
		Kinds: kinds,
		Since: &sinceTs,
	}}

	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Minute)
	defer cancel()

	eventCount := 0
	for event := range d.pool.SubManyEose(ctx, urls, filters) {
		// Process event (would be passed to storage/indexer)
		eventCount++
		_ = event
	}

	log.Info().Int("events", eventCount).Msg("Inbound sync completed")
	return nil
}

// PushTo pushes events TO other relays (outbound sync)
func (d *RelayDiscovery) PushTo(urls []string, events []*nostr.Event) error {
	if len(events) == 0 {
		return nil
	}

	log.Info().
		Strs("urls", urls).
		Int("event_count", len(events)).
		Msg("Pushing events to relays (outbound)")

	ctx, cancel := context.WithTimeout(d.ctx, 2*time.Minute)
	defer cancel()

	successCount := 0
	failCount := 0

	for _, url := range urls {
		relay, err := d.pool.EnsureRelay(url)
		if err != nil {
			log.Warn().Err(err).Str("url", url).Msg("Failed to connect to relay for push")
			failCount++
			continue
		}

		for _, event := range events {
			if err := relay.Publish(ctx, *event); err != nil {
				log.Debug().Err(err).
					Str("url", url).
					Str("event_id", event.ID).
					Msg("Failed to push event")
				failCount++
			} else {
				successCount++
			}
		}
	}

	log.Info().
		Int("success", successCount).
		Int("failed", failCount).
		Msg("Outbound sync completed")

	return nil
}

// BiDirectionalSync performs bi-directional sync with specified relays
// This pulls new events from other relays AND pushes our events to them
func (d *RelayDiscovery) BiDirectionalSync(urls []string, kinds []int, since int64, localEvents []*nostr.Event) error {
	log.Info().
		Strs("urls", urls).
		Int("local_events", len(localEvents)).
		Msg("Starting bi-directional sync")

	// Inbound: Pull events from other relays
	if err := d.SyncWith(urls, kinds, since); err != nil {
		log.Warn().Err(err).Msg("Inbound sync failed")
	}

	// Outbound: Push our events to other relays
	if err := d.PushTo(urls, localEvents); err != nil {
		log.Warn().Err(err).Msg("Outbound sync failed")
	}

	return nil
}

// SyncLoop runs continuous bi-directional sync at regular intervals
func (d *RelayDiscovery) SyncLoop(interval time.Duration, kinds []int, eventProvider func() []*nostr.Event) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Track last sync time
	lastSync := time.Now().Add(-24 * time.Hour)

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			urls := make([]string, 0)
			d.mu.RLock()
			for url, relay := range d.knownRelays {
				if relay.Healthy {
					urls = append(urls, url)
				}
			}
			d.mu.RUnlock()

			if len(urls) == 0 {
				continue
			}

			// Get events since last sync
			since := lastSync.Unix()
			localEvents := eventProvider()

			if err := d.BiDirectionalSync(urls, kinds, since, localEvents); err != nil {
				log.Warn().Err(err).Msg("Bi-directional sync failed")
			} else {
				lastSync = time.Now()
			}
		}
	}
}

// DiscoveryStats returns relay discovery statistics
type DiscoveryStats struct {
	Running       bool
	KnownRelays   int
	HealthyRelays int
	LastAnnounce  time.Time
	LastScan      time.Time
}

// GetStats returns discovery statistics
func (d *RelayDiscovery) GetStats() DiscoveryStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	healthy := 0
	for _, relay := range d.knownRelays {
		if relay.Healthy {
			healthy++
		}
	}

	return DiscoveryStats{
		Running:       d.running,
		KnownRelays:   len(d.knownRelays),
		HealthyRelays: healthy,
	}
}
