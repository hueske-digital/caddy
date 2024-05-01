# Proxy

Start with `docker-compose up -d`.

Proxy hosts are configured in `/hosts/external|cloudflare|internal/*.conf`. Please use the example files as a starting point (must be renamed to `*.conf`). Keep in mind that services that should only be accessible internally or exclusively via Cloudflare must use the access lists.

When changing the config, the proxy is automatically reloaded.

(For DNS-Challenge with Cloudflare: Fill in the `.env` file with your cloudflare token (Scope: Zone / Zone / Read and Zone / DNS / Edit). Make also sure that ssl mode in cloudflare is set to strict.)
