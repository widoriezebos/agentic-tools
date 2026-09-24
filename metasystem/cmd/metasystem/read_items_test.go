package main

import (
	"encoding/json"
	"fmt"
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

func (f *readItemCommandFixture) done() int {
	trySync := func(name string, args []string) (int, bool) {
		return trySyncMutationWithCompletion(name, args, f.repository.commandNow(f.now), f.dependencies, goalParkBranchCheck, completionInputs{localTip: f.localTip, reporter: f.report})
	}
	return runGoalDoneWithSync([]string{"--root", f.repository.root, "--id", "standing-validation", "--conclude", "Finished.", "--lineage", "m1"}, trySync)
}

func TestGoalShowAndNextPrintOpenReadItemFixUnit(t *testing.T) {
	fixture := newReadItemCommandFixture(t)
	root := fixture.repository.root
	showCode, show, showErr := captureCommandOutput(t, true, true, func() int {
		return runGoalShowWithResolver([]string{"--root", root, "--id", "standing-validation"}, fixture.endpoint)
	})
	if showCode != 0 || showErr != "" || !strings.Contains(show, `"heading":"Open read items (fix unit critic): 1"`) || !strings.Contains(show, `"id":"critic-1"`) {
		t.Fatalf("goal show omitted read fix unit: code=%d output=%q stderr=%q", showCode, show, showErr)
	}
	nextCode, next, nextErr := captureCommandOutput(t, true, true, func() int {
		return runGoalNextWithInputs([]string{"--root", root, "--machine", "mac-cli"}, fixture.dependencies, fixture.repository.commandNow(fixture.now))
	})
	if nextCode != 0 || nextErr != "" || !strings.Contains(next, "continue your claimed goal: standing-validation\nOpen read items (fix unit critic): 1\n- critic-1: Name the boundary.\n") {
		t.Fatalf("goal next omitted block after selection: code=%d output=%q stderr=%q", nextCode, next, nextErr)
	}
}

func TestGoalReadItemsListJSONShape(t *testing.T) {
	fixture := newReadItemCommandFixture(t)
	code, output, stderr := captureCommandOutput(t, true, true, func() int {
		return runGoalReadItemsWithInputs([]string{"list", "--root", fixture.repository.root, "--open", "--json"}, fixture.repository.commandNow(fixture.now), fixture.dependencies, fixture.resolveCodeCommit)
	})
	var envelope struct {
		Tip   string              `json:"tip"`
		Goals []readItemsGoalJSON `json:"goals"`
	}
	if code != 0 || stderr != "" || json.Unmarshal([]byte(output), &envelope) != nil || envelope.Tip != fixture.projection().Tip || len(envelope.Goals) != 1 || envelope.Goals[0].Goal != "standing-validation" || envelope.Goals[0].State != goal.StateClaimed || len(envelope.Goals[0].Items) != 1 || envelope.Goals[0].Items[0].ID != "critic-1" {
		t.Fatalf("read-items list JSON shape: code=%d output=%q stderr=%q decoded=%+v", code, output, stderr, envelope)
	}
}

func TestDoneReadItemRefusalRemedyExecutes(t *testing.T) {
	fixture := newReadItemCommandFixture(t)
	before := fixture.projection().Tip
	code, stdout, stderr := captureCommandOutput(t, true, true, fixture.done)
	var refusal struct {
		Detail string `json:"detail"`
	}
	if code != 1 || stderr != "" || json.Unmarshal([]byte(stdout), &refusal) != nil {
		t.Fatalf("done refusal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if fixture.projection().Tip != before || fixture.reports != 0 {
		t.Fatalf("refused done changed accepted tip or ran metrics: reports=%d", fixture.reports)
	}
	template := "metasystem goal read-items close --id standing-validation --item critic-1 --fixed <commit>"
	if !strings.Contains(refusal.Detail, template) {
		t.Fatalf("done refusal lacks executable fixed remedy %q: %s", template, refusal.Detail)
	}
	printed := refusal.Detail[strings.Index(refusal.Detail, template):]
	printed = strings.SplitN(printed, " | ", 2)[0]
	command := strings.Replace(printed, "<commit>", fixture.codeCommit, 1)
	fields := strings.Fields(command)
	closeArgs := append(append([]string{}, fields[3:]...), "--root", fixture.repository.root, "--lineage", "m1")
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runGoalReadItemsWithInputs(closeArgs, fixture.repository.commandNow(fixture.now), fixture.dependencies, fixture.resolveCodeCommit)
	})
	if code != 0 || stderr != "" {
		t.Fatalf("printed fixed remedy failed: command=%q code=%d stdout=%q stderr=%q", command, code, stdout, stderr)
	}
	closed := fixture.projection()
	if closed.Tip == before || closed.Tree.Live["standing-validation"].ReadItems[0].State != goal.ReadItemFixed || closed.Tree.Live["standing-validation"].ReadItems[0].ClosingReference != fixture.codeCommit || fixture.reports != 0 {
		t.Fatalf("printed fixed remedy did not close item: projection=%+v reports=%d", closed, fixture.reports)
	}
	code, stdout, stderr = captureCommandOutput(t, true, true, fixture.done)
	if code != 0 || stderr != "" || fixture.reports != 1 {
		t.Fatalf("done after remedy: code=%d stdout=%q stderr=%q reports=%d", code, stdout, stderr, fixture.reports)
	}
	finished := fixture.projection()
	if finished.Tip == closed.Tip || finished.Tree.Done["standing-validation"] == nil {
		t.Fatalf("done did not advance accepted tip and archive the goal: tip=%q", finished.Tip)
	}
}
