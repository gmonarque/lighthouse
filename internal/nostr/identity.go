package nostr

import (
	"encoding/hex"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// GenerateIdentity creates a new Nostr keypair
func GenerateIdentity() (npub string, nsec string, err error) {
	// Generate private key
	sk := nostr.GeneratePrivateKey()

	// Convert to nsec
	nsec, err = nip19.EncodePrivateKey(sk)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode nsec: %w", err)
	}

	// Get public key
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive public key: %w", err)
	}

	// Convert to npub
	npub, err = nip19.EncodePublicKey(pk)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode npub: %w", err)
	}

	return npub, nsec, nil
}

// NsecToNpub converts an nsec to its corresponding npub
func NsecToNpub(nsec string) (string, error) {
	// Decode nsec
	prefix, sk, err := nip19.Decode(nsec)
	if err != nil {
		return "", fmt.Errorf("failed to decode nsec: %w", err)
	}

	if prefix != "nsec" {
		return "", fmt.Errorf("invalid prefix: expected nsec, got %s", prefix)
	}

	// Get public key
	pk, err := nostr.GetPublicKey(sk.(string))
	if err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}

	// Convert to npub
	npub, err := nip19.EncodePublicKey(pk)
	if err != nil {
		return "", fmt.Errorf("failed to encode npub: %w", err)
	}

	return npub, nil
}

// NpubToHex converts an npub to hex public key
func NpubToHex(npub string) (string, error) {
	prefix, pk, err := nip19.Decode(npub)
	if err != nil {
		return "", fmt.Errorf("failed to decode npub: %w", err)
	}

	if prefix != "npub" {
		return "", fmt.Errorf("invalid prefix: expected npub, got %s", prefix)
	}

	return pk.(string), nil
}

// HexToNpub converts a hex public key to npub
func HexToNpub(hexPk string) (string, error) {
	return nip19.EncodePublicKey(hexPk)
}

// NsecToHex converts an nsec to hex private key
func NsecToHex(nsec string) (string, error) {
	prefix, sk, err := nip19.Decode(nsec)
	if err != nil {
		return "", fmt.Errorf("failed to decode nsec: %w", err)
	}

	if prefix != "nsec" {
		return "", fmt.Errorf("invalid prefix: expected nsec, got %s", prefix)
	}

	return sk.(string), nil
}

// ValidateNpub checks if a string is a valid npub
func ValidateNpub(npub string) bool {
	prefix, _, err := nip19.Decode(npub)
	return err == nil && prefix == "npub"
}

// ValidateNsec checks if a string is a valid nsec
func ValidateNsec(nsec string) bool {
	prefix, _, err := nip19.Decode(nsec)
	return err == nil && prefix == "nsec"
}

// SignEvent signs a Nostr event with the given private key
func SignEvent(event *nostr.Event, nsec string) error {
	sk, err := NsecToHex(nsec)
	if err != nil {
		return err
	}

	return event.Sign(sk)
}

// VerifyEvent verifies a Nostr event signature
func VerifyEvent(event *nostr.Event) bool {
	ok, _ := event.CheckSignature()
	return ok
}

// EventIDFromBytes creates an event ID from bytes
func EventIDFromBytes(b []byte) string {
	return hex.EncodeToString(b)
}
