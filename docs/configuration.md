# Configuration

Complete configuration reference for Lighthouse.

---

## Configuration File

Lighthouse uses `config.yaml` in the working directory. A default configuration is created on first run.

### Complete Example

```yaml
server:
  host: "0.0.0.0"
  port: 9999
  api_key: ""  # auto-generated if empty

database:
  path: "./data/lighthouse.db"

nostr:
  identity:
    npub: ""  # your public key
    nsec: ""  # your private key (keep secret!)
  relays:
    - url: "wss://relay.damus.io"
      name: "Damus"
      preset: "public"
      enabled: true

trust:
  depth: 1  # 0=whitelist only, 1=follows, 2=follows of follows

indexer:
  tag_filter: []
  tag_filter_enabled: false

enrichment:
  enabled: true
  tmdb_api_key: ""
  omdb_api_key: ""
```

---

## Configuration Sections

### Server

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | string | `"0.0.0.0"` | Listen address |
| `port` | integer | `9999` | Listen port |
| `api_key` | string | (generated) | API key for Torznab authentication |

### Database

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `path` | string | `"./data/lighthouse.db"` | SQLite database file path |

### Nostr Identity

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `identity.npub` | string | `""` | Your Nostr public key (npub format) |
| `identity.nsec` | string | `""` | Your Nostr private key (nsec format) |

**Security Note:** Keep your `nsec` private! Never share it or commit it to version control.

### Nostr Relays

Relays are configured as an array:

```yaml
nostr:
  relays:
    - url: "wss://relay.example.com"
      name: "My Relay"
      preset: "public"
      enabled: true
```

| Option | Type | Description |
|--------|------|-------------|
| `url` | string | WebSocket URL of the relay |
| `name` | string | Display name |
| `preset` | string | Category: `public`, `private`, `censorship-resistant` |
| `enabled` | boolean | Whether to connect to this relay |

### Trust

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `depth` | integer | `1` | Web of Trust depth |

Trust depth values:

| Depth | Behavior |
|-------|----------|
| `0` | Only whitelisted publishers |
| `1` | Whitelist + people you follow |
| `2` | Above + friends of friends |

### Indexer

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `tag_filter` | array | `[]` | Tags to filter (include only these) |
| `tag_filter_enabled` | boolean | `false` | Enable tag filtering |

When tag filtering is enabled, only torrents with matching tags are indexed.

### Enrichment

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable metadata enrichment |
| `tmdb_api_key` | string | `""` | The Movie Database API key |
| `omdb_api_key` | string | `""` | Open Movie Database API key |

---

## Environment Variables

Override any configuration option using environment variables with the `LIGHTHOUSE_` prefix.

### Format

Convert the YAML path to uppercase with underscores:
- `server.port` → `LIGHTHOUSE_SERVER_PORT`
- `nostr.identity.npub` → `LIGHTHOUSE_NOSTR_IDENTITY_NPUB`
- `enrichment.tmdb_api_key` → `LIGHTHOUSE_ENRICHMENT_TMDB_API_KEY`

### Examples

```bash
# Change port
export LIGHTHOUSE_SERVER_PORT=8080

# Set API keys
export LIGHTHOUSE_ENRICHMENT_TMDB_API_KEY=your_key_here
export LIGHTHOUSE_ENRICHMENT_OMDB_API_KEY=your_key_here

# Set trust depth
export LIGHTHOUSE_TRUST_DEPTH=2

# Run
./lighthouse
```

### Docker Environment

```yaml
# docker-compose.yml
services:
  lighthouse:
    image: lighthouse
    environment:
      - LIGHTHOUSE_SERVER_PORT=9999
      - LIGHTHOUSE_TRUST_DEPTH=1
      - LIGHTHOUSE_ENRICHMENT_TMDB_API_KEY=your_key
```

---

## Runtime Configuration

Some settings can be changed at runtime through the web UI:

### Settings Page

- **Identity** - View npub/nsec, generate new, import existing
- **Torznab API** - View/copy API key and URL
- **Enrichment** - Configure TMDB/OMDB API keys
- **Tag Filter** - Enable/disable and manage filter tags

### Trust Page

- **Whitelist** - Manually trusted publishers
- **Blacklist** - Blocked publishers
- **Curators** - Trusted curators (federated mode)
- **Aggregation Policy** - How to combine curator decisions

### Relays Page

- Add/remove relays
- Enable/disable individual relays
- View connection status

---

## Data Directory

The default data directory is `./data/`. It contains:

| File | Description |
|------|-------------|
| `lighthouse.db` | SQLite database with all indexed data |

### Database Backup

```bash
# Stop the service first
sqlite3 ./data/lighthouse.db ".backup ./data/backup.db"
```

### Database Location

Change the database location:

```yaml
database:
  path: "/var/lib/lighthouse/lighthouse.db"
```

---

## Security Considerations

### API Key

The API key is used for:
- Torznab API authentication
- Protecting sensitive endpoints

Best practices:
- Use a strong, random key (auto-generated by default)
- Rotate periodically
- Don't expose in URLs (use headers)

### Private Key (nsec)

Your nsec is your Nostr identity. Protect it:
- Never share it
- Don't commit to version control
- Use environment variables in production
- Consider hardware key storage for high-security setups

### Network Security

- Use HTTPS in production (see [Installation](installation.md#ssltls))
- Firewall the port if not using a reverse proxy
- Consider VPN/Tor for privacy

---

## Preset Configurations

### Minimal (Personal Use)

```yaml
server:
  port: 9999
database:
  path: "./data/lighthouse.db"
trust:
  depth: 1
```

### Production

```yaml
server:
  host: "127.0.0.1"  # Behind reverse proxy
  port: 9999
database:
  path: "/var/lib/lighthouse/lighthouse.db"
trust:
  depth: 1
enrichment:
  enabled: true
```

### Curator Node

```yaml
server:
  port: 9999
trust:
  depth: 0  # Strict whitelist
indexer:
  tag_filter_enabled: true
  tag_filter:
    - movies
    - tv
```

---

## Troubleshooting

### Configuration Not Applied

1. Check YAML syntax (indentation matters)
2. Restart the service
3. Check for environment variable overrides

### Database Errors

```bash
# Check database integrity
sqlite3 ./data/lighthouse.db "PRAGMA integrity_check;"

# Vacuum to optimize
sqlite3 ./data/lighthouse.db "VACUUM;"
```

### Permission Issues

```bash
# Fix ownership
chown -R lighthouse:lighthouse /opt/lighthouse

# Fix permissions
chmod 600 config.yaml
chmod 700 data/
```

---

## Next Steps

- [Architecture](architecture.md) - Understand system design
- [API Reference](api-reference.md) - API documentation
- [Web of Trust](web-of-trust.md) - Configure trust
