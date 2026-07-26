#!/bin/sh
# reload.sh laeuft im Hintergrund und laedt Caddy bei Konfigurationsaenderungen
# neu. Stirbt es - etwa weil inotifywait abbricht -, wirkt der Container
# weiterhin gesund, uebernimmt aber nie wieder eine Aenderung. Deshalb wird es
# beaufsichtigt und neu gestartet, statt es einmalig abzusetzen.
(
 while true
 do
  /docker/reload.sh
  echo "ERROR: reload watcher exited (code $?), restarting in 5s" >&2
  sleep 5
 done
) &

exec "$@"
