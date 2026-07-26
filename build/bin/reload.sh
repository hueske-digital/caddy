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
 # validate prueft die GESAMTE Konfiguration. Schlaegt sie fehl, wird nichts
 # geladen - auch keine unbeteiligte Site. Eine einzelne fehlerhafte Datei
 # blockiert also alle weiteren Aenderungen, bis sie korrigiert ist. Deshalb
 # muss der Fehler sichtbar sein: frueher lief der Zweig stillschweigend leer,
 # und der Reload blieb ohne jeden Logeintrag aus.
 if caddy validate --config /etc/caddy/Caddyfile
 then
  echo "Detected Caddy Configuration Change"
  echo "Executing: caddy reload --config /etc/caddy/Caddyfile"
  caddy reload --config /etc/caddy/Caddyfile
 else
  echo "ERROR: Caddy configuration is invalid - NOT reloading." >&2
  echo "ERROR: All config changes stay inactive until this is fixed." >&2
 fi
done
