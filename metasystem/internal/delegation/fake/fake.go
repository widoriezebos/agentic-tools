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
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
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
	Log                *Log
	ConfigIdentityFunc func(runtime string) (string, error)
	ProbeFunc          func(runtime string) error
	OutputStreamFunc   func(runtime, roundDir string) (string, error)
	LaunchFunc         func(delegation.AdapterLaunch) (int64, error)
	CancelFunc         func(runtime, job string) error
	ResultPatchFunc    func(outputPath, failure, phase, usagePath string) error
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

// Set is one double per port over one shared Log.
type Set struct {
	Log     *Log
	Lease   *Lease
	Steward *Steward
	Adapter *Adapter
	Goal    *Goal
	Records *Records
	Events  *Events
}

// NewSet returns a double for every port, all recording into one Log.
func NewSet() *Set {
	log := &Log{}
	return &Set{
		Log: log, Lease: &Lease{Log: log}, Steward: &Steward{Log: log},
		Adapter: &Adapter{Log: log}, Goal: &Goal{Log: log},
		Records: &Records{Log: log}, Events: &Events{Log: log},
	}
}

// Ports returns the doubles as a delegation port set.
func (s *Set) Ports() delegation.Ports {
	return delegation.Ports{
		Lease: s.Lease, Steward: s.Steward, Adapter: s.Adapter,
		Goal: s.Goal, Records: s.Records, Events: s.Events,
	}
}

var (
	_ delegation.LeaseOps   = (*Lease)(nil)
	_ delegation.StewardOps = (*Steward)(nil)
	_ delegation.AdapterOps = (*Adapter)(nil)
	_ delegation.GoalOps    = (*Goal)(nil)
	_ delegation.RecordOps  = (*Records)(nil)
	_ delegation.EventOps   = (*Events)(nil)
)
