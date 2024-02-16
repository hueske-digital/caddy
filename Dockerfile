FROM caddy:2-builder-alpine AS builder
RUN xcaddy build \
    --with github.com/caddy-dns/cloudflare \
    --with github.com/caddyserver/replace-response \
    --with github.com/greenpau/caddy-security

FROM caddy:2-alpine
RUN apk add --update --no-cache inotify-tools
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
