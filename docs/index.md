# Lighthouse

**Self-hosted Nostr indexer for NIP-35 torrent events with federated curation.**

Lighthouse replaces centralized torrent indexing websites with a censorship-resistant, community-governed alternative based on cryptographic trust.

---

## Key Features

- **Decentralized Discovery** - Indexes NIP-35 (Kind 2003) events from Nostr relays
- **Web of Trust** - Filter content based on cryptographic trust relationships
- **Federated Curation** - Distributed content moderation through trusted curators
- **Torznab API** - Seamless integration with Prowlarr, Sonarr, Radarr, and other *arr apps
- **Media Enrichment** - Auto-fetches metadata from TMDB/OMDB
- **Lightweight** - Single Go binary + SQLite, runs anywhere
- **Modern UI** - Web interface built with Svelte

---

## Why Lighthouse?

### The Problem

BitTorrent excels at file transport but lacks native content discovery. Historically, centralized indexing websites filled this gap, creating significant issues:

- **Single point of failure** - If the site shuts down, discovery disappears
- **Centralized trust** - Users must trust one entity to moderate content
- **Censorship vulnerability** - Easy targets for takedowns

### The Solution

Lighthouse uses the **Nostr protocol** as a global, censorship-resistant message bus. NIP-35 allows encapsulating torrent metadata in cryptographically signed events.

The **Web of Trust** model replaces centralized moderation with distributed curation:

| Actor | Role |
|-------|------|
| **Emitter** | Signs and publishes torrent metadata |
| **Curator** | Validates emitters, maintains follow lists |
| **User** | Subscribes to trusted curators |

---

## Quick Links

| Section | Description |
|---------|-------------|
| [Getting Started](getting-started.md) | 5-minute quick start guide |
| [Installation](installation.md) | Detailed installation instructions |
| [Configuration](configuration.md) | Configuration reference |
| [Architecture](architecture.md) | System design and components |
| [API Reference](api-reference.md) | REST and Torznab API documentation |
| [Web of Trust](web-of-trust.md) | Trust system deep dive |
| [Curation](curation.md) | Curator setup and ruleset guide |
| [Federation](federation.md) | Multi-instance deployment |
| [Development](development.md) | Contributing guide |

---

## Disclaimer

This software is a Nostr protocol indexer that reads publicly available NIP-35 events from Nostr relays. It does not host, distribute, or provide access to any copyrighted content. The software merely indexes metadata published on the decentralized Nostr network. Users are solely responsible for their use of this software and must comply with all applicable laws in their jurisdiction.

---

## License

MIT License - See [LICENSE](https://github.com/gmonarque/lighthouse/blob/main/LICENSE)
