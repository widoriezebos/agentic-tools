package lease

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestRenewCompletesInterruptedSweep drives the recovery path: a lease whose
// takeover wrote the record but crashed before stamping the sweep. The same
// holder's next announcement must finish the sweep and stamp it complete.
func TestRenewCompletesInterruptedSweep(t *testing.T) {
	root := t.TempDir()
	seed := &Lease{
		HolderMainId: "main-1-1-aaaaaa", OwnerLineage: "main-1-1-aaaaaa",
		Pid: 123, PidStartedAt: 111, CommandHash: "x",
		ClaimedAt: "t", RenewedAt: "t", Revision: 3, ClaimEpoch: 5,
		Takeovers: []Takeover{{FromMainId: "main-0-0-oldold", ToMainId: "main-1-1-aaaaaa", ClaimEpoch: 5, TakenAt: "t", Reason: "holder-death"}},
	}
	if err := saveLease(root, seed); err != nil {
		t.Fatal(err)
	}
	// No stamp on disk: the sweep is incomplete. A completed job carrying a
	// protocol error so the inherited-error report has something to say.
	writeJSON(t, filepath.Join(root, "artifacts/agents/jobs/job-1.json"),
		`{"jobId":"job-1","claimEpoch":5,"status":"completed","protocolError":{"key":"K1"}}`)

	mustClaim(t, root, ann("main-1-1-aaaaaa", 123, 111, ""))

	lease, _ := loadLease(root, true)
	if lease.Revision != 4 {
		t.Fatalf("renewal should bump the revision to 4, got %d", lease.Revision)
	}
	if !newClaimer(root).stampComplete(lease) {
		t.Fatal("the interrupted sweep should be stamped complete after renewal")
	}
}

func TestProtocolCountsFollowsChains(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts/agents/jobs")
	writeJSON(t, filepath.Join(jobs, "root.json"), `{"jobId":"root","protocolError":{"key":"K1"}}`)
	writeJSON(t, filepath.Join(jobs, "child.json"), `{"jobId":"child","parentJob":"root","protocolError":{"key":"K2"}}`)
	// A cycle must be dropped, not counted or hung on.
	writeJSON(t, filepath.Join(jobs, "x.json"), `{"jobId":"x","parentJob":"y","protocolError":{"key":"KX"}}`)
	writeJSON(t, filepath.Join(jobs, "y.json"), `{"jobId":"y","parentJob":"x"}`)

	counts := protocolCounts(root)
	if counts["root"] != 2 {
		t.Fatalf("root chain should total both K1 and K2: %v", counts)
	}
	if _, ok := counts["x"]; ok {
		t.Fatalf("a cyclic chain must not be counted: %v", counts)
	}
}

// TestRequireHolderClaimsUnheldForAuthenticatedMain covers the path where an
// authenticated main gates a write on an unclaimed checkout: the first gated
// write claims, first-come-first-served.
func TestRequireHolderClaimsUnheldForAuthenticatedMain(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	start := selfStart(t)
	command, ok := ProcessCommand(self)
	if !ok {
		t.Skip("cannot read our own command")
	}
	// Hand-write an announcement (so a lease is NOT yet claimed), authenticating
	// this live process as a main.
	ann := Announcement{
		SessionId: "s", MainId: fmt.Sprintf("main-%d-%d-abcdef", start, self),
		Pid: self, PidStartedAt: start, Pgid: 1, Runtime: "fake", InstanceTag: "t",
		CommandHash: CommandHash(command), AnnouncedAt: "2026-01-01T00:00:00Z",
	}
	if err := atomicJSON(filepath.Join(root, "artifacts/agents/mains", "s.json"), &ann); err != nil {
		t.Fatal(err)
	}
	if lease, _ := loadLease(root, false); lease != nil {
		t.Fatal("precondition: no lease should exist yet")
	}
	held, err := RequireHolder(root, self, nil)
	if err != nil {
		t.Fatalf("an authenticated main should claim an unheld checkout: %v", err)
	}
	if held["class"] != "HOLDER" {
		t.Fatalf("first gated write should become HOLDER: %v", held)
	}
	if lease, _ := loadLease(root, true); lease == nil || lease.ClaimEpoch != 1 {
		t.Fatalf("the checkout should now be claimed at epoch 1")
	}
}
