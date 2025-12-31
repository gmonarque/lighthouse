# Web of Trust

Understanding and configuring Lighthouse's trust system.

---

## Overview

Lighthouse uses a **Web of Trust** model to filter content. Instead of relying on a central authority to moderate content, trust flows through cryptographic relationships.

### The Problem with Centralization

Traditional torrent indexers are:
- **Single points of failure** - Take down the site, lose everything
- **Centrally moderated** - One entity decides what's acceptable
- **Easy to censor** - Clear targets for legal action

### The Solution

Lighthouse distributes trust through three actors:

| Actor | Role |
|-------|------|
| **Emitter** | Creates and signs torrent metadata |
| **Curator** | Validates emitters, maintains trust lists |
| **User** | Chooses which curators to trust |

---

## How Trust Works

### Trust Depth

The `trust.depth` setting controls how far trust extends:

| Depth | What You See |
|-------|--------------|
| **0** | Only whitelisted publishers |
| **1** | Whitelist + people you follow on Nostr |
| **2** | Above + friends of friends |

### Depth 0: Whitelist Only

Most restrictive. Only content from manually added npubs appears.

```yaml
trust:
  depth: 0
```

Use when:
- Running a private instance
- Only want specific publishers
- Maximum control over content

### Depth 1: Direct Follows

Content from your Nostr contact list plus whitelist.

```yaml
trust:
  depth: 1
```

Use when:
- Already have a Nostr identity with follows
- Trust your social graph
- Recommended for most users

### Depth 2: Extended Network

Content from follows of follows. Use carefully - can be noisy.

```yaml
trust:
  depth: 2
```

Use when:
- Want maximum discovery
- Have a well-curated follow list
- Accept more noise for more content

---

## Managing Trust

### Whitelist

Manually trusted publishers. Always visible regardless of depth.

**Add via UI:**
1. Go to **Trust** page
2. Click **Add to Whitelist**
3. Enter npub and optional note
4. Save

**Add via API:**
```bash
curl -X POST http://localhost:9999/api/trust/whitelist \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{"npub": "npub1...", "note": "Trusted uploader"}'
```

### Blacklist

Blocked publishers. Content never appears, regardless of trust.

**Add via UI:**
1. Go to **Trust** page
2. Click **Block User**
3. Enter npub and reason
4. Save

Blacklisting:
- Immediately removes all content from that publisher
- Prevents future content from appearing
- Syncs across sessions

**Add via API:**
```bash
curl -X POST http://localhost:9999/api/trust/blacklist \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{"npub": "npub1...", "reason": "Spam"}'
```

### Import Follows

Import your Nostr contact list:

1. Go to **Trust** page
2. Click **Import Follows**
3. Your Kind 3 (contact list) events are fetched
4. Follows are added to the trust graph

---

## Federated Curation

For more sophisticated trust, use **Curators**.

### What is a Curator?

A curator is a trusted entity that:
- Reviews content
- Applies rulesets (moderation policies)
- Signs verification decisions
- Publishes decisions to Nostr

### Why Use Curators?

| Approach | Pros | Cons |
|----------|------|------|
| **Simple WoT** | Easy, automatic | Limited moderation |
| **Curators** | Active moderation, rulesets | More setup |

### Adding Curators

1. Go to **Trust** → **Curators** tab
2. Click **Add Curator**
3. Enter curator's npub
4. Set weight (for aggregation)
5. Save

### Aggregation Policy

When multiple curators exist, decisions are aggregated:

| Mode | Behavior |
|------|----------|
| `any` | Any accept = content appears |
| `all` | All must accept |
| `quorum` | N of M must agree |
| `weighted` | Weight-based voting |

**Configure in UI:**
1. Go to **Trust** → **Curators**
2. Click **Aggregation Settings**
3. Choose mode
4. Set quorum (if applicable)
5. Save

**Configure via API:**
```bash
curl -X PUT http://localhost:9999/api/trust/aggregation \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{"mode": "quorum", "quorum_required": 2}'
```

---

## Trust Flow

### Simple Mode (Depth 1)

```
You → follow → Alice → uploads content → You see it
You → follow → Bob → follows → Carol → uploads → You DON'T see it (depth 1)
```

### With Curators

```
Curator → accepts → Torrent
You → trust → Curator
Result: You see the torrent
```

### Multiple Curators (Quorum)

```
Curator A → accepts
Curator B → accepts
Curator C → rejects

Policy: quorum=2
Result: Content appears (2/3 accept)
```

---

## Verification Decisions

Curators create signed decisions:

```json
{
  "decision": "accept",
  "reason_codes": [],
  "ruleset_type": "semantic",
  "ruleset_version": "1.0.0",
  "ruleset_hash": "sha256...",
  "target_infohash": "aabbccdd...",
  "curator_pubkey": "npub1...",
  "signature": "sig..."
}
```

### Viewing Decisions

1. Go to **Curation** page
2. Browse decisions
3. Filter by status, curator

### Legal Codes Priority

Certain rejection codes always take priority:

| Code | Description | Behavior |
|------|-------------|----------|
| `LEGAL_DMCA` | DMCA takedown | Always reject |
| `LEGAL_ILLEGAL` | Illegal content | Always reject |
| `ABUSE_SPAM` | Spam | Always reject |
| `ABUSE_MALWARE` | Malware | Always reject |

These override aggregation policy - one rejection with these codes means content is rejected.

---

## Security Considerations

### Key Management

Your Nostr identity is:
- **Your reputation** - How others trust you
- **Your access** - What content you can see
- **Your signature** - For publishing

Protect your nsec:
- Don't share it
- Don't commit to version control
- Consider hardware key for high security

### Curator Trust

Before trusting a curator:
- Review their reputation
- Check their ruleset version
- Consider their moderation style
- Start with low weight, increase over time

### Rogue Curators

If a curator goes rogue:
1. Remove from trust list
2. Their decisions stop affecting your index
3. Content they accepted remains (re-evaluate manually if needed)

---

## Best Practices

### Starting Out

1. Set depth to 1
2. Add a few known-good publishers to whitelist
3. Import your Nostr follows
4. Gradually discover more through your network

### Growing Your Network

1. Follow active uploaders on Nostr
2. Trust curators with good reputations
3. Blacklist bad actors promptly
4. Adjust depth based on noise level

### Running a Curator Node

1. Set depth to 0 (strict whitelist)
2. Enable tag filtering
3. Import/create rulesets
4. Sign and publish decisions
5. Share your npub with users

---

## Troubleshooting

### No Content Appearing

1. Check trust depth (0 = whitelist only)
2. Verify relays are connected
3. Confirm whitelist has entries (if depth 0)
4. Check indexer is running

### Too Much Spam

1. Lower trust depth
2. Use curators instead of simple WoT
3. Add spammers to blacklist
4. Enable tag filtering

### Content Disappearing

1. Check blacklist (may have blocked publisher)
2. Verify curator decisions
3. Check if publisher was removed from follows

---

## Next Steps

- [Curation](curation.md) - Set up as a curator
- [Configuration](configuration.md) - Configure trust settings
- [Architecture](architecture.md) - Understand the system
