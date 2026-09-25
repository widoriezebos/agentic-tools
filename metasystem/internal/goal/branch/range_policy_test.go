package branch

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

const rangeRepo = "range-repo"

type rangeStubReporter struct {
	t        *testing.T
	messages []string
}

func (r *rangeStubReporter) Helper()                { r.t.Helper() }
func (r *rangeStubReporter) Cleanup(cleanup func()) { r.t.Cleanup(cleanup) }
func (r *rangeStubReporter) Errorf(format string, args ...any) {
	r.messages = append(r.messages, fmt.Sprintf(format, args...))
}

func rangeOutput(stdout string, args ...string) testgit.Expectation {
	return testgit.Expectation{
		Call:   testgit.Call{Dir: rangeRepo, Args: args},
		Result: testgit.Result{Stdout: []byte(stdout)},
	}
}

func rangeFailure(err error, args ...string) testgit.Expectation {
	return testgit.Expectation{
		Call:   testgit.Call{Dir: rangeRepo, Args: args},
		Result: testgit.Result{Err: err},
	}
}

func rangeReader(stub *testgit.Stub) func(string, ...string) ([]byte, error) {
	return func(repo string, args ...string) ([]byte, error) {
		result := stub.Run(testgit.Call{Dir: repo, Args: args})
		return result.Stdout, result.Err
	}
}

func rangeTranscript(endpoint, tip, base, rows string) []testgit.Expectation {
	return []testgit.Expectation{
		rangeOutput(base+"\n", "merge-base", endpoint, tip),
		rangeOutput(rows, "rev-list", "--first-parent", "--reverse", "--parents", base+".."+tip),
	}
}

func singleCommitTranscript() []testgit.Expectation {
	return rangeTranscript("base", "tip", "base", "tip base\n")
}

func rawTree(paths ...string) string {
	var out strings.Builder
	for _, path := range paths {
		out.WriteString(":000000 100644 0000000 abcdef0 A\x00")
		out.WriteString(path)
		out.WriteByte(0)
	}
	return out.String()
}

func treeCalls(commit, parent, raw string) []testgit.Expectation {
	return []testgit.Expectation{
		rangeOutput(commit+" "+parent+"\n", "rev-list", "--parents", "-n", "1", commit),
		rangeOutput(raw, "diff-tree", "-r", "-z", "--no-renames", "--full-index", commit+"^", commit),
	}
}

func validateWithTranscript(t *testing.T, endpoint, tip string, calls ...testgit.Expectation) ([]Commit, error) {
	t.Helper()
	stub := testgit.New(t, calls...)
	return validateRangeWithGit(rangeRepo, endpoint, tip, "goal-a", rangeReader(stub))
}

func TestValidateRangeRefusals(t *testing.T) {
	t.Parallel()
	hexA, hexB := strings.Repeat("a", 40), strings.Repeat("b", 40)
	unit := "Goal-Unit: goal-a/u\n"
	plan := "Goal-Plan: goal-a\n"
	read := "Goal-Read: goal-a/u " + hexA + "\n"
	tests := []struct {
		name     string
		endpoint string
		tip      string
		calls    []testgit.Expectation
	}{
		{"no trailer", "base", "tip", append(singleCommitTranscript(),
			rangeOutput("", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"two trailers", "base", "tip", append(singleCommitTranscript(),
			rangeOutput(unit+plan, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"unit whitespace", "base", "tip", append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-a/u\u00a0part\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"merge", "base", "tip", rangeTranscript("base", "tip", "base", "tip base side\n")},
		{"unit plan", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(unit, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/plans/x.md"))...)},
		{"unit exclusion", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(unit, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/"+landing.WorkspaceExclusions()[0]))...)},
		{"plan code", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(plan, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/a.go"))...)},
		{"read twice", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(read, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/records/reads/goal-a/"+hexA+".json", "metasystem/records/reads/goal-a/"+hexB+".json"))...)},
		{"read mismatch", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(read, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/records/reads/goal-a/"+hexB+".json"))...)},
		{"malformed reads path", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(plan, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/records/reads/not-an-attestation.md"))...)},
		{"other goal", "base", "tip", append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-b/u\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"empty unit commit", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(unit, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", "")...)},
		{"empty unit in build list", "base", "tip", append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-a/5++6\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"repeated unit in build list", "base", "tip", append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-a/5+5\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"branch base outside endpoint history", "endpoint", "tip", append(rangeTranscript("endpoint", "tip", "common", "tip common\n"),
			rangeOutput("", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"side unit outside endpoint history", "endpoint", "tip", append(rangeTranscript("endpoint", "tip", "common", "outside common\ntip outside\n"),
			rangeOutput("", "show", "-s", "--format=%(trailers:only,unfold=true)", "outside"))},
		{"plan record in read prose area", "base", "tip", append(append(singleCommitTranscript(),
			rangeOutput(plan, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", rawTree("metasystem/records/misc/plan.md"))...)},
		{"unrelated history", "base", "tip", []testgit.Expectation{
			rangeFailure(errors.New("exit status 1"), "merge-base", "base", "tip"),
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateWithTranscript(t, test.endpoint, test.tip, test.calls...)
			var refusal *RangeError
			if !errors.As(err, &refusal) || refusal.Code != "GOAL_BRANCH_RANGE" || test.name != "side unit outside endpoint history" && !strings.Contains(err.Error(), test.tip) {
				t.Fatalf("tip %s: %v", test.tip, err)
			}
			if test.name == "unrelated history" && !strings.Contains(refusal.Reason, "no common history") {
				t.Fatalf("history refusal changed: %v", err)
			}
			if test.name == "branch base outside endpoint history" && !strings.Contains(refusal.Reason, "expected exactly one kind trailer") {
				t.Fatalf("outside-base refusal changed: %v", err)
			}
			if test.name == "side unit outside endpoint history" && !strings.Contains(refusal.Reason, "expected exactly one kind trailer") {
				t.Fatalf("side-unit refusal changed: %v", err)
			}
		})
	}
}

func TestValidateRangeAllowsBranchBehindEndpoint(t *testing.T) {
	t.Parallel()
	raw := rawTree("metasystem/code.go")
	calls := append(rangeTranscript("endpoint", "tip", "base", "tip base\n"),
		rangeOutput("Goal-Unit: goal-a/u1\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
	calls = append(calls, treeCalls("tip", "base", raw)...)
	calls = append(calls, treeCalls("tip", "base", raw)...)
	commits, err := validateWithTranscript(t, "endpoint", "tip", calls...)
	if err != nil || len(commits) != 1 || commits[0].ID != "tip" {
		t.Fatalf("commits=%+v err=%v", commits, err)
	}
}

func TestValidateRangeCleanRangePasses(t *testing.T) {
	t.Parallel()
	hexUnit := strings.Repeat("a", 40)
	rawUnit := rawTree("metasystem/code.go")
	calls := append(rangeTranscript("base", "tip", "base", "plan base\n"+hexUnit+" plan\ntip "+hexUnit+"\n"),
		rangeOutput("Goal-Plan: goal-a\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "plan"))
	calls = append(calls, treeCalls("plan", "base", rawTree("metasystem/plans/x.md"))...)
	calls = append(calls, rangeOutput("Goal-Unit: goal-a/u1\n", "show", "-s", "--format=%(trailers:only,unfold=true)", hexUnit))
	calls = append(calls, treeCalls(hexUnit, "plan", rawUnit)...)
	calls = append(calls, treeCalls(hexUnit, "plan", rawUnit)...)
	calls = append(calls, rangeOutput("Goal-Read: goal-a/u1 "+hexUnit+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
	calls = append(calls, treeCalls("tip", hexUnit, rawTree("metasystem/records/reads/goal-a/"+hexUnit+".json"))...)
	commits, err := validateWithTranscript(t, "base", "tip", calls...)
	if err != nil || len(commits) != 3 || commits[0].Kind != Plan || commits[1].Kind != Unit || commits[1].Unit != "u1" || commits[1].Digest == "" || commits[2].Kind != Read || commits[2].Unit != "u1" {
		t.Fatalf("commits=%+v err=%v", commits, err)
	}
	empty, err := validateWithTranscript(t, "tip", "tip", rangeTranscript("tip", "tip", "tip", "")...)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}

func TestValidateRangeAcceptsBuildListsAndRejectsDuplicateUnitAcrossBuilds(t *testing.T) {
	t.Parallel()
	rawFirst := rawTree("metasystem/a.go")
	first := append(singleCommitTranscript(), rangeOutput("Goal-Unit: goal-a/5+6+7a+7b\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
	first = append(first, treeCalls("tip", "base", rawFirst)...)
	first = append(first, treeCalls("tip", "base", rawFirst)...)
	commits, err := validateWithTranscript(t, "base", "tip", first...)
	if err != nil || len(commits) != 1 || strings.Join(commits[0].Units, "+") != "5+6+7a+7b" {
		t.Fatalf("multi-unit build=%+v err=%v", commits, err)
	}
	duplicate := append(rangeTranscript("base", "second", "base", "tip base\nsecond tip\n"), first[2:]...)
	duplicate = append(duplicate, rangeOutput("Goal-Unit: goal-a/7b+8\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "second"))
	duplicate = append(duplicate, treeCalls("second", "tip", rawTree("metasystem/b.go"))...)
	if _, err := validateWithTranscript(t, "base", "second", duplicate...); err == nil || !strings.Contains(err.Error(), "unit 7b is already named") {
		t.Fatalf("duplicate unit range=%v", err)
	}
	rawSingle := rawTree("metasystem/a.go")
	single := append(singleCommitTranscript(), rangeOutput("Goal-Unit: goal-a/5f\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
	single = append(single, treeCalls("tip", "base", rawSingle)...)
	single = append(single, treeCalls("tip", "base", rawSingle)...)
	commits, err = validateWithTranscript(t, "base", "tip", single...)
	if err != nil || len(commits) != 1 || len(commits[0].Units) != 1 || commits[0].Unit != "5f" {
		t.Fatalf("single-unit build=%+v err=%v", commits, err)
	}
}

func TestValidateRangeInjectedReaderErrorsAndUnexpectedCalls(t *testing.T) {
	t.Parallel()
	readErr := errors.New("reader failure")
	raw := rawTree("metasystem/a.go")
	tests := []struct {
		name  string
		calls []testgit.Expectation
	}{
		{"first-parent list", []testgit.Expectation{
			rangeOutput("base\n", "merge-base", "base", "tip"),
			rangeFailure(readErr, "rev-list", "--first-parent", "--reverse", "--parents", "base..tip"),
		}},
		{"kind trailer", append(singleCommitTranscript(),
			rangeFailure(readErr, "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))},
		{"raw parent", append(append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-a/u\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			rangeFailure(readErr, "rev-list", "--parents", "-n", "1", "tip"))},
		{"raw diff", append(append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-a/u\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"),
			rangeOutput("tip base\n", "rev-list", "--parents", "-n", "1", "tip")),
			rangeFailure(readErr, "diff-tree", "-r", "-z", "--no-renames", "--full-index", "tip^", "tip"))},
		{"digest parent", append(append(append(singleCommitTranscript(),
			rangeOutput("Goal-Unit: goal-a/u\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip")),
			treeCalls("tip", "base", raw)...),
			rangeFailure(readErr, "rev-list", "--parents", "-n", "1", "tip"))},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateWithTranscript(t, "base", "tip", test.calls...)
			if !errors.Is(err, readErr) {
				t.Fatalf("reader failure not propagated: %v", err)
			}
		})
	}

	hexUnit := strings.Repeat("a", 40)
	t.Run("out-of-range read subject", func(t *testing.T) {
		calls := append(singleCommitTranscript(), rangeOutput("Goal-Read: goal-a/u "+hexUnit+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", "tip"))
		calls = append(calls, treeCalls("tip", "base", rawTree("metasystem/records/reads/goal-a/"+hexUnit+".json"))...)
		calls = append(calls, rangeOutput("Goal-Unit: goal-a/u\n", "show", "-s", "--format=%(trailers:only,unfold=true)", hexUnit))
		_, err := validateWithTranscript(t, "base", "tip", calls...)
		var refusal *RangeError
		if !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "preceding build commit") {
			t.Fatalf("out-of-range read subject: %v", err)
		}
	})
	t.Run("unexpected call is rejected", func(t *testing.T) {
		reporter := &rangeStubReporter{t: t}
		stub := testgit.New(reporter)
		result := stub.Run(testgit.Call{Dir: rangeRepo, Args: []string{"unexpected"}})
		if !errors.Is(result.Err, testgit.ErrUnexpectedCall) || len(reporter.messages) != 1 || !strings.Contains(reporter.messages[0], "unexpected Git call") {
			t.Fatalf("unexpected call was not rejected: result=%+v messages=%q", result, reporter.messages)
		}
	})
}
