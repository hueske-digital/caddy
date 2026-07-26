#!/bin/bash
# Waechter fuer die Auth-Kette: Bypass, Gruppen und Identitaets-Header.
#
# Geprueft wird gegen echtes Caddy, mit den Auth-Bloecken, die der Watcher selbst
# erzeugt - nicht mit einem Nachbau. Und systematisch statt stichprobenartig:
# jede Kombination aus Auth-Modus, Gruppen und Platzierung laeuft dieselben
# Zusicherungen durch.
#
# Der Anlass fuer die Matrix war ein Fehler, der genau durch eine nicht getestete
# Kombination geschluepft ist: bei Full-Site-Auth mit Gruppen ging die
# error-Direktive verloren, womit der Gruppen-Matcher wirkungslos war und JEDE
# Gruppe durchgekommen waere. Eine Stichprobe deckt das nicht auf, eine Matrix
# schon.
#
# Zusicherungen pro Variante:
#
#   P1  Auth erlaubt, Gruppe passt        -> Zugriff, und das Backend sieht die
#                                            Identitaet aus der Auth-Antwort
#   P2  Auth erlaubt, Gruppe passt nicht  -> abgewiesen (nur mit Gruppen)
#   P3  wie P2, Client faelscht die passende Gruppe dazu -> abgewiesen
#   P4  Auth verweigert                   -> kein Zugriff
#   P5  Pfad ohne Auth, Client faelscht   -> Backend sieht keine Identitaet
#   P6  Pfad mit Auth, Client faelscht    -> Auth-Identitaet gewinnt
#   P7  Wurzel eines Auth-Verzeichnisses  -> ebenfalls geschuetzt. Caddys
#       path-Matcher trifft mit "/admin/*" NICHT /admin selbst; ohne Ergaenzung
#       waere ausgerechnet die Wurzel des Adminbereichs offen.
#   P8  Auth antwortet 2xx OHNE Identitaets-Header, Client faelscht einen ->
#       das Backend darf ihn nicht sehen. Genau diese Luecke war in Caddy
#       2.10.0-2.11.1 offen (GHSA-7r4p-vjf4-gxv4): das bedingte Set unterblieb,
#       geloescht wurde nichts, und der Client-Wert lief durch. Mit dem
#       bedingungslosen Scrub haengt die Zusicherung nicht an der Caddy-Version.
#
# Dazu der Bypass ueber einen nicht passenden Host: traegt die
# forward_auth-Unteranfrage den Host des CLIENTS, passt sie beim Auth-Server zu
# keinem Site-Block. Caddy antwortet auf einen unbekannten Host mit "200,
# 0 Bytes", und forward_auth liest jedes 2xx als "authentifiziert". Caddy hat das
# mit PR #7454 (v2.11.0) fuer TLS-Upstreams geloest, fuer http:// nicht - dort
# setzt der Watcher den Host explizit.
#
# Geprueft wird durchgaengig die WIRKUNG, nicht die Umsetzung. Aendert Caddy sein
# Verhalten erneut, in welche Richtung auch immer, schlaegt der Test fehl statt es
# unbemerkt zu lassen.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(dirname "$SCRIPT_DIR")"
WORKDIR="$(mktemp -d)"
CONTAINER="authguard-test"
CADDY_IMAGE="${CADDY_IMAGE:-caddy:2.11-alpine}"
GEN_TEST="$WATCHER_DIR/zz_authguard_gen_test.go"

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

echo "════════════════════════════════════════════════════════"
echo " Auth-Kette: Bypass, Gruppen, Identitaets-Header"
echo "════════════════════════════════════════════════════════"
echo "Caddy-Image: $CADDY_IMAGE"

# ── Auth-Bloecke vom Watcher erzeugen lassen ─────────────────────────────────
cat > "$GEN_TEST" <<'GOEOF'
package main

import (
	"os"
	"strings"
	"testing"
)

// Schreibt den Auth-Block so, wie er im Betrieb entsteht.
func TestZZAuthGuardGen(t *testing.T) {
	out := os.Getenv("AUTH_BLOCK_OUT")
	if out == "" {
		t.Skip()
	}
	split := func(v string) []string {
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	block := generateAuthBlock(
		os.Getenv("AUTH_URL"),
		split(os.Getenv("AUTH_PATHS")),
		split(os.Getenv("AUTH_EXCEPT")),
		split(os.Getenv("AUTH_GROUPS")),
	)
	if err := os.WriteFile(out, []byte(block+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
GOEOF

gen_block() { # gen_block <datei> <auth-url> <paths> <except> <groups>
    ( cd "$WATCHER_DIR" \
        && AUTH_BLOCK_OUT="$WORKDIR/$1.block" AUTH_URL="$2" \
           AUTH_PATHS="$3" AUTH_EXCEPT="$4" AUTH_GROUPS="$5" \
           go test -run TestZZAuthGuardGen . >/dev/null 2>&1 )
    [ -s "$WORKDIR/$1.block" ] || { bad "Auth-Block $1 nicht erzeugt"; exit 1; }
}

# ── Varianten-Matrix ─────────────────────────────────────────────────────────
# name : paths : except : groups : Pfad MIT Auth : Pfad OHNE Auth ("-" = keiner)
VARIANTS=(
    "full-nogrp::::/admin/x:-"
    "full-grp:::admins:/admin/x:-"
    "paths-nogrp:/admin/*:::/admin/x:/public"
    "paths-grp:/admin/*::admins:/admin/x:/public"
    "except-nogrp::/health::/public:/health"
    "except-grp::/health:admins:/public:/health"
)
AUTH_SERVER="https://auth.local:9443"

# Der Client steuert ueber X-Allow, ob der Auth-Server zustimmt, und ueber
# X-Groups, welche Gruppen er meldet. Nur so laesst sich pruefen, dass der
# Gruppen-Check der ANTWORT folgt und nicht dem, was der Client sendet.
auth_logic() {
    echo '	@erlaubt header X-Allow yes'
    echo '	handle @erlaubt {'
    echo '		header Remote-User "alice"'
    echo '		header Remote-Groups {header.X-Groups}'
    echo '		respond 204'
    echo '	}'
    # Stimmt zu, ohne Identitaet zu melden - die Bedingung aus dem Caddy-Advisory.
    echo '	@nackt header X-Allow bare'
    echo '	handle @nackt {'
    echo '		respond 204'
    echo '	}'
    echo '	handle {'
    echo '		respond "AUTH-SAGT-NEIN" 403'
    echo '	}'
}
BACKEND='respond "USER=[{header.Remote-User}] GROUPS=[{header.Remote-Groups}]" 200'

parse_variant() { # setzt v_name, v_paths, v_except, v_groups, v_covered, v_uncovered
    IFS=':' read -r v_name v_paths v_except v_groups v_covered v_uncovered extra <<< "$1"
    # Ein verrutschtes Feld wuerde stillschweigend die falsche Zusicherung
    # pruefen - lieber laut abbrechen.
    if [ -n "${extra:-}" ] || [ -z "$v_covered" ]; then
        bad "Matrix-Zeile fehlerhaft (Feldzahl): $1"
        exit 1
    fi
}

info "Auth-Bloecke erzeugen und Konfiguration bauen"
{
    echo '{'; echo '	admin off'; echo '}'
    echo 'https://auth.local:9443 {'; echo '	tls internal'; auth_logic; echo '}'
    echo 'http://auth.local:9000 {'; auth_logic; echo '}'

    port=9010
    for v in "${VARIANTS[@]}"; do
        parse_variant "$v"
        gen_block "$v_name" "$AUTH_SERVER" "$v_paths" "$v_except" "$v_groups"

        # Platzierung A: einfache Site
        echo "http://:$port {"
        cat "$WORKDIR/$v_name.block"
        echo "	$BACKEND"
        echo '}'
        # Platzierung B: hinter einer Allowlist, Auth im handle @allowed
        echo "http://:$((port+1)) {"
        echo '	@allowed {'
        echo '		remote_ip private_ranges'
        echo '	}'
        echo '	handle @allowed {'
        cat "$WORKDIR/$v_name.block"
        echo "		$BACKEND"
        echo '	}'
        echo '	handle {'
        echo '		error 404'
        echo '	}'
        echo '}'
        port=$((port+2))
    done

    # Bypass-Szenarien: Auth-Upstream ohne bzw. mit TLS
    gen_block bypass-plain "http://auth.local:9000" "" "" ""
    gen_block bypass-tls   "https://auth.local:9443" "" "" ""
    echo 'http://:9001 {'; cat "$WORKDIR/bypass-plain.block"; echo '	respond "GEHEIMER-INHALT" 200'; echo '}'
    echo 'http://:9002 {'; cat "$WORKDIR/bypass-tls.block";   echo '	respond "GEHEIMER-INHALT" 200'; echo '}'
} > "$WORKDIR/Caddyfile"
rm -f "$GEN_TEST"

PUBLISH="-p 19001:9001 -p 19002:9002"
port=9010
for _ in "${VARIANTS[@]}"; do
    PUBLISH="$PUBLISH -p $((port+10000)):$port -p $((port+10001)):$((port+1))"
    port=$((port+2))
done

info "Caddy starten"
docker rm -f "$CONTAINER" >/dev/null 2>&1
# shellcheck disable=SC2086
docker run -d --name "$CONTAINER" --add-host auth.local:127.0.0.1 \
    -v "$WORKDIR/Caddyfile:/etc/caddy/Caddyfile:ro" $PUBLISH "$CADDY_IMAGE" >/dev/null 2>&1
sleep 6
if ! docker ps --filter "name=$CONTAINER" --format '{{.ID}}' | grep -q .; then
    bad "Caddy startet nicht - erzeugte Konfiguration ungueltig?"
    docker logs "$CONTAINER" 2>&1 | tail -15
    exit 1
fi
# Damit die Unteranfrage an den TLS-Auth-Server nicht am Zertifikat scheitert.
docker exec "$CONTAINER" caddy trust >/dev/null 2>&1
docker restart "$CONTAINER" >/dev/null 2>&1
sleep 6

req() { # req <port> <pfad> <header...>
    local p="$1" path="$2"; shift 2
    curl -s -w ' [%{http_code}]' -H "Host: app.local" "$@" "http://localhost:$p$path" 2>/dev/null
}

check_variant() { # check_variant <port> <label> <groups> <covered> <uncovered>
    local p="$1" label="$2" groups="$3" covered="$4" uncovered="$5" res

    # P1
    res="$(req "$p" "$covered" -H 'X-Allow: yes' -H 'X-Groups: admins')"
    case "$res" in
        *"USER=[alice] GROUPS=[admins]"*"[200]") ok "$label P1 Zugriff mit passender Gruppe" ;;
        *) bad "$label P1 kein Zugriff oder falsche Identitaet: $res" ;;
    esac

    if [ -n "$groups" ]; then
        # P2
        res="$(req "$p" "$covered" -H 'X-Allow: yes' -H 'X-Groups: gaeste')"
        case "$res" in
            *"[403]") ok "$label P2 falsche Gruppe abgewiesen" ;;
            *) bad "$label P2 GRUPPEN-BYPASS, falsche Gruppe durchgelassen: $res" ;;
        esac

        # P3
        res="$(req "$p" "$covered" -H 'X-Allow: yes' -H 'X-Groups: gaeste' -H 'Remote-Groups: admins')"
        case "$res" in
            *"[403]") ok "$label P3 gefaelschte Gruppe ignoriert" ;;
            *) bad "$label P3 GRUPPEN-BYPASS ueber Client-Header: $res" ;;
        esac
    fi

    # P4
    res="$(req "$p" "$covered" -H 'X-Groups: admins')"
    case "$res" in
        *"USER=[alice]"*) bad "$label P4 AUTH-BYPASS, Zugriff trotz verweigerter Auth: $res" ;;
        *) ok "$label P4 kein Zugriff bei verweigerter Auth" ;;
    esac

    # P5
    if [ "$uncovered" != "-" ]; then
        res="$(req "$p" "$uncovered" -H 'Remote-User: angreifer' -H 'Remote-Groups: admins')"
        case "$res" in
            *"USER=[] GROUPS=[]"*) ok "$label P5 gefaelschte Header entfernt" ;;
            *) bad "$label P5 HEADER-SPOOFING beim Backend: $res" ;;
        esac
    fi

    # P6
    res="$(req "$p" "$covered" -H 'X-Allow: yes' -H 'X-Groups: admins' \
              -H 'Remote-User: angreifer' -H 'Remote-Groups: root')"
    case "$res" in
        *"USER=[alice] GROUPS=[admins]"*) ok "$label P6 Auth-Identitaet ueberschreibt Faelschung" ;;
        *) bad "$label P6 IDENTITAETS-SPOOFING: $res" ;;
    esac

    # P8: Auth stimmt zu, meldet aber keine Identitaet. Der Client-Wert darf
    # nicht durchlaufen. Bei konfigurierten Gruppen wird die Anfrage ohnehin
    # abgewiesen (keine Gruppe = keine Berechtigung), deshalb nur ohne Gruppen.
    if [ -z "$groups" ]; then
        res="$(req "$p" "$covered" -H 'X-Allow: bare' -H 'Remote-User: angreifer' -H 'Remote-Groups: admins')"
        case "$res" in
            *"USER=[] GROUPS=[]"*) ok "$label P8 Client-Header ueberlebt eine Auth-Antwort ohne Identitaet nicht" ;;
            *"angreifer"*) bad "$label P8 IDENTITAETS-SPOOFING bei Auth-Antwort ohne Header: $res" ;;
            *) bad "$label P8 unerwartete Antwort: $res" ;;
        esac
    fi

    # P7: die Wurzel eines Auth-Verzeichnisses muss mitgeschuetzt sein. Gilt nur
    # dort, wo der Auth-Pfad ein Verzeichnis ist (/admin/x liegt unter /admin/*).
    case "$covered" in
        */*/*)
            local root="${covered%/*}"
            res="$(req "$p" "$root" -H 'Remote-User: angreifer')"
            case "$res" in
                *"USER=[alice]"*|*"[403]"*|*"[401]"*)
                    ok "$label P7 Wurzel $root ist geschuetzt" ;;
                *"USER=[]"*)
                    bad "$label P7 Wurzel $root OHNE Auth erreichbar (Caddy: /x/* trifft /x nicht): $res" ;;
                *) ok "$label P7 Wurzel $root nicht offen ($res)" ;;
            esac
            ;;
    esac
}

port=9010
for v in "${VARIANTS[@]}"; do
    parse_variant "$v"
    echo; echo "── $v_name (einfache Site)"
    check_variant "$((port+10000))" "$v_name/plain" "$v_groups" "$v_covered" "$v_uncovered"
    echo; echo "── $v_name (Auth in handle @allowed)"
    check_variant "$((port+10001))" "$v_name/allowlist" "$v_groups" "$v_covered" "$v_uncovered"
    port=$((port+2))
done

# ── Bypass ueber nicht passenden Host ────────────────────────────────────────
bypass_scenario() { # bypass_scenario <port> <label>
    local p="$1" label="$2" res
    echo; echo "── $label"
    res="$(curl -s -H 'Host: app.local' -H 'X-Allow: yes' "http://localhost:$p/" 2>/dev/null)"
    if [ "$res" = "GEHEIMER-INHALT" ]; then
        ok "Positivkontrolle: Auth-Server wird erreicht und erlaubt"
    else
        bad "Positivkontrolle fehlgeschlagen (Antwort '${res:-leer}') - Sicherheitsteil nicht aussagekraeftig"
    fi
    res="$(curl -s -H 'Host: app.local' "http://localhost:$p/" 2>/dev/null)"
    if [ "$res" = "GEHEIMER-INHALT" ]; then
        bad "AUTH-BYPASS: geschuetzter Inhalt ohne Authentifizierung"
        echo "     Fix: header_up Host {http.reverse_proxy.upstream.hostport} im Auth-Block."
    else
        ok "geschuetzter Inhalt bleibt verborgen"
    fi
}
bypass_scenario 19001 "Bypass: Auth ueber http://"
bypass_scenario 19002 "Bypass: Auth ueber https:// (PR #7454)"

# ── Zustand der Ursache in Caddy, nur Bericht ────────────────────────────────
echo; echo "── Ursache in Caddy (nur Bericht)"
unmatched="$(docker exec "$CONTAINER" sh -c \
    'wget -qS -O /dev/null --header="Host: gibtsnicht.local" http://127.0.0.1:9000/ 2>&1 | head -1' 2>/dev/null)"
echo "     unbekannter Host am Listener: ${unmatched:-<keine Antwort>}"

echo
echo "════════════════════════════════════════════════════════"
echo " ${GREEN}bestanden: $PASS${NC}   ${RED}fehlgeschlagen: $FAIL${NC}"
echo "════════════════════════════════════════════════════════"
[ $FAIL -eq 0 ] || docker logs "$CONTAINER" 2>&1 | tail -20
exit $((FAIL > 0))
