#!/bin/bash
# Waechter fuer die uebrigen Sicherheitsfunktionen - alles ausser der Auth-Kette,
# die test/auth-guard.sh abdeckt.
#
# Geprueft wird die WIRKUNG an echten, vom Watcher erzeugten Konfigurationen, im
# echten Caddy-Image dieses Repos (base.conf braucht das replace-Plugin), mit
# einem Backend, das zurueckmeldet was bei ihm ankommt. Nur so laesst sich sehen,
# ob etwa der SSO-Cookie tatsaechlich entfernt wird - eine Regex-Pruefung im
# Unittest sagt darueber nichts.
#
# Abgedeckt:
#   S1  SSO-Cookie-Strip: TinyAuths Cookie erreicht das Backend nicht, fremde
#       Cookies bleiben unangetastet
#   S2  (security): Zugriff auf .env, .git, Dev-Dateien wird abgewiesen,
#       /.well-known bleibt erreichbar
#   S3  (header): HSTS und die Anti-Clickjacking-Header sind gesetzt, Server-
#       und X-Powered-By-Header sind entfernt
#   S4  (noindex): X-Robots-Tag verhindert Indexierung, wenn SEO aus ist
#   S5  (wordpress): PHP in Upload-/Plugin-/Theme-Verzeichnissen wird abgewiesen
#   S6  (stealth): beim cloudflare-Typ wird eine Anfrage, die nicht aus dem
#       Cloudflare-Netz kommt, mit 404 abgewiesen statt die Existenz zu verraten
#   S7  Allowlist: dokumentiert, dass private_ranges immer mitgilt
#   S8  Forwarded-Header: vom Client gesetzte X-Forwarded-* und X-Real-IP
#       erreichen das Backend nicht - auch dann nicht, wenn
#       CADDY_TRUSTED_PROXIES gesetzt ist und der Client aus einem privaten
#       Netz kommt. TinyAuth stuetzt seine IP-Regeln auf X-Forwarded-For.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(dirname "$SCRIPT_DIR")"
REPO_DIR="$(dirname "$WATCHER_DIR")"
WORKDIR="$(mktemp -d)"
NET="secfeat-net"
CADDY="secfeat-caddy"
ECHO_BACKEND="secfeat-echo"
IMAGE="secfeat-caddy-image"
GEN_TEST="$WATCHER_DIR/zz_secfeat_gen_test.go"

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'
PASS=0; FAIL=0
info() { echo "${YELLOW}[info]${NC} $1"; }
ok()   { echo "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
bad()  { echo "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL+1)); }

cleanup() {
    docker rm -f "$CADDY" "$ECHO_BACKEND" >/dev/null 2>&1
    docker network rm "$NET" >/dev/null 2>&1
    rm -f "$GEN_TEST"
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

echo "════════════════════════════════════════════════════════"
echo " Sicherheitsfunktionen ausser Auth"
echo "════════════════════════════════════════════════════════"

# ── Konfigurationen vom Watcher erzeugen lassen ──────────────────────────────
cat > "$GEN_TEST" <<'GOEOF'
package main

import (
	"os"
	"testing"
)

// Erzeugt Konfigurationen so, wie sie im Betrieb entstehen.
func TestZZSecFeatGen(t *testing.T) {
	out := os.Getenv("GEN_TO")
	if out == "" {
		t.Skip()
	}
	t.Setenv("COMPOSE_PROJECT_NAME", "test")
	t.Setenv("TINYAUTH_DOMAIN", "auth.example.com")

	mgr := NewCaddyManager(out, NewAllowlistManager(0, nil))
	write := func(c *CaddyConfig) {
		c.Network = "sec_caddy"
		c.DNSProvider = "cloudflare"
		c.Upstream = "secfeat-echo:8080"
		if err := mgr.WriteConfig(c); err != nil {
			t.Fatalf("%s: %v", c.Container, err)
		}
	}

	// Regelfall: Sicherheits-Snippets an, SEO aus, im Cookie-Scope
	write(&CaddyConfig{
		Container: "regular", OwnerContainer: "regular", Type: TypeExternal,
		Domains: []string{"regular.example.com"},
		Compression: true, Header: true, Performance: true, Security: true,
	})
	// WordPress-Regeln
	write(&CaddyConfig{
		Container: "wp", OwnerContainer: "wp", Type: TypeExternal,
		Domains: []string{"wp.example.com"},
		Header: true, Security: true, WordPress: true,
	})
	// Ausserhalb des Cookie-Scopes: hier darf NICHT gestrippt werden
	write(&CaddyConfig{
		Container: "fremd", OwnerContainer: "fremd", Type: TypeExternal,
		Domains: []string{"kunde.de"},
		Header: true, Security: true,
	})
	// cloudflare-Typ: Anfragen ausserhalb des CF-Netzes muessen 404 bekommen
	write(&CaddyConfig{
		Container: "cf", OwnerContainer: "cf", Type: TypeCloudflare,
		Domains: []string{"cf.example.com"}, Header: true,
	})
	// Allowlist mit einer fremden IP - private_ranges gilt trotzdem mit
	write(&CaddyConfig{
		Container: "allow", OwnerContainer: "allow", Type: TypeExternal,
		Domains: []string{"allow.example.com"}, Header: true,
		Allowlist: []string{"203.0.113.7"},
	})
	// CADDY_TRUSTED_PROXIES mit einer fremden IP: der Testclient kommt aus
	// einem privaten Netz und darf trotzdem nichts vortaeuschen.
	write(&CaddyConfig{
		Container: "fwd", OwnerContainer: "fwd", Type: TypeExternal,
		Domains: []string{"fwd.example.com"}, Header: true,
		TrustedProxies: []string{"203.0.113.9"},
	})
}
GOEOF

mkdir -p "$WORKDIR/hosts"
( cd "$WATCHER_DIR" && GEN_TO="$WORKDIR/hosts" go test -run TestZZSecFeatGen . >/dev/null 2>&1 )
rm -f "$GEN_TEST"
if [ -z "$(find "$WORKDIR/hosts" -name '*.conf' 2>/dev/null)" ]; then
    bad "Konfigurationen konnten nicht erzeugt werden"
    exit 1
fi
info "erzeugt: $(find "$WORKDIR/hosts" -name '*.conf' | wc -l | tr -d ' ') Konfigurationen"

# base.conf uebernehmen, aber die TLS-Snippets auf die interne CA umbiegen -
# geprueft werden die Sicherheits-Snippets, nicht die Zertifikatsbeschaffung.
python3 - "$REPO_DIR/hosts/base.conf" "$WORKDIR/hosts/base.conf" <<'PYEOF'
import re, sys
src, dst = sys.argv[1], sys.argv[2]
s = open(src).read()
for name in ("tls-cloudflare", "tls-hetzner"):
    s = re.sub(r"\(%s\) \{.*?\n\}" % re.escape(name),
               "(%s) {\n    tls internal\n}" % name, s, flags=re.S)
open(dst, "w").write(s)
PYEOF
cp "$REPO_DIR/build/Caddyfile" "$WORKDIR/hosts/Caddyfile"

info "Caddy-Image bauen (enthaelt das replace-Plugin aus base.conf)"
docker build -q -t "$IMAGE" "$REPO_DIR/build" >/dev/null 2>&1 || { bad "Image-Build fehlgeschlagen"; exit 1; }

docker network create "$NET" >/dev/null 2>&1
docker rm -f "$CADDY" "$ECHO_BACKEND" >/dev/null 2>&1
docker run -d --name "$ECHO_BACKEND" --network "$NET" --network-alias secfeat-echo \
    mendhak/http-https-echo:31 >/dev/null 2>&1
docker run -d --name "$CADDY" --network "$NET" \
    -v "$WORKDIR/hosts:/hosts:ro" \
    -e COMPOSE_PROJECT_NAME=test -e EMAIL=a@b.de -e LOG_LEVEL=ERROR \
    -p 18443:443 "$IMAGE" \
    caddy run --config /hosts/Caddyfile --adapter caddyfile >/dev/null 2>&1
sleep 10
if ! docker ps --filter "name=$CADDY" --format '{{.ID}}' | grep -q .; then
    bad "Caddy startet nicht"
    docker logs "$CADDY" 2>&1 | tail -20
    exit 1
fi

# get <host> <pfad> [curl-args...] -> Body
get() { local h="$1" path="$2"; shift 2; curl -sk --path-as-is --resolve "$h:18443:127.0.0.1" "$@" "https://$h:18443$path" 2>/dev/null; }
code() { local h="$1" path="$2"; shift 2; curl -sk --path-as-is --resolve "$h:18443:127.0.0.1" -o /dev/null -w '%{http_code}' "$@" "https://$h:18443$path" 2>/dev/null; }
hdrs() { local h="$1" path="$2"; shift 2; curl -sk --path-as-is --resolve "$h:18443:127.0.0.1" -D - -o /dev/null "$@" "https://$h:18443$path" 2>/dev/null; }

# ── S1 SSO-Cookie-Strip ──────────────────────────────────────────────────────
echo; echo "── S1 SSO-Cookie erreicht das Backend nicht"
COOKIES='tinyauth-session-8b2f=geheim; sid=behalten; tinyauth-csrf-1a2b=auch-geheim; theme=dark'
body="$(get regular.example.com /x -H "Cookie: $COOKIES")"
recv="$(printf '%s' "$body" | python3 -c "import json,sys; print(json.load(sys.stdin).get('headers',{}).get('cookie',''))" 2>/dev/null)"
echo "     Backend sah: ${recv:-<nichts>}"
case "$recv" in
    *tinyauth*) bad "S1 TinyAuth-Cookie erreicht das Backend: $recv" ;;
    *) ok "S1 kein tinyauth-Cookie beim Backend" ;;
esac
case "$recv" in
    *"sid=behalten"*) ok "S1 fremde Cookies bleiben erhalten" ;;
    *) bad "S1 fremde Cookies verloren - der Strip greift zu weit: ${recv:-<nichts>}" ;;
esac
case "$recv" in
    *"theme=dark"*) ok "S1 auch das letzte Cookie bleibt erhalten" ;;
    *) bad "S1 letztes Cookie verloren: ${recv:-<nichts>}" ;;
esac

echo; echo "── S1b ausserhalb des Cookie-Scopes wird nicht gestrippt"
body="$(get kunde.de /x -H "Cookie: $COOKIES")"
recv="$(printf '%s' "$body" | python3 -c "import json,sys; print(json.load(sys.stdin).get('headers',{}).get('cookie',''))" 2>/dev/null)"
case "$recv" in
    *tinyauth*) ok "S1b fremde Domain unveraendert durchgelassen (Cookie kann dort nie ankommen)" ;;
    *) bad "S1b unerwartet gestrippt: ${recv:-<nichts>}" ;;
esac

# ── S2 (security) ────────────────────────────────────────────────────────────
echo; echo "── S2 sensible Dateien werden abgewiesen"
for path in /.env /.git/config /composer.json /package.json /backup.sql /dump.bak /.hidden; do
    c="$(code regular.example.com "$path")"
    if [ "$c" = "403" ]; then ok "S2 $path -> 403"; else bad "S2 $path -> $c (erwartet 403)"; fi
done
c="$(code regular.example.com /.well-known/acme-challenge/token)"
if [ "$c" != "403" ]; then ok "S2 /.well-known bleibt erreichbar ($c)"; else bad "S2 /.well-known wird blockiert - ACME waere kaputt"; fi

# ── S3 (header) ──────────────────────────────────────────────────────────────
echo; echo "── S3 Sicherheits-Header"
h="$(hdrs regular.example.com /x)"
check_header() { # check_header <regex> <beschreibung>
    if printf '%s' "$h" | grep -qi "$1"; then ok "S3 $2"; else bad "S3 $2 fehlt"; fi
}
check_header '^strict-transport-security:.*max-age=31536000' "HSTS mit max-age"
check_header '^x-content-type-options: *nosniff' "X-Content-Type-Options"
check_header '^x-frame-options: *SAMEORIGIN' "X-Frame-Options"
check_header '^referrer-policy:' "Referrer-Policy"
if printf '%s' "$h" | grep -qiE '^(server|x-powered-by):'; then
    bad "S3 Server/X-Powered-By wird nicht entfernt"
else
    ok "S3 Server und X-Powered-By entfernt"
fi

# ── S4 (noindex) ─────────────────────────────────────────────────────────────
echo; echo "── S4 Indexierung unterbunden, solange SEO aus ist"
if printf '%s' "$h" | grep -qi '^x-robots-tag:'; then
    ok "S4 X-Robots-Tag gesetzt"
else
    bad "S4 X-Robots-Tag fehlt - die Site waere indexierbar"
fi

# ── S5 (wordpress) ───────────────────────────────────────────────────────────
echo; echo "── S5 WordPress-Regeln"
# Der Datumsordner ist der realistische Fall: dorthin schreibt WordPress
# Uploads. Caddys Wildcard ueberschreitet keine Verzeichnisgrenze, eine Regel
# mit "**.php" traf ihn deshalb nicht.
for path in /wp-content/uploads/evil.php /wp-content/uploads/2026/07/shell.php \
            /wp-content/plugins/x/evil.php /wp-content/themes/x/evil.php \
            /wp-content/plugins/tief/verschachtelt/x.php \
            /wp-config.php /wp-admin/install.php; do
    c="$(code wp.example.com "$path")"
    if [ "$c" = "403" ]; then ok "S5 $path -> 403"; else bad "S5 $path -> $c (erwartet 403)"; fi
done
# wp-tinymce.php laedt der klassische Editor - eine pauschale Regel fuer
# wp-includes haette sie mit blockiert.
for path in /wp-content/uploads/bild.jpg /wp-content/uploads/2026/07/bild.jpg \
            /wp-content/themes/x/style.css /wp-includes/js/tinymce/wp-tinymce.php; do
    c="$(code wp.example.com "$path")"
    if [ "$c" != "403" ]; then ok "S5 $path bleibt erreichbar ($c)"; else bad "S5 $path faelschlich blockiert"; fi
done

# ── S6 (stealth) beim cloudflare-Typ ─────────────────────────────────────────
echo; echo "── S6 cloudflare-Typ: Anfrage ausserhalb des CF-Netzes"
c="$(code cf.example.com /x)"
if [ "$c" = "404" ]; then
    ok "S6 404 statt Hinweis auf die Existenz der Site"
else
    bad "S6 erwartet 404, bekommen $c - Anfragen von ausserhalb erreichen das Backend"
fi

# ── S7 Allowlist: private_ranges gilt immer mit ──────────────────────────────
echo; echo "── S7 Allowlist"
c="$(code allow.example.com /x)"
if [ "$c" = "200" ]; then
    info "S7 Zugriff aus einem privaten Netz ist erlaubt, obwohl die Allowlist nur 203.0.113.7 nennt."
    info "    private_ranges steht immer im Matcher - dokumentiertes Verhalten, aber eine"
    info "    Allowlist beschraenkt damit nicht auf die genannten Adressen."
    ok "S7 Verhalten wie erwartet ($c)"
else
    bad "S7 unerwartet: $c - hat sich das Allowlist-Verhalten geaendert?"
fi

# ── S8 Forwarded-Header ──────────────────────────────────────────────────────
echo; echo "── S8 gefaelschte Forwarded-Header erreichen das Backend nicht"
forwarded_check() { # forwarded_check <host> <label>
    local h="$1" label="$2" json
    json="$(get "$h" /x \
        -H 'X-Forwarded-For: 1.2.3.4' \
        -H 'X-Forwarded-Host: evil.example.com' \
        -H 'X-Forwarded-Proto: http' \
        -H 'X-Real-IP: 9.9.9.9')"
    local vals
    vals="$(printf '%s' "$json" | python3 -c "
import json,sys
try: h=json.load(sys.stdin).get('headers',{})
except Exception: print('PARSE-FEHLER'); sys.exit()
print('|'.join(h.get(k,'') for k in ('x-forwarded-for','x-forwarded-host','x-forwarded-proto','x-real-ip')))
" 2>/dev/null)"
    echo "     $label: $vals"
    case "$vals" in
        PARSE-FEHLER|"") bad "S8 $label keine verwertbare Antwort" ; return ;;
    esac
    case "$vals" in
        *1.2.3.4*) bad "S8 $label gefaelschtes X-Forwarded-For erreicht das Backend" ;;
        *) ok "S8 $label X-Forwarded-For bereinigt" ;;
    esac
    case "$vals" in
        *evil.example.com*) bad "S8 $label gefaelschtes X-Forwarded-Host erreicht das Backend" ;;
        *) ok "S8 $label X-Forwarded-Host bereinigt" ;;
    esac
    case "$vals" in
        *"|http|"*) bad "S8 $label gefaelschtes X-Forwarded-Proto erreicht das Backend" ;;
        *) ok "S8 $label X-Forwarded-Proto bereinigt" ;;
    esac
    case "$vals" in
        *9.9.9.9*) bad "S8 $label gefaelschtes X-Real-IP erreicht das Backend" ;;
        *) ok "S8 $label X-Real-IP bereinigt" ;;
    esac
}
forwarded_check regular.example.com "ohne CADDY_TRUSTED_PROXIES"
# Mit gesetztem CADDY_TRUSTED_PROXIES darf ein Client aus einem privaten Netz
# nicht als vertrauenswuerdiger Proxy gelten - sonst duerfte jeder Container
# Identitaetsangaben vortaeuschen.
forwarded_check fwd.example.com "mit CADDY_TRUSTED_PROXIES"

echo
echo "════════════════════════════════════════════════════════"
echo " ${GREEN}bestanden: $PASS${NC}   ${RED}fehlgeschlagen: $FAIL${NC}"
echo "════════════════════════════════════════════════════════"
[ $FAIL -eq 0 ] || docker logs "$CADDY" 2>&1 | tail -20
exit $((FAIL > 0))
