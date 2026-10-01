package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// landingCheckout makes dir a landing checkout in the nested layout: a git
// checkout whose MetaSystem installation is dir/metasystem, so its checkout
// root is not its installation root. An installation already at dir (a
// flat checkout) is left flat.
func landingCheckout(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		command := exec.Command("git", "init", "-q", dir)
		command.Env = gittree.ScrubbedEnviron()
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v: %s", dir, err, out)
		}
	}
	for _, conf := range []string{filepath.Join(dir, "metasystem.conf"), filepath.Join(dir, "metasystem", "metasystem.conf")} {
		if _, err := os.Stat(conf); err == nil {
			return
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metasystem", "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// registerLane is a person's landing set of dir on the computer whose lane
// home is home.
func registerLane(t *testing.T, home, dir, by string, at time.Time) {
	t.Helper()
	landingCheckout(t, dir)
	layout, err := lane.NewLayout(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(home, layout, by, at); err != nil {
		t.Fatal(err)
	}
}
