package lease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestCleanupStaleJobsFailsOnlyOlderInFlightJobs(t *testing.T) {
	savedPids, savedPgid, savedCmd, savedKill := sweepAllPids, sweepGetpgid, sweepProcessCommand, sweepKill
	defer func() {
		sweepAllPids, sweepGetpgid, sweepProcessCommand, sweepKill = savedPids, savedPgid, savedCmd, savedKill
	}()
	sweepAllPids = func() ([]int64, error) { return []int64{7}, nil }
	sweepGetpgid = func(pid int64) (int64, error) { return 999999, nil }
	sweepProcessCommand = func(pid int64, _ identity.FixtureProbe) (string, bool) { return "different-tag", true }
	sweepKill = func(pgid int64, sig unix.Signal) error {
		t.Fatal("a provably unowned group must not be signaled")
		return nil
	}

	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts/agents/jobs")
	// Stale, in-flight, and in a process group whose observed member does not
	// carry the tag, so the sweep marks it failed without killing anything.
	writeJSON(t, filepath.Join(jobs, "job-a.json"),
		`{"jobId":"job-a","claimEpoch":4,"status":"running","pgid":999999,"instanceTag":"tag-a"}`)
	// Current generation: not older than the sweep epoch, untouched.
	writeJSON(t, filepath.Join(jobs, "job-b.json"),
		`{"jobId":"job-b","claimEpoch":6,"status":"running"}`)
	// Already terminal: untouched.
	writeJSON(t, filepath.Join(jobs, "job-c.json"),
		`{"jobId":"job-c","claimEpoch":1,"status":"completed"}`)

	sweepClaimer, _ := newClaimer(root)
	if err := sweepClaimer.cleanupStaleJobs(6); err != nil {
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
	savedPids := sweepAllPids
	defer func() { sweepAllPids = savedPids }()

	self := os.Getpid()
	pgid, err := unix.Getpgid(self)
	if err != nil {
		t.Fatal(err)
	}
	command, ok := ProcessCommand(int64(self), nil)
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
		owned, provable := groupOwnsTag(int64(pgid), command, nil)
		if owned && provable {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("our group should own our command tag: owned=%v provable=%v", owned, provable)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// A tag no process carries is not owned (but still provable). Unlike
	// the own-tag scan, this one must inspect EVERY live pid — so any
	// process on the machine inside its fork-to-execve window makes the
	// sweep rightly unprovable for that instant. Same dossier mechanism,
	// same remedy: the property is steady-state, wait it out bounded.
	deadline = time.Now().Add(2 * time.Second)
	for {
		owned, provable := groupOwnsTag(int64(pgid), "no-process-carries-this-xyzzy", nil)
		if !owned && provable {
			break
		}
		if owned {
			t.Fatalf("absent tag reads as owned")
		}
		if time.Now().After(deadline) {
			t.Fatalf("absent tag never became provable: owned=%v provable=%v", owned, provable)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// An empty member scan contains no observation that could disprove
	// ownership.
	sweepAllPids = func() ([]int64, error) { return nil, nil }
	if owned, provable := groupOwnsTag(int64(pgid), command, nil); owned || provable {
		t.Fatalf("an empty member scan must be unprovable: owned=%v provable=%v", owned, provable)
	}
}

func TestSweepStaleContinuesPastUnprovableOwnership(t *testing.T) {
	root := t.TempDir()
	runs := filepath.Join(root, "artifacts", "agents", "runs")
	writeJSON(t, filepath.Join(runs, "a-unprovable.json"),
		`{"schemaVersion":1,"runId":"a-unprovable","kind":"suite","display":"first stale run","custody":"wrapped","generation":1,"pid":101,"pidStartedAt":1,"pgid":101,"launchNonce":"11111111111111111111111111111111","log":"a.log","startedAt":"2026-08-30T10:00:00Z","claimEpoch":1,"staleAfterMin":30,"windDownMin":10,"evidence":{"mode":"exit-sidecar"},"expect":{},"status":"running"}`)
	writeJSON(t, filepath.Join(runs, "b-owned.json"),
		`{"schemaVersion":1,"runId":"b-owned","kind":"suite","display":"second stale run","custody":"wrapped","generation":1,"pid":202,"pidStartedAt":1,"pgid":202,"launchNonce":"22222222222222222222222222222222","log":"b.log","startedAt":"2026-08-30T11:00:00Z","claimEpoch":1,"staleAfterMin":30,"windDownMin":10,"evidence":{"mode":"exit-sidecar"},"expect":{},"status":"running"}`)

	store := &run.Store{
		Root:         root,
		AllPids:      func() ([]int64, error) { return nil, nil },
		Getpgid:      func(pid int64) (int64, error) { return pid, nil },
		GroupPresent: func(pgid int64) (bool, bool) { return false, true },
	}
	var killed []int64
	err := store.SweepStale(2,
		func(pgid int64, nonce string) (bool, bool) {
			if pgid == 101 {
				return false, false
			}
			return true, true
		},
		func(pgid int64) error { killed = append(killed, pgid); return nil })
	if err == nil || !strings.Contains(err.Error(), "a-unprovable group ownership scan unprovable") {
		t.Fatalf("the unprovable scan must surface by its own reason: %v", err)
	}
	if len(killed) != 1 || killed[0] != 202 {
		t.Fatalf("the later owned group must still be swept: %v", killed)
	}
	first, readErr := store.Read("a-unprovable")
	if readErr != nil || first.Status != run.StatusRunning {
		t.Fatalf("the unprovable run must remain untouched: record=%+v err=%v", first, readErr)
	}
	second, readErr := store.Read("b-owned")
	if readErr != nil || second.Status != run.StatusEndedUnknown {
		t.Fatalf("the later owned run must be concluded: record=%+v err=%v", second, readErr)
	}
}

func TestStopStaleGroupSkipsWithoutPgidOrTag(t *testing.T) {
	c, _ := newClaimer(t.TempDir())
	// No pgid, no tag: nothing to prove or kill.
	if err := c.stopStaleGroup(map[string]any{}, "job-x"); err != nil {
		t.Fatalf("a job without a group should not error: %v", err)
	}
	// pgid <= 1 is never a killable group.
	if err := c.stopStaleGroup(map[string]any{"pgid": float64(1), "instanceTag": "t"}, "job-x"); err != nil {
		t.Fatalf("pgid 1 should be skipped: %v", err)
	}
}

func TestCleanupStaleJobsHonorsTheCancellingMarker(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts/agents/jobs")
	// A cancel marked this record, then its holder died before the
	// conclude; the takeover's sweep must FINISH that cancel — a
	// marked record's only lawful conclusions are cancelled and a
	// genuine completion — while still clearing the work for the new
	// generation.
	// No pgid on either record: stopStaleGroup no-ops below 2 by
	// contract, so this fixture proves the CONCLUSION rule without
	// ever touching the host's process table — no seam rebinding,
	// no group/tag collision risk, no transient scan refusal.
	writeJSON(t, filepath.Join(jobs, "job-marked.json"),
		`{"jobId":"job-marked","claimEpoch":4,"status":"running","phase":"cancelling","instanceTag":"tag-m"}`)
	// The unmarked stale sibling still fails exactly as before.
	writeJSON(t, filepath.Join(jobs, "job-plain.json"),
		`{"jobId":"job-plain","claimEpoch":4,"status":"running","instanceTag":"tag-p"}`)

	sweepClaimer, _ := newClaimer(root)
	if err := sweepClaimer.cleanupStaleJobs(6); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	m, _ := readObject(filepath.Join(jobs, "job-marked.json"))
	if m["status"] != "cancelled" || m["error"] != nil {
		t.Fatalf("the takeover finishes the predecessor's cancel: %v", m)
	}
	if m["staleClaimEpoch"] != true {
		t.Fatalf("the stale-claim evidence stays on the record: %v", m)
	}
	if m["endedAt"] == nil || m["endedAt"] == "" {
		t.Fatalf("the conclusion is stamped: %v", m)
	}
	p, _ := readObject(filepath.Join(jobs, "job-plain.json"))
	if p["status"] != "failed" || p["error"] != "stale-claim-epoch" {
		t.Fatalf("an unmarked stale job still fails: %v", p)
	}
}
