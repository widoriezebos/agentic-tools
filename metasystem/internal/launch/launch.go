package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const DefaultWaitTimeout = 240 * time.Second

type Command struct {
	Program, Directory, Stdin, LogPath, StdoutPath string
	Args, Environment                              []string
}
type Adapter interface {
	Command(Record, string) (Command, error)
	Measure(Record, string) (Measurement, []Output, map[string]json.RawMessage, error)
	Strays() ([]string, error)
}
type Child interface{ Wait() (int, error) }
type ProcessSystem interface {
	SelfRef() (identity.Ref, error)
	StartChild(Command) (Child, identity.Ref, error)
	SignalGroup(int64, syscall.Signal) error
	GroupAlive(int64) (bool, error)
}
type SupervisorStarter interface {
	StartSupervisor(id, stateDir string) (identity.Ref, error)
}
type StartSpec struct {
	ID, Kind, Goal, Tag, WorkingDirectory, Brief, Page string
	Model, Effort, UnitsPage, DiffFile, Package        string
	Wide                                               bool
	Inputs, Outputs, Units                             []string
	AdapterData                                        map[string]json.RawMessage
}
type Manager struct {
	Store             Store
	Adapters          map[string]Adapter
	TemplateDirectory string
	Processes         ProcessSystem
	Prober            identity.Prober
	Supervisor        SupervisorStarter
	Now               func() time.Time
	Sleep             func(time.Duration)
	Grace             time.Duration
	Poll              time.Duration
	StartCap          time.Duration
	Settings          Settings
	SettingsError     error
}

func (m *Manager) resolvedSettings() (Settings, error) {
	if m.SettingsError != nil {
		return Settings{}, m.SettingsError
	}
	if len(m.Settings.Values) == 0 {
		return DefaultSettings(), nil
	}
	return m.Settings, nil
}

func (m *Manager) Start(spec StartSpec) (Record, error) {
	if m.Supervisor == nil {
		return Record{}, errors.New("launch supervisor is unavailable")
	}
	if spec.WorkingDirectory == "" {
		return Record{}, errors.New("launch working directory is required")
	}
	if err := m.Admit(spec); err != nil {
		return Record{}, err
	}
	settings, _ := m.resolvedSettings()
	model, effort, window := settings.launchValues(spec.Kind)
	if value := readString(spec.AdapterData, "model"); value != "" {
		model = value
	}
	if value := readString(spec.AdapterData, "effort"); value != "" {
		effort = value
	}
	if spec.Model != "" {
		model = spec.Model
	}
	if spec.Effort != "" {
		effort = spec.Effort
	}
	adapterName := adapterForKind(spec.Kind, model)
	if adapterName == "" {
		return Record{}, fmt.Errorf("launch kind %q is not available", spec.Kind)
	}
	if spec.Kind == "design" && m.Adapters[adapterName] == nil {
		return Record{}, errors.New("adapter-unavailable")
	}
	absDir, err := filepath.Abs(spec.WorkingDirectory)
	if err != nil {
		return Record{}, err
	}
	id := spec.ID
	if id == "" {
		id, err = newID(m.Now())
		if err != nil {
			return Record{}, err
		}
	}
	inputs, err := measuredInputs(append([]string{spec.Brief}, spec.Inputs...))
	if err != nil {
		return Record{}, err
	}
	if inputs[0].Bytes == 0 {
		return Record{}, errors.New("launch brief is empty")
	}
	record := Record{ID: id, Kind: spec.Kind, Adapter: adapterName, Goal: spec.Goal, Tag: spec.Tag,
		WorkingDirectory: absDir, Inputs: inputs, State: Starting, StartedAt: m.Now().UTC().Format(time.RFC3339Nano),
		AdapterData: spec.AdapterData}
	data := map[string]json.RawMessage{}
	for key, value := range record.AdapterData {
		data[key] = value
	}
	record.AdapterData = data
	setString(record.AdapterData, "brief", inputs[0].Path)
	setStrings(record.AdapterData, "declaredOutputs", spec.Outputs)
	setString(record.AdapterData, "model", model)
	setString(record.AdapterData, "effort", effort)
	setInt64(record.AdapterData, "window", window)
	if spec.Page != "" {
		setString(record.AdapterData, "page", spec.Page)
	}
	if spec.Kind == "build" {
		_, record.DeclaredLines, err = buildSize(spec)
		if err != nil {
			return Record{}, err
		}
	}
	var diff []byte
	if spec.Kind == "read" && spec.DiffFile != "" {
		choice, choiceErr := ChooseReadMode(spec.DiffFile, settings.ReadSplitLines)
		if choiceErr != nil {
			return Record{}, choiceErr
		}
		record.ReadMode, record.ReadPackage = choice.Mode, spec.Package
		diff, record.ChangedLines, err = readDiff(spec.DiffFile, choice.Mode, spec.Package)
		if err != nil {
			return Record{}, err
		}
	}
	if err := m.Store.Create(record); err != nil {
		return Record{}, err
	}
	stateDir, _ := m.Store.StateDir(id)
	if diff != nil {
		path := filepath.Join(stateDir, "read.diff")
		if _, err := atomicfile.WriteText(path, string(diff), filepath.Dir(stateDir)); err != nil {
			return Record{}, err
		}
		setString(record.AdapterData, "readDiff", path)
		record, err = m.Store.Update(id, func(current *Record) error { current.AdapterData = record.AdapterData; return nil })
		if err != nil {
			return Record{}, err
		}
	}
	supervisor, err := m.Supervisor.StartSupervisor(id, stateDir)
	if err != nil {
		failed, writeErr := m.fail(id, "supervisor-start: "+err.Error(), nil)
		if writeErr != nil {
			return Record{}, writeErr
		}
		return failed, err
	}
	deadline := m.Now().Add(m.StartCap)
	for {
		current, readErr := m.Store.Read(id)
		if readErr != nil {
			return Record{}, readErr
		}
		if current.Child != nil || current.State.Terminal() && (m.Prober == nil || identity.AliveRef(m.Prober, supervisor) == identity.Dead) {
			if current.State == Failed && current.Child == nil {
				return current, errors.New(current.Reason)
			}
			return current, nil
		}
		if m.Prober != nil && identity.AliveRef(m.Prober, supervisor) == identity.Dead {
			failed, writeErr := m.fail(id, "supervisor-lost-before-child", nil)
			if writeErr != nil {
				return Record{}, writeErr
			}
			return failed, errors.New(failed.Reason)
		}
		if !m.Now().Before(deadline) {
			_ = m.endGroup(supervisor.Pid)
			failed, writeErr := m.fail(id, "supervisor-ready-timeout", nil)
			if writeErr != nil {
				return Record{}, writeErr
			}
			return failed, errors.New(failed.Reason)
		}
		m.Sleep(m.Poll)
	}
}
func measuredInputs(paths []string) ([]Input, error) {
	inputs := make([]Input, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, Input{Path: absolute, Bytes: info.Size()})
	}
	return inputs, nil
}
func (m *Manager) Supervise(id string) (Record, error) {
	if m.Processes == nil {
		return Record{}, errors.New("launch process system is unavailable")
	}
	self, err := m.Processes.SelfRef()
	if err != nil {
		return Record{}, err
	}
	record, err := m.Store.Update(id, func(record *Record) error {
		if record.Supervisor != nil || record.State.Terminal() {
			return fmt.Errorf("LAUNCH_ALREADY_SUPERVISED: launch %s already has a supervisor", id)
		}
		record.Supervisor = &self
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	adapter := m.Adapters[record.Adapter]
	if adapter == nil {
		return m.failCause(id, "adapter-unavailable", nil)
	}
	stateDir, _ := m.Store.StateDir(id)
	command, err := adapter.Command(record, stateDir)
	if err != nil {
		return m.failCause(id, "command: "+err.Error(), nil)
	}
	child, childRef, err := m.Processes.StartChild(command)
	if err != nil {
		failed, writeErr := m.fail(id, "child-start: "+err.Error(), nil)
		if writeErr != nil {
			return Record{}, writeErr
		}
		return failed, err
	}
	record, err = m.Store.Update(id, func(record *Record) error {
		if record.State.Terminal() {
			return fmt.Errorf("launch %s ended before its child was recorded", id)
		}
		record.Child = &childRef
		record.ProcessGroup = &childRef
		record.State = Running
		return nil
	})
	if err != nil {
		_ = m.Processes.SignalGroup(childRef.Pid, syscall.SIGKILL)
		return Record{}, err
	}
	exitCode, waitErr := child.Wait()
	latest, _ := m.Store.Read(id)
	var measurement Measurement
	var outputs []Output
	var adapterData map[string]json.RawMessage
	var measureErr, copyErr error
	if latest.Reason != "cancel-requested" {
		measurement, outputs, adapterData, measureErr = adapter.Measure(record, stateDir)
		var declared []Output
		declared, copyErr = copyDeclaredOutputs(record, stateDir)
		outputs = append(outputs, declared...)
	}
	if cleanupErr := m.endGroup(childRef.Pid); cleanupErr != nil {
		return record, cleanupErr
	}
	record, err = m.Store.Update(id, func(record *Record) error {
		if record.State.Terminal() {
			return nil
		}
		if record.Reason == "cancel-requested" {
			return nil
		}
		record.ExitCode = &exitCode
		record.FinishedAt = m.Now().UTC().Format(time.RFC3339Nano)
		record.Measurement, record.Outputs = measurement, outputs
		if record.Kind == "read" {
			counts := measurement.Compactions == 0
			record.VerdictCounts = &counts
		}
		for key, value := range adapterData {
			record.AdapterData[key] = value
		}
		record.State, record.Reason = adapterOutcome(adapter, exitCode, measureErr)
		if record.Reason == "" {
			for _, problem := range []error{waitErr, measureErr, copyErr} {
				if problem != nil {
					record.Reason = problem.Error()
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return record, nil
}
func (m *Manager) fail(id, reason string, exit *int) (Record, error) {
	return m.Store.Update(id, func(record *Record) error {
		if record.State.Terminal() {
			return nil
		}
		record.State, record.Reason, record.ExitCode = Failed, reason, exit
		record.FinishedAt = m.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
}
func (m *Manager) failCause(id, reason string, exit *int) (Record, error) {
	record, err := m.fail(id, reason, exit)
	if err != nil {
		return record, err
	}
	return record, errors.New(reason)
}
func (m *Manager) endGroup(pgid int64) error {
	alive, err := m.Processes.GroupAlive(pgid)
	if err != nil || !alive {
		return err
	}
	_ = m.Processes.SignalGroup(pgid, syscall.SIGTERM)
	m.Sleep(m.Grace)
	alive, err = m.Processes.GroupAlive(pgid)
	if err != nil {
		return err
	}
	if alive {
		_ = m.Processes.SignalGroup(pgid, syscall.SIGKILL)
		m.Sleep(m.Grace)
		alive, err = m.Processes.GroupAlive(pgid)
	}
	if err != nil {
		return err
	}
	if alive {
		return fmt.Errorf("process group %d remains alive after KILL", pgid)
	}
	return nil
}
func (m *Manager) WaitCap() (time.Duration, error) {
	settings, err := m.resolvedSettings()
	if err != nil {
		return 0, err
	}
	return time.Duration(settings.WaitCapSeconds) * time.Second, nil
}
func (m *Manager) Wait(id string, timeout time.Duration) (Record, bool, error) {
	if timeout < 0 {
		return Record{}, false, errors.New("wait timeout must not be negative")
	}
	cap, err := m.WaitCap()
	if err != nil {
		return Record{}, false, err
	}
	if timeout > cap {
		timeout = cap
	}
	deadline := m.Now().Add(timeout)
	for {
		record, err := m.Status(id)
		if err != nil {
			return Record{}, false, err
		}
		if record.State.Terminal() {
			return record, true, nil
		}
		if !m.Now().Before(deadline) {
			return record, false, nil
		}
		m.Sleep(m.Poll)
	}
}
func (m *Manager) Status(id string) (Record, error) {
	record, err := m.Store.Read(id)
	if err != nil {
		return Record{}, err
	}
	if record.State == Running && m.Prober != nil && refDead(m.Prober, record.Supervisor) && refDead(m.Prober, record.Child) {
		return m.fail(id, "supervisor-lost", nil)
	}
	return record, nil
}
func (m *Manager) List() ([]Record, error) {
	records, err := m.Store.List()
	if err != nil {
		return nil, err
	}
	for index := range records {
		records[index], err = m.Status(records[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return records, nil
}
func (m *Manager) Cancel(id string) (Record, error) {
	record, err := m.Store.Update(id, func(record *Record) error {
		if !record.State.Terminal() {
			record.Reason = "cancel-requested"
		}
		return nil
	})
	if err != nil || record.State.Terminal() {
		return record, err
	}
	if record.ProcessGroup != nil {
		_ = m.Processes.SignalGroup(record.ProcessGroup.Pid, syscall.SIGTERM)
	}
	m.Sleep(m.Grace)
	if !m.provenDead(record) && record.ProcessGroup != nil {
		_ = m.Processes.SignalGroup(record.ProcessGroup.Pid, syscall.SIGKILL)
		m.Sleep(m.Grace)
	}
	current, err := m.Store.Read(id)
	if err != nil {
		return Record{}, err
	}
	if current.State.Terminal() {
		return current, nil
	}
	if !m.provenDead(current) {
		return current, fmt.Errorf("launch %s cancellation could not prove every recorded process dead", id)
	}
	return m.Store.Update(id, func(record *Record) error {
		record.State, record.Reason = Cancelled, "cancelled"
		record.FinishedAt = m.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
}
func (m *Manager) provenDead(record Record) bool {
	if m.Prober == nil || !refDead(m.Prober, record.Supervisor) || !refDead(m.Prober, record.Child) {
		return false
	}
	if record.ProcessGroup == nil {
		return true
	}
	alive, err := m.Processes.GroupAlive(record.ProcessGroup.Pid)
	return err == nil && !alive
}
func (m *Manager) Census() ([]string, error) {
	records, err := m.Store.List()
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, record := range records {
		alive := refAlive(m.Prober, record.Supervisor) || refAlive(m.Prober, record.Child)
		if record.ProcessGroup != nil {
			group, groupErr := m.Processes.GroupAlive(record.ProcessGroup.Pid)
			alive = alive || groupErr == nil && group
		}
		if record.State == Running && !alive {
			lines = append(lines, fmt.Sprintf("orphan-running id=%s", record.ID))
		}
		if record.State.Terminal() && alive {
			lines = append(lines, fmt.Sprintf("live-terminal id=%s", record.ID))
		}
	}
	for _, adapter := range m.Adapters {
		var strays []string
		var strayErr error
		if aware, ok := adapter.(interface {
			StraysFor([]Record) ([]string, error)
		}); ok {
			strays, strayErr = aware.StraysFor(records)
		} else {
			strays, strayErr = adapter.Strays()
		}
		if strayErr != nil {
			return nil, strayErr
		}
		lines = append(lines, strays...)
	}
	return lines, nil
}

func adapterForKind(kind, model string) string {
	switch kind {
	case "build", "critique":
		return "codex-exec"
	case "design":
		if model == "" || strings.HasPrefix(model, "claude-") {
			return "claude-headless"
		}
		return "codex-exec"
	case "read":
		return "claude-headless"
	case "proof":
		return "plain-exec"
	default:
		return ""
	}
}

func adapterOutcome(adapter Adapter, exitCode int, measureErr error) (State, string) {
	if owner, ok := adapter.(interface {
		Outcome(int, error) (State, string)
	}); ok {
		return owner.Outcome(exitCode, measureErr)
	}
	if exitCode == 0 {
		return Completed, ""
	}
	return Failed, fmt.Sprintf("exit-%d", exitCode)
}
func copyDeclaredOutputs(record Record, stateDir string) ([]Output, error) {
	var paths []string
	_ = json.Unmarshal(record.AdapterData["declaredOutputs"], &paths)
	var outputs []Output
	for _, source := range paths {
		if !filepath.IsAbs(source) {
			source = filepath.Join(record.WorkingDirectory, source)
		}
		name := filepath.Base(source)
		target := filepath.Join(stateDir, "outputs", name)
		if _, err := atomicfile.CopyFile(source, target, stateDir); err != nil {
			return outputs, err
		}
		info, err := os.Stat(target)
		if err != nil {
			return outputs, err
		}
		outputs = append(outputs, Output{Path: target, Bytes: info.Size()})
	}
	return outputs, nil
}
func refDead(prober identity.Prober, ref *identity.Ref) bool {
	return ref == nil || prober != nil && identity.AliveRef(prober, *ref) == identity.Dead
}
func refAlive(prober identity.Prober, ref *identity.Ref) bool {
	return ref != nil && prober != nil && identity.AliveRef(prober, *ref) == identity.Alive
}
func setString(values map[string]json.RawMessage, key, value string) {
	values[key], _ = json.Marshal(value)
}
func setStrings(values map[string]json.RawMessage, key string, value []string) {
	values[key], _ = json.Marshal(value)
}
func setInt64(values map[string]json.RawMessage, key string, value int64) {
	values[key], _ = json.Marshal(value)
}
func readString(values map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(values[key], &value)
	return value
}
func readInt64(values map[string]json.RawMessage, key string) int64 {
	var value int64
	_ = json.Unmarshal(values[key], &value)
	return value
}
