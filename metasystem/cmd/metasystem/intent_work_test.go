package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// workBed is an isolated goal ledger with a goal/G worktree, a private
// launch and unit run store, and a starter that finishes each launch the
// way its supervisor would. The unit runner, its plan reader, its named
// retry entry and its judgement state are the real ones.
type workBed struct {
	*intentBed
	id, worktree string
	manager      *launch.Manager
	starter      *workStarter
	unitRoot     string
	branchListed bool
	head         string
	ledger       sync.Mutex
	// readDirs are the read-findings directories this bed's builds created
	// under the temporary root, found through each result's plan.
	readDirsMu sync.Mutex
	readDirs   map[string]bool
}

type workClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *workClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *workClock) Sleep(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type workStarter struct {
	mu    sync.Mutex
	m     *launch.Manager
	fail  map[string]bool
	hold  string
	kinds []string
	// author, when set, is the fake design author: it writes the staged
	// page before the launch completes.
	author func(launch.Record)
}

func (s *workStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	record, _ := s.m.Store.Read(id)
	s.mu.Lock()
	s.kinds = append(s.kinds, record.Kind)
	red, hold := s.fail[record.Kind], s.hold == record.Kind
	author := s.author
	s.mu.Unlock()
	if author != nil && record.Kind == "design" && !hold {
		author(record)
	}
	s.m.Store.Update(id, func(current *launch.Record) error {
		if hold {
			supervisor, child := workRef(10), workRef(20)
			current.Supervisor, current.Child, current.State = &supervisor, &child, launch.Running
			return nil
		}
		code := 0
		current.State = launch.Completed
		if red {
			current.State, current.Reason, code = launch.Failed, "fixture-red", 1
		}
		current.ExitCode = &code
		if record.Kind == "read" {
			yes := true
			current.VerdictCounts, current.Measurement.Verdict = &yes, "pass"
		}
		return nil
	})
	return workRef(10), nil
}

func (s *workStarter) launched() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.kinds...)
}

func workRef(pid int64) identity.Ref { return identity.Ref{Pid: pid, StartedAtSec: pid} }

type workAdapter struct{}

func (workAdapter) Command(record launch.Record, state string) (launch.Command, error) {
	return launch.Command{LogPath: filepath.Join(state, "log")}, nil
}
func (workAdapter) Measure(launch.Record, string) (launch.Measurement, []launch.Output, map[string]json.RawMessage, error) {
	return launch.Measurement{}, nil, nil, nil
}
func (workAdapter) Strays() ([]string, error) { return nil, nil }

type workProber struct{}

func (workProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == 10 || pid == 20 {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

type workProcesses struct{}

func (workProcesses) SelfRef() (identity.Ref, error) { return workRef(10), nil }
func (workProcesses) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	return nil, identity.Ref{}, errors.New("the fixture starts no child")
}
func (workProcesses) SignalGroup(int64, syscall.Signal) error { return nil }
func (workProcesses) GroupAlive(int64) (bool, error)          { return false, nil }

// workGit answers the unit runner's Git calls for a clean goal/G worktree.
type workGit struct{ bed *workBed }

func (g workGit) Run(directory string, _ []string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case joined == "symbolic-ref --short HEAD":
		return []byte("goal/" + g.bed.id + "\n"), nil
	case joined == "rev-parse --show-toplevel":
		return []byte(g.bed.worktree + "\n"), nil
	case joined == "rev-parse HEAD":
		return []byte(g.bed.head + "\n"), nil
	case strings.HasPrefix(joined, "for-each-ref"):
		return []byte("refs/heads/goal/" + g.bed.id + " " + g.bed.head + "\n"), nil
	case strings.HasSuffix(joined, "--git-path index"):
		return []byte(filepath.Join(filepath.Dir(g.bed.worktree), "index") + "\n"), nil
	case strings.HasSuffix(joined, "--git-path objects"):
		return []byte(filepath.Join(filepath.Dir(g.bed.worktree), "objects") + "\n"), nil
	}
	return nil, nil
}

func newWorkBed(t *testing.T) *workBed {
	t.Helper()
	return newWorkBedWith(t, workApprovedBox)
}

// newWorkBedWith is the work bed with its goal record shaped by amend.
func newWorkBedWith(t *testing.T, amend func(*goal.GoalFile)) *workBed {
	t.Helper()
	bed := &workBed{intentBed: newIntentBed(t, false, amend), id: "standing-validation", head: "base-commit", readDirs: map[string]bool{}}
	// A public build creates the read's findings directory under the shared
	// temporary root with a unique name; the bed removes only its own.
	t.Cleanup(bed.removeReadDirs)
	parent := t.TempDir()
	bed.worktree = filepath.Join(parent, "work")
	for _, dir := range []string{bed.worktree, filepath.Join(parent, "objects")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(parent, "index"), []byte("index"), 0o600)
	bed.branchListed = true
	clock := &workClock{now: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)}
	bed.manager = &launch.Manager{Store: launch.Store{Root: filepath.Join(parent, "launch")},
		Adapters:  map[string]launch.Adapter{"codex-exec": workAdapter{}, "claude-headless": workAdapter{}, "plain-exec": workAdapter{}},
		Processes: workProcesses{}, Prober: workProber{}, Now: clock.Now, Sleep: clock.Sleep, Grace: time.Second, Poll: time.Second}
	bed.manager.Settings = launch.DefaultSettings()
	bed.manager.Settings.WaitCapSeconds = 2
	for index, value := range bed.manager.Settings.Values {
		if value.Key == launch.ReadModelKey {
			bed.manager.Settings.Values[index].Value = "fixture-read-model"
		}
	}
	bed.starter = &workStarter{m: bed.manager, fail: map[string]bool{}}
	bed.manager.Supervisor = bed.starter
	bed.unitRoot = filepath.Join(parent, "unit")
	layout, err := bed.owners().resolver.ResolveLayout(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "templates", "review-brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	templates := filepath.Join(layout.InstallationRoot, "scripts", "agents", "templates")
	os.MkdirAll(templates, 0o700)
	if err := os.WriteFile(filepath.Join(templates, "review-brief.md"), template, 0o600); err != nil {
		t.Fatal(err)
	}
	designTemplate, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "templates", "design-brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templates, "design-brief.md"), designTemplate, 0o600); err != nil {
		t.Fatal(err)
	}
	bed.manager.TemplateDirectory = templates
	return bed
}

func (b *workBed) workOwners() intentOwners {
	owners := b.owners()
	// The shared goal-ledger fixture records its calls unguarded; concurrent
	// invocations read the ledger one at a time and race only in the runner.
	endpoint, commandNow := owners.dependencies.endpoint, owners.commandNow
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		b.ledger.Lock()
		defer b.ledger.Unlock()
		return endpoint(root)
	}
	owners.commandNow = func(root string) (time.Time, error) {
		b.ledger.Lock()
		defer b.ledger.Unlock()
		return commandNow(root)
	}
	// The work bed's goal is claimed by this session; the unit launch
	// authority asks this per-test claim owner.
	owners.connection = intentConnectionOwners{
		endpoint: func(string) (goal.Endpoint, error) {
			return goal.Endpoint{Remote: "origin", Branch: "refs/heads/main"}, nil
		},
		claimCheck: func(string, string, goal.Endpoint) func() error { return func() error { return nil } },
	}
	owners.work = intentWorkOwners{
		units: func(stateroot.Layout) *launch.UnitRunner {
			return &launch.UnitRunner{Manager: b.manager, Git: workGit{b}, Root: b.unitRoot}
		},
		git: func(dir string, args ...string) ([]byte, error) {
			joined := strings.Join(args, " ")
			switch {
			case joined == "worktree list --porcelain":
				listing := "worktree " + b.root() + "\nHEAD main-commit\nbranch refs/heads/main\n"
				if b.branchListed {
					listing += "\nworktree " + b.worktree + "\nHEAD " + b.head + "\nbranch refs/heads/goal/" + b.id + "\n"
				}
				return []byte(listing), nil
			case joined == "rev-parse HEAD" && dir == b.worktree:
				return []byte(b.head + "\n"), nil
			case strings.HasPrefix(joined, "rev-parse --verify -q refs/heads/goal/"):
				return nil, errors.New("no such branch")
			}
			return nil, errors.New("unexpected git " + joined)
		},
	}
	return owners
}

// work runs one public command with --json placed before any --check, so
// the check's argument vector stays the caller's own.
func (b *workBed) work(args ...string) (int, intentResult, string) {
	b.t.Helper()
	command, ok := findIntentCommand(args[0])
	if !ok {
		b.t.Fatalf("no public command %q", args[0])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, args[1:]...), &stdout, &stderr, b.root(), b.workOwners())
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
	}
	if result.SchemaVersion != 1 || result.Verb != args[0] || result.Outcome == "" || result.Summary == "" {
		b.t.Fatalf("%v envelope incomplete: %+v", args, result)
	}
	b.recordReadDirs(result)
	return code, result, stderr.String()
}

// recordReadDirs notes the read-findings directory named by the result's
// retained plan, when it has one.
func (b *workBed) recordReadDirs(result intentResult) {
	data, _ := result.Data.(map[string]any)
	path, _ := data["plan"].(string)
	if path == "" {
		return
	}
	plan, err := launch.ReadUnitPlan(path)
	if err != nil {
		return
	}
	b.readDirsMu.Lock()
	defer b.readDirsMu.Unlock()
	for _, output := range plan.Read.Outputs {
		if dir := filepath.Dir(output); strings.HasPrefix(filepath.Base(dir), "metasystem-unit-read-") {
			b.readDirs[dir] = true
		}
	}
}

func (b *workBed) removeReadDirs() {
	b.readDirsMu.Lock()
	defer b.readDirsMu.Unlock()
	for dir := range b.readDirs {
		os.RemoveAll(dir)
	}
}

func (b *workBed) brief(name, text string) string {
	b.t.Helper()
	path := filepath.Join(b.root(), name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		b.t.Fatal(err)
	}
	return name
}

func resultData(t *testing.T, result intentResult) map[string]any {
	t.Helper()
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result has no data object: %+v", result)
	}
	return data
}

func (b *workBed) runDirectories() []string {
	entries, _ := os.ReadDir(b.unitRoot)
	var runs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			runs = append(runs, entry.Name())
		}
	}
	return runs
}

var (
	workArgv  = []string{"go", "test", "-count=1", "-run", "TestA|TestB", "./..."}
	workCheck = append([]string{"--read-tool-calls", "12", "--check"}, workArgv...)
)

// workApprovedBox gives the fixture goal an approved box with two review
// rounds, the limit a public build reads.
func workApprovedBox(file *goal.GoalFile) {
	if file.Budget == nil {
		file.Budget = &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1}
	}
	file.Budget.ReviewRoundLimit = 2
	if file.Approved != nil {
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	}
}

func TestIntentBuildConcurrentRepeat(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n\n| Unit | Lines |\n| --- | --- |\n| u1 | 40 |\n")
	args := append([]string{"build", bed.id, "u1", "--brief", brief}, workCheck...)
	var wait sync.WaitGroup
	results := make([]intentResult, 4)
	codes := make([]int, 4)
	for index := range results {
		wait.Add(1)
		go func() {
			defer wait.Done()
			codes[index], results[index], _ = bed.work(args...)
		}()
	}
	wait.Wait()
	runs := map[string]bool{}
	for index, result := range results {
		switch result.Outcome {
		case intentConfirmed:
			runs[resultData(t, result)["run"].(string)] = true
		case intentInProgress:
			if codes[index] != 3 || !strings.HasPrefix(result.Summary, "UNIT_RUN_BUSY") || result.Next == nil || !slices.Equal(result.Next.Argv, append([]string{"metasystem", "build", "--json"}, args[1:]...)) {
				t.Fatalf("busy caller: code=%d %+v", codes[index], result)
			}
		default:
			t.Fatalf("concurrent build %d: code=%d %+v", index, codes[index], result)
		}
	}
	code, repeat, _ := bed.work(args...)
	if code != 0 || repeat.Outcome != intentConfirmed {
		t.Fatalf("repeat: code=%d %+v", code, repeat)
	}
	runs[resultData(t, repeat)["run"].(string)] = true
	if len(runs) != 1 || len(bed.runDirectories()) != 1 {
		t.Fatalf("runs=%v directories=%v", runs, bed.runDirectories())
	}
	if launched := bed.starter.launched(); !slices.Equal(launched, []string{"build", "proof", "read"}) {
		t.Fatalf("launches=%v, want one build, proof and read", launched)
	}
}

func TestIntentBuildSizeInput(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	plain := bed.brief("plain.md", "Build the unit.\n")
	code, result, _ := bed.work(append([]string{"build", bed.id, "unsized", "--brief", plain}, workCheck...)...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "LAUNCH_BUILD_UNSIZED unit=unsized") || !strings.Contains(result.Decision, "--lines N") {
		t.Fatalf("absent estimate: code=%d %+v", code, result)
	}
	if len(bed.starter.launched()) != 0 || len(bed.runDirectories()) != 0 {
		t.Fatal("an unsized build launched or recorded a run")
	}
	code, result, _ = bed.work(append([]string{"build", bed.id, "estimated", "--brief", plain, "--lines", "120"}, workCheck...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("--lines: code=%d %+v", code, result)
	}
	data := resultData(t, result)
	if plan, _ := launch.ReadUnitPlan(data["plan"].(string)); plan.Build.UnitsPage != plan.Build.Brief {
		t.Fatalf("the estimate is not read from the generated build brief: %v", data)
	}
	buildLaunch := data["steps"].([]any)[0].(map[string]any)["launchId"].(string)
	if record, err := bed.manager.Store.Read(buildLaunch); err != nil || record.DeclaredLines != 120 {
		t.Fatalf("build admission size: %+v %v", record.DeclaredLines, err)
	}
	rowed := bed.brief("rowed.md", "Build it.\n\n| Unit | Lines |\n| --- | --- |\n| rowed | 75 |\n")
	code, result, _ = bed.work(append([]string{"build", bed.id, "rowed", "--brief", rowed}, workCheck...)...)
	if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(string(must(os.ReadFile(filepath.Join(resultData(t, result)["inputs"].(string), "build-brief.md")))), "| rowed | 75 |") {
		t.Fatalf("units row: code=%d %+v", code, result)
	}
	buildLaunch = resultData(t, result)["steps"].([]any)[0].(map[string]any)["launchId"].(string)
	if record, err := bed.manager.Store.Read(buildLaunch); err != nil || record.DeclaredLines != 75 {
		t.Fatalf("row size: %+v %v", record.DeclaredLines, err)
	}
	launches := len(bed.starter.launched())
	code, result, _ = bed.work(append([]string{"build", bed.id, "rowed", "--brief", rowed, "--lines", "80"}, workCheck...)...)
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "75") || !strings.Contains(result.Summary, "80") || len(bed.starter.launched()) != launches {
		t.Fatalf("conflicting estimate: code=%d %+v", code, result)
	}
}

func TestIntentGeneratedUnitPlan(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n")
	args := append([]string{"build", bed.id, "planned", "--brief", brief, "--lines", "30"}, workCheck...)
	code, result, _ := bed.work(args...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("code=%d %+v", code, result)
	}
	data := resultData(t, result)
	planPath := data["plan"].(string)
	plan, err := launch.ReadUnitPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Unit != "planned" || plan.Goal != bed.id || plan.Worktree != bed.worktree || plan.Base != bed.head ||
		!slices.Equal(plan.Build.Units, []string{"planned"}) || plan.Read.Model != "fixture-read-model" {
		t.Fatalf("plan=%+v", plan)
	}
	if len(plan.Proof) != 1 || !slices.Equal(plan.Proof[0].Argv, workArgv) || plan.Proof[0].Dir != bed.worktree {
		t.Fatalf("proof=%+v, want the exact argv %v", plan.Proof, workArgv)
	}
	pack := &launch.Manager{TemplateDirectory: bed.manager.TemplateDirectory}
	if _, err := pack.CheckPack(launch.StartSpec{Kind: "read", Brief: plan.Read.Brief, WorkingDirectory: bed.worktree}); err != nil {
		t.Fatalf("read brief: %v", err)
	}
	readBrief, _ := os.ReadFile(plan.Read.Brief)
	buildBrief, _ := os.ReadFile(plan.Build.Brief)
	for _, want := range []string{"unit planned of goal " + bed.id, bed.worktree, bed.head, "VERDICT: land"} {
		if !strings.Contains(string(readBrief), want) {
			t.Fatalf("read brief lacks %q:\n%s", want, readBrief)
		}
	}
	if !strings.Contains(string(buildBrief), `["go","test","-count=1","-run","TestA|TestB","./..."]`) || !strings.HasSuffix(string(buildBrief), "Build the unit.\n") {
		t.Fatalf("build brief:\n%s", buildBrief)
	}
	before := map[string][]byte{}
	for _, path := range []string{planPath, plan.Build.Brief, plan.Read.Brief} {
		before[path], _ = os.ReadFile(path)
	}
	if code, repeat, _ := bed.work(args...); code != 0 || resultData(t, repeat)["run"] != data["run"] || resultData(t, repeat)["plan"] != planPath {
		t.Fatalf("repeat: code=%d %+v", code, repeat)
	}
	for path, bytesBefore := range before {
		if after, _ := os.ReadFile(path); !bytes.Equal(after, bytesBefore) {
			t.Fatalf("%s changed on repeat", path)
		}
	}
}

// TestIntentBuildResume drives one unit from a red proof through a retry, a
// resume, a refused changed input, a follow-up round, a capped wait and its
// continuation, and checks it only ever ends awaiting judgement.
func TestIntentBuildResume(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n")
	args := append([]string{"build", bed.id, "journey", "--brief", brief, "--lines", "20"}, workCheck...)
	bed.starter.fail["proof"] = true
	code, result, _ := bed.work(args...)
	data := resultData(t, result)
	run := data["run"].(string)
	if code != 0 || result.Outcome != intentConfirmed || data["state"] != "awaiting-judgement" || data["outcome"] != "proof-red" {
		t.Fatalf("red proof: code=%d %+v", code, result)
	}
	if launched := bed.starter.launched(); !slices.Equal(launched, []string{"build", "proof"}) {
		t.Fatalf("a red proof still read: %v", launched)
	}
	for _, again := range [][]string{args, {"build", "--resume", run}, {"wait", "unit", run}} {
		code, repeat, _ := bed.work(again...)
		if code != 0 || resultData(t, repeat)["run"] != run || resultData(t, repeat)["outcome"] != "proof-red" || len(bed.starter.launched()) != 2 {
			t.Fatalf("%v: code=%d %+v launches=%v", again, code, repeat, bed.starter.launched())
		}
	}
	bed.brief("brief.md", "Build the unit, changed.\n")
	code, result, _ = bed.work(args...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(result.Summary, "run="+run) || len(bed.starter.launched()) != 2 {
		t.Fatalf("changed input: code=%d %+v", code, result)
	}
	delete(bed.starter.fail, "proof")
	followUp := bed.brief("follow-up.md", "Fix the red proof.\n")
	code, result, _ = bed.work("fold", "unit", run, "--brief", followUp)
	data = resultData(t, result)
	if code != 0 || result.Outcome != intentConfirmed || data["run"] != run || data["round"].(float64) != 2 || data["outcome"] != "green" || data["state"] != "awaiting-judgement" {
		t.Fatalf("follow-up: code=%d %+v", code, result)
	}
	if launched := bed.starter.launched(); !slices.Equal(launched, []string{"build", "proof", "build", "proof", "read"}) {
		t.Fatalf("follow-up launches=%v", launched)
	}

	bed.starter.hold = "build"
	held := append([]string{"build", bed.id, "held", "--brief", brief, "--lines", "20"}, workCheck...)
	code, result, _ = bed.work(held...)
	heldRun := resultData(t, result)["run"].(string)
	if code != 3 || result.Outcome != intentInProgress || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "goal", bed.id, "--work", "held"}) || heldRun == "" {
		t.Fatalf("capped build: code=%d %+v", code, result)
	}
	bed.starter.hold = ""
	buildLaunch := resultData(t, result)["steps"].([]any)[0].(map[string]any)["launchId"].(string)
	bed.manager.Store.Update(buildLaunch, func(record *launch.Record) error {
		exit := 0
		record.State, record.ExitCode = launch.Completed, &exit
		return nil
	})
	code, result, _ = bed.work("wait", "unit", heldRun)
	if code != 0 || result.Outcome != intentConfirmed || resultData(t, result)["outcome"] != "green" {
		t.Fatalf("continued wait: code=%d %+v", code, result)
	}
	if launched := bed.starter.launched(); len(launched) != 8 || launched[5] != "build" || launched[6] != "proof" || launched[7] != "read" {
		t.Fatalf("the continued wait started another build: %v", launched)
	}
}

func TestIntentBuildRefusals(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n")
	bed.branchListed = false
	code, result, _ := bed.work(append([]string{"build", bed.id, "u", "--brief", brief, "--lines", "5"}, workCheck...)...)
	layout, _ := bed.owners().resolver.ResolveLayout(bed.root())
	target := filepath.Join(filepath.Dir(layout.GitRoot), filepath.Base(layout.GitRoot)+"-"+bed.id)
	// Without a goal worktree, build prepares one only under a verified
	// claim; this fixture has no goal-branch endpoint, so it refuses before
	// any Git effect and prints no manual preparation recipe.
	if _, statErr := os.Stat(target); code != 1 || result.Outcome != intentRefused || result.Next != nil ||
		!strings.Contains(result.Summary, "nothing was built") || !os.IsNotExist(statErr) {
		t.Fatalf("missing worktree: code=%d %+v next=%v", code, result, result.Next)
	}
	bed.branchListed = true
	missing := bed.brief("missing.md", "Build.\n\nMISSING DECISION: the acceptance criteria\n")
	code, result, _ = bed.work(append([]string{"build", bed.id, "u", "--brief", missing, "--lines", "5"}, workCheck...)...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "1 missing decision") {
		t.Fatalf("missing decision: code=%d %+v", code, result)
	}
	code, result, _ = bed.work("build", bed.id, "u", "--brief", brief, "--lines", "5", "--check")
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "--check needs a command") {
		t.Fatalf("empty check: code=%d %+v", code, result)
	}
	code, result, _ = bed.work("build", "--resume", "run-1", "--brief", brief)
	if code != 2 || result.Outcome != intentRefused {
		t.Fatalf("resume with brief: code=%d %+v", code, result)
	}
	if len(bed.starter.launched()) != 0 || len(bed.runDirectories()) != 0 {
		t.Fatal("a refused build launched")
	}
	input, problem := parseIntentArgs(mustIntentCommand(t, "build"), []string{"g", "u", "--brief", "b", "--check", "sh", "-c", "--brief x", "--", "--json"})
	if problem != nil || !slices.Equal(input.values["check"], []string{"sh", "-c", "--brief x", "--", "--json"}) || input.text("brief") != "b" || input.switched("json") {
		t.Fatalf("--check did not end the options: %+v %+v", input, problem)
	}
}

func mustIntentCommand(t *testing.T, name string) intentCommand {
	t.Helper()
	command, ok := findIntentCommand(name)
	if !ok {
		t.Fatalf("no public command %q", name)
	}
	return command
}

func TestIntentBriefScaffold(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	code, result, _ := bed.work("brief", bed.id, "--out", "brief.md")
	out := filepath.Join(bed.root(), "brief.md")
	written, _ := os.ReadFile(out)
	if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(string(written), bed.worktree) || !strings.Contains(string(written), "MISSING DECISION: observable") {
		t.Fatalf("code=%d %+v\n%s", code, result, written)
	}
	if code, again, _ := bed.work("brief", bed.id, "--out", "brief.md"); code != 0 || again.Outcome != intentUnchanged {
		t.Fatalf("repeat: code=%d %+v", code, again)
	}
	os.WriteFile(out, []byte("edited\n"), 0o600)
	if code, again, _ := bed.work("brief", bed.id, "--out", "brief.md"); code != 1 || again.Outcome != intentRefused {
		t.Fatalf("overwrite: code=%d %+v", code, again)
	}
	if kept, _ := os.ReadFile(out); string(kept) != "edited\n" {
		t.Fatal("brief overwrote an edited file")
	}
	os.WriteFile(out, written, 0o600)
	code, result, _ = bed.work(append([]string{"build", bed.id, "u", "--brief", "brief.md", "--lines", "5"}, workCheck...)...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "missing decision") {
		t.Fatalf("build from an unfilled scaffold: code=%d %+v", code, result)
	}
}

func TestIntentWaitTestAndSettingsAdapters(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	var waitArgs []string
	waitResult := metarun.WaitResult{SchemaVersion: 2, WaitID: "wait-1", ExitCode: metarun.ExitWaitDeadline, SourceOutcome: "wait-deadline", Reason: "still running"}
	owners.work.wait = func(args []string, print func(metarun.WaitResult, bool)) int {
		waitArgs = args
		print(waitResult, true)
		return waitResult.ExitCode
	}
	var parsed testingSelectionRequest
	owners.work.subprocess = func(dir string, argv []string, stderr io.Writer) ([]byte, int, error) {
		// The real test-run parser reads what the public command passes.
		request, _, code := parseTestingSelection(argv[0]+" "+argv[1], argv[2:], true)
		parsed = request
		if code != 0 {
			return nil, code, nil
		}
		return []byte(`{"outcome":"red"}`), 1, nil
	}
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	code, result := run("wait", "job", "job-1", "--timeout", "5m")
	layout, _ := owners.resolver.ResolveLayout(bed.root())
	if code != metarun.ExitWaitDeadline || result.Outcome != intentInProgress || result.Next == nil ||
		!slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "resume", "wait-1"}) || layout.InstallationRoot == "" ||
		!slices.Equal(waitArgs, []string{"--root", layout.InstallationRoot, "--job", "job-1", "--timeout", "5m0s"}) {
		t.Fatalf("wait deadline: code=%d %+v args=%v", code, result, waitArgs)
	}
	// The printed continuation is itself a public command that resumes the
	// same recorded wait through the wait owner.
	code, result = run("wait", "resume", "wait-1")
	if code != metarun.ExitWaitDeadline || result.Outcome != intentInProgress || !slices.Equal(waitArgs, []string{"--root", layout.InstallationRoot, "--resume", "wait-1"}) ||
		result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "resume", "wait-1"}) {
		t.Fatalf("wait resume: code=%d %+v args=%v", code, result, waitArgs)
	}
	waitResult = metarun.WaitResult{SchemaVersion: 2, WaitID: "wait-2", ExitCode: metarun.ExitGreen, SourceOutcome: "green"}
	// The goal event needs --for (or its --event alias); bare wait goal G is
	// the goal's running work.
	if code, result := run("wait", "goal", bed.id, "--for", "landing"); code != 0 || result.Outcome != intentConfirmed || len(waitArgs) != 8 || waitArgs[2] != "--goal" ||
		waitArgs[4] != "--event" || waitArgs[5] != "landing" || waitArgs[6] != "--after" || waitArgs[7] == "" {
		t.Fatalf("wait goal: code=%d %+v args=%v", code, result, waitArgs)
	}
	selector := metarun.WaitSelector{Kind: "goal", TargetID: bed.id, GoalID: bed.id, Event: waitArgs[5], After: waitArgs[7]}
	if err := metarun.ValidateWaitSelector(selector); err != nil {
		t.Fatalf("the default goal wait fails the wait owner's validation: %v", err)
	}
	waitArgs = nil
	if code, result := run("wait", "goal", bed.id, "--event", "landing", "--question", "q1"); code != metarun.ExitInvalidWait || result.Outcome != intentRefused || waitArgs != nil ||
		!strings.Contains(result.Summary, "a question selector applies only to an answer wait") {
		t.Fatalf("invalid goal selector: code=%d %+v args=%v", code, result, waitArgs)
	}
	if code, result := run("wait", "goal", bed.id, "--event", "human-act", "--verb", "answer", "--question", "q1", "--after", "abc"); code != 0 || result.Outcome != intentConfirmed ||
		!slices.Equal(waitArgs[4:], []string{"--event", "human-act", "--after", "abc", "--verb", "answer", "--question", "q1"}) {
		t.Fatalf("advanced goal selectors: code=%d %+v args=%v", code, result, waitArgs)
	}
	if code, result := run("wait", "job", "job-1", "--event", "landing"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("goal selector on a job wait: code=%d %+v", code, result)
	}
	owners.work.wait = func([]string, func(metarun.WaitResult, bool)) int { return metarun.ExitInvalidWait }
	if code, result := run("wait", "job", "job-1"); code != metarun.ExitInvalidWait || result.Outcome != intentFailed {
		t.Fatalf("wait without a result: code=%d %+v", code, result)
	}
	code, result = run("test", "--goal", bed.id, "--authority", "claimed-goal", "--mode", "standard")
	if code != 1 || result.Outcome != intentFailed || parsed.Root != layout.InstallationRoot || parsed.GoalID != bed.id || parsed.AuthorityGoalID != "claimed-goal" {
		t.Fatalf("test: code=%d %+v parsed=%+v", code, result, parsed)
	}
	if encoded, _ := json.Marshal(result.Data); string(encoded) != `{"outcome":"red"}` {
		t.Fatalf("test data=%s", encoded)
	}
	families := families()
	for _, row := range []struct {
		args   []string
		legacy bool
	}{
		{[]string{"wait", "--job", "j"}, true}, {[]string{"wait", "register", "--pid", "1"}, true}, {[]string{"wait"}, false},
		{[]string{"wait", "job", "j"}, false}, {[]string{"wait", "--help"}, false},
		{[]string{"test", "plan"}, true}, {[]string{"test", "verify"}, true}, {[]string{"test", "--goal", "g"}, false},
		{[]string{"build", "g", "u"}, false}, {[]string{"settings"}, false},
	} {
		if got := intentYieldsToLegacy(row.args, families); got != row.legacy {
			t.Fatalf("%v legacy=%t, want %t", row.args, got, row.legacy)
		}
	}
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
