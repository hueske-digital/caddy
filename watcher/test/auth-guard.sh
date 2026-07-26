#!/bin/bash
# Waechter fuer die Auth-Kette: Bypass, Gruppen und Identitaets-Header.
#
# Der Angriff: traegt die forward_auth-Unteranfrage den Host des CLIENTS statt
# den des Auth-Servers, passt sie dort zu keinem Site-Block. Caddy antwortet auf
# einen nicht zuordenbaren Host mit "200, 0 Bytes", und forward_auth liest jedes
# 2xx als "authentifiziert" - die geschuetzte Seite wird freigegeben, ohne dass
# der Auth-Dienst je gefragt wurde.
#
# Caddy hat das mit PR #7454 (v2.11.0) fuer TLS-Upstreams geloest: reverse_proxy
# setzt den Host dort selbst auf die Upstream-Adresse. Fuer http://-Upstreams
# gilt das NICHT - mit 2.11.4 ist der Bypass dort reproduzierbar. Deshalb setzt
# der Watcher den Host bei http:// explizit und bei https:// nicht (dort waere
# er redundant und wuerde nur Reload-Warnungen erzeugen).
#
# Geprueft wird die EIGENSCHAFT, nicht die Umsetzung: sagt der Auth-Server nein,
# darf der geschuetzte Inhalt nicht erscheinen - egal wie Caddy intern mit dem
# Host umgeht. Aendert Caddy sein Verhalten erneut, in welche Richtung auch
# immer, schlaegt dieser Test fehl statt es unbemerkt zu lassen.
#
# Die Auth-Bloecke kommen aus dem Watcher-Code selbst, nicht aus einem Nachbau -
# sonst wuerde etwas anderes geprueft als das, was im Betrieb entsteht.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(dirname "$SCRIPT_DIR")"
WORKDIR="$(mktemp -d)"
CONTAINER="authbypass-test"
CADDY_IMAGE="${CADDY_IMAGE:-caddy:2.11-alpine}"
GEN_TEST="$WATCHER_DIR/zz_authbypass_gen_test.go"

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'
PASS=0; FAIL=0
info() { echo "${YELLOW}[info]${NC} $1"; }
ok()   { echo "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
bad()  { echo "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL+1)); }

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1
    rm -f "$GEN_TEST"
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

echo "════════════════════════════════════════════════"
echo " forward_auth: Bypass ueber nicht passenden Host"
echo "════════════════════════════════════════════════"
echo "Caddy-Image: $CADDY_IMAGE"

# ── Auth-Bloecke vom Watcher erzeugen lassen ─────────────────────────────────
cat > "$GEN_TEST" <<'GOEOF'
package main

import (
	"os"
	"testing"
)

// Schreibt den Auth-Block so, wie er im Betrieb entsteht.
func TestZZAuthBypassGen(t *testing.T) {
	out := os.Getenv("AUTH_BLOCK_OUT")
	if out == "" {
		t.Skip()
	}
	var paths, groups []string
	if p := os.Getenv("AUTH_PATHS"); p != "" {
		paths = []string{p}
	}
	if g := os.Getenv("AUTH_GROUPS"); g != "" {
		groups = []string{g}
	}
	block := generateAuthBlock(os.Getenv("AUTH_URL"), paths, nil, groups)
	if err := os.WriteFile(out, []byte(block+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
GOEOF

gen_block() { # gen_block <name> <auth-url> [pfad] [gruppe]
    ( cd "$WATCHER_DIR" \
        && AUTH_BLOCK_OUT="$WORKDIR/$1.block" AUTH_URL="$2" \
           AUTH_PATHS="${3:-}" AUTH_GROUPS="${4:-}" \
           go test -run TestZZAuthBypassGen . >/dev/null 2>&1 )
    [ -s "$WORKDIR/$1.block" ]
}

gen_block plain  "http://auth.local:9000"                          || { bad "Auth-Block (http) nicht erzeugt"; exit 1; }
gen_block tls    "https://auth.local:9443"                         || { bad "Auth-Block (https) nicht erzeugt"; exit 1; }
gen_block groups "https://auth.local:9443" "/admin/*" "admins"     || { bad "Auth-Block (Gruppen) nicht erzeugt"; exit 1; }
rm -f "$GEN_TEST"
info "Auth-Bloecke aus dem Watcher-Code erzeugt"

# ── Testaufbau ───────────────────────────────────────────────────────────────
# Die Auth-Server sind NUR unter ihrem eigenen Host erreichbar. Sie erlauben,
# wenn der Client X-Allow: yes mitschickt, und verweigern sonst. Damit laesst
# sich neben dem Sicherheitsfall auch pruefen, dass der Auth-Server ueberhaupt
# erreicht wird - sonst wuerde ein kaputter Aufbau als "sicher" durchgehen.
auth_logic() {
    # Der Client steuert ueber X-Allow, ob der Auth-Server zustimmt, und ueber
    # X-Groups, welche Gruppen er meldet. So laesst sich pruefen, dass der
    # Gruppen-Check der Auth-ANTWORT folgt und nicht dem, was der Client sendet.
    echo '	@erlaubt header X-Allow yes'
    echo '	handle @erlaubt {'
    echo '		header Remote-User "alice"'
    echo '		header Remote-Groups {header.X-Groups}'
    echo '		respond 204'
    echo '	}'
    echo '	handle {'
    echo '		respond "AUTH-SAGT-NEIN" 403'
    echo '	}'
}

{
    echo '{'
    echo '	admin off'
    echo '}'
    echo 'https://auth.local:9443 {'
    echo '	tls internal'
    auth_logic
    echo '}'
    echo 'http://auth.local:9000 {'
    auth_logic
    echo '}'
    # Geschuetzte Sites - Auth-Block unveraendert aus dem Watcher.
    echo 'http://:9001 {'
    cat "$WORKDIR/plain.block"
    echo '	respond "GEHEIMER-INHALT" 200'
    echo '}'
    echo 'http://:9002 {'
    cat "$WORKDIR/tls.block"
    echo '	respond "GEHEIMER-INHALT" 200'
    echo '}'
    # Diese Site gibt zurueck, welche Identitaets-Header beim Backend ankommen.
    echo 'http://:9003 {'
    cat "$WORKDIR/groups.block"
    echo '	respond "USER=[{header.Remote-User}] GROUPS=[{header.Remote-Groups}]" 200'
    echo '}'
    # Dieselbe Auth, aber im remote_ip-Fall: bei einer Allowlist steckt der
    # Auth-Block in einem handle @allowed, also an einer strukturell anderen
    # Stelle. Geprueft wird, dass Scrub und Gruppen dort genauso greifen.
    echo 'http://:9004 {'
    echo '	@allowed {'
    echo '		remote_ip private_ranges'
    echo '	}'
    echo '	handle @allowed {'
    cat "$WORKDIR/groups.block"
    echo '		respond "USER=[{header.Remote-User}] GROUPS=[{header.Remote-Groups}]" 200'
    echo '	}'
    echo '	handle {'
    echo '		error 404'
    echo '	}'
    echo '}'
} > "$WORKDIR/Caddyfile"

info "Caddy starten"
docker rm -f "$CONTAINER" >/dev/null 2>&1
docker run -d --name "$CONTAINER" \
    --add-host auth.local:127.0.0.1 \
    -v "$WORKDIR/Caddyfile:/etc/caddy/Caddyfile:ro" \
    -p 19501:9001 -p 19502:9002 -p 19503:9003 -p 19504:9004 "$CADDY_IMAGE" >/dev/null 2>&1
sleep 6

if ! docker ps --filter "name=$CONTAINER" --format '{{.ID}}' | grep -q .; then
    bad "Caddy startet nicht - erzeugte Konfiguration ungueltig?"
    docker logs "$CONTAINER" 2>&1 | tail -12
    exit 1
fi

# Damit die Unteranfrage an den TLS-Auth-Server nicht am Zertifikat scheitert.
docker exec "$CONTAINER" caddy trust >/dev/null 2>&1
docker restart "$CONTAINER" >/dev/null 2>&1
sleep 6

body() { # body <port> <host> [extra-header]
    if [ -n "${3:-}" ]; then
        curl -s -H "Host: $2" -H "$3" "http://localhost:$1/" 2>/dev/null
    else
        curl -s -H "Host: $2" "http://localhost:$1/" 2>/dev/null
    fi
}
status() {
    if [ -n "${3:-}" ]; then
        curl -s -o /dev/null -w '%{http_code}' -H "Host: $2" -H "$3" "http://localhost:$1/" 2>/dev/null
    else
        curl -s -o /dev/null -w '%{http_code}' -H "Host: $2" "http://localhost:$1/" 2>/dev/null
    fi
}

# scenario <port> <beschreibung>
scenario() {
    local port="$1" label="$2"
    echo; echo "── $label"

    # Positivkontrolle zuerst: erreicht die Unteranfrage den Auth-Server
    # ueberhaupt? Ohne das koennte ein kaputter Aufbau faelschlich "sicher"
    # aussehen, weil einfach alles scheitert.
    local allowed; allowed="$(body "$port" app.local 'X-Allow: yes')"
    if [ "$allowed" = "GEHEIMER-INHALT" ]; then
        ok "Auth-Server wird erreicht und erlaubt den Zugriff (Positivkontrolle)"
    else
        bad "Positivkontrolle fehlgeschlagen - Auth-Server nicht erreichbar (Antwort: '${allowed:-leer}', Status $(status "$port" app.local 'X-Allow: yes'))"
        echo "     Der Sicherheitsteil unten ist damit nicht aussagekraeftig."
    fi

    # Der eigentliche Waechter: Auth verweigert, Client-Host passt NICHT zum
    # Site-Block des Auth-Servers.
    local denied st
    denied="$(body "$port" app.local)"
    st="$(status "$port" app.local)"
    if [ "$denied" = "GEHEIMER-INHALT" ]; then
        bad "AUTH-BYPASS: geschuetzter Inhalt ohne Authentifizierung ausgeliefert (Status $st)"
        echo "     Die Unteranfrage traegt offenbar den Client-Host."
        echo "     Fix: header_up Host {http.reverse_proxy.upstream.hostport} im Auth-Block."
    else
        ok "geschuetzter Inhalt bleibt verborgen (Status $st)"
    fi
}

scenario 19501 "Auth ueber http:// - Watcher muss den Host explizit setzen"
scenario 19502 "Auth ueber https:// - Caddy setzt den Host selbst (PR #7454)"

# ── Gruppen und Identitaets-Header ───────────────────────────────────────────
# Geprueft wird, dass der Gruppen-Check der AUTH-ANTWORT folgt und nicht dem,
# was der Client schickt - und dass auf Pfaden ohne Auth keine vom Client
# gesetzten Remote-*-Header ans Backend gelangen.
ask() { # ask <port> <pfad> <header...>
    local port="$1" path="$2"; shift 2
    curl -s -w ' [%{http_code}]' -H "Host: app.local" "$@" "http://localhost:$port$path" 2>/dev/null
}

identity_scenario() { # identity_scenario <port> <label>
    local port="$1" label="$2" res

    echo; echo "── $label: Gruppen-Beschraenkung"
    res="$(ask "$port" /admin/x -H 'X-Allow: yes' -H 'X-Groups: admins')"
    case "$res" in
        *"GROUPS=[admins]"*"[200]") ok "passende Gruppe wird durchgelassen" ;;
        *) bad "passende Gruppe abgelehnt: $res" ;;
    esac

    res="$(ask "$port" /admin/x -H 'X-Allow: yes' -H 'X-Groups: users')"
    case "$res" in
        *"[403]") ok "fremde Gruppe wird mit 403 abgewiesen" ;;
        *) bad "fremde Gruppe NICHT abgewiesen: $res" ;;
    esac

    echo; echo "── $label: Gruppen sind nicht faelschbar"
    res="$(ask "$port" /admin/x -H 'X-Allow: yes' -H 'X-Groups: users' -H 'Remote-Groups: admins')"
    case "$res" in
        *"[403]") ok "vom Client gesetztes Remote-Groups wird ignoriert" ;;
        *) bad "GRUPPEN-BYPASS: Client-Header hat entschieden: $res" ;;
    esac

    echo; echo "── $label: Pfade ohne Auth, Identitaets-Header werden entfernt"
    res="$(ask "$port" /public -H 'Remote-User: angreifer' -H 'Remote-Groups: admins')"
    case "$res" in
        *"USER=[] GROUPS=[]"*) ok "gefaelschte Remote-*-Header erreichen das Backend nicht" ;;
        *) bad "HEADER-SPOOFING: Backend sieht Client-Header: $res" ;;
    esac
}

identity_scenario 19503 "einfache Site"
identity_scenario 19504 "mit Allowlist (Auth in handle @allowed)"

# ── Zustand der Ursache in Caddy, nur Bericht ────────────────────────────────
echo; echo "── Ursache in Caddy (nur Bericht, kein Fehler)"
unmatched="$(docker exec "$CONTAINER" sh -c \
    'wget -qS -O /dev/null --header="Host: gibtsnicht.local" http://127.0.0.1:9000/ 2>&1 | head -1' 2>/dev/null)"
echo "     unbekannter Host am Listener: ${unmatched:-<keine Antwort>}"
case "$unmatched" in
    *200*) info "weiterhin 200 - der explizite Host bei http:// bleibt notwendig" ;;
    *)     info "nicht mehr 200 - Ursache moeglicherweise behoben" ;;
esac

# ── Was der Watcher erzeugt ──────────────────────────────────────────────────
echo; echo "── Erzeugte Auth-Bloecke"
if grep -q "header_up Host" "$WORKDIR/plain.block"; then
    ok "http://: header_up Host gesetzt"
else
    bad "http://: header_up Host FEHLT - Bypass moeglich"
fi
if grep -q "header_up Host" "$WORKDIR/tls.block"; then
    info "https://: header_up Host gesetzt - redundant, erzeugt Reload-Warnungen"
else
    ok "https://: kein redundantes header_up Host"
fi

echo
echo "════════════════════════════════════════════════"
echo " ${GREEN}bestanden: $PASS${NC}   ${RED}fehlgeschlagen: $FAIL${NC}"
echo "════════════════════════════════════════════════"
[ $FAIL -eq 0 ] || docker logs "$CONTAINER" 2>&1 | tail -15
exit $((FAIL > 0))
