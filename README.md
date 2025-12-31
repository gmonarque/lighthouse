# Lighthouse

Self-hosted Nostr indexer for NIP-35 torrent events with federated curation. Comes with a Torznab API so it works with Prowlarr, Sonarr, Radarr, and other *arr apps.

**[Full Documentation](https://gmonarque.github.io/lighthouse/)** | [Quick Start](#quick-start) | [API Reference](docs/api-reference.md)

## Disclaimer

This software is a Nostr protocol indexer that reads publicly available NIP-35 events from Nostr relays. It does not host, distribute, or provide access to any copyrighted content. The software merely indexes metadata published on the decentralized Nostr network. Users are solely responsible for their use of this software and must comply with all applicable laws in their jurisdiction.

> **Note:** To fully understand the context and use case behind Lighthouse, please read the [whitepaper](whitepaper.pdf) first.

## What it does

- Indexes NIP-35 (Kind 2003) events from Nostr relays
- Publish metadata events to Nostr relays (parses .torrent file headers only)
- Filter what is indexed based on tags
- Web of Trust filtering - only see content from people you trust
- Federated curation- trust curators who apply moderation rulesets
- Verification decisions - cryptographically signed accept/reject decisions
- Report/appeal system - formal channels for content moderation
- Comments - NIP-35 compatible comment system (Kind 2004)
- Torznab API for seamless *arr apps integration
- Auto-fetches metadata from TMDB/OMDB
- Single Go binary + SQLite, runs anywhere
- Web UI built with Svelte

## Quick start

### From source (recommended)

Needs Go 1.22+ and Node.js 20+

```bash
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse
make build
./lighthouse
```

### Docker 

```bash
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse
docker-compose up -d
```

Open http://localhost:9999

## Configuration

Lighthouse uses `config.yaml`. A default one is created on first run:

```yaml
server:
  host: "0.0.0.0"
  port: 9999
  api_key: ""  # auto-generated

database:
  path: "./data/lighthouse.db"

nostr:
  identity:
    npub: ""
    nsec: ""
  relays:
    - url: "wss://relay.damus.io"
      enabled: true

trust:
  depth: 1  # 0=whitelist only, 1=follows, 2=follows of follows

enrichment:
  tmdb_api_key: ""
  omdb_api_key: ""
  enabled: true
```

### Environment variables

Override any setting with `LIGHTHOUSE_` prefix:

```bash
LIGHTHOUSE_SERVER_PORT=8080
LIGHTHOUSE_ENRICHMENT_TMDB_API_KEY=your_key
```

## Using with *arr apps

1. Complete the setup wizard at http://localhost:9999
2. Go to Settings and copy your API key
3. In Prowlarr/Sonarr/Radarr, add a new indexer:
   - Type: Torznab
   - URL: `http://localhost:9999/api/torznab`
   - API Key: paste from step 2
4. Test and save

## Web of Trust

Controls what content shows up based on who uploaded it:

| Depth | What you see |
|-------|--------------|
| 0 | Only whitelisted uploaders |
| 1 | Whitelist + people you follow on Nostr |
| 2 | Above + friends of friends (use carefully) |

From the Trust page you can:
- **Whitelist**: manually add trusted uploaders by npub
- **Blacklist**: block bad actors (removes all their torrents)
- **Import follows**: pull your Nostr contact list

## Categories

Full Torznab category support with subcategories.

## API Reference

### REST API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/stats` | GET | Dashboard statistics |
| `/api/search` | GET | Search torrents |
| `/api/torrents/:id` | GET | Torrent details |
| `/api/torrents/:id` | DELETE | Remove torrent |
| `/api/publish/parse-torrent` | POST | Parse .torrent file |
| `/api/publish` | POST | Publish torrent to relays |
| `/api/trust/whitelist` | GET/POST/DELETE | Manage whitelist |
| `/api/trust/whitelist/{npub}/discover-relays` | POST | Discover user's relays (NIP-65) |
| `/api/trust/blacklist` | GET/POST/DELETE | Manage blacklist |
| `/api/relays` | GET/POST/PUT/DELETE | Manage relays |
| `/api/settings` | GET/PUT | App settings |
| `/api/indexer/start` | POST | Start indexer |
| `/api/indexer/stop` | POST | Stop indexer |
| `/api/indexer/resync` | POST | Resync historical events |

### Torznab API

| Endpoint | Parameters | Description |
|----------|------------|-------------|
| `/api/torznab?t=caps` | - | Capabilities |
| `/api/torznab?t=search` | q, cat, limit | General search |
| `/api/torznab?t=tvsearch` | q, season, ep | TV search |
| `/api/torznab?t=movie` | q, imdbid, tmdbid | Movie search |

## Federated Curation

Lighthouse supports federated content curation through trusted curators:

| Component | Description |
|-----------|-------------|
| **Curators** | Trusted entities that review and validate content |
| **Rulesets** | Versioned moderation policies (censoring + semantic) |
| **Decisions** | Cryptographically signed accept/reject decisions |
| **Aggregation** | Combine decisions from multiple curators (quorum, weighted, etc.) |

See [Curation Documentation](docs/curation.md) for setup instructions.

## Project structure

```
lighthouse/
├── cmd/lighthouse/        # entry point
├── internal/
│   ├── api/              # HTTP handlers & router
│   ├── comments/         # torrent comments
│   ├── config/           # configuration
│   ├── curator/          # curation engine
│   ├── database/         # SQLite & migrations
│   ├── decision/         # verification decisions
│   ├── explorer/         # relay event discovery
│   ├── indexer/          # torrent indexing
│   ├── models/           # shared data models
│   ├── moderation/       # reports & appeals
│   ├── nostr/            # Nostr client & events
│   ├── relay/            # Nostr relay server
│   ├── ruleset/          # rule engine
│   ├── torznab/          # Torznab protocol
│   └── trust/            # Web of Trust
├── web/                  # Svelte frontend
├── docs/                 # documentation
└── docker/               # Docker stuff
```

## Development

```bash
make deps          # install dependencies
make dev           # backend with hot reload
make dev-frontend  # frontend dev server (separate terminal)
make test          # run tests
make build         # production build
make docker        # build docker image
```

## Documentation

- [Full Documentation](https://gmonarque.github.io/lighthouse/) - Complete user and developer guide
- [Getting Started](docs/getting-started.md) - Quick start guide
- [Installation](docs/installation.md) - Detailed installation
- [Configuration](docs/configuration.md) - Configuration reference
- [API Reference](docs/api-reference.md) - REST and Torznab API
- [Architecture](docs/architecture.md) - System design
- [Web of Trust](docs/web-of-trust.md) - Trust system guide
- [Curation](docs/curation.md) - Curator setup
- [Federation](docs/federation.md) - Multi-node deployment
- [Development](docs/development.md) - Contributing guide

## Links

- [NIP-35 spec](https://github.com/nostr-protocol/nips/blob/master/35.md) - Nostr torrent protocol
- [go-nostr](https://github.com/nbd-wtf/go-nostr) - Nostr library used
- [Whitepaper](whitepaper.pdf) - Project whitepaper

## License

MIT
