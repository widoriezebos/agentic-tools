package plain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ExecutionAdmission belongs to one attempt's pinned tree and declaration.
// State changes under the queue lock: pending, launched, claimed, or failed.
type ExecutionAdmission struct {
	// Decision pins the complete scope across the detached process boundary.
	Decision *admittedScope `json:"decision,omitempty"`
	// DepthScope marks a batch depth decision, independently of the executed scope.
	DepthScope string       `json:"depthScope,omitempty"`
	DepthBase  string       `json:"depthBase,omitempty"`
	Groups     []string     `json:"groups,omitempty"`
	Lane       *lane.Record `json:"lane,omitempty"`
	Attempt    string       `json:"attempt"`
	Commit     string       `json:"commit"`
	Tree       string       `json:"tree"`
	BatchID    string       `json:"batch-id,omitempty"`
	Gate       bool         `json:"gate,omitempty"`
	Trunk      bool         `json:"trunk,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	Scope      string       `json:"scope"`
	Base       string       `json:"base,omitempty"`
	BaseSHA    string       `json:"baseCommit,omitempty"`
	Command    string       `json:"command"`
	Policy     PolicyValue  `json:"policy"`
	At         string       `json:"at"`
	State      string       `json:"state"`
	Warning    string       `json:"warning,omitempty"`
	Override   bool         `json:"override,omitempty"`
}

// admittedScope keeps the selection and its source facts for the admitted execution.
type admittedScope struct {
	scopeRecord
	Standing *standingEvidence         `json:"standing,omitempty"`
	Source   Result                    `json:"source,omitempty"`
	Contract testpolicy.Contract       `json:"contract,omitempty"`
	Affected testpolicy.AffectedResult `json:"affected,omitempty"`
}

// proofContention reads the first hold and its later release in append order.
// An empty subject selects the latest commit's hold.
func proofContention(entries []Intervention, subject string) (held, released Intervention) {
	for _, entry := range entries {
		if entry.Row != "proof-beside-builds" || subject != "" && entry.Subject != subject {
			continue
		}
		if strings.HasPrefix(entry.Outcome, "held") && (held.At == "" || held.Subject != entry.Subject) {
			held, released = entry, Intervention{}
		}
		if strings.HasPrefix(entry.Outcome, "released") && held.At != "" && held.Subject == entry.Subject {
			released = entry
		}
	}
	return
}

// ContentionHold is the lane's open proof hold: the held commit, the held
// line's text, when it started and its deadline.
type ContentionHold struct {
	Subject, Outcome string
	Since, Until     time.Time
}

// currentHold says whether the lane holds its proof start now. A release line,
// the 30-minute deadline, or a quiet host ends the hold; an unreadable host
// holds, and an unreadable hold time holds with its error. The host is read only for an open hold
// inside its deadline.
func currentHold(entries []Intervention, now time.Time, hostWork func() (int, int, error)) (ContentionHold, bool, error) {
	held, released := proofContention(entries, "")
	if held.At == "" || released.At != "" {
		return ContentionHold{}, false, nil
	}
	hold := ContentionHold{Subject: held.Subject, Outcome: held.Outcome}
	at, err := time.Parse(time.RFC3339Nano, held.At)
	if err != nil {
		return hold, true, err
	}
	hold.Since, hold.Until = at, at.Add(30*time.Minute)
	if !now.Before(hold.Until) {
		return ContentionHold{}, false, nil
	}
	if hostWork != nil {
		if builds, checks, err := hostWork(); err == nil && builds == 0 && checks == 0 {
			return ContentionHold{}, false, nil
		}
	}
	return hold, true, nil
}

// CurrentHold reads the lane's current proof hold from its log, its clock and
// its observation of host work.
func CurrentHold(install string, seams ProveSeams) (ContentionHold, bool, error) {
	entries, err := readLines[Intervention](interventionsPath(install))
	if err != nil {
		return ContentionHold{}, false, err
	}
	return currentHold(entries, seams.now(), seams.HostWork)
}

func holdBesideBuildsLocked(install, commit string, seams ProveSeams) error {
	if seams.HostWork == nil {
		return nil
	}
	builds, checks, readErr := seams.HostWork()
	quiet := readErr == nil && builds == 0 && checks == 0
	entries, err := readLines[Intervention](interventionsPath(install))
	if err != nil && quiet {
		return nil
	}
	if err != nil {
		return err
	}
	now := seams.now()
	work := fmt.Sprintf("%d builds and %d unit checks run", builds, checks)
	if readErr != nil {
		work = "the launch store cannot be read: " + readErr.Error()
	}
	record := func(subject, outcome string) error {
		load := "unmeasured"
		reader := seams.Load
		if reader == nil {
			reader = hostload.Read
		}
		if sample := reader(now); sample.Available {
			load = fmt.Sprintf("%g", sample.Load1m)
		}
		return appendLine(interventionsPath(install), Intervention{At: now.UTC().Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: subject, Outcome: outcome + work + "; load " + load}, seams)
	}
	if quiet {
		if open, held, _ := currentHold(entries, now, nil); held {
			return record(open.Subject, "released: cause ended; ")
		}
		return nil
	}
	held, released := proofContention(entries, commit)
	if released.At != "" {
		return nil
	}
	if held.At == "" {
		if err := record(commit, "held: "); err != nil {
			return err
		}
		held.At = now.UTC().Format(time.RFC3339Nano)
	}
	at, err := time.Parse(time.RFC3339Nano, held.At)
	if err != nil {
		return err
	}
	until := at.Add(30 * time.Minute)
	if !now.Before(until) {
		return record(commit, "released after the 30-minute hold; ")
	}
	return &Refusal{Code: "LANE_PROOF_CONTENTION", Reason: "the lane holds its own proof start while " + work + ", until " + until.Local().Format("15:04 MST") + " at the latest", Next: proofCommand(seams.Gate, seams.Trunk)}
}

// KeeperContentionHold keeps the agent out while the lane holds its proof start.
func KeeperContentionHold(install string, seams ProveSeams) (string, error) {
	hold, held, err := CurrentHold(install, seams)
	if err != nil {
		return "", err
	}
	if held {
		return landingstate.Phrase("the-lane-s-proof-start-is-held-beside") + hold.Until.Local().Format("15:04 MST"), nil
	}
	return "", nil
}

func proofCommand(gate, trunk bool) string {
	command := "metasystem landing prove"
	if gate {
		command += " --gate"
	} else if trunk {
		command += " --trunk"
	}
	return command
}

func proofScope(install, checkout string, running Running, seams ProveSeams) scopeDecision {
	if running.Admission != nil && running.Admission.Decision != nil {
		pinned := running.Admission.Decision
		return scopeDecision{scopeRecord: pinned.scopeRecord, base: pinned.Source, contract: pinned.Contract, affected: pinned.Affected, standing: pinned.Standing}
	}
	if running.Gate {
		return scopeDecision{scopeRecord: scopeRecord{Scope: "gate"}}
	}
	if (running.Fresh || seams.ClassificationOf != "") && !running.Impact {
		return scopeDecision{scopeRecord: scopeRecord{Scope: "full", ScopeReason: "fresh full check requested"}}
	}
	if running.Trunk {
		return decideScope(install, checkout, running, seams)
	}
	depth, reason := BatchProofDepth(install, checkout, running, seams)
	if depth == "full" && seams.DepthScope == "" && !seams.Impact {
		return scopeDecision{scopeRecord: scopeRecord{Scope: depth, ScopeReason: reason}}
	}
	if depth == "impact" && seams.DepthScope == "" && !seams.Impact {
		d := impactScope(install, checkout, seams)
		if !strings.HasPrefix(d.ScopeReason, "impact proof error:") {
			d.ScopeReason = reason
		}
		return d
	}
	if seams.DepthScope != "" {
		d := scopeDecision{scopeRecord: scopeRecord{Scope: depth, ScopeReason: reason}}
		if seams.Impact {
			d = impactScope(install, checkout, seams)
			if strings.HasPrefix(d.ScopeReason, "impact check error:") {
				d.ScopeReason += "; " + reason
			} else {
				d.ScopeReason = reason
				if depth == "full" {
					d.ScopeReason += "; explicit impact check does not satisfy the push at decided full depth"
				}
			}
		}
		return d
	}
	if seams.Impact {
		return impactScope(install, checkout, seams)
	}

	return decideScope(install, checkout, running, seams)
}

// BatchProofDepth chooses cadence and affected checks before the batch policy.
// An empty scope leaves the existing scoped-proof selector in charge.
func BatchProofDepth(install, checkout string, running Running, seams ProveSeams) (string, string) {
	if batch, err := ReadBatch(install); err == nil && batch != nil && batch.ID == running.BatchID && batch.ClockFull {
		return "full", "main's clock is due; this batch needs a full check"
	}
	previous, found, _ := resultFor(seams.resultsPath(install), running.Tree)
	if reason, _ := overdueBatch(install, running.BatchID, seams); reason != "" {
		return "full", reason
	}
	if found && strings.HasPrefix(previous.ScopeReason, "full check pending:") {
		return "full", previous.ScopeReason
	}
	if seams.DepthScope != "" {
		return seams.DepthScope, seams.DepthReason
	}
	if !seams.Impact && seams.BatchDepth != nil && decideScope(install, checkout, running, seams).Scope == "scoped" {
		return "", ""
	}
	if running.BatchID != "" && seams.BatchDepth != nil {
		return seams.BatchDepth(install, checkout, running)
	}
	return "", ""
}

// admitExecutionLocked reads policy for each whole execution, with the
// computed scope known. A prior person's admission grants no later execution.
func admitExecutionLocked(install, checkout, command string, running *Running, decision scopeDecision, seams ProveSeams) error {
	if decision.Scope == "impact" && decision.Base == "" {
		return fmt.Errorf("%s", decision.ScopeReason)
	}
	admission := ExecutionAdmission{Attempt: running.Attempt, Commit: running.Commit, Tree: running.Tree, BatchID: running.BatchID, Gate: running.Gate, Trunk: running.Trunk, Groups: decision.groupIDs(), Scope: decision.Scope, Reason: decision.ScopeReason, Base: decision.Base, BaseSHA: decision.BaseCommit, Command: command, At: seams.now().Format(time.RFC3339Nano), State: "pending"}
	// Keep only the source facts needed by a scoped proof, without its execution history.
	pinned := admittedScope{scopeRecord: decision.scopeRecord, Contract: decision.contract, Affected: decision.affected, Standing: decision.standing, Source: Result{Tree: decision.base.Tree, Attempt: decision.base.Attempt, FullTree: decision.base.FullTree, FullAt: decision.base.FullAt, Environment: decision.base.Environment}}
	admission.Decision = &pinned
	admission.DepthScope = seams.DepthScope
	if seams.DepthScope != "" {
		batch, err := ReadBatch(install)
		if err != nil {
			return err
		}
		if batch != nil {
			admission.DepthBase = batch.Base
		}
	}
	if seams.Lane != nil {
		registered, err := seams.Lane()
		if err != nil {
			return err
		}
		admission.Lane = &registered
	}
	person := seams.Person != nil
	if running.Trunk {
		incidents, err := seams.incidents(install, checkout, running.Commit)
		if err != nil {
			admission.Warning = "incident binding unknown: " + err.Error()
		} else {
			running.ObservedIncidents = incidents
		}
	}
	if decision.Scope == "full" {
		admission.Policy = PolicyValue{Value: "auto", Source: "default"}
		var readErr error
		if seams.Policy != nil {
			admission.Policy, readErr = seams.Policy("landing.proof")
		}
		if readErr != nil || admission.Policy.Value == "person" {
			if !person {
				if seams.CallerRefusal != "" {
					return &Refusal{Code: "LANE_PROOF_PERSON", Reason: seams.CallerRefusal}
				}
				reason := "the full check needs the person at an enrolled terminal"
				if readErr != nil {
					reason = "the check policy cannot be read, so this full check needs a person"
				}
				return holdProofLocked(install, *running, seams, "LANE_PROOF_PERSON", reason)
			}
			if readErr != nil {
				admission.Warning = "the check policy cannot be read: " + readErr.Error()
			}
		}
		if !running.Trunk {
			if err := seams.checkBudget(install, checkout, running.Commit); err != nil {
				if !person && seams.CallerRefusal != "" {
					return &Refusal{Code: "LANE_PROOF_BUDGET", Reason: seams.CallerRefusal}
				}
				if !person || seams.ClassificationOf != "" {
					return holdProofLocked(install, *running, seams, "LANE_PROOF_BUDGET", (&ProofBudget{}).Error())
				}
				admission.Override = true
				admission.Warning += "; this one execution exceeds the automatic full-check allowance"
				var budget *ProofBudget
				if !errors.As(err, &budget) {
					admission.Warning += ": " + err.Error()
				}
			}
		}
	}
	if person {
		previous, found, err := resultFor(seams.resultsPath(install), running.Tree)
		if err != nil || found && previous.Result == Red && previous.Repeat != "allowed" {
			admission.Override = true
			admission.Warning += "; this one execution overrides the automatic repeat hold"
		}
		provenance := *seams.Person
		provenance.Subject = append([]GoalSHA{}, running.BatchMembers...)
		running.Person = &provenance
	}
	running.ClassificationOf = seams.ClassificationOf
	running.Admission = &admission
	running.Executions = append(running.Executions, admission)
	return nil
}

func holdProofLocked(install string, running Running, seams ProveSeams, code, reason string) error {
	if !running.Gate && !running.Trunk && running.BatchID != "" {
		if err := sealBatchLocked(install, running.BatchID, seams); err != nil {
			return err
		}
	}
	subject := running.Tree
	if running.Trunk {
		subject = "main"
	} else if running.BatchID != "" {
		subject = running.BatchID
	}
	stop := Stop{Loop: "lane-proof", Subject: subject, Tree: running.Tree, BatchID: running.BatchID, Trunk: running.Trunk, Scope: "full", Budget: 2,
		Class: reason, Decision: "stop", Handoff: "ask lane", Evidence: runningPath(install), At: seams.now().Format(time.RFC3339Nano), Required: strings.Fields(proofCommand(running.Gate, running.Trunk))}
	if running.Gate {
		stop.Loop, stop.Scope = "lane-gate", "gate"
	}
	// Any open request for the same act, not only the newest stop, makes a repeat a no-op.
	open, err := OpenStops(install)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(open, func(o Stop) bool {
		return o.Loop == stop.Loop && o.Subject == stop.Subject && o.Tree == stop.Tree && o.Class == stop.Class && o.Trunk == stop.Trunk
	}) {
		if err := appendLine(stopsPath(install), stop, seams); err != nil {
			return err
		}
	}
	return &Refusal{Code: code, Reason: reason, Next: stop.Command()}
}

// Only the matching proof admission closes its request. Spent result history
// remains intact, including after an explicit budget exception.
func closeProofAdmissionStopsLocked(install string, running Running, now time.Time, effects ...ProveSeams) error {
	if running.Person == nil {
		return nil
	}
	loop := "lane-proof"
	if running.Gate {
		loop = "lane-gate"
	}
	err := closeMatchingStopsLocked(install, fmt.Sprintf("person admitted check %s", running.Attempt), now, func(stop Stop) bool {
		if !strings.HasPrefix(stop.Command(), "metasystem landing prove") || stop.Loop != loop || stop.Trunk != running.Trunk {
			return false
		}
		if running.Trunk {
			return (stop.Scope == "" || stop.Scope == "full") && stop.Subject == "main"
		}
		return running.BatchID != "" && stop.BatchID == running.BatchID || stop.Subject == proofStopSubject(running.BatchMembers, running.Trunk) || stop.Subject == running.Tree
	}, effects...)
	if err != nil && running.Admission != nil {
		running.Admission.Warning += "; check request closure pending: " + err.Error()
		running.Executions[len(running.Executions)-1].Warning = running.Admission.Warning
		return writeRunning(install, running, effects...)
	}
	return nil
}

func proofStopSubject(goals []GoalSHA, trunk bool) string {
	names := []string{}
	for _, g := range goals {
		names = append(names, g.Goal)
	}
	if len(names) == 0 {
		return "main"
	}
	return strings.Join(names, ", ")
}

func (a ExecutionAdmission) matches(r Running) bool {
	return a.Attempt == r.Attempt && a.Commit == r.Commit && a.Tree == r.Tree && a.BatchID == r.BatchID && a.Gate == r.Gate && a.Trunk == r.Trunk
}

func (r *Running) setAdmissionState(state string) {
	r.Admission.State = state
	r.Executions[len(r.Executions)-1].State = state
}
