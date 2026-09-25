package branch_test

// The park check's own tests, moved here with the readers they exercise.
//
// Two facts are worth holding: the scrubbed git wrapper never lets a
// wrapper's stderr reach an object id, and the check reads the remote only
// where the goal actually has a branch — which is what makes a park of an
// ordinary queue goal one local read rather than two fetches.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestGoalBranchGitKeepsStderrOutOfObjectIDs(t *testing.T) {
	root := t.TempDir()
	want := strings.Repeat("a", 40)
	bin := t.TempDir()
	transcript := filepath.Join(bin, "calls")
	mock := filepath.Join(bin, "git")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$GOAL_BRANCH_GIT_LOG\"\n" +
		"if [ \"$#\" -ne 4 ] || [ \"$1\" != -C ] || [ \"$2\" != \"$GOAL_BRANCH_GIT_ROOT\" ] || [ \"$3\" != rev-parse ] || [ \"$4\" != HEAD ]; then\n" +
		"  printf 'unexpected git argv\\n' >&2; exit 2\nfi\n" +
		"printf '%s\\n' '" + want + "'\nprintf 'wrapper warning\\n' >&2\n"
	if err := testexec.WriteFile(mock, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOAL_BRANCH_GIT_ROOT", root)
	t.Setenv("GOAL_BRANCH_GIT_LOG", transcript)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := branch.ScrubbedGit(root, "rev-parse", "HEAD")
	if err != nil || got != want {
		t.Fatalf("object id = %q, want %q, err=%v", got, want, err)
	}
	calls, err := os.ReadFile(transcript)
	if err != nil || string(calls) != "-C\n"+root+"\nrev-parse\nHEAD\n" {
		t.Fatalf("mock git calls = %q, err=%v", calls, err)
	}
}

// The cost of a park is the thing this decides. A goal with no local branch
// and no unit commit named on its next step is answered without touching the
// remote at all; one with a branch reads both tips. An endpoint that is not
// refs/heads/main refuses before either read, and only for the goal that
// actually needs the remote read.
func TestParkCheckReadsTheRemoteOnlyForABranchBackedGoal(t *testing.T) {
	repo := t.TempDir()
	endpoint := goal.Endpoint{Root: repo, Remote: "origin", Branch: "refs/heads/main"}
	tip := strings.Repeat("7", 40)

	for _, test := range []struct {
		name          string
		localBranch   bool
		branchRef     string
		wantEndpoint  int
		wantOrigin    int
		wantSummary   string
		wantRefusalIn string
	}{
		{name: "no branch reads no remote", wantSummary: ""},
		{name: "branch reads both tips", localBranch: true, wantEndpoint: 1, wantOrigin: 1,
			wantRefusalIn: branch.ParkUnpushedCode},
		{name: "no branch never sees the endpoint refusal", branchRef: "refs/heads/develop"},
		{name: "branch refuses an unsupported endpoint", localBranch: true, branchRef: "refs/heads/develop",
			wantRefusalIn: "GOAL_BRANCH_ENDPOINT_UNSUPPORTED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			local, endpointReads, originReads := 0, 0, 0
			at := endpoint
			if test.branchRef != "" {
				at.Branch = test.branchRef
			}
			check := branch.ParkCheckWithReaders(repo, at,
				func(_, ref string) (string, bool, error) {
					local++
					if ref != "refs/heads/goal/standing" {
						t.Fatalf("local tip read %q", ref)
					}
					return tip, test.localBranch, nil
				},
				func(string, goal.Endpoint) (string, error) { endpointReads++; return tip, nil },
				// A different origin tip stops the check at its own refusal,
				// after both remote reads and before any range read.
				func(string, goal.Endpoint, string) (string, bool, error) {
					originReads++
					return strings.Repeat("9", 40), true, nil
				},
			)
			summary, err := check("standing", "Start standing.")
			switch {
			case test.wantRefusalIn != "":
				if err == nil || !strings.Contains(err.Error(), test.wantRefusalIn) {
					t.Fatalf("refusal = %v, want one naming %s", err, test.wantRefusalIn)
				}
			case err != nil:
				t.Fatalf("check: %v", err)
			case summary != test.wantSummary:
				t.Fatalf("summary = %q, want %q", summary, test.wantSummary)
			}
			if local != 1 || endpointReads != test.wantEndpoint || originReads != test.wantOrigin {
				t.Fatalf("reads local=%d endpoint=%d origin=%d, want 1/%d/%d",
					local, endpointReads, originReads, test.wantEndpoint, test.wantOrigin)
			}
		})
	}
}

// The disposable ref a tip read opens is deleted whether the fetch answered
// or not: a check that left refs behind would accumulate one per park.
func TestEndpointTipDeletesItsDisposableRef(t *testing.T) {
	for _, test := range []struct {
		name     string
		fetchErr error
	}{
		{name: "after a fetch that answered"},
		{name: "after a fetch that failed", fetchErr: fmt.Errorf("transport unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := [][]string{}
			_, err := branch.EndpointTipWithGit("/repo",
				goal.Endpoint{Root: "/repo", Remote: "origin", Branch: "refs/heads/main"},
				func(_ string, args ...string) (string, error) {
					calls = append(calls, args)
					if args[0] == "fetch" {
						return "", test.fetchErr
					}
					return strings.Repeat("b", 40), nil
				})
			if (err != nil) != (test.fetchErr != nil) {
				t.Fatalf("err = %v, want fetch error %v", err, test.fetchErr)
			}
			deleted := ""
			for _, call := range calls {
				if len(call) == 3 && call[0] == "update-ref" && call[1] == "-d" {
					deleted = call[2]
				}
			}
			if !strings.HasPrefix(deleted, "refs/metasystem/goals/endpoint/branch-") {
				t.Fatalf("the disposable ref was not deleted; calls=%v", calls)
			}
		})
	}
}
