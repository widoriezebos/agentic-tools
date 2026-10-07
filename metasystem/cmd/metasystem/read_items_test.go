package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
)

func TestReadItemsFileSkipsBlanksAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.txt")
	if err := os.WriteFile(path, []byte("# read return\n\n  First item.  \r\n   # ignored\r\nSecond item.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := readItemsFile(path)
	if err != nil || !reflect.DeepEqual(items, []string{"First item.", "Second item."}) {
		t.Fatalf("items file = %q, %v", items, err)
	}
}

type readItemCommandFixture struct {
	repository   *proofAdmissionRepository
	dependencies syncRequestDependencies
	now          time.Time
	codeCommit   string
	reports      int
	t            *testing.T
}

func newReadItemCommandFixture(t *testing.T) *readItemCommandFixture {
	t.Helper()
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	codeCommit := repository.canonical
	repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
		file.ReadItems = []goal.ReadItem{
			{ID: "critic-1", Read: "critic", Text: "Name the boundary.", State: goal.ReadItemOpen, AddedAt: "2026-08-30T08:20:00Z"},
			{ID: "critic-2", Read: "critic", Text: "Already repaired.", State: goal.ReadItemFixed, AddedAt: "2026-08-30T08:20:00Z", ChangedAt: "2026-08-30T08:40:00Z", ClosingReference: codeCommit},
		}
	})
	return &readItemCommandFixture{repository: repository, dependencies: repository.extendBudgetInputs(t), now: now, codeCommit: codeCommit, t: t}
}

func (f *readItemCommandFixture) endpoint(root string) (goal.Endpoint, error) {
	return f.dependencies.endpoint(root)
}

func (f *readItemCommandFixture) projection() goal.Projection {
	f.t.Helper()
	endpoint, err := f.endpoint(f.repository.root)
	if err != nil {
		f.t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, f.now)
	if err != nil {
		f.t.Fatal(err)
	}
	return projection
}

func (f *readItemCommandFixture) resolveCodeCommit(root, ref string) (string, error) {
	if root != f.repository.root || ref != f.codeCommit {
		return "", fmt.Errorf("unexpected code reference root=%q ref=%q", root, ref)
	}
	f.repository.mu.Lock()
	defer f.repository.mu.Unlock()
	if _, err := f.repository.known(ref); err != nil {
		return "", err
	}
	return ref + "\n", nil
}

func (f *readItemCommandFixture) localTip(root, ref string) (string, bool, error) {
	if root != f.repository.root || ref != "refs/heads/goal/standing-validation" {
		return "", false, fmt.Errorf("unexpected local branch root=%q ref=%q", root, ref)
	}
	return "", false, nil
}

func (f *readItemCommandFixture) report(opts metrics.Options) (metrics.Result, error) {
	f.reports++
	if opts.Root != f.repository.root || opts.GoalID != "standing-validation" || opts.PeriodEnd != "" || opts.Since != "" {
		return metrics.Result{}, fmt.Errorf("unexpected metrics options: %+v", opts)
	}
	if f.projection().Tree.Done[opts.GoalID] == nil {
		return metrics.Result{}, fmt.Errorf("metrics ran before done publication")
	}
	return metrics.Result{}, nil
}

func (f *readItemCommandFixture) done(stdout, stderr io.Writer) int {
	trySync := func(name string, args []string) (int, bool) {
		return trySyncMutationWithCompletion(name, args, f.repository.commandNow(f.now), withStreams(f.dependencies, stdout, stderr), goalParkBranchCheck, completionInputs{localTip: f.localTip, reporter: f.report})
	}
	return runGoalDoneWithSync([]string{"--root", f.repository.root, "--id", "standing-validation", "--conclude", "Finished.", "--lineage", "m1"}, trySync)
}

func TestGoalShowAndNextPrintOpenReadItemFixUnit(t *testing.T) {
	t.Parallel()
	fixture := newReadItemCommandFixture(t)
	// Explicit next binds every ledger read to its cancellation allowance.
	resolve := fixture.dependencies.endpoint
	fixture.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		endpoint, err := resolve(root)
		endpoint.Repository = freshCommandRepository{Repository: endpoint.Repository, attempts: new(int)}
		return endpoint, err
	}
	root := fixture.repository.root
	showCode, show, showErr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalShowWithResolver([]string{"--root", root, "--id", "standing-validation"}, fixture.endpoint, stdout, stderr)
	})
	if showCode != 0 || showErr != "" || !strings.Contains(show, `"heading":"Open read items (fix unit critic): 1"`) || !strings.Contains(show, `"id":"critic-1"`) {
		t.Fatalf("goal show omitted read fix unit: code=%d output=%q stderr=%q", showCode, show, showErr)
	}
	nextCode, next, nextErr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalNextWithInputs([]string{"--root", root, "--machine", "mac-cli"}, withStreams(fixture.dependencies, stdout, stderr), fixture.repository.commandNow(fixture.now), stdout, stderr)
	})
	if nextCode != 0 || nextErr != "" || !strings.Contains(next, "continue your claimed goal: standing-validation\nOpen read items (fix unit critic): 1\n- critic-1: Name the boundary.\n") {
		t.Fatalf("goal next omitted block after selection: code=%d output=%q stderr=%q", nextCode, next, nextErr)
	}
}

func TestDoneReadItemRefusalRemedyExecutes(t *testing.T) {
	fixture := newReadItemCommandFixture(t)
	before := fixture.projection().Tip
	code, stdout, stderr := runOnOwnStreams(fixture.done)
	var refusal struct {
		Detail string `json:"detail"`
	}
	if code != 1 || stderr != "" || json.Unmarshal([]byte(stdout), &refusal) != nil {
		t.Fatalf("done refusal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if fixture.projection().Tip != before || fixture.reports != 0 {
		t.Fatalf("refused done changed accepted tip or ran metrics: reports=%d", fixture.reports)
	}
	// Line 2 names the goal's notes, which point at their close form; that
	// public form (goal notes G --close ITEM --fixed COMMIT) closes the item.
	remedy := "run: metasystem goal notes standing-validation"
	if !strings.Contains(refusal.Detail, remedy) || !strings.Contains(refusal.Detail, "critic-1") {
		t.Fatalf("done refusal lacks its notes remedy %q: %s", remedy, refusal.Detail)
	}
	command := "metasystem goal notes standing-validation --close critic-1 --fixed " + fixture.codeCommit
	fields := strings.Fields(command)
	// The printed public form (goal notes G --close ITEM --fixed COMMIT) runs
	// the read-items close owner with the same goal, item and closure.
	if len(fields) != 8 || fields[1] != "goal" || fields[2] != "notes" || fields[4] != "--close" {
		t.Fatalf("printed remedy is not the public goal notes form: %q", command)
	}
	closeArgs := []string{"--id", fields[3], "--item", fields[5], fields[6], fields[7], "--root", fixture.repository.root, "--lineage", "m1"}
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalReadItemsCloseWithInputs(closeArgs, fixture.repository.commandNow(fixture.now), withStreams(fixture.dependencies, stdout, stderr), fixture.resolveCodeCommit)
	})
	if code != 0 || stderr != "" {
		t.Fatalf("printed fixed remedy failed: command=%q code=%d stdout=%q stderr=%q", command, code, stdout, stderr)
	}
	closed := fixture.projection()
	if closed.Tip == before || closed.Tree.Live["standing-validation"].ReadItems[0].State != goal.ReadItemFixed || closed.Tree.Live["standing-validation"].ReadItems[0].ClosingReference != fixture.codeCommit || fixture.reports != 0 {
		t.Fatalf("printed fixed remedy did not close item: projection=%+v reports=%d", closed, fixture.reports)
	}
	code, stdout, stderr = runOnOwnStreams(fixture.done)
	if code != 0 || stderr != "" || fixture.reports != 1 {
		t.Fatalf("done after remedy: code=%d stdout=%q stderr=%q reports=%d", code, stdout, stderr, fixture.reports)
	}
	finished := fixture.projection()
	if finished.Tip == closed.Tip || finished.Tree.Done["standing-validation"] == nil {
		t.Fatalf("done did not advance accepted tip and archive the goal: tip=%q", finished.Tip)
	}
}
