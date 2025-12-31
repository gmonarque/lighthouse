# API Reference

Complete documentation for Lighthouse REST and Torznab APIs.

---

## Authentication

Most endpoints require an API key passed in the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" http://localhost:9999/api/stats
```

The API key is configured in `config.yaml` or auto-generated on first run.

---

## REST API

Base URL: `http://localhost:9999/api`

### Dashboard

#### Get Statistics

```http
GET /api/stats
```

Returns dashboard statistics.

**Response:**
```json
{
  "total_torrents": 1234,
  "total_events": 5678,
  "trusted_publishers": 42,
  "categories": {
    "movies": 456,
    "tv": 234,
    "audio": 123
  }
}
```

---

### Search

#### Search Torrents

```http
GET /api/search
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `q` | string | Search query |
| `category` | integer | Torznab category code |
| `limit` | integer | Max results (default: 50) |
| `offset` | integer | Pagination offset |

**Example:**
```bash
curl "http://localhost:9999/api/search?q=ubuntu&category=4000"
```

**Response:**
```json
{
  "results": [
    {
      "id": "abc123",
      "infohash": "aabbccdd...",
      "name": "Ubuntu 24.04 Desktop",
      "title": "Ubuntu 24.04 LTS",
      "size": 4500000000,
      "category": 4000,
      "seeders": 1500,
      "leechers": 50,
      "created_at": "2024-04-01T00:00:00Z",
      "magnet_uri": "magnet:?xt=urn:btih:..."
    }
  ],
  "total": 1
}
```

---

### Torrents

#### Get Torrent Details

```http
GET /api/torrents/{id}
```

**Response:**
```json
{
  "id": "abc123",
  "infohash": "aabbccdd...",
  "infohash_version": "v1",
  "name": "Example Torrent",
  "title": "Example Title",
  "size": 1000000000,
  "category": 2000,
  "tags": ["linux", "distro"],
  "magnet_uri": "magnet:?xt=urn:btih:...",
  "curation_status": "accepted",
  "created_at": "2024-01-01T00:00:00Z",
  "event_id": "nostr_event_id",
  "pubkey": "publisher_npub"
}
```

#### Delete Torrent

```http
DELETE /api/torrents/{id}
```

Removes a torrent from the local index.

---

### Publishing

#### Parse Torrent File

```http
POST /api/publish/parse-torrent
Content-Type: multipart/form-data
```

Upload a `.torrent` file to extract metadata.

**Request:**
```bash
curl -X POST \
  -F "file=@example.torrent" \
  http://localhost:9999/api/publish/parse-torrent
```

**Response:**
```json
{
  "name": "Example File",
  "infohash": "aabbccdd...",
  "size": 1000000000,
  "files": [
    {"path": "file1.txt", "size": 500000000},
    {"path": "file2.txt", "size": 500000000}
  ]
}
```

#### Publish Torrent

```http
POST /api/publish
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "Example Torrent",
  "infohash": "aabbccdd...",
  "size": 1000000000,
  "category": 2000,
  "tags": ["linux"],
  "trackers": ["udp://tracker.example.com:6969"]
}
```

**Response:**
```json
{
  "event_id": "nostr_event_id",
  "relays_published": ["wss://relay.damus.io"]
}
```

---

### Trust Management

#### Get Whitelist

```http
GET /api/trust/whitelist
```

**Response:**
```json
{
  "entries": [
    {
      "npub": "npub1...",
      "note": "Trusted uploader",
      "added_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### Add to Whitelist

```http
POST /api/trust/whitelist
Content-Type: application/json
```

**Request Body:**
```json
{
  "npub": "npub1...",
  "note": "Trusted uploader"
}
```

#### Remove from Whitelist

```http
DELETE /api/trust/whitelist/{npub}
```

#### Discover User Relays

Discovers a user's preferred relays via NIP-65 and adds their write relays to your relay list.

```http
POST /api/trust/whitelist/{npub}/discover-relays
```

**Response:**
```json
{
  "npub": "npub1...",
  "relays_found": 5,
  "relays_added": 3,
  "message": "Discovered 5 relays, added 3 new relays"
}
```

#### Discover All Trusted Relays

Discovers relays for all whitelisted users.

```http
POST /api/trust/whitelist/discover-all-relays
```

**Response:**
```json
{
  "users_processed": 10,
  "total_relays_added": 8,
  "results": [
    {"npub": "npub1...", "relays_added": 3},
    {"npub": "npub2...", "relays_added": 0, "error": "No NIP-65 relay list"}
  ]
}
```

#### Get Blacklist

```http
GET /api/trust/blacklist
```

#### Add to Blacklist

```http
POST /api/trust/blacklist
Content-Type: application/json
```

**Request Body:**
```json
{
  "npub": "npub1...",
  "reason": "Spam"
}
```

#### Remove from Blacklist

```http
DELETE /api/trust/blacklist/{npub}
```

---

### Curators (Federated Mode)

#### List Trusted Curators

```http
GET /api/trust/curators
```

**Response:**
```json
{
  "curators": [
    {
      "pubkey": "npub1...",
      "name": "Movie Curator",
      "weight": 1.0,
      "added_at": "2024-01-01T00:00:00Z"
    }
  ],
  "aggregation_policy": {
    "mode": "quorum",
    "quorum_required": 2
  }
}
```

#### Add Curator

```http
POST /api/trust/curators
Content-Type: application/json
```

**Request Body:**
```json
{
  "pubkey": "npub1...",
  "name": "Movie Curator",
  "weight": 1.0
}
```

#### Update Curator

```http
PUT /api/trust/curators/{pubkey}
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "Updated Name",
  "weight": 2.0
}
```

#### Remove Curator

```http
DELETE /api/trust/curators/{pubkey}
```

#### Update Aggregation Policy

```http
PUT /api/trust/aggregation
Content-Type: application/json
```

**Request Body:**
```json
{
  "mode": "quorum",
  "quorum_required": 2
}
```

Modes: `any`, `all`, `quorum`, `weighted`

---

### Decisions

#### List Decisions

```http
GET /api/decisions
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `status` | string | `accept` or `reject` |
| `curator` | string | Curator pubkey |
| `limit` | integer | Max results |
| `offset` | integer | Pagination |

**Response:**
```json
{
  "decisions": [
    {
      "decision_id": "abc123",
      "decision": "accept",
      "reason_codes": [],
      "ruleset_type": "semantic",
      "ruleset_version": "1.0.0",
      "target_infohash": "aabbccdd...",
      "curator_pubkey": "npub1...",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 100
}
```

#### Get Decisions for Torrent

```http
GET /api/decisions/{infohash}
```

---

### Rulesets

#### List Rulesets

```http
GET /api/rulesets
```

**Response:**
```json
{
  "rulesets": [
    {
      "id": "abc123",
      "name": "Default Censoring",
      "type": "censoring",
      "version": "1.0.0",
      "hash": "sha256...",
      "rules_count": 10
    }
  ]
}
```

#### Get Ruleset

```http
GET /api/rulesets/{id}
```

#### Import Ruleset

```http
POST /api/rulesets
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "Custom Rules",
  "type": "semantic",
  "version": "1.0.0",
  "rules": [
    {
      "id": "rule1",
      "name": "Minimum Size",
      "type": "threshold",
      "field": "size",
      "operator": "gte",
      "value": 1000000,
      "reason_code": "SEM_LOW_QUALITY"
    }
  ]
}
```

---

### Reports

#### Submit Report

```http
POST /api/reports
Content-Type: application/json
```

**Request Body:**
```json
{
  "kind": "report",
  "category": "dmca",
  "target_infohash": "aabbccdd...",
  "evidence": "Description of issue",
  "jurisdiction": "US"
}
```

Categories: `dmca`, `illegal`, `spam`, `malware`, `false_info`, `duplicate`, `other`

#### List Reports

```http
GET /api/reports
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `status` | string | `pending`, `acknowledged`, `investigating`, `resolved`, `rejected` |
| `category` | string | Filter by category |

#### Get Pending Reports

```http
GET /api/reports/pending
```

#### Update Report Status

```http
PUT /api/reports/{id}
Content-Type: application/json
```

**Request Body:**
```json
{
  "status": "resolved",
  "resolution": "Content removed"
}
```

#### Acknowledge Report

```http
POST /api/reports/{id}/acknowledge
```

---

### Comments

#### Get Comments for Torrent

```http
GET /api/torrents/{infohash}/comments
```

**Response:**
```json
{
  "comments": [
    {
      "id": "abc123",
      "content": "Great release!",
      "rating": 5,
      "author_pubkey": "npub1...",
      "created_at": "2024-01-01T00:00:00Z",
      "replies": []
    }
  ],
  "stats": {
    "total": 10,
    "average_rating": 4.5
  }
}
```

#### Add Comment

```http
POST /api/torrents/{infohash}/comments
Content-Type: application/json
```

**Request Body:**
```json
{
  "content": "Great release!",
  "rating": 5
}
```

#### Get Recent Comments

```http
GET /api/comments/recent
```

---

### Relays

#### List Relays

```http
GET /api/relays
```

**Response:**
```json
{
  "relays": [
    {
      "url": "wss://relay.damus.io",
      "name": "Damus",
      "preset": "public",
      "enabled": true,
      "connected": true,
      "last_event": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### Add Relay

```http
POST /api/relays
Content-Type: application/json
```

**Request Body:**
```json
{
  "url": "wss://relay.example.com",
  "name": "My Relay",
  "preset": "private"
}
```

#### Update Relay

```http
PUT /api/relays/{url}
Content-Type: application/json
```

**Request Body:**
```json
{
  "enabled": false
}
```

#### Delete Relay

```http
DELETE /api/relays/{url}
```

---

### Settings

#### Get Settings

```http
GET /api/settings
```

#### Update Settings

```http
PUT /api/settings
Content-Type: application/json
```

**Request Body:**
```json
{
  "trust.depth": 1,
  "enrichment.tmdb_api_key": "your-key"
}
```

#### Get Identity

```http
GET /api/settings/identity
```

#### Generate Identity

```http
POST /api/settings/identity/generate
```

#### Import Identity

```http
POST /api/settings/identity/import
Content-Type: application/json
```

**Request Body:**
```json
{
  "nsec": "nsec1..."
}
```

---

### Indexer Control

#### Get Indexer Status

```http
GET /api/indexer/status
```

**Response:**
```json
{
  "running": true,
  "events_processed": 12345,
  "last_event": "2024-01-01T00:00:00Z"
}
```

#### Start Indexer

```http
POST /api/indexer/start
```

#### Stop Indexer

```http
POST /api/indexer/stop
```

#### Resync Historical Events

Fetches historical torrent events from trusted uploaders. By default fetches all events (no time limit).

```http
POST /api/indexer/resync
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `days` | integer | Limit to last N days (default: 0 = no limit) |

**Example:**
```bash
# Fetch all historical events
curl -X POST http://localhost:9999/api/indexer/resync

# Fetch last 7 days only
curl -X POST "http://localhost:9999/api/indexer/resync?days=7"
```

**Response:**
```json
{
  "status": "syncing",
  "message": "Fetching historical torrents",
  "days": 0
}
```

---

## Torznab API

Torznab is a standardized API for torrent indexers, compatible with Prowlarr, Sonarr, Radarr, and other *arr applications.

Base URL: `http://localhost:9999/api/torznab`

### Authentication

Pass API key as query parameter:

```
/api/torznab?apikey=your-api-key&t=search&q=query
```

### Capabilities

```http
GET /api/torznab?t=caps
```

Returns XML describing supported features and categories.

### General Search

```http
GET /api/torznab?t=search&q={query}
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `q` | string | Search query |
| `cat` | string | Category IDs (comma-separated) |
| `limit` | integer | Max results (default: 100) |
| `offset` | integer | Pagination offset |

### TV Search

```http
GET /api/torznab?t=tvsearch
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `q` | string | Search query |
| `season` | string | Season number |
| `ep` | string | Episode number |
| `tvdbid` | string | TVDB ID |
| `imdbid` | string | IMDB ID |

### Movie Search

```http
GET /api/torznab?t=movie
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `q` | string | Search query |
| `imdbid` | string | IMDB ID |
| `tmdbid` | string | TMDB ID |

### Categories

| ID | Category |
|----|----------|
| 2000 | Movies |
| 2010 | Movies/Foreign |
| 2020 | Movies/Other |
| 2030 | Movies/SD |
| 2040 | Movies/HD |
| 2045 | Movies/UHD |
| 2050 | Movies/BluRay |
| 2060 | Movies/3D |
| 2070 | Movies/DVD |
| 2080 | Movies/WEB-DL |
| 5000 | TV |
| 5010 | TV/WEB-DL |
| 5020 | TV/Foreign |
| 5030 | TV/SD |
| 5040 | TV/HD |
| 5045 | TV/UHD |
| 5050 | TV/Other |
| 5060 | TV/Sport |
| 5070 | TV/Anime |
| 5080 | TV/Documentary |
| 3000 | Audio |
| 3010 | Audio/MP3 |
| 3020 | Audio/Video |
| 3030 | Audio/Audiobook |
| 3040 | Audio/Lossless |
| 4000 | PC |
| 4010 | PC/0day |
| 4020 | PC/ISO |
| 4030 | PC/Mac |
| 4040 | PC/Mobile-Other |
| 4050 | PC/Games |
| 4060 | PC/Mobile-iOS |
| 4070 | PC/Mobile-Android |
| 7000 | Books |
| 7010 | Books/Mags |
| 7020 | Books/EBook |
| 7030 | Books/Comics |
| 7040 | Books/Technical |
| 7050 | Books/Other |
| 7060 | Books/Foreign |
| 6000 | XXX |

---

## Error Responses

All endpoints return consistent error format:

```json
{
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request |
| 401 | Unauthorized (invalid/missing API key) |
| 404 | Not Found |
| 500 | Internal Server Error |

---

## Rate Limiting

Default limits:
- 100 requests per minute per API key
- 1000 requests per minute per IP

Exceeded limits return `429 Too Many Requests`.

---

## Next Steps

- [Configuration](configuration.md) - Configure API settings
- [Getting Started](getting-started.md) - Integration examples
