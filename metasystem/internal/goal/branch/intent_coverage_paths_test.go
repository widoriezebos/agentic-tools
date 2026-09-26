package branch_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The landing-red classifier's authority edges. A red landing proof is either
// the goal's own failure or a trunk-red hold, and each classification must be
// backed by the owner that records it: a missing owner, a failing owner, or a
// diagnostic without an attempt refuses the classification instead of
// guessing one. These tests reuse the fakes of red_test.go.

type failingRedRunner struct{ err error }

func (f failingRedRunner) Run(branch.DiagnosticRun) (branch.DiagnosticResult, error) {
	return branch.DiagnosticResult{}, f.err
}

type failingTrunkRed struct{ err error }

func (f failingTrunkRed) RecordTrunkRed(branch.TrunkRedEntry) error { return f.err }

type failingLandingProgress struct{ err error }

func (f failingLandingProgress) RecordLandingProgress(string, string) error { return f.err }

// A request that does not name the goal, exact endpoint, branch tip, proof and
// failing groups is refused before any group is classified or any diagnostic
// is admitted.
func TestLandingRedRefusesAnIncompleteRequestBeforeAdmission(t *testing.T) {
	t.Parallel()

	for name, spoil := range map[string]func(*branch.RedRequest){
		"no goal":              func(r *branch.RedRequest) { r.Goal = "" },
		"a short endpoint":     func(r *branch.RedRequest) { r.Endpoint = "abc" },
		"a short branch tip":   func(r *branch.RedRequest) { r.BranchTip = "abc" },
		"proof zero":           func(r *branch.RedRequest) { r.Proof.Number = 0 },
		"a short candidate":    func(r *branch.RedRequest) { r.Proof.Candidate = "abc" },
		"a short landing":      func(r *branch.RedRequest) { r.Proof.Landing = "abc" },
		"no failing groups":    func(r *branch.RedRequest) { r.FailingGroups = nil },
		"an unknown red group": func(r *branch.RedRequest) { r.FailingGroups = []string{"nobody"} },
	} {
		admitted := false
		runner := &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "must-not-run"}}
		req := redRequest(runner, &fakeTrunkRed{}, &fakeLandingProgress{})
		req.AdmitDiagnostic = func() error { admitted = true; return nil }
		spoil(&req)
		result, err := branch.HandleLandingRed(req)
		if err == nil || result.Classification != "" || admitted || len(runner.runs) != 0 {
			t.Errorf("%s: result %+v err %v admitted %v runs %d; want a refusal before admission", name, result, err, admitted, len(runner.runs))
		}
	}
}

// A goal-owned classification is only as good as its progress record: without
// the recorder, or when the recorder fails, no classification is returned.
func TestLandingRedGoalClassificationRequiresItsProgressRecord(t *testing.T) {
	t.Parallel()

	req := redRequest(&fakeRedRunner{}, &fakeTrunkRed{}, nil)
	req.FailingGroups = []string{"owned"}
	if result, err := branch.HandleLandingRed(req); err == nil || result.Classification != "" {
		t.Fatalf("an owned red with no progress recorder = %+v, %v; want a refusal", result, err)
	}

	unwritable := errors.New("ledger is read-only")
	req.Progress = failingLandingProgress{err: unwritable}
	if result, err := branch.HandleLandingRed(req); !errors.Is(err, unwritable) || result.Classification != "" {
		t.Fatalf("an owned red whose record failed = %+v, %v; want the record's error", result, err)
	}
	if err := branch.RecordLandingGreen(nil, "goal-a", "u3", req.Proof); err == nil {
		t.Fatalf("a green landing with no progress recorder was recorded")
	}
}

// A foreign red needs its diagnostic owners in order: admission, runner, a
// real attempt id, and then the trunk-red ledger. A missing or failing owner
// at each step refuses the classification, and no later owner is reached.
func TestLandingRedDiagnosticRefusesWithoutEachOwner(t *testing.T) {
	t.Parallel()

	runFailed := errors.New("runner lost its claim")
	ledgerFailed := errors.New("trunk-red ledger is read-only")
	progressFailed := errors.New("progress ledger is read-only")
	cases := map[string]struct {
		spoil func(*branch.RedRequest)
		want  error
	}{
		"no admission": {spoil: func(r *branch.RedRequest) { r.AdmitDiagnostic = nil }},
		"no runner":    {spoil: func(r *branch.RedRequest) { r.Runner = nil }},
		"a failed run": {spoil: func(r *branch.RedRequest) { r.Runner = failingRedRunner{err: runFailed} }, want: runFailed},
		"a run with no attempt": {spoil: func(r *branch.RedRequest) {
			r.Runner = &fakeRedRunner{result: branch.DiagnosticResult{Green: true}}
		}},
		"a green endpoint whose progress fails": {spoil: func(r *branch.RedRequest) {
			r.Runner = &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "endpoint-green", Green: true}}
			r.Progress = failingLandingProgress{err: progressFailed}
		}, want: progressFailed},
		"a red endpoint with no trunk-red ledger": {spoil: func(r *branch.RedRequest) { r.TrunkRed = nil }},
		"a red endpoint whose ledger fails": {spoil: func(r *branch.RedRequest) {
			r.TrunkRed = failingTrunkRed{err: ledgerFailed}
		}, want: ledgerFailed},
		"a red endpoint whose progress fails": {spoil: func(r *branch.RedRequest) {
			r.Progress = failingLandingProgress{err: progressFailed}
		}, want: progressFailed},
	}
	for name, c := range cases {
		req := redRequest(&fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "endpoint-red"}}, &fakeTrunkRed{}, &fakeLandingProgress{})
		c.spoil(&req)
		result, err := branch.HandleLandingRed(req)
		if err == nil || result.Classification != "" {
			t.Errorf("%s: result %+v err %v; want a refusal with no classification", name, result, err)
			continue
		}
		var refusal *branch.OpError
		if errors.As(err, &refusal) && refusal.Code == branch.LandTrunkRedCode {
			t.Errorf("%s: answered the trunk-red hold %v; want the owner's refusal", name, err)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: err %v; want %v", name, err, c.want)
		}
	}
}

// A failing group belongs to the goal when one of its manifest inputs names a
// changed path exactly, names the directory itself through "/**", or matches
// it as a glob; a path the inputs only resemble does not make it the goal's.
func TestLandingRedOwnershipFollowsEveryManifestPatternForm(t *testing.T) {
	t.Parallel()

	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "exact", Inputs: []string{"./metasystem/go.mod"}},
		{ID: "tree", Inputs: []string{"metasystem/internal/goal/**"}},
		{ID: "glob", Inputs: []string{"metasystem/scripts/*.sh"}},
	}}
	for name, c := range map[string]struct {
		group, changed string
		owned          bool
	}{
		"an exact input":                  {group: "exact", changed: "metasystem/go.mod", owned: true},
		"a tree input naming its root":    {group: "tree", changed: "metasystem/internal/goal", owned: true},
		"a tree input naming a file":      {group: "tree", changed: "metasystem/internal/goal/branch/red.go", owned: true},
		"a tree input's sibling prefix":   {group: "tree", changed: "metasystem/internal/goalkeeper/x.go", owned: false},
		"a glob input":                    {group: "glob", changed: "metasystem/scripts/run.sh", owned: true},
		"a glob input one directory down": {group: "glob", changed: "metasystem/scripts/lib/run.sh", owned: false},
	} {
		runner := &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "endpoint-green", Green: true}}
		req := redRequest(runner, &fakeTrunkRed{}, &fakeLandingProgress{})
		req.Contract, req.FailingGroups, req.ChangeSet = contract, []string{c.group}, []string{c.changed}
		result, err := branch.HandleLandingRed(req)
		if err != nil || result.Classification != "goal-red" {
			t.Errorf("%s: result %+v err %v; want goal-red", name, result, err)
			continue
		}
		if ranDiagnostic := len(runner.runs) == 1; ranDiagnostic == c.owned {
			t.Errorf("%s: diagnostic ran %v; want it to run only for a foreign red", name, ranDiagnostic)
		}
	}
}

// An endpoint tip read that answered is still refused when its disposable ref
// cannot be deleted, because a check that leaves refs behind accumulates one
// per park; a failed fetch keeps its own error over the cleanup's.
func TestEndpointTipAnswersTheCleanupErrorOnlyAfterASuccessfulRead(t *testing.T) {
	t.Parallel()

	fetchFailed := errors.New("transport unavailable")
	cleanupFailed := errors.New("ref lock held")
	for name, c := range map[string]struct {
		fetchErr, want error
	}{
		"a read that answered": {want: cleanupFailed},
		"a fetch that failed":  {fetchErr: fetchFailed, want: fetchFailed},
	} {
		tip, err := branch.EndpointTipWithGit("/repo",
			goal.Endpoint{Root: "/repo", Remote: "origin", Branch: "refs/heads/main"},
			func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "fetch":
					return "", c.fetchErr
				case "update-ref":
					return "", cleanupFailed
				}
				return strings.Repeat("b", 40), nil
			})
		if !errors.Is(err, c.want) {
			t.Errorf("%s: tip %q err %v; want %v", name, tip, err, c.want)
		}
	}
}
