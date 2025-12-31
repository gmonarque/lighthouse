# Installation

Detailed installation instructions for various environments.

---

## System Requirements

### Minimum Requirements

| Resource | Requirement |
|----------|-------------|
| CPU | 1 core |
| RAM | 512 MB |
| Storage | 1 GB (grows with index size) |
| OS | Linux, macOS, Windows |

### Recommended

| Resource | Recommendation |
|----------|----------------|
| CPU | 2+ cores |
| RAM | 2 GB |
| Storage | 10 GB SSD |
| OS | Linux (Debian/Ubuntu) |

---

## Build Dependencies

### Go

Version 1.22 or higher required.

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install golang-go

# macOS (Homebrew)
brew install go

# Verify
go version
```

### Node.js

Version 20 or higher required.

```bash
# Ubuntu/Debian (using NodeSource)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# macOS (Homebrew)
brew install node@20

# Verify
node --version
npm --version
```

---

## Installation Methods

### Method 1: From Source

```bash
# Clone repository
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse

# Install dependencies
make deps

# Build
make build

# Run
./lighthouse
```

The binary will be created at `./lighthouse` (or `lighthouse.exe` on Windows).

### Method 2: Docker

```bash
# Clone repository
git clone https://github.com/gmonarque/lighthouse.git
cd lighthouse

# Build and run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Method 3: Docker (Manual)

```bash
# Build image
docker build -t lighthouse .

# Run container
docker run -d \
  --name lighthouse \
  -p 9999:9999 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config.yaml:/app/config.yaml \
  lighthouse
```

---

## Directory Structure

After installation:

```
lighthouse/
├── lighthouse           # Binary (or lighthouse.exe)
├── config.yaml          # Configuration (created on first run)
├── data/
│   └── lighthouse.db    # SQLite database
├── web/                 # Frontend source (development only)
└── internal/            # Backend source (development only)
```

---

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/lighthouse.service`:

```ini
[Unit]
Description=Lighthouse Nostr Indexer
After=network.target

[Service]
Type=simple
User=lighthouse
Group=lighthouse
WorkingDirectory=/opt/lighthouse
ExecStart=/opt/lighthouse/lighthouse
Restart=always
RestartSec=5

# Environment
Environment=LIGHTHOUSE_SERVER_HOST=0.0.0.0
Environment=LIGHTHOUSE_SERVER_PORT=9999

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable lighthouse
sudo systemctl start lighthouse
sudo systemctl status lighthouse
```

### launchd (macOS)

Create `~/Library/LaunchAgents/com.lighthouse.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.lighthouse</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/lighthouse</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>/usr/local/lighthouse</string>
</dict>
</plist>
```

Load the service:

```bash
launchctl load ~/Library/LaunchAgents/com.lighthouse.plist
```

---

## Reverse Proxy Setup

### Nginx

```nginx
server {
    listen 80;
    server_name lighthouse.example.com;

    location / {
        proxy_pass http://127.0.0.1:9999;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```
lighthouse.example.com {
    reverse_proxy localhost:9999
}
```

### Traefik (Docker)

```yaml
services:
  lighthouse:
    image: lighthouse
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.lighthouse.rule=Host(`lighthouse.example.com`)"
      - "traefik.http.services.lighthouse.loadbalancer.server.port=9999"
```

---

## SSL/TLS

For production deployments, always use HTTPS.

### With Certbot (Let's Encrypt)

```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx

# Obtain certificate
sudo certbot --nginx -d lighthouse.example.com

# Auto-renewal
sudo certbot renew --dry-run
```

### With Caddy

Caddy automatically obtains and renews SSL certificates:

```
lighthouse.example.com {
    reverse_proxy localhost:9999
}
```

---

## Updating

### From Source

```bash
cd lighthouse
git pull
make build
# Restart service
```

### Docker

```bash
cd lighthouse
git pull
docker-compose down
docker-compose build
docker-compose up -d
```

---

## Uninstallation

### Remove Binary

```bash
rm /usr/local/bin/lighthouse
rm -rf /opt/lighthouse
```

### Remove Service

```bash
# systemd
sudo systemctl stop lighthouse
sudo systemctl disable lighthouse
sudo rm /etc/systemd/system/lighthouse.service
sudo systemctl daemon-reload
```

### Remove Docker

```bash
docker-compose down -v
docker rmi lighthouse
```

---

## Next Steps

- [Configuration](configuration.md) - Configure Lighthouse
- [Getting Started](getting-started.md) - First-run setup
