package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Git supplies the effective method and the receiving common directory.
func TestGitAdapterSystemDurability(t *testing.T) {
	t.Parallel()
	bed, home := newSystemSetupBed(t), t.TempDir()
	layout, err := lane.NewLayout(bed.repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(home, layout, "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := systemSetupGit(t, bed.repo, "remote", "add", "origin", "https://example.invalid/ledger"); err != nil {
		t.Fatal(err)
	}
	if _, err := systemSetupGit(t, bed.repo, "config", "core.fsyncMethod", "fsync"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ value, want string }{{"", "missing"}, {"committed", "below minimum"}, {"reference", "below minimum"}, {"all", "meets minimum"}} {
		if c.value != "" {
			if _, err := systemSetupGit(t, bed.repo, "config", "core.fsync", c.value); err != nil {
				t.Fatal(err)
			}
		}
		cfg := filepath.Join(bed.repo, ".git", "config")
		before, err := os.ReadFile(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(cfg, 0444); err != nil {
			t.Fatal(err)
		}
		cmd, rest, _ := resolveIntentArgv([]string{"system", "check", "--durability", "--json"})
		var out bytes.Buffer
		code := runIntentIn(cmd, rest, &out, &out, bed.repo, intentOwners{resolver: stateroot.NewResolver(stateroot.RepositoryTop, noExecutable), landing: laneVerbOwners{home: func() (string, error) { return home, nil }}})
		after, err := os.ReadFile(cfg)
		if err != nil || !bytes.Equal(before, after) || code != 0 || !strings.Contains(out.String(), c.want) || !strings.Contains(out.String(), "not controlled") {
			t.Fatalf("status %d, %s", code, &out)
		}
		if err := os.Chmod(cfg, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
