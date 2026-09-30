// Package custody is the landing lane's one custody barrier (lane runtime
// design r10, K9). Every execution the kernel launches (a proof of any
// subject, a validation run, the retained verifier inside publish) records
// here, before it starts, who launched it; once started, the child's exact
// identity and its process groups. Settlement reads those records, plus the
// lane installation's proof leases (their custodian groups and fixture
// census, judged by proofrun), and says what is still live and what cannot
// be known.
//
// A terminal run record alone never settles custody: a run store may
// terminalize a run whose group still has members (run/conclude.go ends a
// draining run at its wind-down with the group non-empty). Only the kernel's
// own probe of the child and its groups settles a record, and a record once
// proven settled is never probed again (a process group id can be reused).
//
// A new execution, `landing engine advance` and `landing unset` wait while
// anything is live and refuse while anything is unknown; only a person may
// go past unknown custody (--force), never past live custody.
package custody

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
)

// The kinds of kernel-launched execution.
const (
	KindProve    = "prove"
	KindValidate = "validate"
	KindVerify   = "verify"
)

// The states a record's custody reads as.
const (
	Live    = "live"
	Dead    = "dead"
	Unknown = "unknown"
)

const schema = 1

// Record is one kernel-launched execution's custody.
type Record struct {
	Schema int `json:"schema"`
	// ID names the record; the file is <ID>.json in the store.
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	// OpenedAt is when the record was written, before anything started.
	OpenedAt string `json:"openedAt"`
	// Issuer is the exact identity of the kernel verb process that opened
	// the record and launches the execution.
	Issuer string `json:"issuer"`
	// Child is the exact identity of the launched child; empty until bound.
	// For work in the issuer's own process it is the issuer.
	Child string `json:"child,omitempty"`
	// Groups are the process groups the execution runs in: the child's own
	// (it is started as a group leader) and any bound later.
	Groups []int64 `json:"groups,omitempty"`
	// InProcess is work inside the issuer (the retained verifier); it holds
	// custody until Ended or the issuer dies.
	InProcess bool `json:"inProcess,omitempty"`
	// Ended: the issuer declared its in-process work over.
	Ended bool `json:"ended,omitempty"`
	// Cancelled: the launch failed before any child started.
	Cancelled bool `json:"cancelled,omitempty"`
	// Settled is when the record was first proven settled; a settled record
	// is never probed again.
	Settled string `json:"settled,omitempty"`
	// Forced names the person who went past this record's unknown custody;
	// Settled is then when they did.
	Forced string `json:"forced,omitempty"`
}

// State is one record's custody as read now.
type State struct {
	Record Record `json:"record"`
	// State is Live, Dead or Unknown.
	State string `json:"state"`
	// Why says, in words, what is live or unknown.
	Why string `json:"why,omitempty"`
}

// Dir is the custody store in the host lane state.
func Dir(home string) string { return filepath.Join(lane.HostDir(home), "landing-custody") }

func lockPath(home string) string { return filepath.Join(lane.HostDir(home), "landing-custody.lock") }

func recordPath(home, id string) string { return filepath.Join(Dir(home), id+".json") }

func withLock(home string, fn func() error) error {
	if home == "" || !filepath.IsAbs(home) {
		return fmt.Errorf("the landing custody store needs an absolute home, got %q", home)
	}
	if err := os.MkdirAll(Dir(home), 0o700); err != nil {
		return err
	}
	held, err := lock.File(lockPath(home), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer func() { _ = held.Release() }()
	return fn()
}

func write(home string, record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(recordPath(home, record.ID), append(data, '\n'), 0o600, lane.HostDir(home))
	return err
}

// Read reads one record strictly; ok is false when it does not exist.
func Read(home, id string) (Record, bool, error) {
	var record Record
	if err := strictjson.Read(recordPath(home, id), &record); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Record{}, false, nil
		}
		return Record{}, false, err
	}
	if record.Schema != schema || record.ID != id || record.Issuer == "" {
		return Record{}, false, fmt.Errorf("custody record %s is incomplete", id)
	}
	return record, true, nil
}

func update(home, id string, change func(*Record) error) (Record, error) {
	var out Record
	err := withLock(home, func() error {
		record, ok, err := Read(home, id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("custody record %s does not exist", id)
		}
		if err := change(&record); err != nil {
			return err
		}
		out = record
		return write(home, record)
	})
	return out, err
}

// self is this process's exact identity, encoded.
var self = func() (string, error) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return "", fmt.Errorf("this process's identity can't be read: state=%s err=%v", state, err)
	}
	return identity.EncodeRef(exact.Ref())
}

func newID(kind string) (string, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return kind + "-" + hex.EncodeToString(nonce[:]), nil
}

// Open durably records an execution about to start, before anything runs:
// its kind, its subject and the issuer that launches it.
func Open(home, kind, subject string, now time.Time) (Record, error) {
	switch kind {
	case KindProve, KindValidate, KindVerify:
	default:
		return Record{}, fmt.Errorf("custody kind %q is not a kernel execution", kind)
	}
	issuer, err := self()
	if err != nil {
		return Record{}, err
	}
	id, err := newID(kind)
	if err != nil {
		return Record{}, err
	}
	record := Record{Schema: schema, ID: id, Kind: kind, Subject: subject, OpenedAt: now.UTC().Format(time.RFC3339Nano), Issuer: issuer}
	return record, withLock(home, func() error { return write(home, record) })
}

// BindChild records the started child: its exact identity, and its own
// process group (the child is started as a group leader, so its group is
// its pid).
func BindChild(home, id string, child identity.Ref) error {
	encoded, err := identity.EncodeRef(child)
	if err != nil {
		return err
	}
	_, err = update(home, id, func(record *Record) error {
		record.Child = encoded
		record.Groups = appendGroup(record.Groups, child.Pid)
		return nil
	})
	return err
}

// BindGroup adds a process group the execution runs work in.
func BindGroup(home, id string, group int64) error {
	if group <= 1 {
		return fmt.Errorf("process group %d can't hold custody", group)
	}
	_, err := update(home, id, func(record *Record) error {
		record.Groups = appendGroup(record.Groups, group)
		return nil
	})
	return err
}

// BindSelf records work running inside the issuer (the retained verifier):
// custody holds until End or the issuer's death, and then while any bound
// group has members.
func BindSelf(home, id string) error {
	_, err := update(home, id, func(record *Record) error {
		record.Child, record.InProcess = record.Issuer, true
		return nil
	})
	return err
}

// End declares in-process work over.
func End(home, id string) error {
	_, err := update(home, id, func(record *Record) error {
		record.Ended = true
		return nil
	})
	return err
}

// Cancel records that the launch failed before any child started.
func Cancel(home, id string) error {
	_, err := update(home, id, func(record *Record) error {
		if record.Child != "" {
			return fmt.Errorf("custody record %s has a child; it is not cancelled", id)
		}
		record.Cancelled = true
		return nil
	})
	return err
}

func appendGroup(groups []int64, group int64) []int64 {
	for _, existing := range groups {
		if existing == group {
			return groups
		}
	}
	return append(groups, group)
}

// Start opens a record, starts command as a group leader, and binds the
// child; a failed start cancels the record. The caller waits for command.
func Start(home, kind, subject string, now time.Time, command *exec.Cmd) (Record, error) {
	record, err := Open(home, kind, subject, now)
	if err != nil {
		return Record{}, err
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	// The child leads a group of its own unless it leads a session; either
	// way its group id is its pid.
	if !command.SysProcAttr.Setsid {
		command.SysProcAttr.Setpgid, command.SysProcAttr.Pgid = true, 0
	}
	if err := command.Start(); err != nil {
		return record, errors.Join(err, Cancel(home, record.ID))
	}
	exact, state, probeErr := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	ref := identity.Ref{Pid: int64(command.Process.Pid)}
	if probeErr == nil && state == identity.Alive {
		ref = exact.Ref()
	}
	if err := BindChild(home, record.ID, ref); err != nil {
		return record, err
	}
	record, _, err = Read(home, record.ID)
	return record, err
}

// Lease is one proof lease of the lane's installation as proofrun judges it
// (its owner, custodian groups and fixture census).
type Lease struct {
	Name, State, Reason string
}

// Probes are the reads settlement makes.
type Probes struct {
	Prober identity.Prober
	// Group reads whether a process group has members: true live, false
	// empty, an error unknown.
	Group func(int64) (bool, error)
	// Leases are the lane installation's proof leases with state
	// proofrun's HostLease{Live,Unknown,...}; nil reads none.
	Leases func() ([]Lease, error)
	// Proving says whether the host's proving lock is held, and by whom:
	// the batch owner's proofs hold it until unit D deletes that owner.
	// nil reads nothing.
	Proving func() (holder string, busy bool, err error)
	Now     func() time.Time
}

func (probes Probes) defaults() Probes {
	if probes.Prober == nil {
		probes.Prober = identity.KernelProber{}
	}
	if probes.Group == nil {
		probes.Group = GroupMembers
	}
	if probes.Now == nil {
		probes.Now = time.Now
	}
	return probes
}

// GroupMembers reads a process group's membership through the kernel:
// signal 0 to the group.
func GroupMembers(group int64) (bool, error) {
	if group <= 1 {
		return false, fmt.Errorf("process group %d is not a group a kernel execution leads", group)
	}
	switch err := syscall.Kill(-int(group), 0); {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

// Judge reads one record's custody. It never reads a run store: a
// terminal run proves nothing about its processes.
func Judge(record Record, probes Probes) State {
	probes = probes.defaults()
	state := State{Record: record}
	switch {
	case record.Settled != "":
		state.State = Dead
		return state
	case record.Cancelled:
		state.State = Dead
		return state
	}
	liveness := func(encoded, who string) (string, string) {
		ref, err := identity.ParseRef(encoded)
		if err != nil {
			return Unknown, who + " is recorded as " + encoded + ", which is not a process identity"
		}
		switch identity.LiveRef(probes.Prober, ref) {
		case identity.Alive:
			return Live, fmt.Sprintf("%s (pid %d) still runs", who, ref.Pid)
		case identity.Dead:
			return Dead, ""
		}
		return Unknown, fmt.Sprintf("whether %s (pid %d) still runs can't be read", who, ref.Pid)
	}
	if record.Child == "" {
		issuer, why := liveness(record.Issuer, "its launcher")
		switch issuer {
		case Live:
			state.State, state.Why = Live, "it is starting: "+why
		case Dead:
			state.State, state.Why = Unknown, "its launcher died before it recorded what it started"
		default:
			state.State, state.Why = Unknown, why
		}
		return state
	}
	who := "its process"
	if record.InProcess {
		who = "the process it runs in"
	}
	if !record.InProcess || !record.Ended {
		child, why := liveness(record.Child, who)
		if child != Dead {
			state.State, state.Why = child, why
			return state
		}
	}
	for _, group := range record.Groups {
		members, err := probes.Group(group)
		if err != nil {
			state.State, state.Why = Unknown, fmt.Sprintf("whether process group %d still has members can't be read: %v", group, err)
			return state
		}
		if members {
			state.State, state.Why = Live, fmt.Sprintf("process group %d still has members", group)
			return state
		}
	}
	state.State = Dead
	return state
}

// Records lists every record; an unreadable one is returned as an error
// naming it, so a caller reads it as unknown.
func Records(home string) ([]Record, []string, error) {
	entries, err := os.ReadDir(Dir(home))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var records []Record
	var unreadable []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		record, ok, err := Read(home, strings.TrimSuffix(name, ".json"))
		if err != nil {
			unreadable = append(unreadable, fmt.Sprintf("custody record %s can't be read: %v", name, err))
			continue
		}
		if ok {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].OpenedAt < records[j].OpenedAt })
	return records, unreadable, nil
}

// Probe reads one record's custody now and persists a proven settlement.
func Probe(home, id string, probes Probes) (State, error) {
	probes = probes.defaults()
	record, ok, err := Read(home, id)
	if err != nil {
		return State{State: Unknown, Why: err.Error()}, nil
	}
	if !ok {
		return State{State: Unknown, Why: "custody record " + id + " does not exist"}, nil
	}
	state := Judge(record, probes)
	if state.State == Dead && record.Settled == "" {
		if settled, err := settle(home, id, probes); err == nil {
			state.Record = settled
		}
	}
	return state, nil
}

func settle(home, id string, probes Probes) (Record, error) {
	return update(home, id, func(record *Record) error {
		if record.Settled == "" {
			record.Settled = probes.Now().UTC().Format(time.RFC3339Nano)
		}
		return nil
	})
}

// Settle reads the lane's whole custody: every record, the installation's
// proof leases and the host proving lock. Live lists what still runs;
// Unknown what can't be read (a person may go past it, never past live).
// Proven settlements are persisted.
func Settle(home string, probes Probes) (lane.Settlement, error) {
	probes = probes.defaults()
	var settlement lane.Settlement
	records, unreadable, err := Records(home)
	if err != nil {
		settlement.Unknown = append(settlement.Unknown, "the custody store can't be read: "+err.Error())
		return settlement, nil
	}
	settlement.Unknown = append(settlement.Unknown, unreadable...)
	for _, record := range records {
		if record.Settled != "" {
			continue
		}
		state := Judge(record, probes)
		label := fmt.Sprintf("%s %s (%s)", record.Kind, record.Subject, record.ID)
		switch state.State {
		case Live:
			settlement.Live = append(settlement.Live, label+": "+state.Why)
		case Unknown:
			settlement.Unknown = append(settlement.Unknown, label+": "+state.Why)
		default:
			if _, err := settle(home, record.ID, probes); err != nil {
				settlement.Unknown = append(settlement.Unknown, label+": its settlement can't be recorded: "+err.Error())
			}
		}
	}
	if probes.Leases != nil {
		leases, err := probes.Leases()
		if err != nil {
			settlement.Unknown = append(settlement.Unknown, "the lane's proof leases can't be read: "+err.Error())
		}
		for _, lease := range leases {
			switch lease.State {
			case LeaseLive:
				settlement.Live = append(settlement.Live, "proof lease "+lease.Name+": "+lease.Reason)
			case LeaseUnknown:
				settlement.Unknown = append(settlement.Unknown, "proof lease "+lease.Name+": "+lease.Reason)
			}
		}
	}
	if probes.Proving != nil {
		holder, busy, err := probes.Proving()
		switch {
		case err != nil:
			settlement.Unknown = append(settlement.Unknown, "whether a proof holds the host's proving lock can't be read: "+err.Error())
		case busy:
			settlement.Live = append(settlement.Live, "a proof holds the host's proving lock ("+holder+")")
		}
	}
	return settlement, nil
}

// The lease states settlement counts (proofrun's HostLease states).
const (
	LeaseLive    = "live"
	LeaseUnknown = "unknown"
)

// FindSubject finds the newest record of kind opened for subject: how a
// caller that recorded a subject before it recorded the custody id finds
// what its launch opened.
func FindSubject(home, kind, subject string) (Record, bool, error) {
	records, unreadable, err := Records(home)
	if err != nil {
		return Record{}, false, err
	}
	for index := len(records) - 1; index >= 0; index-- {
		if records[index].Kind == kind && records[index].Subject == subject {
			return records[index], true, nil
		}
	}
	if len(unreadable) > 0 {
		return Record{}, false, errors.New(strings.Join(unreadable, "; "))
	}
	return Record{}, false, nil
}

// Override is a person's word past custody that can't be read: every record
// whose custody reads unknown now is recorded settled, forced by by, so it
// no longer holds the lane. Live records are never touched. It returns what
// it went past.
func Override(home, by string, probes Probes) ([]string, error) {
	probes = probes.defaults()
	if strings.TrimSpace(by) == "" {
		return nil, fmt.Errorf("only a named person may go past unknown custody")
	}
	records, _, err := Records(home)
	if err != nil {
		return nil, err
	}
	var passed []string
	for _, record := range records {
		if record.Settled != "" || Judge(record, probes).State != Unknown {
			continue
		}
		if _, err := update(home, record.ID, func(current *Record) error {
			if current.Settled == "" {
				current.Settled, current.Forced = probes.Now().UTC().Format(time.RFC3339Nano), by
			}
			return nil
		}); err != nil {
			return passed, err
		}
		passed = append(passed, record.ID)
	}
	return passed, nil
}

// Held is the custody barrier refusing a new execution: what still runs,
// and what can't be read when no person went past it.
type Held struct {
	Live, Unknown []string
}

func (held *Held) Error() string {
	if len(held.Live) > 0 {
		return "landing work still runs: " + strings.Join(held.Live, "; ")
	}
	return "whether landing work still runs can't be read: " + strings.Join(held.Unknown, "; ")
}

// Clear is the barrier every new kernel execution passes before it opens
// its record (a proof of any subject, a validation, the retained verifier):
// nil when nothing is live and nothing unknown, or only unknown and a
// person forced it; else a *Held.
func Clear(home string, probes Probes, force bool) error {
	settlement, err := Settle(home, probes)
	if err != nil {
		settlement.Unknown = append(settlement.Unknown, err.Error())
	}
	if settlement.Settled(force) {
		return nil
	}
	if len(settlement.Live) > 0 {
		return &Held{Live: settlement.Live}
	}
	return &Held{Unknown: settlement.Unknown}
}
