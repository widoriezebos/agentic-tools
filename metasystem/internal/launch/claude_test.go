package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func rawString(value string) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func claudeRecord(t *testing.T, kind string) (Record, string) {
	t.Helper()
	root := t.TempDir()
	briefPath := filepath.Join(root, "brief.md")
	if err := os.WriteFile(briefPath, []byte("brief text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]json.RawMessage{"brief": rawString(briefPath)}
	setInt64(data, "window", 1000000)
	return Record{Kind: kind, Tag: "alpha", WorkingDirectory: root, AdapterData: data}, root
}

func TestClaudeHeadlessCommand(t *testing.T) {
	record, root := claudeRecord(t, "design")
	record.AdapterData["model"] = rawString("chosen")
	record.AdapterData["resumeSession"] = rawString("session-1")
	state := t.TempDir()
	command, err := (ClaudeHeadless{Binary: "fake-claude"}).Command(record, state)
	want := []string{"-p", "--model", "chosen", "--dangerously-skip-permissions", "--output-format", "json", "--name", "design-alpha", "--resume", "session-1"}
	require(t, err != nil || command.Program != "fake-claude" || command.Directory != root || command.Stdin != "brief text\n" || !reflect.DeepEqual(command.Args, want), "command=%+v err=%v", command, err)
	require(t, command.StdoutPath != filepath.Join(state, "result.json") || command.LogPath != filepath.Join(state, "stderr.log") || command.StdoutPath == command.LogPath, "output paths=%+v", command)
}

func TestReadModelWithContextSuffixPassesThroughToClaude(t *testing.T) {
	t.Parallel()
	record, _ := claudeRecord(t, "read")
	record.AdapterData["model"] = rawString("claude-opus-5[1m]")
	command, err := (ClaudeHeadless{Binary: "claude"}).Command(record, t.TempDir())
	want := []string{"-p", "--model", "claude-opus-5[1m]", "--dangerously-skip-permissions", "--output-format", "json", "--name", "read-alpha"}
	if err != nil || !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("argv=%v want=%v err=%v", command.Args, want, err)
	}
	delete(record.AdapterData, "model")
	command, err = (ClaudeHeadless{Binary: "claude"}).Command(record, t.TempDir())
	if err != nil || !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("fallback argv=%v want=%v err=%v", command.Args, want, err)
	}
}

func TestLaunchEnvironmentHelper(t *testing.T) {
	if os.Getenv("GO_LAUNCH2_CHILD") != "1" {
		return
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	fmt.Fprint(os.Stdout, os.Getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW"))
	os.Exit(0)
}

func TestWindowVariableReachesOnlyTheChild(t *testing.T) {
	wantWindow := "1000000"
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "765432")
	state := t.TempDir()
	prober := identityKernelProber()
	processes := OSProcesses{Prober: prober}
	command := Command{
		Program: os.Args[0], Args: []string{"-test.run=^TestLaunchEnvironmentHelper$"},
		Stdin:       strings.Repeat("x", 16<<20),
		Environment: []string{"GO_LAUNCH2_CHILD=1", "CLAUDE_CODE_AUTO_COMPACT_WINDOW=" + wantWindow},
		LogPath:     filepath.Join(state, "stderr"), StdoutPath: filepath.Join(state, "stdout"),
	}
	child, ref, err := processes.StartChild(command)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = processes.SignalGroup(ref.Pid, syscall.SIGKILL) })
	exit, err := child.Wait()
	if err != nil || exit != 0 {
		t.Fatalf("child exit=%d err=%v", exit, err)
	}
	data, err := os.ReadFile(command.StdoutPath)
	require(t, err != nil || string(data) != wantWindow, "child window=%q err=%v", data, err)
	require(t, os.Getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW") != "765432", "caller window changed to %q", os.Getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW"))
}

func identityKernelProber() identity.Prober { return identity.KernelProber{} }

func TestKindSelectsTheAdapterAndTheDefaultModel(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	m.Adapters["claude-headless"] = fakeAdapter{}
	cases := []struct {
		kind, adapter, model string
	}{
		{"design", "claude-headless", "claude-fable-5-1"},
		{"read", "claude-headless", "claude-opus-5[1m]"},
		{"build", "codex-exec", "gpt-5.6-sol"},
		{"critique", "codex-exec", "gpt-5.6-sol"},
	}
	for index, row := range cases {
		briefPath := brief(t)
		spec := StartSpec{ID: fmt.Sprintf("kind-%d", index), Kind: row.kind, Tag: "tag", Brief: briefPath, WorkingDirectory: t.TempDir()}
		if row.kind == "read" {
			spec.DiffFile = writeLaunchFile(t, "change.diff", "")
		}
		record, err := m.Start(spec)
		if err != nil || record.Adapter != row.adapter {
			t.Fatalf("%s record=%+v err=%v", row.kind, record, err)
		}
		var command Command
		if row.adapter == "claude-headless" {
			command, err = (ClaudeHeadless{Binary: "claude"}).Command(record, t.TempDir())
			if err == nil && flagValue(command.Args, "--model") != row.model {
				err = fmt.Errorf("model %q", flagValue(command.Args, "--model"))
			}
			if err == nil && hasArg(command.Args, "--resume") {
				err = fmt.Errorf("unexpected resume: %v", command.Args)
			}
		} else {
			commandRecord := record
			commandRecord.Kind = "build"
			command, err = (CodexExec{Binary: "codex", Model: "gpt-5.6-sol", Effort: "xhigh"}).Command(commandRecord, t.TempDir())
			if err == nil && flagValue(command.Args, "-m") != row.model {
				err = fmt.Errorf("model %q", flagValue(command.Args, "-m"))
			}
		}
		if err != nil {
			t.Fatalf("%s command: %v", row.kind, err)
		}
	}
	override := map[string]json.RawMessage{"model": rawString("override")}
	record, err := m.Start(StartSpec{ID: "kind-override", Kind: "read", Tag: "tag", Brief: brief(t), WorkingDirectory: t.TempDir(), DiffFile: writeLaunchFile(t, "override.diff", ""), AdapterData: override})
	command, commandErr := (ClaudeHeadless{Binary: "claude"}).Command(record, t.TempDir())
	require(t, err != nil || commandErr != nil || flagValue(command.Args, "--model") != "override", "override command=%+v err=%v/%v", command, err, commandErr)
	before, _ := m.Store.List()
	_, err = m.Start(StartSpec{ID: "unknown-kind", Kind: "other", Brief: brief(t), WorkingDirectory: t.TempDir()})
	after, _ := m.Store.List()
	require(t, err == nil || len(after) != len(before), "unknown kind err=%v before=%d after=%d", err, len(before), len(after))
}

func writeClaudeResult(t *testing.T, state, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state, "result.json"), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func seedClaude(t *testing.T, m *Manager, id string) (Record, string) {
	t.Helper()
	record := seed(t, m, id, Starting)
	state, _ := m.Store.StateDir(id)
	briefPath := filepath.Join(record.WorkingDirectory, "brief")
	os.WriteFile(briefPath, []byte("brief"), 0o600)
	m.Store.Update(id, func(current *Record) error {
		current.Adapter = "claude-headless"
		current.Kind = "design"
		current.AdapterData["brief"] = rawString(briefPath)
		setInt64(current.AdapterData, "window", 200000)
		return nil
	})
	record.Adapter = "claude-headless"
	return record, state
}

func TestClaudeResultErrorFailsTheLaunch(t *testing.T) {
	cases := []struct {
		name, result, reason string
		exit                 int
	}{
		{"error", `{"session_id":"sid","is_error":true,"num_turns":1,"result":"bad"}`, "result-error", 0},
		{"missing", "", "result-unreadable", 0},
		{"broken", "{", "result-unreadable", 0},
		{"exit", `{"session_id":"sid","is_error":false,"num_turns":1,"result":"ok"}`, "exit-7", 7},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			m, processes, _, _ := manager(t)
			projects := t.TempDir()
			m.Adapters["claude-headless"] = ClaudeHeadless{ProjectsRoot: projects, Scanner: scan{}}
			processes.exit = row.exit
			_, state := seedClaude(t, m, "result-"+row.name)
			if row.result != "" {
				writeClaudeResult(t, state, row.result)
			}
			if row.name == "exit" {
				dir := filepath.Join(projects, "project")
				os.MkdirAll(dir, 0o700)
				os.WriteFile(filepath.Join(dir, "sid.jsonl"), nil, 0o600)
			}
			got, err := m.Supervise("result-" + row.name)
			require(t, err != nil || got.State != Failed || got.Reason != row.reason, "record=%+v err=%v", got, err)
		})
	}
}

func measureFixture(t *testing.T, rows string) (Measurement, []Output, error) {
	t.Helper()
	state, projects := t.TempDir(), t.TempDir()
	writeClaudeResult(t, state, `{"session_id":"sid","is_error":false,"num_turns":4,"result":"one\ntwo words\nlast line"}`)
	dir := filepath.Join(projects, "nested", "project")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "sid.jsonl"), []byte(rows), 0o600)
	measurement, outputs, _, err := (ClaudeHeadless{ProjectsRoot: projects}).Measure(Record{Kind: "design", AdapterData: map[string]json.RawMessage{}}, state)
	return measurement, outputs, err
}

func TestClaudeMeasureCountsOnlyCompactBoundaryRows(t *testing.T) {
	rows := strings.Join([]string{
		`{"type":"system","subtype":"compact_boundary"}`,
		`{"type":"system","subtype":"summary","content":"compact_boundary"}`,
		`{"type":"user","message":"compact_boundary compact"}`,
		`{"type":"assistant","message":{"id":"m1","usage":{"input_tokens":1}}}`,
	}, "\n") + "\n"
	got, _, err := measureFixture(t, rows)
	require(t, err != nil || got.Compactions != 1 || got.Turns != 4 || got.ResultLines != 3 || got.ResultWords != 5 || got.ResultTail != "two words | last line", "measurement=%+v err=%v", got, err)
}

func TestClaudeMeasureReadsCallsAndPeakFromUsage(t *testing.T) {
	rows := strings.Join([]string{
		`{"type":"assistant","message":{"id":"same","usage":{"input_tokens":2,"cache_read_input_tokens":100000,"cache_creation_input_tokens":100000}}}`,
		`{"type":"assistant","message":{"id":"same","usage":{"input_tokens":2,"cache_read_input_tokens":100000,"cache_creation_input_tokens":100000}}}`,
		`{"type":"assistant","message":{"id":"second","usage":{"input_tokens":9,"cache_read_input_tokens":210000,"cache_creation_input_tokens":0}}}`,
		`{"type":"assistant","message":{"id":"none"}}`,
		`{"type":"user","message":{"id":"user","usage":{"input_tokens":999999}}}`,
	}, "\n") + "\n"
	got, _, err := measureFixture(t, rows)
	require(t, err != nil || got.Calls != 2 || got.PeakContext != 210009 || got.CallsAbove200 != 2, "measurement=%+v err=%v", got, err)
}

func TestReadRecordsTheVerdictLine(t *testing.T) {
	root, state, projects := t.TempDir(), t.TempDir(), t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	os.WriteFile(first, []byte("heading\nVERDICT: revise\nVERDICT: ignore\n"), 0o600)
	os.WriteFile(second, []byte("VERDICT: later\n"), 0o600)
	writeClaudeResult(t, state, `{"session_id":"sid","is_error":false,"result":"done"}`)
	os.MkdirAll(filepath.Join(projects, "p"), 0o700)
	os.WriteFile(filepath.Join(projects, "p", "sid.jsonl"), nil, 0o600)
	data := map[string]json.RawMessage{}
	setStrings(data, "declaredOutputs", []string{"first.txt", "second.txt"})
	got, _, _, err := (ClaudeHeadless{ProjectsRoot: projects}).Measure(Record{Kind: "read", WorkingDirectory: root, AdapterData: data}, state)
	require(t, err != nil || got.Verdict != "VERDICT: revise", "measurement=%+v err=%v", got, err)
	os.WriteFile(first, []byte("none\n"), 0o600)
	os.WriteFile(second, []byte("also none\n"), 0o600)
	got, _, _, err = (ClaudeHeadless{ProjectsRoot: projects}).Measure(Record{Kind: "read", WorkingDirectory: root, AdapterData: data}, state)
	require(t, err != nil || got.Verdict != "none", "measurement=%+v err=%v", got, err)
}

func TestCensusNamesAStrayHeadlessRun(t *testing.T) {
	m, processes, _, _ := manager(t)
	owned := seed(t, m, "owned-claude", Running)
	m.Store.Update(owned.ID, func(record *Record) error { record.Adapter = "claude-headless"; return nil })
	processes.group = false
	m.Adapters["claude-headless"] = ClaudeHeadless{Scanner: scan{
		{Ref: *owned.Child, Argv: []string{"claude", "-p", "--name", "design-owned"}},
		{Ref: ref(31), Argv: []string{"/bin/claude", "-p", "--name", "design-stray"}},
		{Ref: ref(32), Argv: []string{"claude", "-p", "--name", "read-stray"}},
		{Ref: ref(33), Argv: []string{"claude", "--name", "design-interactive"}},
		{Ref: ref(34), Argv: []string{"claude", "-p", "--name", "build-other"}},
	}}
	lines, err := m.Census()
	joined := strings.Join(lines, "\n")
	require(t, err != nil || !strings.Contains(joined, "design-stray") || !strings.Contains(joined, "read-stray") || strings.Contains(joined, "design-owned") || strings.Contains(joined, "design-interactive") || strings.Contains(joined, "build-other"), "census=%q err=%v", joined, err)
}

func TestRoundTaskMatchesTheGoldenFiles(t *testing.T) {
	r1, err := os.ReadFile(filepath.Join("testdata", "round-task-r1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := RoundTask("demo", 2, r1, "0")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name string
		got  []byte
	}{{"round-task-r2.golden", r2}, {"round-task-r2-constraints.golden", func() []byte {
		data, taskErr := RoundTask("demo", 2, r1, "7")
		if taskErr != nil {
			t.Fatal(taskErr)
		}
		return data
	}()}} {
		want, readErr := os.ReadFile(filepath.Join("testdata", row.name))
		if readErr != nil || !bytes.Equal(row.got, want) {
			t.Fatalf("%s mismatch err=%v\nwant:\n%s\ngot:\n%s", row.name, readErr, want, row.got)
		}
	}
	r3, err := RoundTask("demo", 3, r2, "7")
	want, readErr := os.ReadFile(filepath.Join("testdata", "round-task-r3.golden"))
	if err != nil || readErr != nil || !bytes.Equal(r3, want) {
		t.Fatalf("round 3 mismatch err=%v/%v\nwant:\n%s\ngot:\n%s", err, readErr, want, r3)
	}
	current, err := os.ReadFile(filepath.Join("testdata", "round-task-current-r1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := RoundTask("demo", 2, current, "0")
	want, readErr = os.ReadFile(filepath.Join("testdata", "round-task-current-r2.golden"))
	if err != nil || readErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("current round 2 mismatch err=%v/%v\nwant:\n%s\ngot:\n%s", err, readErr, want, got)
	}
}

func TestEmptyBriefIsRefusedWithoutARecord(t *testing.T) {
	m, _, _, _ := manager(t)
	path := filepath.Join(t.TempDir(), "empty")
	os.WriteFile(path, nil, 0o600)
	_, err := m.Start(StartSpec{ID: "empty-brief", Kind: "design", Brief: path, WorkingDirectory: t.TempDir()})
	records, listErr := m.Store.List()
	require(t, err == nil || listErr != nil || len(records) != 0, "err=%v listErr=%v records=%+v", err, listErr, records)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("module root not found")
		}
		root = parent
	}
}

func TestNoTrackedFileNamesTheDeletedLaunchers(t *testing.T) {
	root := moduleRoot(t)
	forbidden := []string{
		"headless-" + "design-launch.sh",
		"headless-" + "design-wait.sh",
		"codex-" + "critique-launch.sh",
		"critique-" + "round-task.sh",
		"headless-" + "launchers-fixtures.sh",
		"companion CLI",
		"codex-companion",
		"codex:codex-rescue",
		"/codex:",
	}
	paths := []string{"scripts", "docs", "skills", "AGENTS.md"}
	for _, relative := range paths {
		path := filepath.Join(root, relative)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			checkForbiddenNames(t, path, forbidden)
			continue
		}
		err = filepath.WalkDir(path, func(file string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				checkForbiddenNames(t, file, forbidden)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkForbiddenNames(t *testing.T, path string, forbidden []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lowerPath, lowerData := strings.ToLower(path), bytes.ToLower(data)
	for _, name := range forbidden {
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerPath, lowerName) || bytes.Contains(lowerData, []byte(lowerName)) {
			t.Errorf("%s names removed file %s", path, name)
		}
	}
}

func TestProjectRulesNameTheLaunchVerbs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "docs", "project-rules.md"))
	text := string(data)
	for _, phrase := range []string{"--kind design", "--kind read", "--kind critique", "metasystem launch round-task"} {
		if err != nil || !strings.Contains(text, phrase) {
			t.Fatalf("project rules missing %q: %v", phrase, err)
		}
	}
}

func TestDesignCommonTemplateContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "scripts", "agents", "templates", "design-common.md"))
	text := string(data)
	for _, phrase := range []string{"defaults: 60 tool calls, 4000 words", "A launching brief may override these defaults", "read the page that supersedes", "say which unit is first", "the last message is exactly one line"} {
		if err != nil || !strings.Contains(text, phrase) {
			t.Fatalf("design template missing %q: %v", phrase, err)
		}
	}
}

func TestLandingLaneFixtureScenariosHaveGoWitness(t *testing.T) {
	root := moduleRoot(t)
	temp := t.TempDir()
	steps := filepath.Join(temp, "steps")
	os.MkdirAll(steps, 0o700)
	logPath := filepath.Join(temp, "lock.log")
	lockPath := filepath.Join(temp, "lock.sh")
	lock := `
cat >"$STEP_DIR/step-1.sh" <<'STEP'
#!/bin/bash
cat >/dev/null
echo 1 >>"$LOCK_LOG"
touch "$STEP_DIR/quit"
STEP
cat >"$STEP_DIR/step-2.sh" <<'STEP'
#!/bin/bash
echo 2 >>"$LOCK_LOG"
STEP
cat >"$STEP_DIR/step-10.sh" <<'STEP'
#!/bin/bash
echo 10 >>"$LOCK_LOG"
STEP
cat >"$STEP_DIR/step-3-backup.sh" <<'STEP'
#!/bin/bash
echo bad >>"$LOCK_LOG"
STEP
testrun_lock_acquire() {
  printf 'lock seat=%s\n' "$1" >>"$LOCK_LOG"
  [ "${LOCK_FAIL:-0}" != 1 ]
}
testrun_lock_release() {
  echo release >>"$LOCK_LOG"
}
`
	if err := os.WriteFile(lockPath, []byte(lock), 0o600); err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(root, "scripts", "agents", "landing-lane-worker.sh")
	run := func(fail bool) error {
		command := exec.Command("bash", worker, "lane", "--dir", steps, "--lock-lib", lockPath, "--seat", "builder", "--idle", "3600", "--poll", "0", "--repo", root)
		command.Dir = temp
		command.Env = append(os.Environ(), "STEP_DIR="+steps, "LOCK_LOG="+logPath)
		if fail {
			command.Env = append(command.Env, "LOCK_FAIL=1")
		}
		return command.Run()
	}
	os.WriteFile(logPath, nil, 0o600)
	if err := run(false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(logPath)
	if string(data) != "lock seat=builder\n1\n2\n10\nrelease\n" {
		t.Fatalf("ordered execution log=%q", data)
	}
	os.WriteFile(logPath, nil, 0o600)
	if err := run(true); err == nil {
		t.Fatal("lock failure was accepted")
	}
	data, _ = os.ReadFile(logPath)
	if string(data) != "lock seat=builder\nrelease\n" {
		t.Fatalf("lock failure log=%q", data)
	}
}
