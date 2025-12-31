# Getting Started

Get Lighthouse running in 5 minutes.

---

## Prerequisites

- **Go 1.22+** (for building from source)
- **Node.js 20+** (for frontend build)
- Or **Docker** (alternative)

---

## Quick Install

### Option 1: From Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse

# Build and run
make build
./lighthouse
```

### Option 2: Docker

```bash
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse
docker-compose up -d
```

---

## First Run

1. Open **http://localhost:9999** in your browser
2. Complete the **Setup Wizard**:
   - Generate or import a Nostr identity
   - Select relay presets
   - Configure enrichment APIs (optional)
3. You're ready to go!

---

## Setup Wizard Steps

### Step 1: Identity

Choose one of:
- **Generate new identity** - Creates a new Nostr keypair
- **Import existing** - Use your existing nsec

Your identity is used to:
- Sign published torrents
- Follow other users for Web of Trust
- Publish moderation decisions (if acting as curator)

### Step 2: Relays

Select a relay preset:

| Preset | Description |
|--------|-------------|
| **Public** | Major public relays (recommended for beginners) |
| **Private** | No default relays (for advanced users) |
| **Custom** | Specify your own relay URLs |

### Step 3: Enrichment (Optional)

Add API keys for automatic metadata enrichment:
- **TMDB** - Movie/TV metadata, posters
- **OMDB** - Ratings, plot summaries

Both services offer free API keys.

---

## Integrating with *arr Apps

Lighthouse provides a Torznab API compatible with Prowlarr, Sonarr, Radarr, and other *arr applications.

### Setup Steps

1. Go to **Settings** in Lighthouse
2. Copy your **API Key**
3. Note the **Torznab URL**: `http://localhost:9999/api/torznab`

### In Prowlarr

1. Go to **Indexers > Add Indexer**
2. Select **Generic Torznab**
3. Configure:
   - **Name**: Lighthouse
   - **URL**: `http://localhost:9999/api/torznab`
   - **API Key**: (paste from Lighthouse)
4. **Test** and **Save**

### In Sonarr/Radarr

1. Go to **Settings > Indexers > Add**
2. Select **Torznab**
3. Configure with the same URL and API key
4. **Test** and **Save**

---

## Basic Usage

### Dashboard

The dashboard shows:
- Total indexed torrents
- Recent additions
- Indexer status
- Curation statistics

### Search

Search for content using:
- **Keywords** - Title, name
- **Categories** - Movies, TV, Audio, etc.
- **Filters** - Size, seeders, date

### Publishing

To publish a torrent:
1. Go to **Publish**
2. Upload or paste a `.torrent` file
3. Edit metadata if needed
4. Click **Publish to Nostr**

The torrent metadata is signed with your identity and broadcast to configured relays.

---

## Next Steps

- [Installation](installation.md) - Detailed installation guide
- [Configuration](configuration.md) - Configuration options
- [Web of Trust](web-of-trust.md) - Understand trust filtering
- [Curation](curation.md) - Set up content curation

---

## Troubleshooting

### Port Already in Use

Change the port in `config.yaml`:
```yaml
server:
  port: 8080
```

Or use environment variable:
```bash
LIGHTHOUSE_SERVER_PORT=8080 ./lighthouse
```

### Cannot Connect to Relays

1. Check your internet connection
2. Verify relay URLs are correct
3. Some relays may require authentication

### No Torrents Appearing

1. Check that the indexer is running (green status in sidebar)
2. Verify relays are connected (Relays page)
3. Check Web of Trust settings (Trust page)
4. With Trust Depth 0, you need to manually whitelist publishers

### Build Errors

```bash
# Ensure Go is installed
go version

# Ensure Node.js is installed
node --version

# Clean and rebuild
make clean
make deps
make build
```
