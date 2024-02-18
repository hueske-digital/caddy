#!/bin/sh
while true
do
 inotifywait -r --exclude .swp -e create -e modify -e delete -e move /hosts
 caddy validate --config /etc/caddy/Caddyfile
 if [ $? -eq 0 ]
 then
  echo "Detected Caddy Configuration Change"
  echo "Executing: caddy reload --config /etc/caddy/Caddyfile"
  caddy reload --config /etc/caddy/Caddyfile
 fi
done
