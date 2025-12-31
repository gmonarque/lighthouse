# Curation

Guide to operating Lighthouse as a content curator.

---

## What is a Curator?

A curator is a trusted entity that:
- Reviews incoming content
- Applies moderation rulesets
- Signs verification decisions
- Publishes decisions for others to consume

Curators are the "human filters" in the Web of Trust model.

---

## Becoming a Curator

### Prerequisites

1. A Nostr identity (npub/nsec)
2. A Lighthouse instance
3. Understanding of the content you're curating
4. Commitment to consistent moderation

### Setup Steps

1. **Configure Strict Trust**
   ```yaml
   trust:
     depth: 0  # Whitelist only
   ```

2. **Enable Tag Filtering** (optional)
   ```yaml
   indexer:
     tag_filter_enabled: true
     tag_filter:
       - movies
       - tv
   ```

3. **Import or Create Rulesets**

4. **Start Processing Content**

---

## Rulesets

Rulesets define moderation policies. There are two types:

### Censoring Rulesets

Deterministic blocking rules. **Rejections always take priority.**

| Code | Description |
|------|-------------|
| `LEGAL_DMCA` | Documented DMCA takedown |
| `LEGAL_ILLEGAL` | Manifestly illegal content |
| `ABUSE_SPAM` | Spam or flooding |
| `ABUSE_MALWARE` | Malicious content |

### Semantic Rulesets

Quality and classification rules. **Aggregation policy applies.**

| Code | Description |
|------|-------------|
| `SEM_DUPLICATE_EXACT` | Exact duplicate (same infohash) |
| `SEM_DUPLICATE_PROBABLE` | Same files, different event |
| `SEM_BAD_META` | Incomplete metadata |
| `SEM_LOW_QUALITY` | Below quality threshold |
| `SEM_CATEGORY_MISMATCH` | Wrong category |

---

## Creating Rulesets

### Ruleset Structure

```json
{
  "name": "Movie Curation Ruleset",
  "type": "semantic",
  "version": "1.0.0",
  "description": "Quality rules for movie torrents",
  "author": "npub1...",
  "rules": [
    {
      "id": "min-size",
      "name": "Minimum Size",
      "type": "threshold",
      "field": "size",
      "operator": "gte",
      "value": 100000000,
      "reason_code": "SEM_LOW_QUALITY",
      "priority": 10
    }
  ]
}
```

### Rule Types

#### Pattern Rules

Match against text fields.

```json
{
  "type": "pattern",
  "field": "name",
  "operator": "contains",
  "value": "CAM",
  "reason_code": "SEM_LOW_QUALITY"
}
```

Operators: `equals`, `contains`, `starts_with`, `ends_with`, `regex`

#### Threshold Rules

Compare numeric values.

```json
{
  "type": "threshold",
  "field": "size",
  "operator": "gte",
  "value": 100000000,
  "reason_code": "SEM_LOW_QUALITY"
}
```

Operators: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`

#### List Rules

Match against lists.

```json
{
  "type": "list",
  "field": "tags",
  "operator": "contains_any",
  "value": ["spam", "fake"],
  "reason_code": "ABUSE_SPAM"
}
```

Operators: `contains_any`, `contains_all`, `contains_none`

### Available Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Torrent name |
| `size` | integer | Size in bytes |
| `category` | integer | Torznab category |
| `tags` | array | Nostr tags |
| `pubkey` | string | Publisher's npub |
| `created_at` | timestamp | Publication time |

---

## Importing Rulesets

### Via UI

1. Go to **Curation** page
2. Click **Import Ruleset**
3. Paste JSON content
4. Review and save

### Via API

```bash
curl -X POST http://localhost:9999/api/rulesets \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d @ruleset.json
```

### From URL

```bash
curl -X POST http://localhost:9999/api/rulesets/import \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/ruleset.json"}'
```

---

## Verification Decisions

When content is processed, a decision is created:

```json
{
  "decision_id": "abc123",
  "decision": "accept",
  "reason_codes": [],
  "ruleset_type": "semantic",
  "ruleset_version": "1.0.0",
  "ruleset_hash": "sha256...",
  "target_event_id": "nostr_event_id",
  "target_infohash": "aabbccdd...",
  "curator_pubkey": "npub1...",
  "signature": "ed25519_sig...",
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Decision Flow

```
Content → Censoring Rules → Semantic Rules → Decision → Sign → Publish
```

### Signing Decisions

Decisions are signed with your nsec:

```go
signature = Ed25519.Sign(privateKey, decisionHash)
```

The signature proves:
- You made the decision
- The decision hasn't been tampered with
- The exact ruleset version was used

---

## Publishing Decisions

Decisions can be published to Nostr relays.

### Enable Publishing

```yaml
curator:
  publish_decisions: true
  publish_relays:
    - "wss://relay.example.com"
```

### Decision Event Format

Decisions are published as Kind 30172 events:

```json
{
  "kind": 30172,
  "content": "{decision_json}",
  "tags": [
    ["d", "decision_id"],
    ["infohash", "aabbccdd..."],
    ["decision", "accept"],
    ["ruleset", "semantic", "1.0.0", "sha256..."]
  ],
  "pubkey": "curator_pubkey",
  "sig": "signature"
}
```

---

## Handling Reports

Curators receive reports from users.

### Report Flow

```
User submits report → Report queued → Curator reviews → Decision updated
```

### Review Process

1. Go to **Reports** page
2. Review pending reports
3. Acknowledge report (marks as seen)
4. Investigate if needed
5. Resolve or reject

### Report Categories

| Category | Description |
|----------|-------------|
| `dmca` | DMCA takedown request |
| `illegal` | Illegal content |
| `spam` | Spam or flooding |
| `malware` | Malicious content |
| `false_info` | Incorrect metadata |
| `duplicate` | Duplicate content |
| `other` | Other issues |

---

## Appeals

Users can appeal decisions.

### Appeal Flow

```
User submits appeal → Appeal queued → Curator reviews → Decision reconsidered
```

### Handling Appeals

1. Review appeal evidence
2. Re-evaluate original decision
3. Update decision if warranted
4. Respond with resolution

---

## Best Practices

### Consistency

- Apply rules consistently
- Document your policies
- Use versioned rulesets
- Communicate changes

### Transparency

- Publish your ruleset publicly
- Explain rejection reasons
- Respond to appeals fairly

### Specialization

Consider focusing on specific content:
- **Movie Curator** - Film-focused rules
- **Linux Curator** - Open source software
- **Music Curator** - Audio releases

### Collaboration

- Share rulesets with other curators
- Coordinate on legal takedowns
- Build reputation through quality

---

## Ruleset Examples

### Basic Quality Filter

```json
{
  "name": "Basic Quality",
  "type": "semantic",
  "version": "1.0.0",
  "rules": [
    {
      "id": "min-size",
      "name": "Minimum Size (100MB)",
      "type": "threshold",
      "field": "size",
      "operator": "gte",
      "value": 100000000,
      "reason_code": "SEM_LOW_QUALITY"
    },
    {
      "id": "no-cam",
      "name": "No CAM Releases",
      "type": "pattern",
      "field": "name",
      "operator": "regex",
      "value": "\\bCAM\\b",
      "reason_code": "SEM_LOW_QUALITY"
    }
  ]
}
```

### Anti-Spam Filter

```json
{
  "name": "Anti-Spam",
  "type": "censoring",
  "version": "1.0.0",
  "rules": [
    {
      "id": "spam-tags",
      "name": "Spam Tags",
      "type": "list",
      "field": "tags",
      "operator": "contains_any",
      "value": ["spam", "fake", "virus"],
      "reason_code": "ABUSE_SPAM"
    },
    {
      "id": "spam-names",
      "name": "Spam Names",
      "type": "pattern",
      "field": "name",
      "operator": "regex",
      "value": "(\\bFREE\\b.*\\bDOWNLOAD\\b|\\bCLICK\\s+HERE\\b)",
      "reason_code": "ABUSE_SPAM"
    }
  ]
}
```

---

## Metrics

Track your curation effectiveness:

| Metric | Description |
|--------|-------------|
| Decisions/day | Volume processed |
| Accept rate | % accepted |
| Appeal rate | % decisions appealed |
| Overturn rate | % appeals resulting in change |

---

## Next Steps

- [Federation](federation.md) - Multi-curator deployment
- [Web of Trust](web-of-trust.md) - Trust system
- [API Reference](api-reference.md) - Automation
