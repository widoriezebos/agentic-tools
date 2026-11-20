package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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

// Git publishes the carry to the owned bare receiver. A files-backend
// repository ignores reftable, so a device there fails only the directory sync.
func TestGitAdapterCarryDurability(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("the directory barrier runs on Linux")
	}
	for _, mode := range []string{"carry", "automatic recovery"} {
		t.Run(mode, func(t *testing.T) {
			root, receiver, _ := goalBranchMainFixture(t, "m1", ".", false)
			writeFixtureEnrollment(t, root, "Wido")
			goalSyncMutationGit(t, root, "remote", "rename", "upstream", "origin")
			goalSyncMutationGit(t, root, "config", "goal.sync-remote", "origin")
			checkout := goalSyncMutationGit(t, root, "rev-parse", "--show-toplevel")
			const launch = "01J5X0000000000000000000K0"
			setupCLIWrite(t, filepath.Join(checkout, "artifacts", "agents", "ui", "launches", launch+".json"),
				fmt.Sprintf(`{"launch":%q,"destination":%q,"created":{"destination":true}}`, launch, receiver), 0644)
			failSync := func() {
				if err := os.Symlink(os.DevNull, filepath.Join(receiver, "reftable")); err != nil {
					t.Fatal(err)
				}
			}
			carry := func(id string) (string, string, error) {
				tree := goalSyncMutationGit(t, root, "rev-parse", "refs/remotes/origin/main^{tree}")
				cmd := exec.Command(commandTestExecutable(t), goalCarryHelperCommand, "--root", root, "--id", id, "--by", "Wido", "--tree", tree,
					"--past", "missing-declaration", "--why", "exercise the receiving repository", "--raise-format", "--fixture-human-authority", "--lineage", "m1")
				cmd.Env = fixtureCommandEnvironment(t)
				var out, diagnostic bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &diagnostic
				err := cmd.Run()
				return out.String(), diagnostic.String(), err
			}
			if mode == "carry" {
				failSync()
			}
			out, diagnostic, err := carry("standing-validation")
			if err != nil || !strings.Contains(out, "carry=") {
				t.Fatalf("carry failed: %v, %s", err, diagnostic)
			}
			if mode == "automatic recovery" {
				if diagnostic != "" {
					t.Fatalf("healthy carry wrote to stderr: %s", diagnostic)
				}
				// Restore the journal as the exited publisher left it before recording confirmation.
				_, word, _ := strings.Cut(out, "carry=")
				opid, _, _ := strings.Cut(word, " ")
				entry, err := goal.ReadEntry(root, opid)
				if err != nil || entry.Outcome != goal.OutcomeConfirmed || goal.OwnerAlive(entry) {
					t.Fatal("expected a confirmed carry by the exited child", err)
				}
				entry.Phase, entry.Outcome, entry.TerminalAt, entry.Evidence = goal.PhasePushed, "", "", ""
				data, err := json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				setupCLIWrite(t, filepath.Join(root, "artifacts", "agents", "goal-transactions", opid+".json"), string(data), 0600)
				failSync()
				before := goalSyncMutationGit(t, receiver, "rev-parse", "refs/heads/main")
				// The missing goal refuses the new carry after Publish has recovered the first.
				out, diagnostic, err = carry("absent-goal")
				if err == nil || before != goalSyncMutationGit(t, receiver, "rev-parse", "refs/heads/main") {
					t.Fatal("the second carry should leave the receiver unchanged")
				}
				entry, err = goal.ReadEntry(root, opid)
				if err != nil || entry.Phase != goal.PhaseTerminal || entry.Outcome != goal.OutcomeConfirmed {
					t.Fatal("the next carry did not confirm the pushed entry", err)
				}
			}
			if strings.Count(diagnostic, "published; durability in doubt for "+receiver+":") != 1 || strings.Contains(out, "durability in doubt") {
				t.Fatalf("expected one receiver warning on stderr: %q", diagnostic)
			}
		})
	}
}
