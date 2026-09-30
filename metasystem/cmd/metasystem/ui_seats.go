package main

// The interface's lifecycle verbs know the other seats of this computer:
// when this seat runs no interface, stop stops the one other machine that
// does, status names it, and start refuses the address it holds. Another
// seat's interface is only ever judged and stopped through its own record,
// by the lifecycle owner's exact-identity proof.

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// uiSeat is one other machine of this computer and its interface's roots.
type uiSeat struct {
	Name     string
	Checkout string
	Roots    lifecycle.Roots
}

// uiSeatInventory is the other machines of this computer as machine list
// names them, and this seat's own name. Problems says why the list may be
// incomplete; a list with a problem never authorizes a stop across seats.
type uiSeatInventory struct {
	This     string
	Seats    []uiSeat
	Problems []string
}

// uiSeatView is one other machine's interface as a verb names it.
type uiSeatView struct {
	Machine  string          `json:"machine"`
	Checkout string          `json:"checkout"`
	State    lifecycle.State `json:"state"`
	Pid      int64           `json:"pid,omitempty"`
	Address  string          `json:"address,omitempty"`
	Problem  string          `json:"problem,omitempty"`
}

// uiOtherSeats reads the machines of this computer as machine list does,
// without refreshing the fleet's presence, and derives each other one's
// interface roots as its own verbs derive them.
func (inv *intentInvocation) uiOtherSeats(layout stateroot.Layout) uiSeatInventory {
	reader := *inv
	reader.layout = layout
	inventory := uiSeatInventory{This: filepath.Base(layout.GitRoot)}
	if name, ok := reader.machineSeams().nickname(layout.GitRoot); ok {
		inventory.This = name
	}
	report, err := inv.owners.processes.fleet(layout.GitRoot, false, seatFleetNow())
	switch {
	case err != nil:
		inventory.Problems = append(inventory.Problems, "the fleet's presence cannot be read: "+err.Error())
	default:
		if report.CopyProblem != "" {
			inventory.Problems = append(inventory.Problems, "the fleet's presence copy cannot be read: "+report.CopyProblem)
		}
		if report.ClaimsUnavailable != "" {
			inventory.Problems = append(inventory.Problems, "the fleet's claims cannot be read: "+report.ClaimsUnavailable)
		}
	}
	reading := reader.discoverHostMachines(fleetNames(report))
	for _, problem := range []string{reading.RegistryProblem, reading.LaneProblem} {
		if problem != "" {
			inventory.Problems = append(inventory.Problems, problem)
		}
	}
	for _, machine := range reading.Machines {
		if machine.This {
			continue
		}
		if machine.State == "unknown" {
			inventory.Problems = append(inventory.Problems, fmt.Sprintf("machine %s: %s", machine.Name, machine.Reason))
			continue
		}
		child := inv.checkoutInvocation(machine.Checkout)
		seatLayout, installation, _, problem := child.selectInstallation()
		if problem != nil {
			inventory.Problems = append(inventory.Problems, fmt.Sprintf("machine %s: %s", machine.Name, problem.Summary))
			continue
		}
		roots, err := lifecycle.ResolveRootsWith(inv.owners.processes.process.repositoryTop, inv.owners.resolver.RootForInstallation, seatLayout.GitRoot, installation)
		if err != nil {
			inventory.Problems = append(inventory.Problems, fmt.Sprintf("machine %s: %v", machine.Name, err))
			continue
		}
		inventory.Seats = append(inventory.Seats, uiSeat{Name: machine.Name, Checkout: machine.Checkout, Roots: roots})
	}
	return inventory
}

// uiSeatCandidate is another seat whose interface is neither stopped nor a
// stale record: it runs, or it cannot be proven not to.
type uiSeatCandidate struct {
	seat uiSeat
	view uiSeatView
}

// uiSeatCandidates reads each seat's record as ui status reads it there,
// repairing a dead server's stale record as it does.
func uiSeatCandidates(inventory uiSeatInventory, prober identity.Prober) []uiSeatCandidate {
	var candidates []uiSeatCandidate
	for _, other := range inventory.Seats {
		view := uiSeatView{Machine: other.Name, Checkout: other.Checkout}
		status, err := lifecycle.Read(other.Roots.StateRoot, prober)
		if err != nil {
			view.State, view.Problem = lifecycle.Unreadable, err.Error()
		} else {
			view.State = status.State
			if status.State == lifecycle.Stopped || status.State == lifecycle.Stale {
				continue
			}
			if status.Record != nil {
				view.Address = status.Record.Address
				if ref, parseErr := identity.ParseRef(status.Record.Process); parseErr == nil {
					view.Pid = ref.Pid
				}
			}
		}
		candidates = append(candidates, uiSeatCandidate{seat: other, view: view})
	}
	return candidates
}

func uiSeatViews(candidates []uiSeatCandidate) []uiSeatView {
	views := []uiSeatView{}
	for _, candidate := range candidates {
		views = append(views, candidate.view)
	}
	return views
}

// uiSeatStopLine names one candidate with the stop that acts on it.
func uiSeatStopLine(view uiSeatView) string {
	command := shellCommand([]string{"metasystem", "ui", "stop", "--repo", view.Checkout})
	if view.State == lifecycle.Running {
		return fmt.Sprintf("machine %s: pid %d at %s: %s", view.Machine, view.Pid, view.Address, command)
	}
	return fmt.Sprintf("machine %s: %s: %s", view.Machine, uiSeatState(view), command)
}

func uiSeatState(view uiSeatView) string {
	if view.Problem != "" {
		return string(view.State) + " (" + view.Problem + ")"
	}
	return string(view.State)
}

func uiSeatsProblemLine(problems []string) string {
	return "the machines of this computer could not all be read: " + strings.Join(problems, "; ")
}

// uiStopAcrossSeats is a stop whose own seat ran no interface: exactly one
// other machine running one is stopped through its own record; several, or
// an inventory that may be incomplete, stop nothing and are listed.
func uiStopAcrossSeats(own lifecycle.Result, inventory uiSeatInventory, stop lifecycle.StopOptions) uiLifecycleResult {
	candidates := uiSeatCandidates(inventory, stop.Prober)
	views := uiSeatViews(candidates)
	problems := append([]string{}, inventory.Problems...)
	if len(problems) > 0 {
		lines := append(append([]string{}, own.Lines...), uiSeatsProblemLine(problems)+"; nothing was stopped")
		for _, view := range views {
			lines = append(lines, uiSeatStopLine(view))
		}
		return uiLifecycleResult{Result: lifecycle.Result{Lines: lines, Code: 1}, Seats: views, SeatsProblems: problems}
	}
	prefix := "no interface for " + inventory.This + "; "
	switch len(candidates) {
	case 0:
		return uiLifecycleResult{Result: own, Unchanged: true, Seats: views, SeatsProblems: problems}
	case 1:
		candidate := candidates[0]
		stopped, unchanged := lifecycle.StopReport(candidate.seat.Roots.StateRoot, stop)
		acted := candidate.seat
		result := uiLifecycleResult{Seat: &acted, Unchanged: unchanged, Seats: views, SeatsProblems: problems}
		if stopped.Code == 0 && !unchanged {
			port := candidate.view.Address
			if _, onlyPort, err := net.SplitHostPort(port); err == nil {
				port = onlyPort
			}
			result.Result = lifecycle.Result{Lines: []string{fmt.Sprintf("%sstopped the interface of machine %s (pid %d, :%s)", prefix, acted.Name, candidate.view.Pid, port)}}
			return result
		}
		lines := make([]string, 0, len(stopped.Lines))
		for _, line := range stopped.Lines {
			lines = append(lines, prefix+"machine "+acted.Name+": "+line)
		}
		result.Result = lifecycle.Result{Lines: lines, Code: stopped.Code}
		return result
	}
	lines := []string{fmt.Sprintf("%s%d machines of this computer run one; nothing was done", prefix, len(candidates))}
	for _, view := range views {
		lines = append(lines, uiSeatStopLine(view))
	}
	return uiLifecycleResult{Result: lifecycle.Result{Lines: lines, Code: 1}, Seats: views, SeatsProblems: problems}
}

// uiStatusAcrossSeats adds to a status whose own seat runs no interface
// the other machines that run one, or cannot be proven not to.
func uiStatusAcrossSeats(result uiLifecycleResult, inventory uiSeatInventory, prober identity.Prober) uiLifecycleResult {
	candidates := uiSeatCandidates(inventory, prober)
	lines := append([]string{}, result.Result.Lines...)
	for _, candidate := range candidates {
		view := candidate.view
		if view.State == lifecycle.Running {
			lines = append(lines, fmt.Sprintf("machine %s runs one (pid %d at %s): %s", view.Machine, view.Pid, view.Address,
				shellCommand([]string{"metasystem", "ui", "status", "--repo", view.Checkout})))
			continue
		}
		lines = append(lines, fmt.Sprintf("machine %s: %s", view.Machine, uiSeatState(view)))
	}
	if len(inventory.Problems) > 0 {
		lines = append(lines, uiSeatsProblemLine(inventory.Problems))
	}
	result.Result.Lines = lines
	result.Seats, result.SeatsProblems = uiSeatViews(candidates), append([]string{}, inventory.Problems...)
	return result
}

// uiStartCollision is the other machine whose running interface holds the
// address this start asks for, if the inventory knows one. A port-0 listen
// asks for any port, so nothing holds it.
func uiStartCollision(inventory uiSeatInventory, prober identity.Prober, listen string) (lifecycle.Result, *uiSeatView) {
	if _, port, err := net.SplitHostPort(listen); err != nil || port == "0" {
		return lifecycle.Result{}, nil
	}
	for _, candidate := range uiSeatCandidates(inventory, prober) {
		view := candidate.view
		if view.State != lifecycle.Running || view.Address != listen {
			continue
		}
		line := fmt.Sprintf("no interface for %s; machine %s runs one at %s (pid %d); stop it with: %s, or start this seat's at another address with --listen",
			inventory.This, view.Machine, view.Address, view.Pid, shellCommand([]string{"metasystem", "ui", "stop", "--repo", view.Checkout}))
		return lifecycle.Result{Lines: []string{line}, Code: 1}, &view
	}
	return lifecycle.Result{}, nil
}

// uiInstallationEngine is the engine an installation carries: its own
// bin/metasystem, which ui start and ui restart launch whoever typed them.
func uiInstallationEngine(installation string) (string, error) {
	binary := filepath.Join(installation, "bin", "metasystem")
	if !regularFile(binary) {
		return "", fmt.Errorf("the installation %s carries no engine at bin/metasystem", shellCommand([]string{installation}))
	}
	return binary, nil
}

// uiEngineDecision is what a person does about an installation without an
// engine.
const uiEngineDecision = "build and install this checkout's engine (go run ./cmd/devgate build)"

// uiEngineFor resolves the target's engine before anything is stopped or
// started; a missing one is a refusal that changed nothing.
func uiEngineFor(engine func(string) (string, error), target lifecycle.Roots, prefix string) (string, *uiLifecycleResult) {
	binary, err := engine(target.Installation)
	if err != nil {
		return "", &uiLifecycleResult{Result: lifecycle.Result{Lines: []string{prefix + err.Error() + "; nothing was done"}, Code: 1}, Decision: uiEngineDecision}
	}
	return binary, nil
}

// uiFileDigest is an engine file's digest in the form the server records.
func uiFileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

// uiSeatsExcept is the inventory without the seat a start targets.
func uiSeatsExcept(inventory uiSeatInventory, target lifecycle.Roots) uiSeatInventory {
	others := inventory
	others.Seats = nil
	for _, other := range inventory.Seats {
		if other.Roots.StateRoot != target.StateRoot {
			others.Seats = append(others.Seats, other)
		}
	}
	return others
}

// uiLaunched is the address and pid a launched server reported ready with.
type uiLaunched struct {
	address string
	pid     int
}

// uiRestartAcrossSeats is a restart whose own seat runs no interface:
// exactly one other machine running one, with a complete inventory, is
// restarted by restart; several, or an inventory that may be incomplete,
// restart none and are listed. It answers false when no other machine runs
// one, and this seat's own restart follows.
func uiRestartAcrossSeats(inventory uiSeatInventory, prober identity.Prober, restart func(other uiSeat, prefix string) uiLifecycleResult, launched func() *uiLaunched) (uiLifecycleResult, bool) {
	candidates := uiSeatCandidates(inventory, prober)
	views := uiSeatViews(candidates)
	problems := append([]string{}, inventory.Problems...)
	prefix := "no interface for " + inventory.This + "; "
	restartLine := func(view uiSeatView) string {
		return strings.Replace(uiSeatStopLine(view), "metasystem ui stop --repo", "metasystem ui restart --repo", 1)
	}
	if len(problems) > 0 || len(candidates) > 1 {
		lines := []string{fmt.Sprintf("%s%d machines of this computer run one; nothing was done", prefix, len(candidates))}
		if len(problems) > 0 {
			lines = []string{prefix + uiSeatsProblemLine(problems) + "; nothing was stopped"}
		}
		for _, view := range views {
			lines = append(lines, restartLine(view))
		}
		return uiLifecycleResult{Result: lifecycle.Result{Lines: lines, Code: 1}, Seats: views, SeatsProblems: problems}, true
	}
	if len(candidates) == 0 {
		return uiLifecycleResult{}, false
	}
	candidate := candidates[0]
	acted := candidate.seat
	result := restart(acted, prefix+"machine "+acted.Name+": ")
	result.Seat = &acted
	if result.Seats == nil {
		result.Seats = views
	}
	result.SeatsProblems = problems
	report := result.Restart
	if report == nil {
		return result, true
	}
	if ready := launched(); report.Started && report.Start.Code == 0 && ready != nil {
		port := ready.address
		if _, onlyPort, err := net.SplitHostPort(port); err == nil {
			port = onlyPort
		}
		result.Result = lifecycle.Result{Lines: []string{fmt.Sprintf("%srestarted the interface of machine %s (pid %d -> %d, :%s)", prefix, acted.Name, candidate.view.Pid, ready.pid, port)}}
		return result, true
	}
	lines := make([]string, 0, len(result.Result.Lines))
	for _, line := range result.Result.Lines {
		lines = append(lines, prefix+"machine "+acted.Name+": "+line)
	}
	result.Result.Lines = lines
	return result, true
}
