package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// The detached proof's own test run is admitted to the lane by the lane's
// running-proof record, not by descent from the landing agent: once
// landing prove returned, the agent that asked has exited and the job was
// reparented, so no landing agent is its ancestor. The real lane admission
// accepts a caller that descends from the exact process the record names,
// for the tree it names, while it runs; a record whose process is gone, of
// another tree, or no record at all, is refused as before.
func TestDetachedProofIsAdmittedByItsRunningRecord(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	lane := newKernelBed(t)
	lane.landMain(t)
	tree := lane.git(t, lane.checkout, "rev-parse", "HEAD^{tree}")
	exact, live, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || live != identity.Alive {
		t.Fatalf("probe self: %v %v", live, err)
	}
	self, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	gone := exact.Ref()
	if gone.StartTicks != 0 {
		gone.StartTicks = 1
	} else {
		gone.StartedAtUnixMicro, gone.StartedAtSec = 1_000_000, 1
	}
	died, err := identity.EncodeRef(gone)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(lane.checkout, "artifacts", "agents", "landing-proofs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := func(proofTree, process string) {
		data, err := json.Marshal(kernel.TreeProof{Tree: proofTree, Attempt: "a9", Status: batch.AttemptRunning, Process: process, StartedAt: "2026-10-01T09:00:00Z"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, proofTree+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const account = "lane:0123456789ab"
	request := proofLaunchAdmission{ControlRoot: repository.root, ExecutionRoot: repository.root, ConfPath: filepath.Join(repository.root, "metasystem.conf"),
		LaneID: account, CapMin: "1", ScopeClass: "full", CommandClass: "testing", Now: now, CandidateTree: tree,
		// This test process stands in for the detached landing prove
		// --wait, the test run's caller; no landing agent holds the lane.
		CallerPID:   int64(os.Getpid()),
		laneAccount: func(string) (string, error) { return account, nil },
		laneHome:    func() (string, error) { return lane.home, nil }}
	refused := func(why string) {
		t.Helper()
		if _, _, _, err := admitAsLaneOwner(t, repository, request, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "landing agent, proven by its identity") {
			t.Fatalf("%s: admitted (%v); want refused", why, err)
		}
	}
	refused("no proof recorded running")
	keep(tree, died)
	refused("a proof whose process died")
	other := strings.Repeat("c", 40)
	if err := os.Remove(filepath.Join(dir, tree+".json")); err != nil {
		t.Fatal(err)
	}
	keep(other, self)
	refused("a running proof of another tree")
	if err := os.Remove(filepath.Join(dir, other+".json")); err != nil {
		t.Fatal(err)
	}
	keep(tree, self)
	attempt, decision, _, err := admitAsLaneOwner(t, repository, request, lease.ClassMain)
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || attempt.GoalID != account {
		t.Fatalf("the running proof's own test run: attempt=%+v decision=%+v err=%v; want it admitted to the lane", attempt, decision, err)
	}
}
