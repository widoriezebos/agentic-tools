package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeSelectionRegistry(t *testing.T, rows ...map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "armed-checkouts.jsonl")
	var content strings.Builder
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		content.Write(payload)
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultPathUsesOnlyAbsoluteRegistryHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv(supervisionRegistryHomeEnv, home)
	want := filepath.Join(home, ".metasystem", "armed-checkouts.jsonl")
	if got, err := DefaultPath(); err != nil || got != want {
		t.Fatalf("absolute registry home resolved to %q, %v; want %q", got, err, want)
	}

	t.Setenv(supervisionRegistryHomeEnv, "relative-home")
	if got, err := DefaultPath(); err == nil || got != "relative-home" || !strings.Contains(err.Error(), "absolute run-scoped home") {
		t.Fatalf("relative registry home resolved to %q, %v", got, err)
	}

	t.Setenv(supervisionRegistryHomeEnv, "")
	t.Setenv("HOME", home)
	if got, err := DefaultPath(); err != nil || got != want {
		t.Fatalf("default user home resolved to %q, %v; want %q", got, err, want)
	}
}

func TestOwnerCheckoutPathSelectsOpenProductionPublication(t *testing.T) {
	relaunched := raw(EventRelaunched, "owner-a", map[string]any{
		"generation": 1.0, "watcherTag": "watcher-a", "reaperTag": "reaper-a", "retiredThrough": 0.0,
	})
	relaunched["checkoutPath"] = "/checkout/a"
	launched := raw(EventLaunched, "owner-a", map[string]any{
		"generation": 1.0, "component": "watcher", "pid": 41.0, "pidStartedAt": 42.0,
	})
	launched["checkoutPath"] = "/checkout/a"
	path := writeSelectionRegistry(t, relaunched, launched)

	checkout, found, err := OwnerCheckoutPath(path, "owner-a")
	if err != nil || !found || checkout != "/checkout/a" {
		t.Fatalf("owner checkout selection = %q, %v, %v; want /checkout/a, true, nil", checkout, found, err)
	}
	if checkout, found, err := OwnerCheckoutPath(path, "missing-owner"); err != nil || found || checkout != "" {
		t.Fatalf("missing owner selection = %q, %v, %v; want empty, false, nil", checkout, found, err)
	}

	exited := raw(EventExited, "owner-a", map[string]any{"reason": "shutdown", "teardownComplete": true})
	exited["checkoutPath"] = "/checkout/a"
	closedPath := writeSelectionRegistry(t, relaunched, launched, exited)
	if checkout, found, err := OwnerCheckoutPath(closedPath, "owner-a"); err != nil || found || checkout != "" {
		t.Fatalf("closed owner selection = %q, %v, %v; want empty, false, nil", checkout, found, err)
	}
}

func TestOwnerCheckoutPathSurfacesReadAndReductionErrors(t *testing.T) {
	t.Run("registry read", func(t *testing.T) {
		if _, _, err := OwnerCheckoutPath(t.TempDir(), "owner-a"); err == nil {
			t.Fatal("a registry path naming a directory must surface its read error")
		}
	})

	t.Run("registry reduction", func(t *testing.T) {
		invalid := raw(EventRelaunched, "owner-a", map[string]any{
			"generation": 1.0, "watcherTag": "watcher-a", "reaperTag": "reaper-a", "retiredThrough": 0.0,
		})
		invalid["schemaVersion"] = 2.0
		path := writeSelectionRegistry(t, invalid)
		if _, _, err := OwnerCheckoutPath(path, "owner-a"); err == nil || !strings.Contains(err.Error(), "schemaVersion 1") {
			t.Fatalf("invalid production row did not surface its reduction error: %v", err)
		}
	})
}

func TestPublishedOwnerCheckoutConflictIsDroppedAndListed(t *testing.T) {
	relaunched := raw(EventRelaunched, "owner-a", map[string]any{
		"generation": 1.0, "watcherTag": "watcher-a", "reaperTag": "reaper-a", "retiredThrough": 0.0,
	})
	relaunched["checkoutPath"] = "/checkout/a"
	conflicting := raw(EventLaunched, "owner-a", map[string]any{
		"generation": 1.0, "component": "watcher", "pid": 41.0, "pidStartedAt": 42.0,
	})
	conflicting["checkoutPath"] = "/checkout/b"

	reduction := reduceOrFail(t, relaunched, conflicting)
	owner := reduction.PublishedOwners["owner-a"]
	if owner == nil || len(owner.Generations[1].Identities) != 0 {
		t.Fatalf("conflicting row changed the published owner: %+v", owner)
	}
	if len(reduction.Dropped) != 1 || !strings.Contains(reduction.Dropped[0], "/checkout/a") || !strings.Contains(reduction.Dropped[0], "/checkout/b") {
		t.Fatalf("conflicting row was not listed with both checkout paths: %v", reduction.Dropped)
	}
}

func TestCompactionRetainsActiveLegacyPublication(t *testing.T) {
	relaunched := raw(EventRelaunched, "legacy-owner", map[string]any{
		"generation": 1.0, "watcherTag": "watcher-legacy", "reaperTag": "reaper-legacy", "retiredThrough": 0.0,
	})
	relaunched["checkoutPath"] = "/legacy/state-root"
	launched := raw(EventLaunched, "legacy-owner", map[string]any{
		"generation": 1.0, "component": "watcher", "pid": 51.0, "pidStartedAt": 52.0,
	})
	launched["checkoutPath"] = "/legacy/state-root"

	kept, err := CompactFrames(frames(relaunched, launched), time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got := events(kept); len(got) != 2 || got[0] != EventRelaunched || got[1] != EventLaunched {
		t.Fatalf("active legacy publication was not retained intact: %v", got)
	}
	reduction, err := Reduce(kept)
	if err != nil {
		t.Fatal(err)
	}
	if checkout, found := reduction.OwnerCheckoutPath("legacy-owner"); !found || checkout != "/legacy/state-root" {
		t.Fatalf("compacted legacy owner selection = %q, %v", checkout, found)
	}
}
