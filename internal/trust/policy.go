package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// TrustPolicy represents an instance's trust policy for curators
type TrustPolicy struct {
	PolicyID    string         `json:"policy_id"`
	Version     string         `json:"version"`
	Hash        string         `json:"hash,omitempty"`
	Allowlist   []CuratorEntry `json:"allowlist"`
	Denylist    []string       `json:"denylist,omitempty"`
	Revoked     []RevokedKey   `json:"revoked,omitempty"`
	EffectiveAt time.Time      `json:"effective_at"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	Comment     string         `json:"comment,omitempty"`
	AdminPubkey string         `json:"admin_pubkey"`
	Signature   string         `json:"signature,omitempty"`
}

// CuratorEntry represents a trusted curator in the allowlist
type CuratorEntry struct {
	Pubkey           string    `json:"pubkey"`
	Alias            string    `json:"alias,omitempty"`
	Weight           int       `json:"weight"`
	ApprovedRulesets []string  `json:"approved_rulesets,omitempty"`
	AddedAt          time.Time `json:"added_at"`
	Notes            string    `json:"notes,omitempty"`
}

// RevokedKey represents a revoked curator key
type RevokedKey struct {
	Pubkey    string    `json:"pubkey"`
	Reason    string    `json:"reason"`
	RevokedAt time.Time `json:"revoked_at"`
	Signature string    `json:"signature,omitempty"` // Per-revocation signature
}

// AggregationMode represents how decisions are aggregated
type AggregationMode string

const (
	AggregationModeAny      AggregationMode = "any"      // Accept if any curator accepts
	AggregationModeAll      AggregationMode = "all"      // Accept only if all curators accept
	AggregationModeQuorum   AggregationMode = "quorum"   // Accept if N of M curators accept
	AggregationModeWeighted AggregationMode = "weighted" // Accept based on weighted votes
)

// AggregationPolicy defines how multiple curator decisions are combined
type AggregationPolicy struct {
	Mode            AggregationMode `json:"mode"`
	QuorumRequired  int             `json:"quorum_required,omitempty"`
	WeightThreshold int             `json:"weight_threshold,omitempty"`
}

// NewTrustPolicy creates a new trust policy
func NewTrustPolicy(adminPubkey string) *TrustPolicy {
	return &TrustPolicy{
		PolicyID:    generatePolicyID(),
		Version:     "1.0.0",
		Allowlist:   []CuratorEntry{},
		Denylist:    []string{},
		Revoked:     []RevokedKey{},
		EffectiveAt: time.Now().UTC(),
		AdminPubkey: adminPubkey,
	}
}

// generatePolicyID creates a unique policy ID
func generatePolicyID() string {
	data := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// ComputeHash computes the SHA256 hash of the policy
func (p *TrustPolicy) ComputeHash() string {
	// Create copy without hash and signature
	copy := *p
	copy.Hash = ""
	copy.Signature = ""

	data, err := json.Marshal(copy)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// AddCurator adds a curator to the allowlist
func (p *TrustPolicy) AddCurator(pubkey, alias string, weight int) {
	// Check if already exists
	for i, c := range p.Allowlist {
		if c.Pubkey == pubkey {
			// Update existing
			p.Allowlist[i].Alias = alias
			p.Allowlist[i].Weight = weight
			return
		}
	}

	// Add new
	p.Allowlist = append(p.Allowlist, CuratorEntry{
		Pubkey:  pubkey,
		Alias:   alias,
		Weight:  weight,
		AddedAt: time.Now().UTC(),
	})
}

// RemoveCurator removes a curator from the allowlist
func (p *TrustPolicy) RemoveCurator(pubkey string) bool {
	for i, c := range p.Allowlist {
		if c.Pubkey == pubkey {
			p.Allowlist = append(p.Allowlist[:i], p.Allowlist[i+1:]...)
			return true
		}
	}
	return false
}

// RevokeCurator revokes a curator and moves them to the revoked list
func (p *TrustPolicy) RevokeCurator(pubkey, reason string) {
	// Remove from allowlist
	p.RemoveCurator(pubkey)

	// Add to revoked
	p.Revoked = append(p.Revoked, RevokedKey{
		Pubkey:    pubkey,
		Reason:    reason,
		RevokedAt: time.Now().UTC(),
	})
}

// RevokeCuratorWithSignature revokes a curator with a per-revocation signature (spec 4.6.2)
func (p *TrustPolicy) RevokeCuratorWithSignature(pubkey, reason, privateKey string) error {
	// Remove from allowlist
	p.RemoveCurator(pubkey)

	revokedAt := time.Now().UTC()

	// Create a signing event for this specific revocation
	event := &nostr.Event{
		Kind:      KindTrustPolicy,
		Content:   fmt.Sprintf("revoke:%s:%s", pubkey, reason),
		CreatedAt: nostr.Timestamp(revokedAt.Unix()),
		Tags: nostr.Tags{
			{"d", "revocation"},
			{"p", pubkey},
			{"reason", reason},
		},
	}

	// Sign the revocation event
	if err := event.Sign(privateKey); err != nil {
		return fmt.Errorf("failed to sign revocation: %w", err)
	}

	// Add to revoked with signature
	p.Revoked = append(p.Revoked, RevokedKey{
		Pubkey:    pubkey,
		Reason:    reason,
		RevokedAt: revokedAt,
		Signature: event.Sig,
	})

	return nil
}

// DenyCurator adds a curator to the denylist
func (p *TrustPolicy) DenyCurator(pubkey string) {
	// Check if already denied
	for _, denied := range p.Denylist {
		if denied == pubkey {
			return
		}
	}
	p.Denylist = append(p.Denylist, pubkey)
}

// IsCuratorApproved checks if a curator is approved
func (p *TrustPolicy) IsCuratorApproved(pubkey string) bool {
	// Check denylist first
	for _, denied := range p.Denylist {
		if denied == pubkey {
			return false
		}
	}

	// Check revoked
	for _, revoked := range p.Revoked {
		if revoked.Pubkey == pubkey {
			return false
		}
	}

	// Check allowlist
	for _, curator := range p.Allowlist {
		if curator.Pubkey == pubkey {
			return true
		}
	}

	return false
}

// GetCurator returns curator info if approved
func (p *TrustPolicy) GetCurator(pubkey string) *CuratorEntry {
	if !p.IsCuratorApproved(pubkey) {
		return nil
	}

	for _, curator := range p.Allowlist {
		if curator.Pubkey == pubkey {
			return &curator
		}
	}
	return nil
}

// GetCuratorWeight returns the weight of a curator (0 if not approved)
func (p *TrustPolicy) GetCuratorWeight(pubkey string) int {
	curator := p.GetCurator(pubkey)
	if curator == nil {
		return 0
	}
	return curator.Weight
}

// IsExpired checks if the policy has expired
func (p *TrustPolicy) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*p.ExpiresAt)
}

// IsEffective checks if the policy is currently effective
func (p *TrustPolicy) IsEffective() bool {
	if time.Now().Before(p.EffectiveAt) {
		return false
	}
	return !p.IsExpired()
}

// Sign signs the policy with the admin's private key using a Nostr event
func (p *TrustPolicy) Sign(privateKey string) error {
	// Compute hash
	p.Hash = p.ComputeHash()

	// Create a signing event for the policy
	event := &nostr.Event{
		Kind:      KindTrustPolicy,
		Content:   p.Hash,
		CreatedAt: nostr.Timestamp(p.EffectiveAt.Unix()),
		Tags: nostr.Tags{
			{"d", "trust-policy"},
			{"version", p.Version},
		},
	}

	// Sign the event
	if err := event.Sign(privateKey); err != nil {
		return fmt.Errorf("failed to sign policy: %w", err)
	}

	p.Signature = event.Sig
	p.AdminPubkey = event.PubKey
	return nil
}

// Verify verifies the policy signature by reconstructing the signing event
func (p *TrustPolicy) Verify() (bool, error) {
	if p.Signature == "" {
		return false, fmt.Errorf("policy has no signature")
	}
	if p.AdminPubkey == "" {
		return false, fmt.Errorf("policy has no admin pubkey")
	}

	// Compute hash
	hash := p.ComputeHash()

	// Recreate the signing event
	event := &nostr.Event{
		Kind:      KindTrustPolicy,
		PubKey:    p.AdminPubkey,
		Content:   hash,
		CreatedAt: nostr.Timestamp(p.EffectiveAt.Unix()),
		Tags: nostr.Tags{
			{"d", "trust-policy"},
			{"version", p.Version},
		},
		Sig: p.Signature,
	}

	// Compute the event ID
	event.ID = event.GetID()

	// Verify signature
	return event.CheckSignature()
}

// ToJSON serializes the policy to JSON
func (p *TrustPolicy) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// TrustPolicyFromJSON deserializes a policy from JSON
func TrustPolicyFromJSON(data []byte) (*TrustPolicy, error) {
	var p TrustPolicy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Nostr event kind for trust policy
const KindTrustPolicy = 30173

// ToNostrEvent converts the policy to a Nostr event
func (p *TrustPolicy) ToNostrEvent(privateKey string) (*nostr.Event, error) {
	content, err := p.ToJSON()
	if err != nil {
		return nil, err
	}

	event := &nostr.Event{
		Kind:      KindTrustPolicy,
		Content:   string(content),
		CreatedAt: nostr.Timestamp(p.EffectiveAt.Unix()),
		Tags: nostr.Tags{
			{"d", "trust-policy"},
			{"k", "org.legalindex.p2p.trust.v1"},
			{"version", p.Version},
		},
	}

	// Add curator pubkeys as tags
	for _, curator := range p.Allowlist {
		event.Tags = append(event.Tags, nostr.Tag{"p", curator.Pubkey, "curator"})
	}

	if err := event.Sign(privateKey); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return event, nil
}

// TrustPolicyFromNostrEvent extracts a policy from a Nostr event
func TrustPolicyFromNostrEvent(event *nostr.Event) (*TrustPolicy, error) {
	if event.Kind != KindTrustPolicy {
		return nil, fmt.Errorf("unexpected event kind: %d", event.Kind)
	}

	return TrustPolicyFromJSON([]byte(event.Content))
}
