package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func retainedInputs(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, name := range []string{"request.json", "plan.json", "build-brief.md", "read-brief.md"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatalf("retained %s: %v", name, err)
		}
		files[name] = data
	}
	return files
}

// TestIntentBuildRetainedRequest: a run keeps the inputs and base it started
// with. A repeat after the goal branch moves reaches that run unchanged, and
// a different request, sequential or concurrent, is refused with the run id
// and rewrites none of the retained bytes.
func TestIntentBuildRetainedRequest(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	first := bed.brief("first.md", "Build the first way.\n")
	second := bed.brief("second.md", "Build the second way.\n")
	argsFor := func(brief string) []string {
		return append([]string{"build", bed.id, "shared", "--brief", brief, "--lines", "10"}, workCheck...)
	}
	var wait sync.WaitGroup
	results := make([]intentResult, 2)
	for index, brief := range []string{first, second} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, results[index], _ = bed.work(argsFor(brief)...)
		}()
	}
	wait.Wait()
	winner := -1
	for index, result := range results {
		switch {
		case result.Outcome == intentConfirmed:
			if winner >= 0 {
				t.Fatalf("both competing requests ran: %+v", results)
			}
			winner = index
		case strings.HasPrefix(result.Summary, "UNIT_RUN_BUSY") || strings.HasPrefix(result.Summary, "UNIT_NAMED_INPUT_CHANGED"):
		default:
			t.Fatalf("competing request %d: %+v", index, result)
		}
	}
	if winner < 0 {
		t.Fatalf("neither competing request ran: %+v", results)
	}
	data := resultData(t, results[winner])
	run, directory := data["run"].(string), data["inputs"].(string)
	winnerBrief, loserBrief := []string{first, second}[winner], []string{first, second}[1-winner]
	buildBrief, _ := os.ReadFile(filepath.Join(directory, "build-brief.md"))
	if !strings.HasSuffix(string(buildBrief), map[string]string{first: "Build the first way.\n", second: "Build the second way.\n"}[winnerBrief]) {
		t.Fatalf("the run's build brief is not its own request's:\n%s", buildBrief)
	}
	retained := retainedInputs(t, directory)
	launched := len(bed.starter.launched())

	code, refused, _ := bed.work(argsFor(loserBrief)...)
	if code != 1 || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(refused.Summary, "run="+run) {
		t.Fatalf("different request: code=%d %+v", code, refused)
	}
	bed.head = "moved-commit"
	code, repeat, _ := bed.work(argsFor(winnerBrief)...)
	if code != 0 || repeat.Outcome != intentConfirmed || resultData(t, repeat)["run"] != run || resultData(t, repeat)["base"] != "base-commit" {
		t.Fatalf("repeat after the branch moved: code=%d %+v", code, repeat)
	}
	for name, before := range retained {
		if after := retainedInputs(t, directory)[name]; !bytes.Equal(after, before) {
			t.Fatalf("retained %s changed", name)
		}
	}
	if len(bed.starter.launched()) != launched {
		t.Fatalf("a refused or repeated request launched: %v", bed.starter.launched())
	}
}

// verdictAdapter stands in for a reader: while its child runs it writes the
// findings file the read declares, and it measures the verdict from that
// file, as the model adapters do.
type verdictAdapter struct{ findings *string }

func (a verdictAdapter) Command(record launch.Record, state string) (launch.Command, error) {
	var declared []string
	_ = json.Unmarshal(record.AdapterData["declaredOutputs"], &declared)
	if *a.findings != "" && len(declared) > 0 {
		if err := os.WriteFile(declared[0], []byte(*a.findings), 0o600); err != nil {
			return launch.Command{}, err
		}
	}
	return launch.Command{LogPath: filepath.Join(state, "log")}, nil
}

func (a verdictAdapter) Measure(record launch.Record, _ string) (launch.Measurement, []launch.Output, map[string]json.RawMessage, error) {
	var declared []string
	_ = json.Unmarshal(record.AdapterData["declaredOutputs"], &declared)
	measurement := launch.Measurement{Verdict: "none"}
	for _, path := range declared {
		data, _ := os.ReadFile(path)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VERDICT:") {
				measurement.Verdict = strings.TrimSpace(line)
			}
		}
	}
	return measurement, nil, nil, nil
}
func (verdictAdapter) Strays() ([]string, error) { return nil, nil }

type exitedChild struct{}

func (exitedChild) Wait() (int, error) { return 0, nil }

// readProcesses runs the read's child to exit 0 and proves its group gone.
type readProcesses struct{}

func (readProcesses) SelfRef() (identity.Ref, error) { return workRef(10), nil }
func (readProcesses) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	return exitedChild{}, workRef(30), nil
}
func (readProcesses) SignalGroup(int64, syscall.Signal) error { return nil }
func (readProcesses) GroupAlive(int64) (bool, error)          { return false, nil }

// superviseReads runs every read launch through the launch owner's own
// supervision: declared outputs, adapter measure and output copies.
type superviseReads struct{ *workStarter }

func (s superviseReads) StartSupervisor(id, stateDir string) (identity.Ref, error) {
	record, _ := s.m.Store.Read(id)
	if record.Kind != "read" {
		return s.workStarter.StartSupervisor(id, stateDir)
	}
	s.mu.Lock()
	s.kinds = append(s.kinds, record.Kind)
	s.mu.Unlock()
	if _, err := s.m.Supervise(id); err != nil {
		return identity.Ref{}, err
	}
	return workRef(10), nil
}

// TestIntentReadVerdictFromRetainedFindings: the read's findings are a
// declared output, copied into each read launch and measured for its
// verdict. A fix-first verdict is reported as such, never a clean read; a
// reader that writes no findings fails the read.
func TestIntentReadVerdictFromRetainedFindings(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	findings := "1. witness.txt: wrong bytes\nVERDICT: fix first (1 material findings)\n"
	adapter := verdictAdapter{findings: &findings}
	for _, name := range []string{"codex-exec", "claude-headless"} {
		bed.manager.Adapters[name] = adapter
	}
	bed.manager.Processes = readProcesses{}
	bed.manager.Supervisor = superviseReads{bed.starter}
	brief := bed.brief("brief.md", "Build the unit.\n")
	code, result, _ := bed.work(append([]string{"build", bed.id, "read", "--brief", brief, "--lines", "5"}, workCheck...)...)
	data := resultData(t, result)
	if code != 0 || data["outcome"] != "green" || data["readClean"] != false || !strings.Contains(result.Summary, "read verdict: fix first (1 material findings)") {
		t.Fatalf("fix-first read: code=%d %+v", code, result)
	}
	copies := data["readFindings"].([]any)
	if len(copies) != 1 {
		t.Fatalf("read findings copies=%v", copies)
	}
	firstCopy := copies[0].(string)
	if retained, _ := os.ReadFile(firstCopy); string(retained) != findings {
		t.Fatalf("retained findings=%q", retained)
	}
	plan, _ := launch.ReadUnitPlan(data["plan"].(string))
	if len(plan.Read.Outputs) != 1 {
		t.Fatalf("read outputs=%v", plan.Read.Outputs)
	}
	findings = "No material findings.\nVERDICT: land\n"
	run := data["run"].(string)
	code, result, _ = bed.work("fold", "unit", run, "--brief", bed.brief("follow-up.md", "Fix the witness.\n"))
	data = resultData(t, result)
	if code != 0 || data["round"].(float64) != 2 || data["readClean"] != true || !strings.Contains(result.Summary, "read verdict: land") {
		t.Fatalf("clean read after the fold: code=%d %+v", code, result)
	}
	if retained, _ := os.ReadFile(firstCopy); !strings.Contains(string(retained), "fix first") {
		t.Fatalf("round one's findings copy was replaced: %q", retained)
	}
	findings = ""
	code, result, _ = bed.work(append([]string{"build", bed.id, "silent", "--brief", brief, "--lines", "5"}, workCheck...)...)
	data = resultData(t, result)
	if code != 0 || data["outcome"] != "read-failed" || data["readClean"] != false {
		t.Fatalf("reader without findings: code=%d %+v", code, result)
	}
}

// TestIntentBuildRoundLimitAndReadBudget: the read's rounds come from the
// goal's approved box and its tool calls from the brief or an explicit
// option; neither is assumed, and the unit runner refuses a round past the
// limit the run started with.
func TestIntentBuildRoundLimitAndReadBudget(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	plain := bed.brief("plain.md", "Build the unit.\n")
	check := append([]string{"--check"}, workArgv...)
	code, result, _ := bed.work(append([]string{"build", bed.id, "budget", "--brief", plain, "--lines", "5"}, check...)...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "Maximum reader tool calls: N") {
		t.Fatalf("undecided read budget: code=%d %+v", code, result)
	}
	budgeted := bed.brief("budgeted.md", "Build the unit.\n\nMaximum reader tool calls: 25\n")
	code, result, _ = bed.work(append([]string{"build", bed.id, "budget", "--brief", budgeted, "--lines", "5", "--read-tool-calls", "30"}, check...)...)
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "25") {
		t.Fatalf("conflicting read budget: code=%d %+v", code, result)
	}
	code, result, _ = bed.work(append([]string{"build", bed.id, "budget", "--brief", budgeted, "--lines", "5"}, check...)...)
	data := resultData(t, result)
	if code != 0 || data["maxRounds"].(float64) != 2 {
		t.Fatalf("budgeted build: code=%d %+v", code, result)
	}
	plan, _ := launch.ReadUnitPlan(data["plan"].(string))
	readBrief, _ := os.ReadFile(plan.Read.Brief)
	if !strings.Contains(string(readBrief), "Round budget: 2 focused rounds, the goal's approved review-round limit") || !strings.Contains(string(readBrief), "Maximum reader tool calls: 25") {
		t.Fatalf("read brief budgets:\n%s", readBrief)
	}
	run := data["run"].(string)
	followUp := bed.brief("follow-up.md", "Again.\n")
	if code, result, _ = bed.work("fold", "unit", run, "--brief", followUp); code != 0 || resultData(t, result)["round"].(float64) != 2 {
		t.Fatalf("second round: code=%d %+v", code, result)
	}
	launched := len(bed.starter.launched())
	code, result, _ = bed.work("fold", "unit", run, "--brief", followUp)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "UNIT_ROUND_LIMIT") || len(bed.starter.launched()) != launched {
		t.Fatalf("third round: code=%d %+v", code, result)
	}

	unapproved := newWorkBed(t)
	unapproved.intentBed = newIntentBed(t, false, func(file *goal.GoalFile) { file.Budget, file.Approved = nil, nil })
	code, result, _ = unapproved.work(append([]string{"build", unapproved.id, "u", "--brief", unapproved.brief("b.md", "B.\n"), "--lines", "5"}, workCheck...)...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "no approved box") || !strings.Contains(result.Decision, "metasystem approve") {
		t.Fatalf("unapproved goal: code=%d %+v", code, result)
	}
}

// TestIntentBuildModelOverride: --model and --effort are the unit's own,
// recorded in the run, used by its build launches, kept on resume and part
// of the request's identity.
func TestIntentBuildModelOverride(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n")
	args := func(model string) []string {
		return append([]string{"build", bed.id, "modelled", "--brief", brief, "--lines", "5", "--model", model, "--effort", "high"}, workCheck...)
	}
	code, result, _ := bed.work(args("claude-sonnet-5")...)
	data := resultData(t, result)
	if code != 0 || data["buildModel"] != "claude-sonnet-5" || data["buildEffort"] != "high" {
		t.Fatalf("override: code=%d %+v", code, result)
	}
	buildLaunch := data["steps"].([]any)[0].(map[string]any)["launchId"].(string)
	record, err := bed.manager.Store.Read(buildLaunch)
	if err != nil || string(record.AdapterData["model"]) != `"claude-sonnet-5"` || string(record.AdapterData["effort"]) != `"high"` {
		t.Fatalf("build launch model=%s effort=%s err=%v", record.AdapterData["model"], record.AdapterData["effort"], err)
	}
	run := data["run"].(string)
	if code, result, _ = bed.work("build", "--resume", run); code != 0 || resultData(t, result)["buildModel"] != "claude-sonnet-5" {
		t.Fatalf("resume: code=%d %+v", code, result)
	}
	if code, result, _ = bed.work(args("claude-opus-5-5")...); code != 1 || !strings.Contains(result.Summary, "UNIT_NAMED_INPUT_CHANGED") {
		t.Fatalf("another model for the same unit: code=%d %+v", code, result)
	}
	if code, result, _ = bed.work(append([]string{"build", bed.id, "bad", "--brief", brief, "--lines", "5", "--effort", "extreme"}, workCheck...)...); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("unknown effort: code=%d %+v", code, result)
	}
}

func (b *workBed) stateRoot() string {
	b.t.Helper()
	resolver := b.owners().resolver
	layout, err := resolver.ResolveLayout(b.root())
	if err != nil {
		b.t.Fatal(err)
	}
	root, err := resolver.RootForInstallation(layout.InstallationRoot)
	if err != nil {
		b.t.Fatal(err)
	}
	return root
}

// TestIntentBriefCarriesAcceptedDesign: the scaffold carries the accepted
// design's units, constraints, return and acceptance text and marks only the
// decision neither record holds; a project that cannot be read is refused.
func TestIntentBriefCarriesAcceptedDesign(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	designs := filepath.Join(bed.stateRoot(), "plans", "designs")
	os.MkdirAll(designs, 0o700)
	design := "# Standing validation\n\n- Kind: design\n- Id: 01M3CGR7CNZTS2NNTQCYRCF4JZ\n- Status: accepted\n- Goals: standing-validation\n\n" +
		"## Non-goals\n\nNo new ledger schema.\n\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| u1 | 40 |\n\n" +
		"## Return\n\nThe diff and the proof log.\n\n## Acceptance\n\nThe validator refuses a stale box.\n"
	if err := os.WriteFile(filepath.Join(designs, "standing-validation.md"), []byte(design), 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _ := bed.work("brief", bed.id, "--out", "brief.md")
	written, _ := os.ReadFile(filepath.Join(bed.root(), "brief.md"))
	if code != 0 {
		t.Fatalf("brief: code=%d %+v", code, result)
	}
	for _, want := range []string{"No new ledger schema.", "| u1 | 40 |", "The diff and the proof log.", "The validator refuses a stale box.", "(accepted; the specification this brief builds)"} {
		if !strings.Contains(string(written), want) {
			t.Fatalf("scaffold lacks %q:\n%s", want, written)
		}
	}
	missing, _ := resultData(t, result)["missingDecisions"].([]any)
	if code != 0 || len(missing) != 1 || !strings.Contains(missing[0].(string), "tool-call budget") {
		t.Fatalf("only the read budget is undecided: code=%d missing=%v\n%s", code, missing, written)
	}
	filled := strings.Replace(string(written), intentMissingDecision+" the read's tool-call budget, written as the line 'Maximum reader tool calls: N'", "Maximum reader tool calls: 20", 1)
	os.WriteFile(filepath.Join(bed.root(), "filled.md"), []byte(filled), 0o600)
	code, result, _ = bed.work(append([]string{"build", bed.id, "u1", "--brief", "filled.md", "--check"}, workArgv...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build from the filled scaffold: code=%d %+v", code, result)
	}
	plan, _ := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if len(plan.Build.Inputs) != 1 || !strings.HasSuffix(plan.Build.Inputs[0], "standing-validation.md") || !slices.Equal(plan.Read.Inputs, plan.Build.Inputs) {
		t.Fatalf("the build does not bind the accepted design: %+v", plan.Build)
	}

	broken := newWorkBed(t)
	blocked := filepath.Join(broken.stateRoot(), "plans", "designs")
	os.MkdirAll(filepath.Dir(blocked), 0o700)
	os.WriteFile(blocked, []byte("not a directory\n"), 0o600)
	for _, args := range [][]string{
		{"brief", broken.id, "--out", "b.md"},
		append([]string{"build", broken.id, "u", "--brief", broken.brief("x.md", "X.\n"), "--lines", "5"}, workCheck...),
	} {
		code, result, _ := broken.work(args...)
		if code != 1 || result.Outcome != intentFailed || !strings.Contains(result.Summary, "cannot read the project's design records") {
			t.Fatalf("%v with an unreadable project: code=%d %+v", args[0], code, result)
		}
	}
}

// TestIntentSettingsSelectedInstallation: settings reads the installation
// --repo selects, launch keys and other configuration keys alike, with the
// source of each value.
func TestIntentSettingsSelectedInstallation(t *testing.T) {
	t.Parallel()
	roots := []string{t.TempDir(), t.TempDir()}
	for index, root := range roots {
		os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o700)
		conf := "launch.read.model=model-" + string(rune('a'+index)) + "\ncontext.ceiling.tokens=" + []string{"100000", "200000"}[index] + "\n"
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	top := func(path string) (string, error) {
		for _, root := range roots {
			if resolved, _ := filepath.EvalSymlinks(root); withinPath(path, root) || withinPath(path, resolved) {
				return root, nil
			}
		}
		return fakeTop(roots[0])(path)
	}
	owners := intentOwners{resolver: stateroot.NewResolver(top, noExecutable)}
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, "settings"), append(args, "--json"), &stdout, &stderr, roots[0], owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	value := func(result intentResult) launch.Setting {
		if result.Data == nil {
			t.Fatalf("no settings: %+v", result)
		}
		encoded, _ := json.Marshal(result.Data.(map[string]any)["settings"])
		var settings []launch.Setting
		json.Unmarshal(encoded, &settings)
		if len(settings) != 1 {
			t.Fatalf("settings=%s", encoded)
		}
		return settings[0]
	}
	for index, root := range roots {
		code, result := run(launch.ReadModelKey, "--repo", root)
		if got := value(result); code != 0 || got.Value != "model-"+string(rune('a'+index)) || got.Source != "conf" {
			t.Fatalf("%s launch key: code=%d %+v", root, code, result)
		}
		code, result = run("context.ceiling.tokens", "--repo", root)
		if got := value(result); code != 0 || got.Value != []string{"100000", "200000"}[index] || got.Source != "conf" {
			t.Fatalf("%s configuration key: code=%d %+v", root, code, result)
		}
	}
	if code, result := run("no.such.key"); code != 1 || result.Outcome != intentRefused {
		t.Fatalf("unknown key: code=%d %+v", code, result)
	}
	if code, result := run(); code != 0 || !slices.ContainsFunc(result.Data.(map[string]any)["settings"].([]any), func(item any) bool {
		return item.(map[string]any)["Key"] == launch.ReadModelKey && item.(map[string]any)["Value"] == "model-a"
	}) {
		t.Fatalf("all launch settings: code=%d %+v", code, result)
	}
}
