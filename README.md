# Caddy Reverse Proxy

Docker-based Caddy reverse proxy with automatic service discovery.

## Quick Start

```bash
# Clone and configure
git clone https://github.com/hueske-digital/caddy
cd caddy
cp .env.example .env
# Edit .env with your Cloudflare API token and email

# Start
make up
```

### Available Commands

| Command | Description |
|---------|-------------|
| `make up` | Start Caddy only |
| `make up-auth` | Start Caddy + TinyAuth |
| `make up-all` | Start all services |
| `make down` | Stop Caddy |
| `make logs` | Follow logs |

## How It Works

The **watcher** sidecar automatically discovers services and generates Caddy configs:

1. Your service joins a `*_caddy` network
2. Watcher detects it and connects Caddy to that network
3. Watcher reads `CADDY_*` environment variables from your service
4. Caddy config is generated and auto-reloaded

## Service Configuration

Add these environment variables to any service:

```yaml
services:
  myapp:
    image: myapp:latest
    environment:
      - CADDY_DOMAIN=app.example.com        # Required: domain(s), comma-separated
      - CADDY_TYPE=external                  # Required: external|internal|cloudflare
      - CADDY_PORT=8080                      # Required: container port
      - CADDY_ALLOWLIST=home.dyndns.org     # Optional: IP allowlist (external only)
      - CADDY_AUTH=true                      # Optional: enable forward auth (default: off)
      - CADDY_LOGGING=true                   # Optional: enable request logging (default: off)
      - CADDY_TLS=false                      # Optional: disable TLS (default: on)
      - CADDY_COMPRESSION=false              # Optional: disable compression (default: on)
      - CADDY_HEADER=false                   # Optional: disable security headers (default: on)
    networks:
      - caddy

networks:
  caddy:
```

### Types

| Type | Access |
|------|--------|
| `external` | Public (or restricted via allowlist) |
| `internal` | Private IP ranges only (10.x, 172.16-31.x, 192.168.x) |
| `cloudflare` | Cloudflare IP ranges only |

### Allowlist

Restrict `external` services by IP:

```yaml
CADDY_ALLOWLIST=home.dyndns.org,office.example.com,1.2.3.4
```

- Hostnames resolved via DNS-over-HTTPS (Cloudflare/Google)
- Auto-refreshes every 60 seconds
- Non-matching requests: connection aborted

## Manual Configs

Place custom `.conf` files in:
- `hosts/internal/` - internal services
- `hosts/external/` - public services
- `hosts/cloudflare/` - Cloudflare-proxied services

Available snippets (defined in `hosts/base.conf`):
- `(tls)` - Cloudflare DNS challenge
- `(internal)` - private IP matcher
- `(cloudflare)` - Cloudflare IP matcher
- `(compression)` - zstd/gzip
- `(header)` - security headers
- `(logging)` - stdout logging
- `(auth)` - forward auth (requires tinyauth)

## Authentication

Optional forward auth via tinyauth + OIDC provider (e.g., PocketID, Authelia):

```bash
# Configure in .env
TINYAUTH_SECRET=random-secret
TINYAUTH_APP_URL=https://auth.example.com
TINYAUTH_DOMAIN=auth.example.com
OIDC_PROVIDER_URL=https://pocketid.example.com
OIDC_CLIENT_ID=tinyauth
OIDC_CLIENT_SECRET=your-secret

# Start with auth
make up-auth
```

Then enable per service:
```yaml
environment:
  - CADDY_AUTH=true
```

## Status Dashboard

Set `CADDY_DOMAIN` in `.env` to enable the built-in status page. Defaults to `internal` access on port `8080`.

![Status Dashboard](docs/watcher.png)

## Environment Variables

### Required (`.env`)

| Variable | Description |
|----------|-------------|
| `CF_API_TOKEN` | Cloudflare API token (Zone Read + DNS Edit) |
| `EMAIL` | Email for SSL notifications |

### Optional (docker-compose)

| Variable | Default | Description |
|----------|---------|-------------|
| `NETWORK_SUFFIX` | `_caddy` | Network suffix to watch |
| `HOSTS_DIR` | `/hosts` | Config output directory |
| `DNS_REFRESH_INTERVAL` | `60` | Seconds between DNS refreshes |
| `CODE_EDITOR_URL` | - | Base URL for editor links in status page |

### Optional (`.env`)

| Variable | Description |
|----------|-------------|
| `CADDY_DOMAIN` | Status page domain (enables dashboard) |

## Ports

Required for external access:
- `80/tcp` - HTTP + Let's Encrypt
- `443/tcp` - HTTPS
- `443/udp` - HTTP/3

## Legacy Migration

For existing services using `proxy_apps` network with manual configs:

```bash
# Create legacy network (if not exists)
docker network create proxy_apps
```

- Legacy services continue working with manual `.conf` files
- New services use `*_caddy` networks with auto-discovery
- Migrate gradually by adding `CADDY_*` env vars

## Testing

### Unit Tests

```bash
cd watcher && go test -v ./...
```

Tests core functionality without Docker:
- `ParseCaddyEnv` - Environment variable parsing and validation
- Template generation for all types (internal/external/cloudflare)
- Config file writing and removal
- Domain extraction from config files
- Option handling (logging, TLS, compression, headers)

### Integration Tests

```bash
./watcher/test/integration.sh
```

Full end-to-end tests with Docker containers:

| Test | Description |
|------|-------------|
| Service start | Creates config when container with `CADDY_*` vars starts |
| Service stop | Config persists when container stops (not removed) |
| Service down | Config and network removed on `docker compose down` |
| External type | Config created in `hosts/external/` |
| Cloudflare type | Config includes `import cloudflare` directive |
| Logging option | `CADDY_LOGGING=true` adds `import logging` |
| Disabled options | `CADDY_TLS/COMPRESSION/HEADER=false` removes imports |
| Multiple domains | Comma-separated domains all included in config |
| File ownership | Files created with UID/GID 1000:1000 (Linux only) |
| Status API | `/api/status` returns valid JSON (if enabled) |

Options:
```bash
# Keep caddy stack running after tests (for debugging)
./watcher/test/integration.sh --keep-stack
```

The integration tests automatically:
- Build the watcher image locally
- Start the caddy stack if not running
- Clean up test containers and networks
- Stop the stack after tests (unless `--keep-stack`)

### Pre-Push Hook

Enable automatic testing before push:

```bash
git config core.hooksPath .githooks
```

This runs unit tests before every `git push`. If tests fail, push is aborted.

## Notes

- Set Cloudflare SSL mode to **Full (strict)**
- Caddy auto-reloads when configs change
- Generated configs are gitignored
