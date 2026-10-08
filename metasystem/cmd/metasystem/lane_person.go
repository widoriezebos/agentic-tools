package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type lanePersonObservation struct {
	Proof      humanauthority.Proof
	Name, Root string
	At         time.Time
}

// lanePerson observes the original calling checkout, independently of --repo.
// A linked checkout keeps its own enrollment rather than borrowing its primary's.
func (inv *intentInvocation) lanePerson(act, target string) (lanePersonObservation, *intentResult) {
	var observed lanePersonObservation
	path := inv.cwd
	if _, err := helm.Locate(path); err != nil {
		// An unreadable checkout marker is still a calling checkout. It must
		// never turn a failed calling read into destination authority.
		marker := false
		for dir := filepath.Clean(path); ; dir = filepath.Dir(dir) {
			if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
				marker = true
				break
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
		if !marker {
			path = target
		}
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err == nil {
		observed.Root = checkoutAuthorityRoot(layout)
	}
	refused := func(reason string, cause error) (lanePersonObservation, *intentResult) {
		remedy := humanauthority.RemedyFor(observed.Root, cause, inv.personName(""), inv.typedArgv())
		result := &intentResult{Outcome: intentRefused, code: 1, Summary: "only the person at the calling checkout's enrolled terminal may " + act + "; " + reason + "; nothing was recorded", next: remedy.Argv, nextReason: remedy.Then}
		if len(result.next) == 0 {
			result.next, result.nextReason = inv.typedArgv(), "run at that enrolled terminal, without a helm or grant"
		}
		return observed, result
	}
	if err != nil {
		return refused("the calling checkout cannot be resolved", err)
	}
	if inv.owners.prove == nil || inv.owners.commandNow == nil {
		return refused("the terminal cannot be checked", nil)
	}
	observed.At, err = inv.owners.commandNow(observed.Root)
	if err != nil {
		return refused("the clock cannot be read", err)
	}
	observed.Proof, err = inv.owners.prove(observed.Root, int64(os.Getppid()), nil, "", "", observed.At)
	if err != nil {
		return refused(humanauthority.PlainReason(err), err)
	}
	if observed.Proof.Helm != nil || !observed.Proof.EnrolledTerminalFor(observed.Root) {
		return refused("this shell has no direct enrolled-terminal proof", nil)
	}
	observed.Name = "author unknown"
	enrollment, err := humanauthority.ReadEnrollment(observed.Root)
	if err == nil && enrollment.Generation == observed.Proof.TerminalGeneration && enrollment.TerminalRef == observed.Proof.TerminalRef && strings.TrimSpace(enrollment.Human) != "" {
		observed.Name = strings.TrimSpace(enrollment.Human)
	}
	if inv.input.has("by") {
		by := strings.TrimPrefix(inv.input.text("by"), "human:")
		if observed.Name == "author unknown" {
			return refused("the enrolled terminal has no recorded name; enroll it with your name", humanauthority.Refused(humanauthority.OutcomeNotEnrolled, nil))
		}
		if by != observed.Name {
			return refused("--by does not match the enrolled person "+observed.Name, nil)
		}
	}
	return observed, nil
}
