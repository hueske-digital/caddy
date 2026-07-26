package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// baseEnv liefert die Pflichtvariablen, damit die Tests nur die jeweils
// gepruefte Variable variieren muessen.
func baseEnv(extra map[string]string) map[string]string {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "80",
	}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

// TestParseCaddyEnv_RejectsCaddyfileInjection deckt den Kern von K1 ab: ein
// Container darf ueber keine ENV-Variable Caddy-Direktiven einschleusen koennen.
func TestParseCaddyEnv_RejectsCaddyfileInjection(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"auth path closes site block", map[string]string{
			"CADDY_AUTH":       "true",
			"CADDY_AUTH_PATHS": "/x\n    }\n    respond \"pwned\" 200",
		}},
		{"auth except closes site block", map[string]string{
			"CADDY_AUTH":        "true",
			"CADDY_AUTH_EXCEPT": "/x\n}\nhttps://victim.example.com {",
		}},
		{"auth path without leading slash", map[string]string{
			"CADDY_AUTH":       "true",
			"CADDY_AUTH_PATHS": "admin",
		}},
		{"auth path with brace", map[string]string{
			"CADDY_AUTH":       "true",
			"CADDY_AUTH_PATHS": "/admin{",
		}},
		{"group with newline", map[string]string{
			"CADDY_AUTH":        "true",
			"CADDY_AUTH_GROUPS": "admins\n    respond 200",
		}},
		{"noindex type with directive", map[string]string{
			"CADDY_SEO":               "true",
			"CADDY_SEO_NOINDEX_TYPES": "pdf\n    respond 200",
		}},
		{"allowlist with markup", map[string]string{
			"CADDY_ALLOWLIST": "<img src=x onerror=alert(1)>",
		}},
		{"trusted proxies with directive", map[string]string{
			"CADDY_TRUSTED_PROXIES": "1.2.3.4\n    respond 200",
		}},
		{"auth url with userinfo", map[string]string{
			"CADDY_AUTH":     "true",
			"CADDY_AUTH_URL": "https://user:pass@login.example.com",
		}},
		{"auth url with foreign scheme", map[string]string{
			"CADDY_AUTH":     "true",
			"CADDY_AUTH_URL": "file:///etc/passwd",
		}},
		{"unknown dns provider", map[string]string{
			"CADDY_DNS_PROVIDER": "foo",
		}},
		// Caddys Lexer trennt Tokens an unicode.IsSpace: ein NBSP erzeugt aus
		// einem scheinbar einzelnen Pfad zwei Matcher, der zweite (/*) nimmt
		// die ganze Site von der Auth aus.
		{"auth except with non-breaking space", map[string]string{
			"CADDY_AUTH":        "true",
			"CADDY_AUTH_EXCEPT": "/health /*",
		}},
		{"auth path with ideographic space", map[string]string{
			"CADDY_AUTH":       "true",
			"CADDY_AUTH_PATHS": "/admin　/*",
		}},
		{"group with non-breaking space", map[string]string{
			"CADDY_AUTH":        "true",
			"CADDY_AUTH_GROUPS": "admins x",
		}},
		{"allowlist with non-breaking space", map[string]string{
			"CADDY_ALLOWLIST": "1.2.3.4 5.6.7.8",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseCaddyEnv(baseEnv(tc.env), "test_caddy", "test-container")
			if err == nil {
				t.Fatalf("expected rejection, got config %+v", cfg)
			}
		})
	}
}

// TestParseCaddyEnv_PortRange deckt K3 ab: ein Port ausserhalb 1-65535 erzeugt
// eine Konfiguration, die "caddy validate" ablehnt - und blockiert damit den
// Reload aller Sites, nicht nur der fehlerhaften.
func TestParseCaddyEnv_PortRange(t *testing.T) {
	for _, port := range []string{"0", "-1", "65536", "99999", "abc", ""} {
		env := baseEnv(map[string]string{"CADDY_PORT": port})
		if port == "" {
			delete(env, "CADDY_PORT")
		}
		if _, err := ParseCaddyEnv(env, "test_caddy", "test-container"); err == nil {
			t.Errorf("port %q: expected rejection", port)
		}
	}
	for _, port := range []string{"1", "80", "8080", "65535"} {
		env := baseEnv(map[string]string{"CADDY_PORT": port})
		if _, err := ParseCaddyEnv(env, "test_caddy", "test-container"); err != nil {
			t.Errorf("port %q: unexpected rejection: %v", port, err)
		}
	}
}

// TestParseCaddyEnv_AcceptsValidValues stellt sicher, dass die Validierung die
// dokumentierten Schreibweisen nicht mit abraeumt.
func TestParseCaddyEnv_AcceptsValidValues(t *testing.T) {
	env := baseEnv(map[string]string{
		"CADDY_AUTH":       "true",
		"CADDY_AUTH_PATHS": "/admin/*, /dashboard/*, /api/private/*",
		// OIDC-Gruppen enthalten oft ":" oder "+"; regexp.QuoteMeta sichert sie
		// ab, sie duerfen deshalb nicht abgelehnt werden.
		"CADDY_AUTH_GROUPS":       "admins, dev-team, org.users, realm:admins, ops+prod",
		"CADDY_AUTH_URL":          "https://login.example.com",
		"CADDY_ALLOWLIST":         "1.2.3.4, home.dyndns.org, 10.0.0.0/8, 2001:db8::1",
		"CADDY_TRUSTED_PROXIES":   "192.168.1.10, my-proxy.example.com",
		"CADDY_SEO":               "true",
		"CADDY_SEO_NOINDEX_TYPES": "pdf, .doc, docx",
		"CADDY_DNS_PROVIDER":      "hetzner",
	})

	if _, err := ParseCaddyEnv(env, "test_caddy", "test-container"); err != nil {
		t.Fatalf("unexpected rejection of valid config: %v", err)
	}

	// Auth-Upstreams: nur https, aber dort alle gaengigen Schreibweisen -
	// bracketed IPv6 und Containernamen mit Unterstrich, die Docker erlaubt und
	// oeffentliches DNS nicht. (Ein Zone-Index muesste in URL-Form "%25eth0"
	// geschrieben werden; als Auth-Server praktisch irrelevant.)
	for _, authURL := range []string{
		"https://[2001:db8::1]",
		"https://[2001:db8::1]:3000",
		"https://tinyauth:3000",
		"https://auth_service:3000",
		"https://auth.example.com:8080",
	} {
		env := baseEnv(map[string]string{"CADDY_AUTH": "true", "CADDY_AUTH_URL": authURL})
		if _, err := ParseCaddyEnv(env, "test_caddy", "test-container"); err != nil {
			t.Errorf("auth URL %q: unexpected rejection: %v", authURL, err)
		}
	}
}

// TestParseCaddyEnv_AuthURLProducesValidUpstream: Werte, die Caddy als
// Upstream-Adresse ablehnen wuerde, muessen hier scheitern - sonst landen sie
// in /hosts und blockieren den Reload aller Sites.
func TestParseCaddyEnv_AuthURLProducesValidUpstream(t *testing.T) {
	for _, authURL := range []string{
		"https://auth.example.com/",     // Pfad ist in Upstream-Adressen verboten
		"https://auth.example.com/auth", // dito
		"https://2001:db8::1",           // IPv6 ohne Klammern
		"https://auth.example.com?x=1",
		"https://auth.example.com:0",
		// Klartext ist unzulaessig: die Antwort entscheidet ueber den Zugang
		// und traegt Remote-User/-Groups. Bei Upstreams ohne TLS gibt Caddy
		// zudem den Client-Host weiter, was einen Auth-Bypass ermoeglicht.
		"http://auth.example.com",
		"http://auth.example.com:8080",
		"tinyauth:3000",    // schemalos = Klartext-HTTP
		"auth.example.com", // dito
	} {
		env := baseEnv(map[string]string{"CADDY_AUTH": "true", "CADDY_AUTH_URL": authURL})
		if _, err := ParseCaddyEnv(env, "test_caddy", "test-container"); err == nil {
			t.Errorf("auth URL %q: expected rejection", authURL)
		}
	}
}

// TestValidateAuthURL_HTTPSOnly haelt die aeussere Verteidigungslinie fest.
//
// Bei Auth-Upstreams ohne TLS gibt Caddy den Host des Clients weiter (PR #7454
// deckt nur TLS ab). Ist der Auth-Server selbst ein Caddy, passt die Anfrage
// dort zu keinem Site-Block und die Antwort ist "200, 0 Bytes" - was
// forward_auth als "authentifiziert" liest. Die innere Linie
// (header_up Host im erzeugten Block) prueft test/auth-bypass.sh.
func TestValidateAuthURL_HTTPSOnly(t *testing.T) {
	for _, raw := range []string{
		"http://auth.example.com",
		"http://auth.example.com:8080",
		"tinyauth:3000",
		"auth.example.com",
		"//auth.example.com",
		"HTTP://auth.example.com",
	} {
		if err := validateAuthURL(raw); err == nil {
			t.Errorf("%q: expected rejection (plaintext auth upstream)", raw)
		}
	}
	for _, raw := range []string{
		"https://auth.example.com",
		"https://auth.example.com:8443",
		"https://tinyauth:3000",
	} {
		if err := validateAuthURL(raw); err != nil {
			t.Errorf("%q: unexpected rejection: %v", raw, err)
		}
	}
}

// TestValidateServiceName_AllowsLeadingUnderscore: "_" ist ein uebliches
// Praefix und technisch unbedenklich, nur "-" bleibt gesperrt.
func TestValidateServiceName_AllowsLeadingUnderscore(t *testing.T) {
	if err := validateServiceName("_admin"); err != nil {
		t.Errorf("unexpected rejection of '_admin': %v", err)
	}
	if err := validateServiceName("-admin"); err == nil {
		t.Error("expected rejection of leading dash")
	}
}

// TestParseAllCaddyEnv_RejectsTraversalServiceName deckt K2 ab: das Suffix aus
// CADDY_DOMAIN_<name> wird Teil des Dateinamens.
func TestParseAllCaddyEnv_RejectsTraversalServiceName(t *testing.T) {
	for _, name := range []string{
		"x/../../internal/victim",
		"../evil",
		"a/b",
		`a\b`,
		"-leading-dash",
	} {
		env := map[string]string{
			"CADDY_DOMAIN_" + name: "evil.example.com",
			"CADDY_TYPE_" + name:   "external",
			"CADDY_PORT_" + name:   "80",
		}
		if _, err := ParseAllCaddyEnv(env, "attack_caddy", "/web"); err == nil {
			t.Errorf("service name %q: expected rejection", name)
		}
	}
}

// TestWriteConfig_RejectsTraversalPath ist die zweite Verteidigungslinie: auch
// wenn ein Container-/Servicename die Parser-Pruefung umgehen wuerde, darf
// WriteConfig nicht ausserhalb des Typverzeichnisses schreiben.
func TestWriteConfig_RejectsTraversalPath(t *testing.T) {
	base := t.TempDir()
	victimDir := filepath.Join(base, "internal")
	if err := os.MkdirAll(victimDir, 0755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(victimDir, "victim_other_caddy.conf")
	if err := os.WriteFile(victim, []byte("ORIGINAL\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewCaddyManager(base, nil)
	cfg := &CaddyConfig{
		Network:     "attack_caddy",
		Container:   "web-x/../../internal/victim_other",
		Type:        TypeExternal,
		Domains:     []string{"evil.example.com"},
		Upstream:    "web:80",
		DNSProvider: "cloudflare",
	}

	if err := mgr.WriteConfig(cfg); err == nil {
		t.Fatal("expected WriteConfig to reject traversing config key")
	}

	content, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("victim config disappeared: %v", err)
	}
	if string(content) != "ORIGINAL\n" {
		t.Fatalf("victim config was overwritten: %q", content)
	}
}

// TestWriteWildcardConfigs_RejectsInvalidDomain: WILDCARD_DOMAINS geht ebenfalls
// in einen Dateinamen und in das Caddyfile.
func TestWriteWildcardConfigs_RejectsInvalidDomain(t *testing.T) {
	mgr := NewCaddyManager(t.TempDir(), nil)

	if err := mgr.WriteWildcardConfigs([]string{"../../etc/evil"}, "cloudflare"); err == nil {
		t.Error("expected rejection of traversing wildcard domain")
	}
	if err := mgr.WriteWildcardConfigs([]string{"example.com"}, "foo"); err == nil {
		t.Error("expected rejection of unknown wildcard DNS provider")
	}
	// "http" ist fuer normale Sites gueltig, fuer Wildcards nicht: das Template
	// importiert immer tls-<provider>, ein Wildcard braucht die DNS-Challenge.
	if err := mgr.WriteWildcardConfigs([]string{"example.com"}, "http"); err == nil {
		t.Error("expected rejection of 'http' as wildcard DNS provider")
	}
	if err := mgr.WriteWildcardConfigs([]string{"example.com"}, "hetzner"); err != nil {
		t.Errorf("unexpected rejection of valid wildcard config: %v", err)
	}
}

// TestGenerateAuthBlock_QuotesGroupRegex sichert die Stelle ab, an der
// Regex-Metazeichen in Gruppennamen unschaedlich gemacht werden. Die
// Zeichen-Validierung laesst sie bewusst durch, weil OIDC-Gruppen Zeichen wie
// ":" oder "+" enthalten - deshalb muss das Quoting hier halten.
func TestGenerateAuthBlock_QuotesGroupRegex(t *testing.T) {
	block := generateAuthBlock("", nil, nil, []string{"a.b", "c*"})
	if strings.Contains(block, "(a.b|c*)") {
		t.Errorf("group names were not quoted: %s", block)
	}
	if !strings.Contains(block, `a\.b`) {
		t.Errorf("expected quoted group name in: %s", block)
	}

	// ".*" als Gruppenname darf keine beliebige Gruppe zulassen.
	wildcard := generateAuthBlock("", nil, nil, []string{".*"})
	if !strings.Contains(wildcard, `\.\*`) {
		t.Errorf("wildcard group name was not quoted: %s", wildcard)
	}
}
