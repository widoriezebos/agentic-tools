package main

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// A bare origin observes the commits installed on main. A status-only fixture
// cannot detect a different coverage decision during landing preparation.
func TestCoveringReadLandsGoalGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary-tip", "superseded", "main-advanced"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			writeUnitCarryFile(t, filepath.Join(c.primary, "metasystem/memory/receipts.log"), "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
			connectionGit(t, c.primary, "add", "metasystem/memory/receipts.log")
			connectionGit(t, c.primary, "commit", "-qm", "seed landing receipts")
			connectionGit(t, c.primary, "push", "-q", "origin", "main")
			connectionGit(t, c.worktree, "merge", "--ff-only", "main")
			owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
			connectionGit(t, c.worktree, "config", "goal.human.Wido", "Wido <wido@example.invalid>")
			connectionGit(t, c.worktree, "update-ref", goal.AcceptedRef, c.endpointTip())
			commit := func(unit, path string) string {
				writeUnitCarryFile(t, filepath.Join(c.worktree, path), unit+"\n")
				connectionGit(t, c.worktree, "add", path)
				code, result := c.runJSON(owners, "work", "commit", c.id, "--work", unit)
				if code != 0 {
					t.Fatalf("commit %s: %+v", unit, result)
				}
				return resultData(t, result)["commit"].(string)
			}
			first := commit("Ua", "first.txt")
			secondPath := "first.txt"
			rootJob := "crit1"
			if mode == "ordinary-tip" {
				secondPath = "second.txt"
			} else {
				_, started := c.runJSON(owners, "work", "review", "--commit", first, "--goal", c.id)
				if started.Outcome != intentInProgress {
					t.Fatal(started)
				}
				c.writeCritic(c.worktree, rootJob, first, "completed", true)
				rootJob = "crit2"
			}
			subject := commit("Ua2", secondPath)
			if mode == "ordinary-tip" {
				writeUnitCarryFile(t, filepath.Join(c.worktree, "metasystem/plans/handoff.md"), "Finished work.\n")
				connectionGit(t, c.worktree, "add", "metasystem/plans/handoff.md")
				connectionGit(t, c.worktree, "commit", "-qm", "handoff\n\nGoal-Plan: "+c.id)
				subject = connectionGit(t, c.worktree, "rev-parse", "HEAD")
				_, err := branch.Push(branch.PushRequest{Repo: c.worktree, GoalID: c.id, Remote: "origin", EndpointTip: c.endpointTip(), OpID: "handoff", CheckClaim: func() error { return nil }})
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "main-advanced" {
				writeUnitCarryFile(t, filepath.Join(c.primary, "main.txt"), "independent work\n")
				connectionGit(t, c.primary, "add", "main.txt")
				connectionGit(t, c.primary, "commit", "-qm", "independent work on main")
				connectionGit(t, c.primary, "push", "-q", "origin", "main")
				connectionGit(t, c.worktree, "update-ref", goal.AcceptedRef, c.endpointTip())
			}
			args := []string{"work", "review", "--commit", subject, "--goal", c.id, "--units", "Ua,Ua2"}
			if _, started := c.runJSON(owners, args...); started.Outcome != intentInProgress {
				t.Fatal(started)
			}
			c.writeCritic(c.worktree, rootJob, subject, "completed", true)
			if code, result := c.runJSON(owners, args...); code != 0 {
				t.Fatal(result)
			}
			tip := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			read, err := branch.InspectBranchRead(c.worktree, c.id, first)
			if err != nil || !read.Published || read.CoveredCommit != subject || (read.SupersededBy == "Ua2") != (mode != "ordinary-tip") {
				t.Fatalf("wrong covering read: %+v %v", read, err)
			}
			if code, result := c.runJSON(owners, "work", "review", c.id, "--work", "Ua"); code != 0 {
				t.Fatalf("named review: %+v", result)
			}
			req := branch.LandRequest{Repo: c.worktree, Remote: "origin", EndpointTip: c.endpointTip(), BranchTip: tip, GoalID: c.id, Last: true, LandingReady: true, CandidateOnly: true, GoalPage: string(goal.RenderFile(c.goalFile(c.id))), ApprovedBy: "human:Wido", Seat: "seat", CheckClaim: func() error { return nil }}
			candidate, err := branch.PrepareLanding(req)
			if err != nil {
				t.Fatal(err)
			}
			req.CandidateOnly = false
			req.Out = filepath.Join(t.TempDir(), "prepared")
			req.TestReceipt = filepath.Join(t.TempDir(), "receipt.json")
			c.writeJSON(req.TestReceipt, map[string]any{"schemaVersion": 3, "tree": candidate.Candidate, "exitStatus": 0, "time": "2026-11-20T09:00:00Z", "proof": map[string]string{"attemptId": "fixture-green"}})
			prepared, err := branch.PrepareLanding(req)
			if err != nil {
				t.Fatal(err)
			}
			_, err = branch.LandPush(branch.LandPushRequest{Repo: c.worktree, Remote: "origin", EndpointRef: "refs/heads/main", GoalID: c.id, Prepared: req.Out, CheckClaim: req.CheckClaim})
			if err != nil {
				t.Fatal(err)
			}
			if got := connectionGit(t, c.origin, "rev-parse", "refs/heads/main"); got != prepared.Landing {
				t.Fatalf("main = %s, want %s", got, prepared.Landing)
			}
			if content := connectionGit(t, c.origin, "show", "main:"+secondPath); content != "Ua2" {
				t.Fatalf("landed content %q", content)
			}
			if mode == "ordinary-tip" && connectionGit(t, c.origin, "show", "main:first.txt") != "Ua" {
				t.Fatal("first unit missing on main")
			}
			if mode == "main-advanced" && connectionGit(t, c.origin, "show", "main:main.txt") != "independent work" {
				t.Fatal("independent work missing on main")
			}
			series, err := branch.VerifyLandedSeries(c.worktree, prepared.Landing)
			if err != nil || len(series) != 2 {
				t.Fatalf("landed series: %v %v", series, err)
			}
		})
	}
}
