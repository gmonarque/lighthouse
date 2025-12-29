package nostr

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/lighthouse-client/lighthouse/internal/database"
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
	for url, client := range rm.clients {
		wg.Add(1)
		go func(url string, c *Client) {
			defer wg.Done()
			if err := c.Connect(rm.ctx); err != nil {
				log.Error().Err(err).Str("url", url).Msg("Failed to connect to relay")
				rm.updateRelayStatus(url, "error")
			} else {
				rm.updateRelayStatus(url, "connected")
			}
		}(url, client)
	}

	wg.Wait()
	log.Info().Int("count", len(rm.clients)).Msg("Relay manager started")
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

	for _, client := range clients {
		url := client.URL()
		err := client.Subscribe(ctx, filters, func(event *nostr.Event) {
			handler(event, url)
		})
		if err != nil {
			log.Error().Err(err).Str("url", url).Msg("Failed to subscribe")
		}
	}

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
