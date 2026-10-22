package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type laneFixBed struct {
	*connectionBed
	registered lane.Record
	registry   string
	hookTrace  string
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
	return newLaneFixBedWith(t, workApprovedBox)
}

func newLaneFixBedWith(t *testing.T, amend func(*goal.GoalFile)) *laneFixBed {
	t.Helper()
	c := &connectionBed{workBed: newWorkBedWith(t, amend), t: t, edits: map[string]string{"code.go": "package fixture\n"}}
	root := c.root()
	connectionGit(t, root, "init", "-q", "-b", "main")
	connectionGit(t, root, "config", "user.name", "Fixture")
	connectionGit(t, root, "config", "user.email", "fixture@example.invalid")
	connectionGit(t, root, "config", "core.hooksPath", filepath.Join(t.TempDir(), "no-hooks"))
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/\n.claude/settings.local.json\nmetasystem.conf.local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(root, "plans", "goals", "backlog.md")
	raw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	rootRecord, problems := goal.ParseRoot(raw)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	rootRecord.SyncMode = goal.SyncRemote
	if err := os.WriteFile(rootPath, goal.RenderRoot(rootRecord), 0600); err != nil {
		t.Fatal(err)
	}
	connectionGit(t, root, "add", "-A")
	connectionGit(t, root, "commit", "-qm", "fixture batch")
	base := connectionGit(t, root, "rev-parse", "HEAD")
	connectionGit(t, root, "update-ref", goal.AcceptedRef, base)
	connectionGit(t, root, "checkout", "-b", "goal/"+c.id)
	if err := os.WriteFile(filepath.Join(root, "broken.txt"), []byte("broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	connectionGit(t, root, "add", "broken.txt")
	connectionGit(t, root, "commit", "-qm", "member")
	member := connectionGit(t, root, "rev-parse", "HEAD")
	connectionGit(t, root, "checkout", "--detach", base)
	connectionGit(t, root, "merge", "--no-ff", "-m", "batch", "goal/"+c.id)
	connectionGit(t, root, "branch", "-d", "goal/"+c.id)
	head := connectionGit(t, root, "rev-parse", "HEAD")
	origin := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, root, "init", "--bare", origin)
	connectionGit(t, root, "remote", "add", "origin", origin)
	connectionGit(t, root, "push", "origin", base+":refs/heads/main")
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
	members := []plain.GoalSHA{{Goal: c.id, SHA: member}}
	if _, _, err := plain.HandIn(root, plain.Line{Goal: c.id, Branch: "goal/" + c.id, SHA: member, Seat: "seat"}); err != nil {
		t.Fatal(err)
	}
	dir := plain.Dir(root)
	writeLaneFixJSON(t, filepath.Join(dir, "batch.json"), plain.Batch{ID: "batch", Base: base, Lane: b.registered, State: plain.BatchRunning, Members: members})
	result, err := plain.Run(root, root, "fixture", "", io.Discard, plain.ProveSeams{Now: func() time.Time { return time.Now() }, Command: func(cmd *exec.Cmd) error {
		for _, env := range cmd.Env {
			if env == "LANDING_COMMIT="+head {
				fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tpackage-a\tTestBroken\nLANDING-FAILED\tpackage-b\tTestBrokenB\nLANDING-CHECKED\t2\n")
				return fmt.Errorf("red")
			}
		}
		fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
		return nil
	}})
	if err != nil || result.Result != plain.Red || result.Cause == nil || result.Cause.Kind != "own" {
		t.Fatalf("real red %+v: %v", result, err)
	}
	registry := t.TempDir()
	b.registry, b.hookTrace = registry, filepath.Join(t.TempDir(), "hook-trace")
	writeLaneFixJSON(t, lane.RecordPath(filepath.Join(registry, ".metasystem")), b.registered)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hookDir := filepath.Join(root, ".git", "hooks")
	hook := "#!/bin/sh\necho hook >> " + shellQuote(b.hookTrace) + "\nexport GO_WANT_BATCH_E2E_COMMAND=1\nexport METASYSTEM_SUPERVISION_REGISTRY_HOME=" + shellQuote(registry) + "\nexec " + shellQuote(executable) + " internal pre-commit --root " + shellQuote(root) + "\n"
	if err := testexec.WriteFile(filepath.Join(hookDir, "pre-commit"), []byte(hook), 0755); err != nil {
		t.Fatal(err)
	}
	connectionGit(t, root, "config", "core.hooksPath", hookDir)
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

func (b *laneFixBed) fixUnit() string {
	b.t.Helper()
	running, _, _, err := plain.ReadRunning(b.root(), plain.ProveSeams{})
	if err != nil {
		b.t.Fatal(err)
	}
	if fix, err := plain.FixForAttempt(b.root(), running.Attempt); err == nil && fix != nil {
		return fix.Unit()
	}
	return (plain.Fix{Attempt: running.Attempt, Round: 1}).Unit()
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
	brief := filepath.Join(plain.Dir(b.root()), "fixes", "brief.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte("Repair the fixture.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, built := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", brief, "--lines", "2", "--check", "metasystem test impact")
	if code != 0 || resultData(t, built)["state"] != "awaiting-judgement" {
		t.Fatalf("build: %d %s", code, built.Summary)
	}
	fix, err := plain.ActiveFix(b.root())
	if err != nil || fix == nil || fix.State != "building" || fix.Commit != "" || fix.Job == "" || fix.Attempt == "" || strings.Join(fix.Units, ",") != "package-a,package-b" {
		t.Fatalf("build record %+v: %v", fix, err)
	}
	work, err := b.owners.work.units(stateroot.Layout{}).NamedWork(b.root(), b.id)
	if err != nil || len(work) != 1 {
		t.Fatalf("work %+v: %v", work, err)
	}
	plan, err := launch.ReadUnitPlan(filepath.Join(work[0].Record.Rounds[0].Directory, "plan.json"))
	if err != nil || plan.Check == nil || plan.Check.Cheap != "metasystem test impact --check" || plan.Check.SelectedBy != "" {
		t.Fatalf("lane check %+v: %v", plan.Check, err)
	}
	if connectionGit(t, b.root(), "rev-parse", "HEAD") != head {
		t.Fatal("build committed before review")
	}
	if worktrees := connectionGit(t, b.root(), "worktree", "list", "--porcelain"); strings.Count(worktrees, "worktree ") != 1 {
		t.Fatalf("prepared goal worktree: %s", worktrees)
	}
	code, reviewed := b.run("work", "review", b.id, "--work", b.fixUnit())
	if trace, err := os.ReadFile(b.hookTrace); err != nil || !strings.Contains(string(trace), "hook") {
		t.Fatalf("engine commit skipped real hook: %s %v", trace, err)
	}
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
	want := "goal " + b.id + ": lane fix of package-a, package-b (fix round 1)\n\nGoal-Unit: " + b.id + "/" + b.fixUnit()
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
	code, _ = b.run("work", "review", b.id, "--work", b.fixUnit())
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
		{"work", "review", b.id, "--work", b.fixUnit(), "--retry", "1"},
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
				seat := t.TempDir()
				pid := int64(os.Getpid())
				exact, _, err := (identity.KernelProber{}).Probe(pid)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := lease.AnnounceWithPair(seat, "seat", pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "seat", "metasystem", "ordinary-seat"); err != nil {
					t.Fatal(err)
				}
				if _, err := lease.RequireHolder(seat, pid, nil); err != nil {
					t.Fatal(err)
				}
				b.owners.connection.laneFix = func(_, checkout, id string) *plain.Fix {
					return landingFixForRegistered(seat, checkout, id, pid, b.registered)
				}
				b.owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
					return func() error { return fmt.Errorf("seat has no batch custody") }
				}
			case "stale attempt":
				running, _, _, _ := plain.ReadRunning(b.root(), plain.ProveSeams{})
				running.Attempt = "old"
				writeLaneFixJSON(t, filepath.Join(plain.Dir(b.root()), "running.json"), running)
			case "attached branch":
				connectionGit(t, b.root(), "checkout", "-b", "other")
			}
			code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact")
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
	code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "/usr/bin/true")
	if code == 0 || !strings.Contains(result.Summary, "manual check needs") {
		t.Fatalf("manual override: exit %d, %s", code, result.Summary)
	}
	if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
		t.Fatalf("builder ran: %v", err)
	}
}

func TestLaneFixBuildRefusesPrimaryCheckoutWithSelectedLane(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	primary := filepath.Join(t.TempDir(), "primary")
	connectionGit(t, b.root(), "worktree", "add", primary, "main")
	original := b.owners.resolver
	b.owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
		if withinPath(path, primary) {
			return primary, nil
		}
		return b.root(), nil
	}, noExecutable)
	// Selecting the lane installation must not turn the caller's primary checkout into the lane.
	command, rest, _ := resolveIntentArgv([]string{"work", "build", b.id, "--repo", b.root(), "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"})
	b.owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
		return func() error { return fmt.Errorf("the live seat holds the goal") }
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, primary, b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code == 0 || !strings.Contains(result.Summary, "live seat holds") {
		t.Fatalf("primary build: exit %d %s; %s", code, &stdout, &stderr)
	}
	if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
		t.Fatalf("builder ran: %v", err)
	}
	b.owners.resolver = original
}

func TestLaneFixBuildAllowanceCannotRestartOrRebuild(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"building", "closed", "stale record"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newLaneFixBed(t)
			running, _, _, _ := plain.ReadRunning(b.root(), plain.ProveSeams{})
			fix := plain.Fix{Attempt: running.Attempt, Parent: running.Commit, Goal: b.id, Round: 1, Units: []string{"package-a"}, Job: "first-build", State: "building"}
			if name == "closed" {
				fix.State = "closed"
			}
			if name == "stale record" {
				fix.Attempt = "older"
				if err := plain.WriteFix(b.root(), &fix); err != nil {
					t.Fatal(err)
				}
				active, err := plain.ActiveFix(b.root())
				retained, readErr := plain.FixForAttempt(b.root(), "older")
				if err != nil || readErr != nil || active != nil || retained == nil || retained.State != "building" {
					t.Fatalf("stale fix %+v retained %+v: %v %v", active, retained, err, readErr)
				}
				return
			}
			if err := plain.WriteFix(b.root(), &fix); err != nil {
				t.Fatal(err)
			}
			code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact")
			if code == 0 {
				t.Fatalf("second builder ran: %s", result.Summary)
			}
			if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
				t.Fatalf("builder ran: %v", err)
			}
		})
	}
}

func init() {
	testHelperCommands["fixture-lane-claim-owner"] = func(args []string) int {
		root, id := args[0], args[1]
		if landingFixFor(root, root, id, int64(os.Getpid())) == nil {
			fmt.Fprintln(os.Stderr, "fixture lacks authenticated lane custody")
			return 2
		}
		called := false
		err := goalBranchClaimCheckWith(root, id, goal.Endpoint{}, func(string, string) (string, error) { called = true; return "", nil }, func(string) string { return root })()
		if !called || err == nil || !strings.Contains(err.Error(), "this machine has no name") {
			fmt.Fprintf(os.Stderr, "generic claim borrowed custody: %v\n", err)
			return 1
		}
		return 0
	}
}

func init() {
	testHelperCommands["fixture-lane-read-owner"] = func(args []string) int {
		root, id, commit := args[0], args[1], args[2]
		result, _, err := goalBranchReadRun([]string{"--root", root, "--goal", id, "--unit", commit}, goalBranchReadDependencies{
			Gate: func(string) (string, error) { return "green", nil },
			Delegate: func(string, string, string, string, string) (string, error) {
				return "", fmt.Errorf("lane committed read reached delegate")
			},
		})
		if err != nil || result.RootJob != "lane-read" {
			fmt.Fprintf(os.Stderr, "lane read: %+v %v\n", result, err)
			return 1
		}
		return 0
	}
}

func TestLaneFixCommittedReadOwnerRealProcess(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	if code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 0 {
		t.Fatalf("build %d %s", code, result.Summary)
	}
	if code, result := b.run("work", "review", b.id, "--work", b.fixUnit()); code != 0 {
		t.Fatalf("review %d %s", code, result.Summary)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture := pinProofBinaryFixture(t, b.root())
	cmd := fixture.command(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry), executable, "fixture-lane-read-owner", b.root(), b.id, connectionGit(t, b.root(), "rev-parse", "HEAD"))
	cmd.Dir = b.root()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("committed read owner: %v %s", err, output)
	}
}

func TestLaneFixGenericClaimOwnerRealProcessRefusesCustody(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture := pinProofBinaryFixture(t, b.root())
	cmd := fixture.command(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry), executable, "fixture-lane-claim-owner", b.root(), b.id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generic claim owner: %v %s", err, output)
	}
}

func readLaneFixGoalFixture(t *testing.T) *goal.GoalFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "lane_fix_goal.md"))
	if err != nil {
		t.Fatalf("lane fix goal fixture can't be read: %v", err)
	}
	file, problems := goal.ParseFile(raw)
	if len(problems) > 0 || file.Budget == nil {
		t.Fatalf("lane fix goal fixture has no valid budget: %v", problems)
	}
	return file
}

func TestLaneFixLapsedGoalBoxAdmitsBuild(t *testing.T) {
	t.Parallel()
	realGoal := readLaneFixGoalFixture(t)
	b := newLaneFixBedWith(t, func(file *goal.GoalFile) {
		box := *realGoal.Budget
		file.Budget = &box
		file.Budget.ElapsedLimit = "1m"
		file.Budget.ReviewRoundLimit = 0
		file.Budget.AttemptLimit = 1
		file.Budget.ReservedJobMinutesLimit = 1
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	contract, err := os.ReadFile(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	realContract, err := testpolicy.Decode(contract)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := testpolicy.Select(realContract, testpolicy.SelectionRequest{
		ChangedPaths:  []string{"metasystem/cmd/metasystem/intent_lane_fix.go", "metasystem/internal/config/defaults.go", "metasystem/skills/landing-agent/SKILL.md"},
		GoalRisk:      testpolicy.GoalRisk{Severity: int(realGoal.Risk.Severity), Novelty: int(realGoal.Risk.Novelty), Exposure: int(realGoal.Risk.Exposure), Accumulation: int(realGoal.Risk.Accumulation)},
		RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic,
	})
	if err != nil {
		t.Fatal(err)
	}
	output := testrun.PlanOutputOf(testrun.Preparation{EffectiveContract: realContract, Plan: selected})
	if len(output.Groups) == 0 || len(output.Plan.RequiredGroups) == 0 {
		t.Fatalf("real testing plan is empty: %+v", output.Plan)
	}
	t.Logf("real testing plan mode=%s required=%v", output.Plan.ExecutedMode, output.Plan.RequiredGroups)
	if err := os.WriteFile(filepath.Join(b.root(), "testing.json"), contract, 0600); err != nil {
		t.Fatal(err)
	}
	before := b.goalFile(b.id)
	spending := dispatchcore.ProjectBudget(b.root(), before, b.manager.Now())
	if code, ordinary := b.run("work", "build", b.id, "--work", "ordinary", "--brief", b.brief("ordinary.md", "Build ordinary work.\n"), "--lines", "2"); code == 0 || !strings.Contains(ordinary.Summary, "not approved with a budget yet") {
		t.Fatalf("ordinary work escaped goal admission: %s", ordinary.Summary)
	}
	args := []string{"work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"}
	code, result := b.run(args...)
	if code != 0 || strings.Contains(result.Summary, "not approved with a budget yet") {
		t.Fatalf("lane allowance refused: exit %d %s", code, result.Summary)
	}
	after := b.goalFile(b.id)
	charged := dispatchcore.ProjectBudget(b.root(), after, b.manager.Now())
	if !reflect.DeepEqual(before, after) || charged.Attempts != spending.Attempts || charged.ReservedJobMinutes != spending.ReservedJobMinutes || charged.ActiveJobs != spending.ActiveJobs {
		t.Fatalf("lane repair spent goal budget: before=%+v after=%+v", spending, charged)
	}
	runs, err := b.owners.work.units(stateroot.Layout{}).NamedWork(b.root(), b.id)
	if err != nil || len(runs) != 1 {
		t.Fatalf("plan runs %+v: %v", runs, err)
	}
	plan, err := launch.ReadUnitPlan(filepath.Join(runs[0].Record.Rounds[0].Directory, "plan.json"))
	if err != nil || plan.Check == nil || plan.Check.Cheap != "metasystem test impact --check" || runs[0].Record.MaxRounds != 1 {
		t.Fatalf("real build plan %+v: %v", plan, err)
	}
	t.Logf("real contract schema, captured ledger budget and emitted plan: unit=%s check=%s", plan.Unit, plan.Check.Cheap)
	records, err := b.manager.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		var text string
		if err := json.Unmarshal(record.AdapterData["deadline"], &text); err != nil {
			t.Fatal(err)
		}
		deadline, err := time.Parse(time.RFC3339Nano, text)
		started, _ := time.Parse(time.RFC3339Nano, record.StartedAt)
		if err != nil || deadline.Sub(started) != 120*time.Minute {
			t.Fatalf("lane job's box: %s %s %v", record.ID, text, err)
		}
	}
	if code, result := b.run(args...); code != 0 {
		t.Fatalf("idempotent repeat: %s", result.Summary)
	}
	repeated, _ := b.manager.Store.List()
	if len(repeated) != len(records) {
		t.Fatal("repeat launched a second repair")
	}
}

func init() {
	testHelperCommands["fixture-lane-real-read-dispatch"] = func(args []string) int {
		root, id, commit, brief := args[0], args[1], args[2], args[3]
		read, code, err := goalBranchReadRun([]string{"--root", root, "--goal", id, "--unit", commit, "--brief", brief}, goalBranchReadDependencies{
			Gate: func(string) (string, error) { return "fixture-green", nil },
			Delegator: func(request delegateRequest, stdout, stderr io.Writer) int {
				return runDelegateWith(request, stdout, io.MultiWriter(stderr, os.Stderr))
			},
		})
		if err != nil || code != 0 || read.RootJob == "" {
			fmt.Fprintf(os.Stderr, "real lane read dispatch: %+v code=%d err=%v\n", read, code, err)
			return 1
		}
		fmt.Fprintln(os.Stdout, read.RootJob)
		return 0
	}
}

// Real Git, custody, the committed-read owner, dispatch admission and the fake
// runtime adapter prove a lapsed goal cannot deny or pay for the lane's read.
func TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch(t *testing.T) {
	t.Parallel()
	realGoal := readLaneFixGoalFixture(t)
	b := newLaneFixBedWith(t, func(file *goal.GoalFile) {
		box := *realGoal.Budget
		box.ElapsedLimit, box.ReviewRoundLimit = "1m", 0
		file.Budget = &box
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, box, file.Risk)
	})
	brief := b.brief("fix.md", "Repair the fixture.\n")
	if code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", brief, "--lines", "2", "--check", "metasystem test impact"); code != 0 {
		t.Fatalf("build: %d %s", code, result.Summary)
	}
	// Commit through the repair owner, leaving its read for the real dispatch.
	b.owners.delivery.branchRead = func([]string) (branch.BranchReadResult, int, error) { return branch.BranchReadResult{}, 0, nil }
	if _, result := b.run("work", "review", b.id, "--work", b.fixUnit()); result.Outcome != intentInProgress {
		t.Fatalf("commit: %s", result.Summary)
	}
	commit := connectionGit(t, b.root(), "rev-parse", "HEAD")
	before := dispatchcore.ProjectBudget(b.root(), b.goalFile(b.id), time.Now())
	if before.Elapsed < time.Minute {
		t.Fatalf("fixture has no elapsed breach: %+v", before)
	}
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range []string{"docs", "skills"} {
		if output, err := exec.Command("cp", "-R", filepath.Join(module, asset), b.root()).CombinedOutput(); err != nil {
			t.Fatalf("install %s: %v %s", asset, err, output)
		}
	}
	enginePath := filepath.Join(b.root(), "bin", "metasystem")
	if err := os.Remove(enginePath); err != nil {
		t.Fatal(err)
	}
	engine := testenv.Link(t, testenv.Engine(t), enginePath)
	engineBytes, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	writeLaneFixJSON(t, filepath.Join(b.root(), "artifacts", "agents", "steward", "identity.json"), map[string]any{
		"repoIdentity": b.root(), "generation": 1, "installPath": engine,
		"installDigest": fmt.Sprintf("sha256:%x", sha256.Sum256(engineBytes)), "mintedAt": "1970-01-01T00:00:00Z", "enrollment": "fixture",
	})
	conf := filepath.Join(b.root(), "metasystem.conf")
	configBytes, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	configBytes = append(configBytes, []byte("\nrole.code-critic.runtime=fake\nrole.code-critic.model.fake=fake-model\nlanding.fix-minutes=7\nmetasystem.budget.review-round-max=6\nwatch.interval-sec=600\nevidence.root="+t.TempDir()+"\n")...)
	if err := os.WriteFile(conf, configBytes, 0600); err != nil {
		t.Fatal(err)
	}
	fixture := pinProofBinaryFixture(t, b.root())
	env := append(os.Environ(), "HOME="+b.registry, "METASYSTEM_BIN="+engine, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry, "METASYSTEM_AGENT_RUNTIME=fake")
	arm := []string{"up", "--metasystem-root", b.root(), "--repo", b.root()}
	// The dispatched job's supervisor runs in a process group of its own that
	// no shutdown stops; it writes under the root until it ends, so the root
	// is removed only after the whole group is gone.
	jobGroup := 0
	t.Cleanup(func() {
		cmd := fixture.command(env, engine, append(arm, "--shutdown")...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("shutdown: %v %s", err, output)
		}
		if _, err := steward.Disarm(b.root()); err != nil {
			t.Errorf("disarm fixture steward: %v", err)
		}
		if jobGroup > 0 {
			if err := testenv.AwaitProcessTargetGone(-jobGroup); err != nil {
				t.Errorf("dispatched job: %v", err)
			}
		}
	})
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe fixture holder: %s %v", state, err)
	}
	start := append(append([]string{}, arm...), "--session", "landing", "--pid", fmt.Sprint(exact.Pid), "--start-time", fmt.Sprint(exact.StartedAt.Unix()), "--tag", "landing", "--owner-lineage", lane.AgentLineage)
	upRun := fixture.command(env, engine, start...)
	var upOutput bytes.Buffer
	upRun.Stdout, upRun.Stderr = &upOutput, &upOutput
	if err := upRun.Start(); err != nil {
		t.Fatalf("arm real dispatcher: %v", err)
	}
	upDone := make(chan struct{})
	var upErr error
	go func() { upErr = upRun.Wait(); close(upDone) }()
	upExited := func() bool {
		select {
		case <-upDone:
			return true
		default:
			return false
		}
	}
	t.Cleanup(func() {
		if !upExited() {
			_ = upRun.Process.Kill()
			<-upDone
		}
	})
	supervision := filepath.Join(b.root(), "artifacts", "agents", "supervision")
	var watcherPid int
	testenv.AwaitFrom(t, "the first watcher heartbeat", func() bool {
		var state struct {
			Components map[string]struct {
				Pid       int    `json:"pid"`
				Heartbeat string `json:"heartbeat"`
			} `json:"components"`
		}
		data, err := os.ReadFile(filepath.Join(supervision, "state.json"))
		if err != nil || json.Unmarshal(data, &state) != nil {
			return false
		}
		watcher := state.Components["watcher"]
		if watcher.Pid <= 0 {
			return false
		}
		if _, err := os.Stat(watcher.Heartbeat); err != nil {
			return false
		}
		watcherPid = watcher.Pid
		return true
	}, func() (bool, string) {
		if upExited() {
			return false, upOutput.String()
		}
		return true, ""
	}, nil)
	if err := syscall.Kill(watcherPid, syscall.SIGUSR1); err != nil {
		t.Fatalf("wake the watcher: %v", err)
	}
	testenv.Await(t, "the arming of the real dispatcher", upExited)
	if upErr != nil {
		t.Fatalf("arm real dispatcher: %v %s", upErr, upOutput.String())
	}
	var census struct {
		ScanSeq int64 `json:"scanSeq"`
	}
	testenv.Await(t, "the census the wake asked for", func() bool {
		data, err := os.ReadFile(filepath.Join(supervision, "last-census.json"))
		return err == nil && json.Unmarshal(data, &census) == nil && census.ScanSeq >= 2
	})
	t.Logf("woken watcher census: scanSeq=%d", census.ScanSeq)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := fixture.command(append(env, "GO_WANT_BATCH_E2E_COMMAND=1"), executable, "fixture-lane-real-read-dispatch", b.root(), b.id, commit, brief)
	cmd.Dir = b.root()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real dispatch: %v %s", err, output)
	}
	job := lastOutputLine(output)
	var record map[string]any
	data, err := os.ReadFile(filepath.Join(b.root(), "artifacts", "agents", "jobs", job+".json"))
	if err != nil || json.Unmarshal(data, &record) != nil {
		t.Fatalf("dispatched job %q: %v", job, err)
	}
	if pgid, _ := record["pgid"].(float64); pgid > 0 {
		jobGroup = int(pgid)
	} else {
		t.Fatalf("dispatched job %q records no process group: %v", job, record["pgid"])
	}
	if record["goalId"] != nil || record["reviewRoundLimit"] != float64(6) || record["capMin"] != float64(7) || record["reviews"] != "commit:"+commit || record["runtime"] != "fake" || record["requestedModel"] != "fake-model" {
		t.Fatalf("lane read allowance/roster: %+v", record)
	}
	data, err = os.ReadFile(filepath.Join(b.root(), ".git", "metasystem", "goal-reads", b.id, commit+".json"))
	if err != nil || json.Unmarshal(data, &record) != nil || record["goal"] != b.id || record["unitCommit"] != commit || record["rootJob"] != job {
		t.Fatalf("read lost goal/unit identity: %s %v", data, err)
	}
	after := dispatchcore.ProjectBudget(b.root(), b.goalFile(b.id), time.Now())
	if after.CodeCritiques != before.CodeCritiques || after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes {
		t.Fatalf("read charged the goal: before=%+v after=%+v", before, after)
	}
	t.Logf("real committed read admitted: goal=%s commit=%s job=%s, cap=7m rounds=6", b.id, commit, job)
}

func TestLaneFixGreenAndDisabledAllowanceAdmitNoBuild(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"green", "disabled", "wrong unit"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newLaneFixBed(t)
			unit := b.fixUnit()
			switch name {
			case "green":
				result, _, err := plain.LastResult(b.root())
				if err != nil {
					t.Fatal(err)
				}
				result.Result = plain.Green
				data, _ := json.Marshal(result)
				if err := os.WriteFile(filepath.Join(plain.Dir(b.root()), "results.jsonl"), append(data, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				file, err := os.OpenFile(filepath.Join(b.root(), "metasystem.conf"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.WriteString("\nlanding.fix-rounds=0\n")
				if closeErr := file.Close(); err != nil || closeErr != nil {
					t.Fatalf("setting: %v %v", err, closeErr)
				}
			case "wrong unit":
				unit = "lane-fix-20261011T000000-1"
			}
			code, result := b.run("work", "build", b.id, "--work", unit, "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact")
			if code == 0 {
				t.Fatalf("unauthorized build: %s", result.Summary)
			}
			if _, err := os.Stat(filepath.Join(b.root(), "code.go")); !os.IsNotExist(err) {
				t.Fatal("builder ran without an own red allowance")
			}
		})
	}
}

func TestLaneFixSpentReturnAndChangedHandInFreshIdentity(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	firstUnit := b.fixUnit()
	if code, result := b.run("work", "build", b.id, "--work", firstUnit, "--brief", b.brief("first.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 0 {
		t.Fatalf("first build: %s", result.Summary)
	}
	fix, err := plain.ActiveFix(b.root())
	if err != nil || fix == nil {
		t.Fatalf("fix %+v: %v", fix, err)
	}
	entry, changed, err := plain.ReturnProven(b.root(), b.id, "own", "still red", false, "", time.Now())
	want := "the lane's 1 fix round for proof " + fix.RepairProof() + " is spent"
	if err != nil || !changed || !strings.Contains(entry.Reason, want) || !strings.Contains(entry.Reason, fix.Job) {
		t.Fatalf("spent return %+v: %v", entry, err)
	}
	// The stopped repair's existing owner releases the checkout before the next hand-in.
	runner := b.owners.work.units(stateroot.Layout{})
	if _, err := runner.CancelRun(fix.Job); err != nil {
		t.Fatal(err)
	}
	// The changed hand-in belongs to a fresh batch and proof on the same checkout.
	head := connectionGit(t, b.root(), "rev-parse", "HEAD")
	if head == entry.SHA {
		t.Fatal("fixture hand-in did not change")
	}
	if _, changed, err := plain.HandIn(b.root(), plain.Line{Goal: b.id, Branch: "goal/" + b.id, SHA: head, Seat: "seat"}); err != nil || !changed {
		t.Fatalf("changed hand-in: %v", err)
	}
	batch, err := plain.ReadBatch(b.root())
	if err != nil {
		t.Fatal(err)
	}
	batch.ID, batch.State, batch.Members = "batch-again", plain.BatchRunning, []plain.GoalSHA{{Goal: b.id, SHA: head}}
	running := plain.Running{Attempt: "20261012T040737.033326000Z", Commit: head, Tree: "new-batch-tree", BatchID: batch.ID, BatchMembers: batch.Members, Checkpoint: true}
	red := plain.Result{Attempt: running.Attempt, Commit: head, Tree: running.Tree, BatchID: batch.ID, BatchMembers: batch.Members, Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: b.id, SHA: head}, Failed: []plain.FailedUnit{{Unit: "package-a"}}}
	writeLaneFixJSON(t, filepath.Join(plain.Dir(b.root()), "batch.json"), batch)
	writeLaneFixJSON(t, filepath.Join(plain.Dir(b.root()), "running.json"), running)
	data, _ := json.Marshal(red)
	if err := os.WriteFile(filepath.Join(plain.Dir(b.root()), "results.jsonl"), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	unit := b.fixUnit()
	if unit == firstUnit || unit != "lane-fix-20261012T040737-1" {
		t.Fatalf("episode identity %q equals %q", unit, firstUnit)
	}
	// Make the next repair's own change distinct, as a changed hand-in's brief is.
	b.edits["code.go"] = "package fixture\n// next episode\n"
	if code, result := b.run("work", "build", b.id, "--work", unit, "--brief", b.brief("second.md", "Repair the next episode.\n"), "--lines", "3", "--check", "metasystem test impact"); code != 0 || strings.Contains(resultWords(result), "UNIT_NAMED_INPUT_CHANGED") {
		t.Fatalf("fresh allowance: %s", resultWords(result))
	}
	works, err := b.owners.work.units(stateroot.Layout{}).NamedWork(b.root(), b.id)
	if err != nil || len(works) != 2 {
		t.Fatalf("episode jobs %+v: %v", works, err)
	}
}

func TestSettingsLaneFixKeysAndMeaning(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	owners := b.workOwners()
	owners.work.config = configSettingWithDefault
	owners.work.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
	code, keys, stderr := b.intentBed.run(owners, "settings", "keys")
	if code != 0 || !strings.Contains(keys, "landing.fix-rounds") || !strings.Contains(keys, "landing.fix-minutes") {
		t.Fatalf("keys exit %d: %s %s", code, keys, stderr)
	}
	if code, text, stderr := b.intentBed.run(owners, "settings", "show"); code != 0 || !strings.Contains(text, "landing.fix-rounds") || !strings.Contains(text, "landing.fix-minutes") || !strings.Contains(text, "independent of the goal budget") {
		t.Fatalf("show exit %d: %s %s", code, text, stderr)
	}
	for key, want := range map[string]string{"landing.fix-rounds": "1", "landing.fix-minutes": "120"} {
		code, text, stderr := b.intentBed.run(owners, "settings", "show", key)
		if code != 0 || !strings.Contains(text, key+" is "+want) || !strings.Contains(strings.Join(strings.Fields(text), " "), "independent of the goal budget") {
			t.Fatalf("%s exit %d: %s %s", key, code, text, stderr)
		}
		code, text, stderr = b.intentBed.run(owners, "settings", "show", key, "--json")
		var result struct {
			Data struct {
				Settings []launch.Setting `json:"settings"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &result); err != nil {
			t.Fatalf("%s JSON: %v: %s", key, err, text)
		}
		values := result.Data.Settings
		if code != 0 || len(values) != 1 || values[0].Key != key || values[0].Value != want || values[0].Source != "default" {
			t.Fatalf("%s JSON exit %d: %s %s", key, code, text, stderr)
		}
	}
}

func TestLaneFixJobUsesConfiguredMinutes(t *testing.T) {
	t.Parallel()
	b := newLaneFixBed(t)
	conf := filepath.Join(b.root(), "metasystem.conf")
	raw, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(raw, []byte("\nlanding.fix-minutes=7\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if code, result := b.run("work", "build", b.id, "--work", b.fixUnit(), "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 0 {
		t.Fatalf("build: %s", result.Summary)
	}
	records, err := b.manager.Store.List()
	if err != nil || len(records) == 0 {
		t.Fatalf("jobs: %v", err)
	}
	for _, record := range records {
		var text string
		if err := json.Unmarshal(record.AdapterData["deadline"], &text); err != nil {
			t.Fatal(err)
		}
		deadline, err := time.Parse(time.RFC3339Nano, text)
		started, _ := time.Parse(time.RFC3339Nano, record.StartedAt)
		if err != nil || deadline.Sub(started) != 7*time.Minute {
			t.Fatalf("configured job box=%s: %v", text, err)
		}
	}
}
