package stoptransition

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// LocalConfig binds the aggregate transition to one checkout and its engine.
type LocalConfig struct {
	Root         string
	Checkout     string
	Installation string
	Binary       string
	ScaleMilli   int
}

// LocalFamilies returns every per-checkout process family in stop order.
func LocalFamilies(config LocalConfig) []Family {
	processes := &processSnapshot{installation: config.Installation}
	suiteStops := &suiteStopGroups{groups: map[int64]bool{}}
	return []Family{
		newMissionFamily(config.Root),
		newJobFamily(config),
		newProofRunFamily(config.Root, config.ScaleMilli, suiteStops),
		newRunFamily(config.Root, config.ScaleMilli, suiteStops),
		newStewardFamily(config.Root),
		newSupervisionFamily(config, processes),
		newUntrackedFamily(config),
	}
}

type processSnapshot struct {
	installation string
	read         bool
	processes    []census.Process
	err          error
}

func (s *processSnapshot) Read() ([]census.Process, error) {
	if !s.read {
		s.processes, s.err = census.EnumerateConfiguredProcesses(s.installation)
		s.read = true
	}
	return s.processes, s.err
}

func (s *processSnapshot) Reset() {
	s.read = false
	s.processes = nil
	s.err = nil
}

type missionFamily struct {
	root            string
	items           map[string]missionrunner.Item
	runnerConcluded map[string]bool
}

func newMissionFamily(root string) *missionFamily {
	return &missionFamily{root: root, items: map[string]missionrunner.Item{}, runnerConcluded: map[string]bool{}}
}

func (f *missionFamily) Name() string { return "mission" }

func (f *missionFamily) Inventory() ([]Item, error) {
	items, err := missionrunner.Inventory(f.root)
	if err != nil {
		return nil, err
	}
	result := make([]Item, 0, len(items))
	for _, current := range items {
		key := fmt.Sprintf("mission:%s:%s:%s", current.MissionID, current.Kind, current.TurnID)
		f.items[key] = current
		generation := current.FenceGeneration
		line := fmt.Sprintf("mission %s runner pid %d pgid %d tag %s: %s", current.MissionID, current.Pid, current.Pgid, current.Tag, current.Status)
		component := "mission-runner"
		id := current.MissionID
		if current.Kind == missionrunner.ItemTurn {
			line = fmt.Sprintf("mission %s turn %s host pid %d pgid %d: %s", current.MissionID, current.TurnID, current.Pid, current.Pgid, current.Status)
			component = "mission-turn"
			id += "/" + current.TurnID
		}
		result = append(result, Item{Key: key, StatusLine: line, FenceGeneration: &generation,
			Survivor: stopfence.Survivor{Component: component, ID: id, Pid: current.Pid, PidStartedAt: current.PidStartedAt, Tag: current.Tag}})
	}
	return result, nil
}

func (f *missionFamily) Stop(item Item) (Outcome, error) {
	current, ok := f.items[item.Key]
	if !ok {
		return Outcome{}, fmt.Errorf("mission item %s disappeared from the typed inventory", item.Key)
	}
	outcome, err := missionrunner.Stop(current, missionrunner.StopOptions{RunnerConcluded: f.runnerConcluded[current.MissionID]})
	if err != nil {
		return Outcome{}, err
	}
	complete := outcome.Result != "not-stopped"
	if current.Kind == missionrunner.ItemRunner {
		f.runnerConcluded[current.MissionID] = outcome.Signal == missionrunner.TerminationTerm && outcome.Reason == "runner-concluded"
		line := fmt.Sprintf("mission %s runner pid %d pgid %d tag %s: ", current.MissionID, current.Pid, current.Pgid, current.Tag)
		switch {
		case !complete:
			line = "NOT STOPPED " + strings.TrimSuffix(line, ": ") + ": " + outcome.Reason + "; did: left the runner record and stop intent"
		case outcome.Result == "already-gone":
			line += "already gone"
		case outcome.Signal == missionrunner.TerminationKill:
			line += "killed (TERM ignored; turn and lease closed by stop)"
		default:
			line += "stopped (TERM, runner concluded)"
		}
		return Outcome{Line: line, Complete: complete, Survivor: item.Survivor}, nil
	}
	line := fmt.Sprintf("mission %s turn %s host pid %d pgid %d: ", current.MissionID, current.TurnID, current.Pid, current.Pgid)
	if !complete {
		line = "NOT STOPPED " + strings.TrimSuffix(line, ": ") + ": " + outcome.Reason + "; did: left the turn record"
	} else if outcome.Signal == missionrunner.TerminationKill {
		if outcome.ByRunner {
			line += "killed (by the runner, TERM ignored)"
		} else {
			line += "killed (TERM ignored)"
		}
	} else if outcome.ByRunner {
		line += "stopped (by the runner, TERM)"
	} else {
		line += "stopped (TERM)"
	}
	return Outcome{Line: line, Complete: complete, Survivor: item.Survivor}, nil
}

type jobFamily struct {
	config LocalConfig
	items  map[string]jobItem
}

type jobItem struct {
	path       string
	record     map[string]any
	lens       dispatch.JobRecord
	pid        int64
	started    int64
	pgid       int64
	remote     bool
	machine    string
	generation *int64
}

func newJobFamily(config LocalConfig) *jobFamily {
	return &jobFamily{config: config, items: map[string]jobItem{}}
}

func (f *jobFamily) Name() string { return "job" }

func (f *jobFamily) Inventory() ([]Item, error) {
	paths, err := filepath.Glob(filepath.Join(f.config.Root, "artifacts", "agents", "jobs", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var result []Item
	localMachine := ""
	localRead := false
	for _, path := range paths {
		record, readErr := dispatch.ReadRecordObject(path)
		if readErr != nil {
			return nil, &InventoryReadError{Path: path, Err: readErr}
		}
		lens := dispatch.JobRecordOf(record)
		if dispatch.TerminalStatus(lens.Status()) {
			continue
		}
		machine := lens.MachineID()
		remote := false
		if machine != "" {
			if !localRead {
				localMachine, err = goal.ResolveMachine(f.config.Checkout)
				if err != nil {
					return nil, fmt.Errorf("resolve local machine for job inventory: %w", err)
				}
				localRead = true
			}
			remote = machine != localMachine
		}
		id := lens.JobID()
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		pid, _ := integer(record["pid"])
		started, _ := integer(record["pidStartedAt"])
		pgid, _ := integer(record["pgid"])
		var generation *int64
		if value, ok := lens.FenceGeneration(); ok {
			generation = &value
		}
		current := jobItem{path: path, record: record, lens: lens, pid: pid, started: started, pgid: pgid, remote: remote, machine: machine, generation: generation}
		key := "job:" + id
		f.items[key] = current
		line := jobIdentityLine(id, lens.Status(), pid, pgid, lens.Role()) + ": running"
		if remote {
			line = fmt.Sprintf("job %s %s machine %s: running", id, lens.Status(), machine)
		}
		result = append(result, Item{Key: key, StatusLine: line, FenceGeneration: generation,
			Survivor: stopfence.Survivor{Component: "job", ID: id, MachineID: conditionalString(remote, machine), Pid: pid, PidStartedAt: started}})
	}
	return result, nil
}

func (f *jobFamily) Stop(item Item) (Outcome, error) {
	current, ok := f.items[item.Key]
	if !ok {
		return Outcome{}, fmt.Errorf("job item %s disappeared from the typed inventory", item.Key)
	}
	id := current.lens.JobID()
	if id == "" {
		id = strings.TrimPrefix(item.Key, "job:")
	}
	if current.remote {
		line := fmt.Sprintf("NOT STOPPED job %s %s machine %s: owned by another machine; did: nothing, cancel it from %s with metasystem delegate --cancel %s", id, current.lens.Status(), current.machine, current.machine, id)
		survivor := item.Survivor
		survivor.Reason = fmt.Sprintf("owned by another machine; cancel it from %s with metasystem delegate --cancel %s, then restore its terminal record", current.machine, id)
		return Outcome{Line: line, Complete: false, Survivor: survivor}, nil
	}
	command := exec.Command(f.config.Binary, "delegate", "--cancel", id)
	command.Env = append(os.Environ(), "METASYSTEM_DELEGATE_ROOT="+f.config.Installation)
	output, commandErr := command.CombinedOutput()
	record, readErr := dispatch.ReadRecordObject(current.path)
	if readErr != nil {
		return Outcome{}, readErr
	}
	lens := dispatch.JobRecordOf(record)
	if !dispatch.TerminalStatus(lens.Status()) {
		detail := strings.TrimSpace(string(output))
		if commandErr != nil {
			detail = firstNonEmpty(detail, commandErr.Error())
		}
		return Outcome{Line: "NOT STOPPED " + item.StatusLine + ": cancellation did not reach a terminal record; did: " + detail, Complete: false, Survivor: item.Survivor}, nil
	}
	killed, _ := record["cancelEscalatedToKill"].(bool)
	mechanism := "cancel path, TERM"
	if killed {
		mechanism = "cancel path, KILL after TERM was ignored"
	}
	return Outcome{Line: jobIdentityLine(id, current.lens.Status(), current.pid, current.pgid, current.lens.Role()) + ": cancelled (" + mechanism + ")", Complete: true}, nil
}

func jobIdentityLine(id, status string, pid, pgid int64, role string) string {
	line := fmt.Sprintf("job %s %s", id, status)
	if pid > 0 {
		line += fmt.Sprintf(" pid %d", pid)
	}
	if pgid > 0 {
		line += fmt.Sprintf(" pgid %d", pgid)
	}
	if role != "" {
		line += " role " + role
	}
	return line
}

type proofRunFamily struct {
	root       string
	scaleMilli int
	items      map[string]proofRunItem
	outcomes   map[string]proofrun.StopOutcome
	suiteStops *suiteStopGroups
}

// suiteStopGroups is the in-memory join between a proof-run launcher and the
// monitored wrapper group that hosts it. Both durable records already carry
// the process group; no new record or inferred argv relationship is needed.
type suiteStopGroups struct {
	groups map[int64]bool
}

type proofRunItem struct {
	record    proofrun.Record
	component string
	identity  proofrun.ProcessIdentity
}

func newProofRunFamily(root string, scaleMilli int, suiteStops *suiteStopGroups) *proofRunFamily {
	return &proofRunFamily{root: root, scaleMilli: scaleMilli, items: map[string]proofRunItem{}, outcomes: map[string]proofrun.StopOutcome{}, suiteStops: suiteStops}
}

func (f *proofRunFamily) Name() string { return "proof-run" }

func (f *proofRunFamily) Inventory() ([]Item, error) {
	records, err := proofrun.ReadRecords(f.root)
	if err != nil {
		return nil, err
	}
	var result []Item
	for _, record := range records {
		if record.Status != proofrun.StatusRunning {
			continue
		}
		processes := []struct {
			component string
			process   proofrun.ProcessIdentity
		}{{"suite", record.SuiteProcess}, {"watchdog", record.Watchdog}, {"launcher", record.Launcher}}
		for _, process := range processes {
			if identity.AliveRef(identity.KernelProber{}, process.process.Ref()) != identity.Alive {
				continue
			}
			key := fmt.Sprintf("proof-run:%s:%s", record.Suite, process.component)
			f.items[key] = proofRunItem{record: record, component: process.component, identity: process.process}
			generation := record.FenceGeneration
			line := fmt.Sprintf("proof-run %s %s pid %d", record.Suite, process.component, process.process.Pid)
			if process.component == "suite" && process.process.Pgid > 0 {
				line += fmt.Sprintf(" pgid %d", process.process.Pgid)
			}
			line += ": running"
			result = append(result, Item{Key: key, StatusLine: line, FenceGeneration: &generation,
				Survivor: stopfence.Survivor{Component: "proof-run-" + process.component, ID: record.Suite, Pid: process.process.Pid, PidStartedAt: process.process.PidStartedAt}})
		}
	}
	return result, nil
}

func (f *proofRunFamily) Stop(item Item) (Outcome, error) {
	current, ok := f.items[item.Key]
	if !ok {
		return Outcome{}, fmt.Errorf("proof-run item %s disappeared from the typed inventory", item.Key)
	}
	cacheKey := current.record.Suite + ":" + current.component
	outcome, ok := f.outcomes[cacheKey]
	if !ok {
		waitScale := f.scaleMilli
		if waitScale < 1 {
			waitScale = 1000
		}
		term := time.Duration((int64(5*time.Second)*int64(waitScale) + 999) / 1000)
		kill := time.Duration((int64(time.Second)*int64(waitScale) + 999) / 1000)
		for _, stopped := range proofrun.Stop(current.record, proofrun.StopOptions{TermGrace: term, KillGrace: kill}) {
			f.outcomes[current.record.Suite+":"+stopped.Component] = stopped
			if stopped.Component == "suite" && stopped.Result == proofrun.StopStopped && f.suiteStops != nil && current.record.Launcher.Pgid > 1 {
				f.suiteStops.groups[current.record.Launcher.Pgid] = true
			}
		}
		outcome = f.outcomes[cacheKey]
	}
	line := fmt.Sprintf("proof-run %s %s pid %d", current.record.Suite, current.component, current.identity.Pid)
	if current.component == "suite" && current.identity.Pgid > 0 {
		line += fmt.Sprintf(" pgid %d", current.identity.Pgid)
	}
	complete := outcome.Result != proofrun.StopNotStopped
	if !complete {
		line = "NOT STOPPED " + line + ": " + outcome.Reason
	} else if outcome.Result == proofrun.StopAlreadyGone {
		line += ": exited (" + outcome.Reason + ")"
	} else if outcome.Signal == proofrun.StopSignalKill {
		line += ": killed (" + outcome.Reason + ")"
	} else {
		line += ": stopped (" + outcome.Reason + ")"
	}
	return Outcome{Line: line, Complete: complete, Survivor: item.Survivor}, nil
}

type runFamily struct {
	store      *runpkg.Store
	items      map[string]runpkg.Record
	scaleMilli int
	suiteStops *suiteStopGroups
	now        func() time.Time
	sleep      func(time.Duration)
}

func newRunFamily(root string, scaleMilli int, suiteStops *suiteStopGroups) *runFamily {
	return &runFamily{
		store: &runpkg.Store{Root: root}, items: map[string]runpkg.Record{}, scaleMilli: scaleMilli,
		suiteStops: suiteStops, now: time.Now, sleep: time.Sleep,
	}
}

func (f *runFamily) Name() string { return "run" }

func (f *runFamily) Inventory() ([]Item, error) {
	records, unreadable := f.store.List()
	if len(unreadable) > 0 {
		return nil, fmt.Errorf("run records are unreadable: %s", strings.Join(unreadable, "; "))
	}
	var result []Item
	for _, record := range records {
		if record.Status != runpkg.StatusLaunching && record.Status != runpkg.StatusRunning && record.Status != runpkg.StatusDraining {
			continue
		}
		if record.Status != runpkg.StatusLaunching {
			if record.Pid == nil || record.PidStartedAt == nil {
				continue
			}
			ref := identity.Ref{Pid: *record.Pid, StartedAtSec: *record.PidStartedAt, StartTicks: record.PidStartTicks, BootID: record.BootID}
			if identity.AliveRef(identity.KernelProber{}, ref) != identity.Alive {
				continue
			}
		}
		key := "run:" + record.RunId
		f.items[key] = record
		generation := record.FenceGeneration
		line := fmt.Sprintf("run %s %s", record.RunId, record.Status)
		pid, started, pgid := int64(0), int64(0), int64(0)
		if record.Pid != nil {
			pid = *record.Pid
			line += fmt.Sprintf(" pid %d", pid)
		}
		if record.PidStartedAt != nil {
			started = *record.PidStartedAt
		}
		if record.Pgid != nil {
			pgid = *record.Pgid
			line += fmt.Sprintf(" pgid %d", pgid)
		}
		line += " " + record.Custody + ": running"
		result = append(result, Item{Key: key, StatusLine: line, FenceGeneration: &generation,
			Survivor:    stopfence.Survivor{Component: "run", ID: record.RunId, Pid: pid, PidStartedAt: started},
			ObserveOnly: record.Custody != runpkg.CustodyWrapped})
	}
	return result, nil
}

func (f *runFamily) Stop(item Item) (Outcome, error) {
	record, ok := f.items[item.Key]
	if !ok {
		return Outcome{}, fmt.Errorf("run item %s disappeared from the typed inventory", item.Key)
	}
	if f.suiteStopped(record) {
		concluded, err := f.waitForSuiteConclusion(record.RunId)
		if err != nil {
			return Outcome{}, err
		}
		if concluded != nil && runpkg.Terminal(concluded.Status) {
			outcome := outcomeForConcludedRun(record, concluded.Status)
			line, complete, err := runOutcomeLine(outcome, true)
			return Outcome{Line: line, Complete: complete}, err
		}
	}
	outcome, err := f.store.Stop(record.RunId)
	if err != nil {
		return Outcome{}, err
	}
	line, complete, err := runOutcomeLine(outcome, false)
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Line: line, Complete: complete, Survivor: item.Survivor}, nil
}

func (f *runFamily) suiteStopped(record runpkg.Record) bool {
	return record.Custody == runpkg.CustodyWrapped && record.Pgid != nil && f.suiteStops != nil && f.suiteStops.groups[*record.Pgid]
}

func (f *runFamily) waitForSuiteConclusion(id string) (*runpkg.Record, error) {
	wait := scaledFamilyDuration(5*time.Second, f.scaleMilli)
	deadline := f.now().Add(wait)
	for {
		record, err := f.store.Read(id)
		if err != nil || record == nil || runpkg.Terminal(record.Status) {
			return record, err
		}
		ready, err := f.store.HasMatchingSidecar(id)
		if err != nil {
			return nil, err
		}
		if ready {
			if _, err := f.store.Assess(id); err != nil {
				return nil, err
			}
			record, err = f.store.Read(id)
			if err != nil || record == nil || runpkg.Terminal(record.Status) {
				return record, err
			}
		}
		if !f.now().Before(deadline) {
			return record, nil
		}
		poll := scaledFamilyDuration(100*time.Millisecond, f.scaleMilli)
		if remaining := deadline.Sub(f.now()); poll > remaining {
			poll = remaining
		}
		f.sleep(poll)
	}
}

func scaledFamilyDuration(base time.Duration, scaleMilli int) time.Duration {
	if scaleMilli < 1 {
		scaleMilli = 1000
	}
	result := time.Duration((int64(base)*int64(scaleMilli) + 999) / 1000)
	if result < time.Millisecond {
		return time.Millisecond
	}
	return result
}

func outcomeForConcludedRun(record runpkg.Record, status string) runpkg.StopOutcome {
	outcome := runpkg.StopOutcome{
		RunID: record.RunId, InitialStatus: record.Status, Status: status, Custody: record.Custody,
		Signal: runpkg.StopSignalNone, Result: runpkg.StopResultAlreadyGone, Reason: "suite stopped",
	}
	if record.Pid != nil {
		outcome.PID = *record.Pid
	}
	if record.Pgid != nil {
		outcome.PGID = *record.Pgid
	}
	return outcome
}

func runOutcomeLine(outcome runpkg.StopOutcome, suiteStopped bool) (string, bool, error) {
	if outcome.InitialStatus == "" {
		return "", false, fmt.Errorf("run %s stop outcome carries no initial record status", outcome.RunID)
	}
	line := runIdentityLine(outcome)
	if suiteStopped {
		if !runpkg.Terminal(outcome.Status) {
			return "", false, fmt.Errorf("run %s suite stop completed without a terminal record status", outcome.RunID)
		}
		return line + "concluded " + outcome.Status + " (suite stopped)", true, nil
	}
	if outcome.Custody != runpkg.CustodyWrapped {
		return line + "not the metasystem's process, not signalled", true, nil
	}
	switch outcome.Result {
	case runpkg.StopResultNotStopped:
		return "NOT STOPPED " + strings.TrimSuffix(line, ": ") + ": " + outcome.Reason + "; did: left the run record", false, nil
	case runpkg.StopResultAlreadyGone:
		return line + "already gone", true, nil
	case runpkg.StopResultStopped:
		if outcome.Status == runpkg.StatusLaunchFailed {
			return line + "launch failed (stopped)", true, nil
		}
		if !runpkg.Terminal(outcome.Status) {
			return "", false, fmt.Errorf("run %s stop completed without a terminal record status", outcome.RunID)
		}
		reason, err := runStopReason(outcome)
		if err != nil {
			return "", false, err
		}
		return line + "concluded " + outcome.Status + " (" + reason + ")", true, nil
	default:
		return "", false, fmt.Errorf("run %s stop returned unknown result %q", outcome.RunID, outcome.Result)
	}
}

func runIdentityLine(outcome runpkg.StopOutcome) string {
	line := fmt.Sprintf("run %s %s", outcome.RunID, outcome.InitialStatus)
	if outcome.PID > 0 {
		line += fmt.Sprintf(" pid %d", outcome.PID)
	}
	if outcome.PGID > 0 {
		line += fmt.Sprintf(" pgid %d", outcome.PGID)
	}
	line += " " + outcome.Custody + ": "
	return line
}

func runStopReason(outcome runpkg.StopOutcome) (string, error) {
	switch outcome.Signal {
	case runpkg.StopSignalTerm:
		return "TERM", nil
	case runpkg.StopSignalKill:
		return "KILL after TERM was ignored", nil
	default:
		return "", fmt.Errorf("run %s stopped without a known signal outcome", outcome.RunID)
	}
}

type stewardFamily struct {
	root  string
	items map[string]steward.RunnerRecord
}

func newStewardFamily(root string) *stewardFamily {
	return &stewardFamily{root: root, items: map[string]steward.RunnerRecord{}}
}

func (f *stewardFamily) Name() string { return "steward" }

func (f *stewardFamily) Inventory() ([]Item, error) {
	record, live := steward.LiveRunner(f.root)
	if !live {
		return nil, nil
	}
	key := "steward:runner"
	f.items[key] = record
	generation := record.FenceGeneration
	return []Item{{Key: key, StatusLine: fmt.Sprintf("steward-runner pid %d started %d: running", record.Pid, record.PidStartedAt), FenceGeneration: &generation,
		Survivor: stopfence.Survivor{Component: "steward-runner", Pid: record.Pid, PidStartedAt: record.PidStartedAt}}}, nil
}

func (f *stewardFamily) Stop(item Item) (Outcome, error) {
	record, ok := f.items[item.Key]
	if !ok {
		return Outcome{}, fmt.Errorf("steward item %s disappeared from the typed inventory", item.Key)
	}
	outcome, err := steward.Disarm(f.root)
	if err != nil && outcome.Result == "" {
		return Outcome{}, err
	}
	complete := outcome.Result != "not-stopped"
	line := fmt.Sprintf("steward-runner pid %d started %d: ", record.Pid, record.PidStartedAt)
	if !complete {
		line = "NOT STOPPED " + strings.TrimSuffix(line, ": ") + ": " + outcome.Reason + "; did: left the runner record"
	} else if outcome.Result == "already-gone" {
		line += "already gone"
	} else if outcome.Signal == "kill" {
		line += "stopped (KILL after TERM was ignored)"
	} else {
		line += "stopped (TERM)"
	}
	additional := []string(nil)
	if complete {
		additional = []string{"narrator: stopped with the steward runner"}
	}
	return Outcome{Line: line, AdditionalLines: additional, Complete: complete, Survivor: item.Survivor}, nil
}

type supervisionFamily struct {
	config           LocalConfig
	processes        *processSnapshot
	items            map[string]supervise.InventoryItem
	outcomes         map[string]supervise.ComponentOutcome
	stopErr          error
	stopped          bool
	failures         []supervise.ShutdownFailure
	reportedFailures bool
	failureAnchor    string
	shutdown         func(string, string, string, string, int) (supervise.ShutdownReport, error)
}

func newSupervisionFamily(config LocalConfig, processes *processSnapshot) *supervisionFamily {
	return &supervisionFamily{
		config: config, processes: processes, items: map[string]supervise.InventoryItem{}, outcomes: map[string]supervise.ComponentOutcome{},
		shutdown: supervise.ShutdownAt,
	}
}

func (f *supervisionFamily) Name() string { return "supervision" }

func (f *supervisionFamily) BeginInventory() { f.processes.Reset() }

func (f *supervisionFamily) Inventory() ([]Item, error) {
	processes, err := f.processes.Read()
	if err != nil {
		return nil, err
	}
	items, err := supervise.ReadInventory(f.config.Root, processes)
	if err != nil {
		return nil, err
	}
	result := make([]Item, 0, len(items))
	for _, current := range items {
		key := fmt.Sprintf("supervision:%s:%d:%d", current.Component, current.Generation, current.Identity.Pid)
		f.items[key] = current
		generation := current.FenceGeneration
		line := fmt.Sprintf("%s pid %d", current.Component, current.Identity.Pid)
		if current.Component == "supervision-owner" {
			line += fmt.Sprintf(" tag %s generation %d", current.Tag, current.Generation)
		}
		line += ": running"
		result = append(result, Item{Key: key, StatusLine: line, FenceGeneration: &generation,
			Survivor: stopfence.Survivor{Component: current.Component, Pid: current.Identity.Pid, PidStartedAt: current.Identity.StartedAtSec, Tag: current.Tag}})
	}
	if !f.stopped && len(result) > 0 {
		// ShutdownAt acts on the whole set during the first Stop call, but its
		// bookkeeping happens after every process outcome. Anchor processless
		// failures to the last inventoried component so report order remains
		// the action order.
		f.failureAnchor = result[len(result)-1].Key
	}
	return result, nil
}

func (f *supervisionFamily) Stop(item Item) (result Outcome, err error) {
	defer func() {
		f.attachFailures(item, &result)
	}()
	itemLine := strings.TrimSuffix(item.StatusLine, ": running")
	current, ok := f.items[item.Key]
	if !ok {
		return Outcome{Line: "NOT STOPPED " + itemLine + ": item disappeared from the typed inventory; did: left it in the fence record", Complete: false, Survivor: item.Survivor}, nil
	}
	if !f.stopped {
		prefix := "metasystem-supervision-owner-" + lease.Slug(f.config.Checkout) + "-"
		report, shutdownErr := f.shutdown(f.config.Root, f.config.Installation, f.config.Root, prefix, f.config.ScaleMilli)
		for _, stopped := range report.Outcomes {
			f.outcomes[supervisionOutcomeKey(stopped)] = stopped
		}
		f.stopErr = shutdownErr
		f.failures = append(f.failures, report.Failures...)
		f.stopped = true
	}
	outcome, ok := f.outcomes[supervisionItemKey(current)]
	if !ok {
		reason := "identity could not be inspected because shutdown returned no outcome"
		if f.stopErr != nil {
			reason += ": " + f.stopErr.Error()
		}
		return Outcome{Line: "NOT STOPPED " + itemLine + ": " + reason + "; did: left it in the fence record", Complete: false, Survivor: item.Survivor}, nil
	}
	line := fmt.Sprintf("%s pid %d", current.Component, current.Identity.Pid)
	if current.Component == "supervision-owner" {
		line += fmt.Sprintf(" tag %s generation %d", current.Tag, current.Generation)
	}
	complete := outcome.Result != supervise.ShutdownNotStopped
	if !complete {
		line = "NOT STOPPED " + line + ": " + outcome.Reason + "; did: left it in the fence record"
	} else if outcome.Result == supervise.ShutdownAlreadyGone {
		line += ": already gone"
		if outcome.Reason == "by the owner" {
			line += " (by the owner)"
		}
	} else if outcome.Signal == supervise.ShutdownSignalKill {
		if current.Component == "supervision-owner" && outcome.Reason == "shutdown-escalated" {
			line += ": killed (TERM ignored; reaped reason=shutdown-escalated)"
		} else {
			line += ": killed (TERM ignored)"
		}
	} else {
		if current.Component == "supervision-owner" && outcome.Reason == "shutdown" {
			line += ": stopped (TERM, exited reason=shutdown)"
		} else {
			line += ": stopped (TERM)"
		}
	}
	result = Outcome{Line: line, Complete: complete, Survivor: item.Survivor}
	return result, nil
}

func (f *supervisionFamily) attachFailures(item Item, result *Outcome) {
	if !f.reportedFailures && (f.failureAnchor == "" || item.Key == f.failureAnchor) {
		for _, failure := range f.failures {
			survivor := stopfence.Survivor{Component: failure.Component, Path: failure.Path, Reason: failure.Reason}
			result.Auxiliary = append(result.Auxiliary, AuxiliaryOutcome{
				Key: failure.Component + ":" + failure.Path, Line: failure.Line(), Complete: false, Survivor: survivor,
			})
		}
		f.reportedFailures = true
	}
}

func supervisionItemKey(item supervise.InventoryItem) string {
	return fmt.Sprintf("%s:%d:%d", item.Component, item.Generation, item.Identity.Pid)
}

func supervisionOutcomeKey(outcome supervise.ComponentOutcome) string {
	return fmt.Sprintf("%s:%d:%d", outcome.Component, outcome.Generation, outcome.Identity.Pid)
}

type untrackedFamily struct {
	config LocalConfig
	items  map[string]census.InventoryItem
}

func newUntrackedFamily(config LocalConfig) *untrackedFamily {
	return &untrackedFamily{config: config, items: map[string]census.InventoryItem{}}
}

func (f *untrackedFamily) Name() string { return "untracked" }

func (f *untrackedFamily) Inventory() ([]Item, error) {
	var verdict census.Verdict
	var err error
	if _, configured, fixtureErr := census.ConfiguredProcessFixture(f.config.Installation); fixtureErr != nil {
		return nil, fixtureErr
	} else if configured {
		verdict, err = census.RunFixtureCensusAt(f.config.Installation, f.config.Root, f.config.Checkout, os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"), "stop-inventory", 60, time.Now())
	} else {
		verdict, err = census.RunProductionCensusAt(f.config.Installation, f.config.Root, f.config.Checkout, "stop-inventory", 60, time.Now())
	}
	if err != nil {
		return nil, err
	}
	if verdict.Verdict != "SUCCESS" && !onlyStoppedSupervisionErrors(verdict.Errors) {
		return nil, fmt.Errorf("process census failed: %s", strings.Join(verdict.Errors, "; "))
	}
	var result []Item
	for _, current := range verdict.Inventory {
		if current.Class != "UNTRACKED" || current.Pid == int64(os.Getpid()) {
			continue
		}
		key := fmt.Sprintf("untracked:%d:%d", current.Pid, current.PidStartedAt)
		f.items[key] = current
		line := fmt.Sprintf("untracked pid %d %s %s: running", current.Pid, current.Runtime, current.Argv)
		result = append(result, Item{Key: key, StatusLine: line,
			Survivor:    stopfence.Survivor{Component: "untracked", Pid: current.Pid, PidStartedAt: current.PidStartedAt},
			ObserveOnly: true})
	}
	return result, nil
}

func onlyStoppedSupervisionErrors(all []string) bool {
	if len(all) == 0 {
		return false
	}
	for _, item := range all {
		if !strings.HasPrefix(item, "supervision-") {
			return false
		}
	}
	return true
}

func (f *untrackedFamily) Stop(item Item) (Outcome, error) {
	current, ok := f.items[item.Key]
	if !ok {
		return Outcome{}, fmt.Errorf("untracked item %s disappeared from the typed inventory", item.Key)
	}
	return Outcome{Line: fmt.Sprintf("untracked pid %d %s %s: not the metasystem's, not touched", current.Pid, current.Runtime, current.Argv), Complete: true}, nil
}

func integer(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case float64:
		return int64(typed), typed == float64(int64(typed))
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	}
	return 0, false
}

func conditionalString(condition bool, value string) string {
	if condition {
		return value
	}
	return ""
}
