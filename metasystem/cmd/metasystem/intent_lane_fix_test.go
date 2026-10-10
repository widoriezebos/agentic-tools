package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	c := &connectionBed{workBed: newWorkBed(t), t: t, edits: map[string]string{"code.go": "package fixture\n"}}
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
	if err := os.WriteFile(filepath.Join(hookDir, "pre-commit"), []byte(hook), 0755); err != nil {
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
	code, built := b.run("work", "build", b.id, "--work", "lane-fix-1", "--brief", brief, "--lines", "2", "--check", "metasystem test impact")
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
	command, rest, _ := resolveIntentArgv([]string{"work", "build", b.id, "--repo", b.root(), "--work", "lane-fix-1", "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"})
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
				if err := plain.WriteFix(b.root(), fix); err != nil {
					t.Fatal(err)
				}
				active, err := plain.ActiveFix(b.root())
				retained, readErr := plain.FixForAttempt(b.root(), "older")
				if err != nil || readErr != nil || active != nil || retained == nil || retained.State != "building" {
					t.Fatalf("stale fix %+v retained %+v: %v %v", active, retained, err, readErr)
				}
				return
			}
			if err := plain.WriteFix(b.root(), fix); err != nil {
				t.Fatal(err)
			}
			code, result := b.run("work", "build", b.id, "--work", "lane-fix-1", "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact")
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
	if code, result := b.run("work", "build", b.id, "--work", "lane-fix-1", "--brief", b.brief("fix.md", "Repair the fixture.\n"), "--lines", "2", "--check", "metasystem test impact"); code != 0 {
		t.Fatalf("build %d %s", code, result.Summary)
	}
	if code, result := b.run("work", "review", b.id, "--work", "lane-fix-1"); code != 0 {
		t.Fatalf("review %d %s", code, result.Summary)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "fixture-lane-read-owner", b.root(), b.id, connectionGit(t, b.root(), "rev-parse", "HEAD"))
	cmd.Dir = b.root()
	cmd.Env = append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
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
	cmd := exec.Command(executable, "fixture-lane-claim-owner", b.root(), b.id)
	cmd.Env = append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generic claim owner: %v %s", err, output)
	}
}
