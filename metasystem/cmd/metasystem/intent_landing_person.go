package main

import (
	"slices"
	"strings"
)

// landingPersonCheck carries the caller's authority refusal into the landing act.
func (inv *intentInvocation) landingPersonCheck(act, target string) (lanePersonObservation, *intentResult) {
	observed, problem := inv.lanePerson(act, target)
	if problem == nil {
		return observed, nil
	}
	parts := strings.SplitN(problem.Summary, "; ", 3)
	reason := problem.Summary
	if len(parts) > 1 {
		reason = parts[1]
	}
	if len(problem.next) >= 3 && slices.Equal(problem.next[:3], []string{"metasystem", "system", "enroll"}) {
		if observed.Proof.EnrolledTerminalFor(observed.Root) {
			problem.Summary = reason + "; type: " + strings.Join(problem.next, " ")
		} else {
			problem.Summary = "this terminal is not enrolled for the landing install; type: " + strings.Join(problem.next, " ")
		}
	} else {
		problem.Summary = reason
		if slices.Equal(problem.next, inv.typedArgv()) {
			problem.next = nil
			problem.nextReason = ""
		}
	}
	return observed, problem
}
