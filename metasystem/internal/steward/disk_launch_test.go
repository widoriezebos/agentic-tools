package steward

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// The machine pass carries the launch store's retention (Part B U5d): with
// the host's disk.launch-target-mib exceeded, an ended launch past
// disk.launch-keep-days goes and a young one stays, and the class line
// counts the store.
func TestMachinePassReleasesEndedLaunchesOverTheTarget(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	if err := os.WriteFile(filepath.Join(bed.inst, "metasystem.conf"), []byte("metasystem.template=true\ndisk.floor-gib=1\ndisk.launch-target-mib=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	launches := filepath.Join(bed.home, "launch")
	for id, finished := range map[string]string{"20260801t000000-aaaaaaaaaa": "2026-08-01T01:00:00Z", "20260928t000000-bbbbbbbbbb": "2026-09-28T01:00:00Z"} {
		dir := filepath.Join(launches, id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		record, _ := json.Marshal(map[string]any{"id": id, "kind": "build", "state": "completed", "startedAt": finished, "finishedAt": finished})
		if err := os.WriteFile(filepath.Join(dir, "record.json"), record, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "exec.log"), []byte(strings.Repeat("x", 2<<20)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeApply, []string{bed.inst}, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(launches, "20260801t000000-aaaaaaaaaa")); !os.IsNotExist(err) {
		t.Errorf("the ended launch past its window is released: %v; report %+v", err, result.Machine)
	}
	if _, err := os.Stat(filepath.Join(launches, "20260928t000000-bbbbbbbbbb")); err != nil {
		t.Errorf("the young launch stays: %v", err)
	}
	var counted, units bool
	for _, class := range result.Machine.Classes {
		counted = counted || class.Name == "launch store" && class.Items == 2 && class.Released == 1
		units = units || class.Name == "unit records"
	}
	if !units {
		t.Errorf("the machine pass carries the unit records before the launches: %+v", result.Machine.Classes)
	}
	if !counted {
		t.Errorf("the launch store's class line: %+v", result.Machine.Classes)
	}
}

// The checkout pass carries the landing release sets: a swept landing's
// pending entry whose store no longer exists is marked absent by the next
// pass.
func TestCheckoutPassRetriesLandingReleaseSets(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	landed := filepath.Join(bed.inst, "artifacts", "agents", "landing-intent", "g", "c1-e1", "landed.json")
	if err := os.MkdirAll(filepath.Dir(landed), 0o700); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(map[string]any{"Landing": "land1", "Swept": true, "ReleaseSet": diskstore.ReleaseSet{Tip: "c1",
		Stores: []diskstore.ReleaseEntry{{ID: "01K00000000000000000000000", State: diskstore.ReleasePending}}}})
	if err := os.WriteFile(landed, record, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeApply, []string{bed.inst}, false)); err != nil {
		t.Fatal(err)
	}
	var after struct{ ReleaseSet diskstore.ReleaseSet }
	written, _ := os.ReadFile(landed)
	if json.Unmarshal(written, &after) != nil || after.ReleaseSet.Stores[0].State != diskstore.ReleaseAbsent {
		t.Fatalf("the checkout pass finishes the set: %s", written)
	}
}

// A checkout pass carries the workspace proof over its own repository and
// goal ledger; a checkout whose ledger cannot be read leaves every goal's
// state unknown, so no workspace is released on a guess.
func TestCheckoutProofsCarryTheWorkspaceProof(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	classes, ok := checkoutProofs(bed.inst, DiskPass{Now: staleNow})[diskstore.OwnerGoal].(diskstore.ClassProofs)
	if !ok {
		t.Fatalf("the goal's proofs are not by class: %+v", classes)
	}
	proof, ok := classes.ByClass[diskstore.WorkspaceClass].(diskstore.WorkspaceProof)
	if !ok || proof.GitRoot == "" || proof.Git == nil || proof.Ended == nil {
		t.Fatalf("proof = %+v", proof)
	}
	if worktree, ok := classes.ByClass[diskstore.GoalWorktreeClass].(diskstore.GoalWorktreeProof); !ok || worktree.Plan == nil || worktree.Sweep == nil {
		t.Fatalf("the goal worktree's proof = %+v", classes.ByClass[diskstore.GoalWorktreeClass])
	}
	if ended, known, _ := proof.Ended(diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "g"}); ended || known {
		t.Fatalf("no readable ledger: ended=%v known=%v", ended, known)
	}
	fixture := map[diskstore.OwnerKind]diskstore.OwnerProof{}
	if proofs := checkoutProofs(bed.inst, DiskPass{Proofs: fixture}); len(proofs) != 0 {
		t.Fatal("a fixture's proofs are used as named")
	}
}

// Both passes carry the process proof (Part B U1a): a process scratch root
// whose process ended and whose writer lock is free goes by it; the machine
// registry is where process scratch registers.
func TestDiskPassesCarryTheProcessProof(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	for name, proofs := range map[string]map[diskstore.OwnerKind]diskstore.OwnerProof{
		"machine": DiskPass{}.machineProofs(bed.inst), "checkout": checkoutProofs(bed.inst, DiskPass{Now: staleNow})} {
		if proof, ok := proofs[diskstore.OwnerProcess].(diskstore.ProcessProof); !ok || proof.Prober == nil {
			t.Errorf("the %s pass has no process proof: %+v", name, proofs[diskstore.OwnerProcess])
		}
	}
	// A unit read's findings store ends with its unit (Round D3 N4).
	machine := DiskPass{}.machineProofs(bed.inst)
	if proof, ok := machine[diskstore.OwnerUnit].(diskstore.UnitFindingsProof); !ok || proof.UnitRoot != filepath.Join(bed.inst, "unit") {
		t.Errorf("the machine pass has no unit findings proof: %+v", proof)
	}
}
