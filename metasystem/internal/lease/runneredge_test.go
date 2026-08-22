package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeRunnerAnnouncement puts a valid runner-tagged announcement on disk so
// holderIsMissionRunner can authenticate the holder's role and identity.
func writeRunnerAnnouncement(t *testing.T, root, mainID string, pid, start int64, lineage, tag string) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "mains")
	os.MkdirAll(dir, 0o755)
	ann := map[string]any{
		"sessionId": "s-" + mainID, "mainId": mainID, "pid": pid, "pidStartedAt": start,
		"pgid": pid, "runtime": "metasystem", "instanceTag": tag,
		"commandHash": CommandHash("cmd-" + mainID), "announcedAt": "2026-08-18T00:00:00Z",
		"ownerLineage": lineage,
	}
	data, _ := json.Marshal(ann)
	path := filepath.Join(dir, fmt.Sprintf("s-%s-%d.json", mainID, pid))
	if err := os.WriteFile(path, append(data, byte(10)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The ONE exception to "a live holder never yields" — a
// mission's host turn, PROVEN a kernel descendant of the live runner,
// succeeds it (epoch preserved, revision advanced). Everything forgeable
// alone is insufficient: same lineage + non-runner tag from a NON-descendant
// is refused, as are foreign lineage and runner-tagged claimants.
func TestHostSucceedsLiveRunnerHolder(t *testing.T) {
	root := t.TempDir()
	// The RUNNER is this test process: the claimed lease and the on-disk
	// announcement carry its real kernel identity, and liveChild spawns
	// genuine kernel DESCENDANTS to play the host.
	runnerPid := int64(os.Getpid())
	runnerStart, ok := StartedAt(runnerPid, nil)
	if !ok {
		t.Fatal("cannot read own start")
	}
	lineage := "mission-bm-x"
	runnerMain := fmt.Sprintf("main-%d-%d-aaaaaa", runnerStart, runnerPid)
	writeRunnerAnnouncement(t, root, runnerMain, runnerPid, runnerStart, lineage, "mission-runner.sh")
	runner := ann(runnerMain, runnerPid, runnerStart, lineage)
	runner.InstanceTag = "mission-runner.sh"
	mustClaim(t, root, runner)
	lease, _ := loadLease(root, true)
	if lease.HolderMainId != runnerMain || lease.ClaimEpoch != 1 {
		t.Fatalf("runner claim wrong: %+v", lease)
	}

	// A foreign main (different lineage) is refused while the runner lives.
	foreignPid, foreignStart := liveChild(t)
	foreign := ann(fmt.Sprintf("main-%d-%d-cccccc", foreignStart, foreignPid), foreignPid, foreignStart, "other-owner")
	mustClaim(t, root, foreign)
	if lease, _ = loadLease(root, true); lease.HolderMainId != runnerMain {
		t.Fatalf("foreign main took a live runner's lease: %+v", lease)
	}

	// A runner-tagged DESCENDANT lawfully succeeds too — that is the
	// detached run loop succeeding its own foreground launcher.
	twinPid, twinStart := liveChild(t)
	twin := ann(fmt.Sprintf("main-%d-%d-dddddd", twinStart, twinPid), twinPid, twinStart, lineage)
	twin.InstanceTag = "mission-runner.sh"
	mustClaim(t, root, twin)
	if lease, _ = loadLease(root, true); lease.HolderMainId != twin.MainId {
		t.Fatalf("the runner's own loop child could not succeed it: %+v", lease)
	}
	// Fresh root for the host scenario: the twin now holds the first
	// one, and a parent does not descend from its child.
	root = t.TempDir()
	writeRunnerAnnouncement(t, root, runnerMain, runnerPid, runnerStart, lineage, "mission-runner.sh")
	mustClaim(t, root, runner)

	// The mission's own host turn — a REAL kernel child of the runner —
	// succeeds the live runner: epoch preserved (not a seizure), revision
	// advanced.
	hostPid, hostStart := liveChild(t)
	host := ann(fmt.Sprintf("main-%d-%d-bbbbbb", hostStart, hostPid), hostPid, hostStart, lineage)
	host.InstanceTag = "metasystem-host-turn"
	mustClaim(t, root, host)
	lease, _ = loadLease(root, true)
	if lease.HolderMainId != host.MainId {
		t.Fatalf("host did not succeed the live runner: %+v", lease)
	}
	if lease.ClaimEpoch != 1 || lease.Revision < 2 {
		t.Fatalf("succession signature wrong (epoch preserved, revision advanced): %+v", lease)
	}
}

// The impostor case: same lineage, non-runner tag, live —
// but NOT a kernel descendant of the holder. The edge must refuse.
func TestImpostorCannotStealLiveRunnerLease(t *testing.T) {
	root := t.TempDir()
	holderPid, holderStart := liveChild(t)
	lineage := "mission-bm-y"
	holderMain := fmt.Sprintf("main-%d-%d-aaaaaa", holderStart, holderPid)
	writeRunnerAnnouncement(t, root, holderMain, holderPid, holderStart, lineage, "mission-runner.sh")
	holder := ann(holderMain, holderPid, holderStart, lineage)
	holder.InstanceTag = "mission-runner.sh"
	mustClaim(t, root, holder)

	// The impostor is a SIBLING (child of the test, not of the holder):
	// every forgeable predicate matches, the kernel ancestry does not.
	impostorPid, impostorStart := liveChild(t)
	impostor := ann(fmt.Sprintf("main-%d-%d-eeeeee", impostorStart, impostorPid), impostorPid, impostorStart, lineage)
	impostor.InstanceTag = "metasystem-host-turn"
	mustClaim(t, root, impostor)
	lease, _ := loadLease(root, true)
	if lease.HolderMainId != holderMain {
		t.Fatalf("an impostor stole the live runner's lease: %+v", lease)
	}

	// A runner-TAGGED non-descendant is refused just the same: the tag
	// is text, the ancestry is the fence.
	twinPid, twinStart := liveChild(t)
	twin := ann(fmt.Sprintf("main-%d-%d-ffffff", twinStart, twinPid), twinPid, twinStart, lineage)
	twin.InstanceTag = "mission-runner.sh"
	mustClaim(t, root, twin)
	if lease, _ = loadLease(root, true); lease.HolderMainId != holderMain {
		t.Fatalf("a non-descendant twin runner took the lease: %+v", lease)
	}
}
