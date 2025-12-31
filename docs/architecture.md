# Architecture

Technical overview of Lighthouse's system design and components.

---

## System Overview

Lighthouse is a federated torrent metadata indexer built on Nostr. It separates three key concerns:

| Layer | Responsibility |
|-------|----------------|
| **File Storage** | BitTorrent/DHT (unchanged) |
| **Metadata Publication** | Nostr protocol (NIP-35) |
| **Trust Validation** | Web of Trust + Federated Curation |

```
┌─────────────────────────────────────────────────────────────────┐
│                        Lighthouse Node                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────────────┐ │
│  │Explorer │ → │Curator  │ → │Indexer  │ → │ Web UI / API    │ │
│  └────┬────┘   └────┬────┘   └────┬────┘   └─────────────────┘ │
│       │             │             │                             │
│       ▼             ▼             ▼                             │
│  ┌─────────┐   ┌─────────┐   ┌─────────────────────────────┐   │
│  │ Nostr   │   │Rulesets │   │      SQLite Database        │   │
│  │ Relays  │   │         │   │  (torrents, decisions, etc) │   │
│  └─────────┘   └─────────┘   └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Core Components

### Explorer

**Location:** `internal/explorer/`

The Explorer connects to Nostr relays and collects NIP-35 events.

**Responsibilities:**
- Connect to configured relays
- Subscribe to Kind 2003 (torrent) events
- Queue events for curation
- **Does NOT** make accept/reject decisions

**Data Flow:**
```
Nostr Relays → Explorer → Event Queue → Curator
```

### Curator

**Location:** `internal/curator/`

The Curator applies rulesets to events and signs verification decisions.

**Responsibilities:**
- Apply Censoring Ruleset (deterministic blocking)
- Apply Semantic Ruleset (quality/classification)
- Sign decisions with curator's private key
- Publish decisions to relays (optional)

**Decision Process:**
```
Event → Censoring Rules → Semantic Rules → Decision (accept/reject)
                                                  ↓
                                           Signature + Publish
```

### Indexer

**Location:** `internal/indexer/`

The Indexer stores curated content and handles search/enrichment.

**Responsibilities:**
- Store accepted torrents
- Apply deduplication
- Enrich metadata (TMDB/OMDB)
- Handle search queries

### API Server

**Location:** `internal/api/`

HTTP server providing REST and Torznab APIs.

**Endpoints:**
- `/api/*` - REST API
- `/api/torznab` - Torznab API for *arr apps
- Static frontend serving

---

## Data Model

### Torrent

Primary indexed entity.

```go
type Torrent struct {
    ID              string    // Internal ID
    Infohash        string    // BitTorrent infohash
    InfohashVersion string    // v1, v2, or hybrid
    Name            string    // Original name
    Title           string    // Enriched title
    Size            int64     // Size in bytes
    Category        int       // Torznab category
    Tags            []string  // Nostr tags
    MagnetURI       string    // Magnet link
    CurationStatus  string    // accepted/rejected/pending
    CreatedAt       time.Time

    // Nostr metadata
    EventID     string
    Pubkey      string
    SourceRelay string
}
```

### Verification Decision

Curator's accept/reject decision.

```go
type VerificationDecision struct {
    DecisionID     string       // Unique ID
    Decision       string       // "accept" or "reject"
    ReasonCodes    []ReasonCode // Why
    RulesetType    string       // censoring or semantic
    RulesetVersion string       // Semver
    RulesetHash    string       // SHA-256 of ruleset
    TargetEventID  string       // Nostr event being judged
    TargetInfohash string       // Torrent infohash
    CuratorPubkey  string       // Who made the decision
    Signature      string       // Ed25519 signature
    CreatedAt      time.Time
}
```

### Ruleset

Content moderation rules.

```go
type Ruleset struct {
    ID          string    // Unique ID
    Name        string    // Display name
    Type        string    // censoring or semantic
    Version     string    // Semver
    Hash        string    // SHA-256 of rules
    Rules       []Rule    // Actual rules
    Description string
    Author      string
    CreatedAt   time.Time
}
```

---

## Ruleset System

### Censoring Ruleset

Deterministic blocking rules. **Reject always wins.**

| Code | Description |
|------|-------------|
| `LEGAL_DMCA` | Documented legal report |
| `LEGAL_ILLEGAL` | Manifestly illegal content |
| `ABUSE_SPAM` | Spam, flooding |
| `ABUSE_MALWARE` | Malicious content |

### Semantic Ruleset

Quality and classification rules. **Aggregation policy applies.**

| Code | Description |
|------|-------------|
| `SEM_DUPLICATE_EXACT` | Same infohash exists |
| `SEM_DUPLICATE_PROBABLE` | Same files (different event) |
| `SEM_BAD_META` | Incomplete metadata |
| `SEM_LOW_QUALITY` | Quality below threshold |
| `SEM_CATEGORY_MISMATCH` | Wrong categorization |

### Rule Evaluation

```go
type Rule struct {
    ID          string            // Unique ID
    Name        string            // Display name
    Type        string            // pattern, list, threshold
    Field       string            // What to check
    Operator    string            // equals, contains, regex, etc.
    Value       interface{}       // Comparison value
    ReasonCode  string            // Code if triggered
    Priority    int               // Evaluation order
}
```

---

## Trust System

### Web of Trust

Simple follow-based trust.

| Depth | Description |
|-------|-------------|
| 0 | Whitelist only |
| 1 | Whitelist + direct follows |
| 2 | + follows of follows |

### Federated Curation

Trust curators instead of individual publishers.

```
User → trusts → Curators → validate → Publishers
```

### Aggregation Policy

When multiple curators exist:

| Mode | Behavior |
|------|----------|
| `any` | Any curator acceptance = accept |
| `all` | All curators must accept |
| `quorum` | N of M must agree |
| `weighted` | Weight-based voting |

---

## Deduplication

### Exact Duplicates

Same infohash = same torrent. Merge metadata, keep one.

### Probable Duplicates

Same size + same file structure. Group together, mark canonical.

```go
type DedupGroup struct {
    GroupID     string
    CanonicalID string   // Best version
    Members     []string // All infohashes
}
```

### Semantic Similarity

Similar title/tags/structure. Score 0-100, configurable threshold.

---

## Relay Module

**Location:** `internal/relay/`

Optional Nostr relay server.

### Features

- Full NIP-01 support
- Event storage and serving
- Subscription handling
- Policy-based filtering
- Bi-directional sync

### Policy

```go
type TorrentPolicy struct {
    AcceptOnlyCurated bool     // Only curated content
    AllowedKinds      []int    // Event kinds to accept
    DenyPubkeys       []string // Blocked publishers
    AllowPubkeys      []string // Whitelisted publishers
}
```

---

## Relay Discovery

**Location:** `internal/relay/discovery.go`

Discover other Lighthouse relay nodes via Nostr.

### How It Works

Lighthouse nodes find each other by publishing and subscribing to relay announcement events (Kind 30166) on Nostr:

1. **Announce** - Publish a Kind 30166 event advertising this relay's URL and capabilities
2. **Scan** - Subscribe to Kind 30166 events on known relays to discover other nodes
3. **Track** - Maintain a list of discovered relays with health status
4. **Sync** - Exchange events with discovered relays
5. **Prune** - Remove stale relays not seen in 24+ hours

### Announcement Event

```json
{
  "kind": 30166,
  "tags": [
    ["d", "wss://my-relay.example.com"],
    ["r", "wss://my-relay.example.com"],
    ["name", "My Lighthouse Relay"]
  ],
  "content": "{\"name\":\"My Relay\",\"description\":\"...\"}"
}
```

---

## Data Resilience & Replication

Nostr's architecture provides inherent data resilience through event replication.

### How Nostr Prevents Data Loss

```
┌──────────────────────────────────────────────────────────────┐
│                    Event Publication                          │
│                                                               │
│   Publisher ──┬──► Relay A ◄──sync──► Relay B                │
│               │                           ▲                   │
│               ├──► Relay B                │                   │
│               │                       sync│                   │
│               └──► Relay C ◄──────────────┘                   │
│                                                               │
│   Same event (same ID) exists on multiple relays             │
└──────────────────────────────────────────────────────────────┘
```

### Key Resilience Properties

| Property | Description |
|----------|-------------|
| **Event Immutability** | Events are signed and identified by hash - same event ID = same content |
| **Multi-relay Publishing** | Clients publish to multiple relays simultaneously |
| **No Single Point of Failure** | If one relay goes down, events exist elsewhere |
| **Eventual Consistency** | Events propagate through sync and re-publishing |

### Event Replication Methods

1. **Direct Multi-publish** - Clients publish events to all configured relays
2. **Relay-to-Relay Sync** - Relays sync events with each other (via `sync_with` config)
3. **Client Re-fetch** - Clients fetch events from multiple relays and can re-publish missing ones
4. **Discovery Sync** - Discovered relays automatically sync torrent events

### Lighthouse Configuration

```yaml
# Connect to multiple relays for redundancy
nostr:
  relays:
    - url: "wss://relay1.example.com"
      enabled: true
    - url: "wss://relay2.example.com"
      enabled: true
    - url: "wss://relay3.example.com"
      enabled: true

# Enable relay-to-relay sync
relay:
  enabled: true
  sync_with:
    - "wss://partner-relay.example.com"
  enable_discovery: true  # Auto-discover and sync with other nodes
```

### What Gets Replicated

| Event Kind | Content | Replicated? |
|------------|---------|-------------|
| 2003 | Torrent metadata | Yes - across all connected relays |
| 2004 | Comments/ratings | Yes - follows torrent events |
| 30172 | Verification decisions | Yes - curator decisions propagate |
| 30166 | Relay announcements | Yes - enables discovery |
| 30173 | Trust policies | Yes - curator trust lists |

### Failure Scenarios

| Scenario | Outcome |
|----------|---------|
| Single relay down | Events still available on other relays |
| Multiple relays down | Events persist on remaining relays + local DB |
| All public relays down | Community relays and local nodes retain data |
| Network partition | Events sync when connectivity restores |

This decentralized model ensures that torrent metadata survives individual relay failures, making the network resilient without requiring centralized coordination.

---

## Comments System

**Location:** `internal/comments/`

NIP-35 compliant torrent comments (Kind 2004).

### Features

- Threaded replies
- Star ratings (1-5)
- Per-torrent statistics

```go
type Comment struct {
    ID          string
    Infohash    string    // Target torrent
    ParentID    string    // Reply to (optional)
    Content     string    // Comment text
    Rating      int       // 1-5 stars
    AuthorPubkey string
    CreatedAt   time.Time
}
```

---

## Database Schema

SQLite with WAL mode.

### Core Tables

| Table | Description |
|-------|-------------|
| `torrents` | Indexed torrents |
| `verification_decisions` | Curation decisions |
| `rulesets` | Moderation rulesets |
| `curator_trust` | Trusted curators |
| `trust_policies` | Policy history |
| `reports` | Reports and appeals |
| `torrent_comments` | User comments |
| `relay_events` | Relay event storage |
| `dedup_groups` | Deduplication groups |

### Migrations

Located in `internal/database/migrations/`.

---

## Directory Structure

```
lighthouse/
├── cmd/lighthouse/        # Entry point
├── internal/
│   ├── api/               # HTTP server
│   │   ├── handlers/      # Request handlers
│   │   ├── middleware/    # Auth, logging
│   │   └── static/        # Embedded frontend
│   ├── comments/          # Comment system
│   ├── config/            # Configuration
│   ├── curator/           # Curation engine
│   ├── database/          # SQLite + migrations
│   ├── decision/          # Decision types
│   ├── explorer/          # Relay exploration
│   ├── indexer/           # Core indexer
│   ├── moderation/        # Reports/appeals
│   ├── models/            # Shared types
│   ├── nostr/             # Nostr client
│   ├── relay/             # Relay server
│   ├── ruleset/           # Rule engine
│   ├── torznab/           # Torznab API
│   └── trust/             # Trust system
└── web/                   # Svelte frontend
```

---

## Event Flow

### Indexing a Torrent

```
1. Explorer receives Kind 2003 event from relay
2. Event queued for Curator
3. Curator loads applicable rulesets
4. Censoring ruleset applied
5. Semantic ruleset applied
6. Decision created and signed
7. Decision published (optional)
8. If accepted, Indexer stores torrent
9. Enrichment fetches metadata
10. Available via API/UI
```

### User Search

```
1. User submits search query
2. API validates request
3. Query executed against SQLite
4. Results filtered by trust policy
5. Enriched data returned
6. UI displays results
```

---

## Next Steps

- [API Reference](api-reference.md) - Endpoint documentation
- [Curation](curation.md) - Curator setup
- [Federation](federation.md) - Multi-node deployment
