package main

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// caddyfileStructuralChars tragen im Caddyfile Struktur: "}" schliesst den
// Site-Block, "#" kommentiert den Rest der Zeile aus, Quotes beenden ein Token.
const caddyfileStructuralChars = "{}\"'`#\\"

// containsCaddyfileMeta lehnt alles ab, was nicht druckbares ASCII ohne
// Strukturzeichen ist.
//
// Die Positivliste ist Absicht. Caddys Lexer trennt Tokens an unicode.IsSpace,
// nicht nur am ASCII-Leerzeichen - eine Sperrliste einzelner Zeichen liesse
// deshalb ein NBSP durch, und aus "/health /*" wuerden zwei Pfad-Matcher
// statt einem. Der zweite (/*) naehme die komplette Site von der Auth aus,
// obwohl der Wert wie ein einzelner Pfad aussieht.
func containsCaddyfileMeta(s string) bool {
	for _, r := range s {
		if r < '!' || r > '~' {
			return true
		}
		if strings.ContainsRune(caddyfileStructuralChars, r) {
			return true
		}
	}
	return false
}

// validDNSProviders sind die Provider, fuer die hosts/base.conf ein
// (tls-<provider>)-Snippet bereitstellt. "http" bedeutet: kein TLS-Import.
var validDNSProviders = []string{"cloudflare", "hetzner", "http"}

// validWildcardDNSProviders schliesst "http" aus: das Wildcard-Template
// importiert immer tls-<provider>, ein Wildcard-Zertifikat braucht zwingend
// eine DNS-Challenge. "http" ergaebe "import tls-http" und damit ein
// ungueltiges Caddyfile, das den globalen Reload blockiert.
var validWildcardDNSProviders = []string{"cloudflare", "hetzner"}

// validatePort prueft einen TCP-Port. Ein bloss numerischer Wert genuegt nicht:
// "0" oder "-1" erzeugen eine Caddy-Konfiguration, die "caddy validate" ablehnt,
// woraufhin der Reload-Watcher gar keine Konfiguration mehr laedt - fuer alle
// Sites, nicht nur fuer die fehlerhafte.
func validatePort(port string) error {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid port: %q (must be 1-65535)", port)
	}
	return nil
}

// validateDNSProvider haelt CADDY_DNS_PROVIDER auf den Providern, fuer die ein
// Snippet existiert. Ohne die Pruefung erzeugt ein Tippfehler "import tls-foo"
// und legt damit den globalen Reload lahm.
func validateDNSProvider(provider string) error {
	return validateProviderAgainst(provider, validDNSProviders)
}

// validateWildcardDNSProvider prueft WILDCARD_DNS_PROVIDER.
func validateWildcardDNSProvider(provider string) error {
	return validateProviderAgainst(provider, validWildcardDNSProviders)
}

func validateProviderAgainst(provider string, allowed []string) error {
	for _, p := range allowed {
		if provider == p {
			return nil
		}
	}
	return fmt.Errorf("invalid DNS provider: %q (must be %s)", provider, strings.Join(allowed, "|"))
}

// validateServiceName begrenzt das Suffix aus CADDY_DOMAIN_<name>. Der Name
// wird Teil des Konfigurationsdateinamens; ohne diese Grammatik koennte ein
// Suffix wie "x/../../internal/victim" in ein fremdes Typverzeichnis schreiben.
func validateServiceName(name string) error {
	if len(name) == 0 || len(name) > 63 {
		return fmt.Errorf("invalid service name: %q (1-63 characters)", name)
	}
	// Ein fuehrender Bindestrich bleibt gesperrt, damit kein Dateiname
	// entsteht, den Werkzeuge als Option lesen; "_" ist als Praefix ueblich
	// und unbedenklich.
	if name[0] == '-' {
		return fmt.Errorf("invalid service name: %q (must not start with '-')", name)
	}
	for _, c := range name {
		alnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if alnum || c == '_' || c == '-' {
			continue
		}
		return fmt.Errorf("invalid service name: %q (only letters, digits, '_' and '-')", name)
	}
	return nil
}

// validatePathMatcher prueft einen Eintrag aus CADDY_AUTH_PATHS bzw.
// CADDY_AUTH_EXCEPT. Die Werte landen als Token einer path-Matcher-Liste im
// Caddyfile, sind dort also durch Leerzeichen getrennt - ein Leerzeichen im
// Wert erzeugt daher zusaetzliche Tokens und wird mit abgelehnt.
func validatePathMatcher(path string) error {
	if path == "" || path[0] != '/' {
		return fmt.Errorf("invalid auth path: %q (must start with '/')", path)
	}
	if containsCaddyfileMeta(path) {
		return fmt.Errorf("invalid auth path: %q (contains illegal characters)", path)
	}
	return nil
}

// validateGroup prueft einen Eintrag aus CADDY_AUTH_GROUPS.
//
// Regex-Metazeichen muessen hier nicht gesperrt werden - generateAuthBlock
// fuehrt die Namen durch regexp.QuoteMeta, ".*" waere also bereits wirkungslos.
// Uebrig bleibt die Caddyfile-Tokenisierung: OIDC-Gruppen wie "realm:admins"
// oder "ops+prod" sind gaengig und bleiben deshalb zulaessig.
func validateGroup(group string) error {
	if group == "" || len(group) > 128 {
		return fmt.Errorf("invalid auth group: %q (1-128 characters)", group)
	}
	if containsCaddyfileMeta(group) {
		return fmt.Errorf("invalid auth group: %q (contains illegal characters)", group)
	}
	return nil
}

// validateFileExtension prueft einen Eintrag aus CADDY_SEO_NOINDEX_TYPES. Der
// Wert wird zu "*.<ext>" in einem path-Matcher; ein fuehrender Punkt ist
// erlaubt, weil die Dokumentation beide Schreibweisen zulaesst.
func validateFileExtension(ext string) error {
	ext = strings.TrimPrefix(ext, ".")
	if len(ext) == 0 || len(ext) > 16 {
		return fmt.Errorf("invalid file extension: %q (1-16 characters)", ext)
	}
	for _, c := range ext {
		alnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !alnum {
			return fmt.Errorf("invalid file extension: %q (letters and digits only)", ext)
		}
	}
	return nil
}

// validateAuthURL prueft CADDY_AUTH_URL. Der Wert wird zum Upstream eines
// forward_auth, ein Container kann Caddy damit zu Requests an beliebige
// erreichbare Hosts bewegen. Erlaubt sind daher nur http/https bzw. host:port,
// ohne Userinfo, Pfad, Query oder Fragment.
func validateAuthURL(raw string) error {
	if containsCaddyfileMeta(raw) {
		return fmt.Errorf("invalid auth URL: %q (contains illegal characters)", raw)
	}

	if !strings.Contains(raw, "://") {
		return validateHostPort(raw)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid auth URL: %q (%v)", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid auth URL: %q (scheme must be http or https)", raw)
	}
	if u.User != nil {
		return fmt.Errorf("invalid auth URL: %q (userinfo not allowed)", raw)
	}
	// Caddy verbietet Pfade in Upstream-Adressen - auch der abschliessende "/"
	// aus "https://auth.example.com/" ist einer und wuerde eine ungueltige
	// Konfiguration erzeugen.
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid auth URL: %q (no path, query or fragment allowed; drop any trailing '/')", raw)
	}
	return validateHostPort(u.Host)
}

// validateHostPort prueft "host" oder "host:port". Der Host darf ein einzelnes
// Label sein, weil Upstreams im Docker-Netz per Containername adressiert werden.
func validateHostPort(hostport string) error {
	host := hostport
	bracketed := strings.HasPrefix(hostport, "[")

	if h, p, err := net.SplitHostPort(hostport); err == nil {
		host = h
		if err := validatePort(p); err != nil {
			return err
		}
	} else if bracketed && strings.HasSuffix(host, "]") {
		// Portlose IPv6-Literale wie "[2001:db8::1]": SplitHostPort meldet hier
		// "missing port", die Klammern gehoeren aber trotzdem entfernt.
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if host == "" {
		return fmt.Errorf("invalid host: %q (empty)", hostport)
	}

	// Zone-ID einer Link-Local-Adresse abtrennen (fe80::1%eth0).
	addr := host
	if i := strings.IndexByte(addr, '%'); i >= 0 {
		addr = addr[:i]
	}

	if ip := net.ParseIP(addr); ip != nil {
		// Caddy erwartet IPv6 in eckigen Klammern. Unbeklammert entstuende eine
		// Adresse, die "caddy validate" ablehnt - und damit den Reload aller
		// Sites blockiert.
		if strings.Contains(addr, ":") && !bracketed {
			return fmt.Errorf("invalid host: %q (IPv6 must be bracketed, e.g. [%s])", hostport, addr)
		}
		return nil
	}
	if addr != host {
		return fmt.Errorf("invalid host: %q (zone identifiers are only valid for IP addresses)", hostport)
	}
	if !isValidUpstreamHost(host) {
		return fmt.Errorf("invalid host: %q", hostport)
	}
	return nil
}

// validateNetworkTarget prueft einen Eintrag aus CADDY_ALLOWLIST oder
// CADDY_TRUSTED_PROXIES. Hostnamen bleiben zulaessig, weil sie per DNS
// aufgeloest werden; IPs und CIDRs gehen direkt in einen remote_ip-Matcher.
func validateNetworkTarget(entry string) error {
	if containsCaddyfileMeta(entry) {
		return fmt.Errorf("invalid allowlist entry: %q (contains illegal characters)", entry)
	}
	if net.ParseIP(entry) != nil {
		return nil
	}
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return nil
	}
	// Unterstriche sind hier zugelassen: der Eintrag wird nur zum Aufloesen
	// benutzt, in die Konfiguration wandert allein die daraus gewonnene IP.
	// Manche DynDNS-Namen enthalten sie, und ein abgelehnter Eintrag wuerde die
	// gesamte Config des Containers ungueltig machen.
	if !isValidUpstreamHost(entry) {
		return fmt.Errorf("invalid allowlist entry: %q (not an IP, CIDR or hostname)", entry)
	}
	return nil
}

// validateCaddyConfig prueft alle aus Container-ENV stammenden Felder, bevor
// sie in ein Caddyfile interpoliert werden. Sie laeuft fuer Single- und
// Multi-Service-Modus gleichermassen, damit beide Pfade nicht auseinanderlaufen.
func validateCaddyConfig(cfg *CaddyConfig) error {
	if err := validateConfigFileName(cfg.ConfigKey() + ".conf"); err != nil {
		return err
	}
	if err := validateDNSProvider(cfg.DNSProvider); err != nil {
		return err
	}
	for _, d := range cfg.Domains {
		if !isValidDomain(d) {
			return fmt.Errorf("invalid domain: %s", d)
		}
	}
	if cfg.AuthURL != "" {
		if err := validateAuthURL(cfg.AuthURL); err != nil {
			return err
		}
	}
	for _, p := range cfg.AuthPaths {
		if err := validatePathMatcher(p); err != nil {
			return err
		}
	}
	for _, p := range cfg.AuthExcept {
		if err := validatePathMatcher(p); err != nil {
			return err
		}
	}
	for _, g := range cfg.AuthGroups {
		if err := validateGroup(g); err != nil {
			return err
		}
	}
	for _, e := range cfg.SEONoindexTypes {
		if err := validateFileExtension(e); err != nil {
			return err
		}
	}
	for _, e := range cfg.Allowlist {
		if err := validateNetworkTarget(e); err != nil {
			return err
		}
	}
	for _, e := range cfg.TrustedProxies {
		if err := validateNetworkTarget(e); err != nil {
			return err
		}
	}
	return nil
}

// validateConfigFileName stellt sicher, dass ein Dateiname wirklich nur ein
// Name ist. filepath.Join loest "../" auf, statt es abzuweisen - ein Name mit
// Separator wuerde also stillschweigend in ein anderes Verzeichnis schreiben
// und dort eine fremde Konfiguration ueberschreiben.
func validateConfigFileName(name string) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || name != filepath.Base(name) {
		return fmt.Errorf("unsafe config file name: %q", name)
	}
	return nil
}

// isValidHostname prueft die Label-Grammatik eines Hostnamens nach DNS-Regeln.
// Anders als isValidDomain verlangt sie keinen Punkt.
func isValidHostname(host string) bool {
	return validHostLabels(host, false)
}

// isValidUpstreamHost gilt fuer Adressen, die innerhalb des Docker-Netzes
// aufgeloest werden. Dort sind Unterstriche in Container- und Servicenamen
// erlaubt, auch wenn oeffentliches DNS sie nicht kennt.
func isValidUpstreamHost(host string) bool {
	return validHostLabels(host, true)
}

func validHostLabels(host string, allowUnderscore bool) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, c := range label {
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-':
			case allowUnderscore && c == '_':
			default:
				return false
			}
		}
	}
	return true
}
