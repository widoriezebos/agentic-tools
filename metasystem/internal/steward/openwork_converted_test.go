package steward

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// convertedBed builds a migrated checkout: an enrolled repository whose
// accepted ref carries a valid root record and the given goal files —
// the world the open-work judgment must read after the migration.
func bedHistory(id, verb string) []goal.HistoryLine {
	return []goal.HistoryLine{{
		At:      "2026-08-23T00:00:00Z",
		Opid:    "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000",
		Verb:    verb,
		Actor:   "bed-m1+coordinator",
		Targets: []string{id},
		Keep:    -1,
	}}
}

func convertedBed(t *testing.T, machine string, files map[string]*goal.GoalFile) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "metasystem.goal.machine", machine)
	run("config", "goal.sync-remote", "local")
	run("config", "user.name", "steward-fixture")
	run("config", "user.email", "steward-fixture@example.invalid")

	rootRecord := &goal.RootRecord{
		Identity:      "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		FormatVersion: "1",
		SyncMode:      goal.SyncLocal,
		Revision:      1,
	}
	write := func(rel string, data []byte) {
		t.Helper()
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", rel)
	}
	write("plans/goals/backlog.md", goal.RenderRoot(rootRecord))
	for id, f := range files {
		write("plans/goals/"+id+".md", goal.RenderFile(f))
	}
	run("commit", "-q", "-m", "converted bed")
	run("update-ref", goal.AcceptedRef, "HEAD")
	return root
}

func liveProcessRecord(t *testing.T) map[string]any {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe fixture process: %v %v", state, err)
	}
	record := map[string]any{"pid": exact.Pid, "pidStartedAt": exact.StartedAt.Unix()}
	if exact.StartTicks > 0 && exact.BootID != "" {
		record["pidStartTicks"] = exact.StartTicks
		record["bootId"] = exact.BootID
	} else {
		record["pidStartedAtExactMicro"] = exact.StartedAt.UnixMicro()
	}
	return record
}

func writeStewardRecord(t *testing.T, path string, record map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConvertedClaimByThisMachineIsOwnedWork(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"fix-it": {
			Id: "fix-it", State: "claimed", Intent: "Repair it", Origin: "main",
			NextStep: "Do the repair.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
			History: bedHistory("fix-it", "claim"),
		},
	})
	announcement := liveProcessRecord(t)
	announcement["mainId"] = "main-fixture"
	announcement["ownerLineage"] = "coordinator"
	writeStewardRecord(t, filepath.Join(root, "artifacts", "agents", "mains", "fixture.json"), announcement)
	w, reason, err := ReadOpenWork(root)
	if err != nil || w != WorkInFlight || !strings.Contains(reason, "claim:fix-it") {
		t.Fatalf("a claim counts only when its matching process is live: %v %q %v", w, reason, err)
	}
}

func TestConvertedForeignClaimAndQueueIsNotOwnedHere(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"theirs": {
			Id: "theirs", State: "claimed", Intent: "Elsewhere", Origin: "main",
			NextStep: "Work elsewhere.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "bed-m2", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
			History: bedHistory("theirs", "claim"),
		},
		"waiting": {
			Id: "waiting", State: "queued", Intent: "Awaits a claim", Origin: "main",
			NextStep: "Work someday.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
			History: bedHistory("waiting", "open"),
		},
	})
	w, reason, err := ReadOpenWork(root)
	if err != nil || w != WorkNone || !strings.Contains(reason, "queued") {
		t.Fatalf("a foreign claim plus a queue is visible, never owned here: %v %q %v", w, reason, err)
	}
}

func TestConvertedUnenrolledMachineDegradesNeverGuesses(t *testing.T) {
	root := convertedBed(t, "bed-m1", nil)
	run := exec.Command("git", "-C", root, "config", "--unset", "metasystem.goal.machine")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("unset: %v\n%s", err, out)
	}
	w, reason, err := ReadOpenWork(root)
	if err != nil || w != WorkDegraded {
		t.Fatalf("no enrollment means no judgment — degraded, never no-work: %v %q %v", w, reason, err)
	}
}

func stewardBudget() *goal.Budget {
	budget := goal.Budget{
		ElapsedLimit: "4h", AttemptLimit: 4,
		ReservedJobMinutesLimit: 240, ActiveJobLimit: 2,
	}
	return &budget
}

func approvedStewardGoal(id, intent, next, openedAt string) *goal.GoalFile {
	budget := stewardBudget()
	approvedAt := "2026-08-23T00:01:00Z"
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "bed-m1", "approval")
	history := append(bedHistory(id, "open"), goal.HistoryLine{
		At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido",
		Targets: []string{id}, Keep: -1,
	})
	return &goal.GoalFile{
		Id: id, State: goal.StateApproved, Tier: 3, Intent: intent, Origin: goal.OriginMain,
		NextStep: next, OpenedAt: openedAt, Revision: 2, Budget: budget, History: history,
		Approved: &goal.ApprovalRecord{
			By: "human:Wido", At: approvedAt, Revision: 2, Opid: approvalOpid,
			Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 3, *budget),
		},
	}
}

func TestConvertedIdleBacklogIsDeadAndEscalatesEveryTick(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"waiting": approvedStewardGoal("waiting", "Awaits a claim", "Claim it.", "2026-08-23T00:00:00Z"),
		"stale-claim": {
			Id: "stale-claim", State: goal.StateClaimed, Intent: "A record is not liveness", Origin: "main",
			NextStep: "Continue it.", OpenedAt: "2026-08-22T00:00:00Z", Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
			History: bedHistory("stale-claim", "claim"),
		},
	})
	w, reason, err := ReadOpenWork(root)
	if err != nil || w != WorkClaimable || !strings.Contains(reason, "waiting") {
		t.Fatalf("claimable backlog must remain actionable: %v %q %v", w, reason, err)
	}
	for tick := 1; tick <= 2; tick++ {
		decision := Decide(Snapshot{Work: w})
		if decision.Verdict != VerdictIdleBacklogDead || decision.Action != ActNotify ||
			!strings.Contains(decision.Reason, "every steward tick") {
			t.Fatalf("runtime-independent tick %d must escalate idle backlog: %+v", tick, decision)
		}
	}
}

func TestConvertedJobsCountOnlyWithLiveProcessesAndPendingSetupAgrees(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"waiting": approvedStewardGoal("waiting", "Awaits a claim", "Claim it.", "2026-08-23T00:00:00Z"),
	})
	jobPath := filepath.Join(root, "artifacts", "agents", "jobs", "delegate.json")
	writeStewardRecord(t, jobPath, map[string]any{
		"jobId": "delegate", "status": "running", "pid": 999999, "pidStartedAt": 1,
	})
	if w, _, err := ReadOpenWork(root); err != nil || w != WorkClaimable {
		t.Fatalf("a stale running record must not suppress the escalation: %v %v", w, err)
	}
	live := liveProcessRecord(t)
	live["jobId"], live["status"] = "delegate", "running"
	writeStewardRecord(t, jobPath, live)
	if w, reason, err := ReadOpenWork(root); err != nil || w != WorkInFlight || !strings.Contains(reason, "job:delegate") {
		t.Fatalf("a live running process must count as in flight: %v %q %v", w, reason, err)
	}
	writeStewardRecord(t, jobPath, map[string]any{
		"jobId": "delegate", "status": "pending-setup", "creatorLiveness": liveProcessRecord(t),
	})
	if w, reason, err := ReadOpenWork(root); err != nil || w != WorkInFlight || !strings.Contains(reason, "job:delegate") {
		t.Fatalf("pending-setup must use its live creator in the shared predicate: %v %q %v", w, reason, err)
	}
}
