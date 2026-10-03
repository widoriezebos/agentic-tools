package main

import (
	"errors"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// typedArgv is this command as the person typed it: the retry a refusal
// names.
func (inv *intentInvocation) typedArgv() []string {
	return append(append([]string{"metasystem"}, inv.command.words()...), inv.raw...)
}

// withoutOption is argv less every --name VALUE (or --name=VALUE).
func withoutOption(argv []string, name string) []string {
	var kept []string
	for index := 0; index < len(argv); index++ {
		switch {
		case argv[index] == "--"+name:
			index++
		case strings.HasPrefix(argv[index], "--"+name+"="):
		default:
			kept = append(kept, argv[index])
		}
	}
	return kept
}

// personName is who the refused person most likely is: a typed --by, else
// the person at this seat's helm; empty leaves it to the enrollment.
func (inv *intentInvocation) personName(typed string) string {
	if typed != "" {
		return typed
	}
	if state := helm.Active(inv.cwd); state.Active && state.Malformed == "" {
		return state.By
	}
	return ""
}

// personRefusal is the refusal of a person's act this shell was not proven
// to be, in the two lines of "Messages a Person Reads": why, in plain words,
// and the one command that resolves it. The proof's own refusal is a detail.
func (inv *intentInvocation) personRefusal(target string, err error, typed string) *intentResult {
	remedy := humanauthority.RemedyFor(inv.stateRoot, err, inv.personName(typed), inv.typedArgv())
	return &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target),
		Summary: remedy.Reason + ", so nothing was done", next: remedy.Argv, nextReason: remedy.Then,
		Details: []string{"refused because: " + refusalCause(err)}}
}

// refusalCause is the innermost refusal behind err, codes included: the
// detail --verbose shows. A proof's outcome code leads it when the
// innermost error does not carry the outcome itself.
func refusalCause(err error) string {
	outcome, _ := humanauthority.OutcomeOf(err)
	for inner := errors.Unwrap(err); inner != nil; inner = errors.Unwrap(err) {
		err = inner
	}
	if carried, _ := humanauthority.OutcomeOf(err); outcome != "" && carried == "" {
		return outcome + ": " + err.Error()
	}
	return err.Error()
}

// eitherRefusal is personRefusal for an act a person or the agent session
// holding the work may perform: an agent's shell is told the option that
// names its session, and a person's way is a detail.
func (inv *intentInvocation) eitherRefusal(target string, err error, stopping bool) *intentResult {
	result := inv.personRefusal(target, err, "")
	retry := inv.typedArgv()
	walkedToAgent := humanauthority.RemedyFor(inv.stateRoot, err, "", nil).Kind == humanauthority.RemedyAgent
	if walkedToAgent || inv.agentSessionCaller() {
		if walkedToAgent {
			result.Summary = strings.TrimSuffix(result.Summary, ", so nothing was done") + " and named no session, so nothing was done"
		} else {
			result.Summary = "an agent session ran this without naming its session, so nothing was done"
		}
		result.next, result.nextReason = append(retry, "--lineage", "LINEAGE"), "as the session that holds the work"
		result.Details = append(result.Details, "a person runs it in a terminal they opened: "+shellCommand(retry))
	} else {
		result.Details = append(result.Details, "an agent session runs it with --lineage LINEAGE; its launcher sets METASYSTEM_OWNER_LINEAGE")
	}
	if stopping {
		named := append(slices.Clone(retry), "--by", "NAME")
		result.Details = append(result.Details, "a person at a terminal that is not enrolled may name themself instead: "+shellCommand(named))
		if !walkedToAgent && !inv.agentSessionCaller() {
			// A stopping act needs no enrolled terminal (H1): the person names
			// themself, which resolves it where enrolling would move the
			// enrollment.
			result.next, result.nextReason = named, "names you; a stopping act needs no enrolled terminal"
		}
	}
	return result
}

// agentSessionCaller reports whether the checkout's lease classifies this
// command's caller as an agent session: a seat's main session or a delegate.
// The walk to the enrolled terminal stops at the first shell whose terminal
// is not the one it looks for, so it does not always reach the agent that
// started that shell; a headless session has no terminal at all.
func (inv *intentInvocation) agentSessionCaller() bool {
	checkout := inv.layout.GitRoot
	if checkout == "" {
		checkout = inv.stateRoot
	}
	if checkout == "" {
		return false
	}
	class := inv.owners.agent.withDefaults().caller(inv, checkout)
	return class == lease.ClassMain || class == lease.ClassDelegate
}
