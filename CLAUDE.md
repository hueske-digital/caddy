# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

Caddy reverse proxy with automatic service discovery via Docker events. The watcher sidecar generates Caddyfiles from container `CADDY_*` environment variables.

## Commands

```bash
docker compose up -d          # Start
docker compose logs -f        # All logs
docker compose logs watcher   # Watcher logs
docker compose down           # Stop
```

## Architecture

```
caddy/
├── build/                    # Caddy Docker image
│   ├── Dockerfile
│   ├── Caddyfile
│   └── bin/
│       ├── entrypoint.sh
│       └── reload.sh         # Auto-reload on config change
├── watcher/                  # Go sidecar (auto-discovery)
│   ├── main.go               # Entry point, event loop
│   ├── config.go             # ENV parsing
│   ├── docker.go             # Docker client
│   ├── caddy.go              # Config generation (embedded templates)
│   ├── dns.go                # DNS-over-HTTPS for allowlists
│   ├── status.go             # status.json management
│   ├── server.go             # Status web UI
│   └── Dockerfile
├── hosts/
│   ├── base.conf             # Shared snippets
│   ├── internal/             # Generated internal configs
│   ├── external/             # Generated external configs
│   └── cloudflare/           # Generated cloudflare configs
└── docker-compose.yml
```

## Watcher Event Handling

| Event | Action |
|-------|--------|
| `network:create` (*_caddy) | Connect Caddy, generate configs |
| `network:destroy` | Remove config |
| `network:connect` | Generate config for new container |
| `container:start` | Regenerate config |

## Service ENV Variables (Single-Service Mode)

| Variable | Required | Description |
|----------|----------|-------------|
| `CADDY_DOMAIN` | Yes | Domain(s), comma-separated |
| `CADDY_TYPE` | Yes | `external`, `internal`, or `cloudflare` |
| `CADDY_PORT` | Yes | Container port |
| `CADDY_DNS_PROVIDER` | No | `cloudflare`, `hetzner`, or `http` (default: cloudflare) |
| `CADDY_ALLOWLIST` | No | IPs/hostnames for allowlist (external only) |
| `CADDY_TRUSTED_PROXIES` | No | IPs/hostnames for X-Forwarded-* trust |

## Multi-Service Mode

For containers with `network_mode: service:*` (e.g., VPN setups) that expose multiple services on different ports, use the `_servicename` suffix:

```yaml
environment:
  # Service: browser
  CADDY_DOMAIN_browser: browser.example.com
  CADDY_PORT_browser: "3000"
  CADDY_TYPE_browser: external
  CADDY_ALLOWLIST_browser: my-hostname.example.com

  # Service: streamer
  CADDY_DOMAIN_streamer: streamer.example.com
  CADDY_PORT_streamer: "3030"
  CADDY_TYPE_streamer: internal
```

All single-service variables support the `_servicename` suffix. Config files are named `{container}-{servicename}_{network}.conf`.

## Watcher ENV Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NETWORK_SUFFIX` | `_caddy` | Network suffix to watch |
| `HOSTS_DIR` | `/hosts` | Config output directory |
| `ENABLE_LOGGING` | `true` | Add `import logging` to configs |
| `DNS_REFRESH_INTERVAL` | `60` | Seconds between allowlist DNS refresh |
| `CODE_EDITOR_URL` | - | Optional base URL for editor links in status UI |
| `WILDCARD_DOMAINS` | - | Domains for wildcard certificates (comma-separated) |
| `WILDCARD_DNS_PROVIDER` | `cloudflare` | DNS provider for wildcard certs (cloudflare\|hetzner) |

## Key Implementation Details

- Config files are named `{container}_{network}.conf` (e.g., `myapp-web-1_myproject_caddy.conf`)
- Templates are embedded in `watcher/caddy.go` (no external files)
- Allowlist DNS uses DNS-over-HTTPS (1.1.1.1, 8.8.8.8 fallback)
- Configs written atomically via temp file + rename
- Watcher can discover itself (for status page) if `CADDY_*` vars are set

## CI/CD

- `.github/workflows/docker-build-push.yml` - Caddy image
- `.github/workflows/watcher-build-push.yml` - Watcher image
