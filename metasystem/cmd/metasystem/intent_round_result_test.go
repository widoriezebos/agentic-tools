package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// This adapter fixture observes actual Git replay, conflicts, trees and
// installation. A fake Git response cannot prove those adapter operations.
func roundResultBed(t *testing.T) *connectionBed {
	t.Helper()
	c := &connectionBed{workBed: newWorkBed(t), t: t, edits: map[string]string{"unit.txt": "built\n"}}
	root := c.root()
	connectionGit(t, root, "init", "-q", "-b", "main")
	connectionGit(t, root, "config", "user.name", "Fixture")
	connectionGit(t, root, "config", "user.email", "fixture@example.invalid")
	connectionGit(t, root, "config", "commit.gpgsign", "false")
	connectionGit(t, root, "config", "core.hooksPath", t.TempDir())
	writeUnitCarryFile(t, filepath.Join(root, ".gitignore"), "artifacts/\n.claude/settings.local.json\n")
	writeUnitCarryFile(t, filepath.Join(root, "unit.txt"), "base\n")
	connectionGit(t, root, "add", "-A")
	connectionGit(t, root, "commit", "-qm", "fixture base")
	c.origin = filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(c.origin), "init", "-q", "--bare", "-b", "main", c.origin)
	connectionGit(t, root, "remote", "add", "origin", c.origin)
	connectionGit(t, root, "push", "-q", "origin", "main")
	resolved, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	c.worktree = filepath.Join(resolved, filepath.Base(root)+"-"+c.id)
	return c
}

type roundResultStarter struct {
	c           *connectionBed
	proofTrees  []string
	beforeProof func(launch.Record)
	holdProof   bool
	pending     string
}

func (s *roundResultStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.c.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind != "proof" {
		_, err := s.c.StartSupervisor(id, state)
		if err == nil {
			_, err = s.c.manager.Store.Update(id, func(current *launch.Record) error {
				current.FinishedAt = s.c.manager.Now().UTC().Format(time.RFC3339Nano)
				return nil
			})
		}
		return workProcessRef(30), err
	}
	if len(s.proofTrees) > 0 && s.beforeProof != nil {
		s.beforeProof(record)
	}
	if s.holdProof && len(s.proofTrees) > 0 {
		s.pending = id
		s.proofTrees = append(s.proofTrees, connectionGit(s.c.t, record.WorkingDirectory, "write-tree"))
		_, err := s.c.manager.Store.Update(id, func(current *launch.Record) error {
			current.State = launch.Running
			supervisor, child := workProcessRef(10), workProcessRef(20)
			current.Supervisor, current.Child = &supervisor, &child
			return nil
		})
		return workProcessRef(10), err
	}
	return s.finishProof(record, state)
}

func (s *roundResultStarter) finishProof(record launch.Record, state string) (identity.Ref, error) {
	id := record.ID
	spec, err := (launch.PlainExec{}).Command(record, state)
	if err != nil {
		return identity.Ref{}, err
	}
	command := exec.Command(spec.Program, spec.Args...)
	command.Dir, command.Env = spec.Directory, spec.Environment
	output, runErr := command.CombinedOutput()
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return identity.Ref{}, runErr
	}
	if err := os.WriteFile(spec.LogPath, output, 0600); err != nil {
		return identity.Ref{}, err
	}
	s.proofTrees = append(s.proofTrees, connectionGit(s.c.t, record.WorkingDirectory, "write-tree"))
	_, err = s.c.manager.Store.Update(id, func(current *launch.Record) error {
		code := 0
		current.State = launch.Completed
		if runErr != nil {
			code, current.State = exit.ExitCode(), launch.Failed
			current.Reason = runErr.Error() + ": " + string(output)
		}
		current.ExitCode = &code
		current.FinishedAt = s.c.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
	return workProcessRef(30), err
}

func buildRoundResult(t *testing.T, c *connectionBed, starter *roundResultStarter, checks ...string) (string, string) {
	t.Helper()
	c.manager.Supervisor = starter
	check := filepath.Join(t.TempDir(), "check")
	body := "#!/bin/sh\ntest \"$(cat unit.txt)\" = built || exit 1\ntest ! -f fail-replay\n"
	if len(checks) > 0 {
		body = checks[0]
	}
	if err := testexec.WriteFile(check, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	code, result := c.do("work", "build", c.id, "result", "--brief", c.brief("brief.md", "Build the result.\n"), "--lines", "5", "--read-tool-calls", "12", "--check", check)
	if code != 0 {
		t.Fatalf("build exit=%d: %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	return run, connectionGit(t, c.worktree, "rev-parse", "HEAD")
}

func advanceRoundResult(t *testing.T, c *connectionBed, base, path, body string, local bool, expected ...string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "advance")
	connectionGit(t, c.root(), "worktree", "add", "--quiet", "--detach", dir, base)
	t.Cleanup(func() { connectionGit(t, c.root(), "worktree", "remove", "--force", dir) })
	writeUnitCarryFile(t, filepath.Join(dir, path), body)
	connectionGit(t, dir, "add", path)
	connectionGit(t, dir, "commit", "-qm", "independent advance\n\nGoal-Unit: "+c.id+"/advance")
	tip := connectionGit(t, dir, "rev-parse", "HEAD")
	push := []string{"push", "-q", "origin", tip + ":refs/heads/goal/" + c.id}
	if len(expected) > 0 {
		push = append(push, "--force-with-lease=refs/heads/goal/"+c.id+":"+expected[0])
	}
	connectionGit(t, dir, push...)
	if local {
		connectionGit(t, c.worktree, "update-ref", "refs/heads/goal/"+c.id, tip, base)
		writeUnitCarryFile(t, filepath.Join(c.worktree, path), body)
		connectionGit(t, c.worktree, "add", path)
	}
	return tip
}

func roundResultReview(t *testing.T, c *connectionBed) (int, intentResult) {
	t.Helper()
	return c.runJSON(c.connectionOwners(), "work", "review", c.id, "--work", "result")
}

func TestRoundResultReplayGitAdapter(t *testing.T) {
	t.Parallel()
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprint(local), func(t *testing.T) {
			t.Parallel()
			c := roundResultBed(t)
			starter := &roundResultStarter{c: c}
			run, base := buildRoundResult(t, c, starter)
			round := c.runRecord(run).Rounds[0]
			if round.Result == nil || round.Result.Parent != base || len(round.Result.Tree) != 40 || len(round.Result.PatchDigest) != 64 || len(round.Result.ProofIdentity) != 64 {
				t.Fatalf("builder result not recorded: %+v", round.Result)
			}
			tip := advanceRoundResult(t, c, base, "advance.txt", "unrelated\n", local)
			c.loseCommit, c.failPushes = true, 1
			code, result := roundResultReview(t, c)
			if code == 0 || result.Outcome != intentPartial || c.commits != 1 {
				t.Fatalf("lost commit and failed push: exit=%d commits=%d %+v", code, c.commits, result)
			}
			if resultData(t, result)["expectedParent"] != tip {
				t.Fatal("the review result hid the current publication parent")
			}
			subject := c.runRecord(run).Subjects[0]
			if subject.ExpectedParent != tip || subject.GateRunID == "" || len(starter.proofTrees) != 2 || starter.proofTrees[1] != subject.StagedTree || subject.StagedTree == round.Result.Tree {
				t.Fatalf("exact replay was not gated: %+v proof trees=%v", subject, starter.proofTrees)
			}
			code, result = roundResultReview(t, c)
			if result.Outcome != intentInProgress || c.commits != 1 || len(starter.proofTrees) != 2 || len(c.runRecord(run).Rounds) != 1 {
				t.Fatalf("repeat rebuilt or recommitted: exit=%d commits=%d %+v", code, c.commits, result)
			}
			if len(c.delegates) != 1 || slices.Contains(c.reads[0], "--unit-read") {
				t.Fatalf("replayed parent needs its own read: critics=%v reads=%v", c.delegates, c.reads)
			}
			c.writeCritic(c.worktree, "crit1", subject.Commit, "completed", true)
			code, result = roundResultReview(t, c)
			if code != 0 || c.commits != 1 || len(c.delegates) != 1 || len(starter.proofTrees) != 2 {
				t.Fatalf("critic collection repeated the publication: exit=%d %+v", code, result)
			}
			published := connectionGit(t, c.origin, "rev-parse", "refs/heads/goal/"+c.id)
			if connectionGit(t, c.worktree, "show", published+":unit.txt") != "built" || connectionGit(t, c.worktree, "show", published+":advance.txt") != "unrelated" {
				t.Fatal("publication lost the retained patch or branch advance")
			}
		})
	}
}

func TestRoundResultConflictGitAdapter(t *testing.T) {
	t.Parallel()
	c := roundResultBed(t)
	starter := &roundResultStarter{c: c}
	run, base := buildRoundResult(t, c, starter)
	tip := advanceRoundResult(t, c, base, "unit.txt", "conflicting\n", false)
	code, result := roundResultReview(t, c)
	if code == 0 || result.Next == nil || flagValue(result.Next.Argv, "--after") != "1" || flagValue(result.Next.Argv, "--work") != "result" || !slices.Contains(result.Next.Argv, "revise") || flagValue(result.Next.Argv, "--reason") == "" || flagValue(result.Next.Argv, "--by") == "" || c.commits != 0 {
		t.Fatalf("conflict correction is not bound: exit=%d %+v", code, result)
	}
	record := c.runRecord(run)
	if !strings.Contains(record.Subjects[0].Conflict, "unit.txt") || len(record.Rounds) != 1 || len(starter.proofTrees) != 1 || connectionGit(t, c.origin, "rev-parse", "refs/heads/goal/"+c.id) != tip {
		t.Fatalf("conflict evidence or custody lost: %+v", record)
	}
	if connectionGit(t, c.worktree, "rev-parse", "HEAD") != base || connectionGit(t, c.worktree, "diff", "--cached", "--name-only") != "unit.txt" {
		t.Fatal("conflict changed the retained checkout")
	}
	correction := slices.Clone(result.Next.Argv[1:])
	brief := c.brief("correction.md", "Resolve the retained publication conflict and keep the original requirement.\n")
	for i, word := range correction {
		if word == "FILE" {
			correction[i] = filepath.Join(c.root(), brief)
		} else if word == "NAME" {
			correction[i] = "Wido"
		}
	}
	c.edits["fix.txt"] = "correction\n"
	code, corrected := c.do(correction...)
	if code != 0 || len(c.runRecord(run).Rounds) != 2 || c.runRecord(run).Plan != record.Plan || len(c.runRecord(run).Revisions) != 1 || c.runRecord(run).Revisions[0].After != 1 {
		t.Fatalf("printed correction could not use the retained plan: exit=%d %+v", code, corrected)
	}
}

func TestRoundResultGateRedGitAdapter(t *testing.T) {
	t.Parallel()
	c := roundResultBed(t)
	starter := &roundResultStarter{c: c}
	run, base := buildRoundResult(t, c, starter)
	tip := advanceRoundResult(t, c, base, "fail-replay", "fails only in replay\n", false)
	code, result := roundResultReview(t, c)
	if code == 0 || c.commits != 0 || len(starter.proofTrees) != 2 || !strings.Contains(result.Summary, "publication checks failed") || result.Next == nil || flagValue(result.Next.Argv, "--after") != "1" || !slices.Contains(result.Next.Argv, "revise") {
		t.Fatalf("red replay published: exit=%d commits=%d %+v", code, c.commits, result)
	}
	if len(c.runRecord(run).Rounds) != 1 || connectionGit(t, c.worktree, "rev-parse", "HEAD") != base || connectionGit(t, c.origin, "rev-parse", "refs/heads/goal/"+c.id) != tip {
		t.Fatal("failed replay gate changed the branch or rebuilt")
	}
}

func TestRoundResultCorrectionGateGitAdapter(t *testing.T) {
	t.Parallel()
	c := roundResultBed(t)
	starter := &roundResultStarter{c: c}
	run, base := buildRoundResult(t, c, starter, "#!/bin/sh\nif [ \"$(cat unit.txt)\" = corrected ]; then test -f later.txt; else test \"$(cat unit.txt)\" = built; fi\n")
	_, first := roundResultReview(t, c)
	commit := resultData(t, first)["commit"].(string)
	c.writeCritic(c.worktree, "crit1", commit, "completed", true)
	if code, read := roundResultReview(t, c); code != 0 {
		t.Fatalf("initial publication: exit=%d %+v", code, read)
	}
	writeUnitCarryFile(t, filepath.Join(c.worktree, "later.txt"), "a later dependency\n")
	connectionGit(t, c.worktree, "add", "later.txt")
	tip, err := branch.CommitStaged(branch.CommitRequest{Repo: c.worktree, Remote: "origin", EndpointTip: base,
		GoalID: c.id, Unit: "later", OpID: "later-op", Kind: branch.Unit, CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	c.edits["unit.txt"] = "corrected\n"
	code, corrected := c.do("work", "revise", c.id, "--work", "result", "--after", "1", "--brief", c.brief("correction.md", "Correct the existing result.\n"), "--reason", "Correct explicitly", "--by", "Wido")
	if code != 0 || c.runRecord(run).Rounds[1].Outcome != "green" {
		t.Fatalf("combined correction did not pass: exit=%d %+v", code, corrected)
	}
	owners := c.connectionOwners()
	static := 0
	owners.connection.rebaseGate = func(dir string) (string, error) {
		static++
		body, err := os.ReadFile(filepath.Join(dir, "unit.txt"))
		if err != nil || string(body) != "corrected\n" {
			t.Fatalf("static gate did not see the correction: %q %v", body, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "later.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("static gate ran on the combined later tree")
		}
		return "static-green", nil
	}
	var scratch string
	starter.beforeProof = func(record launch.Record) { scratch = record.WorkingDirectory }
	code, held := c.runJSON(owners, "work", "review", c.id, "--work", "result")
	if code == 0 || static != 1 || c.commits != 1 || len(starter.proofTrees) != 3 || connectionGit(t, c.worktree, "rev-parse", "HEAD") != tip {
		t.Fatalf("correction installed without its own check: exit=%d static=%d commits=%d %+v", code, static, c.commits, held)
	}
	if scratch == "" {
		t.Fatal("correction check did not expose its scratch tree")
	}
	if _, err := os.Stat(filepath.Dir(scratch)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed correction scratch remains: %v", err)
	}
	if strings.Contains(connectionGit(t, c.root(), "worktree", "list", "--porcelain"), scratch) {
		t.Fatal("failed correction scratch remains registered")
	}

}

func TestRoundResultOriginRaceGitAdapter(t *testing.T) {
	t.Parallel()
	c := roundResultBed(t)
	starter := &roundResultStarter{c: c}
	run, base := buildRoundResult(t, c, starter)
	first := advanceRoundResult(t, c, base, "advance.txt", "first\n", true)
	owners := c.connectionOwners()
	commit := owners.connection.commit
	race := true
	owners.connection.commit = func(request branch.CommitRequest) (string, error) {
		if !race {
			return commit(request)
		}
		race = false
		moved := advanceRoundResult(t, c, base, "second.txt", "second\n", false, first)
		installed, err := commit(request)
		connectionGit(t, c.worktree, "push", "-q", "--force-with-lease=refs/heads/goal/"+c.id+":"+moved, "origin", first+":refs/heads/goal/"+c.id)
		return installed, err
	}
	code, result := c.runJSON(owners, "work", "review", c.id, "--work", "result")
	if code == 0 || result.Next == nil || slices.Contains(result.Next.Argv, "revise") || c.runRecord(run).Subjects[0].Conflict != "" || c.commits != 0 {
		t.Fatalf("origin race became a correction: exit=%d %+v", code, result)
	}
	code, result = c.runJSON(owners, "work", "review", c.id, "--work", "result")
	subject := c.runRecord(run).Subjects[0]
	if result.Outcome != intentInProgress || subject.Published == "" || subject.ExpectedParent != first || subject.Conflict != "" || c.commits != 1 || len(c.runRecord(run).Rounds) != 1 {
		t.Fatalf("repeat did not publish retained result: exit=%d %+v subject=%+v", code, result, subject)
	}
}

func TestRoundResultWaitResumeGitAdapter(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"cap", "start-retake", "wait-retake", "terminal-retake"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			c := roundResultBed(t)
			starter := &roundResultStarter{c: c}
			run, base := buildRoundResult(t, c, starter)
			advanceRoundResult(t, c, base, "advance.txt", "advance\n", false)
			starter.holdProof = boundary != "terminal-retake"
			changeRun := func() {
				record := c.runRecord(run)
				record.Notes = append(record.Notes, "another command updated the run while locks were released")
				data, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(c.unitRoot, run, "run.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var terminalScratch string
			if boundary == "start-retake" || boundary == "terminal-retake" {
				starter.beforeProof = func(record launch.Record) {
					starter.pending, terminalScratch = record.ID, record.WorkingDirectory
					changeRun()
					starter.beforeProof = nil
				}
			}
			if boundary == "wait-retake" {
				sleep := c.manager.Sleep
				changed := false
				c.manager.Sleep = func(d time.Duration) {
					sleep(d)
					if !changed {
						changed = true
						changeRun()
					}
				}
			}
			code, result := roundResultReview(t, c)
			subject := c.runRecord(run).Subjects[0]
			if result.Outcome != intentInProgress || code != 3 || result.Next == nil || slices.Contains(result.Next.Argv, "revise") || subject.GateWorktree == "" && boundary != "terminal-retake" || len(subject.GateLaunches) != 1 || subject.GateLaunches[0] != starter.pending {
				t.Fatalf("wait cap lost its check: exit=%d %+v subject=%+v", code, result, subject)
			}
			scratch := subject.GateWorktree
			if boundary == "terminal-retake" {
				scratch = terminalScratch
				if _, err := os.Stat(scratch); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("terminal wait kept scratch: %v", err)
				}
			} else if _, err := os.Stat(scratch); err != nil {
				t.Fatal(err)
			}
			pending, err := c.manager.Store.Read(starter.pending)
			if err != nil {
				t.Fatal(err)
			}
			// Finish the same recorded command after the injected clock exceeded the cap.
			if boundary != "terminal-retake" {
				state, err := c.manager.Store.StateDir(starter.pending)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := starter.finishProof(pending, state); err != nil {
					t.Fatal(err)
				}
			}
			launches := len(starter.proofTrees)
			code, result = roundResultReview(t, c)
			subject = c.runRecord(run).Subjects[0]
			if result.Outcome != intentInProgress || subject.Published == "" || c.commits != 1 || len(starter.proofTrees) != launches || len(subject.GateLaunches) != 1 {
				t.Fatalf("repeat launched another check: exit=%d %+v subject=%+v", code, result, subject)
			}
			if _, err := os.Stat(scratch); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("completed scratch remains: %v", err)
			}
			if strings.Contains(connectionGit(t, c.root(), "worktree", "list", "--porcelain"), scratch) {
				t.Fatal("completed scratch remains registered")
			}

		})
	}
}

func TestRoundResultTerminalCleanupGitAdapter(t *testing.T) {
	t.Parallel()
	c := roundResultBed(t)
	starter := &roundResultStarter{c: c}
	run, base := buildRoundResult(t, c, starter)
	advanceRoundResult(t, c, base, "fail-replay", "fails\n", false)
	var scratch string
	starter.beforeProof = func(record launch.Record) { scratch = record.WorkingDirectory }
	code, result := roundResultReview(t, c)
	if code == 0 || !strings.Contains(result.Summary, "publication checks failed") || scratch == "" || c.commits != 0 {
		t.Fatalf("expected terminal failure: exit=%d %+v", code, result)
	}
	if _, err := os.Stat(filepath.Dir(scratch)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed scratch folder remains: %v", err)
	}
	if strings.Contains(connectionGit(t, c.root(), "worktree", "list", "--porcelain"), scratch) {
		t.Fatal("failed scratch remains registered")
	}
	if c.runRecord(run).Subjects[0].GateWorktree != "" {
		t.Fatal("terminal subject still owns scratch")
	}
}

func TestRoundResultConcurrentBranchGitAdapter(t *testing.T) {
	t.Parallel()
	c := roundResultBed(t)
	starter := &roundResultStarter{c: c}
	run, base := buildRoundResult(t, c, starter)
	advanceRoundResult(t, c, base, "advance.txt", "advance\n", false)
	other := filepath.Join(t.TempDir(), "other")
	connectionGit(t, c.root(), "worktree", "add", "--quiet", "-b", "goal/other", other, base)
	t.Cleanup(func() { connectionGit(t, c.root(), "worktree", "remove", "--force", other) })
	starter.beforeProof = func(record launch.Record) {
		writeUnitCarryFile(t, filepath.Join(other, "other.txt"), "other goal\n")
		connectionGit(t, other, "add", "other.txt")
		connectionGit(t, other, "commit", "-qm", "another goal commits")
	}
	code, result := roundResultReview(t, c)
	if result.Outcome != intentInProgress || c.runRecord(run).Subjects[0].Published == "" || c.commits != 1 {
		t.Fatalf("shared branch move failed publication: exit=%d %+v", code, result)
	}
}
