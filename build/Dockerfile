ARG CADDY_VERSION=2.7

FROM caddy:${CADDY_VERSION}-builder-alpine AS builder
RUN xcaddy build \
    --with github.com/caddy-dns/cloudflare

FROM caddy:${CADDY_VERSION}-alpine
RUN apk add --update --no-cache inotify-tools
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
COPY bin/*.sh /docker/
RUN chmod +x /docker/*.sh
