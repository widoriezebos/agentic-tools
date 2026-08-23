package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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

func TestConvertedClaimByThisMachineIsOwnedWork(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"fix-it": {
			Id: "fix-it", State: "claimed", Intent: "Repair it", Origin: "main",
			NextStep: "Appetite: 1h — do the repair.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
			History: bedHistory("fix-it", "claim"),
		},
	})
	w, reason, err := ReadOpenWork(root)
	if err != nil || w != WorkOwned || !strings.Contains(reason, "fix-it") {
		t.Fatalf("this machine's claim on a converted checkout is owned work: %v %q %v", w, reason, err)
	}
}

func TestConvertedForeignClaimAndQueueIsNotOwnedHere(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"theirs": {
			Id: "theirs", State: "claimed", Intent: "Elsewhere", Origin: "main",
			NextStep: "Appetite: 1h — elsewhere.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "bed-m2", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
			History: bedHistory("theirs", "claim"),
		},
		"waiting": {
			Id: "waiting", State: "queued", Intent: "Awaits a claim", Origin: "main",
			NextStep: "Appetite: 1h — someday.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
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
	_ = time.Now
}
