package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func retainedInputs(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	planPath := filepath.Join(directory, "plan.json")
	plan, err := launch.ReadUnitPlan(planPath)
	if err != nil {
		t.Fatalf("retained plan: %v", err)
	}
	paths := []string{filepath.Join(directory, "request.json"), planPath, plan.Build.Brief}
	if plan.HasRead() {
		paths = append(paths, plan.Read.Brief)
	}
	files := map[string][]byte{}
	for _, path := range paths {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
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
	first := bed.brief("first.md", "Read each round: yes\nBuild the first way.\n")
	second := bed.brief("second.md", "Read each round: yes\nBuild the second way.\n")
	argsFor := func(brief string) []string {
		return append([]string{"work", "build", bed.id, "shared", "--brief", brief, "--lines", "10"}, workCheck...)
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
		case strings.Contains(resultWords(result), "UNIT_RUN_BUSY") || strings.Contains(resultWords(result), "UNIT_NAMED_INPUT_CHANGED"):
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
	if !strings.HasSuffix(string(buildBrief), map[string]string{first: "Read each round: yes\nBuild the first way.\n", second: "Read each round: yes\nBuild the second way.\n"}[winnerBrief]) {
		t.Fatalf("the run's build brief is not its own request's:\n%s", buildBrief)
	}
	retained := retainedInputs(t, directory)
	launched := len(bed.starter.launched())

	code, refused, _ := bed.work(argsFor(loserBrief)...)
	if code != 1 || refused.Outcome != intentRefused || !strings.Contains(resultWords(refused), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(resultWords(refused), "run="+run) {
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
type verdictAdapter struct{ findings, structured *string }

func (a verdictAdapter) Command(record launch.Record, state string) (launch.Command, error) {
	var declared []string
	_ = json.Unmarshal(record.AdapterData["declaredOutputs"], &declared)
	if *a.findings != "" && len(declared) > 0 {
		if err := os.WriteFile(declared[0], []byte(*a.findings), 0o600); err != nil {
			return launch.Command{}, err
		}
		if len(declared) > 1 && a.structured != nil {
			if err := os.WriteFile(declared[1], []byte(*a.structured), 0o600); err != nil {
				return launch.Command{}, err
			}
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

func (readProcesses) SelfRef() (identity.Ref, error) { return workProcessRef(10), nil }
func (readProcesses) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	return exitedChild{}, workProcessRef(30), nil
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
	return workProcessRef(10), nil
}

// TestIntentReadVerdictFromRetainedFindings: the read's findings are a
// declared output, copied into each read launch and measured for its
// verdict. A fix-first verdict is reported as such, never a clean read; a
// reader that writes no findings fails the read.
func TestIntentReadVerdictFromRetainedFindings(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	findings := "1. witness.txt: wrong bytes\nVERDICT: fix first (1 material findings)\n"
	// Collection requires structured findings as well as the declared
	// prose output; the prose alone cannot authorize a correction.
	structured := `{"findings":[{"class":"regression","where":"witness.txt","severity":"high","material":true,"claim":"wrong bytes","evidence":"witness.txt:1","change":"correct the bytes"}],"verdictMaterialCount":1}`
	adapter := verdictAdapter{findings: &findings, structured: &structured}
	for _, name := range []string{"codex-exec", "claude-headless"} {
		bed.manager.Adapters[name] = adapter
	}
	bed.manager.Processes = readProcesses{}
	bed.manager.Supervisor = superviseReads{bed.starter}
	brief := bed.brief("brief.md", "Read each round: yes\nBuild the unit.\n")
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "read", "--brief", brief, "--lines", "5"}, workCheck...)...)
	data := resultData(t, result)
	if code != 0 || data["outcome"] != "green" || data["readClean"] != false || !strings.Contains(result.Summary, "read verdict: fix first (1 material findings)") {
		t.Fatalf("fix-first read: code=%d %+v", code, result)
	}
	copies := data["readFindings"].([]any)
	// Each read retains both its prose and its structured stop evidence.
	if len(copies) != 2 {
		t.Fatalf("read findings copies=%v", copies)
	}
	firstCopy := copies[0].(string)
	if retained, _ := os.ReadFile(firstCopy); string(retained) != findings {
		t.Fatalf("retained findings=%q", retained)
	}
	if retained, _ := os.ReadFile(copies[1].(string)); string(retained) != structured {
		t.Fatalf("retained structured findings=%q", retained)
	}
	plan, _ := launch.ReadUnitPlan(data["plan"].(string))
	if len(plan.Read.Outputs) != 1 {
		t.Fatalf("read outputs=%v", plan.Read.Outputs)
	}
	findings = "No material findings.\nVERDICT: land\n"
	run := data["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil || len(record.Rounds[0].Reads) != 1 {
		t.Fatalf("structured read was not collected: %+v %v", record, err)
	}
	structured = `{"findings":[],"verdictMaterialCount":0}`
	correction := fmt.Sprintf("Fix the witness.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n| %s | fixed | witness.txt:1 |\n", record.Rounds[0].Reads[0].Findings[0].ID)
	code, result, _ = bed.work("work", "revise", "run:"+run, "--brief", bed.brief("follow-up.md", correction))
	data = resultData(t, result)
	if code != 0 || data["round"].(float64) != 2 || data["readClean"] != true || !strings.Contains(result.Summary, "read verdict: land") {
		t.Fatalf("clean read after the fold: code=%d %+v", code, result)
	}
	if retained, _ := os.ReadFile(firstCopy); !strings.Contains(string(retained), "fix first") {
		t.Fatalf("round one's findings copy was replaced: %q", retained)
	}
	bed.manager.Prober = &treeProber{dead: true}
	if _, err := (&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}).CancelRun(run); err != nil {
		t.Fatal(err)
	}
	bed.manager.Prober = workProber{}
	findings = ""
	code, result, _ = bed.work(append([]string{"work", "build", bed.id, "silent", "--brief", brief, "--lines", "5"}, workCheck...)...)
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
	material := []readsubject.Finding{stopFinding("regression", "a.go"), stopFinding("incomplete-item", "b.go")}
	bed.manager.Supervisor = &stopReadStarter{bed: bed, reads: [][]readsubject.Finding{material, material, {stopFinding("scope", "c.go")}}}
	plain := bed.brief("plain.md", "Read each round: yes\nBuild the unit.\n")
	check := []string{}
	// A brief that names no read budget uses the configured allowance.
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "plain", "--brief", plain, "--lines", "5"}, check...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("configured read budget: code=%d %+v", code, result)
	}
	if plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string)); err != nil {
		t.Fatal(err)
	} else if readBrief, _ := os.ReadFile(plan.Read.Brief); !strings.Contains(string(readBrief), "Maximum reader tool calls: 48") {
		t.Fatalf("the configured allowance is not the read's budget:\n%s", readBrief)
	}
	plainRun := resultData(t, result)["run"].(string)
	budgeted := bed.brief("budgeted.md", "Read each round: yes\nBuild the unit.\n\nMaximum reader tool calls: 25\n")
	if _, err := (&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}).CancelRun(resultData(t, result)["run"].(string)); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work(append([]string{"work", "build", bed.id, "budget", "--brief", budgeted, "--lines", "5", "--read-tool-calls", "30"}, check...)...)
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "25") {
		t.Fatalf("conflicting read budget: code=%d %+v", code, result)
	}
	if _, err := (&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}).CancelRun(plainRun); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work(append([]string{"work", "build", bed.id, "budget", "--brief", budgeted, "--lines", "5"}, check...)...)
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
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	correction := "Again.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n"
	for _, finding := range record.Rounds[0].Reads[0].Findings {
		correction += fmt.Sprintf("| %s | fixed | %s:1 |\n", finding.ID, finding.Where)
	}
	followUp := bed.brief("follow-up.md", correction)
	if code, result, _ = bed.work("work", "revise", "run:"+run, "--brief", followUp); code != 0 || resultData(t, result)["round"].(float64) != 2 {
		t.Fatalf("second round: code=%d %+v", code, result)
	}
	launched := len(bed.starter.launched())
	// A distinct request tests the cap; identical input rejoins its frozen round
	// (plans/designs/briefs-carry-their-rules.md:66).
	third := bed.brief("third.md", "Correct another defect.\n")
	code, result, _ = bed.work("work", "revise", "run:"+run, "--brief", third)
	// The goal's approved cap is frozen into the collection decision;
	// its stop is consumed before another round can be admitted.
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(resultWords(result), "UNIT_STOPPED") ||
		resultData(t, result)["stop"].(map[string]any)["class"] != "correction allowance spent" || len(bed.starter.launched()) != launched {
		t.Fatalf("third round: code=%d %+v", code, result)
	}

	unapproved := newWorkBed(t)
	unapproved.intentBed = newIntentBed(t, false, func(file *goal.GoalFile) { file.Budget, file.Approved = nil, nil })
	code, result, _ = unapproved.work(append([]string{"work", "build", unapproved.id, "u", "--brief", unapproved.brief("b.md", "B.\n"), "--lines", "5"}, workCheck...)...)
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "not approved with a budget") || result.Next == nil || !strings.HasPrefix(shellCommand(result.Next.Argv), "metasystem goal approve") {
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
		return append([]string{"work", "build", bed.id, "modelled", "--brief", brief, "--lines", "5", "--model", model, "--effort", "high"}, workCheck...)
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
	bed.manager.Settings.BuildModel, bed.manager.Settings.BuildEffort = "seat-model", "low"
	if code, result, _ = bed.work("work", "build", "run:"+run); code != 0 || resultData(t, result)["buildModel"] != "claude-sonnet-5" {
		t.Fatalf("resume: code=%d %+v", code, result)
	}
	if code, result, _ = bed.work(args("claude-sonnet-5")...); code != 0 || resultData(t, result)["run"] != run {
		t.Fatalf("repeat after seat settings changed: code=%d %+v", code, result)
	}
	if code, result, _ = bed.work(args("claude-opus-5-5")...); code != 1 || !strings.Contains(resultWords(result), "UNIT_NAMED_INPUT_CHANGED") {
		t.Fatalf("another model for the same unit: code=%d %+v", code, result)
	}
	if code, result, _ = bed.work(append([]string{"work", "build", bed.id, "bad", "--brief", brief, "--lines", "5", "--effort", "extreme"}, workCheck...)...); code != 2 || result.Outcome != intentRefused {
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
	return root.Path()
}

// TestIntentBriefCarriesAcceptedDesign: the scaffold carries the accepted
// design's units, constraints, return and acceptance text and marks only the
// decision neither record holds; a project that cannot be read is refused.
func TestIntentBriefCarriesAcceptedDesign(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	designs := filepath.Join(bed.stateRoot(), "plans", "designs")
	os.MkdirAll(designs, 0o700)
	// A unit maps to its accepted Decision (plans/designs/briefs-carry-their-rules.md:58).
	design := "# Standing validation\n\n- Kind: design\n- Id: 01M3CGR7CNZTS2NNTQCYRCF4JZ\n- Status: accepted\n- Goals: standing-validation\n\n" +
		"## Non-goals\n\nNo new ledger schema.\n\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| u1 | 40 |\n\n" +
		"## u1 — Standing validation\n\nBuild the validator.\n\n## Return\n\nThe diff and the proof log.\n\n## Acceptance\n\nThe validator refuses a stale box.\n"
	if err := os.WriteFile(filepath.Join(designs, "standing-validation.md"), []byte(design), 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _ := bed.work("work", "brief", bed.id, "--work", "u1", "--out", "brief.md")
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
	if code != 0 || len(missing) != 0 || !strings.Contains(string(written), "Maximum reader tool calls: 48") {
		t.Fatalf("the read budget is the configured allowance, nothing undecided: code=%d missing=%v\n%s", code, missing, written)
	}
	filled := "Read each round: yes\n" + strings.Replace(string(written), "Maximum reader tool calls: 48", "Maximum reader tool calls: 20", 1)
	os.WriteFile(filepath.Join(bed.root(), "filled.md"), []byte(filled), 0o600)
	code, result, _ = bed.work([]string{"work", "build", bed.id, "u1", "--brief", "filled.md"}...)
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
		{"work", "brief", broken.id, "--out", "b.md"},
		append([]string{"work", "build", broken.id, "u", "--brief", broken.brief("x.md", "X.\n"), "--lines", "5"}, workCheck...),
	} {
		code, result, _ := broken.work(args...)
		if code != 1 || result.Outcome != intentFailed || !strings.Contains(result.Summary, "design records can't be read") {
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
		code := runIntentIn(mustIntentCommand(t, "settings show"), append(args, "--json"), &stdout, &stderr, roots[0], owners)
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

// An overrides-only metasystem.conf: `settings show KEY` answers a key the
// file does not name with its compiled default and says so, and `settings
// keys` lists it (the lesson of the evidence-root landing, for every key).
func TestIntentSettingsShowAndKeysAnswerCompiledDefaults(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o700)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("# overrides only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	owners := intentOwners{resolver: stateroot.NewResolver(fakeTop(root), noExecutable)}
	show := func(key string) (int, string) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, "settings show"), []string{key, "--repo", root}, &stdout, &stderr, root, owners)
		return code, stdout.String()
	}
	for key, want := range map[string]string{"watch.stale-min": "20", "suite.section-cap-min": "45", "testing.contract": "testing.json",
		"disk.go-cache-cap-gib": "30", "disk.floor-gib": "50"} {
		code, text := show(key)
		if code != 0 || !strings.HasPrefix(text, key+" is "+want+" · default\n") {
			t.Fatalf("settings show %s: code=%d %q; want %s=%s (default)", key, code, text, key, want)
		}
	}
	var stdout bytes.Buffer
	if code := configKeysTo(&stdout, filepath.Join(root, "metasystem.conf"), "watch.", nil); code != 0 ||
		!strings.Contains(stdout.String(), "watch.stale-min\n") || !strings.Contains(stdout.String(), "watch.cap-min\n") {
		t.Fatalf("settings keys watch.: code=%d %q", code, stdout.String())
	}
	// The disk-lifetime rows and the cache trimmer's keys are in the one
	// table: `settings keys` lists them with the others.
	stdout.Reset()
	if code := configKeysTo(&stdout, filepath.Join(root, "metasystem.conf"), "disk.", nil); code != 0 ||
		!strings.Contains(stdout.String(), "disk.go-cache-cap-gib\n") || !strings.Contains(stdout.String(), "disk.floor-gib\n") {
		t.Fatalf("settings keys disk.: code=%d %q", code, stdout.String())
	}
}
