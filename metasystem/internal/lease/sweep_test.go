package lease

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCleanupStaleJobsFailsOnlyOlderInFlightJobs(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts/agents/jobs")
	// Stale, in-flight, and in a process group we do not own (999999 matches
	// nothing) so the sweep marks it failed without killing anything.
	writeJSON(t, filepath.Join(jobs, "job-a.json"),
		`{"jobId":"job-a","claimEpoch":4,"status":"running","pgid":999999,"instanceTag":"tag-a"}`)
	// Current generation: not older than the sweep epoch, untouched.
	writeJSON(t, filepath.Join(jobs, "job-b.json"),
		`{"jobId":"job-b","claimEpoch":6,"status":"running"}`)
	// Already terminal: untouched.
	writeJSON(t, filepath.Join(jobs, "job-c.json"),
		`{"jobId":"job-c","claimEpoch":1,"status":"completed"}`)

	if err := newClaimer(root).cleanupStaleJobs(6); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	a, _ := readObject(filepath.Join(jobs, "job-a.json"))
	if a["status"] != "failed" || a["phase"] != "claim-sweep" || a["error"] != "stale-claim-epoch" {
		t.Fatalf("stale job should be failed by the sweep: %v", a)
	}
	b, _ := readObject(filepath.Join(jobs, "job-b.json"))
	if b["status"] != "running" {
		t.Fatalf("current-epoch job must be untouched: %v", b)
	}
	c, _ := readObject(filepath.Join(jobs, "job-c.json"))
	if c["status"] != "completed" {
		t.Fatalf("terminal job must be untouched: %v", c)
	}
}

func TestGroupOwnsTag(t *testing.T) {
	self := os.Getpid()
	pgid, err := unix.Getpgid(self)
	if err != nil {
		t.Fatal(err)
	}
	command, ok := ProcessCommand(int64(self))
	if !ok {
		t.Skip("cannot read our own command")
	}
	// Our own group carries our own command, so it is proven owned. The
	// group scan can transiently fail to read argvs under nested-gate load
	// (the execve/report window the flake dossier root-caused for the
	// other child-probing tests); the property is steady-state, so the
	// assertion waits it out, bounded.
	deadline := time.Now().Add(2 * time.Second)
	for {
		owned, provable := groupOwnsTag(int64(pgid), command)
		if owned && provable {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("our group should own our command tag: owned=%v provable=%v", owned, provable)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// A tag no process carries is not owned (but still provable).
	if owned, provable := groupOwnsTag(int64(pgid), "no-process-carries-this-xyzzy"); owned || !provable {
		t.Fatalf("absent tag should not be owned: owned=%v provable=%v", owned, provable)
	}
}

func TestStopStaleGroupSkipsWithoutPgidOrTag(t *testing.T) {
	c := newClaimer(t.TempDir())
	// No pgid, no tag: nothing to prove or kill.
	if err := c.stopStaleGroup(map[string]any{}, "job-x"); err != nil {
		t.Fatalf("a job without a group should not error: %v", err)
	}
	// pgid <= 1 is never a killable group.
	if err := c.stopStaleGroup(map[string]any{"pgid": float64(1), "instanceTag": "t"}, "job-x"); err != nil {
		t.Fatalf("pgid 1 should be skipped: %v", err)
	}
}
