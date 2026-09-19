package launch

import (
	"encoding/json"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeProber struct{ states map[int64]identity.Liveness }

func (p *fakeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, found := p.states[pid]
	if !found {
		state = identity.Dead
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, state, nil
}

type fakeChild struct{ exit int }

func (child fakeChild) Wait() (int, error) { return child.exit, nil }

type fakeProcesses struct {
	probe       *fakeProber
	self, child identity.Ref
	exit        int
	group       bool
	signals     []syscall.Signal
}

func (p *fakeProcesses) SelfRef() (identity.Ref, error) { return p.self, nil }
func (p *fakeProcesses) StartChild(Command) (Child, identity.Ref, error) {
	return fakeChild{p.exit}, p.child, nil
}
func (p *fakeProcesses) SignalGroup(_ int64, signal syscall.Signal) error {
	p.signals = append(p.signals, signal)
	if signal == syscall.SIGKILL {
		p.group = false
		for pid := range p.probe.states {
			p.probe.states[pid] = identity.Dead
		}
	}
	return nil
}
func (p *fakeProcesses) GroupAlive(int64) (bool, error) { return p.group, nil }

type fakeAdapter struct{}

func (a fakeAdapter) Command(record Record, state string) (Command, error) {
	return Command{LogPath: filepath.Join(state, "log")}, nil
}
func (a fakeAdapter) Measure(Record, string) (Measurement, []Output, map[string]json.RawMessage, error) {
	return Measurement{}, nil, nil, nil
}
func (a fakeAdapter) Strays() ([]string, error) { return nil, nil }

type fakeStarter struct{ start func(string) }

func (s fakeStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	s.start(id)
	return ref(10), nil
}
func ref(pid int64) identity.Ref { return identity.Ref{Pid: pid, StartedAtSec: pid} }
func childStarter(m *Manager) fakeStarter {
	return fakeStarter{func(id string) {
		m.Store.Update(id, func(r *Record) error {
			supervisor, child := ref(10), ref(20)
			r.Supervisor, r.Child, r.ProcessGroup, r.State = &supervisor, &child, &child, Running
			return nil
		})
	}}
}
func deathSleep(p *fakeProcesses, probe *fakeProber) func(time.Duration) {
	return func(time.Duration) {
		p.group = false
		probe.states[10], probe.states[20] = identity.Dead, identity.Dead
	}
}
func require(t *testing.T, failed bool, format string, values ...any) {
	t.Helper()
	if failed {
		t.Fatalf(format, values...)
	}
}
func manager(t *testing.T) (*Manager, *fakeProcesses, *fakeProber, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	probe := &fakeProber{states: map[int64]identity.Liveness{10: identity.Alive, 20: identity.Alive}}
	processes := &fakeProcesses{probe: probe, self: ref(10), child: ref(20)}
	templates := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(templates, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"design-brief.md", "review-brief.md"} {
		if err := os.WriteFile(filepath.Join(templates, name), []byte("filled fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := &Manager{Store: Store{Root: t.TempDir()}, Adapters: map[string]Adapter{"codex-exec": fakeAdapter{}}, Processes: processes, Prober: probe, Now: func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d) }, Grace: time.Second, Poll: time.Second}
	field := reflect.ValueOf(m).Elem().FieldByName("TemplateDirectory")
	if field.IsValid() {
		field.SetString(templates)
	}
	return m, processes, probe, &now
}
func brief(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(path, []byte("| Unit | Lines |\n|---|---|\n| fixture | 1 |\n"), 0o600)
	return path
}
func seed(t *testing.T, m *Manager, id string, state State) Record {
	t.Helper()
	record := Record{ID: id, Kind: "build", Adapter: "codex-exec", WorkingDirectory: t.TempDir(), State: state, StartedAt: m.Now().Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{}}
	if state == Running {
		supervisor, child := ref(10), ref(20)
		record.Supervisor, record.Child, record.ProcessGroup = &supervisor, &child, &child
	}
	if err := m.Store.Create(record); err != nil {
		t.Fatal(err)
	}
	return record
}
func TestLaunchStartReturnsAfterTheChildIsRecorded(t *testing.T) {
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	record, err := m.Start(StartSpec{ID: "start-recorded", Kind: "build", Brief: brief(t), WorkingDirectory: t.TempDir()})
	require(t, err != nil || record.Child == nil || record.State != Running, "start returned before child record: %+v err=%v", record, err)
}
func TestLaunchStartFailureLeavesNoProcess(t *testing.T) {
	m, processes, probe, _ := manager(t)
	m.Supervisor = fakeStarter{func(id string) {
		m.Store.Update(id, func(r *Record) error {
			supervisor := ref(10)
			r.Supervisor, r.State, r.Reason = &supervisor, Failed, "child-start: missing executable"
			return nil
		})
		probe.states[10] = identity.Dead
	}}
	record, err := m.Start(StartSpec{ID: "start-failed", Kind: "build", Brief: brief(t), WorkingDirectory: t.TempDir()})
	require(t, err == nil || record.State != Failed || record.Child != nil || processes.group, "failed start left process: %+v err=%v", record, err)
}
func TestSuperviseRecordsTheTerminalStateFromTheExitStatus(t *testing.T) {
	for _, row := range []struct {
		exit int
		want State
	}{{0, Completed}, {7, Failed}} {
		m, p, _, _ := manager(t)
		p.exit = row.exit
		seed(t, m, "exit-"+string(rune('a'+row.exit)), Starting)
		got, err := m.Supervise("exit-" + string(rune('a'+row.exit)))
		require(t, err != nil || got.State != row.want || got.ExitCode == nil || *got.ExitCode != row.exit, "exit %d: %+v err=%v", row.exit, got, err)
	}
}
func TestSuperviseEndsTheGroupAfterTheChild(t *testing.T) {
	m, p, _, _ := manager(t)
	p.group = true
	seed(t, m, "group-end", Starting)
	_, err := m.Supervise("group-end")
	require(t, err != nil || p.group || !reflect.DeepEqual(p.signals, []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}), "group=%v signals=%v err=%v", p.group, p.signals, err)
}
func TestCancelProvesDeathBeforeItReportsCancelled(t *testing.T) {
	m, p, probe, _ := manager(t)
	p.group = true
	seed(t, m, "cancel-proof", Running)
	checked := false
	m.Sleep = func(time.Duration) {
		record, _ := m.Store.Read("cancel-proof")
		if len(p.signals) == 1 {
			checked = record.State == Running
		}
		if len(p.signals) > 1 {
			p.group = false
			probe.states[10], probe.states[20] = identity.Dead, identity.Dead
		}
	}
	record, err := m.Cancel("cancel-proof")
	require(t, err != nil || !checked || record.State != Cancelled, "cancel=%+v checked=%v err=%v", record, checked, err)
}
func TestCancelRightAfterStartFindsTheChild(t *testing.T) {
	m, p, probe, _ := manager(t)
	p.group = true
	m.Supervisor = childStarter(m)
	record, err := m.Start(StartSpec{ID: "start-cancel", Kind: "build", Brief: brief(t), WorkingDirectory: t.TempDir()})
	if err != nil || record.Child == nil {
		t.Fatal(err)
	}
	m.Sleep = deathSleep(p, probe)
	record, err = m.Cancel(record.ID)
	require(t, err != nil || record.State != Cancelled, "cancel=%+v err=%v", record, err)
}
func TestCancelResolvesTheIdFromAnyDirectory(t *testing.T) {
	m, p, probe, _ := manager(t)
	p.group = true
	seed(t, m, "elsewhere", Running)
	old, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(old)
	m.Sleep = deathSleep(p, probe)
	got, err := m.Cancel("elsewhere")
	require(t, err != nil || got.State != Cancelled, "cancel=%+v err=%v", got, err)
}
func TestTerminalStateIsFinal(t *testing.T) {
	m, _, _, _ := manager(t)
	record := seed(t, m, "final", Completed)
	code := 0
	m.Store.Update(record.ID, func(r *Record) error { r.ExitCode = &code; return nil })
	got, err := m.Cancel(record.ID)
	require(t, err != nil || got.State != Completed, "cancel changed terminal: %+v %v", got, err)
	_, err = m.Supervise(record.ID)
	require(t, err == nil || !strings.Contains(err.Error(), "LAUNCH_ALREADY_SUPERVISED"), "second supervisor err=%v", err)
}
func TestWaitHonoursTheCapAndReportsTheTerminalState(t *testing.T) {
	m, _, _, now := manager(t)
	seed(t, m, "wait", Running)
	got, terminal, err := m.Wait("wait", 3*time.Second)
	require(t, err != nil || terminal || now.Sub(time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)) != 3*time.Second || DefaultWaitTimeout != 240*time.Second, "wait=%+v terminal=%v now=%s err=%v", got, terminal, now, err)
	m.Store.Update("wait", func(r *Record) error { r.State = Failed; return nil })
	got, terminal, err = m.Wait("wait", 0)
	require(t, err != nil || !terminal || got.State != Failed, "terminal wait=%+v %v %v", got, terminal, err)
}
func TestWaitCapIsTheValueWaitClampsTo(t *testing.T) {
	t.Parallel()
	m, _, _, now := manager(t)
	start := *now
	m.Settings = Settings{WaitCapSeconds: 2, Values: []Setting{{Key: WaitCapKey, Value: "2", Source: "fixture"}}}
	seed(t, m, "capped", Running)
	cap, err := m.WaitCap()
	require(t, err != nil || cap != 2*time.Second, "cap=%s err=%v", cap, err)
	_, terminal, err := m.Wait("capped", time.Hour)
	require(t, err != nil || terminal || now.Sub(start) != 2*time.Second, "terminal=%v elapsed=%s err=%v", terminal, now.Sub(start), err)
}
func TestStatusReconcilesALostSupervisor(t *testing.T) {
	m, _, probe, _ := manager(t)
	seed(t, m, "lost", Running)
	probe.states[10], probe.states[20] = identity.Dead, identity.Dead
	got, err := m.Status("lost")
	require(t, err != nil || got.State != Failed || got.Reason != "supervisor-lost", "status=%+v err=%v", got, err)
}

type scan []Process

func (s scan) Scan() ([]Process, error) { return s, nil }

type recordingSignaler struct {
	pids    []int64
	signals []syscall.Signal
	err     error
}

func (s *recordingSignaler) Signal(pid int64, signal syscall.Signal) error {
	s.pids = append(s.pids, pid)
	s.signals = append(s.signals, signal)
	return s.err
}

func TestCensusReapOnlySignalsBrokersWithoutAnyCompanion(t *testing.T) {
	t.Parallel()
	m, _, probe, _ := manager(t)
	processes := scan{
		{Ref: ref(40), Argv: []string{"node", "/p/app-server-broker.mjs", "serve", "--cwd", "/idle"}},
		{Ref: ref(41), Argv: []string{"node", "/p/app-server-broker.mjs", "serve", "--cwd", "/worker"}},
		{Ref: ref(42), Argv: []string{"node", "/p/app-server-broker.mjs", "serve", "--cwd", "/foreground"}},
		{Ref: ref(43), Argv: []string{"node", "/p/codex-companion.mjs", "task-worker", "--cwd", "/worker"}},
		{Ref: ref(44), Argv: []string{"node", "/p/codex-companion.mjs", "task", "--cwd", "/foreground"}},
	}
	for pid := int64(40); pid <= 44; pid++ {
		probe.states[pid] = identity.Alive
	}
	signaler := &recordingSignaler{}
	m.Signaler = signaler
	m.Adapters["codex-exec"] = CodexExec{Scanner: processes}
	lines, err := m.Census(true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(signaler.pids, []int64{40}) || !slices.Equal(signaler.signals, []syscall.Signal{syscall.SIGTERM}) {
		t.Fatalf("signalled pids=%v signals=%v", signaler.pids, signaler.signals)
	}
	if got := strings.Join(lines, "\n"); got != "reaped idle-plugin-broker pid=40 cwd=/idle" {
		t.Fatalf("reap output=%q", got)
	}
}

type changedIdentityProber struct{}

func (changedIdentityProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid+1, 0)}, identity.Alive, nil
}

func TestCensusReapSkipsChangedIdentity(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	signaler := &recordingSignaler{}
	m.Prober = changedIdentityProber{}
	m.Signaler = signaler
	m.Adapters["codex-exec"] = CodexExec{Scanner: scan{{Ref: ref(50), Argv: []string{"node", "app-server-broker.mjs", "serve", "--cwd", "/moved"}}}}
	lines, err := m.Census(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(signaler.pids) != 0 || !slices.Equal(lines, []string{"skipped idle-plugin-broker pid=50: identity changed"}) {
		t.Fatalf("signals=%v lines=%v", signaler.pids, lines)
	}
}

func TestCensusWithoutReapSignalsNothing(t *testing.T) {
	t.Parallel()
	m, _, probe, _ := manager(t)
	probe.states[60], probe.states[61], probe.states[62] = identity.Alive, identity.Alive, identity.Alive
	signaler := &recordingSignaler{}
	m.Signaler = signaler
	m.Adapters["codex-exec"] = CodexExec{Scanner: scan{
		{Ref: ref(60), Argv: []string{"node", "app-server-broker.mjs", "serve", "--cwd", "/foreground"}},
		{Ref: ref(61), Argv: []string{"node", "codex-companion.mjs", "task", "--cwd", "/foreground"}},
		{Ref: ref(62), Argv: []string{"node", "app-server-broker.mjs", "serve", "--cwd", "/idle"}},
	}}
	lines, err := m.Census()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"idle-plugin-broker pid=60 cwd=/foreground", "idle-plugin-broker pid=62 cwd=/idle"}
	if len(signaler.pids) != 0 || !slices.Equal(lines, want) {
		t.Fatalf("signals=%v lines=%v", signaler.pids, lines)
	}
}

func TestCensusReapNamesSignalFailure(t *testing.T) {
	t.Parallel()
	m, _, probe, _ := manager(t)
	probe.states[70] = identity.Alive
	signaler := &recordingSignaler{err: errors.New("permission denied")}
	m.Signaler = signaler
	m.Adapters["codex-exec"] = CodexExec{Scanner: scan{{Ref: ref(70), Argv: []string{"node", "app-server-broker.mjs", "serve", "--cwd", "/idle"}}}}
	_, err := m.Census(true)
	if err == nil || !strings.Contains(err.Error(), "pid 70") || len(signaler.pids) != 1 {
		t.Fatalf("signals=%v err=%v", signaler.pids, err)
	}
}

func TestCensusNamesOrphansAndIdleBrokers(t *testing.T) {
	m, p, probe, _ := manager(t)
	seed(t, m, "orphan", Running)
	probe.states[10], probe.states[20] = identity.Dead, identity.Dead
	terminal := seed(t, m, "terminal-live", Completed)
	supervisor := ref(30)
	terminal.Supervisor = &supervisor
	m.Store.Update(terminal.ID, func(r *Record) error { r.Supervisor = &supervisor; return nil })
	probe.states[30] = identity.Alive
	p.group = false
	m.Adapters["codex-exec"] = CodexExec{Scanner: scan{{Ref: ref(40), Argv: []string{"node", "/p/app-server-broker.mjs", "serve", "--cwd", "/idle"}}, {Ref: ref(41), Argv: []string{"node", "/p/app-server-broker.mjs", "serve", "--cwd", "/busy"}}, {Ref: ref(42), Argv: []string{"node", "/p/codex-companion.mjs", "task-worker", "--cwd", "/busy", "--job-id", "j"}}}}
	lines, err := m.Census()
	joined := strings.Join(lines, "\n")
	require(t, err != nil || !strings.Contains(joined, "orphan-running id=orphan") || !strings.Contains(joined, "live-terminal id=terminal-live") || !strings.Contains(joined, "cwd=/idle") || strings.Contains(joined, "cwd=/busy"), "census=%q err=%v", joined, err)
}
func TestCodexExecCommand(t *testing.T) {
	root := t.TempDir()
	briefPath := filepath.Join(root, "brief")
	os.WriteFile(briefPath, []byte("do it"), 0o600)
	data := map[string]json.RawMessage{}
	setString(data, "brief", briefPath)
	setString(data, "model", "m")
	setString(data, "effort", "high")
	setInt64(data, "window", 200000)
	command, err := (CodexExec{Binary: "fake-codex"}).Command(Record{Kind: "build", WorkingDirectory: root, AdapterData: data}, root)
	want := []string{"exec", "-m", "m", "-c", "model_reasoning_effort=high", "-C", root, "-s", "workspace-write", "-o", filepath.Join(root, "last-message.txt"), "-c", "model_auto_compact_token_limit=200000", "-"}
	require(t, err != nil || command.Program != "fake-codex" || command.Stdin != "do it" || !reflect.DeepEqual(command.Args, want), "command=%+v err=%v", command, err)
}
func TestCodexMeasureReadsTheRolloutFile(t *testing.T) {
	state, sessions := t.TempDir(), t.TempDir()
	id := "abc-123"
	os.WriteFile(filepath.Join(state, "exec.log"), []byte("session id: "+id+"\n"), 0o600)
	os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("done now\n"), 0o600)
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	dir := filepath.Join(sessions, "2026", "09", "17")
	os.MkdirAll(dir, 0o700)
	rows := strings.Join([]string{
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100000},"total_token_usage":{"input_tokens":100,"cached_input_tokens":200,"cache_write_input_tokens":30,"output_tokens":40}}}}`,
		`{"type":"response_item","payload":{"type":"custom_tool_call"}}`,
		`{"type":"response_item","payload":{"type":"custom_tool_call"}}`,
		`{"type":"response_item","payload":{"type":"function_call"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":210001},"total_token_usage":{"input_tokens":1000,"cached_input_tokens":2000,"cache_write_input_tokens":300,"output_tokens":400}}}}`,
		`{"type":"compacted","payload":{}}`,
	}, "\n") + "\n"
	os.WriteFile(filepath.Join(dir, "rollout-x-"+id+".jsonl"), []byte(rows), 0o600)
	got, _, _, err := (CodexExec{SessionsRoot: sessions, Now: func() time.Time { return day }}).Measure(Record{Kind: "build"}, state)
	usage := measurementJSON(t, got)
	require(t, err != nil || got.Calls != 2 || usage.ToolCalls != 3 || usage.InputTokens != 1000 ||
		usage.CacheReadTokens != 2000 || usage.CacheCreationTokens != 300 || usage.OutputTokens != 400 ||
		got.Compactions != 1 || got.PeakContext != 210001 || got.CallsAbove200 != 1,
		"measurement=%+v err=%v", got, err)
}

func TestClaudeMeasureSumsUsageAndCountsToolCalls(t *testing.T) {
	rows := strings.Join([]string{
		`{"type":"assistant","message":{"id":"a","content":[{"type":"text","text":"working"}],"usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":30,"output_tokens":40}}}`,
		`{"type":"assistant","message":{"id":"a","content":[{"type":"tool_use","name":"Read"}],"usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":30,"output_tokens":40}}}`,
		`{"type":"assistant","message":{"id":"b","content":[{"type":"tool_use","name":"Write"}],"usage":{"input_tokens":1,"cache_read_input_tokens":2,"cache_creation_input_tokens":3,"output_tokens":4}}}`,
	}, "\n") + "\n"
	got, _, err := measureFixture(t, rows)
	usage := measurementJSON(t, got)
	require(t, err != nil || got.Calls != 2 || usage.ToolCalls != 2 || usage.InputTokens != 11 ||
		usage.CacheReadTokens != 22 || usage.CacheCreationTokens != 33 || usage.OutputTokens != 44,
		"measurement=%+v err=%v", got, err)
}

type measuredUsage struct {
	ToolCalls           int   `json:"toolCalls"`
	InputTokens         int64 `json:"inputTokens"`
	CacheReadTokens     int64 `json:"cacheReadTokens"`
	CacheCreationTokens int64 `json:"cacheCreationTokens"`
	OutputTokens        int64 `json:"outputTokens"`
}

func measurementJSON(t *testing.T, measurement Measurement) measuredUsage {
	t.Helper()
	data, err := json.Marshal(measurement)
	if err != nil {
		t.Fatal(err)
	}
	var values measuredUsage
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	return values
}
func TestCoreRecordNamesNoRuntime(t *testing.T) {
	data, _ := json.Marshal(Record{})
	var values map[string]any
	json.Unmarshal(data, &values)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	expected := []string{"adapter", "adapterData", "child", "exitCode", "finishedAt", "goal", "id", "inputs", "kind", "measured", "measurement", "outputs", "processGroup", "reason", "startedAt", "state", "supervisor", "tag", "workingDirectory"}
	slices.Sort(keys)
	require(t, !reflect.DeepEqual(keys, expected) || strings.Contains(string(data), "codex") || strings.Contains(string(data), "claude"), "record keys=%v json=%s", keys, data)
}
func TestCritiqueCopiesTheReportAndRecordsTheExit(t *testing.T) {
	worktree, source, state := t.TempDir(), t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(worktree, "metasystem"), 0o700)
	task, page, design, common := filepath.Join(source, "task"), filepath.Join(source, "page"), filepath.Join(source, "design"), filepath.Join(source, "common")
	for _, path := range []string{task, page, design, common} {
		os.WriteFile(path, []byte(filepath.Base(path)), 0o600)
	}
	data := map[string]json.RawMessage{}
	setString(data, "brief", task)
	setInt64(data, "window", 200000)
	record := Record{Kind: "critique", Tag: "tag", WorkingDirectory: worktree, Inputs: []Input{{Path: task}, {Path: page}, {Path: design}}, AdapterData: data}
	adapter := CodexExec{CommonTemplate: common}
	command, err := adapter.Command(record, state)
	if err != nil || command.Directory != filepath.Join(worktree, "metasystem") {
		t.Fatal(err)
	}
	reportDir := filepath.Join(worktree, "metasystem", "artifacts", "reports")
	os.WriteFile(filepath.Join(reportDir, "tag-critique-r1.md"), []byte("material: yes\nVERDICT: revise\n"), 0o600)
	id := "crit"
	os.WriteFile(filepath.Join(state, "exec.log"), []byte("session id: "+id+"\n"), 0o600)
	sessions := t.TempDir()
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	rolloutDir := filepath.Join(sessions, "2026", "09", "17")
	os.MkdirAll(rolloutDir, 0o700)
	os.WriteFile(filepath.Join(rolloutDir, "rollout-"+id+".jsonl"), nil, 0o600)
	adapter.SessionsRoot, adapter.Now = sessions, func() time.Time { return day }
	measurement, outputs, _, err := adapter.Measure(record, state)
	require(t, err != nil || measurement.MaterialCount != 1 || measurement.Verdict != "VERDICT: revise" || len(outputs) != 1 || !strings.HasPrefix(outputs[0].Path, state+string(os.PathSeparator)), "measure=%+v outputs=%+v err=%v", measurement, outputs, err)
}
func TestLaunchOutputsStayUnderTheStateDirectory(t *testing.T) {
	m, _, _, _ := manager(t)
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "result.txt")
	os.WriteFile(source, []byte("result"), 0o600)
	record := seed(t, m, "outputs", Starting)
	setStrings(record.AdapterData, "declaredOutputs", []string{source})
	m.Store.Update(record.ID, func(r *Record) error { r.AdapterData = record.AdapterData; return nil })
	got, err := m.Supervise(record.ID)
	state, _ := m.Store.StateDir(record.ID)
	require(t, err != nil || len(got.Outputs) != 1 || !strings.HasPrefix(got.Outputs[0].Path, state+string(os.PathSeparator)) || strings.Contains(got.Outputs[0].Path, sourceDir), "outputs=%+v err=%v", got.Outputs, err)
}
