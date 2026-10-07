package main

// The machines of this computer (Wido, 2026-09-30): one command lists what
// MetaSystem runs in the background on this computer, and one stops it
// everywhere. Nothing here detects a process: each checkout is read by the
// same stop transition status reads, and stopped by system stop itself.
// The machines come from the host registry of armed checkouts, the landing
// lane record and the checkout the command runs in; a registry that cannot
// be read is reported, never read as no machines. A machine is a fleet
// machine (Wido, 2026-09-30): a checkout with a machine nickname that is
// armed now or appears in the fleet's presence, and the landing lane's
// checkout. The registry's other registrations (test beds, scratch and
// builder clones, gone fixtures) are not machines: they are only counted.

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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
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
	// worktreesUnread says Git could not list its registered worktrees
	// when a launch was placed, so a launch elsewhere may be its own.
	worktreesUnread bool
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
	// OtherRegistered counts the host registry's checkouts that are not
	// machines: no nickname, or neither armed nor in the fleet.
	OtherRegistered int `json:"otherRegistered"`
	// FleetProblem says the fleet's presence could not be read, so a
	// stopped machine only it names may be missing.
	FleetProblem  string `json:"fleetProblem,omitempty"`
	LaneProblem   string `json:"laneProblem,omitempty"`
	LaunchProblem string `json:"launchProblem,omitempty"`
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

// fleetNames are the nicknames the fleet's presence names, the reader's own
// included.
func fleetNames(report seat.Report) map[string]bool {
	names := map[string]bool{}
	if report.This != "" {
		names[report.This] = true
	}
	for _, standing := range report.Machines {
		names[standing.Machine] = true
	}
	return names
}

type hostCheckout struct{ path, source string }

// discoverHostCheckouts reads local registrations without machine filtering.
func (inv *intentInvocation) discoverHostCheckouts() ([]hostCheckout, hostReading) {
	owners := inv.machineSeams()
	reading := hostReading{LaunchesElsewhere: []machineLaunch{}, NotOurs: []machineProcess{}}
	var candidates []hostCheckout
	if inv.layout.GitRoot != "" {
		candidates = append(candidates, hostCheckout{inv.layout.GitRoot, "this checkout"})
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
			candidates = append(candidates, hostCheckout{checkout.Path, source})
		}
	}
	landing := inv.landing()
	if home, err := landing.home(); err != nil {
		reading.LaneProblem = "the landing lane cannot be read: " + err.Error()
	} else if record, ok, err := lane.Read(home); err != nil {
		reading.LaneProblem = "the landing lane cannot be read: " + err.Error()
	} else if ok {
		reading.laneHome = home
		candidates = append(candidates, hostCheckout{record.Root, "landing lane"})
	}
	return candidates, reading
}

// discoverHostMachines names this computer's machines: of the checkouts the
// host registry records and this checkout, each once by its Git root, those
// with a nickname that are armed or that fleet names; and the landing lane's
// checkout. This checkout comes first, then by name. The other registered
// checkouts are counted in OtherRegistered, never listed or stopped.
func (inv *intentInvocation) discoverHostMachines(fleet map[string]bool) hostReading {
	owners := inv.machineSeams()
	candidates, reading := inv.discoverHostCheckouts()
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
	machines := reading.Machines[:0]
	for _, machine := range reading.Machines {
		if name, ok := owners.nickname(machine.Checkout); ok {
			machine.Name, machine.Nickname = name, true
		} else {
			machine.Name = filepath.Base(machine.Checkout)
		}
		armed := slices.Contains(machine.Sources, "registry: armed")
		if machine.Lane || (machine.Nickname && (armed || fleet[machine.Name])) {
			machines = append(machines, machine)
			continue
		}
		if armed || slices.Contains(machine.Sources, "registry: stopped") {
			reading.OtherRegistered++
		}
	}
	reading.Machines = machines
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
// in the checkout they work in, and the lane's landing agent.
func (inv *intentInvocation) readHostMachines(fleet map[string]bool) hostReading {
	reading := inv.discoverHostMachines(fleet)
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
	inv.placeLaunchRecords(reading, records)
}

// placeLaunchRecords places each running record: in the machine whose
// checkout is its working directory or holds it; else in the machine one
// of whose registered worktrees (git worktree list) is or holds it, where
// a goal's build runs as the sibling <checkout>-<goal>; else elsewhere.
// A worktree two machines' repositories share goes to the machine whose
// checkout path shares the longest prefix with the launch's, so every
// reader places a launch alike whichever checkout it runs from. The
// worktrees are read only for a launch no checkout holds.
func (inv *intentInvocation) placeLaunchRecords(reading *hostReading, records []launch.Record) {
	worktrees := map[*hostMachine][]string{}
	read := map[*hostMachine]bool{}
	var problems []string
	worktreesOf := func(machine *hostMachine) []string {
		if !read[machine] && machine.State != "unknown" {
			read[machine] = true
			registered, err := inv.registeredWorktreesOf(machine.Checkout)
			if err != nil {
				machine.worktreesUnread = true
				problems = append(problems, fmt.Sprintf("the worktrees of %s cannot be listed, so a launch in one may be missing from it: %v", machine.Checkout, err))
			}
			for path := range registered {
				if path != machine.Checkout {
					worktrees[machine] = append(worktrees[machine], path)
				}
			}
		}
		return worktrees[machine]
	}
	for _, record := range records {
		if record.State.Terminal() {
			continue
		}
		job := intentJob{id: record.ID, kind: "launch", launch: record}
		view := machineLaunch{Reference: jobReference(job), Purpose: jobPurpose(job)}
		var owner *hostMachine
		for _, machine := range reading.Machines {
			if machinePathWithin(record.WorkingDirectory, machine.Checkout) {
				owner = machine
				break
			}
		}
		if owner == nil && record.WorkingDirectory != "" {
			shared := -1
			for _, machine := range reading.Machines {
				for _, worktree := range worktreesOf(machine) {
					if !machinePathWithin(record.WorkingDirectory, worktree) {
						continue
					}
					if common := commonPrefixLength(machine.Checkout, record.WorkingDirectory); common > shared || (common == shared && machine.Checkout < owner.Checkout) {
						owner, shared = machine, common
					}
				}
			}
		}
		if owner != nil {
			owner.Launches = append(owner.Launches, view)
			owner.launchRecords = append(owner.launchRecords, record)
			continue
		}
		reading.LaunchesElsewhere = append(reading.LaunchesElsewhere, view)
		reading.launchesElsewhere = append(reading.launchesElsewhere, record)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		problems = append([]string{reading.LaunchProblem}, problems...)
		if problems[0] == "" {
			problems = problems[1:]
		}
		reading.LaunchProblem = strings.Join(problems, "; ")
	}
}

// commonPrefixLength is how many leading bytes a and b share.
func commonPrefixLength(a, b string) int {
	length := 0
	for length < len(a) && length < len(b) && a[length] == b[length] {
		length++
	}
	return length
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

// processLine is one process of a machine as --verbose prints it.
func (p machineProcess) processLine(env textui.Env) string {
	since := ""
	if p.started > 0 {
		since = " " + env.Since(time.Unix(p.started, 0))
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
	running, stopped, unknown, jobs := machineCounts(reading)
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
func machineListDetail(reading hostReading, others []otherComputerMachine, env textui.Env) []string {
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
			lines = append(lines, "  "+process.processLine(env))
		}
		for _, process := range machine.Work {
			lines = append(lines, "  "+process.processLine(env))
		}
		for _, launched := range machine.Launches {
			lines = append(lines, "  launch "+launched.Reference+": "+launched.Purpose)
		}
		if machine.LaneOwner != nil {
			owner := "  landing agent: " + machine.LaneOwner.State
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
	if reading.OtherRegistered > 0 {
		lines = append(lines, plural(reading.OtherRegistered, "other registered checkout", "other registered checkouts")+" (not machines); metasystem disk clean forgets those whose directories are gone")
	}
	for _, problem := range []string{reading.RegistryProblem, reading.FleetProblem, reading.LaneProblem, reading.LaunchProblem} {
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
			lines = append(lines, "  "+process.processLine(env))
		}
	}
	return lines
}

// machineListFailure is machine list's failure to read the fleet's
// presence: the read may pass, so line 2 is the command again.
func machineListFailure(err error) intentResult {
	return intentResult{Outcome: intentFailed, code: 1, Summary: "the machines' presence could not be read", retry: "try again",
		Details: []string{"fleet: " + err.Error()}}
}

// runIntentMachineList is every machine's presence, this computer's
// machines summarized first.
func runIntentMachineList(inv *intentInvocation) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	report, err := inv.owners.processes.fleet(inv.layout.GitRoot, inv.input.switched("refresh"), seatFleetNow())
	if err != nil {
		return inv.render(machineListFailure(err))
	}
	encoded, err := report.JSON()
	if err != nil {
		return inv.render(machineListFailure(err))
	}
	data := map[string]any{}
	if err := json.Unmarshal(encoded, &data); err != nil {
		return inv.render(machineListFailure(err))
	}
	reading := inv.readHostMachines(fleetNames(report))
	others := otherComputers(report, reading)
	data["thisComputer"], data["otherComputers"] = reading, others
	result := intentResult{Outcome: intentConfirmed, Summary: machineListSummary(reading, len(others)), Data: data,
		view: inv.machineListView(report, reading, others)}
	if reading.RegistryProblem != "" {
		// A partial reading keeps its lines on the error stream.
		result.text = intentOwnerLines(report.Text())
		if inv.input.switched("verbose") {
			result.text = append(result.text, machineListDetail(reading, others, inv.textEnv(inv.stderr))...)
		}
		result.Outcome, result.code = intentPartial, 1
		result.Decision = "repair or remove the host registry named above; until then machine list names only the machines the other sources name"
	}
	return inv.render(result)
}

// machineListView is machine list's page (output-style §6.3, 6.4): the
// headline counts this computer's machines, their jobs and the machines
// elsewhere; one row per machine of the fleet follows. --verbose adds each
// machine of this computer with its checkout, helpers, jobs, launches and
// the lane's landing agent, the machines on other computers by their last report,
// and the processes that are not MetaSystem's.
func (inv *intentInvocation) machineListView(report seat.Report, reading hostReading, others []otherComputerMachine) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		running, stopped, unknown, jobs := machineCounts(reading)
		count := len(reading.Machines)
		states := fmt.Sprintf("%d running, %d stopped", running, stopped)
		switch {
		case count > 0 && running == count:
			states = "all running"
		case count > 0 && stopped == count:
			states = "all stopped"
		}
		if unknown > 0 {
			states += fmt.Sprintf(", %d unknown", unknown)
		}
		elsewhere := ""
		if len(others) > 0 {
			elsewhere = fmt.Sprintf("%d elsewhere", len(others))
		}
		page.Headline(textui.Count(count, "machine on this computer", "machines on this computer"), states,
			textui.Count(jobs, "job running", "jobs running"), elsewhere)

		local := map[string]*hostMachine{}
		for _, machine := range reading.Machines {
			if machine.Nickname {
				local[machine.Name] = machine
			}
		}
		if report.NoNickname || len(report.Machines) > 0 {
			section := page.Section("Fleet", "")
			if report.NoNickname {
				section.Text("this checkout has no machine nickname and publishes no presence")
			}
			table := section.Table(textui.Column{Title: "machine"}, textui.Column{Title: "doing"}, textui.Column{Title: "seen"},
				textui.Column{Title: "engine"}, textui.Column{Title: "free", Right: true}, textui.Column{Flex: true, Wrap: true})
			for _, standing := range report.Machines {
				table.Row(machineFleetRow(env, standing, local[standing.Machine])...)
			}
		}
		for _, problem := range []string{report.ClaimsUnavailable, report.CopyProblem, reading.LaneProblem, reading.LaunchProblem} {
			if problem != "" {
				page.Section("", "").Item(textui.Alert, problem)
			}
		}
		if !page.Verbose() {
			return
		}
		for _, machine := range reading.Machines {
			machineCard(page, env, machine)
		}
		if len(reading.LaunchesElsewhere) > 0 {
			section := page.Section("Launches outside every machine", "")
			for _, launched := range reading.LaunchesElsewhere {
				section.KV(launched.Reference, textui.Plain(launched.Purpose))
			}
		}
		if len(others) > 0 {
			title := "On other computers"
			if reading.RegistryProblem != "" {
				title = "Not found on this computer"
			}
			table := page.Section(title, "").Table(textui.Column{}, textui.Column{Flex: true})
			for _, other := range others {
				reported := "never reported"
				if at, err := time.Parse(time.RFC3339, other.LastReported); err == nil {
					reported = "last reported " + env.Time(at)
				}
				table.Row(textui.Plain(other.Machine), textui.Dim(reported))
			}
		}
		if len(reading.NotOurs) > 0 {
			table := page.Section("Not ours, left alone", "").Table(textui.Column{Right: true}, textui.Column{}, textui.Column{Flex: true}, textui.Column{})
			for _, process := range reading.NotOurs {
				since := ""
				if process.started > 0 {
					since = env.Since(time.Unix(process.started, 0))
				}
				runtime, argv := untrackedCommand(process.Line)
				table.Row(textui.Plain(fmt.Sprint(process.Pid)), textui.Plain(runtime), textui.Plain(argv), textui.Dim(since))
			}
		}
		if report.Publication != nil || report.CopyReadAt != "" {
			section := page.Section("Presence", "")
			if report.Publication != nil {
				section.KV("this machine", textui.Plain(machinePublication(env, *report.Publication)))
			}
			if at, err := time.Parse(time.RFC3339, report.CopyReadAt); err == nil {
				section.KV("copy", textui.Plain("fetched "+env.Ago(at)+" by "+report.CopySource))
			}
		}
		if reading.OtherRegistered > 0 {
			page.Section("", "").Text(textui.Count(reading.OtherRegistered, "other registered checkout is not a machine", "other registered checkouts are not machines"))
			page.Hint(textui.Hint{Argv: inv.publicArgv("disk", "clean"), Reason: "forgets those whose directories are gone"})
		}
	}
}

// machineCounts are this computer's machines by state, and its running jobs.
func machineCounts(reading hostReading) (running, stopped, unknown, jobs int) {
	jobs = len(reading.LaunchesElsewhere)
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
	return running, stopped, unknown, jobs
}

// machineFleetRow is one machine of the fleet: its state (this computer's
// reading for a machine it runs, its presence otherwise), what it does,
// when it last reported, its engine and free space, and why it needs a look.
func machineFleetRow(env textui.Env, standing seat.MachineStanding, local *hostMachine) []textui.Span {
	state := textui.Running
	switch {
	case standing.Standing == seat.Unreachable:
		state = textui.Alert
	case standing.Standing != seat.Reachable:
		state = textui.Unknown
	case local != nil && local.State == "stopped":
		state = textui.Stopped
	case local != nil && local.State != "running":
		state = textui.Unknown
	}
	name := textui.Marked(state, standing.Machine)
	if standing.Record == nil {
		note := standing.Reason
		if len(standing.Holds) > 0 {
			note += "; named by the claim on " + strings.Join(standing.Holds, ", ")
		}
		return []textui.Span{name, textui.Plain(string(standing.Standing)), textui.Plain(""), textui.Plain(""), textui.Plain(""), textui.Dim(note)}
	}
	record := *standing.Record
	doing := machineActivity(record, env.Now)
	if local != nil && local.State == "stopped" {
		doing = "stopped"
	}
	seen := "never"
	if at := record.At(); !at.IsZero() {
		seen = env.Ago(at)
	}
	free := ""
	if record.DiskFreeBytes != nil {
		free = textui.Bytes(*record.DiskFreeBytes)
	}
	var notes []string
	if standing.Standing != seat.Reachable {
		notes = append(notes, standing.Reason)
	}
	if len(standing.Holds) > 0 {
		holds := "holds " + strings.Join(standing.Holds, ", ")
		if standing.Flag != "" {
			holds += " (" + standing.Flag + ")"
		}
		notes = append(notes, holds)
	}
	return []textui.Span{name, textui.Plain(doing), textui.Plain(seen), textui.Plain(textui.SHA(record.Engine)), textui.Plain(free),
		textui.Dim(strings.Join(notes, "; "))}
}

// machineActivity is what a machine's presence says it does: its phase
// where the record carries one, its chain's older words where it does not.
func machineActivity(record seat.Record, now time.Time) string {
	if record.Working != nil {
		return seat.PhaseWords(record.Working, now)
	}
	if record.Chain == nil {
		return "idle"
	}
	words := "running " + record.Chain.Role
	if record.Chain.Round > 0 {
		words += fmt.Sprintf(" round %d", record.Chain.Round)
	}
	if record.Chain.Goal != "" {
		words += " on " + record.Chain.Goal
	}
	return words
}

// machinePublication is this machine's own publishing state.
func machinePublication(env textui.Env, state seat.PublicationState) string {
	if state.LastOutcome == "" {
		return "no publish has been attempted"
	}
	if at, err := time.Parse(time.RFC3339, state.LastSuccessAt); err == nil && state.LastOutcome == seat.OutcomePublished {
		return "published " + env.Ago(at)
	}
	return state.LastOutcome
}

// machineCard is one machine of this computer as --verbose shows it: its
// checkout and roles, its helpers, jobs and launches, and the lane's agent.
func machineCard(page *textui.Page, env textui.Env, machine *hostMachine) {
	aside := []string{env.Path(machine.Checkout)}
	if machine.This {
		aside = append(aside, "this checkout")
	}
	if machine.Lane {
		aside = append(aside, "landing lane")
	}
	aside = append(aside, machine.State)
	var helpers []string
	var first int64
	for _, process := range machine.Components {
		helpers = append(helpers, fmt.Sprintf("%s %d", helperName(process.Component), process.Pid))
		if process.started > 0 && (first == 0 || process.started < first) {
			first = process.started
		}
	}
	if first > 0 {
		aside = append(aside, env.Since(time.Unix(first, 0)))
	}
	section := page.Section(machine.Name, strings.Join(aside, " · "))
	if machine.Reason != "" {
		section.KV("why", textui.Plain(machine.Reason))
	}
	if len(helpers) > 0 {
		section.KV("pids", textui.Plain(strings.Join(helpers, " · ")))
	}
	for _, process := range machine.Work {
		section.KV("job", textui.Plain(process.processLine(env)))
	}
	for _, launched := range machine.Launches {
		section.KV("launch", textui.Plain(launched.Reference+": "+strings.ReplaceAll(launched.Purpose, machine.Checkout, env.Path(machine.Checkout))))
	}
	if owner := machine.LaneOwner; owner != nil {
		words := owner.State
		if owner.PID != nil {
			words += fmt.Sprintf(", pid %d", *owner.PID)
		}
		if owner.Since != nil {
			if at, err := time.Parse(time.RFC3339, *owner.Since); err == nil {
				words += ", " + env.Since(at)
			}
		}
		section.KV("lane agent", textui.Plain(words))
	}
	if machine.State == "stopped" && machine.FenceState == stopfence.StateClosed {
		if at, err := time.Parse(time.RFC3339, machine.FenceChangedAt); err == nil {
			section.KV("stopped", textui.Plain(env.Since(at)))
		}
	}
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
	fleet := map[string]bool{}
	fleetProblem := ""
	if report, err := inv.owners.processes.fleet(inv.layout.GitRoot, false, seatFleetNow()); err != nil {
		fleetProblem = "the fleet's presence cannot be read (" + err.Error() + "), so a stopped machine only it names is not listed"
	} else {
		fleet = fleetNames(report)
	}
	reading := inv.discoverHostMachines(fleet)
	reading.FleetProblem = fleetProblem
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
			// It is stopped on that computer: metasystem system stop there,
			// with its checkout's path, which this computer cannot know.
			return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "machine", ID: name}},
				Summary: fmt.Sprintf("%s runs on another computer (%s), which this one cannot stop; nothing was done", name, reported),
				next:    []string{"metasystem", "system", "stop", "--repo", "PATH"}, nextReason: "on that computer, with its checkout's path",
				Details: []string{"refusal " + codeMachineOnAnotherComputer + ": this computer's stop cannot reach another computer's processes"}}
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
	stoppedAlready, stopped, unfinished, cancelled, cancelFailed := 0, 0, 0, 0, 0
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
		// system stop cancelled the launches of a checkout it stopped, a
		// line each; an already-stopped checkout's are cancelled below.
		if data, ok := result.Data.(map[string]any); ok {
			launchLines, _ := data["launchLines"].([]string)
			lines = append(lines, launchLines...)
			done, _ := data["launchesCancelled"].(int)
			failed, _ := data["launchesNotCancelled"].(int)
			cancelled, cancelFailed = cancelled+done, cancelFailed+failed
		}
		switch result.Outcome {
		case intentUnchanged:
			stoppedAlready++
			launches = append(launches, machine.launchRecords...)
		case intentConfirmed:
			stopped++
		default:
			unfinished++
		}
	}
	if all && unfinished == 0 {
		launches = append(launches, reading.launchesElsewhere...)
	}
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
	if reading.FleetProblem != "" && all {
		lines = append(lines, reading.FleetProblem+"; it may still run")
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
	incomplete := unfinished > 0 || cancelFailed > 0 || len(skipped) > 0 || (all && (reading.RegistryProblem != "" || reading.FleetProblem != ""))
	switch {
	case incomplete:
		result.Outcome, result.code = intentPartial, 1
		result.Summary = fmt.Sprintf("machine stop did not finish: %d of %d machines of this computer stopped or already stopped; what did not stop is listed below", stopped+stoppedAlready, len(targets))
		result.next, result.nextReason = inv.publicArgv(append([]string{"machine", "stop"}, inv.machineStopTargetArgs()...)...), "stop again; a machine that cannot be read is stopped by its own system stop --repo PATH"
		result.text = lines
	case stopped == 0 && cancelled == 0 && !all:
		result.Outcome = intentUnchanged
		result.Summary = fmt.Sprintf("MetaSystem is already stopped on %s (%s); %s", targets[0].Name, targets[0].Checkout, machineStoppedTail(targets[0]))
		result.view = machineStopView(fmt.Sprintf("MetaSystem is already stopped on %s; %s", targets[0].Name, machineStoppedTail(targets[0])), targets[0], lines)
	case stopped == 0 && cancelled == 0:
		result.Outcome = intentUnchanged
		result.Summary = fmt.Sprintf("MetaSystem is already stopped on all %s of this computer; nothing of MetaSystem's is running", plural(len(targets), "machine", "machines"))
		result.view = machineStopView(result.Summary, nil, lines)
	case !all:
		result.Outcome = intentConfirmed
		result.Summary = fmt.Sprintf("stopped MetaSystem on %s (%s)%s", targets[0].Name, targets[0].Checkout, launchWords)
		result.view = machineStopView("stopped MetaSystem on "+targets[0].Name+launchWords, targets[0], lines)
	default:
		result.Outcome = intentConfirmed
		result.Summary = fmt.Sprintf("stopped MetaSystem on %s of this computer", plural(len(targets), "machine", "machines"))
		if stoppedAlready > 0 {
			result.Summary += fmt.Sprintf(" (%d already stopped)", stoppedAlready)
		}
		result.Summary += launchWords
		result.view = machineStopView(result.Summary, nil, lines)
	}
	return result
}

// machineStopView is a finished machine stop: what it did in one line;
// --verbose adds the machine's checkout and each machine's own stop.
func machineStopView(done string, machine *hostMachine, lines []string) func(*textui.Page) {
	return func(page *textui.Page) {
		page.Done(done)
		if !page.Verbose() {
			return
		}
		if machine != nil {
			page.Facts(textui.KV{Key: "checkout", Value: []textui.Span{textui.Plain(page.Env().Path(machine.Checkout))}})
		}
		if len(lines) > 0 {
			section := page.Section("Each machine", "")
			for _, line := range lines {
				section.Text(strings.TrimSpace(line))
			}
		}
	}
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
