package main

// The machines of this computer (Wido, 2026-09-30): one command lists what
// MetaSystem runs in the background on this computer, and one stops it
// everywhere. Nothing here detects a process: each checkout is read by the
// same stop transition status reads, and stopped by system stop itself.
// The machines come from the host registry of armed checkouts, the landing
// lane record and the checkout the command runs in; a registry that cannot
// be read is reported, never read as no machines.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// codeMachineOnAnotherComputer refuses a stop of a machine this computer
// does not run: only that computer's own system stop reaches its processes.
const codeMachineOnAnotherComputer = "MACHINE_ON_ANOTHER_COMPUTER"

// machineOwners are the machine verbs' seams; the zero value is production.
type machineOwners struct {
	// registryPath is the host registry of armed checkouts.
	registryPath func() (string, error)
	// nickname is a checkout's machine nickname, false when it has none.
	nickname func(checkout string) (string, bool)
}

func (inv *intentInvocation) machineSeams() machineOwners {
	owners := inv.owners.machines
	if owners.registryPath == nil {
		owners.registryPath = registry.DefaultPath
	}
	if owners.nickname == nil {
		owners.nickname = seat.Machine
	}
	return owners
}

// hostMachine is one checkout of this computer and what status read of it.
type hostMachine struct {
	Name     string   `json:"name"`
	Nickname bool     `json:"nickname"`
	Checkout string   `json:"checkout"`
	Sources  []string `json:"sources"`
	Lane     bool     `json:"landingLane,omitempty"`
	This     bool     `json:"thisCheckout,omitempty"`
	// State is running (a MetaSystem process of it runs), stopped (none
	// runs) or unknown (Reason says why it could not be read).
	State          string           `json:"state"`
	Reason         string           `json:"reason,omitempty"`
	FenceState     string           `json:"fence,omitempty"`
	FenceChangedAt string           `json:"fenceChangedAt,omitempty"`
	Components     []machineProcess `json:"components"`
	Work           []machineProcess `json:"work"`
	Launches       []machineLaunch  `json:"launches"`
	LaneOwner      *lane.OwnerView  `json:"landingOwner,omitempty"`
	StatusLines    []string         `json:"statusLines"`
	// paths are every path a source named it by.
	paths []string
	// launchRecords are the running launches in its checkout.
	launchRecords []launch.Record
	// stopResult is machine stop's system stop result for it.
	stopResult *intentResult
}

// machineProcess is one process status listed, with its start.
type machineProcess struct {
	Family    string `json:"family"`
	Component string `json:"component"`
	ID        string `json:"id,omitempty"`
	Pid       int64  `json:"pid,omitempty"`
	Since     string `json:"since,omitempty"`
	Line      string `json:"line"`
	started   int64
}

// machineLaunch is one running launch of this user in a machine's checkout.
type machineLaunch struct {
	Reference string `json:"reference"`
	Purpose   string `json:"purpose"`
}

// hostReading is every machine of this computer the sources name.
type hostReading struct {
	Machines        []*hostMachine `json:"machines"`
	Registry        string         `json:"registry"`
	RegistryProblem string         `json:"registryProblem,omitempty"`
	LaneProblem     string         `json:"laneProblem,omitempty"`
	LaunchProblem   string         `json:"launchProblem,omitempty"`
	// LaunchesElsewhere are this user's running launches outside every
	// machine's checkout.
	LaunchesElsewhere []machineLaunch  `json:"launchesElsewhere"`
	NotOurs           []machineProcess `json:"notOurs"`
	launchesElsewhere []launch.Record
	laneHome          string
}

// otherComputerMachine is a machine of the fleet this computer does not run.
type otherComputerMachine struct {
	Machine      string `json:"machine"`
	LastReported string `json:"lastReported,omitempty"`
	Standing     string `json:"standing"`
	// OnAnotherComputer is false when the host registry cannot be read:
	// a stopped checkout of this computer then looks the same.
	OnAnotherComputer bool `json:"onAnotherComputer"`
}

// checkoutInvocation is inv acting on one checkout, as if --repo named it.
func (inv *intentInvocation) checkoutInvocation(checkout string) *intentInvocation {
	child := *inv
	child.input = intentInput{values: map[string][]string{"repo": {checkout}}}
	child.layout, child.stateRoot = stateroot.Layout{}, ""
	return &child
}

// discoverHostMachines names this computer's machines: the checkouts the
// host registry records, the landing lane's checkout and this checkout, each
// once by its Git root, this checkout first, then by name.
func (inv *intentInvocation) discoverHostMachines() hostReading {
	owners := inv.machineSeams()
	reading := hostReading{LaunchesElsewhere: []machineLaunch{}, NotOurs: []machineProcess{}}
	type candidate struct{ path, source string }
	var candidates []candidate
	if inv.layout.GitRoot != "" {
		candidates = append(candidates, candidate{inv.layout.GitRoot, "this checkout"})
	}
	if path, err := owners.registryPath(); err != nil {
		reading.RegistryProblem = "the host registry cannot be located: " + err.Error()
	} else {
		reading.Registry = path
		host, err := registry.HostCheckouts(path)
		if err != nil {
			reading.RegistryProblem = fmt.Sprintf("the host registry %s cannot be read: %v", path, err)
		}
		for _, checkout := range host {
			source := "registry: stopped"
			if checkout.Armed {
				source = "registry: armed"
			}
			candidates = append(candidates, candidate{checkout.Path, source})
		}
	}
	landing := inv.landing()
	if home, err := landing.home(); err != nil {
		reading.LaneProblem = "the landing lane cannot be read: " + err.Error()
	} else if record, ok, err := lane.Read(home); err != nil {
		reading.LaneProblem = "the landing lane cannot be read: " + err.Error()
	} else if ok {
		reading.laneHome = home
		candidates = append(candidates, candidate{record.Root, "landing lane"})
	}
	byRoot := map[string]*hostMachine{}
	for _, current := range candidates {
		root := current.path
		layout, err := inv.owners.resolver.ResolveLayout(current.path)
		if err == nil {
			root = layout.GitRoot
		}
		machine := byRoot[root]
		if machine == nil {
			machine = &hostMachine{Checkout: root, Components: []machineProcess{}, Work: []machineProcess{}, Launches: []machineLaunch{}, StatusLines: []string{}}
			if err != nil {
				machine.State, machine.Reason = "unknown", err.Error()
			}
			byRoot[root] = machine
			reading.Machines = append(reading.Machines, machine)
		}
		if !slices.Contains(machine.Sources, current.source) {
			machine.Sources = append(machine.Sources, current.source)
		}
		if !slices.Contains(machine.paths, current.path) {
			machine.paths = append(machine.paths, current.path)
		}
		machine.This = machine.This || current.source == "this checkout"
		machine.Lane = machine.Lane || current.source == "landing lane"
	}
	for _, machine := range reading.Machines {
		if name, ok := owners.nickname(machine.Checkout); ok {
			machine.Name, machine.Nickname = name, true
		} else {
			machine.Name = filepath.Base(machine.Checkout)
		}
	}
	sort.SliceStable(reading.Machines, func(i, j int) bool {
		left, right := reading.Machines[i], reading.Machines[j]
		if left.This != right.This {
			return left.This
		}
		return left.Name < right.Name
	})
	return reading
}

// readHostMachines is discoverHostMachines with each machine's status read
// by the stop transition status uses, this user's running launches placed
// in the checkout they work in, and the lane owner's view.
func (inv *intentInvocation) readHostMachines() hostReading {
	reading := inv.discoverHostMachines()
	notOurs := map[int64]bool{}
	for _, machine := range reading.Machines {
		if machine.State == "unknown" {
			continue
		}
		child := inv.checkoutInvocation(machine.Checkout)
		scope, scale, problem := child.selectProcessScope()
		if problem != nil {
			machine.State, machine.Reason = "unknown", problem.Summary
			continue
		}
		report, err := inv.owners.processes.process.status(scope, scale)
		if err != nil {
			machine.State, machine.Reason = "unknown", "status is unknown: "+err.Error()
			continue
		}
		machine.StatusLines = nonNilLines(report.Lines)
		machine.FenceState, machine.FenceChangedAt = report.FenceState, report.FenceChangedAt
		machine.State = "stopped"
		for _, item := range report.Items {
			process := machineProcess{Family: item.Family, Component: item.Component, ID: item.ID, Pid: item.Pid, Line: item.Line, started: item.PidStartedAt}
			if item.PidStartedAt > 0 {
				process.Since = time.Unix(item.PidStartedAt, 0).UTC().Format(time.RFC3339)
			}
			switch {
			case item.Family == "untracked":
				if !notOurs[item.Pid] {
					notOurs[item.Pid] = true
					reading.NotOurs = append(reading.NotOurs, process)
				}
				continue
			case item.Family == "steward" || item.Family == "supervision":
				machine.Components = append(machine.Components, process)
			default:
				machine.Work = append(machine.Work, process)
			}
			machine.State = "running"
		}
		if report.ExitCode != 0 {
			machine.State, machine.Reason = "unknown", "status incomplete: "+strings.Join(report.Lines, "; ")
		}
	}
	inv.placeLaunches(&reading)
	if home := reading.laneHome; home != "" {
		view := inv.laneView(inv.landing(), home)
		for _, machine := range reading.Machines {
			if machine.Lane {
				owner := view.Owner
				machine.LaneOwner = &owner
			}
		}
	}
	return reading
}

// placeLaunches puts each running launch of this user in the machine whose
// checkout it works in; the rest are elsewhere.
func (inv *intentInvocation) placeLaunches(reading *hostReading) {
	if inv.owners.processes.launches == nil {
		return
	}
	records, err := inv.owners.processes.launches().List()
	if err != nil {
		reading.LaunchProblem = "this user's launches cannot be listed: " + err.Error()
		return
	}
	for _, record := range records {
		if record.State.Terminal() {
			continue
		}
		job := intentJob{id: record.ID, kind: "launch", launch: record}
		view := machineLaunch{Reference: jobReference(job), Purpose: jobPurpose(job)}
		placed := false
		for _, machine := range reading.Machines {
			if machinePathWithin(record.WorkingDirectory, machine.Checkout) {
				machine.Launches = append(machine.Launches, view)
				machine.launchRecords = append(machine.launchRecords, record)
				placed = true
				break
			}
		}
		if !placed {
			reading.LaunchesElsewhere = append(reading.LaunchesElsewhere, view)
			reading.launchesElsewhere = append(reading.launchesElsewhere, record)
		}
	}
}

// machinePathWithin reports whether path is root or lies below it.
func machinePathWithin(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// machineComponentNames are the helpers' names as a person knows them.
var machineComponentNames = map[string]string{
	"steward-runner":    "steward runner",
	"supervision-owner": "supervision owner",
	"watcher":           "repo watcher",
	"reaper":            "job reaper",
	"landing-owner":     "landing batch owner",
	"narrator":          "narrator",
}

func machineLocalTime(epoch int64) string {
	return time.Unix(epoch, 0).Local().Format("2006-01-02 15:04 MST")
}

// processLine is one process of a machine as --verbose prints it.
func (p machineProcess) processLine() string {
	since := ""
	if p.started > 0 {
		since = " since " + machineLocalTime(p.started)
	}
	if name, ok := machineComponentNames[p.Component]; ok && p.Family != "untracked" {
		return fmt.Sprintf("%s pid %d%s", name, p.Pid, since)
	}
	line := strings.TrimSuffix(p.Line, ": running")
	if p.Family == "untracked" {
		line = strings.TrimPrefix(line, "untracked ")
	}
	return line + since
}

// otherComputers are the fleet's machines this computer does not run.
func otherComputers(report seat.Report, reading hostReading) []otherComputerMachine {
	local := map[string]bool{}
	for _, machine := range reading.Machines {
		if machine.Nickname {
			local[machine.Name] = true
		}
	}
	others := []otherComputerMachine{}
	for _, standing := range report.Machines {
		if local[standing.Machine] || standing.This {
			continue
		}
		other := otherComputerMachine{Machine: standing.Machine, Standing: string(standing.Standing), OnAnotherComputer: reading.RegistryProblem == ""}
		if standing.Record != nil {
			other.LastReported = standing.Record.TickAt
		}
		others = append(others, other)
	}
	return others
}

func plural(count int, one, many string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, one)
	}
	return fmt.Sprintf("%d %s", count, many)
}

// machineListSummary is the one line machine list prints first.
func machineListSummary(reading hostReading, others int) string {
	running, stopped, unknown, jobs := 0, 0, 0, len(reading.LaunchesElsewhere)
	for _, machine := range reading.Machines {
		switch machine.State {
		case "running":
			running++
		case "stopped":
			stopped++
		default:
			unknown++
		}
		jobs += len(machine.Launches)
		for _, work := range machine.Work {
			if work.Family == "job" {
				jobs++
			}
		}
	}
	count := plural(len(reading.Machines), "machine", "machines")
	if reading.RegistryProblem != "" {
		count = "at least " + count
	}
	states := fmt.Sprintf("%d running, %d stopped", running, stopped)
	if unknown > 0 {
		states += fmt.Sprintf(", %d unknown", unknown)
	}
	other := plural(others, "on another computer", "on other computers")
	if reading.RegistryProblem != "" {
		other = fmt.Sprintf("%d not found on this computer", others)
	}
	summary := fmt.Sprintf("%s on this computer: %s; %s running; %s", count, states, plural(jobs, "job", "jobs"), other)
	if reading.RegistryProblem != "" {
		summary += "; " + reading.RegistryProblem + ", so this computer may run more"
	}
	return summary
}

// machineListDetail is --verbose: each machine of this computer, the
// machines on other computers, and what is not ours.
func machineListDetail(reading hostReading, others []otherComputerMachine) []string {
	lines := []string{"on this computer:"}
	for _, machine := range reading.Machines {
		header := fmt.Sprintf("%s  %s  %s", machine.Name, machine.State, machine.Checkout)
		var roles []string
		if machine.This {
			roles = append(roles, "this checkout")
		}
		if machine.Lane {
			roles = append(roles, "landing lane")
		}
		if len(roles) > 0 {
			header += " (" + strings.Join(roles, ", ") + ")"
		}
		lines = append(lines, header)
		if machine.Reason != "" {
			lines = append(lines, "  "+machine.Reason)
		}
		for _, process := range machine.Components {
			lines = append(lines, "  "+process.processLine())
		}
		for _, process := range machine.Work {
			lines = append(lines, "  "+process.processLine())
		}
		for _, launched := range machine.Launches {
			lines = append(lines, "  launch "+launched.Reference+": "+launched.Purpose)
		}
		if machine.LaneOwner != nil {
			owner := "  landing lane owner: " + machine.LaneOwner.State
			if machine.LaneOwner.PID != nil {
				owner += fmt.Sprintf(", pid %d", *machine.LaneOwner.PID)
			}
			if machine.LaneOwner.Since != nil {
				owner += ", since " + lane.LocalText(*machine.LaneOwner.Since)
			}
			lines = append(lines, owner)
		}
		if machine.State == "stopped" && machine.FenceState == stopfence.StateClosed {
			lines = append(lines, "  stopped since "+lane.LocalText(machine.FenceChangedAt))
		}
	}
	if len(reading.LaunchesElsewhere) > 0 {
		lines = append(lines, "launches outside every machine's checkout:")
		for _, launched := range reading.LaunchesElsewhere {
			lines = append(lines, "  launch "+launched.Reference+": "+launched.Purpose)
		}
	}
	for _, problem := range []string{reading.RegistryProblem, reading.LaneProblem, reading.LaunchProblem} {
		if problem != "" {
			lines = append(lines, problem)
		}
	}
	if len(others) > 0 {
		where := "on another computer"
		if reading.RegistryProblem != "" {
			where = "not found on this computer"
		}
		lines = append(lines, map[bool]string{true: "on other computers:", false: "not found on this computer (the host registry cannot be read, so each may be a stopped checkout here or a machine on another computer):"}[reading.RegistryProblem == ""])
		for _, other := range others {
			reported := "never reported"
			if other.LastReported != "" {
				reported = "last reported " + lane.LocalText(other.LastReported)
			}
			lines = append(lines, fmt.Sprintf("%s  %s: %s", other.Machine, where, reported))
		}
	}
	if len(reading.NotOurs) > 0 {
		lines = append(lines, "not ours, not touched:")
		for _, process := range reading.NotOurs {
			lines = append(lines, "  "+process.processLine())
		}
	}
	return lines
}

// runIntentMachineList is every machine's presence, this computer's
// machines summarized first.
func runIntentMachineList(inv *intentInvocation) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	report, err := inv.owners.processes.fleet(inv.layout.GitRoot, inv.input.switched("refresh"), seatFleetNow())
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "fleet: " + err.Error()})
	}
	encoded, err := report.JSON()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "fleet: " + err.Error()})
	}
	data := map[string]any{}
	if err := json.Unmarshal(encoded, &data); err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "fleet: " + err.Error()})
	}
	reading := inv.readHostMachines()
	others := otherComputers(report, reading)
	data["thisComputer"], data["otherComputers"] = reading, others
	text := intentOwnerLines(report.Text())
	if inv.input.switched("verbose") {
		text = append(text, machineListDetail(reading, others)...)
	}
	result := intentResult{Outcome: intentConfirmed, Summary: machineListSummary(reading, len(others)), text: text, Data: data}
	if reading.RegistryProblem != "" {
		result.Outcome, result.code = intentPartial, 1
		result.Decision = "repair or remove the host registry named above; until then machine list names only the machines the other sources name"
	}
	return inv.render(result)
}

// runIntentMachineStop stops MetaSystem on one machine of this computer, or
// on every one, each through system stop.
func runIntentMachineStop(inv *intentInvocation) int {
	all := inv.input.switched("all")
	if all == (len(inv.input.args) == 1) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "machine stop needs one machine's name or --all, not both; nothing was done",
			next:    inv.publicArgv("machine", "list", "--verbose"), nextReason: "names every machine of this computer"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	reading := inv.discoverHostMachines()
	targets := reading.Machines
	if !all {
		name := inv.input.args[0]
		matched, problem := inv.matchMachine(reading, name)
		if problem != nil {
			return inv.render(*problem)
		}
		targets = []*hostMachine{matched}
	}
	inv.placeLaunches(&reading)
	return inv.render(inv.stopHostMachines(reading, targets, all))
}

// matchMachine finds the one machine of this computer name names: its
// nickname, its checkout's base name or a path it was named by. A machine
// only the fleet names runs on another computer.
func (inv *intentInvocation) matchMachine(reading hostReading, name string) (*hostMachine, *intentResult) {
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(inv.cwd, path)
	}
	if layout, err := inv.owners.resolver.ResolveLayout(path); err == nil && strings.ContainsRune(name, filepath.Separator) {
		path = layout.GitRoot
	}
	var matches []*hostMachine
	for _, machine := range reading.Machines {
		if machine.Name == name || filepath.Base(machine.Checkout) == name || machine.Checkout == path || slices.Contains(machine.paths, name) {
			matches = append(matches, machine)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
	default:
		var named []string
		for _, machine := range matches {
			named = append(named, machine.Checkout)
		}
		return nil, &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("%s names %d machines of this computer (%s); nothing was done", name, len(matches), strings.Join(named, ", ")),
			next:    inv.publicArgv("machine", "stop", "PATH"), nextReason: "name the machine by its checkout path"}
	}
	if report, err := inv.owners.processes.fleet(inv.layout.GitRoot, false, seatFleetNow()); err == nil {
		for _, other := range otherComputers(report, reading) {
			if other.Machine != name || !other.OnAnotherComputer {
				continue
			}
			reported := "it has never reported"
			if other.LastReported != "" {
				reported = "it last reported " + lane.LocalText(other.LastReported)
			}
			return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "machine", ID: name}},
				Summary:  fmt.Sprintf("%s: %s runs on another computer (%s), where this computer's stop cannot reach its processes; stop it on that computer: metasystem system stop --repo PATH; nothing was done", codeMachineOnAnotherComputer, name, reported),
				Decision: "on that computer, run metasystem system stop --repo PATH with its checkout's path"}
		}
	}
	if reading.RegistryProblem != "" {
		return nil, &intentResult{Outcome: intentFailed, code: 1,
			Summary: fmt.Sprintf("%s is not among the machines this computer can name, and %s; nothing was done", name, reading.RegistryProblem),
			next:    inv.publicArgv("system", "stop", "--repo", "PATH"), nextReason: "stop a checkout of this computer by its path"}
	}
	return nil, &intentResult{Outcome: intentRefused, code: 2,
		Summary: fmt.Sprintf("no machine %s on this computer or in the fleet; nothing was done", name),
		next:    inv.publicArgv("machine", "list", "--verbose"), nextReason: "names every machine of this computer"}
}

// stopHostMachines runs system stop for each target, then cancels this
// user's running launches in each stopped checkout (with --all, every one)
// as work stop cancels them. A first stop that is refused, the proof that
// a person asks, ends the act before any machine or launch is touched.
func (inv *intentInvocation) stopHostMachines(reading hostReading, targets []*hostMachine, all bool) intentResult {
	var stoppable, skipped []*hostMachine
	for _, machine := range targets {
		if machine.State == "unknown" {
			skipped = append(skipped, machine)
			continue
		}
		if _, _, problem := inv.checkoutInvocation(machine.Checkout).selectProcessScope(); problem != nil {
			machine.Reason = problem.Summary
			skipped = append(skipped, machine)
			continue
		}
		stoppable = append(stoppable, machine)
	}
	stoppedAlready, stopped, unfinished := 0, 0, 0
	var lines []string
	var launches []launch.Record
	views := []map[string]any{}
	for index, machine := range stoppable {
		result := systemStopResult(inv.checkoutInvocation(machine.Checkout))
		machine.stopResult = &result
		if index == 0 && result.Outcome == intentRefused {
			result.Summary = fmt.Sprintf("machine stop refused at %s: %s; no machine was stopped", machine.Name, result.Summary)
			result.Targets = []intentTarget{{Kind: "machine", ID: machine.Name}}
			return result
		}
		views = append(views, map[string]any{"name": machine.Name, "checkout": machine.Checkout, "outcome": result.Outcome, "summary": result.Summary, "lines": nonNilLines(result.text)})
		lines = append(lines, machine.Name+": "+result.Summary)
		if inv.input.switched("verbose") {
			for _, line := range result.text {
				if line != result.Summary {
					lines = append(lines, "  "+line)
				}
			}
		}
		switch result.Outcome {
		case intentUnchanged:
			stoppedAlready++
		case intentConfirmed:
			stopped++
		default:
			unfinished++
			continue
		}
		launches = append(launches, machine.launchRecords...)
	}
	if all && unfinished == 0 {
		launches = append(launches, reading.launchesElsewhere...)
	}
	cancelled, cancelFailed := 0, 0
	for _, record := range launches {
		result := inv.stopResolvedJob(intentJob{id: record.ID, kind: "launch", launch: record})
		lines = append(lines, "launch "+launchJobPrefix+record.ID+": "+result.Summary)
		if result.Outcome == intentConfirmed {
			cancelled++
		} else if result.Outcome != intentUnchanged {
			cancelFailed++
		}
	}
	for _, machine := range skipped {
		lines = append(lines, fmt.Sprintf("%s: not stopped, it cannot be read from here: %s", machine.Name, machine.Reason))
	}
	if reading.RegistryProblem != "" && all {
		lines = append(lines, reading.RegistryProblem+"; a machine only it names may still run")
	}
	data := map[string]any{"machines": views, "launchesCancelled": cancelled, "registryProblem": reading.RegistryProblem}
	targetsOut := []intentTarget{}
	for _, machine := range targets {
		targetsOut = append(targetsOut, intentTarget{Kind: "machine", ID: machine.Name})
	}
	result := intentResult{Targets: targetsOut, text: lines, Data: data}
	if !inv.input.switched("verbose") && !inv.input.switched("json") && len(stoppable) > 0 && unfinished == 0 && cancelFailed == 0 && len(skipped) == 0 {
		// Summary by default: the per-machine lines are --verbose's.
		result.text = nil
	}
	launchWords := ""
	if cancelled > 0 {
		launchWords = "; " + plural(cancelled, "launch", "launches") + " cancelled"
	}
	incomplete := unfinished > 0 || cancelFailed > 0 || len(skipped) > 0 || (all && reading.RegistryProblem != "")
	switch {
	case incomplete:
		result.Outcome, result.code = intentPartial, 1
		result.Summary = fmt.Sprintf("machine stop did not finish: %d of %d machines of this computer stopped or already stopped; what did not stop is listed below", stopped+stoppedAlready, len(targets))
		result.next, result.nextReason = inv.publicArgv(append([]string{"machine", "stop"}, inv.machineStopTargetArgs()...)...), "stop again; a machine that cannot be read is stopped by its own system stop --repo PATH"
		result.text = lines
	case stopped == 0 && cancelled == 0 && !all:
		result.Outcome = intentUnchanged
		result.Summary = fmt.Sprintf("MetaSystem is already stopped on %s (%s); %s", targets[0].Name, targets[0].Checkout, machineStoppedTail(targets[0]))
	case stopped == 0 && cancelled == 0:
		result.Outcome = intentUnchanged
		result.Summary = fmt.Sprintf("MetaSystem is already stopped on all %s of this computer; nothing of MetaSystem's is running", plural(len(targets), "machine", "machines"))
	case !all:
		result.Outcome = intentConfirmed
		result.Summary = fmt.Sprintf("stopped MetaSystem on %s (%s)%s", targets[0].Name, targets[0].Checkout, launchWords)
	default:
		result.Outcome = intentConfirmed
		result.Summary = fmt.Sprintf("stopped MetaSystem on %s of this computer", plural(len(targets), "machine", "machines"))
		if stoppedAlready > 0 {
			result.Summary += fmt.Sprintf(" (%d already stopped)", stoppedAlready)
		}
		result.Summary += launchWords
	}
	return result
}

// machineStoppedTail is the rest of system stop's already-stopped line.
func machineStoppedTail(machine *hostMachine) string {
	if machine.stopResult == nil {
		return "nothing of MetaSystem's is running"
	}
	summary := machine.stopResult.Summary
	if index := strings.Index(summary, "; "); index >= 0 {
		summary = summary[index+2:]
	}
	if index := strings.Index(summary, "; start again"); index >= 0 {
		summary = summary[:index]
	}
	return summary
}

func (inv *intentInvocation) machineStopTargetArgs() []string {
	if inv.input.switched("all") {
		return []string{"--all"}
	}
	return inv.input.args
}
