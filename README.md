# Caddy Proxy with Docker

This repository provides a Docker-based setup for running a Caddy reverse proxy with support for internal, external, and Cloudflare-protected services. It includes automatic configuration reloading and DNS challenge support for SSL certificates.

## Features

- Reverse proxy for internal, external, and Cloudflare-protected services.
- Automatic SSL certificate management using Caddy.
- DNS challenge support for Cloudflare.
- Automatic configuration reload on changes.
- Pre-configured security headers and compression.
- Modular configuration for easy service management.

## Prerequisites

- Docker and Docker Compose installed on your system.
- A Cloudflare account (if using DNS challenge for SSL certificates).

## Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/hueske-digital/caddy
cd caddy
```

### 2. Configure Environment Variables

Copy the `.env.example` file to `.env` and fill in the required values:

```bash
cp .env.example .env
```

- `CF_API_TOKEN`: Your Cloudflare API token with the following permissions (can be generated using [this link](https://dash.cloudflare.com/profile/api-tokens?permissionGroupKeys=%5B%7B%22key%22%3A%22dns%22%2C%22type%22%3A%22edit%22%7D%2C%7B%22key%22%3A%22zone%22%2C%22type%22%3A%22read%22%7D%5D&name=Caddy&accountId=*&zoneId=all)):
  - Zone / Zone / Read
  - Zone / DNS / Edit
- `EMAIL`: Your email address for SSL certificate notifications.

### 3. Configure Proxy Hosts

Proxy host configurations are located in the `hosts` directory. Use the provided example files as a starting point:

- Internal services: `hosts/internal/*.example`
- External services: `hosts/external/*.example`
- Cloudflare-protected services: `hosts/cloudflare/*.example`

Rename the example files to `.conf` and customize them as needed. For example:

```bash
mv hosts/external/external.example hosts/external/my-service.conf
```

### 4. Start the Proxy

Run the following command to start the proxy:

```bash
docker compose up -d
```

### 5. Verify Configuration

Caddy automatically validates and reloads the configuration when changes are detected. Logs can be viewed using:

```bash
docker compose logs app -f
```

### 6. Open and Forward Required Ports

To allow external access to your services, ensure the following ports are open and forwarded to your Docker machine:  
- **Port 80 (TCP):** For HTTP traffic and Let's Encrypt challenges.
- **Port 443 (TCP):** For HTTPS traffic.
- **Port 443 (UDP):** For HTTP/3 traffic.

## Directory Structure

```
caddy/
├── build/
│   ├── Dockerfile          # Custom Caddy build with plugins
│   ├── bin/
│   │   ├── entrypoint.sh   # Entrypoint script for the container
│   │   └── reload.sh       # Script for automatic config reload
├── hosts/
│   ├── base.conf           # Base configuration for Caddy
│   ├── internal/           # Internal service configurations
│   ├── external/           # External service configurations
│   └── cloudflare/         # Cloudflare-protected service configurations
├── .env.example            # Example environment variables file
├── docker-compose.yml      # Docker Compose configuration
└── README.md               # Project documentation
```

## Configuration Details

### Base Configuration

The `hosts/base.conf` file includes global settings such as:

- Security headers
- Compression
- Logging
- TLS configuration with Cloudflare DNS challenge

### Internal Services

Internal services are accessible only from private IP ranges. Example configuration:

```conf
https://internal.hueske.services {
    import internal
    import tls
    import header

    handle @internal {
        reverse_proxy internal-app-1:8080
    }
    respond 403
}
```

### External Services

External services are publicly accessible. Example configuration:

```conf
https://external.hueske.services {
    import tls
    import compression
    import header

    reverse_proxy external-app-1:3000
}
```

### Cloudflare-Protected Services

Services behind Cloudflare are restricted to Cloudflare IP ranges. Example configuration:

```conf
https://cloudflare.hueske.services {
    import cloudflare
    import tls
    import compression
    import header

    handle @cloudflare {
        reverse_proxy cloudflare-app-1:3001
    }
    respond 403
}
```

## Plugins

This setup includes the following Caddy plugins:

- [Cloudflare DNS](https://github.com/caddy-dns/cloudflare): For DNS challenge support.
- [Replace Response](https://github.com/caddyserver/replace-response): For response manipulation.

## Automatic Configuration Reload

The `reload.sh` script monitors the `hosts` directory for changes and reloads the Caddy configuration automatically.

## Notes

- Ensure that the SSL mode in Cloudflare is set to **Strict**.
- Use the provided access lists (`internal` and `cloudflare`) to restrict access to services appropriately.