package diskstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func settingsCheckout(t *testing.T, committed, local string) string {
	t.Helper()
	dir := filepath.Join(realDir(t), "checkout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, "metasystem.conf")
	if err := os.WriteFile(conf, []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	if local != "" {
		if err := os.WriteFile(conf+".local", []byte(local), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return conf
}

func testEnv(home string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name == "HOME" {
			return home, true
		}
		return "", false
	}
}

// The compiled defaults stand with source "default"; .local overrides and
// its source is named; conf and .local only override (3.13).
func TestLoadSettingsReadsDefaultsAndOverrides(t *testing.T) {
	home := realDir(t)
	conf := settingsCheckout(t, "metasystem.version=1\n", "disk.floor-gib=80\n")
	settings, err := LoadSettings(conf, testEnv(home))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Bytes(config.DiskFloorKey) != 80<<30 || settings.Sources[config.DiskFloorKey] != "conf-local" {
		t.Fatalf("floor = %d from %s", settings.Bytes(config.DiskFloorKey), settings.Sources[config.DiskFloorKey])
	}
	checks := map[string]time.Duration{config.DiskContextKeepKey: 14 * 24 * time.Hour, config.DiskSweepBudgetKey: 20 * time.Second,
		config.DiskFloorMinAgeKey: time.Hour, config.DiskEvidenceAgeFloorKey: 90 * 24 * time.Hour}
	for key, want := range checks {
		if got := settings.Duration(key); got != want || settings.Sources[key] != "default" {
			t.Errorf("%s = %s from %s, want %s from default", key, got, settings.Sources[key], want)
		}
	}
	if settings.Count(config.DiskSweepItemsPerLockKey) != 8 || settings.Bytes(config.DiskEvidenceMachineCapKey) != 50<<30 {
		t.Fatal("compiled defaults were not read")
	}
	if settings.EvidenceRoot.Origin != "default" || !strings.HasPrefix(settings.EvidenceRoot.Path, filepath.Join(home, "metasystem-evidence")) {
		t.Fatalf("evidence root = %+v", settings.EvidenceRoot)
	}
	bad := settingsCheckout(t, "disk.floor-gib=0\n", "")
	if _, err := LoadSettings(bad, testEnv(home)); err == nil || !strings.Contains(err.Error(), "disk.floor-gib") {
		t.Fatalf("an invalid value = %v; want the key named", err)
	}
	if _, err := LoadSettings(filepath.Join(home, "absent", "metasystem.conf"), testEnv(home)); err == nil {
		t.Fatal("an unreadable settings file loaded")
	}
}

// Host-scoped keys resolve to the most conservative value with a conflict
// line; an unreadable participant makes the host policy Unknown and no
// other participant's value stands in; readable again, it resolves (DL4D-09,
// DL4E-08).
func TestHostSettingsResolveConservativelyOrNotAtAll(t *testing.T) {
	home := realDir(t)
	m1e, err := LoadSettings(settingsCheckout(t, "evidence.machine-cap-gib=50\n", "disk.sweep-budget-sec=30\n"), testEnv(home))
	if err != nil {
		t.Fatal(err)
	}
	m1b, err := LoadSettings(settingsCheckout(t, "evidence.machine-cap-gib=80\n", "evidence.citation-roots=records/extra\n"), testEnv(home))
	if err != nil {
		t.Fatal(err)
	}
	host := ResolveHost([]Participant{{Checkout: "m1e", Settings: m1e}, {Checkout: "m1b", Settings: m1b}})
	if !host.Known() || host.Bytes(config.DiskEvidenceMachineCapKey) != 80<<30 || host.Duration(config.DiskSweepBudgetKey) != 20*time.Second {
		t.Fatalf("host = %+v", host.Values)
	}
	joined := strings.Join(host.Conflicts, "\n")
	for _, want := range []string{"settings conflict: evidence.machine-cap-gib 50 (m1e), 80 (m1b): 80 in force", "disk.sweep-budget-sec 20 (m1b), 30 (m1e): 20 in force"} {
		if !strings.Contains(joined, want) {
			t.Errorf("conflicts %q lack %q", joined, want)
		}
	}
	if m1b.Values[config.DiskEvidenceCitationKey] != "records/extra" || m1e.Values[config.DiskEvidenceCitationKey] != "" {
		t.Fatal("citation roots are not each checkout's own")
	}
	unreadable := ResolveHost([]Participant{{Checkout: "m1e", Settings: m1e}, {Checkout: "m1c", Err: os.ErrPermission}})
	if unreadable.Known() || unreadable.Values != nil || len(unreadable.Unknown) != 1 || !strings.Contains(unreadable.Unknown[0], "host settings unknown: m1c unreadable") {
		t.Fatalf("an unreadable participant = %+v", unreadable)
	}
	if again := ResolveHost([]Participant{{Checkout: "m1e", Settings: m1e}, {Checkout: "m1b", Settings: m1b}}); !again.Known() {
		t.Fatal("readable again, the host did not resolve")
	}
	if defaults := ResolveHost(nil); !defaults.Known() || defaults.Bytes(config.DiskFloorKey) != 50<<30 {
		t.Fatal("with no participant the compiled defaults do not stand")
	}
}
