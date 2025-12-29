package nostr

import (
	"context"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

// Client represents a Nostr client for subscribing to events
type Client struct {
	relay     *nostr.Relay
	url       string
	connected bool
	mu        sync.RWMutex
}

// NewClient creates a new Nostr client
func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

// Connect establishes connection to the relay
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	relay, err := nostr.RelayConnect(ctx, c.url)
	if err != nil {
		return err
	}

	c.relay = relay
	c.connected = true

	log.Info().Str("url", c.url).Msg("Connected to relay")
	return nil
}

// Disconnect closes the connection to the relay
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.relay != nil {
		c.relay.Close()
		c.connected = false
		log.Info().Str("url", c.url).Msg("Disconnected from relay")
	}
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// URL returns the relay URL
func (c *Client) URL() string {
	return c.url
}

// Subscribe subscribes to events matching the given filters
func (c *Client) Subscribe(ctx context.Context, filters []nostr.Filter, handler func(*nostr.Event)) error {
	c.mu.RLock()
	relay := c.relay
	c.mu.RUnlock()

	if relay == nil {
		return ErrNotConnected
	}

	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				sub.Unsub()
				return
			case event := <-sub.Events:
				if event != nil {
					handler(event)
				}
			}
		}
	}()

	return nil
}

// SubscribeTorrents subscribes to torrent events (Kind 2003)
func (c *Client) SubscribeTorrents(ctx context.Context, since time.Time, handler func(*nostr.Event)) error {
	timestamp := nostr.Timestamp(since.Unix())
	filters := []nostr.Filter{
		{
			Kinds: []int{KindTorrent},
			Since: &timestamp,
		},
	}

	return c.Subscribe(ctx, filters, handler)
}

// SubscribeContactLists subscribes to contact list events (Kind 3)
func (c *Client) SubscribeContactLists(ctx context.Context, pubkeys []string, handler func(*nostr.Event)) error {
	filters := []nostr.Filter{
		{
			Kinds:   []int{KindContactList},
			Authors: pubkeys,
		},
	}

	return c.Subscribe(ctx, filters, handler)
}

// FetchContactList fetches the contact list for a given public key
func (c *Client) FetchContactList(ctx context.Context, pubkey string) (*nostr.Event, error) {
	c.mu.RLock()
	relay := c.relay
	c.mu.RUnlock()

	if relay == nil {
		return nil, ErrNotConnected
	}

	filters := []nostr.Filter{
		{
			Kinds:   []int{KindContactList},
			Authors: []string{pubkey},
			Limit:   1,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		return nil, err
	}
	defer sub.Unsub()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event := <-sub.Events:
		return event, nil
	}
}

// Publish publishes an event to the relay
func (c *Client) Publish(ctx context.Context, event *nostr.Event) error {
	c.mu.RLock()
	relay := c.relay
	c.mu.RUnlock()

	if relay == nil {
		return ErrNotConnected
	}

	return relay.Publish(ctx, *event)
}

// QueryEvents queries events from the relay
func (c *Client) QueryEvents(ctx context.Context, filters []nostr.Filter) ([]*nostr.Event, error) {
	c.mu.RLock()
	relay := c.relay
	c.mu.RUnlock()

	if relay == nil {
		return nil, ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		return nil, err
	}
	defer sub.Unsub()

	var events []*nostr.Event
	for {
		select {
		case <-ctx.Done():
			return events, nil
		case event := <-sub.Events:
			if event != nil {
				events = append(events, event)
			}
		case <-sub.EndOfStoredEvents:
			return events, nil
		}
	}
}
