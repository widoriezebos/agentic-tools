package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type laneFixBed struct {
	*connectionBed
	registered lane.Record
	owners     intentOwners
}

func writeLaneFixJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func newLaneFixBed(t *testing.T) *laneFixBed {
	t.Helper()
	c := &connectionBed{workBed: newWorkBed(t), t: t, edits: map[string]string{"code.go": "package fixture\n"}}
	root := c.root()
	connectionGit(t, root, "init", "-q", "-b", "main")
	connectionGit(t, root, "config", "user.name", "Fixture")
	connectionGit(t, root, "config", "user.email", "fixture@example.invalid")
	connectionGit(t, root, "config", "core.hooksPath", filepath.Join(t.TempDir(), "no-hooks"))
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/\n.claude/settings.local.json\nmetasystem.conf.local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	connectionGit(t, root, "add", "-A")
	connectionGit(t, root, "commit", "-qm", "fixture batch")
	head := connectionGit(t, root, "rev-parse", "HEAD")
	connectionGit(t, root, "checkout", "--detach", head)
	c.worktree, c.manager.Supervisor = root, c
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe %s: %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "landing", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "landing", "metasystem", lane.AgentLineage); err != nil {
		t.Fatal(err)
	}
	if _, err := lease.RequireHolder(root, pid, nil); err != nil {
		t.Fatal(err)
	}
	b := &laneFixBed{connectionBed: c, registered: lane.Record{Root: root, Install: root, CustodyEpoch: 1}}
	members := []plain.GoalSHA{{Goal: c.id, SHA: head}}
	dir := plain.Dir(root)
	writeLaneFixJSON(t, filepath.Join(dir, "batch.json"), plain.Batch{ID: "batch", Base: head, Lane: b.registered, State: plain.BatchRunning, Members: members})
	writeLaneFixJSON(t, filepath.Join(dir, "running.json"), plain.Running{Attempt: "attempt", Commit: head, Tree: connectionGit(t, root, "rev-parse", "HEAD^{tree}"), BatchID: "batch", BatchMembers: members})
	result := plain.Result{Attempt: "attempt", Commit: head, Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: c.id}, Failed: []plain.FailedUnit{{Unit: "package-a", Tests: []string{"TestBroken"}}, {Unit: "package-b"}}}
	writeLaneFixJSON(t, filepath.Join(dir, "results.jsonl"), result)
	b.owners = c.workOwners()
	b.owners.work.git = nil
	b.owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Manager: c.manager, Git: launch.OSGitRunner{}, Root: c.unitRoot}
	}
	b.owners.connection = intentConnectionOwners{
		laneFix: func(root, checkout, goal string) *plain.Fix {
			return landingFixForRegistered(root, checkout, goal, pid, b.registered)
		},
		endpoint: func(string) (goal.Endpoint, error) {
			return goal.Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}, nil
		},
		claimCheck: func(root, id string, _ goal.Endpoint) func() error {
			return func() error {
				if landingFixForRegistered(root, root, id, pid, b.registered) == nil {
					return fmt.Errorf("batch custody refused")
				}
				return nil
			}
		},
		endpointTip: func(string, goal.Endpoint) (string, error) {
			t.Fatal("lane review resolved goal branch tip")
			return "", nil
		},
		isolate:     func(string, string) error { t.Fatal("lane prepared goal worktree"); return nil },
		commitToken: func(_ string, commit func() error) error { return commit() },
		push: func(branch.PushRequest) (branch.PushResult, error) {
			t.Fatal("lane pushed a goal branch")
			return branch.PushResult{}, nil
		},
	}
	b.owners.delivery = &intentDeliveryOwners{
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			var commit, briefPath, briefSHA string
			var join bool
			for i, arg := range args {
				if arg == "--unit" {
					commit = args[i+1]
				}
				if arg == "--brief" {
					briefPath = args[i+1]
				}
				if arg == "--build-brief-sha256" {
					briefSHA = args[i+1]
				}
				if arg == "--join=true" {
					join = true
				}
			}
			if commit == "" || commit != connectionGit(t, root, "rev-parse", "HEAD") {
				t.Fatalf("read subject %q, args %q", commit, args)
			}
			if body := connectionGit(t, root, "show", commit+":code.go"); body != "package fixture" {
				t.Fatalf("read tree %q", body)
			}
			fix, err := plain.ActiveFix(root)
			if err != nil || fix == nil {
				t.Fatalf("fix %+v: %v", fix, err)
			}
			read, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: root, GoalID: c.id, UnitCommit: commit, EndpointTip: fix.Parent, BranchTip: commit, BriefPath: briefPath, BuildBriefSHA256: briefSHA, Join: join, CheckClaim: func() error { return nil },
				Gate: func(string) (string, error) { return "fixture-green-gate", nil },
				Delegate: func(brief, goal, subject, runtime, model string) (string, error) {
					contents, err := os.ReadFile(brief)
					if err != nil {
						t.Fatal(err)
					}
					if goal != c.id || subject != commit || !strings.Contains(string(contents), "git diff "+commit+"^ "+commit) || !strings.Contains(string(contents), "Repair the fixture.") || briefSHA == "" {
						t.Fatalf("wrong committed read: goal=%s subject=%s brief=%s", goal, subject, brief)
					}
					c.writeCritic(root, "lane-read", commit, "completed", true)
					return "lane-read", nil
				}})
			return read, 0, err
		},
	}
	return b
}

func (b *laneFixBed) run(args ...string) (int, intentResult) {
	b.t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		b.t.Fatalf("unknown command %q", args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, b.root(), b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("result %v: %s; stderr %s", err, &stdout, &stderr)
	}
	b.t.Logf("%s: exit %d, %s; %s", strings.Join(args, " "), code, result.Summary, &stderr)
	return code, result
}

func TestLaneFixBuildAndReviewBatchCustody(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	head := connectionGit(t, b.root(), "rev-parse", "HEAD")
	brief := filepath.Join(plain.Dir(b.root()), "fixes", "attempt", "brief.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte("Repair the fixture.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, built := b.run("work", "build", b.id, "--work", "lane-fix-1", "--brief", brief, "--lines", "2", "--check", "metasystem test impact")
	if code != 0 || resultData(t, built)["state"] != "awaiting-judgement" {
		t.Fatalf("build: %d %s", code, built.Summary)
	}
	fix, err := plain.ActiveFix(b.root())
	if err != nil || fix == nil || fix.State != "building" || fix.Commit != "" || fix.Job == "" || fix.Attempt != "attempt" || strings.Join(fix.Units, ",") != "package-a,package-b" {
		t.Fatalf("build record %+v: %v", fix, err)
	}
	work, err := b.owners.work.units(stateroot.Layout{}).NamedWork(b.root(), b.id)
	if err != nil || len(work) != 1 {
		t.Fatalf("work %+v: %v", work, err)
	}
	plan, err := launch.ReadUnitPlan(filepath.Join(work[0].Record.Rounds[0].Directory, "plan.json"))
	if err != nil || plan.Check == nil || plan.Check.Cheap != "metasystem test impact" || plan.Check.SelectedBy != "" {
		t.Fatalf("lane check %+v: %v", plan.Check, err)
	}
	if connectionGit(t, b.root(), "rev-parse", "HEAD") != head {
		t.Fatal("build committed before review")
	}
	if worktrees := connectionGit(t, b.root(), "worktree", "list", "--porcelain"); strings.Count(worktrees, "worktree ") != 1 {
		t.Fatalf("prepared goal worktree: %s", worktrees)
	}
	code, reviewed := b.run("work", "review", b.id, "--work", "lane-fix-1")
	if code != 0 || reviewed.Outcome != intentConfirmed {
		t.Fatalf("review: %d %s", code, reviewed.Summary)
	}
	commit := connectionGit(t, b.root(), "rev-parse", "HEAD")
	if files := connectionGit(t, b.root(), "diff", "--name-only", head, commit); files != "code.go" {
		t.Fatalf("fix changed files %q", files)
	}
	if parent := connectionGit(t, b.root(), "show", "-s", "--format=%P", commit); parent != head {
		t.Fatalf("fix parent %s", parent)
	}
	if count := connectionGit(t, b.root(), "rev-list", "--count", head+"..HEAD"); count != "1" {
		t.Fatalf("fix commits: %s", count)
	}
	want := "goal " + b.id + ": lane fix of package-a, package-b (fix round 1)\n\nGoal-Unit: " + b.id + "/lane-fix-1"
	if message := connectionGit(t, b.root(), "show", "-s", "--format=%B", "HEAD"); message != want {
		t.Fatalf("message %q, want %q", message, want)
	}
	if branches := connectionGit(t, b.root(), "for-each-ref", "--format=%(refname)", "refs/heads/goal/"); branches != "" {
		t.Fatalf("goal branch written: %s", branches)
	}
	fix, err = plain.ActiveFix(b.root())
	if err != nil || fix == nil || fix.Commit != commit || fix.Read != "lane-read" || fix.Verdict == "" || fix.State != "reviewing" {
		t.Fatalf("review record %+v: %v", fix, err)
	}
	code, _ = b.run("work", "review", b.id, "--work", "lane-fix-1")
	if code != 0 || connectionGit(t, b.root(), "rev-parse", "HEAD") != commit {
		t.Fatal("repeated review made another commit")
	}
	code, _ = b.run("work", "review", "--commit", commit[:12], "--goal", b.id)
	if code != 0 || connectionGit(t, b.root(), "rev-parse", "HEAD") != commit {
		t.Fatal("commit-form review changed the batch")
	}
	retries := 0
	b.owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		if !strings.Contains(strings.Join(args, " "), "--retry 1") {
			t.Fatalf("retry dropped: %q", args)
		}
		retries++
		return branch.BranchReadResult{}, 1, fmt.Errorf("fixture retry reached the committed-read owner")
	}
	for _, args := range [][]string{
		{"work", "review", b.id, "--work", "lane-fix-1", "--retry", "1"},
		{"work", "review", "--commit", commit[:12], "--goal", b.id, "--retry", "1"},
	} {
		code, result := b.run(args...)
		if code != 1 || !strings.Contains(result.Summary, "fixture retry reached") {
			t.Fatalf("retry: %d %s", code, result.Summary)
		}
	}
	if retries != 2 {
		t.Fatalf("retry calls: %d", retries)
	}
}

func TestLaneFixCustodyRejectsOutsidersAndStaleAttempts(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"non-member", "seat", "stale attempt", "attached branch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newLaneFixBed(t)
			switch name {
			case "non-member":
				batch, err := plain.ReadBatch(b.root())
				if err != nil {
					t.Fatal(err)
				}
				batch.Members[0].Goal = "other"
				writeLaneFixJSON(t, filepath.Join(plain.Dir(b.root()), "batch.json"), batch)
				running, _, _, _ := plain.ReadRunning(b.root(), plain.ProveSeams{})
				running.BatchMembers = batch.Members
				writeLaneFixJSON(t, filepath.Join(plain.Dir(b.root()), "running.json"), running)
			case "seat":
				b.owners.connection.laneFix = func(root, checkout, id string) *plain.Fix {
					return landingFixForRegistered(root, checkout, id, int64(os.Getppid()), b.registered)
				}
				b.owners.connection.claimCheck = func(root, id string, _ goal.Endpoint) func() error {
					return func() error {
						if b.owners.connection.laneFix(root, root, id) == nil {
							return fmt.Errorf("seat has no batch custody")
						}
						return nil
					}
				}
			case "stale attempt":
				running, _, _, _ := plain.ReadRunning(b.root(), plain.ProveSeams{})
				running.Attempt = "old"
				writeLaneFixJSON(t, filepath.Join(plain.Dir(b.root()), "running.json"), running)
			case "attached branch":
				connectionGit(t, b.root(), "checkout", "-b", "other")
			}
			code, result := b.run("work", "build", b.id, "--work", "lane-fix-1", "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact")
			if code == 0 || result.Outcome != intentRefused {
				t.Fatalf("unauthorized build: %d %s", code, result.Summary)
			}
			if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
				t.Fatalf("builder ran: %v", err)
			}
		})
	}
}

func TestLaneFixCheckDoesNotGrantManualOverride(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	code, result := b.run("work", "build", b.id, "--work", "lane-fix-1", "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "/usr/bin/true")
	if code == 0 || !strings.Contains(result.Summary, "manual check needs") {
		t.Fatalf("manual override: exit %d, %s", code, result.Summary)
	}
	if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
		t.Fatalf("builder ran: %v", err)
	}
}
