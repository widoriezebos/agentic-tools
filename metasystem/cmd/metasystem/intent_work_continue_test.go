package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// releaseHeldBuild completes the one build launch a held starter left
// running, as its supervisor would.
func (b *workBed) releaseHeldBuild(result intentResult) {
	b.t.Helper()
	b.starter.mu.Lock()
	b.starter.hold = ""
	b.starter.mu.Unlock()
	launchID := resultData(b.t, result)["steps"].([]any)[0].(map[string]any)["launchId"].(string)
	if _, err := b.manager.Store.Update(launchID, func(record *launch.Record) error {
		exit := 0
		record.State, record.ExitCode = launch.Completed, &exit
		return nil
	}); err != nil {
		b.t.Fatal(err)
	}
}

func tamper(t *testing.T, path string) func() {
	t.Helper()
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(append([]byte(nil), original...), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestIntentNamedContinuationKeepsTheReservation: build --resume, wait unit
// and fold unit continue a named run under its named lock with its
// reservation verified, so a changed retained input or launch setting starts
// no pending launch; restored, the run goes on. A follow-up brief is the
// caller's own new input. A run no named entry claims continues as before.
func TestIntentNamedContinuationKeepsTheReservation(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n")
	bed.starter.hold = "build"
	code, result, _ := bed.work(append([]string{"build", bed.id, "guarded", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 3 || result.Outcome != intentInProgress {
		t.Fatalf("capped build: code=%d %+v", code, result)
	}
	run, inputs := resultData(t, result)["run"].(string), resultData(t, result)["inputs"]
	if inputs == nil {
		plan, _ := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
		inputs = filepath.Dir(plan.Build.Brief)
	}
	bed.releaseHeldBuild(result)
	launched := len(bed.starter.launched())
	restore := tamper(t, filepath.Join(inputs.(string), "build-brief.md"))
	for _, args := range [][]string{{"build", "--resume", run}, {"wait", "run", run}} {
		code, refused, _ := bed.work(args...)
		if code != 1 || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(refused.Summary, "run="+run) {
			t.Fatalf("%v after a retained input changed: code=%d %+v", args, code, refused)
		}
	}
	restore()
	bed.manager.Settings.BuildLinesCap++
	if code, refused, _ := bed.work("wait", "run", run); code != 1 || !strings.Contains(refused.Summary, "UNIT_NAMED_INPUT_CHANGED") {
		t.Fatalf("wait unit after a launch setting changed: code=%d %+v", code, refused)
	}
	bed.manager.Settings.BuildLinesCap--
	if len(bed.starter.launched()) != launched {
		t.Fatalf("a refused continuation launched: %v", bed.starter.launched())
	}
	code, result, _ = bed.work("wait", "run", run)
	if code != 0 || resultData(t, result)["outcome"] != "green" || !slices.Equal(bed.starter.launched()[launched:], []string{"proof", "read"}) {
		t.Fatalf("restored continuation: code=%d %+v launches=%v", code, result, bed.starter.launched())
	}

	launched = len(bed.starter.launched())
	restore = tamper(t, filepath.Join(inputs.(string), "read-brief.md"))
	followUp := bed.brief("follow-up.md", "Fix it.\n")
	if code, refused, _ := bed.work("revise", "run", run, "--brief", followUp); code != 1 || !strings.Contains(refused.Summary, "UNIT_NAMED_INPUT_CHANGED") || len(bed.starter.launched()) != launched {
		t.Fatalf("fold after a retained input changed: code=%d %+v", code, refused)
	}
	restore()
	bed.brief("follow-up.md", "Fix it, differently.\n")
	// The run keeps its own plan, proof and round limit: a goal's attempt,
	// work item or decisions file is refused before the runner is asked.
	for _, conflict := range [][]string{{"--after", "1"}, {"--work", "guarded"}, {"--dispositions", followUp}} {
		if code, refused, _ := bed.work(append([]string{"revise", "run", run, "--brief", followUp}, conflict...)...); code != 2 || refused.Outcome != intentRefused || len(bed.starter.launched()) != launched {
			t.Fatalf("revise run with %v: code=%d %+v", conflict, code, refused)
		}
	}
	code, result, _ = bed.work("revise", "run", run, "--brief", followUp)
	if code != 0 || resultData(t, result)["round"].(float64) != 2 || resultData(t, result)["maxRounds"].(float64) != 2 {
		t.Fatalf("fold with its own brief: code=%d %+v", code, result)
	}
	if code, refused, _ := bed.work("revise", "run", run, "--brief", followUp); code != 1 || !strings.Contains(refused.Summary, "UNIT_ROUND_LIMIT") {
		t.Fatalf("round past the limit: code=%d %+v", code, refused)
	}

	// A run started from a plan without a named entry continues directly.
	plan, _ := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	data, _ := os.ReadFile(resultData(t, result)["plan"].(string))
	legacyPlan := filepath.Join(bed.root(), "legacy-plan.json")
	os.WriteFile(legacyPlan, []byte(strings.Replace(string(data), `"unit": "guarded"`, `"unit": "legacy"`, 1)), 0o600)
	runner := &launch.UnitRunner{Manager: bed.manager, Git: workGit{bed}, Root: bed.unitRoot}
	legacy, err := runner.Advance(launch.UnitRequest{Plan: legacyPlan})
	if err != nil || plan.Unit != "guarded" {
		t.Fatalf("legacy plan: %v", err)
	}
	if code, result, _ = bed.work("wait", "run", legacy.Record.ID); code != 0 || resultData(t, result)["run"] != legacy.Record.ID {
		t.Fatalf("legacy continuation: code=%d %+v", code, result)
	}
}

// codexVerdictAdapter builds the read's command with the real Codex adapter
// and measures the verdict from the declared findings file.
type codexVerdictAdapter struct {
	launch.CodexExec
	verdictAdapter
}

func (a codexVerdictAdapter) Command(record launch.Record, state string) (launch.Command, error) {
	return a.CodexExec.Command(record, state)
}
func (a codexVerdictAdapter) Measure(record launch.Record, state string) (launch.Measurement, []launch.Output, map[string]json.RawMessage, error) {
	return a.verdictAdapter.Measure(record, state)
}
func (a codexVerdictAdapter) Strays() ([]string, error) { return nil, nil }

var writeFindingsTo = regexp.MustCompile(`(?m)^Write findings to: (.+)$`)

// sandboxReader stands in for the Codex child: it writes the findings file
// the brief it is handed names, and records the command it was started with.
type sandboxReader struct {
	mu       *sync.Mutex
	commands *[]launch.Command
	findings *string
}

func (r sandboxReader) SelfRef() (identity.Ref, error) { return workRef(10), nil }
func (r sandboxReader) StartChild(command launch.Command) (launch.Child, identity.Ref, error) {
	r.mu.Lock()
	*r.commands = append(*r.commands, command)
	r.mu.Unlock()
	if match := writeFindingsTo.FindStringSubmatch(command.Stdin); match != nil {
		if err := os.WriteFile(match[1], []byte(*r.findings), 0o600); err != nil {
			return nil, identity.Ref{}, err
		}
	}
	return exitedChild{}, workRef(30), nil
}
func (sandboxReader) SignalGroup(int64, syscall.Signal) error { return nil }
func (sandboxReader) GroupAlive(int64) (bool, error)          { return false, nil }

// TestIntentReadFindingsInSandboxTemp: the read's findings file is in a
// private directory under the temporary root, which Codex's workspace-write
// sandbox may write and which is outside the worktree; the launch owner
// keeps each read's copy, and a cleaned directory is recreated before the
// next read.
func TestIntentReadFindingsInSandboxTemp(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	findings := "No material findings.\nVERDICT: land\n"
	var mu sync.Mutex
	var commands []launch.Command
	adapter := codexVerdictAdapter{CodexExec: launch.CodexExec{Binary: "codex", Model: "gpt-6-sol", Effort: "high"}, verdictAdapter: verdictAdapter{findings: new(string)}}
	for _, name := range []string{"codex-exec", "claude-headless"} {
		bed.manager.Adapters[name] = adapter
	}
	bed.manager.Processes = sandboxReader{mu: &mu, commands: &commands, findings: &findings}
	bed.manager.Supervisor = superviseReads{bed.starter}
	brief := bed.brief("brief.md", "Build the unit.\n")
	code, result, _ := bed.work(append([]string{"build", bed.id, "sandboxed", "--brief", brief, "--lines", "5"}, workCheck...)...)
	data := resultData(t, result)
	if code != 0 || data["outcome"] != "green" || data["readClean"] != true {
		t.Fatalf("sandboxed read: code=%d %+v", code, result)
	}
	plan, _ := launch.ReadUnitPlan(data["plan"].(string))
	output := plan.Read.Outputs[0]
	temporary, _ := filepath.EvalSymlinks(os.TempDir())
	resolved, _ := filepath.EvalSymlinks(filepath.Dir(output))
	if !strings.HasPrefix(resolved, temporary+string(filepath.Separator)) || strings.HasPrefix(output, bed.worktree) || strings.HasPrefix(output, bed.unitRoot) {
		t.Fatalf("findings path %s is not a private temporary path outside the worktree and run store", output)
	}
	if len(commands) != 1 || !slices.Contains(commands[0].Args, "workspace-write") || commands[0].Directory != bed.worktree ||
		!strings.Contains(commands[0].Stdin, "Write findings to: "+output) {
		t.Fatalf("codex command=%+v", commands)
	}
	copies := data["readFindings"].([]any)
	if retained, _ := os.ReadFile(copies[0].(string)); len(copies) != 1 || string(retained) != findings {
		t.Fatalf("retained copies=%v", copies)
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(output)); err != nil {
		t.Fatal(err)
	}
	code, result, _ = bed.work("revise", "run", data["run"].(string), "--brief", bed.brief("follow-up.md", "Again.\n"))
	info, err := os.Lstat(filepath.Dir(output))
	if code != 0 || resultData(t, result)["readClean"] != true || err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("read after the findings directory was cleaned: code=%d %+v info=%v err=%v", code, result, info, err)
	}
}
