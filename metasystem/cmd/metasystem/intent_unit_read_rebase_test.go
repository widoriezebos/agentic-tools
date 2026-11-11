package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// Git must replay the units, retain their evidence and publish the recovered reads.
func TestRebasedReadPublishesAfterInterruptedCarryAndAppend(t *testing.T) {
	forUnitLayouts(t, func(t *testing.T, installation string) {
		t.Parallel()
		c := newConnectionBed(t, installation)
		connectionGit(t, c.root(), "update-ref", goal.AcceptedRef, c.endpointTip())
		owners := c.connectionOwners()
		owners.connection.rebase = branch.Rebase
		owners.connection.section = func(_ string, body func(func(func() error) error) error) error {
			return body(func(act func() error) error { return act() })
		}
		owners.connection.recordRebase = func(*intentInvocation, string, branch.RebaseResult) error { return nil }
		checks, interrupted := 0, false
		owners.connection.subjectCheck = func(_ string, subject branch.AttestationSubject) (branch.GateObservation, error) {
			checks++
			if interrupted && checks == 2 {
				return branch.GateObservation{}, errors.New("fixture: carry interrupted")
			}
			return branch.GateObservation{Kind: "go-gate-fast", Tree: subject.Tree, RunID: "carry-" + subject.Commit}, nil
		}
		call := func(args ...string) (int, intentResult) { return c.runJSON(owners, args...) }
		build := func(unit string) string {
			t.Helper()
			c.edits = map[string]string{unit + ".txt": unit + " result\n"}
			code, result := call(append([]string{"work", "build", c.id, "--work", unit, "--brief", c.brief(unit+".md", "Build "+unit+".\n"), "--lines", "5"}, workCheck...)...)
			if code != 0 {
				t.Fatalf("build %s: %d %+v", unit, code, result)
			}
			return resultData(t, result)["run"].(string)
		}
		for i, unit := range []string{"u1", "u2"} {
			run := build(unit)
			if code, result := call("work", "review", c.id, "--work", unit); result.Outcome != intentInProgress {
				t.Fatalf("start %s: %d %+v", unit, code, result)
			}
			c.writeCritic(c.workInstallation(), fmt.Sprintf("crit%d", i+1), c.runRecord(run).Subjects[0].Commit, "completed", true)
			if code, result := call("work", "review", c.id, "--work", unit); code != 0 {
				t.Fatalf("publish %s: %d %+v", unit, code, result)
			}
		}
		if code, result := call("work", "review", c.id, "--work", "u1"); code != 0 || result.Outcome != intentUnchanged {
			t.Fatalf("repeat before rebase: %d %+v", code, result)
		}
		writeUnitCarryFile(t, filepath.Join(c.primary, "main.txt"), "main moved\n")
		connectionGit(t, c.primary, "add", "main.txt")
		connectionGit(t, c.primary, "commit", "-qm", "independent main change")
		connectionGit(t, c.primary, "push", "-q", "origin", "main")
		connectionGit(t, c.root(), "update-ref", goal.AcceptedRef, c.endpointTip())
		checks, interrupted = 0, true
		if code, result := call("work", "rebase", c.id); code == 0 || !strings.Contains(result.Summary, "carry interrupted") || checks != 2 {
			t.Fatalf("interrupted carry: %d %+v checks=%d", code, result, checks)
		}
		interrupted = false
		build("u3")
		if code, result := call("work", "review", c.id, "--work", "u3"); result.Outcome != intentInProgress || checks != 3 {
			t.Fatalf("append resumes missing carry: %d %+v checks=%d", code, result, checks)
		}
		tip := connectionGit(t, c.worktree, "rev-parse", "HEAD")
		rangeCommits, err := branch.ValidateRange(c.workInstallation(), c.endpointTip(), tip, c.id)
		if err != nil {
			t.Fatal(err)
		}
		commits, reads, critics := c.commits, c.commitReads, len(c.delegates)
		for _, unit := range []string{"u1", "u2"} {
			for repeat := 0; repeat < 2; repeat++ {
				if code, result := call("work", "review", c.id, "--work", unit); code != 0 || result.Outcome != intentUnchanged {
					t.Fatalf("repeat %s: %d %+v", unit, code, result)
				}
			}
		}
		for _, commit := range rangeCommits {
			if commit.Kind != branch.Unit || commit.Unit == "u3" {
				continue
			}
			read, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: c.workInstallation(), Remote: "origin", EndpointTip: c.endpointTip(), BranchTip: tip,
				GoalID: c.id, UnitCommit: commit.ID, Collect: true, CheckClaim: func() error { return nil }})
			if err != nil || read.State != "already-collected" || read.AttestationCommit == "" {
				t.Fatalf("recover carried journal for %s: %+v %v", commit.Unit, read, err)
			}
		}
		if c.commits != commits || c.commitReads != reads || len(c.delegates) != critics || connectionGit(t, c.worktree, "rev-parse", "HEAD") != tip {
			t.Fatal("repeated publication created another commit or critic")
		}
		writeUnitCarryFile(t, filepath.Join(c.worktree, "u1.txt"), "changed after review\n")
		connectionGit(t, c.worktree, "add", "u1.txt")
		connectionGit(t, c.worktree, "commit", "-qm", "changed reviewed content\n\nGoal-Unit: "+c.id+"/changed")
		if code, result := call("work", "review", c.id, "--work", "u1"); code == 0 || !strings.Contains(result.Summary, "changed after the read") {
			t.Fatalf("changed reviewed content: %d %+v", code, result)
		}
	})
}
