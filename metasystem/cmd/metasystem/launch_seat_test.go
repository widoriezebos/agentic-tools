package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// A launch's seat on the host board names the installation that holds the
// engine. The steward and its launches run the enrolled engine from its pin
// (<installation>/artifacts/agents/steward/engine-pins/<pin>), so the seat is
// the installation above the pin, not the directory above the pin's folder;
// the engine at <installation>/bin/metasystem names the same installation.
// A card naming the steward folder reads unknown on every board.
func TestLaunchSeatNamesTheInstallationForAPinnedEngine(t *testing.T) {
	t.Parallel()
	installation := filepath.Join(t.TempDir(), "metasystem")
	pins := filepath.Join(installation, "artifacts", "agents", "steward", "engine-pins")
	for _, dir := range []string{pins, filepath.Join(installation, "bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := realpath.Resolve(installation)
	var resolved string
	resolve := func(root string) (string, error) { resolved = root; return "m1e", nil }
	for _, executable := range []string{filepath.Join(pins, "generation-9-abc123"), filepath.Join(installation, "bin", "metasystem")} {
		if err := testexec.WriteFile(executable, []byte("engine"), 0o755); err != nil {
			t.Fatal(err)
		}
		seat := launchSeat(executable, nil, resolve)
		if seat.Installation != want || resolved != want || seat.Machine != "m1e" {
			t.Fatalf("seat for %s = %+v (resolved %q), want installation %q", executable, seat, resolved, want)
		}
	}
}
