package decision

import (
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// Signer handles decision signing and verification
type Signer struct {
	privateKey string
	publicKey  string
}

// NewSigner creates a new decision signer
func NewSigner(nsec string) (*Signer, error) {
	if nsec == "" {
		return nil, fmt.Errorf("private key is required")
	}

	// Decode nsec to get hex private key
	var privateKey string
	if len(nsec) == 64 {
		// Already hex
		privateKey = nsec
	} else {
		// NIP-19 encoded
		prefix, data, err := nip19.Decode(nsec)
		if err != nil {
			return nil, fmt.Errorf("failed to decode nsec: %w", err)
		}
		if prefix != "nsec" {
			return nil, fmt.Errorf("expected nsec, got %s", prefix)
		}
		privateKey = data.(string)
	}

	// Derive public key
	publicKey, err := nostr.GetPublicKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return &Signer{
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

// GetPublicKey returns the signer's public key (hex)
func (s *Signer) GetPublicKey() string {
	return s.publicKey
}

// GetNpub returns the signer's public key in npub format
func (s *Signer) GetNpub() string {
	npub, _ := nip19.EncodePublicKey(s.publicKey)
	return npub
}

// Sign signs a verification decision using a Nostr event as the signing mechanism
func (s *Signer) Sign(d *VerificationDecision) error {
	// Set the curator pubkey
	d.CuratorPubkey = s.publicKey

	// Create a Nostr event to sign the decision
	// This is the proper Nostr way to create verifiable signatures
	content, err := json.Marshal(map[string]interface{}{
		"decision_id":      d.DecisionID,
		"target_event_id":  d.TargetEventID,
		"target_infohash":  d.TargetInfohash,
		"decision":         string(d.Decision),
		"reason_codes":     d.ReasonCodes,
		"ruleset_hash":     d.RulesetHash,
		"curator_pubkey":   d.CuratorPubkey,
		"created_at":       d.CreatedAt.Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create a signing event
	event := &nostr.Event{
		Kind:      30175, // Decision event kind
		Content:   string(content),
		CreatedAt: nostr.Timestamp(d.CreatedAt.Unix()),
		Tags: nostr.Tags{
			{"d", d.TargetInfohash},
			{"e", d.TargetEventID},
		},
	}

	// Sign the event - this sets event.ID, event.PubKey, and event.Sig
	if err := event.Sign(s.privateKey); err != nil {
		return fmt.Errorf("failed to sign: %w", err)
	}

	// Store the event signature as the decision signature
	d.Signature = event.Sig
	return nil
}

// Verify verifies a decision's signature by reconstructing the signing event
func Verify(d *VerificationDecision) (bool, error) {
	if d.Signature == "" {
		return false, fmt.Errorf("decision has no signature")
	}
	if d.CuratorPubkey == "" {
		return false, fmt.Errorf("decision has no curator pubkey")
	}

	// Recreate the content that was signed
	content, err := json.Marshal(map[string]interface{}{
		"decision_id":      d.DecisionID,
		"target_event_id":  d.TargetEventID,
		"target_infohash":  d.TargetInfohash,
		"decision":         string(d.Decision),
		"reason_codes":     d.ReasonCodes,
		"ruleset_hash":     d.RulesetHash,
		"curator_pubkey":   d.CuratorPubkey,
		"created_at":       d.CreatedAt.Unix(),
	})
	if err != nil {
		return false, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Recreate the event that was signed
	event := &nostr.Event{
		Kind:      30175,
		PubKey:    d.CuratorPubkey,
		Content:   string(content),
		CreatedAt: nostr.Timestamp(d.CreatedAt.Unix()),
		Tags: nostr.Tags{
			{"d", d.TargetInfohash},
			{"e", d.TargetEventID},
		},
		Sig: d.Signature,
	}

	// Compute the event ID (required for verification)
	event.ID = event.GetID()

	// Verify using the event's built-in verification
	valid, err := event.CheckSignature()
	if err != nil {
		return false, fmt.Errorf("failed to verify signature: %w", err)
	}

	return valid, nil
}

// VerifyAndValidate verifies signature and validates decision structure
func VerifyAndValidate(d *VerificationDecision) error {
	// Validate required fields
	if d.DecisionID == "" {
		return fmt.Errorf("missing decision_id")
	}
	if d.TargetEventID == "" {
		return fmt.Errorf("missing target_event_id")
	}
	if d.TargetInfohash == "" {
		return fmt.Errorf("missing target_infohash")
	}
	if d.Decision != DecisionAccept && d.Decision != DecisionReject {
		return fmt.Errorf("invalid decision: %s", d.Decision)
	}
	if d.CuratorPubkey == "" {
		return fmt.Errorf("missing curator_pubkey")
	}
	if d.Signature == "" {
		return fmt.Errorf("missing signature")
	}
	if d.CreatedAt.IsZero() {
		return fmt.Errorf("missing created_at")
	}

	// Verify signature
	valid, err := Verify(d)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// DecisionToNostrEvent converts a decision to a Nostr event for publishing
func DecisionToNostrEvent(d *VerificationDecision, privateKey string) (*nostr.Event, error) {
	content, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal decision: %w", err)
	}

	// Use a custom kind for verification decisions
	// Kind 30175 = Parameterized replaceable event for curation decisions
	event := &nostr.Event{
		Kind:      30175,
		Content:   string(content),
		CreatedAt: nostr.Timestamp(d.CreatedAt.Unix()),
		Tags: nostr.Tags{
			{"d", d.TargetInfohash},
			{"e", d.TargetEventID},
			{"p", d.CuratorPubkey},
			{"decision", string(d.Decision)},
		},
	}

	// Add reason codes as tags
	for _, code := range d.ReasonCodes {
		event.Tags = append(event.Tags, nostr.Tag{"reason", string(code)})
	}

	// Add ruleset info
	if d.RulesetHash != "" {
		event.Tags = append(event.Tags, nostr.Tag{"ruleset", d.RulesetHash, d.RulesetVersion})
	}

	// Sign the event
	if err := event.Sign(privateKey); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return event, nil
}

// NostrEventToDecision converts a Nostr event to a decision
func NostrEventToDecision(event *nostr.Event) (*VerificationDecision, error) {
	if event.Kind != 30175 {
		return nil, fmt.Errorf("unexpected event kind: %d", event.Kind)
	}

	var d VerificationDecision
	if err := json.Unmarshal([]byte(event.Content), &d); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decision: %w", err)
	}

	// Verify the decision was signed by the event author
	if d.CuratorPubkey != event.PubKey {
		return nil, fmt.Errorf("decision curator doesn't match event author")
	}

	return &d, nil
}
