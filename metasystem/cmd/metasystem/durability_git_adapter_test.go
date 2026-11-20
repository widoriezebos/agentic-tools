package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// The goal owner commits to the fixture repository. Git cannot find that
// commit in the owned directory, so the real barrier reports a doubt.
func TestGitAdapterPageReceivesPublicationDoubt(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("the directory barrier runs on Linux")
	}
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	bed.addGoal(queuedIntentGoal("queued-goal", 1))
	root := bed.root()
	if _, err := systemSetupGit(t, root, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	const id = "01J5X0000000000000000000K0"
	setupCLIWrite(t, filepath.Join(root, "artifacts", "agents", "ui", "launches", id+".json"),
		fmt.Sprintf(`{"launch":%q,"destination":%q,"created":{"destination":true}}`, id, root), 0644)
	cmd, args, _ := resolveIntentArgv([]string{"goal", "edit", "queued-goal", "--next", "Measure it.", "--json"})
	before := bed.publications()
	answer, err := verbAnswer(cmd.name, cmd, args, func(cmd intentCommand, args []string, stdout, stderr io.Writer) {
		if code := runIntentIn(cmd, args, stdout, stderr, root, bed.owners()); code != 0 {
			t.Errorf("goal edit exited %d", code)
		}
	})
	if err != nil || answer.Outcome != intentConfirmed || strings.Count(answer.Summary, "published; durability in doubt for "+filepath.Join(root, ".git")) != 1 ||
		bed.publications() != before+1 || bed.goalFile("queued-goal").NextStep != "Measure it." {
		t.Fatalf("captured goal edit: %+v, %v", answer, err)
	}
}
