package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Git's immutable commits and published attestation ancestry are the claim;
// the critic and publication transport use the connection fixture's adapters.
func TestSupersededReadProductionRoutesGitAdapter(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"single-commit-does-not-cover", "full-range-covers", "unpublished", "late-base", "missing-evidence", "wrong-goal", "later-unread", "partial-repair", "revert", "no-overlap", "chain"} {
		coverage := name != "single-commit-does-not-cover"
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			connectionGit(t, c.worktree, "update-ref", "refs/metasystem/goals/accepted", c.endpointTip())
			held := c.goalFile(c.id)
			held.AcceptedRisks = []goal.AcceptedRiskRecord{{Finding: "F1", Chain: "crit1", By: "Wido", Opid: "retained-risk"}}
			c.addGoal(held)
			owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
			commit := func(unit, content string) string {
				path := "hand.txt"
				if unit == "Ua2" && name == "no-overlap" {
					path = "unrelated.txt"
				}
				if unit == "Ua2" && name == "revert" {
					if err := os.Remove(filepath.Join(c.worktree, path)); err != nil {
						t.Fatal(err)
					}
				} else {
					writeUnitCarryFile(t, filepath.Join(c.worktree, path), content)
				}
				connectionGit(t, c.worktree, "add", path)
				code, result := c.runJSON(owners, "work", "commit", c.id, "--work", unit)
				if code != 0 {
					t.Fatalf("commit: %+v", result)
				}
				return resultData(t, result)["commit"].(string)
			}
			if name == "partial-repair" {
				writeUnitCarryFile(t, filepath.Join(c.worktree, "other.txt"), "earlier change\n")
				connectionGit(t, c.worktree, "add", "other.txt")
			}
			first := commit("Ua", "original\n")
			review := []string{"work", "review", "--commit", first, "--goal", c.id}
			c.runJSON(owners, review...)
			c.writeCritic(c.worktree, "crit1", first, "completed", true)
			content := "corrected\n"
			if name == "revert" {
				content = ""
			}
			second := commit("Ua2", content)
			code, original := c.runJSON(owners, review...)
			if code == 0 && name != "no-overlap" {
				t.Fatalf("unread repair closed original: %+v", original)
			}
			later := []string{"work", "review", "--commit", second, "--goal", c.id}
			if coverage {
				later = append(later, "--units", "Ua,Ua2")
			}
			if name == "full-range-covers" {
				if original.Next == nil {
					t.Fatal("unread work has no review command")
				}
				later = original.Next.Argv[1:]
			}
			if _, result := c.runJSON(owners, later...); result.Outcome != intentInProgress {
				t.Fatalf("start repair read: %+v", result)
			}
			c.writeCritic(c.worktree, "crit2", second, "completed", true)
			if name == "unpublished" {
				c.failPushes = 1
				c.runJSON(owners, later...)
				code, result := c.runJSON(owners, review...)
				if code == 0 || !strings.Contains(result.Summary, "not yet published") || result.Next == nil {
					t.Fatalf("unpublished coverage: %+v", result)
				}
				if code, result = c.runJSON(owners, result.Next.Argv[1:]...); code != 0 {
					t.Fatalf("publish remedy: %+v", result)
				}
			}

			if code, result := c.runJSON(owners, later...); code != 0 {
				t.Fatalf("publish repair: %+v", result)
			}
			if name == "late-base" || name == "missing-evidence" || name == "wrong-goal" {
				common := connectionGit(t, c.worktree, "rev-parse", "--git-common-dir")
				if !filepath.IsAbs(common) {
					common = filepath.Join(c.worktree, common)
				}
				path := filepath.Join(common, "metasystem", "goal-reads", c.id, second+".json")
				var record map[string]any
				data, err := os.ReadFile(path)
				if err != nil || json.Unmarshal(data, &record) != nil {
					t.Fatal("read journal", err)
				}
				briefPath := record["brief"].(string)
				if name == "wrong-goal" {
					record["goal"] = "another-goal"
					c.writeJSON(path, record)
				} else if name == "missing-evidence" {
					if err := os.Remove(briefPath); err != nil {
						t.Fatal(err)
					}
				} else {
					brief, err := os.ReadFile(briefPath)
					if err != nil {
						t.Fatal(err)
					}
					brief = []byte(strings.ReplaceAll(string(brief), "git diff "+c.endpointTip()+"...", "git diff "+first+"..."))
					if err := os.WriteFile(briefPath, brief, 0600); err != nil {
						t.Fatal(err)
					}
					record["frozenBriefSha256"] = fmt.Sprintf("%x", sha256.Sum256(brief))
					c.writeJSON(path, record)
				}
				coverage = false
				connectionGit(t, c.worktree, "config", "goal.human.Wido", "Wido <wido@example.invalid>")
				_, err = branch.PrepareLanding(branch.LandRequest{Repo: c.worktree, Remote: "origin", EndpointTip: c.endpointTip(), BranchTip: connectionGit(t, c.worktree, "rev-parse", "HEAD"),
					GoalID: c.id, Last: true, LandingReady: true, CandidateOnly: true, ApprovedBy: "human:Wido", Seat: "seat", CheckClaim: func() error { return nil }})
				if err == nil || !strings.Contains(err.Error(), "no reviewed build ready to land") {
					t.Fatalf("landing did not hold the unread unit: %v", err)
				}
			}
			if name == "no-overlap" {
				read, err := branch.InspectBranchRead(c.worktree, c.id, first)
				if err != nil || !read.Published || read.SupersededBy != "" {
					t.Fatalf("unrelated repair superseded original: %+v %v", read, err)
				}
				return
			}
			wantUnit := "Ua2"
			wantDelegates := 2
			if name == "later-unread" {
				commit("Ua3", "unread repair\n")
			}
			if name == "chain" {
				third := commit("Ua3", "third repair\n")
				args := []string{"work", "review", "--commit", third, "--goal", c.id, "--units", "Ua,Ua2,Ua3"}
				c.runJSON(owners, args...)
				c.writeCritic(c.worktree, "crit3", third, "completed", true)
				if code, result := c.runJSON(owners, args...); code != 0 {
					t.Fatalf("third read: %+v", result)
				}
				wantUnit, wantDelegates = "Ua3", 3
			}
			if name == "single-commit-does-not-cover" {
				code, repeat := c.runJSON(owners, append(append([]string(nil), later...), "--units", "Ua,Ua2")...)
				if code == 0 || repeat.Next == nil || !strings.Contains(strings.Join(repeat.Next.Argv, " "), "--units Ua,Ua2") {
					t.Fatalf("published read was relabelled: %+v", repeat)
				}
			}
			tip := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			for repeat := 0; repeat < 2; repeat++ {
				code, result := c.runJSON(owners, review...)
				if !strings.Contains(strings.Join(result.Details, " "), "Accepted risk F1 on read crit1, by Wido") {
					t.Fatalf("risk lost on original: %+v", result)
				}
				if got := c.goalFile(c.id).AcceptedRisks; len(got) != 1 || got[0].Chain != "crit1" {
					t.Fatalf("risk rebound: %+v", got)
				}
				if coverage {
					if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "superseded by "+wantUnit) {
						t.Fatalf("covered original: exit=%d %+v", code, result)
					}
					if connectionGit(t, c.worktree, "rev-parse", "HEAD") != tip || len(c.delegates) != wantDelegates {
						t.Fatal("repeat changed commits or launched a read")
					}
				} else {
					if code == 0 || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "--units Ua,Ua2") || !strings.Contains(strings.Join(result.Details, " "), "hand.txt") {
						t.Fatalf("uncovered original has no full-range remedy: %+v", result)
					}
					if repeat == 1 && name == "single-commit-does-not-cover" {
						_, next := c.runJSON(owners, result.Next.Argv[1:]...)
						if next.Outcome != intentInProgress || len(c.delegates) != 3 {
							t.Fatalf("printed remedy did not launch: %+v", next)
						}
					}
				}
			}
			if coverage {
				code, result := c.runJSON(owners, "work", "review", c.id, "--work", "Ua")
				if code != 0 || !strings.Contains(result.Summary, "superseded by "+wantUnit) {
					t.Fatalf("named route: %+v", result)
				}
				state, err := branch.InspectStatus(c.worktree, c.endpointTip(), tip, c.id)
				if err != nil || len(state.Units) == 0 || !state.Units[0].Resolved() {
					t.Fatalf("landing status: %+v %v", state, err)
				}
				if name == "later-unread" && state.Prefix == len(state.Units) {
					t.Fatal("unread repair passed landing")
				}
				code, status := c.runJSON(owners, "work", "status", c.id)
				if code != 0 || !strings.Contains(strings.Join(status.Details, " ")+status.Summary+fmt.Sprint(status.Data), "superseded by "+wantUnit) {
					t.Fatalf("status loses supersession: %+v", status)
				}
			}
		})
	}
}

// The runner retains Ua's original subject while a hand correction changes
// its files. The named review must use the covering read before recommitting.
func TestSupersededBuiltWorkGitAdapter(t *testing.T) {
	t.Parallel()
	c := newConnectionBed(t)
	c.edits = map[string]string{"connect.txt": "original\n"}
	args := append([]string{"work", "build", c.id, "Ua", "--last", "--brief", c.brief("brief.md", "Build it.\n"), "--lines", "10"}, workCheck...)
	code, built := c.do(args...)
	if code != 0 {
		t.Fatalf("build: %+v", built)
	}
	run := resultData(t, built)["run"].(string)
	if _, result := c.do("work", "review", c.id, "--work", "Ua"); result.Outcome != intentInProgress {
		t.Fatalf("original review: %+v", result)
	}
	first := c.runRecord(run).Subjects[0].Commit
	c.writeCritic(c.worktree, "crit1", first, "completed", true)
	connectionGit(t, c.worktree, "update-ref", goal.AcceptedRef, c.endpointTip())
	writeUnitCarryFile(t, filepath.Join(c.worktree, "connect.txt"), "corrected\n")
	connectionGit(t, c.worktree, "add", "connect.txt")
	owners := c.connectionOwners()
	connectionGit(t, c.worktree, "commit", "-qm", "repair\n\nGoal-Unit: "+c.id+"/Ua2")
	second := connectionGit(t, c.worktree, "rev-parse", "HEAD")
	if _, err := branch.Push(branch.PushRequest{Repo: c.worktree, Remote: "origin", EndpointTip: c.endpointTip(), GoalID: c.id, OpID: "repair", CheckClaim: func() error { return nil }}); err != nil {
		t.Fatal(err)
	}
	review := []string{"--root", c.worktree, "--unit", second, "--goal", c.id, "--units", "Ua,Ua2"}
	if _, code, err := owners.delivery.branchRead(review); code != 0 || err != nil {
		t.Fatalf("start covering read: %v", err)
	}
	c.writeCritic(c.worktree, "crit2", second, "completed", true)
	if _, code, err := owners.delivery.branchRead(append(review, "--collect")); code != 0 || err != nil {
		t.Fatalf("collect covering read: %v", err)
	}
	if _, err := owners.delivery.publishRead(c.worktree, c.id, second); err != nil {
		t.Fatal(err)
	}
	tip := connectionGit(t, c.worktree, "rev-parse", "HEAD")
	for _, args := range [][]string{{"work", "review", c.id, "--work", "Ua"}, {"work", "review", c.id, "--work", "Ua", "--dispositions", c.brief("decisions.md", "Prior decisions remain.\n")}} {
		code, result := c.runJSON(owners, args...)
		if code != 0 || !strings.Contains(result.Summary, "superseded by Ua2") {
			t.Fatalf("named original: %+v", result)
		}
		if connectionGit(t, c.worktree, "rev-parse", "HEAD") != tip {
			t.Fatal("covered original recommitted")
		}
	}
}
