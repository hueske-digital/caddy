# Proxy

_Uses the following image: https://github.com/thueske/caddy-cloudflare._

Fill in the `.env` file with your cloudflare token (Scope: Zone / Zone / Read and Zone / DNS / Edit). Make also sure that ssl mode in cloudflare is set to strict.

Start with `docker-compose up -d`.

Proxy hosts are configured in `/hosts/01_*.conf`. Please use the example files (`/hosts/examples/`) as a starting point (must be renamed to `*.conf`). Keep in mind that services that should only be accessible internally or exclusively via Cloudflare must use the access lists.

When changing configuration files you have to reload the proxy with `./reload`.
