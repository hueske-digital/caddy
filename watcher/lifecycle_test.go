package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestConfig(t *testing.T, mgr *CaddyManager, container, network, typ string) {
	t.Helper()
	cfg := &CaddyConfig{
		Network:     network,
		Container:   container,
		Type:        typ,
		Domains:     []string{"a.example.com"},
		Upstream:    container + ":80",
		DNSProvider: "cloudflare",
	}
	if err := mgr.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig(%s/%s): %v", container, network, err)
	}
}

func configExists(t *testing.T, base, typ, key string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(base, typ, key+".conf"))
	return err == nil
}

// TestRemoveConfig_DoesNotTouchOtherNetworks deckt H3 ab. ConfigKey ist
// "container_network", der Key "web_bar_foo_caddy" endet daher ebenfalls auf
// "_foo_caddy" - ein Suffix-Match haette beide Netze abgeraeumt.
func TestRemoveConfig_DoesNotTouchOtherNetworks(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	writeTestConfig(t, mgr, "web", "foo_caddy", TypeInternal)
	writeTestConfig(t, mgr, "web", "bar_foo_caddy", TypeInternal)

	removed, err := mgr.RemoveConfig("foo_caddy")
	if err != nil {
		t.Fatalf("RemoveConfig: %v", err)
	}
	if !removed {
		t.Error("expected removal to report a change")
	}

	if configExists(t, base, TypeInternal, "web_foo_caddy") {
		t.Error("config of the targeted network should be gone")
	}
	if !configExists(t, base, TypeInternal, "web_bar_foo_caddy") {
		t.Error("config of the unrelated network bar_foo_caddy was removed")
	}
	if _, ok := mgr.configs["web_bar_foo_caddy"]; !ok {
		t.Error("stored config of the unrelated network was dropped")
	}
}

// TestRemoveConfig_ExactAfterRestart: nach einem Watcher-Neustart ist
// m.configs leer. Die Zuordnung muss dann aus dem Dateikopf kommen, sonst
// greift die Suffix-Mehrdeutigkeit aus H3 wieder.
func TestRemoveConfig_ExactAfterRestart(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "web", "foo_caddy", TypeInternal)
	writeTestConfig(t, seed, "web", "bar_foo_caddy", TypeInternal)

	// Frischer Manager = leerer In-Memory-State, wie nach einem Neustart.
	mgr := NewCaddyManager(base, nil)

	if _, err := mgr.RemoveConfig("foo_caddy"); err != nil {
		t.Fatalf("RemoveConfig: %v", err)
	}

	if configExists(t, base, TypeInternal, "web_foo_caddy") {
		t.Error("config of the targeted network should be gone")
	}
	if !configExists(t, base, TypeInternal, "web_bar_foo_caddy") {
		t.Error("config of the unrelated network bar_foo_caddy was removed")
	}
}

// TestRemoveConfig_LeavesManualConfigsAlone: handgepflegte Dateien tragen den
// Watcher-Kopf nicht und duerfen deshalb nie geloescht werden.
func TestRemoveConfig_LeavesManualConfigsAlone(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, TypeExternal)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(dir, "handwritten_foo_caddy.conf")
	if err := os.WriteFile(manual, []byte("# von Hand gepflegt\nhttps://a.example.com {\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewCaddyManager(base, nil)
	if _, err := mgr.RemoveConfig("foo_caddy"); err != nil {
		t.Fatalf("RemoveConfig: %v", err)
	}

	if _, err := os.Stat(manual); err != nil {
		t.Errorf("manual config was removed: %v", err)
	}
}

// TestPruneNetwork_RemovesStaleConfigs deckt H4 ab: eine Config, die nicht mehr
// deklariert wird, muss verschwinden - sonst routet die Domain weiter.
func TestPruneNetwork_RemovesStaleConfigs(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	writeTestConfig(t, mgr, "keeper", "app_caddy", TypeExternal)
	writeTestConfig(t, mgr, "goner", "app_caddy", TypeExternal)
	writeTestConfig(t, mgr, "other", "different_caddy", TypeExternal)

	// "goner" deklariert seine Config nicht mehr. Entfernt wird sie erst nach
	// der Karenz - auch wenn der Container laeuft.
	declared := map[string]bool{"keeper_app_caddy": true}
	start := time.Now()
	if pruned, err := mgr.PruneNetwork("app_caddy", declared, nil, true, start); err != nil || len(pruned) != 0 {
		t.Fatalf("first pass must only start the grace period: %v, %v", pruned, err)
	}
	pruned, err := mgr.PruneNetwork("app_caddy", declared, nil, true, start.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatalf("PruneNetwork: %v", err)
	}

	if len(pruned) != 1 || pruned[0] != "goner_app_caddy" {
		t.Errorf("expected only goner_app_caddy to be pruned, got %v", pruned)
	}
	if !configExists(t, base, TypeExternal, "keeper_app_caddy") {
		t.Error("still-declared config was removed")
	}
	if configExists(t, base, TypeExternal, "goner_app_caddy") {
		t.Error("stale config still on disk")
	}
	if !configExists(t, base, TypeExternal, "other_different_caddy") {
		t.Error("config of another network was pruned")
	}
}

// TestPruneNetwork_LeavesManualConfigsAlone: handgepflegte Dateien stehen nicht
// in m.configs und duerfen nie angefasst werden.
func TestPruneNetwork_LeavesManualConfigsAlone(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	dir := filepath.Join(base, TypeExternal)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(dir, "handwritten_app_caddy.conf")
	if err := os.WriteFile(manual, []byte("# manual\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.PruneNetwork("app_caddy", nil, nil, true, time.Now()); err != nil {
		t.Fatalf("PruneNetwork: %v", err)
	}

	if _, err := os.Stat(manual); err != nil {
		t.Errorf("manual config was removed: %v", err)
	}
}

// TestOwnerMatches: bei bekanntem Besitzer entscheidet der exakte Name. Nur
// ohne ihn - also fuer Dateien, die einen Watcher-Neustart ueberlebt haben -
// wird aus dem Dateinamen geraten.
func TestOwnerMatches(t *testing.T) {
	mgr := NewCaddyManager(t.TempDir(), nil)
	present := map[string]bool{"web": true, "vpn-gluetun-1": true}

	// Besitzer bekannt: exakter Vergleich, keine Praefix-Treffer.
	if !mgr.ownerMatches("vpn-gluetun-1", "vpn-gluetun-1-browser_n_caddy", "n_caddy", present) {
		t.Error("known owner must match")
	}
	if mgr.ownerMatches("web-api", "web-api_n_caddy", "n_caddy", present) {
		t.Error("a \"web\" entry must not stand in for \"web-api\"")
	}

	// Besitzer unbekannt: Rueckfall auf den Dateinamen.
	cases := []struct {
		key  string
		want bool
		why  string
	}{
		{"web_n_caddy", true, "Container laeuft"},
		{"vpn-gluetun-1-browser_n_caddy", true, "Multi-Service auf laufendem Container"},
		{"api_n_caddy", false, "Container fehlt"},
		{"webshop_n_caddy", false, "Praefix-Kollision darf nicht greifen"},
	}
	for _, tc := range cases {
		if got := mgr.ownerMatches("", tc.key, "n_caddy", present); got != tc.want {
			t.Errorf("ownerMatches(\"\", %q) = %v, want %v (%s)", tc.key, got, tc.want, tc.why)
		}
	}
}

// TestPruneNetwork_ClockResetsWithoutRemovalAllowed: bei unvollstaendigem
// Container-Bestand wird nicht geloescht - die Uhren muessen aber trotzdem
// gepflegt werden.
//
// Sonst gilt: Config faellt weg (Uhr startet), Container kommt zurueck, aber
// ausgerechnet dieser Durchlauf hat einen unbenennbaren Container gesehen und
// laesst die Uhr stehen. Der naechste vollstaendige Durchlauf loescht dann
// sofort anhand der alten Uhr - ohne dass je durchgehend Abwesenheit vorlag.
func TestPruneNetwork_ClockResetsWithoutRemovalAllowed(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)
	writeTestConfig(t, mgr, "web", "app_caddy", TypeExternal)
	key := "web_app_caddy"
	start := time.Now()

	// Container weg -> Uhr startet.
	if _, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start); err != nil {
		t.Fatal(err)
	}
	if _, running := mgr.absentSince[key]; !running {
		t.Fatal("expected the clock to be running")
	}

	// Container zurueck, aber Bestand unvollstaendig: nicht loeschen, Uhr aber
	// zuruecksetzen.
	declared := map[string]bool{key: true}
	pruned, err := mgr.PruneNetwork("app_caddy", declared, nil, false, start.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 0 {
		t.Fatalf("nothing may be removed while the inventory is incomplete: %v", pruned)
	}
	if _, running := mgr.absentSince[key]; running {
		t.Error("clock was not reset although the config was declared again")
	}

	// Spaeter faellt sie erneut weg: die Karenz muss von vorn laufen.
	if pruned, _ := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace*2)); len(pruned) != 0 {
		t.Errorf("removal used the stale clock: %v", pruned)
	}
}

// TestPruneNetwork_ProtectedResetsClock: "geschuetzt" heisst, wir konnten nicht
// feststellen, was der Container will. Das ist kein Beleg fuer Abwesenheit -
// eine laufende Uhr darf deshalb nicht weiterticken.
func TestPruneNetwork_ProtectedResetsClock(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)
	writeTestConfig(t, mgr, "web", "app_caddy", TypeExternal)
	key := "web_app_caddy"
	start := time.Now()

	if _, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start); err != nil {
		t.Fatal(err)
	}

	// Container ist wieder da, sein Inspect scheitert aber.
	protected := map[string]bool{"web": true}
	if _, err := mgr.PruneNetwork("app_caddy", nil, protected, true, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, running := mgr.absentSince[key]; running {
		t.Error("clock kept running although the container was protected")
	}

	// Direkt danach ungeschuetzt: es darf nicht sofort geloescht werden.
	if pruned, _ := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace*2)); len(pruned) != 0 {
		t.Errorf("removal used the stale clock: %v", pruned)
	}
}

// TestPruneNetwork_AbsentContainerGetsGrace deckt das Watchtower-Fenster ab:
// waehrend eines Updates ist der Container weg. Seine Config darf dann nicht
// sofort verschwinden - sonst faellt die Site fuer die Dauer des Updates aus
// dem Proxy. Nach der Karenz wird trotzdem aufgeraeumt.
func TestPruneNetwork_AbsentContainerGetsGrace(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)
	writeTestConfig(t, mgr, "updating", "app_caddy", TypeExternal)

	start := time.Now()

	// Erster Durchlauf mitten im Update: Container weg, nichts wird geloescht.
	pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start)
	if err != nil {
		t.Fatalf("PruneNetwork: %v", err)
	}
	if len(pruned) != 0 {
		t.Fatalf("config removed during update window: %v", pruned)
	}
	if !configExists(t, base, TypeExternal, "updating_app_caddy") {
		t.Fatal("config removed during update window")
	}

	// Kurz danach immer noch nicht.
	if pruned, _ := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(2*time.Minute)); len(pruned) != 0 {
		t.Errorf("config removed while still within grace: %v", pruned)
	}

	// Container ist zurueck und deklariert seine Config wieder -> Timer reset.
	declared := map[string]bool{"updating_app_caddy": true}
	if _, err := mgr.PruneNetwork("app_caddy", declared, nil, true, start.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !configExists(t, base, TypeExternal, "updating_app_caddy") {
		t.Fatal("config removed after the container returned")
	}
	if _, tracking := mgr.absentSince["updating_app_caddy"]; tracking {
		t.Error("absence timer was not reset after the container returned")
	}
}

// TestPruneNetwork_AbsentContainerEventuallyRemoved: irgendwann muss aufgeraeumt
// werden, auch wenn das Netzwerk selbst weiter in Benutzung ist.
func TestPruneNetwork_AbsentContainerEventuallyRemoved(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)
	writeTestConfig(t, mgr, "gone", "app_caddy", TypeExternal)

	start := time.Now()
	if _, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start); err != nil {
		t.Fatal(err)
	}

	pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 || pruned[0] != "gone_app_caddy" {
		t.Fatalf("expected cleanup after grace, got %v", pruned)
	}
	if configExists(t, base, TypeExternal, "gone_app_caddy") {
		t.Error("config still on disk after grace elapsed")
	}
}

// TestPruneNetwork_NeverRemovesWithoutGrace haelt die wichtigste Zusicherung
// des Aufraeumens fest: es gibt KEINEN Pfad, auf dem eine Konfiguration ohne
// abgelaufene Karenz verschwindet.
//
// Eine frueher vorhandene Sonderregel ("der Container laeuft, also ist das
// Fehlen der Deklaration endgueltig") hat im Lasttest mit 20 gleichzeitig
// getauschten Containern Konfigurationen sofort geloescht: die Liste der
// laufenden Container und die Praesenzpruefung waren zwei getrennte Abfragen,
// und ein gerade neu entstandener Container erschien kurzzeitig nur in einer
// von beiden. Der Zeitablauf ist das einzige Signal, das nicht von der
// Konsistenz zweier Momentaufnahmen abhaengt.
func TestPruneNetwork_NeverRemovesWithoutGrace(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)
	writeTestConfig(t, mgr, "web", "app_caddy", TypeExternal)

	start := time.Now()
	for _, at := range []time.Time{start, start.Add(time.Second), start.Add(mgr.absentGrace - time.Second)} {
		pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, at)
		if err != nil {
			t.Fatal(err)
		}
		if len(pruned) != 0 {
			t.Fatalf("config removed before the grace elapsed (at +%s): %v", at.Sub(start), pruned)
		}
	}
	if !configExists(t, base, TypeExternal, "web_app_caddy") {
		t.Fatal("config disappeared during the grace period")
	}

	pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace+time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 {
		t.Errorf("expected removal after the grace elapsed, got %v", pruned)
	}
}

// TestPruneNetwork_AfterRestartStillCleansUp: nach einem Watcher-Neustart ist
// m.configs leer. Ohne den Datei-Durchgang wuerde eine verwaiste Config nie
// aufgeraeumt, solange das Netzwerk in Benutzung bleibt.
func TestPruneNetwork_AfterRestartStillCleansUp(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "gone", "app_caddy", TypeExternal)

	// Frischer Manager = leerer Speicher, wie nach einem Neustart.
	mgr := NewCaddyManager(base, nil)
	start := time.Now()

	if pruned, _ := mgr.PruneNetwork("app_caddy", nil, nil, true, start); len(pruned) != 0 {
		t.Fatalf("nothing may be removed on the first pass after a restart: %v", pruned)
	}
	pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 || pruned[0] != "gone_app_caddy" {
		t.Fatalf("orphan from before the restart was not cleaned up: %v", pruned)
	}
}

// TestPruneNetwork_MultiServiceOwner: der Config-Container heisst
// "<container>-<service>". Laeuft der Container, gilt die Config als lebendig.
func TestPruneNetwork_MultiServiceOwner(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	cfg := &CaddyConfig{
		Network:        "app_caddy",
		Container:      "vpn-gluetun-1-browser",
		OwnerContainer: "vpn-gluetun-1",
		Type:           TypeExternal,
		Domains:        []string{"a.example.com"},
		Upstream:       "vpn-gluetun-1:3000",
		DNSProvider:    "cloudflare",
	}
	if err := mgr.WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Service nicht mehr deklariert -> nach der Karenz weg.
	start := time.Now()
	if _, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start); err != nil {
		t.Fatal(err)
	}
	pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 {
		t.Fatalf("expected removal after the grace elapsed, got %v", pruned)
	}
}

// TestPruneNetwork_ProtectedSurvivesEmptyMemory ist der wichtigste Test dieser
// Datei.
//
// Nach einem Watcher-Neustart ist m.configs leer. Scheitert dann der Inspect
// eines laufenden Containers, ist seine Config nirgends als "behalten"
// vermerkt - und weil der Container laeuft, wuerde die Karenz uebersprungen und
// die Datei SOFORT geloescht. Eine Produktions-Site waere weg. Die
// protected-Liste muss unabhaengig vom Speicherzustand greifen.
func TestPruneNetwork_ProtectedSurvivesEmptyMemory(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "web", "app_caddy", TypeExternal)

	// Frischer Manager = leerer Speicher, wie nach einem Neustart.
	mgr := NewCaddyManager(base, nil)

	protected := map[string]bool{"web": true}
	start := time.Now()

	// Auch weit nach Ablauf der Karenz bleibt eine geschuetzte Config liegen.
	for _, at := range []time.Time{start, start.Add(mgr.absentGrace * 3)} {
		pruned, err := mgr.PruneNetwork("app_caddy", nil, protected, true, at)
		if err != nil {
			t.Fatal(err)
		}
		if len(pruned) != 0 {
			t.Fatalf("protected config was pruned: %v", pruned)
		}
	}
	if !configExists(t, base, TypeExternal, "web_app_caddy") {
		t.Fatal("config of a container whose inspect failed was deleted")
	}

	// Gegenprobe: ohne Schutz greift die Karenz, danach wird aufgeraeumt.
	if _, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start); err != nil {
		t.Fatal(err)
	}
	pruned, err := mgr.PruneNetwork("app_caddy", nil, nil, true, start.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 {
		t.Errorf("expected removal without protection, got %v", pruned)
	}
}

// TestPruneNetwork_ProtectedMultiService: der Schutz muss auch fuer die
// Configs eines Multi-Service-Containers gelten.
func TestPruneNetwork_ProtectedMultiService(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	cfg := &CaddyConfig{
		Network: "app_caddy", Container: "vpn-gluetun-1-browser",
		OwnerContainer: "vpn-gluetun-1", Type: TypeExternal,
		Domains: []string{"a.example.com"}, Upstream: "vpn-gluetun-1:3000",
		DNSProvider: "cloudflare",
	}
	if err := mgr.WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}

	protected := map[string]bool{"vpn-gluetun-1": true}
	start := time.Now()
	for _, at := range []time.Time{start, start.Add(mgr.absentGrace * 3)} {
		pruned, err := mgr.PruneNetwork("app_caddy", nil, protected, true, at)
		if err != nil {
			t.Fatal(err)
		}
		if len(pruned) != 0 {
			t.Errorf("protected multi-service config was pruned: %v", pruned)
		}
	}
}

// TestPruneUnknownNetworks_VerifiesBeforeDeleting: die Netzwerkliste allein
// entscheidet nicht. Vor dem Loeschen wird beim Daemon nachgefragt.
func TestPruneUnknownNetworks_VerifiesBeforeDeleting(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "web", "ghost_caddy", TypeExternal)

	mgr := NewCaddyManager(base, nil)
	existing := map[string]bool{"other_caddy": true}
	start := time.Now()

	// Karenz starten.
	if _, err := mgr.PruneUnknownNetworks(existing, nil, start); err != nil {
		t.Fatal(err)
	}

	// Der Daemon sagt: das Netzwerk gibt es doch. Also nicht loeschen.
	stillThere := func(string) bool { return true }
	pruned, err := mgr.PruneUnknownNetworks(existing, stillThere, start.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 0 {
		t.Fatalf("config removed although the daemon still reports the network: %v", pruned)
	}
	if !configExists(t, base, TypeExternal, "web_ghost_caddy") {
		t.Fatal("config deleted despite the network still existing")
	}
}

// TestPruneUnknownNetworks_ClockOnlyStartsAfterVerification: die Uhr darf erst
// laufen, wenn der Daemon das Fehlen des Netzwerks bestaetigt hat.
//
// Sonst startet eine unvollstaendige Liste eine Uhr fuer ein existierendes
// Netzwerk; verschwindet es dann kurz vor Fristende wirklich, wuerde sofort
// geloescht, obwohl es nie durchgehend gefehlt hat.
func TestPruneUnknownNetworks_ClockOnlyStartsAfterVerification(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "web", "app_caddy", TypeExternal)

	mgr := NewCaddyManager(base, nil)
	key := "web_app_caddy"
	start := time.Now()

	// Liste behauptet, das Netz sei weg - der Daemon widerspricht.
	bogus := map[string]bool{"other_caddy": true}
	networkThere := func(string) bool { return true }
	for _, at := range []time.Time{start, start.Add(mgr.absentGrace * 2)} {
		pruned, err := mgr.PruneUnknownNetworks(bogus, networkThere, at)
		if err != nil {
			t.Fatal(err)
		}
		if len(pruned) != 0 {
			t.Fatalf("removed although the daemon confirms the network: %v", pruned)
		}
		if _, running := mgr.absentSince[key]; running {
			t.Fatal("clock started although the network still exists")
		}
	}

	// Jetzt ist das Netz wirklich weg: ab hier laeuft die Karenz von vorn.
	gone := func(string) bool { return false }
	later := start.Add(mgr.absentGrace * 2)
	if pruned, _ := mgr.PruneUnknownNetworks(bogus, gone, later); len(pruned) != 0 {
		t.Fatalf("removed on the very first verified observation: %v", pruned)
	}
	if pruned, _ := mgr.PruneUnknownNetworks(bogus, gone, later.Add(mgr.absentGrace+time.Minute)); len(pruned) != 1 {
		t.Errorf("expected removal one full grace after the verified disappearance, got %v", pruned)
	}
}

// TestPruneUnknownNetworks_EmptyListIsNotAuthoritative: eine leere
// Netzwerkliste sieht aus wie "alles weg" und darf keine Loeschungen ausloesen.
func TestPruneUnknownNetworks_EmptyListIsNotAuthoritative(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "web", "app_caddy", TypeExternal)

	mgr := NewCaddyManager(base, nil)
	start := time.Now()
	for _, at := range []time.Time{start, start.Add(mgr.absentGrace + time.Minute)} {
		if pruned, err := mgr.PruneUnknownNetworks(map[string]bool{}, nil, at); err != nil || len(pruned) != 0 {
			t.Fatalf("empty network list must not trigger deletions: %v, %v", pruned, err)
		}
	}
	if !configExists(t, base, TypeExternal, "web_app_caddy") {
		t.Error("config deleted based on an empty network list")
	}
}

// TestWriteConfig_TypeChangeKeepsFileOnFailure: beim Typwechsel darf die alte
// Datei erst verschwinden, wenn die neue steht.
func TestWriteConfig_TypeChangeKeepsFileOnFailure(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	writeTestConfig(t, mgr, "web", "app_caddy", TypeInternal)
	if !configExists(t, base, TypeInternal, "web_app_caddy") {
		t.Fatal("setup failed")
	}

	// Zielverzeichnis blockieren: eine Datei dort, wo das Verzeichnis
	// entstehen muesste, laesst MkdirAll scheitern.
	if err := os.WriteFile(filepath.Join(base, TypeExternal), []byte("blocker"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &CaddyConfig{
		Network: "app_caddy", Container: "web", OwnerContainer: "web",
		Type: TypeExternal, Domains: []string{"a.example.com"},
		Upstream: "web:80", DNSProvider: "cloudflare",
	}
	if err := mgr.WriteConfig(cfg); err == nil {
		t.Fatal("expected the write to fail")
	}

	if !configExists(t, base, TypeInternal, "web_app_caddy") {
		t.Error("old config was removed even though writing the new one failed")
	}
}

// TestTinyauthHostKeepsItsOwnCookies bildet einen echten Produktionsausfall ab:
// bei leerem TINYAUTH_DOMAIN wurde TinyAuth der eigene Session-Cookie entfernt,
// woraufhin der Login mit "Failed to get OAuth session cookie" scheiterte.
// Der Watcher bekommt fuer die Variable per docker-compose einen leeren
// Default - die Erkennung darf deshalb nicht allein daran haengen.
func TestTinyauthHostKeepsItsOwnCookies(t *testing.T) {
	tinyauth := &CaddyConfig{
		Network: "caddy_caddy", Container: "caddy-tinyauth-1",
		OwnerContainer: "caddy-tinyauth-1", Type: TypeExternal,
		Domains: []string{"auth.example.com"}, Upstream: "caddy-tinyauth-1:3000",
		DNSProvider: "cloudflare",
	}
	other := &CaddyConfig{
		Network: "app_caddy", Container: "app-web-1",
		OwnerContainer: "app-web-1", Type: TypeExternal,
		Domains: []string{"app.example.com"}, Upstream: "app-web-1:80",
		DNSProvider: "cloudflare",
	}

	t.Run("ohne TINYAUTH_DOMAIN", func(t *testing.T) {
		t.Setenv("COMPOSE_PROJECT_NAME", "caddy")
		t.Setenv("TINYAUTH_DOMAIN", "")

		if got := ssoCookieStripDirective(tinyauth); got != "" {
			t.Errorf("TinyAuth must keep its own cookies, got: %s", got)
		}
		if got := ssoCookieStripDirective(other); got == "" {
			t.Error("other backends must still have the SSO cookie stripped")
		}
	})

	t.Run("ueber TINYAUTH_DOMAIN bei abweichendem Aufbau", func(t *testing.T) {
		t.Setenv("COMPOSE_PROJECT_NAME", "")
		t.Setenv("TINYAUTH_DOMAIN", "auth.example.com")

		if got := ssoCookieStripDirective(tinyauth); got != "" {
			t.Errorf("domain-based exemption stopped working, got: %s", got)
		}
	})

	t.Run("fremder Host bleibt gestrippt", func(t *testing.T) {
		t.Setenv("COMPOSE_PROJECT_NAME", "caddy")
		t.Setenv("TINYAUTH_DOMAIN", "auth.example.com")

		if got := ssoCookieStripDirective(other); got == "" {
			t.Error("a regular backend must not be exempted")
		}
	})
}

// TestWriteConfig_SkipsUnchangedContent: der periodische Reconcile laeuft alle
// fuenf Minuten ueber alle Netzwerke. Wuerde WriteConfig dabei jedes Mal
// schreiben, loeste das ueber inotify einen Caddy-Reload aller Sites aus.
func TestWriteConfig_SkipsUnchangedContent(t *testing.T) {
	base := t.TempDir()
	mgr := NewCaddyManager(base, nil)

	writeTestConfig(t, mgr, "web", "app_caddy", TypeExternal)
	path := filepath.Join(base, TypeExternal, "web_app_caddy.conf")
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Zeitliche Aufloesung der mtime abwarten, damit ein erneutes Schreiben
	// wirklich sichtbar waere.
	time.Sleep(20 * time.Millisecond)
	writeTestConfig(t, mgr, "web", "app_caddy", TypeExternal)

	second, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Error("unchanged config was rewritten - this would trigger a Caddy reload")
	}

	// Eine echte Aenderung muss weiterhin durchschlagen.
	changed := &CaddyConfig{
		Network: "app_caddy", Container: "web", OwnerContainer: "web",
		Type: TypeExternal, Domains: []string{"b.example.com"},
		Upstream: "web:80", DNSProvider: "cloudflare",
	}
	if err := mgr.WriteConfig(changed); err != nil {
		t.Fatal(err)
	}
	third, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if third.ModTime().Equal(first.ModTime()) {
		t.Error("changed config was not written")
	}
}

// TestPruneUnknownNetworks deckt das verpasste network:destroy-Event ab: wird
// ein Projekt abgeraeumt, waehrend der Watcher gerade neu startet, bekommt er
// das Event nie. Ohne diesen Sweep bliebe die Datei dauerhaft liegen.
func TestPruneUnknownNetworks(t *testing.T) {
	base := t.TempDir()
	seed := NewCaddyManager(base, nil)
	writeTestConfig(t, seed, "web", "vanished_caddy", TypeExternal)
	writeTestConfig(t, seed, "web", "alive_caddy", TypeExternal)

	// Frischer Manager = leerer Speicher, wie nach einem Neustart.
	mgr := NewCaddyManager(base, nil)
	existing := map[string]bool{"alive_caddy": true}
	start := time.Now()

	// Erster Durchlauf startet nur die Karenz.
	if pruned, err := mgr.PruneUnknownNetworks(existing, nil, start); err != nil || len(pruned) != 0 {
		t.Fatalf("first pass must not remove anything: %v, %v", pruned, err)
	}

	// Ist das Netzwerk wieder da, wird nicht nur nicht geloescht - die Uhr
	// wird auch zurueckgesetzt, damit ein spaeteres Verschwinden die volle
	// Frist neu bekommt. Moeglich ist das nur, weil Netzwerk- und
	// Container-Uhr getrennte Maps sind.
	back := map[string]bool{"alive_caddy": true, "vanished_caddy": true}
	if pruned, _ := mgr.PruneUnknownNetworks(back, nil, start.Add(mgr.absentGrace+time.Minute)); len(pruned) != 0 {
		t.Errorf("config removed although its network came back: %v", pruned)
	}
	if _, running := mgr.networkGone["web_vanished_caddy"]; running {
		t.Error("network clock kept running although the network came back")
	}

	// Verschwindet es erneut, laeuft die Frist von vorn - nicht mit der alten
	// Uhr weiter.
	again := start.Add(mgr.absentGrace + 2*time.Minute)
	if pruned, _ := mgr.PruneUnknownNetworks(existing, nil, again); len(pruned) != 0 {
		t.Fatalf("removal used the clock from before the network returned: %v", pruned)
	}

	// Endgueltig weg: nach einer vollen, durchgehenden Frist aufraeumen.
	pruned, err := mgr.PruneUnknownNetworks(existing, nil, again.Add(mgr.absentGrace+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 || pruned[0] != "web_vanished_caddy" {
		t.Fatalf("expected web_vanished_caddy to be cleaned up, got %v", pruned)
	}
	if configExists(t, base, TypeExternal, "web_vanished_caddy") {
		t.Error("config of the vanished network still on disk")
	}
	if !configExists(t, base, TypeExternal, "web_alive_caddy") {
		t.Error("config of a live network was removed")
	}
}

// TestPruneUnknownNetworks_LeavesManualConfigsAlone: ohne Watcher-Kopf keine
// Zustaendigkeit - handgepflegte Dateien bleiben liegen, auch wenn zu ihrem
// vermeintlichen Netzwerk nichts existiert.
func TestPruneUnknownNetworks_LeavesManualConfigsAlone(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, TypeExternal)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(dir, "handwritten_proxy_apps.conf")
	if err := os.WriteFile(manual, []byte("https://a.example.com {\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewCaddyManager(base, nil)
	start := time.Now()
	for _, at := range []time.Time{start, start.Add(mgr.absentGrace + time.Minute)} {
		if _, err := mgr.PruneUnknownNetworks(map[string]bool{"any_caddy": true}, nil, at); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := os.Stat(manual); err != nil {
		t.Errorf("manual config was removed: %v", err)
	}
}

// TestAuthScrub_ScopedToUnprotectedPaths deckt H1 ab.
//
// Auf Pfaden, die forward_auth abdeckt, setzt copy_headers die Header aus der
// Auth-Antwort (loeschen, dann setzen) - dort darf nicht zusaetzlich gescrubbt
// werden, sonst kaeme beim Backend gar keine Identitaet an. Caddy ordnet
// request_header naemlich NACH forward_auth ein.
func TestAuthScrub_ScopedToUnprotectedPaths(t *testing.T) {
	t.Run("paths mode scrubs the inverse", func(t *testing.T) {
		block := generateAuthBlock("", []string{"/admin/*"}, nil, nil)
		if !strings.Contains(block, "@auth-scrub not path /admin/*") {
			t.Errorf("expected inverted scrub matcher, got:\n%s", block)
		}
		for _, h := range []string{"Remote-User", "Remote-Email", "Remote-Groups"} {
			if !strings.Contains(block, "request_header @auth-scrub -"+h) {
				t.Errorf("expected %s to be scrubbed, got:\n%s", h, block)
			}
		}
	})

	t.Run("except mode scrubs the excepted paths", func(t *testing.T) {
		block := generateAuthBlock("", nil, []string{"/health"}, nil)
		if !strings.Contains(block, "@auth-scrub path /health") {
			t.Errorf("expected scrub on excepted paths, got:\n%s", block)
		}
	})

	t.Run("full site auth needs no scrub", func(t *testing.T) {
		block := generateAuthBlock("", nil, nil, nil)
		if strings.Contains(block, "auth-scrub") {
			t.Errorf("full-site auth must not scrub, got:\n%s", block)
		}
	})
}

// TestWriteConfig_AuthScrubReachesGeneratedFile stellt sicher, dass der Scrub
// auch tatsaechlich in der erzeugten Datei landet - fuer alle drei Typen.
func TestWriteConfig_AuthScrubReachesGeneratedFile(t *testing.T) {
	for _, typ := range ValidTypes {
		t.Run(typ, func(t *testing.T) {
			base := t.TempDir()
			mgr := NewCaddyManager(base, nil)
			cfg := &CaddyConfig{
				Network:     "app_caddy",
				Container:   "web",
				Type:        typ,
				Domains:     []string{"a.example.com"},
				Upstream:    "web:80",
				DNSProvider: "cloudflare",
				Auth:        true,
				AuthPaths:   []string{"/admin/*"},
			}
			if err := mgr.WriteConfig(cfg); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(base, typ, "web_app_caddy.conf"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), "request_header @auth-scrub -Remote-User") {
				t.Errorf("scrub missing from generated %s config:\n%s", typ, content)
			}
		})
	}
}
