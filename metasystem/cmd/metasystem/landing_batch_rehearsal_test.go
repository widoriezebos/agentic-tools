//go:build batchrehearsal

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type batchRehearsalCandidate struct {
	directory string
	files     []string
	groups    []string
}

type batchRehearsalVerb struct {
	argv           []string
	exit           int
	duration       time.Duration
	state, stdout  string
	stderr, status string
}

type batchRehearsalProof struct {
	attempt proofrun.Attempt
}

type batchRehearsalRun struct {
	t                        *testing.T
	origin, work             string
	landing, control         string
	seats                    map[string]string
	now                      time.Time
	startCommit, startTree   string
	unionTree                string
	chosen                   batchRehearsalCandidate
	branchTips               map[string]string
	expectedBlobs            map[string][]byte
	verbs                    []batchRehearsalVerb
	proofs                   []batchRehearsalProof
	refusals, landed, ledger []string
}

func TestBatchLandingRehearsalOnRealOrigin(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)

	// The seed clock must be the clock that actually governs this run, which is
	// the real one. METASYSTEM_GOAL_NOW is set below and is INERT here:
	// fixtureauth.ClockProbe.GoalNow (fixtureauth.go:242) returns early unless
	// the authorization carries fixtureMode, and FixtureModeRoot
	// (fixtureauth.go:292) reads metasystem.conf, never metasystem.conf.local,
	// while writeControlConf declares metasystem.runtimes=fake only in the
	// .local file. Nothing reports that, because the loud path in New()
	// triggers only when METASYSTEM_FAKE_PROCESS_IDENTITY_FILE is also set and
	// this rehearsal never sets it. A frozen seed therefore ages against the
	// real clock: on 2026-09-20 every goal was seeded claimed at
	// 2026-09-19T15:55:00Z under a 4h elapsed limit, so the landing root
	// breach-stopped all three seconds after handover and every proof was
	// refused CANDIDATE_GOAL_REFUSED state=fenced. Seeding from the real clock
	// keeps the whole run inside one elapsed budget whenever it is run.
	run := &batchRehearsalRun{t: t, seats: map[string]string{}, branchTips: map[string]string{}, expectedBlobs: map[string][]byte{}, now: time.Now().UTC().Truncate(time.Second)}
	run.requireInputs()
	t.Cleanup(run.report)
	run.isolateGitEnvironment()

	originalNow, originalLineage := os.Getenv("METASYSTEM_GOAL_NOW"), os.Getenv("METASYSTEM_OWNER_LINEAGE")
	originalCommandHelper := os.Getenv("GO_WANT_BATCH_E2E_COMMAND")
	originalStaticcheckCache, hadStaticcheckCache := os.LookupEnv("STATICCHECK_CACHE")
	if err := os.Setenv("METASYSTEM_GOAL_NOW", run.now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("METASYSTEM_OWNER_LINEAGE", "lineage-goal-rehearsal-a"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GO_WANT_BATCH_E2E_COMMAND", "1"); err != nil {
		t.Fatal(err)
	}
	staticcheckCache := filepath.Join(run.work, "cache", "staticcheck")
	if err := os.MkdirAll(staticcheckCache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("STATICCHECK_CACHE", staticcheckCache); err != nil {
		t.Fatal(err)
	}
	signal.Ignore(syscall.SIGUSR1)
	t.Cleanup(func() {
		_ = os.Setenv("METASYSTEM_GOAL_NOW", originalNow)
		_ = os.Setenv("METASYSTEM_OWNER_LINEAGE", originalLineage)
		_ = os.Setenv("GO_WANT_BATCH_E2E_COMMAND", originalCommandHelper)
		if hadStaticcheckCache {
			_ = os.Setenv("STATICCHECK_CACHE", originalStaticcheckCache)
		} else {
			_ = os.Unsetenv("STATICCHECK_CACHE")
		}
		signal.Reset(syscall.SIGUSR1)
	})

	run.landing = filepath.Join(run.work, "landing")
	run.clone(run.landing, "landing")
	run.control = batch.ModuleRoot(run.landing)
	run.writeSeed()
	run.startCommit = run.git(run.landing, "rev-parse", "HEAD")
	run.startTree = run.git(run.landing, "rev-parse", "HEAD^{tree}")

	engine := &batchE2EEngine{path: filepath.Join(run.control, "bin", "metasystem")}
	fixture := &batchE2EFixture{t: t, landing: run.control, now: run.now}
	fixture.enrollPolicyEngine(run.startCommit, engine)
	run.chosen = run.choosePackage()
	fmt.Printf("REHEARSAL CHOICE path=%s groups=%s\n", run.chosen.directory, strings.Join(run.chosen.groups, ","))

	for _, goalID := range []string{"goal-rehearsal-a", "goal-rehearsal-b", "goal-rehearsal-c"} {
		seat := filepath.Join(run.work, "seat-"+strings.TrimPrefix(goalID, "goal-rehearsal-"))
		run.clone(seat, goalID)
		run.seats[goalID] = batch.ModuleRoot(seat)
		run.announce(run.seats[goalID], "lineage-"+goalID)
		run.addGoalBranch(goalID)
		run.branchTips[goalID] = run.git(run.origin, "rev-parse", "refs/heads/goal/"+goalID)
	}
	run.holdLanding()

	batchID := ""
	for _, goalID := range []string{"goal-rehearsal-a", "goal-rehearsal-b", "goal-rehearsal-c"} {
		stdout := run.verb([]string{"join", "--root", run.seats[goalID], "--goal", goalID, "--last"}, batchID)
		var joined struct {
			BatchID string `json:"batchId"`
		}
		if err := json.Unmarshal([]byte(stdout), &joined); err != nil || joined.BatchID == "" {
			t.Fatalf("join %s did not print a batch id: output=%q error=%v", goalID, stdout, err)
		}
		if batchID == "" {
			batchID = joined.BatchID
		} else if joined.BatchID != batchID {
			t.Fatalf("join %s entered batch %s, want %s", goalID, joined.BatchID, batchID)
		}
	}
	run.unionTree = run.load(batchID).TipTree
	if err := os.Setenv("METASYSTEM_GOAL_NOW", run.now.Add(2*time.Minute).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	for tick := 1; tick <= 12; tick++ {
		run.verb([]string{"tick", "--root", run.seats["goal-rehearsal-a"], "--landing-root", run.landing, "--max-wait", "1m", "--batch", batchID}, batchID)
		record := run.load(batchID)
		run.captureProofs()
		if batchRehearsalTerminal(record.State) {
			break
		}
		if tick == 12 {
			t.Fatalf("batch %s did not reach a terminal state in 12 owner ticks; state=%s", batchID, record.State)
		}
	}
	run.assertOutcome(batchID)
}

func (run *batchRehearsalRun) requireInputs() {
	t := run.t
	origin, work := os.Getenv("METASYSTEM_REHEARSAL_ORIGIN"), os.Getenv("METASYSTEM_REHEARSAL_WORK")
	if origin == "" || work == "" {
		t.Fatalf("METASYSTEM_REHEARSAL_ORIGIN and METASYSTEM_REHEARSAL_WORK are both required")
	}
	var err error
	if run.origin, err = filepath.Abs(origin); err != nil {
		t.Fatalf("resolve METASYSTEM_REHEARSAL_ORIGIN: %v", err)
	}
	if run.work, err = filepath.Abs(work); err != nil {
		t.Fatalf("resolve METASYSTEM_REHEARSAL_WORK: %v", err)
	}
	if info, statErr := os.Stat(run.origin); statErr != nil || !info.IsDir() {
		t.Fatalf("METASYSTEM_REHEARSAL_ORIGIN must be a local directory: path=%s error=%v", run.origin, statErr)
	}
	if info, statErr := os.Stat(run.work); statErr != nil || !info.IsDir() {
		t.Fatalf("METASYSTEM_REHEARSAL_WORK must be an existing directory: path=%s error=%v", run.work, statErr)
	}
	if run.origin, err = filepath.EvalSymlinks(run.origin); err != nil {
		t.Fatalf("resolve METASYSTEM_REHEARSAL_ORIGIN symlinks: %v", err)
	}
	if run.work, err = filepath.EvalSymlinks(run.work); err != nil {
		t.Fatalf("resolve METASYSTEM_REHEARSAL_WORK symlinks: %v", err)
	}
	sourceRoot := run.git(".", "rev-parse", "--show-toplevel")
	if sourceRoot, err = filepath.EvalSymlinks(sourceRoot); err != nil {
		t.Fatalf("resolve source checkout symlinks: %v", err)
	}
	if directoryTreesOverlap(run.work, sourceRoot) {
		t.Fatalf("METASYSTEM_REHEARSAL_WORK must not overlap the source checkout: work=%s source=%s", run.work, sourceRoot)
	}
	if directoryTreesOverlap(run.work, run.origin) {
		t.Fatalf("METASYSTEM_REHEARSAL_WORK must not overlap METASYSTEM_REHEARSAL_ORIGIN: work=%s origin=%s", run.work, run.origin)
	}
	if directoryTreesOverlap(run.origin, sourceRoot) {
		t.Fatalf("METASYSTEM_REHEARSAL_ORIGIN must not overlap the source checkout: origin=%s source=%s", run.origin, sourceRoot)
	}
	if got := run.git(run.origin, "rev-parse", "--is-bare-repository"); got != "true" {
		t.Fatalf("METASYSTEM_REHEARSAL_ORIGIN must be bare, got %q", got)
	}
	command := exec.Command("git", "-C", run.origin, "remote")
	command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C")
	if output, remoteErr := command.CombinedOutput(); remoteErr != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("METASYSTEM_REHEARSAL_ORIGIN must have no remotes: output=%q error=%v", output, remoteErr)
	}
	run.git(run.origin, "rev-parse", "--verify", "refs/heads/main^{commit}")
	for _, name := range []string{"alternates", "http-alternates"} {
		path := filepath.Join(run.origin, "objects", "info", name)
		if data, readErr := os.ReadFile(path); readErr == nil && strings.TrimSpace(string(data)) != "" {
			t.Fatalf("METASYSTEM_REHEARSAL_ORIGIN must not borrow objects through %s", path)
		} else if readErr != nil && !os.IsNotExist(readErr) {
			t.Fatalf("inspect METASYSTEM_REHEARSAL_ORIGIN object alternates: %v", readErr)
		}
	}
	entries, err := os.ReadDir(run.work)
	if err != nil {
		t.Fatalf("METASYSTEM_REHEARSAL_WORK must be an existing empty directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("METASYSTEM_REHEARSAL_WORK must be empty, found %d entries", len(entries))
	}
}

func (run *batchRehearsalRun) isolateGitEnvironment() {
	realGit, err := exec.LookPath("git")
	if err != nil {
		run.t.Fatal(err)
	}
	if realGit, err = filepath.Abs(realGit); err != nil {
		run.t.Fatal(err)
	}
	originalPath := os.Getenv("PATH")
	original := map[string]string{}
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") {
			original[name] = value
			if err := os.Unsetenv(name); err != nil {
				run.t.Fatal(err)
			}
		}
	}
	globalConfig := filepath.Join(run.work, "git-global.conf")
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		run.t.Fatal(err)
	}
	if err := os.Setenv("GIT_CONFIG_GLOBAL", globalConfig); err != nil {
		run.t.Fatal(err)
	}
	if err := os.Setenv("GIT_CONFIG_NOSYSTEM", "1"); err != nil {
		run.t.Fatal(err)
	}
	shimDir := filepath.Join(run.work, "git-shim")
	if err := os.Mkdir(shimDir, 0o755); err != nil {
		run.t.Fatal(err)
	}
	shim := "#!/bin/sh\nexport GIT_CONFIG_NOSYSTEM=1\nexport GIT_CONFIG_SYSTEM=/dev/null\nexport GIT_CONFIG_GLOBAL=/dev/null\nexec " + shellQuote(realGit) + " \"$@\"\n"
	if err := testexec.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o755); err != nil {
		run.t.Fatal(err)
	}
	if err := os.Setenv("PATH", shimDir+string(os.PathListSeparator)+originalPath); err != nil {
		run.t.Fatal(err)
	}
	run.t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if strings.HasPrefix(name, "GIT_") {
				_ = os.Unsetenv(name)
			}
		}
		for name, value := range original {
			_ = os.Setenv(name, value)
		}
	})
}

func (run *batchRehearsalRun) clone(root, machine string) {
	run.git("", "clone", "-q", "--branch", "main", run.origin, root)
	batchE2EConfigureGit(run.t, root, machine)
	run.git(root, "config", "metasystem.goal.machine", machine)
	run.git(root, "config", "goal.sync-remote", "origin")
	run.git(root, "config", "goal.sync-branch", "refs/heads/main")
	run.git(root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	run.git(root, "update-ref", goal.AcceptedRef, "origin/main")
	if remotes := strings.Fields(run.git(root, "remote")); !slices.Equal(remotes, []string{"origin"}) {
		run.t.Fatalf("rehearsal clone %s has remotes %v, want only origin", root, remotes)
	}
	remoteURL := run.git(root, "remote", "get-url", "origin")
	remotePath, err := filepath.Abs(remoteURL)
	if err != nil || filepath.Clean(remotePath) != filepath.Clean(run.origin) {
		run.t.Fatalf("rehearsal clone %s origin=%q resolves to %q error=%v, want %s", root, remoteURL, remotePath, err, run.origin)
	}
	control := batch.ModuleRoot(root)
	if control == root {
		run.t.Fatalf("real rehearsal checkout %s does not contain the metasystem module", root)
	}
	local := strings.Join([]string{
		"landing.batch-root=" + run.landing,
		"landing.batch-max-wait=1m",
		"metasystem.runtimes=fake",
		"goal.human.wido=Wido Rehearsal <wido@example.invalid>",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(control, "metasystem.conf.local"), []byte(local), 0o644); err != nil {
		run.t.Fatal(err)
	}
}

func (run *batchRehearsalRun) writeSeed() {
	for index, goalID := range []string{"goal-rehearsal-a", "goal-rehearsal-b", "goal-rehearsal-c"} {
		intent := "Rehearse real batch landing for " + goalID + "."
		risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The rehearsal changes one isolated Go package."}
		budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 1, ReviewRoundLimit: 2}
		opened := run.now.Add(-10 * time.Minute).Format(time.RFC3339)
		claimed := run.now.Add(-5 * time.Minute).Format(time.RFC3339)
		approved := run.now.Add(-4 * time.Minute).Format(time.RFC3339)
		approvalOpid := batchE2EOpid(index+20, "human", "terminal")
		file := &goal.GoalFile{Id: goalID, State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain,
			NextStep: "Land the rehearsed unit.", OpenedAt: opened, Revision: 3, Budget: budget,
			Approved:       &goal.ApprovalRecord{By: "human:wido", At: approved, Revision: 3, Opid: approvalOpid, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
			Claimed:        &goal.ClaimRecord{Machine: goalID, Lineage: "lineage-" + goalID, At: claimed, Revision: 2, AccountingRevision: 2},
			StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: goalID, ClaimEpoch: 1},
			History: []goal.HistoryLine{{At: opened, Opid: batchE2EOpid(index+12, "human", "terminal"), Verb: "open", Actor: "human:wido", Keep: -1},
				{At: claimed, Opid: batchE2EOpid(index+16, goalID, "lineage-"+goalID), Verb: "claim", Actor: goalID + "+lineage-" + goalID, Keep: -1},
				{At: approved, Opid: approvalOpid, Verb: "approve", Actor: "human:wido", Keep: -1}}}
		path := filepath.Join(run.control, "plans", "goals", goalID+".md")
		if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
			run.t.Fatal(err)
		}
	}
	run.git(run.landing, "add", "metasystem/plans/goals/goal-rehearsal-a.md", "metasystem/plans/goals/goal-rehearsal-b.md", "metasystem/plans/goals/goal-rehearsal-c.md")
	run.gitAt(run.landing, "commit", "-qm", "seed batch rehearsal goals")
	run.git(run.landing, "push", "-q", "origin", "main")
	run.git(run.landing, "update-ref", goal.AcceptedRef, "HEAD")
}

func (run *batchRehearsalRun) choosePackage() batchRehearsalCandidate {
	contract, err := testpolicy.Load(filepath.Join(run.control, "testing.json"))
	if err != nil {
		run.t.Fatalf("load real testing contract: %v", err)
	}
	candidates := []string{"internal/launch", "internal/gaterun", "internal/channel"}
	var viable []batchRehearsalCandidate
	for _, directory := range candidates {
		matches, err := filepath.Glob(filepath.Join(run.control, directory, "*.go"))
		if err != nil {
			run.t.Fatal(err)
		}
		var files []string
		for _, match := range matches {
			if !strings.HasSuffix(match, "_test.go") {
				relative, _ := filepath.Rel(run.landing, match)
				files = append(files, filepath.ToSlash(relative))
			}
		}
		sort.Strings(files)
		if len(files) < 2 {
			continue
		}
		originals := make([][]byte, 2)
		for index, path := range files[:2] {
			original, readErr := os.ReadFile(filepath.Join(run.landing, filepath.FromSlash(path)))
			if readErr != nil {
				run.t.Fatal(readErr)
			}
			originals[index] = original
			probe := fmt.Sprintf("\n// batch rehearsal plan probe %d\n", index+1)
			if err := os.WriteFile(filepath.Join(run.landing, filepath.FromSlash(path)), append(original, []byte(probe)...), 0o644); err != nil {
				run.t.Fatal(err)
			}
		}
		run.git(run.landing, "add", "--", files[0], files[1])
		tree := run.git(run.landing, "write-tree")
		for index, path := range files[:2] {
			if err := os.WriteFile(filepath.Join(run.landing, filepath.FromSlash(path)), originals[index], 0o644); err != nil {
				run.t.Fatal(err)
			}
		}
		run.git(run.landing, "add", "--", files[0], files[1])
		plan, planErr := productionBatchTreePlan(run.landing, "goal-rehearsal-a", tree, testpolicy.ModeAuto)
		if planErr != nil {
			run.t.Fatalf("plan rehearsal candidate %s: %v", directory, planErr)
		}
		coversNewTests := rehearsalPlanRunsAllPackageTests(contract, plan.SelectedGroups, directory)
		fmt.Printf("REHEARSAL CANDIDATE path=%s groups=%s runs-new-tests=%t\n", directory, strings.Join(plan.SelectedGroups, ","), coversNewTests)
		if !coversNewTests {
			continue
		}
		viable = append(viable, batchRehearsalCandidate{directory: directory, files: files[:2], groups: slices.Clone(plan.SelectedGroups)})
	}
	if len(viable) == 0 {
		run.t.Fatal("no rehearsal candidate package has two non-test Go files")
	}
	sort.SliceStable(viable, func(i, j int) bool {
		if len(viable[i].groups) != len(viable[j].groups) {
			return len(viable[i].groups) < len(viable[j].groups)
		}
		return viable[i].directory < viable[j].directory
	})
	return viable[0]
}

func rehearsalPlanRunsAllPackageTests(contract testpolicy.Contract, selected []string, packagePath string) bool {
	for _, group := range contract.Groups {
		if !slices.Contains(selected, group.ID) || group.Adapter != "go" || !slices.Contains(group.Packages, packagePath) {
			continue
		}
		all, _, err := testpolicy.GoTests(group)
		if err == nil && all {
			return true
		}
	}
	return false
}

func (run *batchRehearsalRun) addGoalBranch(goalID string) {
	root := run.seats[goalID]
	repository := filepath.Dir(root)
	base := run.git(repository, "rev-parse", "origin/main")
	path := run.chosen.files[0]
	suffix := strings.TrimPrefix(goalID, "goal-rehearsal-")
	if suffix == "b" {
		path = run.chosen.files[1]
	}
	if suffix == "c" {
		path = filepath.ToSlash(filepath.Join("metasystem", run.chosen.directory, "landing_batch_rehearsal_failure_test.go"))
		packageName := run.packageName(filepath.Join(filepath.Dir(root), filepath.FromSlash(run.chosen.files[0])))
		body := fmt.Sprintf("package %s\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestBatchRehearsalIntentionalRed(t *testing.T) {\n\tt.Parallel()\n\tif os.Getenv(\"METASYSTEM_FIXTURE_ATTEMPT\") != \"\" {\n\t\tt.Fatal(\"intentional batch rehearsal red\")\n\t}\n}\n", packageName)
		if err := os.WriteFile(filepath.Join(filepath.Dir(root), filepath.FromSlash(path)), []byte(body), 0o644); err != nil {
			run.t.Fatal(err)
		}
	} else {
		absolute := filepath.Join(filepath.Dir(root), filepath.FromSlash(path))
		data, err := os.ReadFile(absolute)
		if err != nil {
			run.t.Fatal(err)
		}
		lines := strings.SplitAfter(string(data), "\n")
		changed := false
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "//go:") {
				continue
			}
			newline := ""
			if strings.HasSuffix(line, "\n") {
				line, newline = strings.TrimSuffix(line, "\n"), "\n"
			}
			lines[index] = line + " Batch rehearsal marker " + suffix + "." + newline
			changed = true
			break
		}
		if !changed {
			run.t.Fatalf("rehearsal source %s has no comment line to change", path)
		}
		content := []byte(strings.Join(lines, ""))
		if err := os.WriteFile(absolute, content, 0o644); err != nil {
			run.t.Fatal(err)
		}
		run.expectedBlobs[path] = slices.Clone(content)
	}
	run.git(repository, "add", "--", path)
	commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{Repo: repository, Remote: "origin", EndpointTip: base, GoalID: goalID, Unit: "u1", OpID: "build-" + goalID, Kind: goalbranch.Unit, CheckClaim: func() error { return nil }})
	if err != nil {
		run.t.Fatal(err)
	}
	subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: repository, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		run.t.Fatalf("compute %s read subject: present=%t error=%v", goalID, present, err)
	}
	job := "critic-" + goalID
	run.writeJSON(filepath.Join(repository, "artifacts", "agents", "jobs", job+".json"), map[string]any{"jobId": job, "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(), "closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}})
	run.writeJSON(filepath.Join(repository, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
	run.writeJSON(filepath.Join(repository, "artifacts", "agents", job, "rounds", "1", "return.json"), map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
	if _, _, err := goalbranch.CommitRead(goalbranch.CommitReadRequest{Repo: repository, Remote: "origin", EndpointTip: base, GoalID: goalID, Unit: "u1", OpID: "read-" + goalID, RootJob: job, GateRunID: "fast-" + goalID, GateTree: subject.Tree, CheckClaim: func() error { return nil }}); err != nil {
		run.t.Fatal(err)
	}
	run.git(repository, "push", "-q", "origin", "refs/heads/goal/"+goalID+":refs/heads/goal/"+goalID)
}

func (run *batchRehearsalRun) packageName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		run.t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "package" {
			return fields[1]
		}
	}
	run.t.Fatalf("no package declaration in %s", path)
	return ""
}

func (run *batchRehearsalRun) writeJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		run.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		run.t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		run.t.Fatal(err)
	}
}

func (run *batchRehearsalRun) announce(root, lineage string) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		run.t.Fatalf("probe rehearsal process: state=%s error=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "session-"+lineage, int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "batch-rehearsal", "metasystem", lineage); err != nil {
		run.t.Fatal(err)
	}
}

func (run *batchRehearsalRun) holdLanding() {
	run.announce(run.landing, landingOwnerLineage)
	holder, err := lease.RequireHolder(run.landing, int64(os.Getpid()), nil)
	if err != nil || !holder.Holder || holder.ClaimEpoch == nil {
		run.t.Fatalf("hold rehearsal landing checkout: holder=%+v error=%v", holder, err)
	}
}

func (run *batchRehearsalRun) verb(argv []string, batchID string) string {
	started := time.Now()
	code, stdout, stderr := captureCommandOutput(run.t, true, true, func() int { return runLandingBatch(argv) })
	entry := batchRehearsalVerb{argv: slices.Clone(argv), exit: code, duration: time.Since(started), stdout: stdout, stderr: stderr}
	observedBatchID := batchID
	if observedBatchID == "" && len(argv) != 0 && argv[0] == "join" {
		var joined struct {
			BatchID string `json:"batchId"`
		}
		if json.Unmarshal([]byte(stdout), &joined) == nil {
			observedBatchID = joined.BatchID
		}
	}
	statusCode := 0
	if observedBatchID != "" {
		if record, err := batch.NewStore(run.landing, nil).Load(observedBatchID); err == nil {
			entry.state = record.State
		}
		statusArgs := []string{"status", "--root", run.seats["goal-rehearsal-a"], "--batch", observedBatchID}
		var statusOut, statusErr string
		statusCode, statusOut, statusErr = captureCommandOutput(run.t, true, true, func() int { return runLandingBatch(statusArgs) })
		entry.status = strings.TrimSpace(statusOut + statusErr)
		if statusCode != 0 {
			entry.status = fmt.Sprintf("exit=%d %s", statusCode, entry.status)
		}
	}
	run.verbs = append(run.verbs, entry)
	if code != 0 {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = strings.TrimSpace(stdout)
		}
		run.refusals = append(run.refusals, fmt.Sprintf("argv=%q exit=%d: %s", argv, code, detail))
	}
	for _, text := range []string{stdout, stderr, entry.status} {
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "REFUSED") {
				run.refusals = append(run.refusals, strings.TrimSpace(line))
			}
		}
	}
	fmt.Printf("REHEARSAL VERB argv=%q exit=%d state=%s duration=%s status=%s\n", argv, code, entry.state, entry.duration.Round(time.Millisecond), entry.status)
	if code != 0 {
		run.t.Fatalf("rehearsal verb %q exited %d: stdout=%q stderr=%q", argv, code, stdout, stderr)
	}
	if statusCode != 0 {
		run.t.Fatalf("landing batch status after %q exited %d: %s", argv, statusCode, entry.status)
	}
	return stdout
}

func (run *batchRehearsalRun) captureProofs() {
	for _, root := range []string{run.control, run.landing} {
		attempts, err := proofrun.ReadAttempts(root)
		if err != nil {
			run.t.Fatalf("read real proof attempts from %s: %v", root, err)
		}
		for _, attempt := range attempts {
			index := slices.IndexFunc(run.proofs, func(existing batchRehearsalProof) bool {
				return existing.attempt.AttemptID == attempt.AttemptID
			})
			if index >= 0 {
				run.proofs[index].attempt = attempt
				continue
			}
			run.proofs = append(run.proofs, batchRehearsalProof{attempt: attempt})
		}
	}
}

func (run *batchRehearsalRun) load(batchID string) batch.Record {
	record, err := batch.NewStore(run.landing, nil).Load(batchID)
	if err != nil {
		run.t.Fatal(err)
	}
	return record
}

func batchRehearsalTerminal(state string) bool {
	return state == batch.StateLanded || state == batch.StateDissolved || state == batch.StateHeldTrunkRed || state == batch.StateHeldUnclassified
}

func (run *batchRehearsalRun) assertOutcome(batchID string) {
	t := run.t
	record := run.load(batchID)
	if record.State != batch.StateLanded {
		t.Fatalf("rehearsal batch terminal state=%s, want landed", record.State)
	}
	var incomplete []string
	var unionDeliveries []proofrun.TestResult
	var survivorGreen []proofrun.TestResult
	for _, proof := range run.proofs {
		attempt := proof.attempt
		if attempt.TestResult == nil {
			incomplete = append(incomplete, attempt.AttemptID+"@"+attempt.CandidateTree)
			continue
		}
		if attempt.TestResult.Purpose == testpolicy.PurposeDelivery && attempt.TestResult.CandidateTree == run.unionTree {
			unionDeliveries = append(unionDeliveries, *attempt.TestResult)
		}
		if attempt.TestResult.Purpose == testpolicy.PurposeDelivery &&
			attempt.TestResult.CandidateTree == record.TipTree && attempt.TestResult.Delivery.Sufficient {
			survivorGreen = append(survivorGreen, *attempt.TestResult)
		}
	}
	if len(incomplete) != 0 {
		t.Fatalf("real proof attempts remained incomplete at terminal state: %v", incomplete)
	}
	if len(unionDeliveries) != 1 || unionDeliveries[0].Delivery.Sufficient {
		t.Fatalf("union tip %s delivery proofs=%+v, want exactly one completed red delivery proof", run.unionTree, unionDeliveries)
	}
	if record.TipTree == run.unionTree || len(survivorGreen) != 1 || record.Proof == nil || record.Proof.AttemptID != survivorGreen[0].AttemptID {
		t.Fatalf("survivor tip %s green proofs=%+v record-proof=%+v, want one exact a+b re-proof", record.TipTree, survivorGreen, record.Proof)
	}
	unitC := batchE2EUnit(record, "goal-rehearsal-c")
	if unitC.Outcome != batch.UnitEjected || !strings.Contains(unitC.Failure, "TestBatchRehearsalIntentionalRed") {
		t.Fatalf("diagnosis did not name and eject goal-rehearsal-c: unit=%+v", unitC)
	}
	if got := run.git(run.origin, "rev-parse", "refs/heads/goal/goal-rehearsal-c"); got != run.branchTips["goal-rehearsal-c"] {
		t.Fatalf("ejected goal-rehearsal-c branch tip changed: got=%s want=%s", got, run.branchTips["goal-rehearsal-c"])
	}
	landedCommits := map[string]string{}
	for _, goalID := range []string{"goal-rehearsal-a", "goal-rehearsal-b"} {
		unit := batchE2EUnit(record, goalID)
		if unit.Outcome != batch.UnitLanded || unit.LandedCommit == "" {
			t.Fatalf("%s did not land with a recorded commit: unit=%+v", goalID, unit)
		}
		landedCommits[goalID] = unit.LandedCommit
		run.landed = append(run.landed, goalID+"="+unit.LandedCommit)
		command := exec.Command("git", "-C", run.origin, "show-ref", "--verify", "--quiet", "refs/heads/goal/"+goalID)
		command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C")
		if err := command.Run(); err == nil {
			t.Fatalf("landed goal branch %s remains on origin", goalID)
		}
	}
	log := run.git(run.origin, "log", "--reverse", "--first-parent", "--format=%H%x00%(trailers:key=Goal-Unit,valueonly)", run.startCommit+"..refs/heads/main")
	type landedHistoryEntry struct {
		commit, goalID string
	}
	var history []landedHistoryEntry
	for _, line := range strings.Split(log, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			t.Fatalf("origin main contains an unrecognized landing commit since rehearsal start: %q", line)
		}
		goalID, _, _ := strings.Cut(strings.TrimSpace(parts[1]), "/")
		history = append(history, landedHistoryEntry{commit: parts[0], goalID: goalID})
	}
	wantOrder := []string{"goal-rehearsal-a", "goal-rehearsal-b"}
	if len(history) != len(wantOrder) {
		t.Fatalf("origin main gained %d first-parent commits, want exactly two separate landing commits: %+v", len(history), history)
	}
	for index, goalID := range wantOrder {
		if history[index].goalID != goalID || history[index].commit != landedCommits[goalID] {
			t.Fatalf("origin main landing commit %d=%+v, want commit=%s goal=%s", index+1, history[index], landedCommits[goalID], goalID)
		}
	}
	if len(record.PrefixTrees) != 2 {
		t.Fatalf("survivor prefix trees=%v, want exact a then a+b prefixes", record.PrefixTrees)
	}
	expectedTrees := map[string]string{"goal-rehearsal-a": record.PrefixTrees[0], "goal-rehearsal-b": record.TipTree}
	pathA, pathB := run.chosen.files[0], run.chosen.files[1]
	ledger := map[string]bool{
		"metasystem/memory/receipts.log":             true,
		"metasystem/plans/goals/goal-rehearsal-a.md": true,
		"metasystem/plans/goals/goal-rehearsal-b.md": true,
		"metasystem/plans/goals/goal-rehearsal-c.md": true,
	}
	startA := run.gitBlob(run.origin, run.startCommit+":"+pathA)
	startB := run.gitBlob(run.origin, run.startCommit+":"+pathB)
	commitA, commitB := landedCommits["goal-rehearsal-a"], landedCommits["goal-rehearsal-b"]
	if !bytes.Equal(run.gitBlob(run.origin, commitA+":"+pathA), run.expectedBlobs[pathA]) ||
		!bytes.Equal(run.gitBlob(run.origin, commitA+":"+pathB), startB) {
		t.Fatalf("goal-rehearsal-a commit %s does not contain exactly A's product edit", commitA)
	}
	if !bytes.Equal(run.gitBlob(run.origin, commitB+":"+pathA), run.expectedBlobs[pathA]) ||
		!bytes.Equal(run.gitBlob(run.origin, commitB+":"+pathB), run.expectedBlobs[pathB]) || bytes.Equal(startA, run.expectedBlobs[pathA]) {
		t.Fatalf("goal-rehearsal-b commit %s does not add B after preserving A's product edit", commitB)
	}
	assertCommitPaths := func(base, commit, productPath, goalID string) {
		changed := strings.Fields(run.git(run.origin, "diff", "--name-only", base+".."+commit))
		if !slices.Contains(changed, productPath) {
			t.Fatalf("%s landing commit %s does not introduce product path %s: changed=%v", goalID, commit, productPath, changed)
		}
		for _, path := range changed {
			if path != productPath && !ledger[path] {
				t.Fatalf("%s landing commit %s transiently changes forbidden path %s: changed=%v", goalID, commit, path, changed)
			}
		}
	}
	assertCommitPaths(run.startCommit, commitA, pathA, "goal-rehearsal-a")
	assertCommitPaths(commitA, commitB, pathB, "goal-rehearsal-b")
	failingPath := filepath.ToSlash(filepath.Join("metasystem", run.chosen.directory, "landing_batch_rehearsal_failure_test.go"))
	for _, commit := range []string{commitA, commitB} {
		command := exec.Command("git", "-C", run.origin, "cat-file", "-e", commit+":"+failingPath)
		command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C")
		if err := command.Run(); err == nil {
			t.Fatalf("ejected failing test %s entered origin main history in commit %s", failingPath, commit)
		}
	}
	receipts := run.git(run.origin, "show", "refs/heads/main:metasystem/memory/receipts.log")
	containsReceipt := func(data, goalID, tree string) bool {
		for _, line := range strings.Split(data, "\n") {
			if strings.Contains(line, "|goal="+goalID+"|") && strings.Contains(line, "note=batch "+batchID+" unit ") && strings.Contains(line, " prefix "+tree) {
				return true
			}
		}
		return false
	}
	for goalID, tree := range expectedTrees {
		if !containsReceipt(receipts, goalID, tree) {
			t.Fatalf("origin receipt ledger does not bind %s to prefix tree %s", goalID, tree)
		}
		commit := landedCommits[goalID]
		commitReceipts := run.git(run.origin, "show", commit+":metasystem/memory/receipts.log")
		parentReceipts := run.git(run.origin, "show", commit+"^:metasystem/memory/receipts.log")
		if !containsReceipt(commitReceipts, goalID, tree) || containsReceipt(parentReceipts, goalID, tree) {
			t.Fatalf("%s receipt for prefix tree %s was not introduced by its recorded landing commit %s", goalID, tree, commit)
		}
	}
	prefixReceipt, ok := record.Receipts["goal-rehearsal-a"]
	if !ok || prefixReceipt.Tree != record.PrefixTrees[0] || prefixReceipt.AttemptID == "" {
		t.Fatalf("goal-rehearsal-a receipt=%+v, want exact first prefix proof", prefixReceipt)
	}
	if !slices.ContainsFunc(run.proofs, func(proof batchRehearsalProof) bool {
		return proof.attempt.AttemptID == prefixReceipt.AttemptID && proof.attempt.TestResult != nil && proof.attempt.TestResult.Delivery.Sufficient
	}) {
		t.Fatalf("goal-rehearsal-a prefix receipt attempt %s has no captured green proof", prefixReceipt.AttemptID)
	}
	changed := strings.Fields(run.git(run.origin, "diff", "--name-only", run.startCommit+"..refs/heads/main"))
	product := map[string]bool{run.chosen.files[0]: true, run.chosen.files[1]: true}
	for _, path := range changed {
		if product[path] {
			continue
		}
		if ledger[path] {
			run.ledger = append(run.ledger, path)
			continue
		}
		t.Fatalf("origin main changed unexpected non-product path %s; changed=%v", path, changed)
	}
	for path := range product {
		if !slices.Contains(changed, path) {
			t.Fatalf("origin main lacks rehearsed comment path %s; changed=%v", path, changed)
		}
		stat := run.git(run.origin, "diff", "--numstat", run.startCommit+"..refs/heads/main", "--", path)
		if !strings.HasPrefix(stat, "1\t1\t") {
			t.Fatalf("rehearsed path %s changed by more than one comment line: %q", path, stat)
		}
		content := run.gitBlob(run.origin, "refs/heads/main:"+path)
		if !bytes.Equal(content, run.expectedBlobs[path]) {
			t.Fatalf("landed blob %s does not byte-for-byte match the rehearsed comment edit: got %d bytes, want %d", path, len(content), len(run.expectedBlobs[path]))
		}
	}
	command := exec.Command("git", "-C", run.origin, "cat-file", "-e", "refs/heads/main:"+failingPath)
	command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C")
	if err := command.Run(); err == nil {
		t.Fatalf("ejected failing test reached origin main at %s", failingPath)
	}
}

func (run *batchRehearsalRun) report() {
	fmt.Println("REHEARSAL REPORT")
	for index, verb := range run.verbs {
		fmt.Printf("verb %02d: argv=%q exit=%d duration=%s state=%s\n", index+1, verb.argv, verb.exit, verb.duration.Round(time.Millisecond), verb.state)
	}
	for _, observed := range run.proofs {
		if observed.attempt.TestResult == nil {
			fmt.Printf("proof %s: status=incomplete candidate-tree=%s\n", observed.attempt.AttemptID, observed.attempt.CandidateTree)
			continue
		}
		proof := *observed.attempt.TestResult
		status := "red"
		if proof.Delivery.Sufficient {
			status = "green"
		}
		for _, group := range proof.Groups {
			fmt.Printf("proof %s: kind=%s status=%s group=%s group-status=%s duration=%s\n", proof.AttemptID, proof.Purpose, status, group.ID, group.Status, time.Duration(group.DurationMS)*time.Millisecond)
		}
	}
	for _, landed := range run.landed {
		fmt.Printf("landed: %s\n", landed)
	}
	for _, path := range run.ledger {
		fmt.Printf("ledger: %s\n", path)
	}
	if len(run.refusals) == 0 {
		fmt.Println("refusals: none")
	} else {
		for _, refusal := range run.refusals {
			fmt.Printf("refusal: %s\n", refusal)
		}
	}
}

func (run *batchRehearsalRun) git(root string, args ...string) string {
	commandArgs := slices.Clone(args)
	if root != "" {
		commandArgs = append([]string{"-C", root}, commandArgs...)
	}
	command := exec.Command("git", commandArgs...)
	command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C")
	output, err := command.CombinedOutput()
	if err != nil {
		run.t.Fatalf("git %s: %v: %s", strings.Join(commandArgs, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (run *batchRehearsalRun) gitBlob(root, object string) []byte {
	command := exec.Command("git", "-C", root, "show", object)
	command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C")
	output, err := command.CombinedOutput()
	if err != nil {
		run.t.Fatalf("git show %s: %v: %s", object, err, output)
	}
	return output
}

func (run *batchRehearsalRun) gitAt(root string, args ...string) string {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(gittree.ScrubbedEnviron(), "LC_ALL=C", "GIT_AUTHOR_DATE="+run.now.Format(time.RFC3339), "GIT_COMMITTER_DATE="+run.now.Format(time.RFC3339))
	output, err := command.CombinedOutput()
	if err != nil {
		run.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
