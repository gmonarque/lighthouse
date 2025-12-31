package nostr

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

var (
	ErrNotConnected = errors.New("not connected to relay")
	ErrRelayExists  = errors.New("relay already exists")
)

// Nostr event kinds
const (
	KindMetadata    = 0
	KindTextNote    = 1
	KindContactList = 3
	KindRelayList   = 10002 // NIP-65 relay list
	KindTorrent     = 2003
)

// RelayManager manages connections to multiple Nostr relays
type RelayManager struct {
	clients map[string]*Client
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRelayManager creates a new relay manager
func NewRelayManager(relays []config.RelayConfig) *RelayManager {
	rm := &RelayManager{
		clients: make(map[string]*Client),
	}

	// Initialize clients for configured relays
	for _, relay := range relays {
		if relay.Enabled {
			rm.clients[relay.URL] = NewClient(relay.URL)
		}
	}

	return rm
}

// Start connects to all configured relays
func (rm *RelayManager) Start(ctx context.Context) error {
	rm.mu.Lock()
	rm.ctx, rm.cancel = context.WithCancel(ctx)
	rm.mu.Unlock()

	// Connect to all relays concurrently
	var wg sync.WaitGroup
	var connectedCount int
	var mu sync.Mutex

	log.Info().Int("total_relays", len(rm.clients)).Msg("Connecting to relays...")

	for url, client := range rm.clients {
		wg.Add(1)
		go func(url string, c *Client) {
			defer wg.Done()
			if err := c.Connect(rm.ctx); err != nil {
				log.Error().Err(err).Str("url", url).Msg("Failed to connect to relay")
				rm.updateRelayStatus(url, "error")
			} else {
				log.Info().Str("url", url).Msg("Connected to relay")
				rm.updateRelayStatus(url, "connected")
				mu.Lock()
				connectedCount++
				mu.Unlock()
			}
		}(url, client)
	}

	wg.Wait()
	log.Info().Int("connected", connectedCount).Int("total", len(rm.clients)).Msg("Relay manager started")
	return nil
}

// Stop disconnects from all relays
func (rm *RelayManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.cancel != nil {
		rm.cancel()
	}

	for url, client := range rm.clients {
		client.Disconnect()
		rm.updateRelayStatus(url, "disconnected")
	}

	log.Info().Msg("Relay manager stopped")
}

// AddRelay adds a new relay
func (rm *RelayManager) AddRelay(url string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.clients[url]; exists {
		return ErrRelayExists
	}

	client := NewClient(url)
	rm.clients[url] = client

	// Connect if manager is running
	if rm.ctx != nil {
		go func() {
			if err := client.Connect(rm.ctx); err != nil {
				log.Error().Err(err).Str("url", url).Msg("Failed to connect to new relay")
				rm.updateRelayStatus(url, "error")
			} else {
				rm.updateRelayStatus(url, "connected")
			}
		}()
	}

	return nil
}

// RemoveRelay removes a relay
func (rm *RelayManager) RemoveRelay(url string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if client, exists := rm.clients[url]; exists {
		client.Disconnect()
		delete(rm.clients, url)
	}
}

// GetClient returns a client for a specific relay
func (rm *RelayManager) GetClient(url string) *Client {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.clients[url]
}

// GetConnectedClients returns all connected clients
func (rm *RelayManager) GetConnectedClients() []*Client {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var connected []*Client
	for _, client := range rm.clients {
		if client.IsConnected() {
			connected = append(connected, client)
		}
	}
	return connected
}

// GetAllClients returns all clients
func (rm *RelayManager) GetAllClients() []*Client {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	clients := make([]*Client, 0, len(rm.clients))
	for _, client := range rm.clients {
		clients = append(clients, client)
	}
	return clients
}

// ConnectedCount returns the number of connected relays
func (rm *RelayManager) ConnectedCount() int {
	return len(rm.GetConnectedClients())
}

// SubscribeAll subscribes to events on all connected relays
func (rm *RelayManager) SubscribeAll(ctx context.Context, filters []nostr.Filter, handler func(*nostr.Event, string)) error {
	clients := rm.GetConnectedClients()
	if len(clients) == 0 {
		return errors.New("no connected relays")
	}

	successCount := 0
	for _, client := range clients {
		url := client.URL()
		err := client.Subscribe(ctx, filters, func(event *nostr.Event) {
			handler(event, url)
		})
		if err != nil {
			log.Error().Err(err).Str("url", url).Msg("Failed to subscribe")
		} else {
			successCount++
			log.Info().Str("url", url).Msg("Subscribed to relay")
		}
	}

	log.Info().Int("subscribed", successCount).Int("total_connected", len(clients)).Msg("Subscription complete")
	return nil
}

// SubscribeTorrents subscribes to torrent events on all connected relays
func (rm *RelayManager) SubscribeTorrents(ctx context.Context, since time.Time, handler func(*nostr.Event, string)) error {
	timestamp := nostr.Timestamp(since.Unix())
	filters := []nostr.Filter{
		{
			Kinds: []int{KindTorrent},
			Since: &timestamp,
		},
	}

	return rm.SubscribeAll(ctx, filters, handler)
}

// SubscribeTrustedTorrents subscribes to torrent events from specific authors (trusted uploaders)
// This is more efficient than fetching all events because relays return ALL events from these authors
func (rm *RelayManager) SubscribeTrustedTorrents(ctx context.Context, pubkeys []string, handler func(*nostr.Event, string)) error {
	if len(pubkeys) == 0 {
		return errors.New("no pubkeys provided")
	}

	// Query for all historical events from trusted authors (no time limit)
	filters := []nostr.Filter{
		{
			Kinds:   []int{KindTorrent},
			Authors: pubkeys,
		},
	}

	log.Info().Int("authors", len(pubkeys)).Msg("Subscribing to torrents from trusted authors")

	return rm.SubscribeAll(ctx, filters, handler)
}

// FetchContactList fetches contact list from any connected relay
func (rm *RelayManager) FetchContactList(ctx context.Context, pubkey string) (*nostr.Event, error) {
	clients := rm.GetConnectedClients()
	if len(clients) == 0 {
		return nil, errors.New("no connected relays")
	}

	// Try each relay until we get a result
	for _, client := range clients {
		event, err := client.FetchContactList(ctx, pubkey)
		if err == nil && event != nil {
			return event, nil
		}
	}

	return nil, errors.New("contact list not found")
}

// PublishToAll publishes an event to all connected relays
func (rm *RelayManager) PublishToAll(ctx context.Context, event *nostr.Event) error {
	clients := rm.GetConnectedClients()
	if len(clients) == 0 {
		return errors.New("no connected relays")
	}

	var wg sync.WaitGroup
	var lastErr error

	for _, client := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			if err := c.Publish(ctx, event); err != nil {
				lastErr = err
				log.Error().Err(err).Str("url", c.URL()).Msg("Failed to publish event")
			}
		}(client)
	}

	wg.Wait()
	return lastErr
}

// PublishResult contains the result of publishing to a single relay
type PublishResult struct {
	RelayID  int    `json:"relay_id"`
	RelayURL string `json:"relay_url"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// PublishToRelays publishes an event to specific relays by their database IDs
// If relayIDs is empty, publishes to all connected relays
func (rm *RelayManager) PublishToRelays(ctx context.Context, event *nostr.Event, relayIDs []int) []PublishResult {
	db := database.Get()

	var results []PublishResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// If no relay IDs specified, publish to all connected
	if len(relayIDs) == 0 {
		clients := rm.GetConnectedClients()
		for _, client := range clients {
			wg.Add(1)
			go func(c *Client) {
				defer wg.Done()
				result := PublishResult{
					RelayURL: c.URL(),
					Success:  true,
				}
				if err := c.Publish(ctx, event); err != nil {
					result.Success = false
					result.Error = err.Error()
					log.Error().Err(err).Str("url", c.URL()).Msg("Failed to publish event")
				} else {
					log.Info().Str("url", c.URL()).Msg("Published event to relay")
				}
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(client)
		}
	} else {
		// Publish to specific relays by ID
		for _, relayID := range relayIDs {
			// Get relay URL from database
			var url string
			if db != nil {
				row := db.QueryRow("SELECT url FROM relays WHERE id = ?", relayID)
				if err := row.Scan(&url); err != nil {
					results = append(results, PublishResult{
						RelayID: relayID,
						Success: false,
						Error:   "relay not found",
					})
					continue
				}
			}

			rm.mu.RLock()
			client, exists := rm.clients[url]
			rm.mu.RUnlock()

			if !exists || client == nil {
				results = append(results, PublishResult{
					RelayID:  relayID,
					RelayURL: url,
					Success:  false,
					Error:    "relay not loaded",
				})
				continue
			}

			if !client.IsConnected() {
				results = append(results, PublishResult{
					RelayID:  relayID,
					RelayURL: url,
					Success:  false,
					Error:    "relay not connected",
				})
				continue
			}

			wg.Add(1)
			go func(id int, c *Client) {
				defer wg.Done()
				result := PublishResult{
					RelayID:  id,
					RelayURL: c.URL(),
					Success:  true,
				}
				if err := c.Publish(ctx, event); err != nil {
					result.Success = false
					result.Error = err.Error()
					log.Error().Err(err).Str("url", c.URL()).Msg("Failed to publish event")
				} else {
					log.Info().Str("url", c.URL()).Msg("Published event to relay")
				}
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(relayID, client)
		}
	}

	wg.Wait()
	return results
}

// updateRelayStatus updates the relay status in the database
func (rm *RelayManager) updateRelayStatus(url, status string) {
	db := database.Get()
	if db == nil {
		return
	}

	query := "UPDATE relays SET status = ?"
	args := []interface{}{status}

	if status == "connected" {
		query += ", last_connected_at = CURRENT_TIMESTAMP"
	}

	query += " WHERE url = ?"
	args = append(args, url)

	db.Exec(query, args...)
}

// ReconnectAll attempts to reconnect to all disconnected relays
func (rm *RelayManager) ReconnectAll() {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.ctx == nil {
		return
	}

	for url, client := range rm.clients {
		if !client.IsConnected() {
			go func(url string, c *Client) {
				if err := c.Connect(rm.ctx); err != nil {
					log.Error().Err(err).Str("url", url).Msg("Failed to reconnect to relay")
				} else {
					rm.updateRelayStatus(url, "connected")
					log.Info().Str("url", url).Msg("Reconnected to relay")
				}
			}(url, client)
		}
	}
}

// HealthCheck performs a health check on all relays
func (rm *RelayManager) HealthCheck() map[string]string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	status := make(map[string]string)
	for url, client := range rm.clients {
		if client.IsConnected() {
			status[url] = "connected"
		} else {
			status[url] = "disconnected"
		}
	}
	return status
}

// LoadRelaysFromDB loads enabled relays from the database and adds them to the manager
func (rm *RelayManager) LoadRelaysFromDB() error {
	db := database.Get()
	if db == nil {
		return nil
	}

	rows, err := db.Query("SELECT url FROM relays WHERE enabled = 1")
	if err != nil {
		return err
	}
	defer rows.Close()

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			continue
		}

		// Add client if not already present
		if _, exists := rm.clients[url]; !exists {
			rm.clients[url] = NewClient(url)
			log.Debug().Str("url", url).Msg("Added relay from database")
		}
	}

	log.Info().Int("total_relays", len(rm.clients)).Msg("Loaded relays from database")
	return nil
}

// RelayListEntry represents a relay from NIP-65 relay list
type RelayListEntry struct {
	URL   string
	Read  bool
	Write bool
}

// FetchRelayList fetches NIP-65 relay list for a pubkey
func (rm *RelayManager) FetchRelayList(ctx context.Context, pubkey string) ([]RelayListEntry, error) {
	clients := rm.GetConnectedClients()

	log.Debug().Str("pubkey", pubkey[:16]+"...").Int("connected_relays", len(clients)).Msg("Fetching NIP-65 relay list")

	filters := []nostr.Filter{
		{
			Kinds:   []int{KindRelayList},
			Authors: []string{pubkey},
			Limit:   1,
		},
	}

	// Helper function to parse relay list from event
	parseRelayList := func(event *nostr.Event) []RelayListEntry {
		var relays []RelayListEntry
		for _, tag := range event.Tags {
			if len(tag) < 2 || tag[0] != "r" {
				continue
			}

			entry := RelayListEntry{URL: tag[1], Read: true, Write: true}

			// Check for read/write marker
			if len(tag) >= 3 {
				switch tag[2] {
				case "read":
					entry.Write = false
				case "write":
					entry.Read = false
				}
			}

			relays = append(relays, entry)
		}
		return relays
	}

	// Try connected relays first
	for _, client := range clients {
		queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		events, err := client.QueryEvents(queryCtx, filters)
		cancel()

		if err != nil {
			log.Debug().Err(err).Str("relay", client.url).Msg("Failed to query relay for NIP-65")
			continue
		}

		if len(events) == 0 {
			continue
		}

		relays := parseRelayList(events[0])
		if len(relays) > 0 {
			log.Info().Str("pubkey", pubkey[:16]+"...").Int("relays", len(relays)).Msg("Fetched user relay list")
			return relays, nil
		}
	}

	// Fallback: try well-known indexing relays that might have the data
	fallbackRelays := []string{
		"wss://relay.nostr.band",
		"wss://purplepag.es",
		"wss://relay.damus.io",
	}

	for _, relayURL := range fallbackRelays {
		// Skip if already in connected clients
		found := false
		for _, c := range clients {
			if c.url == relayURL {
				found = true
				break
			}
		}
		if found {
			continue
		}

		log.Debug().Str("relay", relayURL).Msg("Trying fallback relay for NIP-65")

		// Create temporary client
		tempClient := NewClient(relayURL)
		connectCtx, connectCancel := context.WithTimeout(ctx, 3*time.Second)
		err := tempClient.Connect(connectCtx)
		connectCancel()

		if err != nil {
			log.Debug().Err(err).Str("relay", relayURL).Msg("Failed to connect to fallback relay")
			continue
		}

		queryCtx, queryCancel := context.WithTimeout(ctx, 3*time.Second)
		events, err := tempClient.QueryEvents(queryCtx, filters)
		queryCancel()
		tempClient.Disconnect()

		if err != nil || len(events) == 0 {
			continue
		}

		relays := parseRelayList(events[0])
		if len(relays) > 0 {
			log.Info().Str("pubkey", pubkey[:16]+"...").Int("relays", len(relays)).Str("source", relayURL).Msg("Fetched user relay list from fallback")
			return relays, nil
		}
	}

	return nil, errors.New("user has no NIP-65 relay list published")
}

// DiscoverAndAddUserRelays discovers a user's relays and adds their write relays
func (rm *RelayManager) DiscoverAndAddUserRelays(ctx context.Context, npub string) (int, error) {
	// Convert npub to hex
	pubkey, err := NpubToHex(npub)
	if err != nil {
		return 0, err
	}

	relays, err := rm.FetchRelayList(ctx, pubkey)
	if err != nil {
		return 0, err
	}

	added := 0
	db := database.Get()

	for _, relay := range relays {
		// Only add write relays (where they publish)
		if !relay.Write {
			continue
		}

		// Check if already exists
		rm.mu.RLock()
		_, exists := rm.clients[relay.URL]
		rm.mu.RUnlock()

		if exists {
			continue
		}

		// Add to database
		if db != nil {
			_, err := db.Exec(`
				INSERT OR IGNORE INTO relays (url, name, preset, enabled, status)
				VALUES (?, ?, 'discovered', TRUE, 'disconnected')
			`, relay.URL, "Discovered")
			if err != nil {
				log.Error().Err(err).Str("url", relay.URL).Msg("Failed to add discovered relay to DB")
				continue
			}
		}

		// Add to manager and connect
		if err := rm.AddRelay(relay.URL); err == nil {
			added++
			log.Info().Str("url", relay.URL).Msg("Added discovered relay")
		}
	}

	return added, nil
}
