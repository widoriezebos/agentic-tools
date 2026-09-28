package lease

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Ports of lease-succession-fixtures.sh (verbs-object-action U7c). A mission
// runs as a chain of processes; each announces its own mainId, and before
// ownership lineages every succession looked like a foreign takeover that
// failed the predecessor's in-flight delegates as stale-claim-epoch.

// TestAnnounceLineageFillKeepsTheEpochAndAConflictIsRefusedByName: supplying
// a lineage where none was stored fills the announcement and the lease the
// claim reads without bumping the epoch; the same lineage again is accepted;
// a different one is refused, saying what it refused.
func TestAnnounceLineageFillKeepsTheEpochAndAConflictIsRefusedByName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	self, start := int64(os.Getpid()), selfStart(t)
	announce := func(lineage string) error {
		_, err := Announce(root, "mission-runner-bm-2", self, start, "metasystem-mission-runner", "fake", lineage)
		return err
	}
	if err := announce(""); err != nil {
		t.Fatal(err)
	}
	if err := announce("mission-deadbeef"); err != nil {
		t.Fatalf("supplying a lineage where none was stored must fill it in: %v", err)
	}
	mains, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	announcements := 0
	for _, path := range mains {
		base := filepath.Base(path)
		if strings.Contains(base, "worktree-lease") || strings.Contains(base, "reaped-after-claim") || strings.Contains(base, "protocol-cursor") {
			continue
		}
		announcements++
		if got := readAnnouncement(t, path).OwnerLineage; got != "mission-deadbeef" {
			t.Fatalf("the announcement %s carries lineage %q, want the filled one", base, got)
		}
	}
	if announcements != 1 {
		t.Fatalf("announcements = %d, want 1 (%v)", announcements, mains)
	}
	lease, err := loadLease(root, true)
	if err != nil || lease.OwnerLineage != "mission-deadbeef" || lease.ClaimEpoch != 1 {
		t.Fatalf("the fill must reach the lease without bumping the epoch: %+v %v", lease, err)
	}
	if err := announce("mission-deadbeef"); err != nil {
		t.Fatalf("the same lineage again must be accepted: %v", err)
	}
	err = announce("mission-different")
	if err == nil || !strings.Contains(err.Error(), "refusing to replace it") {
		t.Fatalf("a conflicting lineage must be refused by name, got %v", err)
	}
}

// TestConsecutiveHostTurnsRenewAndKeepTheInFlightDelegate is the bm-2
// scenario: two host-turn processes of one mission lineage announce in
// turn, the first gone before the second arrives, with a delegate still in
// flight between them. The second turn renews at the same epoch, records no
// takeover, and leaves the delegate pending.
func TestConsecutiveHostTurnsRenewAndKeepTheInFlightDelegate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const lineage = "mission-fixture-lineage"

	first := exec.Command("/bin/sh", "-c", "printf r; read line || :")
	hold, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	ready, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	var signal [1]byte
	if _, err := ready.Read(signal[:]); err != nil {
		t.Fatalf("first turn never reported ready: %v", err)
	}
	firstPid := int64(first.Process.Pid)
	firstStart, ok := StartedAt(firstPid, nil)
	if !ok {
		t.Fatalf("first turn %d has no readable start", firstPid)
	}
	if _, err := Announce(root, "host-first", firstPid, firstStart, "metasystem-host-turn", "claude", lineage); err != nil {
		t.Fatal(err)
	}
	// The first turn ends: its process exits before the next turn arrives.
	_ = hold.Close()
	if err := first.Wait(); err != nil {
		t.Fatalf("first turn did not end cleanly: %v", err)
	}

	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := filepath.Join(jobs, "inflight.json")
	if err := os.WriteFile(job, []byte(`{"jobId":"inflight","status":"pending","claimEpoch":1,"mainId":"m","role":"r","runtime":"devin"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	secondPid, secondStart := liveChild(t)
	if _, err := Announce(root, "host-second", secondPid, secondStart, "metasystem-host-turn", "claude", lineage); err != nil {
		t.Fatal(err)
	}
	lease, err := loadLease(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if lease.ClaimEpoch != 1 {
		t.Fatalf("a second host turn must renew, not bump the epoch: %d", lease.ClaimEpoch)
	}
	if len(lease.Takeovers) != 0 {
		t.Fatalf("a second host turn of the same mission is not a takeover: %+v", lease.Takeovers)
	}
	if lease.Pid != secondPid {
		t.Fatalf("the lease did not move to the second turn: holder pid %d, want %d", lease.Pid, secondPid)
	}
	data, err := os.ReadFile(job)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil || record["status"] != "pending" {
		t.Fatalf("a delegate left in flight across a turn boundary must survive: %s %v", data, err)
	}
}
