package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type stubGit struct {
	once         sync.Once
	stub         *testgit.Stub
	makeStub     func() *testgit.Stub
	normalize    func([]string) []string
	snapshot     []testgit.Expectation
	snapshotStub *testgit.Stub
	snapshotNext int
	reporter     testgit.Reporter
	worktree     string
}

func (git *stubGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	git.once.Do(func() { git.stub = git.makeStub() })
	if git.worktree != "" {
		actual, err := filepath.EvalSymlinks(directory)
		expected, expectedErr := filepath.EvalSymlinks(git.worktree)
		if err == nil && expectedErr == nil && actual == expected {
			directory = git.worktree
		}
		if slices.Equal(args, []string{"rev-parse", "--verify", "base^{tree}"}) {
			if directory != git.worktree || len(environment) != 0 {
				return nil, fmt.Errorf("invalid baseline lookup: %s %v", directory, environment)
			}
			return nil, os.ErrNotExist
		}
		if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
			if directory != git.worktree || len(environment) != 0 {
				return nil, fmt.Errorf("invalid tree custody query: %s %v", directory, environment)
			}
			return []byte(git.worktree + "\n"), nil
		}
	}
	if git.normalize != nil {
		args = git.normalize(args)
	}
	if len(git.snapshot) > 0 && (git.snapshotNext > 0 || slices.Equal(args, []string{"rev-parse", "HEAD"})) {
		if git.snapshotNext == 0 {
			expected := slices.Clone(git.snapshot[1:])
			check := isolatedGitEnvironment(expected[0].Call.Dir, strings.TrimSpace(string(expected[3].Result.Stdout)), true)
			for i := range expected {
				if expected[i].Check != nil {
					expected[i].Check = check
				}
			}
			git.snapshotStub = testgit.New(git.reporter, expected...)
		}
		result := git.snapshotStub.Run(testgit.Call{Dir: directory, Env: environment, Args: args})
		git.snapshotNext = (git.snapshotNext + 1) % (len(git.snapshot) - 1)
		return result.Stdout, result.Err
	}
	result := git.stub.Run(testgit.Call{Dir: directory, Env: environment, Args: args})
	return result.Stdout, result.Err
}

type recordingOSGit struct {
	runner OSGitRunner
	calls  [][]string
	envs   [][]string
}

func (git *recordingOSGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	git.calls = append(git.calls, append([]string(nil), args...))
	git.envs = append(git.envs, append([]string(nil), environment...))
	return git.runner.Run(directory, environment, args...)
}

type completingStarter struct {
	m                  *Manager
	failKind, holdKind string
	failCause          string
	order, ids         []string
	readOutput         string
	readVerdict        string
	readCounts         []bool
	readClass          string
	readWhere          string
	skipStructured     bool
	onStart            func(Record) error
	onBuild            func(Record) error
}

func (starter *completingStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	starter.ids = append(starter.ids, id)
	record, _ := starter.m.Store.Read(id)
	starter.order = append(starter.order, record.Kind)
	if record.Kind == "build" && starter.onBuild != nil {
		if err := starter.onBuild(record); err != nil {
			return identity.Ref{}, err
		}
	}
	if starter.onStart != nil {
		if err := starter.onStart(record); err != nil {
			return identity.Ref{}, err
		}
	}
	starter.m.Store.Update(id, func(current *Record) error {
		if record.Kind == starter.holdKind {
			supervisor, child := ref(10), ref(20)
			current.Supervisor, current.Child, current.State = &supervisor, &child, Running
			return nil
		}
		current.State = Completed
		code := 0
		if record.Kind == starter.failKind {
			current.State, current.Reason, code = Failed, "fixture-red", 1
			if starter.failCause != "" {
				current.Cause = starter.failCause
			}
		}
		current.ExitCode = &code
		if record.Kind == "read" {
			yes := true
			if len(starter.readCounts) > 0 {
				yes = starter.readCounts[0]
				starter.readCounts = starter.readCounts[1:]
			}
			material := max(0, 4-record.Round)
			verdict := starter.readVerdict
			if verdict == "" {
				verdict = fmt.Sprintf("fix first (%d material findings)", material)
			}
			if match := regexp.MustCompile(`([0-9]+) material findings?`).FindStringSubmatch(verdict); len(match) == 2 {
				material, _ = strconv.Atoi(match[1])
			} else if strings.EqualFold(verdict, "land") {
				material = 0
			}
			current.VerdictCounts, current.Measurement.Verdict = &yes, verdict
			if current.State == Completed && !starter.skipStructured {
				paths, err := declaredOutputPaths(*current)
				if err != nil {
					return err
				}
				for _, path := range paths {
					if filepath.Base(path) == "return.json" {
						if _, err := os.Stat(path); os.IsNotExist(err) {
							if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
								return err
							}
							if err := os.WriteFile(path, []byte(structuredUnitReturn(material, choose(starter.readClass, []string{"regression", "scope", "incomplete-item", "missing-reader"}[max(0, record.Round-1)%4]), choose(starter.readWhere, fmt.Sprintf("round-%d.go", record.Round)))), 0600); err != nil {
								return err
							}
						}
					} else if filepath.Base(path) == "report.md" {
						if err := os.WriteFile(path, []byte("Retained examination evidence\n"), 0600); err != nil {
							return err
						}
					}
				}
			}
			if starter.readOutput != "" {
				current.Outputs = []Output{{Path: starter.readOutput, Bytes: 4}}
			}
		}
		return nil
	})
	return ref(10), nil
}

func structuredUnitReturn(material int, class, where string) string {
	findings := []map[string]any{}
	for n := 0; n < material; n++ {
		findings = append(findings, map[string]any{"material": true, "class": class, "severity": "high", "claim": fmt.Sprintf("Finding %d", n+1), "evidence": "The result is discarded", "change": "Preserve the result", "where": where, "relation": "new"})
	}
	findings = append(findings, map[string]any{"material": false, "class": "regression", "severity": "low", "claim": "Non-gating observation", "evidence": "The result is retained", "change": "Keep the result observable", "where": "observation.go", "relation": "fold-not-holding"})
	data, _ := json.Marshal(map[string]any{"findings": findings, "verdictMaterialCount": material})
	return string(data)
}

func correctionBrief(record UnitRunRecord, text string) []byte {
	round := record.Rounds[len(record.Rounds)-1]
	brief := text + fmt.Sprintf("\n## Decisions on round %d\n", round.Number)
	for _, read := range round.Reads {
		for _, finding := range read.Findings {
			if finding.Material {
				brief += fmt.Sprintf("| %s | fixed | file.go:12 |\n", finding.ID)
			}
		}
	}
	return []byte(brief)
}

type unitFixture struct {
	runner   *UnitRunner
	manager  *Manager
	starter  *completingStarter
	git      *stubGit
	plan     string
	worktree string
}

func baseUnitFixture(t *testing.T) unitFixture {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "work")
	os.MkdirAll(worktree, 0o700)
	brief := filepath.Join(root, "build.md")
	read := filepath.Join(root, "read.md")
	page := filepath.Join(root, "units.md")
	os.WriteFile(brief, []byte("Declared size: 2 changed lines\n"), 0o600)
	os.WriteFile(read, []byte("read it\n"), 0o600)
	os.WriteFile(page, []byte("| Unit | Size |\n|---|---|\n| U | 2 |\n"), 0o600)
	plan := filepath.Join(root, "plan.json")
	data, _ := json.Marshal(UnitPlan{Unit: "U", Goal: "goal", Worktree: worktree, Base: "base",
		Build: UnitBuildPlan{Brief: brief, Inputs: []string{}, Outputs: []string{}, UnitsPage: page, Units: []string{"U"}},
		Read:  UnitReadPlan{Brief: read, Inputs: []string{}, Outputs: []string{}, Model: "read-model"},
		Proof: []ProofCommand{{Name: "check", Dir: worktree, Argv: []string{"true"}, Env: []string{"A=B"}}}})
	os.WriteFile(plan, data, 0o600)
	m, _, _, _ := manager(t)
	m.Adapters["claude-headless"], m.Adapters["plain-exec"] = fakeAdapter{}, fakeAdapter{}
	m.Settings = DefaultSettings()
	m.Settings.WaitCapSeconds = 2
	starter := &completingStarter{m: m}
	m.Supervisor = starter
	return unitFixture{runner: &UnitRunner{Manager: m, Root: filepath.Join(root, "unit")}, manager: m, starter: starter, plan: plan, worktree: worktree}
}

func newUnitFixture(t *testing.T, diff string, events ...string) unitFixture {
	t.Helper()
	if diff == "" {
		diff = "diff --git a/unit.go b/unit.go\n--- a/unit.go\n+++ b/unit.go\n+implemented\n"
	}
	fixture := baseUnitFixture(t)
	var total int64
	for _, block := range parseDiff([]byte(diff)) {
		total += block.lines
	}
	size := max(int64(2), total)
	root := filepath.Dir(fixture.worktree)
	os.WriteFile(filepath.Join(root, "build.md"), []byte(fmt.Sprintf("Declared size: %d changed lines\n", size)), 0600)
	os.WriteFile(filepath.Join(root, "units.md"), []byte(fmt.Sprintf("| Unit | Size |\n|---|---|\n| U | %d |\n", size)), 0600)
	index := filepath.Join(root, "source-index")
	os.WriteFile(index, []byte("index"), 0o600)
	objects := filepath.Join(root, "objects")
	os.MkdirAll(objects, 0o700)
	defaultEvents := events == nil
	if defaultEvents {
		events = []string{"branch", "branch", "round"}
	}
	var admission []string
	for i, event := range events {
		admission = append(admission, event)
		if event == "branch" && (i == 0 || events[i-1] == "new") && (i+1 == len(events) || events[i+1] != "branch") {
			admission = append(admission, "branch")
		}
	}
	events = admission
	var expanded []string
	rounds := 0
	beforePending := false
	for i, event := range events {
		if event == "before" {
			beforePending = true
		}
		if event == "round" {
			if !beforePending {
				expanded = append(expanded, "before")
			}
			beforePending = false
		}
		expanded = append(expanded, event)
		if event == "new" || event == "resolve" {
			rounds = 0
		}
		if event == "round" {
			rounds++
			if rounds > 1 && (i+1 == len(events) || events[i+1] != "warm") {
				expanded = append(expanded, "warm")
			}
		}
	}
	events = expanded
	var expected []testgit.Expectation
	var snapshot []testgit.Expectation
	add := func(dir string, output string, check func(testgit.Call) error, args ...string) {
		expected = append(expected, testgit.Expectation{Call: testgit.Call{Dir: dir, Args: args}, Result: testgit.Result{Stdout: []byte(output)}, Check: check})
	}
	// A stopped build writes its own diff and counts it against the round's
	// starting diff.
	failedBuild := func() {
		check := isolatedGitEnvironment(fixture.worktree, objects, false)
		add(fixture.worktree, index+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
		add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
		add(fixture.worktree, "", check, "add", "-A", "--sparse", "--", ".")
		add(fixture.worktree, diff, check, "diff", "--cached", "--binary", "--full-index", "base", "--", ".")
		verify := isolatedGitEnvironment(fixture.worktree, objects, false)
		fold := func(call testgit.Call) error {
			if call.Args[0] == "read-tree" {
				if err := os.WriteFile(strings.TrimPrefix(call.Env[0], "GIT_INDEX_FILE="), []byte("index"), 0600); err != nil {
					return err
				}
			}
			return verify(call)
		}
		add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
		add(fixture.worktree, "", fold, "read-tree", "base")
		add(fixture.worktree, "previous-tree\n", fold, "write-tree")
		add(fixture.worktree, "", fold, "read-tree", "base")
		if diff != "" {
			add(fixture.worktree, "", fold, "apply", "--cached", "--binary", "retained-worktree.diff")
		}
		add(fixture.worktree, diff, fold, "diff", "--cached", "--binary", "previous-tree", "--", ".", ":(exclude,literal)records", ":(exclude,literal)metasystem/records")
	}
	for _, event := range events {
		switch event {
		case "new", "resolve":
		case "failed":
			failedBuild()
		case "branch":
			add(fixture.worktree, "goal/goal\n", nil, "symbolic-ref", "--short", "HEAD")
		case "main":
			add(fixture.worktree, "main\n", nil, "symbolic-ref", "--short", "HEAD")
		case "detached":
			expected = append(expected, testgit.Expectation{Call: testgit.Call{Dir: fixture.worktree, Args: []string{"symbolic-ref", "--short", "HEAD"}}, Result: testgit.Result{Err: errors.New("detached HEAD")}})
			add(fixture.worktree, "head\n", nil, "rev-parse", "--verify", "HEAD")
		case "before":
			beforeCheck := isolatedGitEnvironment(fixture.worktree, objects, false)
			add(fixture.worktree, index+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
			add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
			add(fixture.worktree, "", beforeCheck, "add", "-A", "--sparse", "--", ".")
			add(fixture.worktree, "", beforeCheck, "diff", "--cached", "--binary", "--full-index", "base", "--", ".")
		case "round":
			check := isolatedGitEnvironment(fixture.worktree, objects, false)
			add(fixture.worktree, index+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
			add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
			add(fixture.worktree, "", check, "add", "-A", "--sparse", "--", ".")
			add(fixture.worktree, diff, check, "diff", "--cached", "--binary", "--full-index", "base", "--", ".")
			foldVerify := isolatedGitEnvironment(fixture.worktree, objects, false)
			foldCheck := func(call testgit.Call) error {
				if call.Args[0] == "read-tree" {
					if err := os.WriteFile(strings.TrimPrefix(call.Env[0], "GIT_INDEX_FILE="), []byte("index"), 0600); err != nil {
						return err
					}
				}
				return foldVerify(call)
			}
			add(fixture.worktree, index+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
			add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
			patchCheck := isolatedGitEnvironment(fixture.worktree, objects, false)
			add(fixture.worktree, "", patchCheck, "add", "-A", "--sparse", "--", ".")
			add(fixture.worktree, diff, patchCheck, "diff", "--cached", "--binary", "--full-index", "head", "--", ".")
			add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
			add(fixture.worktree, "", foldCheck, "read-tree", "base")
			add(fixture.worktree, "previous-tree\n", foldCheck, "write-tree")
			add(fixture.worktree, "", foldCheck, "read-tree", "base")
			if diff != "" {
				add(fixture.worktree, "", foldCheck, "apply", "--cached", "--binary", "retained-worktree.diff")
			}
			add(fixture.worktree, diff, foldCheck, "diff", "--cached", "--binary", "previous-tree", "--", ".", ":(exclude,literal)records", ":(exclude,literal)metasystem/records")

			{
				snapshotStart := len(expected)
				check := isolatedGitEnvironment(fixture.worktree, objects, true)
				add(fixture.worktree, fixture.worktree+"\n", nil, "rev-parse", "--show-toplevel")
				add(fixture.worktree, "head\n", nil, "rev-parse", "HEAD")
				add(fixture.worktree, "refs/heads/goal/goal head\n", nil, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/tags", "refs/notes", "refs/stash")
				add(fixture.worktree, index+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
				add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
				add(fixture.worktree, "", check, "ls-files", "-v", "--stage", "-z", "--full-name", "--", ".")
				add(fixture.worktree, "", check, "ls-files", "-t", "--stage", "-z", "--full-name", "--", ".")
				add(fixture.worktree, "", check, "diff-index", "--cached", "--raw", "-z", "--ita-invisible-in-index", "HEAD", "--", ".")
				add(fixture.worktree, "", check, "ls-files", "--resolve-undo", "-z", "--full-name", "--", ".")
				add(fixture.worktree, "", check, "add", "-A", "--sparse", "--", ".")
				add(fixture.worktree, "", check, "diff", "--cached", "--raw", "-z", "--no-abbrev", "HEAD", "--", ".")
				add(fixture.worktree, "round-tree\n", check, "write-tree")
				snapshot = append([]testgit.Expectation(nil), expected[snapshotStart:]...)
				expected = expected[:snapshotStart]
			}
		case "warm":
			verify := isolatedGitEnvironment(fixture.worktree, objects, false)
			check := func(call testgit.Call) error {
				if call.Args[0] == "read-tree" {
					if err := os.WriteFile(strings.TrimPrefix(call.Env[0], "GIT_INDEX_FILE="), []byte("index"), 0o600); err != nil {
						return err
					}
				}
				return verify(call)
			}
			add(fixture.worktree, objects+"\n", nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
			add(fixture.worktree, "", check, "read-tree", "base")
			if diff != "" {
				add(fixture.worktree, "", check, "apply", "--cached", "--binary", "retained-worktree.diff")
			}
			add(fixture.worktree, "previous-tree\n", check, "write-tree")
			add(fixture.worktree, "", check, "read-tree", "base")
			if diff != "" {
				add(fixture.worktree, "", check, "apply", "--cached", "--binary", "retained-worktree.diff")
			}
			add(fixture.worktree, "+fixed line\n", check, "diff", "--cached", "--binary", "previous-tree", "--", ".")
		default:
			t.Fatalf("unknown Git fixture event %q", event)
		}
	}
	// Each physical proof/read execution takes fresh snapshots, including a
	// retained step retry. Every snapshot must complete this strict Git sequence.
	start := len(expected)
	failedBuild()
	failed := slices.Clone(expected[start:])
	expected = expected[:start]
	fixture.git = &stubGit{worktree: fixture.worktree, snapshot: snapshot, reporter: t, makeStub: func() *testgit.Stub {
		if defaultEvents && fixture.starter.failKind == "build" {
			return testgit.New(t, append(slices.Clone(expected[:6]), failed...)...)
		}
		return testgit.New(t, expected...)
	}}
	fixture.git.normalize = func(args []string) []string {
		if len(args) == 4 && args[0] == "apply" && args[1] == "--cached" && args[2] == "--binary" {
			data, err := os.ReadFile(args[3])
			if err != nil || string(data) != diff {
				t.Fatalf("warm read applied changed or missing retained diff: %s %v", data, err)
			}
			args = append([]string(nil), args...)
			args[3] = "retained-worktree.diff"
		}
		return args
	}
	fixture.runner.Git = fixture.git
	return fixture
}

func isolatedGitEnvironment(worktree, alternate string, optionalLocks bool) func(testgit.Call) error {
	var owner string
	return func(call testgit.Call) error {
		if len(call.Stdin) != 0 {
			return fmt.Errorf("Git stdin must be empty: %q", call.Stdin)
		}
		count := 3
		if optionalLocks {
			count++
		}
		if len(call.Env) != count {
			return fmt.Errorf("Git environment has %d entries, want %d: %q", len(call.Env), count, call.Env)
		}
		index := strings.TrimPrefix(call.Env[0], "GIT_INDEX_FILE=")
		objects := strings.TrimPrefix(call.Env[1], "GIT_OBJECT_DIRECTORY=")
		if index == call.Env[0] || objects == call.Env[1] || !filepath.IsAbs(index) || !filepath.IsAbs(objects) || filepath.Base(index) != "index" || filepath.Base(objects) != "objects" {
			return fmt.Errorf("invalid isolated Git paths: %q", call.Env)
		}
		currentOwner := filepath.Dir(index)
		if filepath.Dir(objects) != currentOwner || currentOwner == worktree || strings.HasPrefix(currentOwner, worktree+string(os.PathSeparator)) || !strings.HasPrefix(filepath.Base(currentOwner), "metasystem-unit-") {
			return fmt.Errorf("index and objects must share a private temporary owner: %q", call.Env)
		}
		if owner == "" {
			owner = currentOwner
		} else if owner != currentOwner {
			return fmt.Errorf("isolated Git owner changed: %q to %q", owner, currentOwner)
		}
		if info, err := os.Stat(index); err != nil || info.IsDir() {
			return fmt.Errorf("private index missing: %q: %v", index, err)
		}
		if info, err := os.Stat(objects); err != nil || !info.IsDir() {
			return fmt.Errorf("private objects missing: %q: %v", objects, err)
		}
		if call.Env[2] != "GIT_ALTERNATE_OBJECT_DIRECTORIES="+alternate {
			return fmt.Errorf("alternate objects mismatch: %q", call.Env[2])
		}
		if optionalLocks && call.Env[3] != "GIT_OPTIONAL_LOCKS=0" {
			return fmt.Errorf("optional locks mismatch: %q", call.Env[3])
		}
		return nil
	}
}

func TestUnitPlanRetainsDeclaredCheckAndEstimate(t *testing.T) {
	t.Parallel()
	fixture := baseUnitFixture(t)
	plan, err := ReadUnitPlan(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Check = &UnitCheck{SourceTree: "declaration-tree", Cheap: "true", Audits: "true", Minutes: 15,
		Directory: fixture.worktree, Environment: []string{"A=B"}}
	checkMinutes := 2.0
	plan.Estimate = &UnitEstimate{DesignID: "design", SourceSHA256: strings.Repeat("a", 64), BodySHA256: strings.Repeat("b", 64),
		Unit: plan.Unit, ElapsedMinutes: 10, CheckMinutes: &checkMinutes}
	plan.FullArgv = []string{"go", "run", "./cmd/devgate", "full"}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	retained, err := ReadUnitPlan(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retained.Check, plan.Check) || !reflect.DeepEqual(retained.Estimate, plan.Estimate) || !slices.Equal(retained.FullArgv, plan.FullArgv) {
		t.Fatalf("retained plan lost declared checks or telemetry: %+v", retained)
	}
	for _, field := range []string{"estimate", "fullArgv"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			input := retained
			if field == "estimate" {
				input.FullArgv = nil
			} else {
				input.Estimate = nil
			}
			path := filepath.Join(t.TempDir(), "plan.json")
			data, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = ReadUnitPlanInput(path)
			var problem *CodedError
			if !errors.As(err, &problem) || problem.Code != "UNIT_PLAN_INVALID" || problem.Facts != "field="+field {
				t.Fatalf("caller supplied retained %s: %v", field, err)
			}
		})
	}
	plan.Estimate, plan.FullArgv = nil, nil
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := ReadUnitPlanInput(fixture.plan)
	if err != nil || !reflect.DeepEqual(input.Check, plan.Check) {
		t.Fatalf("caller plan lost its declared check: %+v %v", input.Check, err)
	}
}

func TestUnitRunStartsOnGoalBranch(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || result.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPlanWithoutReadIsAdmitted(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	body, _ := os.ReadFile(fixture.plan)
	for _, form := range []string{"absent", "null", "missing brief"} {
		var raw map[string]any
		json.Unmarshal(body, &raw)
		switch form {
		case "absent":
			delete(raw, "read")
		case "null":
			raw["read"] = nil
		default:
			delete(raw["read"].(map[string]any), "brief")
		}
		changed, _ := json.Marshal(raw)
		os.WriteFile(fixture.plan, changed, 0o600)
		plan, err := ReadUnitPlan(fixture.plan)
		if form == "missing brief" {
			if err == nil || !strings.Contains(ErrorDetail(err), "field=read.brief") {
				t.Fatalf("%s: %v", form, err)
			}
		} else if err != nil || plan.HasRead() || plan.Read.Brief != "" {
			t.Fatalf("%s: plan=%+v err=%v", form, plan, err)
		}
	}
}

func TestRoundWithoutReadEndsGreen(t *testing.T) {
	t.Parallel()
	for _, fail := range []string{"", "build", "proof"} {
		fixture := newUnitFixture(t, "")
		fixture.starter.failKind = fail
		plan, _ := ReadUnitPlan(fixture.plan)
		plan.Read = UnitReadPlan{}
		body, _ := json.Marshal(plan)
		os.WriteFile(fixture.plan, body, 0o600)
		result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
		if err != nil {
			t.Fatal(err)
		}
		round := result.Record.Rounds[0]
		want := map[string]string{"": "green", "build": "build-failed", "proof": "proof-red"}[fail]
		launches := []string{"build", "proof"}
		if fail == "build" {
			launches = launches[:1]
		}
		if round.Outcome != want || round.ReadModel != "" || !slices.Equal(fixture.starter.order, launches) || len(round.Steps) != 2 {
			t.Fatalf("%s: round=%+v launches=%v", fail, round, fixture.starter.order)
		}
	}
}

func TestUnitRunRefusesMainBranch(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "main")
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	want := "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED worktree=\"" + fixture.worktree + "\" branch=\"main\" required=\"goal/goal\""
	if err == nil || !strings.HasPrefix(ErrorDetail(err), want+": ") {
		t.Fatalf("error=%v want=%q", ErrorDetail(err), want)
	}
	wantRow := refusal.Row{Code: "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED", Owner: "internal/launch", Site: "unit_run.go#UnitRunner.requireGoalBranch", Shape: refusal.Question, H1: refusal.StandingInput}
	for _, row := range refusal.Rows {
		if row.Code == wantRow.Code {
			if row != wantRow {
				t.Fatalf("refusal row=%+v want=%+v", row, wantRow)
			}
			return
		}
	}
	t.Fatalf("missing refusal row %+v", wantRow)
}

func TestUnitRunRefusesDetachedHead(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "detached")
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err == nil || !strings.Contains(ErrorDetail(err), "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED") || !strings.Contains(ErrorDetail(err), `branch="detached HEAD"`) || !strings.Contains(ErrorDetail(err), `required="goal/goal"`) {
		t.Fatalf("error=%v", err)
	}
}

func TestPlanRefusesWhatItCannotRun(t *testing.T) {
	fixture := newUnitFixture(t, "", []string{}...)
	data, _ := os.ReadFile(fixture.plan)
	for name, mutation := range map[string]func(map[string]any){
		"unknown":        func(value map[string]any) { value["surprise"] = true },
		"missing":        func(value map[string]any) { delete(value, "goal") },
		"repeated proof": func(value map[string]any) { proof := value["proof"].([]any); value["proof"] = append(proof, proof[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			json.Unmarshal(data, &value)
			mutation(value)
			path := filepath.Join(t.TempDir(), "plan.json")
			changed, _ := json.Marshal(value)
			os.WriteFile(path, changed, 0o600)
			_, err := fixture.runner.Advance(UnitRequest{Plan: path})
			if err == nil || !strings.Contains(ErrorDetail(err), "UNIT_PLAN_INVALID") {
				t.Fatalf("err=%v", err)
			}
			entries, _ := os.ReadDir(fixture.runner.Root)
			if len(entries) != 0 {
				t.Fatalf("created run: %v", entries)
			}
		})
	}
}

func TestRunRecordNamesEveryStep(t *testing.T) {
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, step := range result.Record.Rounds[0].Steps {
		names = append(names, step.Name)
	}
	if !slices.Equal(names, []string{"build", "proof:check", "read"}) {
		t.Fatalf("steps=%v", names)
	}
	if _, err := os.Stat(filepath.Join(fixture.runner.runDir(result.Record.ID), "run.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunStopsAtTheCapAndResumeContinues(t *testing.T) {
	fixture := newUnitFixture(t, "", "branch", "before", "branch", "round")
	fixture.starter.holdKind = "build"
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !result.Capped || result.Step != "build" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	fixture.manager.Store.Update(result.Launch, func(record *Record) error { record.State = Completed; return nil })
	fixture.starter.holdKind = ""
	result, err = fixture.runner.Advance(UnitRequest{Resume: result.Record.ID})
	if err != nil || result.Record.State != "awaiting-judgement" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestProofRunsBeforeTheRead(t *testing.T) {
	fixture := newUnitFixture(t, "")
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !slices.Equal(fixture.starter.order, []string{"build", "proof", "read"}) {
		t.Fatalf("order=%v err=%v", fixture.starter.order, err)
	}
}

func TestEveryOutcomeEndsAtAwaitingJudgement(t *testing.T) {
	for _, row := range []struct{ fail, want string }{{"build", "build-failed"}, {"read", "read-failed"}, {"proof", "proof-red"}, {"", "green"}} {
		t.Run(row.want, func(t *testing.T) {
			events := []string{"branch", "round"}
			if row.want == "build-failed" {
				events = []string{"branch", "before", "failed"}
			}
			fixture := newUnitFixture(t, "", events...)
			fixture.starter.failKind = row.fail
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil || result.Record.State != "awaiting-judgement" || result.Record.Rounds[0].Outcome != row.want {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if row.want == "proof-red" {
				for _, step := range result.Record.Rounds[0].Steps {
					if strings.HasPrefix(step.Name, "read") && (step.State != StepSkipped || step.Reason != "proof-red") {
						t.Fatalf("read step=%+v", step)
					}
				}
				if slices.Contains(fixture.starter.order, "read") {
					t.Fatalf("order=%v", fixture.starter.order)
				}
			}
			if row.want == "read-failed" && !slices.Contains(fixture.starter.order, "proof") {
				t.Fatalf("order=%v", fixture.starter.order)
			}
		})
	}
}

func TestRoundCauseOnlyWhenNothingWasJudged(t *testing.T) {
	t.Parallel()
	t.Run("uncopied-findings", func(t *testing.T) {
		t.Parallel()
		m, _, _, _ := manager(t)
		read := Record{State: Failed, AdapterData: map[string]json.RawMessage{}}
		setStrings(read.AdapterData, "declaredOutputs", []string{brief(t)})
		round := UnitRound{Steps: []UnitStep{{Name: "read"}}}
		stepDriver{manager: m, round: &round}.endStep(0, read)
		require(t, round.Steps[0].Cause != "", "existing findings were classified as missing: %+v", round.Steps[0])
	})
	for _, row := range []struct {
		name, fail, hold, output, cause, stepCause string
		counts                                     bool
	}{
		// Decision 3 assigns lost processes to environment and unattributed exits
		// to unclassified; every failed step uses that shared cause vocabulary.
		{"lost-build", "build", "build", "", "environment", "environment", false},
		{"red-proof", "proof", "proof", "", "environment", "environment", false},
		{"no-findings", "read", "", "", "environment", "environment", false},
		{"findings", "read", "", "report", "environment", "unclassified", false},
		{"counted-read", "read", "", "", "environment", "environment", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture := newUnitFixture(t, "")
			fixture.starter.failKind, fixture.starter.holdKind = row.fail, row.hold
			fixture.starter.readCounts = []bool{row.counts}
			if row.output != "" {
				fixture.starter.readOutput = brief(t)
			}
			if row.hold != "" {
				probe := fixture.manager.Prober.(*fakeProber)
				probe.states[10], probe.states[20] = identity.Dead, identity.Dead
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			require(t, err != nil, "advance: %v", err)
			round := result.Record.Rounds[0]
			require(t, round.Cause != row.cause || result.Record.State != "awaiting-judgement", "round=%+v", round)
			for _, step := range round.Steps {
				if step.State == StepFailed {
					require(t, step.Cause != row.stepCause, "step=%+v", step)
				}
			}
			stored, err := fixture.runner.Status(result.Record.ID)
			require(t, err != nil || stored.Rounds[0].Cause != row.cause, "stored=%+v err=%v", stored, err)
		})
	}
}

func TestRefusedBuildLeavesNoRunRecord(t *testing.T) {
	t.Parallel()
	fixture := baseUnitFixture(t)
	fixture.runner.Git = &stubGit{makeStub: func() *testgit.Stub {
		return testgit.New(t, testgit.Expectation{
			Call:   testgit.Call{Dir: fixture.worktree, Args: []string{"symbolic-ref", "--short", "HEAD"}},
			Result: testgit.Result{Stdout: []byte("goal/goal\n")},
		})
	}}
	fixture.manager.Settings.BuildLinesCap = 1
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	entries, _ := os.ReadDir(fixture.runner.Root)
	if err == nil || !strings.Contains(ErrorDetail(err), "LAUNCH_BUILD_OVERSIZE") || len(entries) != 0 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

func TestResumeAfterAKillStartsNoSecondLaunch(t *testing.T) {
	fixture := newUnitFixture(t, "", "branch", "before", "branch", "round")
	fixture.runner.AfterWrite = func(record UnitRunRecord) error {
		for _, step := range record.Rounds[0].Steps {
			if step.State == StepStarting {
				return errors.New("killed")
			}
		}
		return nil
	}
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err == nil {
		t.Fatal("expected interruption")
	}
	entries := unitRunDirectories(t, fixture.runner.Root)
	if len(entries) != 1 {
		t.Fatalf("run directories=%v", entries)
	}
	id := entries[0]
	fixture.runner.AfterWrite = nil
	if _, err := fixture.runner.Advance(UnitRequest{Resume: id}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, id := range fixture.starter.ids {
		if seen[id] {
			t.Fatalf("launch started twice: %s", id)
		}
		seen[id] = true
	}
}

func TestSecondCallerRefusesBusy(t *testing.T) {
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fixture.runner.lock(result.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	_, err = fixture.runner.Advance(UnitRequest{Resume: result.Record.ID})
	if err == nil || !strings.Contains(ErrorDetail(err), "UNIT_RUN_BUSY") {
		t.Fatalf("err=%v", err)
	}
}

func TestDiffCoversUntrackedFilesAndLeavesTheIndexAlone(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(repo, "tracked"), []byte("old\n"), 0o600)
	runGit(t, repo, "add", "tracked")
	runGit(t, repo, "commit", "-m", "base")
	index := runGit(t, repo, "rev-parse", "--git-path", "index")
	before, _ := os.ReadFile(strings.TrimSpace(index))
	os.WriteFile(filepath.Join(repo, "tracked"), []byte("new\n"), 0o600)
	os.WriteFile(filepath.Join(repo, "untracked"), []byte("fresh\n"), 0o600)
	runner := UnitRunner{Git: OSGitRunner{}, Root: filepath.Join(t.TempDir(), "unit")}
	os.MkdirAll(filepath.Join(runner.Root, "r", "round-1"), 0o700)
	target := filepath.Join(runner.Root, "r", "round-1", "worktree.diff")
	if err := runner.writeDiff(repo, "HEAD", target); err != nil {
		t.Fatal(err)
	}
	diff, _ := os.ReadFile(target)
	after, _ := os.ReadFile(strings.TrimSpace(index))
	if !strings.Contains(string(diff), "tracked") || !strings.Contains(string(diff), "untracked") || string(before) != string(after) {
		t.Fatalf("diff=%s index changed=%t", diff, string(before) != string(after))
	}
}

func TestLargeDiffIsReadPerDirectory(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a/x.go b/a/x.go\n--- a/a/x.go\n+++ b/a/x.go\n+x\ndiff --git a/b/y.go b/b/y.go\n--- a/b/y.go\n+++ b/b/y.go\n+y\n"
	fixture := newUnitFixture(t, diff)
	fixture.manager.Settings.ReadSplitLines = 1
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	var reads []string
	for _, step := range result.Record.Rounds[0].Steps {
		if strings.HasPrefix(step.Name, "read:") {
			reads = append(reads, step.Name)
		}
	}
	if !slices.Equal(reads, []string{"read:a", "read:b"}) {
		t.Fatalf("reads=%v", reads)
	}
	fullDiff := filepath.Join(result.Record.Rounds[0].Directory, "worktree.diff")
	data, err := os.ReadFile(fullDiff)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256(data))
	for _, step := range result.Record.Rounds[0].Steps {
		if !strings.HasPrefix(step.Name, "read:") {
			continue
		}
		launched, err := fixture.manager.Store.Read(step.LaunchID)
		if err != nil {
			t.Fatal(err)
		}
		other := map[string]string{"a": "b", "b": "a"}[step.Package]
		packet := readString(launched.AdapterData, "unitReadPacket")
		command, commandErr := (ClaudeHeadless{}).Command(launched, filepath.Join(fixture.manager.Store.Root, launched.ID))
		for _, exact := range []string{
			"Full candidate diff: " + fullDiff,
			"Full candidate SHA-256: " + wantHash,
			"Selected package: " + step.Package,
			"Other package coverage: " + other + " (separate unit read steps)",
			"Diff: " + filepath.Join(fixture.manager.Store.Root, launched.ID, "read.diff"),
		} {
			if commandErr != nil || !strings.Contains(command.Stdin, exact) {
				t.Fatalf("step=%+v packet=%q stdin=%q missing=%q err=%v", step, packet, command.Stdin, exact, commandErr)
			}
		}
	}
}

func TestSplitReadsSharingAReportWaitForPriorCollection(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a/x.go b/a/x.go\n--- a/a/x.go\n+++ b/a/x.go\n+x\ndiff --git a/b/y.go b/b/y.go\n--- a/b/y.go\n+++ b/b/y.go\n+y\n"
	fixture := newUnitFixture(t, diff)
	fixture.manager.Settings.ReadSplitLines = 1
	plan, err := ReadUnitPlan(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Read.Outputs = []string{filepath.Join(t.TempDir(), "shared-read.md")}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	firstCollected := false
	fixture.runner.AfterWrite = func(record UnitRunRecord) error {
		if len(record.Rounds) > 0 {
			for _, step := range record.Rounds[0].Steps {
				if step.Name == "read:a" && step.State == StepPassed {
					firstCollected = true
				}
			}
		}
		return nil
	}
	readStarts := 0
	fixture.starter.onStart = func(record Record) error {
		if record.Kind == "read" {
			readStarts++
			if readStarts == 2 && !firstCollected {
				t.Error("second writer started before the first report was collected")
			}
		}
		return nil
	}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || result.Record.Rounds[0].Outcome != "green" || readStarts != 2 {
		t.Fatalf("outcome=%+v read starts=%d err=%v", result.Record.Rounds, readStarts, err)
	}
}

func TestResumedSplitReadsWaitForTheRunningRead(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a/x.go b/a/x.go\n--- a/a/x.go\n+++ b/a/x.go\n+x\ndiff --git a/b/y.go b/b/y.go\n--- a/b/y.go\n+++ b/b/y.go\n+y\n"
	fixture := newUnitFixture(t, diff, "branch", "round", "branch", "branch")
	fixture.manager.Settings.ReadSplitLines = 1
	plan, err := ReadUnitPlan(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Read.Outputs = []string{filepath.Join(t.TempDir(), "shared-read.md")}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	firstCollected := false
	fixture.runner.AfterWrite = func(record UnitRunRecord) error {
		if len(record.Rounds) > 0 {
			for _, step := range record.Rounds[0].Steps {
				if step.Name == "read:a" && step.State == StepPassed {
					firstCollected = true
				}
			}
		}
		return nil
	}
	readStarts := 0
	fixture.starter.onStart = func(record Record) error {
		if record.Kind == "read" {
			readStarts++
			if readStarts == 2 && !firstCollected {
				t.Error("second writer started before the first report was collected")
			}
		}
		return nil
	}
	fixture.starter.holdKind = "read"
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !first.Capped || first.Step != "read:a" || readStarts != 1 {
		t.Fatalf("first=%+v read starts=%d err=%v", first, readStarts, err)
	}
	held, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
	if err != nil || !held.Capped || held.Step != "read:a" || readStarts != 1 {
		t.Fatalf("resume while held=%+v read starts=%d err=%v", held, readStarts, err)
	}
	if second := stepNamed(t, held.Record.Rounds[0], "read:b"); second.State != StepPending || second.LaunchID != "" {
		t.Fatalf("second read moved while the first ran: %+v", second)
	}
	fixture.manager.Store.Update(held.Launch, func(record *Record) error {
		yes, code := true, 0
		record.State, record.ExitCode, record.VerdictCounts, record.Measurement.Verdict = Completed, &code, &yes, "pass"
		return nil
	})
	fixture.starter.holdKind = ""
	final, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
	if err != nil || final.Record.Rounds[0].Outcome != "green" || readStarts != 2 {
		t.Fatalf("outcome=%+v read starts=%d err=%v", final.Record.Rounds, readStarts, err)
	}
}

func TestEachRoundReadsFreshWithThePreviousReadAsInput(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "round", "branch", "round", "warm")
	output := filepath.Join(t.TempDir(), "read.out")
	os.WriteFile(output, []byte("done"), 0o600)
	fixture.starter.readOutput = output
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow.md")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil {
		t.Fatal(err)
	}
	read := stepNamed(t, second.Record.Rounds[1], "read")
	launched, _ := fixture.manager.Store.Read(read.LaunchID)
	var paths []string
	for _, input := range launched.Inputs {
		paths = append(paths, input.Path)
	}
	hasRetained := func(source string) bool {
		want, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(paths, func(path string) bool {
			got, err := os.ReadFile(path)
			return err == nil && string(got) == string(want)
		})
	}
	if !hasRetained(output) || !hasRetained(second.Record.Rounds[1].FollowUp) || stepNamed(t, second.Record.Rounds[0], "read").LaunchID == read.LaunchID {
		t.Fatalf("inputs=%v", paths)
	}
}

func TestFollowUpRefusedUnlessAwaitingJudgement(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "before", "branch")
	fixture.starter.holdKind = "build"
	result, _ := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("x"), 0o600)
	_, err := fixture.runner.Advance(UnitRequest{Resume: result.Record.ID, FollowUp: follow})
	if err == nil || !strings.Contains(ErrorDetail(err), "UNIT_STOPPED") {
		t.Fatalf("err=%v", err)
	}
	result.Record.State = "awaiting-judgement"
	if err := admitFollowUp(result.Record, filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(ErrorDetail(err), "UNIT_FOLLOW_UP_MISSING") {
		t.Fatalf("err=%v", err)
	}
}

func TestFollowUpStartsTheNextRoundOnTheSameWorktree(t *testing.T) {
	fixture := newUnitFixture(t, "", "branch", "round", "branch", "round")
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil || len(second.Record.Rounds) != 2 || second.Record.Worktree != first.Record.Worktree {
		t.Fatalf("result=%+v err=%v", second, err)
	}
	build, _ := fixture.manager.Store.Read(second.Record.Rounds[1].Steps[0].LaunchID)
	if build.WorkingDirectory != first.Record.Worktree {
		t.Fatalf("worktree=%s", build.WorkingDirectory)
	}
}

func newLinkedGitUnitFixture(t *testing.T) (unitFixture, string) {
	t.Helper()
	fixture, primary := newGitUnitFixture(t)
	if err := os.MkdirAll(filepath.Join(primary, "metasystem"), 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(primary, "metasystem/testing.json"), "original\n")
	runGit(t, primary, "add", "metasystem/testing.json")
	runGit(t, primary, "commit", "-m", "check input")
	runGit(t, primary, "branch", "-m", "main")
	linked := filepath.Join(t.TempDir(), "goal")
	runGit(t, primary, "worktree", "add", "-b", "goal/goal", linked)
	setUnitPlanWorktree(t, &fixture, linked)
	return fixture, primary
}

func TestAMoveOutsideTheCheckDoesNotStopIt(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"main", "ledger", "goal", "tag", "deleted-tag", "read-main"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture, primary := newLinkedGitUnitFixture(t)
			fixture.starter.readVerdict = "land"
			before := strings.TrimSpace(runGit(t, primary, "rev-parse", "HEAD"))
			want := RefMove{Ref: "refs/heads/main", Before: before, Step: "proof:check"}
			if name == "read-main" {
				want.Step = "read"
			}
			if name == "goal" || name == "tag" {
				want.Ref, want.Before = "refs/heads/goal/another", ""
				if name == "tag" {
					want.Ref = "refs/tags/published"
				}
			}
			if name == "deleted-tag" {
				runGit(t, primary, "tag", "published")
				want.Ref = "refs/tags/published"
			}
			moved := false
			fixture.starter.onStart = func(record Record) error {
				kind := "proof"
				if name == "read-main" {
					kind = "read"
				}
				if record.Kind != kind || moved {
					return nil
				}
				moved = true
				switch name {
				case "main", "ledger", "read-main":
					writeFile(t, filepath.Join(primary, "records"), record.ID)
					runGit(t, primary, "add", "records")
					runGit(t, primary, "commit", "-m", "records")
					if name == "ledger" {
						runGit(t, primary, "update-ref", "refs/metasystem/goals/accepted", "HEAD")
					}
				case "goal":
					runGit(t, primary, "branch", "goal/another")
				case "tag":
					runGit(t, primary, "tag", "published")
				case "deleted-tag":
					runGit(t, primary, "tag", "-d", "published")
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			if round.Outcome != "green" || len(stepNamed(t, round, "proof:check").LaunchIDs) != 1 {
				t.Fatalf("unrelated move stopped the check: %+v", round)
			}
			if name != "deleted-tag" {
				want.After = strings.TrimSpace(runGit(t, primary, "rev-parse", want.Ref))
			}
			if !reflect.DeepEqual(round.RefMoves, []RefMove{want}) || !round.ProofRefsNoted {
				t.Fatalf("ref observation: got %+v, want %+v; noted=%t", round.RefMoves, want, round.ProofRefsNoted)
			}
			if name == "main" || name == "goal" || name == "tag" || name == "deleted-tag" {
				read := stepNamed(t, round, "read")
				brief, err := os.ReadFile(read.Retained.Brief)
				from, to := "created", "deleted"
				if want.Before != "" {
					from = want.Before[:10]
				}
				if want.After != "" {
					to = want.After[:10]
				}
				paragraph := "While the checks ran, these refs moved; the check reads none of them: " + want.Ref + " " + from + " to " + to
				if err != nil || !strings.Contains(string(brief), paragraph) || !strings.HasPrefix(string(brief), "read it\n") {
					t.Fatalf("round-one read brief: %q, error=%v; want %q", brief, err, paragraph)
				}
			}
		})
	}
}

func TestTheFactKeepsTheLatestMove(t *testing.T) {
	t.Parallel()
	for _, returns := range []bool{false, true} {
		t.Run(fmt.Sprintf("returns=%t", returns), func(t *testing.T) {
			t.Parallel()
			fixture, primary := newLinkedGitUnitFixture(t)
			a := strings.TrimSpace(runGit(t, primary, "rev-parse", "HEAD"))
			var c string
			fixture.starter.failKind, fixture.starter.failCause, fixture.starter.readVerdict = "read", "environment", "land"
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "read" && strings.Contains(record.ID, "-retry") {
					fixture.starter.holdKind = "read"
					return nil
				}
				if record.Kind == "proof" || record.Kind == "read" {
					writeFile(t, filepath.Join(primary, "records"), record.ID)
					runGit(t, primary, "add", "records")
					runGit(t, primary, "commit", "-m", "records")
					if record.Kind == "read" {
						if returns {
							runGit(t, primary, "update-ref", "refs/heads/main", a)
						}
						c = strings.TrimSpace(runGit(t, primary, "rev-parse", "HEAD"))
					}
				}
				return nil
			}
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil || !first.Capped {
				t.Fatalf("held reader retry: %+v %v", first, err)
			}
			again, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
			if err != nil || !again.Capped {
				t.Fatalf("reader continuation: %+v %v", again, err)
			}
			fixture.starter.failKind, fixture.starter.holdKind = "", ""
			if _, err := fixture.manager.Store.Update(first.Launch, func(record *Record) error { record.State = Completed; return nil }); err != nil {
				t.Fatal(err)
			}
			last, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
			if err != nil {
				t.Fatal(err)
			}
			round := last.Record.Rounds[0]
			want := []RefMove{{Ref: "refs/heads/main", Before: a, After: c, Step: "read"}}
			if round.Outcome != "green" || !round.ProofRefsNoted || !reflect.DeepEqual(round.RefMoves, want) {
				t.Fatalf("continuation replaced the latest move: got %+v, want %+v; round=%+v", round.RefMoves, want, round)
			}
		})
	}
	t.Run("person-retry", func(t *testing.T) {
		t.Parallel()
		fixture, primary := newLinkedGitUnitFixture(t)
		a := strings.TrimSpace(runGit(t, primary, "rev-parse", "HEAD"))
		fixture.starter.failKind, fixture.starter.failCause, fixture.starter.readVerdict = "proof", "environment", "land"
		first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
		if err != nil || !first.Record.Rounds[0].ProofRefsNoted || first.Record.Rounds[0].Stop == nil {
			t.Fatalf("failed proof: %+v %v", first, err)
		}
		retry, err := fixture.runner.RetryFailedStep(first.Record.ID, "Wido", "retry the check", func() error { return nil })
		if err != nil || retry.Record.Rounds[0].ProofRefsNoted {
			t.Fatalf("person's retry kept the consumed proof window: %+v %v", retry, err)
		}
		fixture.starter.failKind = ""
		fixture.starter.onStart = func(record Record) error {
			if record.Kind == "proof" {
				writeFile(t, filepath.Join(primary, "records"), record.ID)
				runGit(t, primary, "add", "records")
				runGit(t, primary, "commit", "-m", "records")
			}
			return nil
		}
		last, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
		if err != nil {
			t.Fatal(err)
		}
		want := []RefMove{{Ref: "refs/heads/main", Before: a, After: strings.TrimSpace(runGit(t, primary, "rev-parse", "HEAD")), Step: "proof:check"}}
		if round := last.Record.Rounds[0]; round.Outcome != "green" || !round.ProofRefsNoted || !reflect.DeepEqual(round.RefMoves, want) {
			t.Fatalf("person's retry lost its ref move: %+v, want %+v", round, want)
		}
	})
}

func TestAChangedWorktreeFileStopsTheRoundByName(t *testing.T) {
	t.Parallel()
	for _, primaryChange := range []bool{false, true} {
		t.Run(fmt.Sprintf("primary=%t", primaryChange), func(t *testing.T) {
			t.Parallel()
			fixture, primary := newLinkedGitUnitFixture(t)
			fixture.starter.readVerdict = "land"
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "proof" && !strings.Contains(record.ID, "-retry") {
					tree := fixture.worktree
					if primaryChange {
						tree = primary
					}
					writeFile(t, filepath.Join(tree, "metasystem/testing.json"), "changed\n")
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			proof := stepNamed(t, round, "proof:check")
			if primaryChange {
				if round.Outcome != "green" || len(proof.LaunchIDs) != 1 {
					t.Fatalf("another worktree stopped the check: %+v", round)
				}
				return
			}
			if proof.State != StepPassed || len(proof.Moved) != 0 || len(proof.LaunchIDs) != 2 || round.Outcome != "proof-wrote" || round.Cause != "environment" || round.Stop == nil || round.Stop.Class != "proof: the check's inputs changed during the checks: worktree file metasystem/testing.json" || stepNamed(t, round, "read").Reason != "proof-wrote:worktree file metasystem/testing.json" {
				t.Fatalf("changed file lost its name through the retry: %+v", round)
			}
		})
	}
}

func TestARefOnlyReentryKeepsTheBaseline(t *testing.T) {
	t.Parallel()
	for _, changedFile := range []bool{false, true} {
		t.Run(fmt.Sprintf("file=%t", changedFile), func(t *testing.T) {
			t.Parallel()
			fixture, primary := newLinkedGitUnitFixture(t)
			fixture.starter.holdKind, fixture.starter.readVerdict = "proof", "land"
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil || !first.Capped {
				t.Fatalf("held proof: %+v %v", first, err)
			}
			path := filepath.Join(first.Record.Rounds[0].Directory, "proof-before.json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if changedFile {
				writeFile(t, filepath.Join(fixture.worktree, "tracked"), "changed\n")
			} else {
				writeFile(t, filepath.Join(primary, "records"), "records\n")
				runGit(t, primary, "add", "records")
				runGit(t, primary, "commit", "-m", "records")
			}
			again, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
			if err != nil {
				t.Fatal(err)
			}
			if changedFile {
				step := stepNamed(t, again.Record.Rounds[0], "proof:check")
				if step.State != StepFailed || step.Reason != "the check's inputs changed during the step: worktree file tracked" {
					t.Fatalf("reentry lost changed input: %+v", again.Record)
				}
				return
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) || !again.Capped || again.Record.Rounds[0].Stop != nil {
				t.Fatalf("ref-only reentry replaced its baseline or failed: %+v %v", again.Record, err)
			}
			fixture.starter.holdKind = ""
			if _, err := fixture.manager.Store.Update(first.Launch, func(record *Record) error { record.State = Completed; return nil }); err != nil {
				t.Fatal(err)
			}
			last, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID})
			if err != nil || last.Record.Rounds[0].Outcome != "green" {
				t.Fatalf("released proof: %+v %v", last.Record, err)
			}
		})
	}
}

func TestAStepKeepsItsEngineWhenTheInstallationIsReplaced(t *testing.T) {
	t.Parallel()
	fixture, repo := newGitUnitFixture(t)
	engine := filepath.Join(t.TempDir(), "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, engine, "original engine\n")
	plan, err := ReadUnitPlan(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Proof[0].Name, plan.Proof[0].Argv = "unit-check", []string{engine}
	if err := writeUnitJSON(fixture.plan, plan, ""); err != nil {
		t.Fatal(err)
	}
	var retained string
	fixture.starter.onStart = func(record Record) error {
		if record.Kind != "proof" {
			return nil
		}
		var brief PlainBrief
		data, err := os.ReadFile(readString(record.AdapterData, "brief"))
		if err != nil || json.Unmarshal(data, &brief) != nil {
			t.Fatalf("proof brief: %s %v", data, err)
		}
		if retained == "" {
			retained = filepath.Join(filepath.Dir(filepath.Dir(readString(record.AdapterData, "brief"))), "step-2-inputs/bin/metasystem")
			writeFile(t, engine+".stage", "replacement engine\n")
			if err := os.Rename(engine+".stage", engine); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(repo, "tracked"), "changed\n")
		}
		data, err = os.ReadFile(brief.Argv[0])
		if brief.Argv[0] != retained || err != nil || string(data) != "original engine\n" {
			t.Fatalf("step executed replaced engine: argv=%v bytes=%q error=%v", brief.Argv, data, err)
		}
		return nil
	}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || len(stepNamed(t, result.Record.Rounds[0], "proof:unit-check").LaunchIDs) != 2 {
		t.Fatalf("retained engine retry: %+v %v", result.Record, err)
	}
}

func TestAGenericProofKeepsItsCommand(t *testing.T) {
	t.Parallel()
	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprintf("absolute=%t", absolute), func(t *testing.T) {
			t.Parallel()
			fixture, _ := newGitUnitFixture(t)
			fixture.starter.readVerdict = "land"
			plan, err := ReadUnitPlan(fixture.plan)
			if err != nil {
				t.Fatal(err)
			}
			if absolute {
				engine := filepath.Join(t.TempDir(), "metasystem")
				writeFile(t, engine, "generic program\n")
				plan.Proof[0].Argv = []string{engine}
				if err := writeUnitJSON(fixture.plan, plan, ""); err != nil {
					t.Fatal(err)
				}
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			step := stepNamed(t, round, "proof:check")
			original, err := os.ReadFile(filepath.Join(round.Directory, "proof-check.json"))
			if err != nil {
				t.Fatal(err)
			}
			retained, err := os.ReadFile(step.Retained.Brief)
			var brief PlainBrief
			_, binErr := os.Stat(filepath.Join(round.Directory, "step-2-inputs/bin"))
			if err != nil || string(retained) != string(original) || json.Unmarshal(retained, &brief) != nil || brief.Argv[0] != plan.Proof[0].Argv[0] || !os.IsNotExist(binErr) || round.Outcome != "green" {
				t.Fatalf("generic command changed: %s %v bin=%v round=%+v", retained, err, binErr, round)
			}
		})
	}
}

func TestProofThatMovesTheRepositoryRetriesTheStep(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, moved string
		effect      func(*testing.T, string)
	}{
		{"commit", "head", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "committed"), "new\n")
			runGit(t, repo, "add", "committed")
			runGit(t, repo, "commit", "-m", "proof commit")
		}},
		// The presence publisher and the goal ledger's fetch move these refs
		// while any proof runs; the proof wrote none of them.
		{"background-refs", "", func(t *testing.T, repo string) {
			for _, ref := range []string{"refs/remotes/origin/presence/m1x", "refs/metasystem/presence-copy/m1x", "refs/metasystem/goals/accepted"} {
				runGit(t, repo, "update-ref", ref, "HEAD")
			}
		}},
		{"staged-blob-and-path", "index", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "staged"), "new\n")
			runGit(t, repo, "add", "staged")
		}},
		{"staged-mode", "index", func(t *testing.T, repo string) {
			if err := os.Chmod(filepath.Join(repo, "tracked"), 0o700); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "add", "tracked")
		}},
		{"staged-rename", "index", func(t *testing.T, repo string) { runGit(t, repo, "mv", "tracked", "renamed") }},
		{"unmerged-stages", "index", func(t *testing.T, repo string) { setConflictStages(t, repo, "tracked") }},
		{"assume-unchanged", "index", func(t *testing.T, repo string) { runGit(t, repo, "update-index", "--assume-unchanged", "tracked") }},
		{"skip-worktree", "index", func(t *testing.T, repo string) { runGit(t, repo, "update-index", "--skip-worktree", "tracked") }},
		{"intent-to-add", "index", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "intent"), "")
			runGit(t, repo, "add", "-N", "intent")
		}},
		{"resolve-undo", "index", func(t *testing.T, repo string) {
			setConflictStages(t, repo, "tracked")
			runGit(t, repo, "add", "tracked")
			if output := runGit(t, repo, "ls-files", "--resolve-undo", "--", "tracked"); output == "" {
				t.Fatal("fixture did not create resolve-undo state")
			}
		}},
		{"tree", "tree", func(t *testing.T, repo string) { writeFile(t, filepath.Join(repo, "tracked"), "changed\n") }},
		{"stat-cache-refresh", "", func(t *testing.T, repo string) {
			before := indexFileDigest(t, repo)
			bumpTrackedMtime(t, repo)
			runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
			if after := indexFileDigest(t, repo); after == before {
				t.Fatal("git status did not rewrite the fixture index cache")
			}
		}},
		{"optional-locks-read", "", func(t *testing.T, repo string) {
			before := indexFileDigest(t, repo)
			bumpTrackedMtime(t, repo)
			runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=0"}, "status", "--short")
			if after := indexFileDigest(t, repo); after != before {
				t.Fatal("GIT_OPTIONAL_LOCKS=0 rewrote the fixture index")
			}
		}},
		{"none", "", nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture, repo := newGitUnitFixture(t)
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "proof" && row.effect != nil && !strings.Contains(record.ID, "-retry") {
					row.effect(t, repo)
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if row.moved == "" {
				if result.Record.Rounds[0].Outcome != "green" {
					t.Fatalf("record=%+v", result.Record)
				}
				return
			}
			round := result.Record.Rounds[0]
			proof := stepNamed(t, round, "proof:check")
			if round.Outcome != "proof-wrote" || round.Cause != "environment" || round.Stop == nil || len(proof.LaunchIDs) != 2 || stepNamed(t, round, "read").State != StepSkipped {
				t.Fatalf("tree movement did not hold the changed result after one proof retry: %+v", round)
			}
			name := map[string]string{"head": "the goal branch head", "index": "the worktree's staged state", "tree": "worktree file tracked"}[row.moved]
			if !strings.Contains(round.Stop.Class, name) {
				t.Fatalf("changed input lost its name: %s", round.Stop.Class)
			}
		})
	}
}

func TestRepositorySnapshotIgnoresIndexCacheRefresh(t *testing.T) {
	t.Parallel()
	fixture, repo := newGitUnitFixture(t)
	before, err := fixture.runner.snapshotRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	rawBefore := indexFileDigest(t, repo)
	bumpTrackedMtime(t, repo)
	runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
	if rawAfter := indexFileDigest(t, repo); rawAfter == rawBefore {
		t.Fatal("git status did not rewrite the fixture index cache")
	}
	after, err := fixture.runner.snapshotRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if changed := before.changed(after); len(changed) != 0 {
		t.Fatalf("cache refresh changed repository snapshot: %v", changed)
	}
}

func TestNestedWorktreeProtectsRepositoryWideIndexState(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		moved  bool
		effect func(*testing.T, string, string)
	}{
		{"staged-sibling", true, func(t *testing.T, repo, _ string) {
			writeFile(t, filepath.Join(repo, "staged-sibling"), "new\n")
			runGit(t, repo, "add", "staged-sibling")
		}},
		{"assume-unchanged-sibling", true, func(t *testing.T, repo, _ string) {
			runGit(t, repo, "update-index", "--assume-unchanged", "tracked")
		}},
		{"root-stat-cache-refresh", false, func(t *testing.T, repo, _ string) {
			before := indexFileDigest(t, repo)
			bumpTrackedMtime(t, repo)
			runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
			if after := indexFileDigest(t, repo); after == before {
				t.Fatal("root status did not rewrite the fixture index cache")
			}
		}},
		{"nested-stat-cache-refresh", false, func(t *testing.T, repo, nested string) {
			before := indexFileDigest(t, repo)
			path := filepath.Join(nested, "tracked")
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := info.ModTime().Add(24 * time.Hour)
			if err := os.Chtimes(path, changed, changed); err != nil {
				t.Fatal(err)
			}
			runGitEnv(t, nested, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
			if after := indexFileDigest(t, repo); after == before {
				t.Fatal("nested status did not rewrite the fixture index cache")
			}
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture, repo := newGitUnitFixture(t)
			nested := filepath.Join(repo, "nested")
			if err := os.Mkdir(nested, 0o700); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(nested, "tracked"), "original\n")
			runGit(t, repo, "add", "nested/tracked")
			runGit(t, repo, "commit", "-m", "add nested worktree")
			setUnitPlanWorktree(t, &fixture, nested)
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "proof" && !strings.Contains(record.ID, "-retry") {
					row.effect(t, repo, nested)
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if row.moved && (result.Record.Rounds[0].Outcome != "proof-wrote" || result.Record.Rounds[0].Cause != "environment" || result.Record.Rounds[0].Stop == nil || stepNamed(t, result.Record.Rounds[0], "read").State != StepSkipped || len(stepNamed(t, result.Record.Rounds[0], "proof:check").LaunchIDs) != 2) {
				t.Fatalf("repository-wide index mutation was not detected: %+v", result.Record)
			}
			if !row.moved && (result.Record.Rounds[0].Outcome != "green" || len(stepNamed(t, result.Record.Rounds[0], "proof:check").LaunchIDs) != 1) {
				t.Fatalf("harmless cache refresh stopped the read: %+v", result.Record)
			}
		})
	}
}

func TestUnitRunNeverWritesToTheRepository(t *testing.T) {
	t.Parallel()
	fixture, repo := newGitUnitFixture(t)
	before := ""
	build := fixture.starter.onBuild
	fixture.starter.onBuild = func(record Record) error {
		if err := build(record); err != nil {
			return err
		}
		before = repositoryDigest(t, repo)
		return nil
	}
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	if actual := repositoryDigest(t, repo); actual != before {
		t.Fatalf("engine changed the repository after the builder: %s != %s", actual, before)
	}
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	if _, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow}); err != nil {
		t.Fatal(err)
	}
	after := repositoryDigest(t, repo)
	if before != after {
		t.Fatalf("repository changed\nbefore=%s\nafter=%s", before, after)
	}
	git := fixture.runner.Git.(*recordingOSGit)
	for index, call := range git.calls {
		if !slices.Contains([]string{"symbolic-ref", "rev-parse", "for-each-ref", "ls-files", "diff-index", "add", "diff", "read-tree", "write-tree", "apply"}, call[0]) {
			t.Fatalf("git call=%v", call)
		}
		if slices.Contains([]string{"add", "read-tree", "write-tree", "apply"}, call[0]) && !envOutside(git.envs[index], first.Record.Worktree) {
			t.Fatalf("add env=%v", git.envs[index])
		}
	}
}

func newGitUnitFixture(t *testing.T) (unitFixture, string) {
	t.Helper()
	fixture := baseUnitFixture(t)
	repo := t.TempDir()
	initializeGoalRepository(t, repo, "goal")
	writeFile(t, filepath.Join(repo, "tracked"), "original\n")
	runGit(t, repo, "add", "tracked")
	runGit(t, repo, "commit", "-m", "base")
	data, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	var plan UnitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan.Worktree = repo
	plan.Base = strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	plan.Proof[0].Dir = repo
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.starter.onBuild = func(record Record) error {
		return os.WriteFile(filepath.Join(record.WorkingDirectory, "built"), []byte("implemented\n"), 0600)
	}
	fixture.runner.Git = &recordingOSGit{}
	fixture.worktree = repo
	return fixture, repo
}

func setUnitPlanWorktree(t *testing.T, fixture *unitFixture, worktree string) {
	t.Helper()
	data, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	var plan UnitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan.Worktree = worktree
	plan.Base = strings.TrimSpace(runGit(t, worktree, "rev-parse", "HEAD"))
	plan.Proof[0].Dir = worktree
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.worktree = worktree
}

func initializeGoalRepository(t *testing.T, directory, goal string) {
	t.Helper()
	runGit(t, directory, "init")
	runGit(t, directory, "symbolic-ref", "HEAD", "refs/heads/goal/"+goal)
	runGit(t, directory, "config", "user.email", "test@example.invalid")
	runGit(t, directory, "config", "user.name", "Test")
	runGit(t, directory, "commit", "--allow-empty", "-m", "base")
}

func repositoryDigest(t *testing.T, repo string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		relative, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(digest, "%s %d %x\n", relative, info.Size(), sum)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	head := runGit(t, repo, "rev-parse", "HEAD")
	refs := runGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
	indexPath := strings.TrimSpace(runGit(t, repo, "rev-parse", "--path-format=absolute", "--git-path", "index"))
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	indexSum := sha256.Sum256(index)
	_, _ = fmt.Fprintf(digest, "head=%srefs=%sindex=%x\n", head, refs, indexSum)
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func setConflictStages(t *testing.T, repo, path string) {
	t.Helper()
	oid := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD:"+path))
	zero := strings.Repeat("0", len(oid))
	input := fmt.Sprintf("0 %s\t%s\n100644 %s 1\t%s\n100644 %s 2\t%s\n100644 %s 3\t%s\n", zero, path, oid, path, oid, path, oid, path)
	runGitInput(t, repo, input, "update-index", "--index-info")
}

func bumpTrackedMtime(t *testing.T, repo string) {
	t.Helper()
	path := filepath.Join(repo, "tracked")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := info.ModTime().Add(24 * time.Hour)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}
}

func indexFileDigest(t *testing.T, repo string) string {
	t.Helper()
	path := strings.TrimSpace(runGit(t, repo, "rev-parse", "--path-format=absolute", "--git-path", "index"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func stepNamed(t *testing.T, round UnitRound, name string) UnitStep {
	t.Helper()
	for _, step := range round.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("step %s not found in %+v", name, round.Steps)
	return UnitStep{}
}

func envOutside(env []string, worktree string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			return !strings.HasPrefix(strings.TrimPrefix(entry, "GIT_INDEX_FILE="), worktree+string(os.PathSeparator))
		}
	}
	return false
}
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGitEnv(t, dir, nil, args...)
}

func runGitEnv(t *testing.T, dir string, environment []string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = childEnvironment(os.Environ(), environment)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func runGitInput(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Stdin = strings.NewReader(input)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestCountedCapRefusesTheNextRound(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "round", "branch", "round", "branch")
	fixture.manager.Settings.UnitCountedRounds = 2
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || first.Record.CountedCap != 2 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	fixture.manager.Settings.UnitCountedRounds = 9
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: writeFollowUp(t)})
	if err != nil || second.Record.CountedCap != 2 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	assertCapAdmissions(t, fixture, second.Record, "UNIT_STOPPED")
}

// A refused admission leaves the retained record, round directories and launches alone.
func assertCapAdmissions(t *testing.T, fixture unitFixture, record UnitRunRecord, code string) {
	t.Helper()
	path := filepath.Join(fixture.runner.runDir(record.ID), "run.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(fixture.runner.runDir(record.ID))
	launched := len(fixture.starter.ids)
	_, followErr := fixture.runner.Advance(UnitRequest{Resume: record.ID, FollowUp: writeFollowUp(t)})
	_, reviseErr := fixture.runner.Revise(UnitRevisionRequest{Run: record.ID, Brief: []byte("Declared size: 1 changed lines\n")})
	for _, err := range []error{followErr, reviseErr} {
		if !IsCode(err, code) {
			t.Fatalf("refusal=%v want %s", err, code)
		}
	}
	after, _ := os.ReadFile(path)
	afterEntries, _ := os.ReadDir(fixture.runner.runDir(record.ID))
	if string(after) != string(before) || len(afterEntries) != len(entries) || len(fixture.starter.ids) != launched {
		t.Fatal("a refused admission wrote state or started a launch")
	}
}

func TestUnknownReadDoesNotConsumeCorrectionRound(t *testing.T) {
	t.Parallel()
	for _, available := range []bool{true, false} {
		t.Run(fmt.Sprintf("fresh-available=%t", available), func(t *testing.T) {
			t.Parallel()
			fixture := newUnitFixture(t, "", "branch", "round", "branch")
			fixture.manager.Settings.UnitCountedRounds = 1
			fixture.starter.skipStructured = true
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if counted, environment := countedRounds(first.Record); counted != 0 || environment != 1 {
				t.Fatalf("unknown read consumed budget: counted=%d environment=%d", counted, environment)
			}
			fixture.starter.skipStructured = !available
			fresh, err := fixture.runner.RetryUnknownRead(first.Record.ID)
			if err != nil || len(fresh.Record.Rounds) != 1 || fresh.Record.Rounds[0].UnknownRetries != 1 {
				t.Fatalf("fresh examination changed attempt: %+v %v", fresh, err)
			}
			wantCounted, wantEnvironment := 0, 1
			if available {
				wantCounted, wantEnvironment = 1, 0
			}
			if counted, environment := countedRounds(fresh.Record); counted != wantCounted || environment != wantEnvironment {
				t.Fatalf("completed examination count: counted=%d environment=%d", counted, environment)
			}
			launched := len(fixture.starter.order)
			if _, err := fixture.runner.RetryUnknownRead(first.Record.ID); !IsCode(err, "UNIT_STOPPED") {
				t.Fatalf("another examination was admitted: %v", err)
			}
			if len(fixture.starter.order) != launched {
				t.Fatal("another examination launched")
			}
			assertCapAdmissions(t, fixture, fresh.Record, "UNIT_STOPPED")

		})
	}
}

func TestCapNamesTheSplitForAnUncommittedUnit(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, verdict, examination, next, reason string
		committed                                bool
	}{
		{"material", "fix first (2 material findings)", "", "work build G --work NEW --brief FILE --check ...", "worktree as it stands", false},
		{"clean", "land", "", "work review G --work U", "goal notes G --read R --add TEXT", false},
		{"committed", "fix first (2 material findings)", "", "work build G --work NEW", "goal accept-risk G --finding F naming NEW", true},
		{"examination overrides clean read", "land", structuredUnitReturn(1, "regression", "code.go"), "work build G --work NEW", "goal accept-risk", true},
		{"clean examination overrides material read", "fix first (2 material findings)", structuredUnitReturn(0, "regression", "code.go"), "work review G --work U", "goal notes", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newUnitFixture(t, "", []string{}...)
			yes := true
			record := UnitRunRecord{ID: "R", Unit: "U", Goal: "G", CountedCap: 2,
				Rounds: []UnitRound{{Number: 1, Cause: "own"}, {Number: 2, Cause: "own", Steps: []UnitStep{{Name: "read", Verdict: test.verdict, VerdictCounts: &yes}}}, {Number: 3, Cause: "provider-limit"}}}
			read, err := readsubject.Collect("launch", readsubject.ReadSubject{}, "engine", "model", "return.json", []byte(structuredUnitReturn(2, "regression", "code.go")), test.verdict)
			if strings.EqualFold(test.verdict, "land") {
				read, err = readsubject.Collect("launch", readsubject.ReadSubject{}, "engine", "model", "return.json", []byte(structuredUnitReturn(0, "regression", "code.go")), test.verdict)
			}
			if err != nil {
				t.Fatal(err)
			}
			record.Rounds[1].Reads = []readsubject.Read{read}
			if test.committed {
				record.Subjects = []UnitSubject{{Round: 2, Commit: "commit"}}
			}
			if test.examination != "" {
				fixture.runner.ExaminationRoot = t.TempDir()
				record.Subjects[0].Examination, record.Subjects[0].ExaminationRound = "critic", 1
				path := filepath.Join(fixture.runner.ExaminationRoot, "artifacts", "agents", "critic", "rounds", "1", "return.json")
				record.Subjects[0].ExaminationReturnPath = path
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, test.examination)
				read, err := readsubject.Collect("critic", readsubject.ReadSubject{}, "engine", "model", path, []byte(test.examination), "")
				if err != nil {
					t.Fatal(err)
				}
				record.Rounds[1].Reads = []readsubject.Read{read}
			}
			err = fixture.runner.countedCap(record)
			var cap *CodedError
			if !errors.As(err, &cap) || cap.Code != "UNIT_ROUND_CAP" || !strings.Contains(cap.Run, test.next) || !strings.Contains(cap.Reason.Error(), test.reason) ||
				!strings.Contains(cap.Facts, "run=R counted=2 cap=2 machinery=1") || !strings.HasPrefix(cap.Reason.Error(), "unit U has used its 2 counted rounds; nothing was started") {
				t.Fatalf("wrong outcome: %v", err)
			}
		})
	}
}

func TestLegacyRunWithoutStopRequiresRead(t *testing.T) {
	t.Parallel()
	for _, revise := range []bool{false, true} {
		t.Run(fmt.Sprint(revise), func(t *testing.T) {
			t.Parallel()
			events := []string{"branch", "round"}
			if !revise {
				events = append(events, "branch")
			}
			fixture := newUnitFixture(t, "", events...)
			first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			record := first.Record
			record.CountedCap = 0
			record.Rounds[0].Reads = nil
			record.Rounds[0].Stop = nil
			record.Rounds[0].Material = -1
			if err := fixture.runner.save(record); err != nil {
				t.Fatal(err)
			}
			launched := len(fixture.starter.order)
			if revise {
				_, err = fixture.runner.Revise(UnitRevisionRequest{Run: record.ID, Brief: []byte("Declared size: 1 changed lines\n")})
			} else {
				_, err = fixture.runner.Advance(UnitRequest{Resume: record.ID, FollowUp: writeFollowUp(t)})
			}
			if !IsCode(err, "UNIT_STOPPED") || !strings.Contains(err.Error(), "no recorded decision") {
				t.Fatalf("legacy input must require a read: %v", err)
			}
			requireRecordedRounds(t, fixture, record.ID, 1)
			if len(fixture.starter.order) != launched {
				t.Fatal("unknown legacy input launched work")
			}
		})
	}
}

func TestRoundMaterialAndJudgement(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", []string{}...)
	yes, no := true, false
	record := UnitRunRecord{ID: "round-material", Worktree: fixture.worktree, Goal: "goal", Unit: "U", CountedCap: 6, MaxRounds: 20, Rounds: []UnitRound{{Number: 1, Cause: "provider-limit"}, {Number: 2, Directory: filepath.Join(fixture.runner.Root, "round-material", "round-2")}}}
	os.MkdirAll(record.Rounds[1].Directory, 0700)
	for _, item := range []struct {
		id       string
		material int
		counts   bool
	}{{"a", 2, yes}, {"b", 3, yes}, {"c", 9, no}, {"d", 0, yes}} {
		// The fixture retains immutable evidence under the launch's own state.
		stateDir, err := fixture.manager.Store.StateDir(item.id)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(stateDir, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(stateDir, "return.json")
		writeFile(t, path, structuredUnitReturn(item.material, "regression", item.id+".go"))
		counts := item.counts
		launch := Record{ID: item.id, Kind: "read", State: Completed, VerdictCounts: &counts, AdapterData: map[string]json.RawMessage{}, Measurement: Measurement{Verdict: fmt.Sprintf("fix first (%d material findings)", item.material)}}
		setStrings(launch.AdapterData, "declaredOutputs", []string{path})
		diff := filepath.Join(filepath.Dir(path), "read.diff")
		writeFile(t, diff, "reviewed diff")
		setString(launch.AdapterData, "readDiff", diff)
		if err := fixture.manager.Store.Create(launch); err != nil {
			t.Fatal(err)
		}
		record.Rounds[1].Steps = append(record.Rounds[1].Steps, UnitStep{Name: "read:" + item.id, LaunchID: item.id, Verdict: launch.Measurement.Verdict, VerdictCounts: &counts, State: StepPassed})
	}

	finished, err := fixture.runner.finish(&record, &record.Rounds[1], "green")
	if err != nil || finished.Record.Rounds[1].Material != 5 {
		t.Fatalf("finish=%+v err=%v", finished, err)
	}
	retained := requireRecordedRounds(t, fixture, record.ID, 2)
	if retained.Rounds[1].Material != 5 {
		t.Fatal("material was not retained")
	}
	card := judgementRound(record, 2)
	if card.N != 1 || card.Max == nil || *card.Max != 6 {
		t.Fatalf("judgement=%+v", card)
	}
	record.CountedCap = 0
	card = judgementRound(record, 2)
	if card.N != 2 || card.Max == nil || *card.Max != 20 {
		t.Fatalf("legacy judgement=%+v", card)
	}
}

func TestARefusedAdmissionLeavesTheStepsLaunchesUnchanged(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"load", "hook", "hook-error", "hook-refusal", "start"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f, _ := newGitUnitFixture(t)
			f.starter.readVerdict = "land"
			gate, attempts := false, map[string]int{}
			var retained, saved string
			f.runner.BeforeModelLaunch = func(_ UnitRunRecord, spec StartSpec) error {
				attempts[spec.ID]++
				if mode == "start" && strings.HasSuffix(spec.ID, "-retry") && attempts[spec.ID] > 1 {
					t.Fatalf("the machine repeated an unadmitted start: %s", spec.ID)
				}
				if gate && mode == "hook" {
					return coded("LAUNCH_BUILD_CAPACITY", "", errors.New("host busy"))
				}
				if gate && mode == "hook-error" {
					return errors.New("the hook died here")
				}
				if gate && mode == "hook-refusal" {
					return coded("BUDGET_REFUSED", "", errors.New("reservation unavailable"))
				}
				return nil
			}
			host := &checkHostLoad{}
			f.manager.CapacitySources.Load = host.sample
			f.starter.onStart = func(r Record) error {
				if r.Kind == "proof" && !strings.HasSuffix(r.ID, "-retry") {
					gate = true
					f.starter.failKind, f.starter.failCause = "proof", "process-lost"
					if mode == "load" {
						host.set(20)
					}
					if mode == "start" {
						retained = r.Inputs[0].Path
						saved = retained + ".saved"
						return os.Rename(retained, saved)
					}
				} else {
					f.starter.failKind = ""
				}
				return nil
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if mode != "start" && result.Record.Rounds[0].Steps[1].LaunchID != result.Record.ID+"-r1-s2-retry" {
				t.Fatalf("the wait returned the prior launch instead of its candidate: %+v", result.Record.Rounds[0].Steps[1])
			}
			r, statusErr := f.runner.Status(result.Record.ID)
			if statusErr != nil {
				t.Fatal(statusErr)
			}
			step, round := r.Rounds[0].Steps[1], r.Rounds[0]
			original := r.ID + "-r1-s2"
			candidate := original + "-retry"
			if !slices.Equal(step.LaunchIDs, []string{original}) {
				t.Fatalf("unstarted candidate was listed: %+v", step)
			}
			if _, readErr := f.manager.Store.Read(candidate); !errors.Is(readErr, fs.ErrNotExist) {
				t.Fatalf("unstarted launch: %v", readErr)
			}
			if mode != "start" {
				returned := IsCode(err, "LAUNCH_BUILD_CAPACITY")
				if mode == "hook-error" {
					returned = IsCode(err, "UNIT_LAUNCH_UNAUTHORIZED") && strings.Contains(err.Error(), "the hook died here")
				}
				if mode == "hook-refusal" {
					returned = IsCode(err, "BUDGET_REFUSED")
				}
				if !returned || step.State != StepStarting || step.LaunchID != candidate || step.Unadmitted != "" || round.Stop != nil {
					t.Fatalf("a wait or a hook refusal became a stop: %+v %v", round, err)
				}
				gate = false
				host.set(0)
				result, err = f.runner.Advance(UnitRequest{Resume: r.ID})
				step = result.Record.Rounds[0].Steps[1]
				if err != nil || !slices.Equal(step.LaunchIDs, []string{original, candidate}) || step.State != StepPassed {
					t.Fatalf("candidate did not resume: %+v %v", step, err)
				}
				return
			}
			if err != nil || step.State != StepFailed || step.Cause != "environment" || step.Unadmitted != candidate || step.Holds != 1 || step.LaunchID != original || round.Stop == nil || round.Stop.Class != step.Name+": "+step.Reason || (mode == "start" && !strings.Contains(step.Reason, retained)) || step.FinishedAt == "" || attempts[candidate] != 1 {
				t.Fatalf("failed admission was not held: %+v %v attempts=%v", round, err, attempts)
			}
			for pass := range 2 {
				if pass == 1 {
					if mode == "start" {
						if err := os.Rename(saved, retained); err != nil {
							t.Fatal(err)
						}
					}
					gate = false
					f.starter.holdKind = "proof"
				}
				if _, err := f.runner.RetryFailedStep(r.ID, "Wido", "start again", func() error { return nil }); err != nil {
					t.Fatal(err)
				}
				attempts = map[string]int{}
				result, err = f.runner.Advance(UnitRequest{Resume: r.ID})
				step = result.Record.Rounds[0].Steps[1]
				if err != nil || step.RetryLaunch != candidate+"#"+strconv.Itoa(pass+1) || attempts[candidate] != 1 {
					t.Fatalf("person's retry: %+v %v attempts=%v", step, err, attempts)
				}
				if pass == 0 && (step.Unadmitted != candidate || step.Holds != 2 || step.State != StepFailed) {
					t.Fatalf("refused person's retry: %+v", step)
				}
			}
			if step.State != StepRunning || step.Unadmitted != "" || !slices.Equal(step.LaunchIDs, []string{original, candidate}) {
				t.Fatalf("identical retry never admitted: %+v", step)
			}
			before, _ := json.Marshal(result.Record)
			again, err := f.runner.RetryFailedStep(r.ID, "Wido", "start again", func() error { t.Fatal("identical retry recorded twice"); return nil })
			after, _ := json.Marshal(again.Record)
			if err != nil || string(before) != string(after) {
				t.Fatalf("running retry changed: %v", err)
			}
		})
	}
	for _, mode := range []string{"hook", "admit"} {
		t.Run("read-"+mode+"-and-store", func(t *testing.T) {
			t.Parallel()
			f, _ := newGitUnitFixture(t)
			plan, _ := ReadUnitPlan(f.plan)
			plan.Read.Outputs = []string{filepath.Join(t.TempDir(), "findings.md")}
			if err := writeUnitJSON(f.plan, plan, ""); err != nil {
				t.Fatal(err)
			}
			f.starter.failKind, f.starter.failCause = "read", "process-lost"
			f.runner.BeforeModelLaunch = func(_ UnitRunRecord, spec StartSpec) error {
				if strings.HasSuffix(spec.ID, "-retry") {
					if mode == "admit" {
						f.manager.Settings.BriefCap = 1
						return nil
					}
					return coded("BUDGET_REFUSED", "", errors.New("budget exhausted"))
				}
				return nil
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if mode == "hook" && !IsCode(err, "BUDGET_REFUSED") {
				t.Fatalf("the read retry's budget refusal did not return to the caller: %v", err)
			}
			if mode == "admit" && err != nil {
				t.Fatalf("read retry did not reach its hold: %v", err)
			}
			r, err := f.runner.Status(result.Record.ID)
			if err != nil {
				t.Fatal(err)
			}
			step := stepNamed(t, r.Rounds[0], "read")
			candidate := step.LaunchIDs[0] + "-retry"
			reason := "budget exhausted"
			if mode == "admit" {
				reason = "over the cap of 1"
			}
			round := r.Rounds[0]
			if mode == "hook" && (step.State != StepStarting || step.LaunchID != candidate || step.Unadmitted != "" || round.Stop != nil) {
				t.Fatalf("the read retry's budget refusal became a stop: %+v", round)
			}
			if mode == "admit" && (step.State != StepFailed || step.Cause != "environment" || step.Unadmitted != candidate || step.LaunchID != step.LaunchIDs[0] || step.FinishedAt == "" || !strings.Contains(step.Reason, reason) || round.Stop == nil || round.Stop.Class != "read: "+step.Reason) {
				t.Fatalf("read admission was not held by name: %+v", round)
			}
			state, _ := f.manager.Store.StateDir(candidate)
			if _, err := os.Stat(state); !errors.Is(err, fs.ErrNotExist) || len(step.LaunchIDs) != 1 {
				t.Fatalf("refused read created a folder or listed its candidate: %+v %v", step, err)
			}
			if err := os.MkdirAll(filepath.Join(f.manager.Store.Root, "never-started"), 0700); err != nil {
				t.Fatal(err)
			}
			listed, err := f.manager.Store.List()
			if err != nil || len(listed) != len(f.starter.ids) {
				t.Fatalf("empty folder stopped listing: %d %v", len(listed), err)
			}
			writeFile(t, filepath.Join(f.manager.Store.Root, "never-started", "record.json"), "unreadable json")
			if _, err := f.manager.Store.List(); err == nil {
				t.Fatal("damaged record was silently skipped")
			}
		})
	}
}

type unitReaderAdapter struct {
	fakeAdapter
	transcript string
}

// A hold is named by the admission it refused, with or without a listed
// launch (U1c1 read 2, F-1 and F-2); a failed launch keeps its own id.
func TestHoldFindingNamesTheRefusedAdmission(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		step UnitStep
		want string
	}{
		{UnitStep{Unadmitted: "run-r1-s2", Holds: 1}, "run-r1-s2#1"},
		{UnitStep{LaunchID: "run-r1-s2", LaunchIDs: []string{"run-r1-s2"}, Unadmitted: "run-r1-s2-retry", Holds: 2}, "run-r1-s2-retry#2"},
		{UnitStep{LaunchID: "run-r1-s2-retry", LaunchIDs: []string{"run-r1-s2", "run-r1-s2-retry"}}, "run-r1-s2-retry"},
	} {
		if got := row.step.HoldFinding(); got != row.want {
			t.Fatalf("hold finding %q, want %q for %+v", got, row.want, row.step)
		}
	}
}

func (a unitReaderAdapter) Transcript(Record, string) (string, error) {
	if a.transcript == "" {
		return "", errors.New("transcript missing")
	}
	return a.transcript, nil
}
func (a unitReaderAdapter) Command(r Record, state string) (Command, error) {
	data, err := os.ReadFile(readString(r.AdapterData, "brief"))
	return Command{Stdin: string(data), LogPath: filepath.Join(state, "log")}, err
}
func (a unitReaderAdapter) Measure(Record, string) (Measurement, []Output, map[string]json.RawMessage, error) {
	return Measurement{Verdict: "land"}, nil, nil, nil
}

type supervisingUnitReader struct{ *completingStarter }

func (s supervisingUnitReader) StartSupervisor(id, state string) (identity.Ref, error) {
	r, err := s.m.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if r.Kind != "read" {
		return s.completingStarter.StartSupervisor(id, state)
	}
	s.ids, s.order = append(s.ids, id), append(s.order, r.Kind)
	_, err = s.m.Supervise(id)
	return ref(10), err
}

func TestASilentReaderIsRetriedOnceThenStopsByName(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"silent", "retry-writes", "transcript-missing", "no-transcript-adapter"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f, _ := newGitUnitFixture(t)
			plan, _ := ReadUnitPlan(f.plan)
			output := filepath.Join(t.TempDir(), "findings.md")
			plan.Read.Outputs = []string{output}
			writeFile(t, plan.Read.Brief, "Write findings to: "+output+"\nStructured findings: "+output+".json\n")
			if err := writeUnitJSON(f.plan, plan, ""); err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			f.manager.Adapters["claude-headless"] = unitReaderAdapter{transcript: transcript}
			if mode == "transcript-missing" {
				f.manager.Adapters["claude-headless"] = unitReaderAdapter{}
			}
			if mode == "no-transcript-adapter" {
				f.manager.Adapters["claude-headless"] = struct{ Adapter }{unitReaderAdapter{}}
			}
			f.manager.Supervisor = supervisingUnitReader{f.starter}
			processes := f.manager.Processes.(*fakeProcesses)
			reads := 0
			processes.onStart = func(c Command) {
				reads++
				if mode == "retry-writes" && reads == 2 {
					path := strings.Split(strings.TrimPrefix(c.Stdin, "Write findings to: "), "\n")[0]
					writeFile(t, path, "Verdict: land\n")
					writeFile(t, path+".json", structuredUnitReturn(0, "regression", "unit.go"))
				}
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			step := stepNamed(t, round, "read")
			if reads != 2 || len(step.LaunchIDs) != 2 {
				t.Fatalf("reader was not retried once: reads=%d round=%+v", reads, round)
			}
			retry, err := f.manager.Store.Read(step.LaunchID)
			if err != nil {
				t.Fatal(err)
			}
			paths, _ := declaredOutputPaths(retry)
			want := "read: the reader " + retry.ID + " ended without writing its declared output " + paths[0] + "; transcript " + transcript
			if mode != "retry-writes" {
				if mode != "silent" {
					state, _ := f.manager.Store.StateDir(retry.ID)
					want = "read: the reader " + retry.ID + " ended without writing its declared output " + paths[0] + "; transcript " + state
				}
				if round.Outcome != "read-failed" || round.Cause != "environment" || round.Material != -1 || round.Stop == nil || round.Stop.Class != want {
					t.Fatalf("silent reader lost its named hold: %+v want=%s", round, want)
				}
				return
			}
			briefData, _ := os.ReadFile(retry.Inputs[0].Path)
			retained, _ := os.ReadFile(step.Retained.Brief)
			if round.Outcome != "green" || len(round.Reads) != 1 || !strings.Contains(string(briefData), "Write findings to: "+paths[0]+"\n") || !strings.Contains(string(briefData), "Structured findings: "+paths[0]+".json\n") || !strings.Contains(string(retained), output) || paths[0] == output {
				t.Fatalf("reader's brief and collected output disagree: round=%+v brief=%s paths=%v", round, briefData, paths)
			}
		})
	}
}

func TestClaudeTranscriptNamesTheRecordedSession(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "session.jsonl")
	writeFile(t, path, "{}\n")
	r := Record{ID: "reader", AdapterData: map[string]json.RawMessage{"sessionID": rawString("session")}}
	adapter := ClaudeHeadless{ProjectsRoot: root}
	if got, err := adapter.Transcript(r, "unused"); err != nil || got != path {
		t.Fatalf("transcript=%s err=%v", got, err)
	}
	setString(r.AdapterData, "sessionID", "missing")
	if _, err := adapter.Transcript(r, "unused"); err == nil {
		t.Fatal("missing transcript was reported as found")
	}
	adapter.ProjectsRoot = filepath.Join(root, "missing")
	if _, err := adapter.Transcript(r, "unused"); err == nil {
		t.Fatal("unreadable projects root was reported as found")
	}
}
