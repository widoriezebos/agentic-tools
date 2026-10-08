package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// These adapter scenarios require Git's actual index, scratch commit and
// attestation installation: a stub cannot prove which bytes were committed.
// The goal ledger, model, clock and process observations stay synthetic.
func readPublicationAdapterBed(t *testing.T, subdirectory ...string) (*connectionBed, intentOwners) {
	t.Helper()
	c := &connectionBed{workBed: newWorkBed(t), t: t, edits: map[string]string{}}
	root := c.root()
	top := root
	if len(subdirectory) > 0 {
		top = filepath.Join(t.TempDir(), "repo")
		root = filepath.Join(top, subdirectory[0])
		if err := os.MkdirAll(top, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(c.root(), root); err != nil {
			t.Fatal(err)
		}
		c.facts.root = root
	}
	connectionGit(t, top, "init", "-q", "-b", "main")
	connectionGit(t, root, "config", "user.name", "Fixture")
	connectionGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeUnitCarryFile(t, filepath.Join(root, ".gitignore"), "artifacts/\n")
	conf, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	writeUnitCarryFile(t, filepath.Join(root, "metasystem.conf"), string(conf)+"\nproof.cheap=true\n")
	if top != root {
		writeUnitCarryFile(t, filepath.Join(root, "metasystem.conf"), string(conf)+"\nmetasystem.template=true\nproof.cheap=true\n")
	}
	connectionGit(t, root, "add", "-A")
	connectionGit(t, root, "commit", "-qm", "fixture base")
	c.origin = filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(c.origin), "init", "-q", "--bare", "-b", "main", c.origin)
	connectionGit(t, root, "remote", "add", "origin", c.origin)
	connectionGit(t, root, "push", "-q", "origin", "main")
	c.worktree = filepath.Join(t.TempDir(), "work")
	connectionGit(t, root, "worktree", "add", "-qb", "goal/"+c.id, c.worktree, "main")
	c.manager.Supervisor = c
	owners := c.connectionOwners()
	if top != root {
		owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
			return goalBranchGit(path, "rev-parse", "--show-toplevel")
		}, noExecutable)
	}
	// The command edge resolves abbreviated commits before calling the branch owner.
	read := owners.delivery.branchRead
	owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		full, err := goalBranchGit(flagValue(args, "--root"), "rev-parse", "--verify", flagValue(args, "--unit")+"^{commit}")
		if err != nil {
			return branch.BranchReadResult{}, 1, err
		}
		return read(replaceFlagValue(args, "--unit", full))
	}
	owners.connection.section = func(_ string, act func(func(func() error) error) error) error {
		return act(func(act func() error) error { return act() })
	}
	return c, owners
}

func TestWorkCommitSubdirectoryGitAdapter(t *testing.T) {
	t.Parallel()
	c, owners := readPublicationAdapterBed(t, "metasystem")
	install := filepath.Join(c.worktree, "metasystem")
	writeUnitCarryFile(t, filepath.Join(install, "metasystem.conf"), "metasystem.template=true\nproof.cheap=test -f metasystem.conf && test -f ../hand.txt\n")
	connectionGit(t, install, "add", "metasystem.conf")
	connectionGit(t, install, "commit", "-qm", "goal "+c.id+" units settings\n\nGoal-Unit: "+c.id+"/settings")
	writeUnitCarryFile(t, filepath.Join(c.worktree, "hand.txt"), "hand change\n")
	connectionGit(t, c.worktree, "add", "hand.txt")
	static := 0
	owners.connection.rebaseGate = func(candidate string) (string, error) {
		static++
		if filepath.Base(candidate) != "metasystem" {
			t.Fatalf("static check outside the installation: %s", candidate)
		}
		return "static-green", nil
	}
	code, result := c.runJSON(owners, "work", "commit", c.id, "--work", "hand")
	if code != 0 || result.Outcome != intentConfirmed || c.commits != 1 || static != 1 {
		t.Fatalf("nested installation commit: exit=%d static=%d %+v", code, static, result)
	}
	if message := connectionGit(t, c.worktree, "log", "-1", "--format=%B"); !strings.Contains(message, "Goal-Unit: "+c.id+"/hand") {
		t.Fatalf("missing unit trailer: %s", message)
	}
}

func TestWorkCommitClaimRemedy(t *testing.T) {
	t.Parallel()
	for _, held := range []bool{true, false} {
		t.Run(fmt.Sprint(held), func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			owners := b.workOwners()
			owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
				return func() error {
					if held {
						return goalHeldElsewhere{goal: b.id, holder: "other-seat", machine: "mac-cli", lineage: "m1"}
					}
					return errors.New("the goal claim could not be read")
				}
			}
			owners.connection.commit = func(branch.CommitRequest) (string, error) {
				t.Fatal("a refused claim committed")
				return "", nil
			}
			code, result := b.runJSON(owners, "work", "commit", b.id, "--work", "hand")
			if code == 0 || result.Outcome != intentRefused || result.Next == nil ||
				!strings.Contains(strings.Join(result.Next.Argv, " "), "goal claim "+b.id+" --take-over --reason TEXT") ||
				!strings.Contains(result.Next.Reason, "a person") {
				t.Fatalf("claim refusal has no person recovery: exit=%d %+v", code, result)
			}
			if held && (!strings.Contains(result.Summary, "other-seat") || !strings.Contains(result.Next.Reason, "other-seat")) {
				t.Fatalf("claim refusal hides its holder: %+v", result)
			}
		})
	}
}

func TestCommitReadPublicationLifecycleGitAdapter(t *testing.T) {
	t.Parallel()
	c, owners := readPublicationAdapterBed(t)
	owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
	writeUnitCarryFile(t, filepath.Join(c.worktree, "hand.txt"), "hand change\n")
	connectionGit(t, c.worktree, "add", "hand.txt")
	code, result := c.runJSON(owners, "work", "commit", c.id, "--work", "hand")
	if code != 0 {
		t.Fatalf("first unit commit: exit=%d %+v", code, result)
	}
	commit := resultData(t, result)["commit"].(string)
	review := []string{"work", "review", "--commit", commit, "--goal", c.id}
	code, result = c.runJSON(owners, review...)
	if result.Outcome != intentInProgress || len(c.delegates) != 1 {
		t.Fatalf("start read in first invocation: exit=%d %+v", code, result)
	}
	identity := sha256.Sum256([]byte(c.id + "\x00" + commit))
	path := filepath.Join(c.worktree, "artifacts", "agents", "read-publication", fmt.Sprintf("%x", identity))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pending read did not retain its tip: %v", err)
	}
	writeUnitCarryFile(t, filepath.Join(c.worktree, "next.txt"), "next unit\n")
	connectionGit(t, c.worktree, "add", "next.txt")
	code, result = c.runJSON(owners, "work", "commit", c.id, "--work", "next")
	if code != 0 {
		t.Fatalf("next unit commit: exit=%d %+v", code, result)
	}
	tip := connectionGit(t, c.worktree, "rev-parse", "HEAD")
	c.writeCritic(c.worktree, "crit1", commit, "completed", true)
	code, result = c.runJSON(owners, review...)
	if code == 0 || c.publications != 0 || c.commitReads != 0 || result.Next == nil ||
		!strings.Contains(result.Next.Reason, "new version") || strings.Contains(result.Summary, "restore") ||
		!strings.Contains(strings.Join(result.Next.Argv, " "), "work review --commit SHA --goal "+c.id) {
		t.Fatalf("moved tip needs a new work version: exit=%d %+v", code, result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("refused publication kept its stale tip: %v", err)
	}
	if connectionGit(t, c.worktree, "rev-parse", "HEAD") != tip {
		t.Fatal("refusal changed the next unit's commit")
	}
	code, result = c.runJSON(owners, review...)
	if code != 0 || result.Outcome != intentConfirmed || c.publications != 1 || c.commitReads != 1 {
		t.Fatalf("publish from fresh invocation: exit=%d %+v", code, result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("successful publication kept its saved tip: %v", err)
	}
	writeUnitCarryFile(t, filepath.Join(c.worktree, "later.txt"), "later unit\n")
	connectionGit(t, c.worktree, "add", "later.txt")
	code, result = c.runJSON(owners, "work", "commit", c.id, "--work", "later")
	if code != 0 {
		t.Fatalf("later unit commit: exit=%d %+v", code, result)
	}
	writeUnitCarryFile(t, filepath.Join(c.worktree, "later.txt"), "uncommitted later change\n")
	reads := len(c.reads)
	code, result = c.runJSON(owners, review...)
	if code != 0 || result.Outcome != intentUnchanged || len(c.reads) != reads || c.publications != 1 || c.commitReads != 1 {
		t.Fatalf("published repeat must precede tree checks: exit=%d %+v", code, result)
	}
}

func TestWorkCommitGatesGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"green", "static-red", "cheap-red", "checks-write", "checks-stage"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			if mode == "cheap-red" || mode == "checks-write" || mode == "checks-stage" {
				command := "false"
				if mode == "checks-write" {
					command = "printf changed > hand.txt"
				}
				if mode == "checks-stage" {
					command = "git update-index --cacheinfo 100644,$(printf wrong | git hash-object -w --stdin),hand.txt"
				}
				writeUnitCarryFile(t, filepath.Join(c.worktree, "metasystem.conf"), "proof.cheap="+command+"\n")
				connectionGit(t, c.worktree, "add", "metasystem.conf")
				connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units settings\n\nGoal-Unit: "+c.id+"/settings")
			}
			before := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			writeUnitCarryFile(t, filepath.Join(c.worktree, "hand.txt"), "hand change\n")
			connectionGit(t, c.worktree, "add", "hand.txt")
			gates := 0
			owners.connection.rebaseGate = func(string) (string, error) {
				gates++
				if mode == "static-red" {
					return "", errors.New("static check failed")
				}
				return "static-green", nil
			}
			code, result := c.runJSON(owners, "work", "commit", c.id, "--work", "hand")
			tip := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			if mode != "green" {
				if code == 0 || tip != before || c.commits != 0 || gates != 1 {
					t.Fatalf("failed check committed: mode=%s code=%d result=%+v gates=%d", mode, code, result, gates)
				}
				return
			}
			if code != 0 || tip == before || c.commits != 1 || gates != 1 {
				t.Fatalf("commit: code=%d result=%+v gates=%d", code, result, gates)
			}
			if message := connectionGit(t, c.worktree, "log", "-1", "--format=%B"); !strings.Contains(message, "Goal-Unit: "+c.id+"/hand") {
				t.Fatalf("missing trailer: %s", message)
			}
			if remote := connectionGit(t, c.root(), "--git-dir", c.origin, "rev-parse", "refs/heads/goal/"+c.id); remote != tip {
				t.Fatalf("unpublished commit: %s != %s", remote, tip)
			}
			writeUnitCarryFile(t, filepath.Join(c.worktree, "hand.txt"), "corrected\n")
			connectionGit(t, c.worktree, "add", "hand.txt")
			code, result = c.runJSON(owners, "work", "commit", c.id, "--work", "hand", "--amend")
			if code != 0 || gates != 2 || c.commits != 2 {
				t.Fatalf("correction: code=%d result=%+v gates=%d", code, result, gates)
			}
		})
	}
}

func TestManualReadPublicationReservationGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"clean", "changed", "tip-moved", "reserved", "push-unlocked", "manual-clean", "manual-changed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			brief := filepath.Join(t.TempDir(), "brief.md")
			writeUnitCarryFile(t, brief, "Add hand.txt.\n")
			writeUnitCarryFile(t, filepath.Join(c.root(), "hand.txt"), "hand change\n")
			code, result := c.runJSON(owners, "work", "review", c.id, "--changes", "--brief", brief, "--work", "hand")
			if result.Outcome != intentInProgress || len(c.delegates) != 1 {
				t.Fatalf("submit: code=%d result=%+v", code, result)
			}
			commit := resultData(t, result)["commit"].(string)
			c.writeCritic(c.worktree, "crit1", commit, "completed", true)
			var release func() error
			switch mode {
			case "changed", "manual-changed":
				writeUnitCarryFile(t, filepath.Join(c.worktree, "hand.txt"), "changed after read\n")
			case "tip-moved":
				writeUnitCarryFile(t, filepath.Join(c.worktree, "other.txt"), "unexamined\n")
				connectionGit(t, c.worktree, "add", "other.txt")
				connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units other\n\nGoal-Unit: "+c.id+"/other")
			case "reserved":
				var err error
				release, err = owners.work.units(stateroot.Layout{}).ReserveMutation(c.worktree, c.id, "other-writer")
				if err != nil {
					t.Fatal(err)
				}
				defer release()
			case "push-unlocked":
				publish := owners.delivery.publishRead
				owners.delivery.publishRead = func(root, goalID, unit string) (branch.PublishReadResult, error) {
					runner := owners.work.units(stateroot.Layout{})
					err := runner.MutationSection(c.worktree, func(*launch.UnitRunner) error { return nil })
					var waiting *launch.TreeWaitingError
					if !errors.As(err, &waiting) {
						t.Fatalf("push holds lock or lost reservation: %v", err)
					}
					return publish(root, goalID, unit)
				}
			}
			review := []string{"work", "review", "--commit", commit[:12], "--goal", c.id}
			if strings.HasPrefix(mode, "manual-") {
				review = []string{"work", "review", c.id, "--changes", "--brief", brief, "--work", "hand"}
			}
			code, result = c.runJSON(owners, review...)
			if mode == "changed" || mode == "manual-changed" || mode == "tip-moved" || mode == "reserved" {
				if code == 0 || c.publications != 0 || c.commitReads != 0 {
					t.Fatalf("unsafe read published: code=%d result=%+v reads=%d publications=%d", code, result, c.commitReads, c.publications)
				}
				return
			}
			if code != 0 || c.publications != 1 {
				t.Fatalf("publication: code=%d result=%+v", code, result)
			}
			code, result = c.runJSON(owners, review...)
			if code != 0 || len(c.delegates) != 1 || c.commitReads != 1 {
				t.Fatalf("repeat: code=%d result=%+v reads=%d", code, result, c.commitReads)
			}
		})
	}
}

func TestCommitReadUsesUnitReservation(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"clean", "changed"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, owners, _, _ := unitCarryIntentBed(t, false)
			b.head = "replayed-tip"
			if state == "changed" {
				b.head = "different-tip"
			}
			called := 0
			read := owners.delivery.branchRead
			owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) { called++; return read(args) }
			code, result := b.runJSON(owners, "work", "review", "--commit", "corrected-commit", "--goal", b.id)
			if state == "changed" {
				if code == 0 || called != 0 {
					t.Fatalf("changed unit collected: code=%d result=%+v calls=%d", code, result, called)
				}
			} else if code != 0 || result.Outcome != intentConfirmed || called != 1 {
				t.Fatalf("the unit's own read waited on its reservation: code=%d result=%+v", code, result)
			}
		})
	}
}
