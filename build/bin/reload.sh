#!/bin/sh
while true
do
 # Block until the first change.
 inotifywait -r --exclude .swp -e create -e modify -e delete -e move /hosts
 # Debounce: coalesce rapid bursts (e.g. ephemeral containers) by waiting
 # for a quiet window before reloading. Each event resets the timer.
 while inotifywait -r -q -t 2 --exclude .swp -e create -e modify -e delete -e move /hosts >/dev/null 2>&1
 do
  :
 done
 caddy validate --config /etc/caddy/Caddyfile
 if [ $? -eq 0 ]
 then
  echo "Detected Caddy Configuration Change"
  echo "Executing: caddy reload --config /etc/caddy/Caddyfile"
  caddy reload --config /etc/caddy/Caddyfile
 fi
done
