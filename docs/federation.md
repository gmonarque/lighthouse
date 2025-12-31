# Federation

Guide to deploying Lighthouse in a federated multi-node configuration.

---

## Overview

Lighthouse can operate in multiple modes:

| Mode | Description |
|------|-------------|
| **Standalone** | Single node, simple WoT |
| **Curator** | Content moderation node |
| **Consumer** | Trusts external curators |
| **Federated** | Full mesh of curators |

---

## Architecture

### Federated Deployment

```
┌─────────────────────────────────────────────────────────────────┐
│                      Public Nostr Relays                        │
└────────────────────────────┬────────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
   ┌──────────┐        ┌──────────┐        ┌──────────┐
   │ Curator  │        │ Curator  │        │ Curator  │
   │ Node A   │        │ Node B   │        │ Node C   │
   │ (Movies) │        │ (TV)     │        │ (Music)  │
   └────┬─────┘        └────┬─────┘        └────┬─────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
                            ▼
                  ┌──────────────────┐
                  │  Community Relay │
                  │  (Curated only)  │
                  └────────┬─────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
   ┌──────────┐      ┌──────────┐      ┌──────────┐
   │ Consumer │      │ Consumer │      │ Consumer │
   │  Node 1  │      │  Node 2  │      │  Node 3  │
   └──────────┘      └──────────┘      └──────────┘
```

### Roles

| Role | Responsibility |
|------|----------------|
| **Explorer** | Collects events from relays |
| **Curator** | Applies rulesets, signs decisions |
| **Indexer** | Stores curated content |
| **Consumer** | Trusts curators, serves users |

---

## Curator Node Setup

### Configuration

```yaml
# Curator node configuration
server:
  port: 9999

trust:
  depth: 0  # Strict whitelist mode

indexer:
  tag_filter_enabled: true
  tag_filter:
    - movies
    - 4k

curator:
  enabled: true
  publish_decisions: true
  publish_relays:
    - "wss://community.relay.example"
```

### Steps

1. **Deploy Lighthouse**
   ```bash
   ./lighthouse
   ```

2. **Generate/Import Identity**
   - This becomes your curator identity
   - Share your npub with users

3. **Import Rulesets**
   - Censoring ruleset (required)
   - Semantic ruleset (recommended)

4. **Configure Publishing**
   - Enable decision publishing
   - Set target relays

5. **Start Curating**
   - Review incoming content
   - Handle reports/appeals

---

## Consumer Node Setup

### Configuration

```yaml
# Consumer node configuration
server:
  port: 9999

trust:
  depth: 0  # Use curators only

nostr:
  relays:
    - url: "wss://community.relay.example"
      enabled: true
```

### Adding Curators

Via UI:
1. Go to **Trust** → **Curators**
2. **Add Curator**
3. Enter curator's npub
4. Set weight
5. Save

Via API:
```bash
curl -X POST http://localhost:9999/api/trust/curators \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "pubkey": "npub1curator...",
    "name": "Movie Curator",
    "weight": 1.0
  }'
```

### Aggregation Policy

Configure how multiple curator decisions combine:

```bash
curl -X PUT http://localhost:9999/api/trust/aggregation \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "quorum",
    "quorum_required": 2
  }'
```

---

## Community Relay

A community relay serves curated content only.

### Policy Configuration

```yaml
# Community relay policy
relay:
  enabled: true
  policy:
    accept_only_curated: true
    allowed_kinds:
      - 2003  # Torrent metadata
      - 30172 # Verification decisions
    curator_pubkeys:
      - "npub1curator_a..."
      - "npub1curator_b..."
```

### Features

- Only accepts events from trusted curators
- Syncs with other community relays
- Provides curated feed to consumers

---

## Relay Discovery

Nodes can discover each other via Nostr relay announcements.

### Enable Discovery

```yaml
relay:
  enable_discovery: true
```

### Discovery Process

1. Node publishes Kind 30166 announcement event to known relays
2. Other nodes subscribe to Kind 30166 events and discover the announcement
3. Nodes verify each other's health
4. Events sync between discovered nodes
5. Stale nodes (not seen in 24h) are pruned

### Announcement Event

Nodes advertise themselves with Kind 30166 events:

```json
{
  "kind": 30166,
  "tags": [
    ["d", "wss://my-relay.example.com"],
    ["r", "wss://my-relay.example.com"],
    ["name", "My Lighthouse Node"]
  ],
  "content": "{\"name\":\"My Node\",\"supported_nips\":[1,35]}"
}
```

---

## Trust Policy Events

Curators publish trust policies to Nostr.

### Policy Event (Kind 30173)

```json
{
  "kind": 30173,
  "content": "{policy_json}",
  "tags": [
    ["d", "trust-policy"],
    ["version", "1.0.0"]
  ],
  "pubkey": "curator_pubkey",
  "sig": "signature"
}
```

### Policy Structure

```json
{
  "version": "1.0.0",
  "curator_pubkey": "npub1...",
  "allowlist": [
    {"pubkey": "npub1trusted...", "added_at": "2024-01-01T00:00:00Z"}
  ],
  "denylist": [
    {"pubkey": "npub1blocked...", "reason": "spam", "added_at": "2024-01-01T00:00:00Z"}
  ],
  "rulesets": [
    {"type": "censoring", "version": "1.0.0", "hash": "sha256..."}
  ]
}
```

---

## Security Considerations

### Key Rotation

Curators should rotate keys periodically:

1. Generate new keypair
2. Publish key rotation event
3. Re-sign active decisions with new key
4. Notify consumers of rotation

### Key Revocation

If a key is compromised:

1. Publish revocation event immediately
2. Notify consumers
3. Decisions from revoked key are ignored

### Revocation Event

```json
{
  "kind": 30174,
  "content": "",
  "tags": [
    ["d", "key-revocation"],
    ["p", "revoked_pubkey"],
    ["reason", "Key compromised"]
  ],
  "pubkey": "new_pubkey",
  "sig": "signature"
}
```

---

## Scaling

### Horizontal Scaling

```
              ┌─────────────────┐
              │  Load Balancer  │
              └────────┬────────┘
                       │
         ┌─────────────┼─────────────┐
         │             │             │
         ▼             ▼             ▼
   ┌──────────┐  ┌──────────┐  ┌──────────┐
   │ Instance │  │ Instance │  │ Instance │
   │    1     │  │    2     │  │    3     │
   └────┬─────┘  └────┬─────┘  └────┬─────┘
        │             │             │
        └─────────────┼─────────────┘
                      │
               ┌──────┴──────┐
               │  Shared DB  │
               │  (Postgres) │
               └─────────────┘
```

### Read Replicas

For high-read workloads:

1. Primary handles writes
2. Replicas handle reads
3. Consumer instances connect to replicas

---

## Monitoring

### Health Endpoints

```bash
# Node health
curl http://localhost:9999/health

# Indexer status
curl http://localhost:9999/api/indexer/status

# Relay status
curl http://localhost:9999/api/relay/status
```

### Metrics (Prometheus)

```yaml
# Prometheus scrape config
scrape_configs:
  - job_name: 'lighthouse'
    static_configs:
      - targets: ['localhost:9999']
```

### Key Metrics

| Metric | Description |
|--------|-------------|
| `lighthouse_events_processed` | Events processed |
| `lighthouse_decisions_total` | Decisions made |
| `lighthouse_relay_connections` | Connected relays |
| `lighthouse_torrents_indexed` | Indexed torrents |

---

## Backup & Recovery

### Database Backup

```bash
# SQLite backup
sqlite3 lighthouse.db ".backup backup.db"

# PostgreSQL backup
pg_dump lighthouse > backup.sql
```

### State Recovery

1. Stop all nodes
2. Restore database from backup
3. Verify integrity
4. Restart nodes
5. Re-sync from relays if needed

---

## Deployment Patterns

### Small Community

```
1 Curator + 1 Community Relay + N Consumers
```

- Single curator handles moderation
- Community relay serves curated content
- Consumers trust the curator

### Medium Community

```
3 Curators + 2 Relays + N Consumers
Aggregation: quorum=2
```

- Multiple curators for resilience
- Quorum voting for decisions
- Relay redundancy

### Large Network

```
10+ Curators (specialized) + 5+ Relays + Many Consumers
Aggregation: weighted by specialty
```

- Specialized curators (movies, TV, music)
- Geographic relay distribution
- Weight-based aggregation

---

## Troubleshooting

### Nodes Not Syncing

1. Check relay connectivity
2. Verify curator pubkeys match
3. Check network firewall
4. Review relay discovery announcements

### Decision Conflicts

1. Check aggregation policy
2. Review ruleset versions
3. Verify curator signatures
4. Check for revoked keys

### Performance Issues

1. Enable caching
2. Add read replicas
3. Optimize database queries
4. Review indexing frequency

---

## Next Steps

- [Curation](curation.md) - Curator guide
- [API Reference](api-reference.md) - API documentation
- [Architecture](architecture.md) - System design
