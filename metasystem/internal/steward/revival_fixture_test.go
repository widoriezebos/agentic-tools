package steward

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type revivalFixture struct {
	root         string
	dependencies openWorkDependencies
	worldReads   atomic.Int32
	workReads    atomic.Int32
}

func newRevivalFixture(t *testing.T, wantWorld, wantWork int32) *revivalFixture {
	t.Helper()
	f := &revivalFixture{root: t.TempDir()}
	writeLedger(t, f.root, "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n")
	f.dependencies = openWorkDependencies{
		NewWorld: func(root string) bool {
			f.worldReads.Add(1)
			if root != f.root {
				t.Errorf("world route read unexpected root %q, want %q", root, f.root)
			}
			return false
		},
		ReadClaimableBudgetedWork: func(root string, _ time.Time) (goal.ClaimableBudgetedWork, error) {
			f.workReads.Add(1)
			if root != f.root {
				return goal.ClaimableBudgetedWork{}, fmt.Errorf("work read unexpected root %q, want %q", root, f.root)
			}
			return goal.ReadLegacyClaimableWork(root, identity.KernelProber{})
		},
	}
	t.Cleanup(func() {
		if got := f.worldReads.Load(); got != wantWorld {
			t.Errorf("world route reads = %d, want %d", got, wantWorld)
		}
		if got := f.workReads.Load(); got != wantWork {
			t.Errorf("legacy work reads = %d, want %d", got, wantWork)
		}
	})
	return f
}

func (f *revivalFixture) complete(cfg TickConfig, census WorkerCensus, nonce string, launch LaunchSeam, claims ...SeatIdleClaimSeam) (ReviveOutcome, error) {
	return completeRevivalWithDependencies(f.root, cfg, census, nonce, launch, f.dependencies, claims...)
}

func (f *revivalFixture) decide(cfg TickConfig, census WorkerCensus, ev Evidence, intent Intent) (Decision, string, error) {
	return decideForRevivalWithDependencies(f.root, cfg, census, ev, intent, f.dependencies)
}

func stagedRevivalFixture(t *testing.T, nonce string) (*revivalFixture, Intent) {
	t.Helper()
	f := newRevivalFixture(t, 1, 2)
	write := func(rel, body string) {
		path := filepath.Join(f.root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("scripts/agents/roles/steward-continuation.md", "# Role: steward-continuation\ncontract\n")
	write("scripts/agents/roles/steward-continuation.requirements.json", `{"required":[]}`)
	write("scripts/agents/schemas/steward-continuation.schema.json", `{"type":"object"}`)
	write("scripts/agents/permissions/workspace.json", `{"write":["workspace"]}`)
	idPath := RepoIdentityPath(f.root)
	if err := os.MkdirAll(filepath.Dir(idPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(idPath, InstallIdentity{RepoIdentity: f.root, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-08-20T15:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	state := writeStagedHandoffFixture(t, f.root, nonce)
	intent, err := StageHandoffIntent(f.root, nonce, "fix-it", "steward-"+nonce, "fake", "fixture", state.binding)
	if err != nil {
		t.Fatal(err)
	}
	return f, intent
}
