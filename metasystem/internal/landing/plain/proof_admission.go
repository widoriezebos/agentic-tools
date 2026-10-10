package plain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
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
	Source   Result                    `json:"source,omitempty"`
	Contract testpolicy.Contract       `json:"contract,omitempty"`
	Affected testpolicy.AffectedResult `json:"affected,omitempty"`
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
		return scopeDecision{scopeRecord: pinned.scopeRecord, base: pinned.Source, contract: pinned.Contract, affected: pinned.Affected}
	}
	if running.Gate {
		return scopeDecision{scopeRecord: scopeRecord{Scope: "gate"}}
	}
	if running.Trunk {
		return scopeDecision{scopeRecord: scopeRecord{Scope: "full", ScopeReason: "fresh full check of main"}}
	}
	depth, reason := BatchProofDepth(install, checkout, running, seams)
	if depth == "inherited" {
		return scopeDecision{scopeRecord: scopeRecord{Scope: depth}}
	}
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

// BatchProofDepth chooses inheritance and cadence before the ordinary batch policy.
// An empty scope leaves the existing scoped-proof selector in charge.
func BatchProofDepth(install, checkout string, running Running, seams ProveSeams) (string, string) {
	previous, found, _ := resultFor(seams.resultsPath(install), running.Tree)
	from, ok := ledgerOnlySinceGreen(seams.git, install, checkout, running.Tree)
	canInherit := (!found || previous.Result == Green) && !strings.HasPrefix(previous.ScopeReason, "full check pending:") && ok && from.fullCurrent(seams.now())
	if !seams.Impact && canInherit && from.Scope == "full" && batchGreenReusable(install, running.BatchID, from, seams) {
		return "inherited", ""
	}
	if reason, _ := overdueBatch(install, running.BatchID, seams); reason != "" {
		return "full", reason
	}
	if found && strings.HasPrefix(previous.ScopeReason, "full check pending:") {
		return "full", previous.ScopeReason
	}
	if !seams.Impact && canInherit {
		return "inherited", ""
	}
	if seams.DepthScope != "" {
		return seams.DepthScope, seams.DepthReason
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
	pinned := admittedScope{scopeRecord: decision.scopeRecord, Contract: decision.contract, Affected: decision.affected, Source: Result{Tree: decision.base.Tree, Attempt: decision.base.Attempt, FullTree: decision.base.FullTree, FullAt: decision.base.FullAt, Environment: decision.base.Environment}}
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
				if !person || seams.ClassificationOf != "" {
					return holdProofLocked(install, *running, seams, "LANE_PROOF_BUDGET", "this batch used two full checks; goals hold; ask a person to run: metasystem landing prove")
				}
				admission.Override = true
				admission.Warning += "; this one execution exceeds the automatic full-check allowance: " + err.Error()
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
		if err := appendLine(stopsPath(install), stop); err != nil {
			return err
		}
	}
	return &Refusal{Code: code, Reason: reason, Next: stop.Command()}
}

// Only the matching proof admission closes its request. Spent result history
// remains intact, including after an explicit budget exception.
func closeProofAdmissionStopsLocked(install string, running Running, now time.Time) error {
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
	})
	if err != nil && running.Admission != nil {
		running.Admission.Warning += "; check request closure pending: " + err.Error()
		running.Executions[len(running.Executions)-1].Warning = running.Admission.Warning
		return writeRunning(install, running)
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
