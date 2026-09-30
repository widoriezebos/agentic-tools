package batchowner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// K6 (design r10): a goal joins the lane by custody handover and enqueue
// only. The production join's admission starts no test run: its execution
// moved behind landing prove, and the member joins with its admission tree
// recorded and deferred.
//
// Not parallel: it replaces the join admission's test executable.
func TestJoinRunsNoTests(t *testing.T) {
	dir := t.TempDir()
	checkout := filepath.Join(dir, "lane")
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", checkout, "-c", "user.name=Seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "init", "-q", "-b", "main", checkout).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for path, content := range map[string]string{".gitignore": "/artifacts/\n", "metasystem/metasystem.conf": "metasystem.template=true\n", "metasystem/app/base.txt": "base\n"} {
		full := filepath.Join(checkout, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	baseTree := git("rev-parse", "HEAD^{tree}")
	git("checkout", "-q", "-b", "goal/joiner")
	if err := os.WriteFile(filepath.Join(checkout, "metasystem", "app", "joiner.txt"), []byte("joiner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "build")
	build := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	digest, err := goalbranch.UnitDigest(checkout, build)
	if err != nil {
		t.Fatal(err)
	}

	store := batch.NewStore(checkout, nil)
	const id = "01j5x00000000000000000jn01"
	if err := store.Create(batch.Record{Schema: 1, BatchID: id, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen}); err != nil {
		t.Fatal(err)
	}
	unit := batch.BindBranchMember(batch.Unit{GoalID: "joiner", Chain: build, SeatRoot: "/seat",
		Claim: batch.Claim{Machine: "m1e", Lineage: "seat", Epoch: 1, Revision: 2, AccountingRevision: 2}},
		batch.BranchMember{GoalID: "joiner", Tip: build, Last: true, Builds: []batch.BranchBuild{{Units: []string{"u1"}, Commit: build, Digest: digest}}})

	executed := 0
	saved := batchJoinAdmissionExecutable
	batchJoinAdmissionExecutable = func() (string, error) {
		executed++
		return "", errors.New("the join started a test run")
	}
	t.Cleanup(func() { batchJoinAdmissionExecutable = saved })

	dependencies := ProductionBatchJoinDependencies()
	// The selection is recorded at join (a plan, not a test run); the
	// handover is the source seat's (K7).
	plan := func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{SelectedGroups: []string{"app-standard"}}, nil
	}
	handedOver := false
	run := func(batchID string, joined batch.Unit) (batch.JoinAdmission, error) {
		return dependencies.AdmissionRun(checkout, batchID, joined)
	}
	if err := dependencies.PublishAdmission(store, id, unit, "m1e+seat", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), plan,
		func() error { handedOver = true; return nil }, run); err != nil {
		t.Fatalf("join: %v", err)
	}
	record, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if executed != 0 {
		t.Fatalf("the join started %d test runs; want none", executed)
	}
	if len(record.Units) != 1 || record.Units[0].State != batch.UnitJoined || record.Units[0].Admission == nil ||
		record.Units[0].Admission.Status != batch.AdmissionDeferred || record.Units[0].Admission.Tree == "" || !handedOver {
		t.Fatalf("joined member = %+v (handed over %v); want joined, its admission tree recorded and deferred to prove", record.Units, handedOver)
	}
}
