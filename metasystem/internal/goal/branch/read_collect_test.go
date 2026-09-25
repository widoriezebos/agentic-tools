package branch_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// A goal branch read through an injected repository keeps one journal per
// unit: nothing is collected before a critic root is dispatched, a gate run
// that was never recorded is run again, a closed critic is committed exactly
// once when collection is asked for, and a repeat reports the recorded
// commit instead of committing a second time. A journal whose unit tree no
// longer matches is refused even after collection.
func TestGLEBranchReadCollectsAClosedCriticOnceThroughItsJournal(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	gates, delegates, commits := 0, 0, 0
	ids := []error{errors.New("declared id failure"), nil}
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: r.unit,
		GoalID: "goal-a", UnitCommit: r.unit, Repository: r, CheckClaim: claimAllowed,
		NewID: func(prefix string) (string, error) {
			err := ids[0]
			ids = ids[1:]
			return prefix + "-collect", err
		},
		Delegate: func(brief, goal, commit, runtime, model string) (string, error) {
			delegates++
			writeReadJobWithSubject(t, r.root, "critic-collect", r.unit, "running", false, r.readSubject())
			return "critic-collect", nil
		},
		Commit: func(branch.CommitReadRequest) (string, branch.Attestation, error) {
			t.Fatal("committed before the critic was collected")
			return "", branch.Attestation{}, nil
		},
	}
	unchanged := func(step string, wantGates int) {
		t.Helper()
		if gates != wantGates || delegates != 0 || commits != 0 {
			t.Fatalf("%s: gates=%d delegates=%d commits=%d", step, gates, delegates, commits)
		}
		if _, err := os.Stat(gleBranchReadRecordPath(t, r.root, r.unit)); !os.IsNotExist(err) {
			t.Fatalf("%s wrote the journal: %v", step, err)
		}
	}

	request.Collect = true
	r.expectStart()
	var refusal *branch.OpError
	if _, err := branch.RunBranchRead(request); !errors.As(err, &refusal) || refusal.Code != branch.ReadInvalidCode || !strings.Contains(err.Error(), "no critic root") {
		t.Fatalf("collect before dispatch: %v", err)
	}
	unchanged("collect before dispatch", 0)

	request.Collect = false
	r.expectStart()
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "no gate command") {
		t.Fatalf("read without a gate: %v", err)
	}
	unchanged("read without a gate", 0)

	request.Gate = func(string) (string, error) { gates++; return "go gate: fast mode passed", nil }
	workspaceErr := errors.New("declared workspace failure")
	r.expectStart()
	r.expect(readFactCall{method: "Detached", args: []string{r.root, r.unit}, err: workspaceErr})
	if _, err := branch.RunBranchRead(request); !errors.Is(err, workspaceErr) {
		t.Fatalf("detached workspace failure: %v", err)
	}
	unchanged("detached workspace failure", 0)

	r.expectStart()
	r.expect(readFactCall{method: "Detached", args: []string{r.root, r.unit}})
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "declared id failure") {
		t.Fatalf("unrecorded gate run: %v", err)
	}
	unchanged("unrecorded gate run", 1)

	r.expectStart()
	r.expectGateAndBrief()
	dispatched, err := branch.RunBranchRead(request)
	if err != nil || dispatched.State != "dispatched" || dispatched.RootJob != "critic-collect" || dispatched.GateRunID != "goal-read-gate-collect" || gates != 2 || delegates != 1 {
		t.Fatalf("dispatch=%+v gates=%d delegates=%d err=%v", dispatched, gates, delegates, err)
	}

	writeJSONFixture(t, r.root, "artifacts/agents/jobs/critic-collect.json", map[string]any{"jobId": "critic-collect", "role": "builder", "status": "completed"})
	request.Collect = true
	r.expectStart()
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "not a code-critic root") {
		t.Fatalf("collect of a non-critic root: %v", err)
	}

	writeReadJobWithSubject(t, r.root, "critic-collect", r.unit, "completed", false, r.readSubject())
	request.Collect = false
	r.expectStart()
	if closed, err := branch.RunBranchRead(request); err != nil || closed.State != "closed" || closed.RootJob != "critic-collect" || closed.AttestationCommit != "" {
		t.Fatalf("closed critic without collection=%+v err=%v", closed, err)
	}

	request.Collect = true
	entriesErr := errors.New("declared entries failure")
	r.expectStart()
	r.expect(readFactCall{method: "Entries", args: []string{r.root, r.unit}, err: entriesErr})
	if _, err := branch.RunBranchRead(request); !errors.Is(err, entriesErr) {
		t.Fatalf("collect without the unit's entries: %v", err)
	}

	installed := strings.Repeat("9", 40)
	request.Commit = func(got branch.CommitReadRequest) (string, branch.Attestation, error) {
		commits++
		want := branch.CommitReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, GoalID: "goal-a", Units: []string{"u1"},
			OpID: "goal-read-gate-collect-collect", RootJob: "critic-collect", GateRunID: "goal-read-gate-collect", GateTree: r.readSubject().Tree,
			TestsChanged: []branch.TestChange{{Path: "metasystem/internal/a/a_test.go", ReaderWord: "reviewed by critic root critic-collect"}}}
		got.CheckClaim = nil
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("collect request=%+v\nwant=%+v", got, want)
		}
		return installed, branch.Attestation{}, nil
	}
	r.expectStart()
	r.expect(readFactCall{method: "Entries", args: []string{r.root, r.unit}, entries: []branch.Entry{
		{Path: "metasystem/internal/a/a.go", SrcBlob: strings.Repeat("1", 40)},
		{Path: "metasystem/internal/a/a_test.go", SrcBlob: strings.Repeat("2", 40)},
		{Path: "metasystem/internal/a/new_test.go", SrcBlob: strings.Repeat("0", 40)},
	}})
	collected, err := branch.RunBranchRead(request)
	if err != nil || collected.State != "collected" || collected.AttestationCommit != installed || commits != 1 {
		t.Fatalf("collect=%+v commits=%d err=%v", collected, commits, err)
	}

	for _, collect := range []bool{true, false} {
		request.Collect = collect
		r.expectStart()
		again, err := branch.RunBranchRead(request)
		if err != nil || again.State != "already-collected" || again.AttestationCommit != installed || again.RootJob != "critic-collect" || commits != 1 || gates != 2 || delegates != 1 {
			t.Fatalf("repeat collect=%v result=%+v commits=%d gates=%d delegates=%d err=%v", collect, again, commits, gates, delegates, err)
		}
	}

	changed := r.attestationSubject()
	changed.Tree = strings.Repeat("7", 40)
	r.expect(readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()},
		readFactCall{method: "Subject", args: []string{r.root, r.unit}, subject: changed},
		readFactCall{method: "CommonDir", args: []string{r.root}})
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "does not match the requested unit") || commits != 1 {
		t.Fatalf("changed unit tree: commits=%d err=%v", commits, err)
	}
}

// A journal that is not exactly the recorded schema is refused before a
// gate runs, by the read and by the commit-time gate resolution alike.
func TestGLEBranchReadRefusesAMalformedJournalBeforeTheGate(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	path := gleBranchReadRecordPath(t, r.root, r.unit)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	malformed := []byte(`{"schemaVersion":1,"goal":"goal-a","unexpected":true}`)
	if err := os.WriteFile(path, malformed, 0o600); err != nil {
		t.Fatal(err)
	}
	gates, delegates := 0, 0
	gate := func(string) (string, error) { gates++; return "green", nil }
	r.expectStart()
	_, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: r.root, Repository: r, EndpointTip: r.base, BranchTip: r.unit,
		GoalID: "goal-a", UnitCommit: r.unit, CheckClaim: claimAllowed, Gate: gate,
		NewID:    func(string) (string, error) { return "id", nil },
		Delegate: func(string, string, string, string, string) (string, error) { delegates++; return "job", nil }})
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("read of a malformed journal: %v", err)
	}
	r.expect(readFactCall{method: "Subject", args: []string{r.root, r.unit}, subject: r.attestationSubject()},
		readFactCall{method: "CommonDir", args: []string{r.root}})
	if _, err := branch.ResolveReadGate(branch.ReadGateRequest{Repo: r.root, GoalID: "goal-a", UnitCommit: r.unit, Repository: r, Gate: gate}); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("gate resolution of a malformed journal: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(malformed) || gates != 0 || delegates != 0 {
		t.Fatalf("journal=%q gates=%d delegates=%d err=%v", data, gates, delegates, err)
	}
}
