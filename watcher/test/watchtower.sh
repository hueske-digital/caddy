#!/bin/bash
# Integrationstest fuer das Verhalten des Watchers waehrend Watchtower-Updates.
#
# Watchtower aktualisiert einen Container, indem es ihn stoppt, ENTFERNT und
# unter gleichem Namen neu anlegt. In diesem Fenster ist der Container fuer den
# Watcher schlicht nicht vorhanden. Frueher wurde dabei sofort aufgeraeumt und
# die Domain fiel aus dem Proxy - genau das pruefen die Faelle hier nach.
#
# Isolation: der Test benutzt einen eigenen NETWORK_SUFFIX. Der Watcher fasst
# ausschliesslich Netzwerke mit diesem Suffix an, produktive "_caddy"-Netze
# bleiben also unberuehrt.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(dirname "$SCRIPT_DIR")"

SUFFIX="_wtcaddytest"
PROJECT="wtcaddytest"
NET1="proj1${SUFFIX}"
NET2="proj2${SUFFIX}"
CADDY_NAME="${PROJECT}-app-1"
IMAGE="alpine:3"
# Verkuerzte Karenz, damit der Test nicht in Echtzeit warten muss. Sie muss
# aber deutlich ueber dem realen Abwesenheitsfenster liegen, sonst prueft der
# Test nicht die Logik, sondern nur die Geschwindigkeit der Docker-Engine:
# 20 Container gleichzeitig neu anzulegen dauert auf einer Laptop-Engine
# durchaus 15-20 Sekunden. In Produktion steht der Default auf 30 Minuten.
GRACE="45s"

WORKDIR="$(mktemp -d)"
HOSTS="$WORKDIR/hosts"
LOG="$WORKDIR/watcher.log"
WATCHER_PID=""

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'
PASS=0; FAIL=0

info() { echo "${YELLOW}[info]${NC} $1"; }
ok()   { echo "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
bad()  { echo "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL+1)); }

cleanup() {
    info "Aufraeumen"
    stop_watcher
    for c in $(docker ps -aq --filter "label=wtcaddytest=1" 2>/dev/null); do
        docker rm -f "$c" >/dev/null 2>&1
    done
    for n in "$NET1" "$NET2"; do
        docker network rm "$n" >/dev/null 2>&1
    done
    if [ "${KEEP_WORKDIR:-0}" = "1" ]; then
        echo "Arbeitsverzeichnis behalten: $WORKDIR"
    else
        rm -rf "$WORKDIR"
    fi
}
trap cleanup EXIT

start_watcher() {
    HOSTS_DIR="$HOSTS" \
    NETWORK_SUFFIX="$SUFFIX" \
    COMPOSE_PROJECT_NAME="$PROJECT" \
    CONFIG_CLEANUP_GRACE="$GRACE" \
    DNS_REFRESH_INTERVAL=3600 \
    "$WORKDIR/watcher" >>"$LOG" 2>&1 &
    WATCHER_PID=$!
    sleep 2
}

stop_watcher() {
    if [ -n "$WATCHER_PID" ] && kill -0 "$WATCHER_PID" 2>/dev/null; then
        kill "$WATCHER_PID" 2>/dev/null
        wait "$WATCHER_PID" 2>/dev/null
    fi
    WATCHER_PID=""
}

# run_service <name> <network> <domain>
run_service() {
    docker run -d --name "$1" --network "$2" --label wtcaddytest=1 \
        -e CADDY_DOMAIN="$3" -e CADDY_TYPE=internal -e CADDY_PORT=8080 \
        "$IMAGE" sleep 3600 >/dev/null
}

# Im Betrieb laeuft der Abgleich alle fuenf Minuten. Fuer den Test wird er
# stattdessen ueber ein echtes network:connect-Event angestossen: ein neuer
# Container im selben Netz loest denselben Codepfad aus.
poke_reconcile() {
    local n="poke-$RANDOM"
    docker run -d --name "$n" --network "$NET1" --label wtcaddytest=1 \
        "$IMAGE" sleep 30 >/dev/null 2>&1
    sleep 2
    docker rm -f "$n" >/dev/null 2>&1
}

conf_path() { echo "$HOSTS/internal/$1_$2.conf"; }

# wait_for_config <container> <network> <timeout_s>
wait_for_config() {
    local p; p="$(conf_path "$1" "$2")"
    local i=0
    while [ $i -lt "${3:-20}" ]; do
        [ -f "$p" ] && return 0
        sleep 1; i=$((i+1))
    done
    return 1
}

# assert_config_stable_during <seconds> <container> <network> -- <cmd...>
# Pollt die Datei engmaschig, waehrend das Kommando laeuft. Ein kurzzeitiges
# Verschwinden wuerde einem Test, der nur am Ende prueft, entgehen.
assert_config_stable_during() {
    local secs="$1" container="$2" network="$3"; shift 4
    local p; p="$(conf_path "$container" "$network")"

    local missing=0
    ( "$@" ) >/dev/null 2>&1 &
    local job=$!

    local deadline=$((SECONDS + secs))
    while [ $SECONDS -lt $deadline ]; do
        [ -f "$p" ] || missing=1
        sleep 0.2
    done
    wait $job 2>/dev/null

    return $missing
}

echo "════════════════════════════════════════════════"
echo " Watchtower-Szenarien fuer den Watcher"
echo "════════════════════════════════════════════════"
echo "Arbeitsverzeichnis: $WORKDIR"

info "Watcher bauen"
if ! (cd "$WATCHER_DIR" && go build -o "$WORKDIR/watcher" .); then
    bad "Build fehlgeschlagen"; exit 1
fi

mkdir -p "$HOSTS/internal" "$HOSTS/external" "$HOSTS/cloudflare"

info "Testumgebung anlegen (Netze, Caddy-Platzhalter, Dienste)"
docker network create "$NET1" >/dev/null
docker network create "$NET2" >/dev/null
docker run -d --name "$CADDY_NAME" --label wtcaddytest=1 "$IMAGE" sleep 3600 >/dev/null
run_service "proj1-web-1" "$NET1" "web.example.com"
run_service "proj1-api-1" "$NET1" "api.example.com"
run_service "proj2-web-1" "$NET2" "other.example.com"

start_watcher

# ── Fall 1 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 1: Configs werden ueberhaupt erzeugt"
if wait_for_config "proj1-web-1" "$NET1" && wait_for_config "proj2-web-1" "$NET2"; then
    ok "Configs fuer beide Projekte erzeugt"
else
    bad "Configs wurden nicht erzeugt"; echo "--- Log ---"; tail -30 "$LOG"; exit 1
fi

# ── Fall 2 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 2: Watchtower-Update (stop + rm + neu anlegen)"
if assert_config_stable_during 12 "proj1-web-1" "$NET1" -- \
    bash -c "docker stop -t 2 proj1-web-1 >/dev/null; docker rm proj1-web-1 >/dev/null; sleep 4; \
             docker run -d --name proj1-web-1 --network $NET1 --label wtcaddytest=1 \
               -e CADDY_DOMAIN=web.example.com -e CADDY_TYPE=internal -e CADDY_PORT=8080 \
               $IMAGE sleep 3600 >/dev/null"
then
    ok "Config blieb waehrend des gesamten Update-Fensters bestehen"
else
    bad "Config verschwand waehrend des Updates"
fi
if wait_for_config "proj1-web-1" "$NET1" 15; then
    ok "Config nach dem Update weiterhin vorhanden"
else
    bad "Config nach dem Update verschwunden"
fi

# ── Fall 3 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 3: langsames Update (Container laenger weg als ein Reconcile)"
if assert_config_stable_during 10 "proj1-api-1" "$NET1" -- \
    bash -c "docker stop -t 2 proj1-api-1 >/dev/null; docker rm proj1-api-1 >/dev/null; sleep 8; \
             docker run -d --name proj1-api-1 --network $NET1 --label wtcaddytest=1 \
               -e CADDY_DOMAIN=api.example.com -e CADDY_TYPE=internal -e CADDY_PORT=8080 \
               $IMAGE sleep 3600 >/dev/null"
then
    ok "Config ueberstand auch das laengere Fenster"
else
    bad "Config verschwand beim langsamen Update"
fi

# ── Fall 4 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 4: Watcher-Neustart waehrend ein Container weg ist"
docker stop -t 2 proj1-web-1 >/dev/null 2>&1; docker rm proj1-web-1 >/dev/null 2>&1
stop_watcher
start_watcher
sleep 3
if [ -f "$(conf_path "proj1-web-1" "$NET1")" ]; then
    ok "Config ueberlebt den Watcher-Neustart, obwohl der Container fehlt"
else
    bad "Config direkt nach dem Watcher-Neustart geloescht"
fi
docker run -d --name proj1-web-1 --network "$NET1" --label wtcaddytest=1 \
    -e CADDY_DOMAIN=web.example.com -e CADDY_TYPE=internal -e CADDY_PORT=8080 \
    "$IMAGE" sleep 3600 >/dev/null
sleep 3
if [ -f "$(conf_path "proj1-web-1" "$NET1")" ]; then
    ok "Config nach Rueckkehr des Containers weiterhin da"
else
    bad "Config nach Rueckkehr des Containers verschwunden"
fi

# ── Fall 5 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 5: fremdes Projekt bleibt unberuehrt"
if [ -f "$(conf_path "proj2-web-1" "$NET2")" ]; then
    ok "Config des zweiten Projekts unangetastet"
else
    bad "Config des zweiten Projekts wurde mit abgeraeumt"
fi

# ── Fall 6 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 6: handgepflegte Config wird nie angefasst"
MANUAL="$HOSTS/internal/handgepflegt_manual.conf"
printf 'https://manual.example.com {\n    respond "manual"\n}\n' > "$MANUAL"
sleep 3
[ -f "$MANUAL" ] && ok "Handgepflegte Config unberuehrt" || bad "Handgepflegte Config wurde geloescht"

# ── Fall 7 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 7: dauerhaft entfernter Container wird nach der Karenz aufgeraeumt"
docker stop -t 2 proj1-api-1 >/dev/null 2>&1; docker rm proj1-api-1 >/dev/null 2>&1

info "Reconcile anstossen (startet die Karenz von $GRACE)"
poke_reconcile
if [ -f "$(conf_path "proj1-api-1" "$NET1")" ]; then
    ok "Config bleibt zunaechst stehen - Karenz laeuft"
else
    bad "Config wurde sofort geloescht, ohne Karenz"
fi

info "Karenz abwarten"
sleep 48
gone=0
for _ in $(seq 1 5); do
    poke_reconcile
    if [ ! -f "$(conf_path "proj1-api-1" "$NET1")" ]; then gone=1; break; fi
done
if [ $gone -eq 1 ]; then
    ok "Config des entfernten Containers wurde aufgeraeumt"
else
    bad "Config des entfernten Containers blieb liegen"
fi
if [ -f "$(conf_path "proj1-web-1" "$NET1")" ]; then
    ok "Config des laufenden Containers blieb dabei erhalten"
else
    bad "Config eines laufenden Containers wurde mit aufgeraeumt"
fi

# ── Fall 8 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 8: Massen-Update - 20 Container gleichzeitig durchgetauscht"
BULK=20
info "$BULK Dienste anlegen"
for i in $(seq 1 $BULK); do
    run_service "bulk-$i-1" "$NET1" "bulk$i.example.com"
done
allthere=1
for i in $(seq 1 $BULK); do
    wait_for_config "bulk-$i-1" "$NET1" 30 || allthere=0
done
[ $allthere -eq 1 ] && ok "alle $BULK Configs erzeugt" || bad "nicht alle Configs erzeugt"

info "alle $BULK gleichzeitig stoppen, entfernen und neu anlegen (wie Watchtower)"
missing_during=0
(
  for i in $(seq 1 $BULK); do
    ( docker stop -t 1 "bulk-$i-1" >/dev/null 2>&1
      docker rm "bulk-$i-1" >/dev/null 2>&1
      sleep $((RANDOM % 4))
      docker run -d --name "bulk-$i-1" --network "$NET1" --label wtcaddytest=1 \
        -e CADDY_DOMAIN="bulk$i.example.com" -e CADDY_TYPE=internal -e CADDY_PORT=8080 \
        "$IMAGE" sleep 3600 >/dev/null 2>&1 ) &
  done
  wait
) &
bulkjob=$!
# Waehrend des gesamten Umbaus engmaschig pruefen, dass keine Config verschwindet.
deadline=$((SECONDS + 25))
while [ $SECONDS -lt $deadline ]; do
    for i in $(seq 1 $BULK); do
        [ -f "$(conf_path "bulk-$i-1" "$NET1")" ] || { missing_during=1; break; }
    done
    [ $missing_during -eq 1 ] && break
    sleep 0.3
done
wait $bulkjob 2>/dev/null
if [ $missing_during -eq 0 ]; then
    ok "keine einzige Config verschwand waehrend des Massen-Updates"
else
    bad "mindestens eine Config verschwand waehrend des Massen-Updates"
fi
sleep 5
stillthere=1
for i in $(seq 1 $BULK); do
    [ -f "$(conf_path "bulk-$i-1" "$NET1")" ] || stillthere=0
done
[ $stillthere -eq 1 ] && ok "nach dem Massen-Update sind alle Configs da" || bad "Configs fehlen nach dem Massen-Update"

info "Bulk-Dienste wieder entfernen"
for i in $(seq 1 $BULK); do docker rm -f "bulk-$i-1" >/dev/null 2>&1; done

# ── Fall 9 ────────────────────────────────────────────────────────────────
echo; echo "── Fall 9: Update mit GEAENDERTER Konfiguration (neues Image, neue Domain)"
docker rm -f proj1-web-1 >/dev/null 2>&1
sleep 1
docker run -d --name proj1-web-1 --network "$NET1" --label wtcaddytest=1 \
    -e CADDY_DOMAIN=web-neu.example.com -e CADDY_TYPE=internal -e CADDY_PORT=9090 \
    "$IMAGE" sleep 3600 >/dev/null
sleep 4
CONF="$(conf_path "proj1-web-1" "$NET1")"
if [ -f "$CONF" ] && grep -q "web-neu.example.com" "$CONF" && grep -q "proj1-web-1:9090" "$CONF"; then
    ok "geaenderte Konfiguration wurde uebernommen"
else
    bad "geaenderte Konfiguration nicht uebernommen"
fi

# ── Fall 10 ───────────────────────────────────────────────────────────────
echo; echo "── Fall 10: schnelle Neustarts hintereinander (Flapping)"
flap_missing=0
for round in 1 2 3 4 5; do
    docker rm -f proj1-web-1 >/dev/null 2>&1
    docker run -d --name proj1-web-1 --network "$NET1" --label wtcaddytest=1 \
        -e CADDY_DOMAIN=web-neu.example.com -e CADDY_TYPE=internal -e CADDY_PORT=9090 \
        "$IMAGE" sleep 3600 >/dev/null 2>&1
    [ -f "$CONF" ] || flap_missing=1
done
sleep 4
if [ $flap_missing -eq 0 ] && [ -f "$CONF" ]; then
    ok "Config ueberstand fuenf schnelle Neustarts"
else
    bad "Config ging bei schnellen Neustarts verloren"
fi

# ── Fall 11 ───────────────────────────────────────────────────────────────
echo; echo "── Fall 11: Netzwerk verschwindet, waehrend der Watcher nicht laeuft"
stop_watcher
docker rm -f proj2-web-1 >/dev/null 2>&1
docker network rm "$NET2" >/dev/null 2>&1
start_watcher
sleep 3
GONECONF="$(conf_path "proj2-web-1" "$NET2")"
if [ -f "$GONECONF" ]; then
    ok "Config bleibt zunaechst - Karenz laeuft auch hier"
else
    bad "Config wurde ohne Karenz geloescht"
fi
info "Karenz abwarten; Aufraeumen laeuft im 5-Minuten-Zyklus, deshalb hier nur die Karenz pruefen"

# ── Fall 12 ───────────────────────────────────────────────────────────────
echo; echo "── Fall 12: Caddy-Container selbst wird durchgetauscht"
caddy_missing=0
docker rm -f "$CADDY_NAME" >/dev/null 2>&1
sleep 3
[ -f "$CONF" ] || caddy_missing=1
docker run -d --name "$CADDY_NAME" --label wtcaddytest=1 "$IMAGE" sleep 3600 >/dev/null
sleep 4
[ -f "$CONF" ] || caddy_missing=1
if [ $caddy_missing -eq 0 ]; then
    ok "Configs unberuehrt, waehrend der Caddy-Container erneuert wird"
else
    bad "Configs verschwanden beim Erneuern des Caddy-Containers"
fi

# ── Fall 13 ───────────────────────────────────────────────────────────────
echo; echo "── Fall 13: Container haengt sich nur kurz ab und wieder an"
docker network disconnect "$NET1" proj1-web-1 >/dev/null 2>&1
sleep 3
detach_ok=1
[ -f "$CONF" ] || detach_ok=0
docker network connect "$NET1" proj1-web-1 >/dev/null 2>&1
sleep 4
[ -f "$CONF" ] || detach_ok=0
if [ $detach_ok -eq 1 ]; then
    ok "kurzzeitiges Abhaengen vom Netz laesst die Config unberuehrt"
else
    bad "Config verschwand beim kurzzeitigen Abhaengen"
fi

echo
echo "════════════════════════════════════════════════"
echo " ${GREEN}bestanden: $PASS${NC}   ${RED}fehlgeschlagen: $FAIL${NC}"
echo "════════════════════════════════════════════════"
[ $FAIL -eq 0 ] || { echo "--- Watcher-Log ---"; tail -60 "$LOG"; }
exit $((FAIL > 0))
