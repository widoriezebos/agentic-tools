// Package fake supplies recording doubles for every delegation port, so
// lifecycle tests drive owner outcomes without a lease, a steward, a runtime
// adapter, a goal ledger, job records or the flight recorder.
//
// Each double records its calls in order into a shared Log and answers from
// its function fields; an unset field answers with the zero value and nil.
package fake

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Log is the ordered record of port calls across every double sharing it.
type Log struct {
	mu    sync.Mutex
	calls []string
}

func (l *Log) add(format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, fmt.Sprintf(format, args...))
}

// Calls returns a copy of the recorded calls.
func (l *Log) Calls() []string {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.calls...)
}

// Lease doubles delegation.LeaseOps.
type Lease struct {
	Log           *Log
	ClassifyFunc  func(delegation.Invocation) (lease.ClassifyResult, error)
	RequireFunc   func(delegation.Invocation, *int64) (lease.HolderView, error)
	RenewFunc     func(delegation.Invocation) (lease.RenewResult, error)
	HeldRefusal   error
	AuthorizeFunc func(delegation.Invocation, delegation.AuthorityMode, string) error
}

func (f *Lease) Classify(inv delegation.Invocation) (lease.ClassifyResult, error) {
	f.Log.add("lease.Classify pid=%d", inv.CallerPid)
	if f.ClassifyFunc == nil {
		return lease.ClassifyResult{}, nil
	}
	return f.ClassifyFunc(inv)
}

func (f *Lease) RequireHolder(inv delegation.Invocation, expectedEpoch *int64) (lease.HolderView, error) {
	f.Log.add("lease.RequireHolder pid=%d epoch=%s", inv.CallerPid, epoch(expectedEpoch))
	if f.RequireFunc == nil {
		return lease.HolderView{}, nil
	}
	return f.RequireFunc(inv, expectedEpoch)
}

func (f *Lease) Renew(inv delegation.Invocation) (lease.RenewResult, error) {
	f.Log.add("lease.Renew pid=%d", inv.CallerPid)
	if f.RenewFunc == nil {
		return lease.RenewResult{}, nil
	}
	return f.RenewFunc(inv)
}

// Held runs fn unless HeldRefusal is set, which it returns without running fn.
func (f *Lease) Held(inv delegation.Invocation, expectedEpoch *int64, fn func() error) error {
	f.Log.add("lease.Held pid=%d epoch=%s", inv.CallerPid, epoch(expectedEpoch))
	if f.HeldRefusal != nil {
		return f.HeldRefusal
	}
	return fn()
}

func (f *Lease) Authorize(inv delegation.Invocation, mode delegation.AuthorityMode, job string) error {
	f.Log.add("lease.Authorize pid=%d mode=%s job=%s", inv.CallerPid, mode, job)
	if f.AuthorizeFunc == nil {
		return nil
	}
	return f.AuthorizeFunc(inv, mode, job)
}

func epoch(value *int64) string {
	if value == nil {
		return "none"
	}
	return fmt.Sprint(*value)
}

// Steward doubles delegation.StewardOps.
type Steward struct {
	Log           *Log
	AuthorizeFunc func(delegation.Invocation, string) (steward.DispatchAuthorization, error)
}

func (f *Steward) AuthorizeDispatch(inv delegation.Invocation, intent string) (steward.DispatchAuthorization, error) {
	f.Log.add("steward.AuthorizeDispatch pid=%d intent=%s", inv.CallerPid, intent)
	if f.AuthorizeFunc == nil {
		return steward.DispatchAuthorization{}, nil
	}
	return f.AuthorizeFunc(inv, intent)
}

// Adapter doubles delegation.AdapterOps.
type Adapter struct {
	Log *Log
	// NotInstalled lists runtimes the double reports without an adapter.
	NotInstalled       map[string]bool
	ConfigIdentityFunc func(runtime string) (string, error)
	ProbeFunc          func(runtime string) error
	OutputStreamFunc   func(runtime, roundDir string) (string, error)
	LaunchFunc         func(delegation.AdapterLaunch) (int64, error)
	CancelFunc         func(runtime, job string) error
	ResultPatchFunc    func(outputPath, failure, phase, usagePath string) error
}

func (f *Adapter) Installed(runtime string) bool {
	f.Log.add("adapter.Installed runtime=%s", runtime)
	return !f.NotInstalled[runtime]
}

func (f *Adapter) ConfigIdentity(_ context.Context, runtime string) (string, error) {
	f.Log.add("adapter.ConfigIdentity runtime=%s", runtime)
	if f.ConfigIdentityFunc == nil {
		return "", nil
	}
	return f.ConfigIdentityFunc(runtime)
}

func (f *Adapter) Probe(_ context.Context, runtime string) error {
	f.Log.add("adapter.Probe runtime=%s", runtime)
	if f.ProbeFunc == nil {
		return nil
	}
	return f.ProbeFunc(runtime)
}

func (f *Adapter) OutputStream(_ context.Context, runtime, roundDir string) (string, error) {
	f.Log.add("adapter.OutputStream runtime=%s round=%s", runtime, roundDir)
	if f.OutputStreamFunc == nil {
		return "", nil
	}
	return f.OutputStreamFunc(runtime, roundDir)
}

func (f *Adapter) Launch(_ context.Context, request delegation.AdapterLaunch) (int64, error) {
	f.Log.add("adapter.Launch runtime=%s verb=%s job=%s", request.Runtime, request.Verb, request.Job)
	if f.LaunchFunc == nil {
		return 0, nil
	}
	return f.LaunchFunc(request)
}

func (f *Adapter) Cancel(_ context.Context, runtime, job string) error {
	f.Log.add("adapter.Cancel runtime=%s job=%s", runtime, job)
	if f.CancelFunc == nil {
		return nil
	}
	return f.CancelFunc(runtime, job)
}

func (f *Adapter) ResultPatch(outputPath, failure, phase, usagePath string) error {
	f.Log.add("adapter.ResultPatch output=%s failure=%s phase=%s", outputPath, failure, phase)
	if f.ResultPatchFunc == nil {
		return nil
	}
	return f.ResultPatchFunc(outputPath, failure, phase, usagePath)
}

// Goal doubles delegation.GoalOps.
type Goal struct {
	Log         *Log
	BindingFunc func(goalID string) (delegation.GoalBinding, error)
	// Identity is the accepted ledger identity the brain fence reads.
	Identity func() string
}

func (f *Goal) LedgerIdentity() string {
	f.Log.add("goal.LedgerIdentity")
	if f.Identity == nil {
		return ""
	}
	return f.Identity()
}

func (f *Goal) Binding(goalID string) (delegation.GoalBinding, error) {
	f.Log.add("goal.Binding goal=%s", goalID)
	if f.BindingFunc == nil {
		return delegation.GoalBinding{}, nil
	}
	return f.BindingFunc(goalID)
}

// Records doubles delegation.RecordOps.
type Records struct {
	Log        *Log
	CreateFunc func(job, sourcePath string) error
	SetupFunc  func(job, sourcePath string) error
	CASFunc    func(job, expect, target, patchPath string) (string, error)
	ReadFunc   func(job string) (map[string]any, error)
}

func (f *Records) Create(job, sourcePath string) error {
	f.Log.add("records.Create job=%s", job)
	if f.CreateFunc == nil {
		return nil
	}
	return f.CreateFunc(job, sourcePath)
}

func (f *Records) Setup(job, sourcePath string) error {
	f.Log.add("records.Setup job=%s", job)
	if f.SetupFunc == nil {
		return nil
	}
	return f.SetupFunc(job, sourcePath)
}

func (f *Records) CAS(job, expect, target, patchPath string) (string, error) {
	f.Log.add("records.CAS job=%s %s->%s", job, expect, target)
	if f.CASFunc == nil {
		return "", nil
	}
	return f.CASFunc(job, expect, target, patchPath)
}

func (f *Records) Read(job string) (map[string]any, error) {
	f.Log.add("records.Read job=%s", job)
	if f.ReadFunc == nil {
		return nil, nil
	}
	return f.ReadFunc(job)
}

// Events doubles delegation.EventOps and keeps what was emitted.
type Events struct {
	Log     *Log
	mu      sync.Mutex
	Emitted []Event
}

// Event is one recorded emission.
type Event struct {
	Name    string
	Summary string
	Fields  map[string]string
}

func (f *Events) Emit(event, summary string, fields map[string]string) {
	f.Log.add("events.Emit event=%s", event)
	copied := make(map[string]string, len(fields))
	for key, value := range fields {
		copied[key] = value
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Emitted = append(f.Emitted, Event{Name: event, Summary: summary, Fields: copied})
}

// Process doubles delegation.ProcessOps over a scripted process table:
// Tags maps a pid to its live argv tag (absent is dead), Groups the live
// process groups, Owned the groups whose ownership is proven.
type Process struct {
	Log     *Log
	mu      sync.Mutex
	Tags    map[int64]string
	Unknown map[int64]bool
	Groups  map[int64]bool
	Owned   map[int64]bool
	// Survivors are groups that ignore TERM (and die only on KILL).
	Survivors map[int64]bool
	// Immortal groups survive KILL too.
	Immortal       map[int64]bool
	Starts         map[int64]int64
	CustodyTargets func(map[string]any) ([]int64, error)
	Signals        []string
	// ClaimProcessesFunc answers the claim owner's process reading; nil
	// reads the real process table (and Root's fixture table), which the
	// integration beds' real launched supervisors need.
	ClaimProcessesFunc func() (delegation.ClaimProcesses, error)
	Root               string
}

func (f *Process) ClaimProcesses() (delegation.ClaimProcesses, error) {
	f.Log.add("process.ClaimProcesses")
	if f.ClaimProcessesFunc != nil {
		return f.ClaimProcessesFunc()
	}
	reader, err := dispatchproc.StartReader(f.Root)
	if err != nil {
		return delegation.ClaimProcesses{}, err
	}
	return delegation.ClaimProcesses{Reader: reader, Scanner: dispatchproc.TaggedProcessScanner{Root: f.Root}, Verifier: dispatchproc.ClaimProcessVerifier{}}, nil
}

func (f *Process) TagState(pid int64, tag string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Unknown[pid] {
		return "unknown"
	}
	live, ok := f.Tags[pid]
	switch {
	case !ok:
		return "dead"
	case tag != "" && live == tag:
		return "live"
	}
	return "stale"
}

func (f *Process) Exists(pid int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.Tags[pid]
	return ok
}

func (f *Process) GroupExists(pgid int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Groups[pgid]
}

func (f *Process) GroupOwned(_ string, pgid int64, _ string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Owned[pgid]
}

func (f *Process) SignalGroup(pgid int64, sig delegation.Signal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Signals = append(f.Signals, fmt.Sprintf("%d:%d", pgid, sig))
	f.Log.add("process.Signal pgid=%d sig=%d", pgid, sig)
	switch {
	case f.Immortal[pgid]:
	case sig == delegation.SignalTerm && f.Survivors[pgid]:
	default:
		delete(f.Groups, pgid)
	}
	return nil
}

func (f *Process) CustodyGroups(record map[string]any) ([]int64, error) {
	if f.CustodyTargets != nil {
		return f.CustodyTargets(record)
	}
	// Every recorded custody process leads its own group in the scripted
	// table: the owner derives the targets, the table answers the groups.
	return dispatch.CustodyGroupTargets(record, func(pid int64) (int64, error) { return pid, nil })
}

func (f *Process) StartedAt(pid int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if started, ok := f.Starts[pid]; ok {
		return started, nil
	}
	return 0, fmt.Errorf("pid %d is not alive", pid)
}

// Git doubles delegation.GitOps with an argv-keyed script; an unscripted
// call fails, so a test never falls back to real Git.
type Git struct {
	Log *Log
	mu  sync.Mutex
	// Responses maps "dir|arg arg ..." (or "*|arg arg ..." for any dir) to
	// its answer.
	Responses map[string]GitResponse
	Calls     []string
}

// GitResponse is one scripted Git answer.
type GitResponse struct {
	Stdout, Stderr string
	Code           int
	// Effect runs when the call is made (a worktree add creating its
	// directory, say).
	Effect func()
}

func (f *Git) Run(_ context.Context, dir string, args ...string) ([]byte, []byte, error) {
	key := strings.Join(args, " ")
	f.mu.Lock()
	f.Calls = append(f.Calls, dir+"|"+key)
	response, ok := f.Responses[dir+"|"+key]
	if !ok {
		response, ok = f.Responses["*|"+key]
	}
	f.mu.Unlock()
	f.Log.add("git %s", key)
	if !ok {
		return nil, []byte("unscripted git call"), &delegation.GitExitError{Code: 128, Stderr: "unscripted git " + key}
	}
	if response.Effect != nil {
		response.Effect()
	}
	if response.Code != 0 {
		return []byte(response.Stdout), []byte(response.Stderr), &delegation.GitExitError{Code: response.Code, Stderr: response.Stderr}
	}
	return []byte(response.Stdout), []byte(response.Stderr), nil
}

// Clock doubles delegation.Clock: Sleep advances Now.
type Clock struct {
	mu      sync.Mutex
	Current time.Time
	// OnSleep runs after every advance (a test's world changing while the
	// lifecycle waits).
	OnSleep func(now time.Time)
}

func (f *Clock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Current
}

func (f *Clock) Sleep(d time.Duration) {
	f.mu.Lock()
	f.Current = f.Current.Add(d)
	now := f.Current
	hook := f.OnSleep
	f.mu.Unlock()
	if hook != nil {
		hook(now)
	}
}

// Host doubles delegation.HostOps.
type Host struct {
	Log          *Log
	WaitFunc     func(root, job string, callerPid int64) delegation.WaitOutcome
	WatchFunc    func(root, job string, callerPid int64, progressRoot string) int
	ExtendFunc   func(delegation.ExtendBudgetRequest) (string, int)
	ExtendCalled []delegation.ExtendBudgetRequest
}

func (f *Host) WaitJob(_ context.Context, root, job string, callerPid int64) delegation.WaitOutcome {
	f.Log.add("host.WaitJob job=%s", job)
	if f.WaitFunc == nil {
		return delegation.WaitOutcome{}
	}
	return f.WaitFunc(root, job, callerPid)
}

func (f *Host) WatchJob(_ context.Context, root, job string, callerPid int64, progressRoot string) int {
	f.Log.add("host.WatchJob job=%s progress=%s", job, progressRoot)
	if f.WatchFunc == nil {
		return 0
	}
	return f.WatchFunc(root, job, callerPid, progressRoot)
}

func (f *Host) ExtendBudget(_ context.Context, request delegation.ExtendBudgetRequest) (string, int) {
	f.Log.add("host.ExtendBudget goal=%s", request.GoalID)
	f.ExtendCalled = append(f.ExtendCalled, request)
	if f.ExtendFunc == nil {
		return "", 1
	}
	return f.ExtendFunc(request)
}

// Guard doubles delegation.GuardOps.
type Guard struct {
	Log         *Log
	AcquireFunc func(root string, pid int64, owner string) (gaterun.GuardResult, error)
	Released    []int64
}

func (f *Guard) Acquire(root string, pid int64, owner string, _, _ time.Duration, _ io.Writer) (gaterun.GuardResult, error) {
	f.Log.add("guard.Acquire owner=%s", owner)
	if f.AcquireFunc == nil {
		return gaterun.GuardAcquired, nil
	}
	return f.AcquireFunc(root, pid, owner)
}

func (f *Guard) Release(_ string, pid int64) error {
	f.Log.add("guard.Release pid=%d", pid)
	f.Released = append(f.Released, pid)
	return nil
}

// Set is one double per port over one shared Log.
type Set struct {
	Log     *Log
	Lease   *Lease
	Steward *Steward
	Adapter *Adapter
	Goal    *Goal
	Records *Records
	Events  *Events
	Process *Process
	Git     *Git
	Clock   *Clock
	Host    *Host
	Guard   *Guard
}

// NewSet returns a double for every port, all recording into one Log.
func NewSet() *Set {
	log := &Log{}
	return &Set{
		Log: log, Lease: &Lease{Log: log}, Steward: &Steward{Log: log},
		Adapter: &Adapter{Log: log}, Goal: &Goal{Log: log},
		Records: &Records{Log: log}, Events: &Events{Log: log},
		Process: &Process{Log: log, Tags: map[int64]string{}, Groups: map[int64]bool{}, Owned: map[int64]bool{}},
		Git:     &Git{Log: log, Responses: map[string]GitResponse{}},
		Clock:   &Clock{Current: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)},
		Host:    &Host{Log: log}, Guard: &Guard{Log: log},
	}
}

// Ports returns the doubles as a delegation port set.
func (s *Set) Ports() delegation.Ports {
	return delegation.Ports{
		Lease: s.Lease, Steward: s.Steward, Adapter: s.Adapter,
		Goal: s.Goal, Records: s.Records, Events: s.Events,
		Process: s.Process, Git: s.Git, Clock: s.Clock, Host: s.Host, Guard: s.Guard,
	}
}

var (
	_ delegation.LeaseOps   = (*Lease)(nil)
	_ delegation.StewardOps = (*Steward)(nil)
	_ delegation.AdapterOps = (*Adapter)(nil)
	_ delegation.GoalOps    = (*Goal)(nil)
	_ delegation.RecordOps  = (*Records)(nil)
	_ delegation.EventOps   = (*Events)(nil)
	_ delegation.ProcessOps = (*Process)(nil)
	_ delegation.GitOps     = (*Git)(nil)
	_ delegation.Clock      = (*Clock)(nil)
	_ delegation.HostOps    = (*Host)(nil)
	_ delegation.GuardOps   = (*Guard)(nil)
)
