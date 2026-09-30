package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

const sessionStopLifetime = 8 * time.Hour

var (
	classifySessionStopCaller = personClassify
	currentSessionStopHolder  = lease.CurrentHolder
	classifySessionStopView   = classifyVerbCaller
	sessionStopNow            = time.Now
	proveSessionStopHuman     = func(root string, pid int64, now time.Time) (humanauthority.Proof, error) {
		proof, err := humanauthority.ProveTerminal(root, pid, nil, now)
		if err != nil {
			return proof, err
		}
		if !proof.TerminalValidFor(root) {
			return humanauthority.Proof{}, fmt.Errorf("terminal human authority was not proven")
		}
		return proof, nil
	}
	// proveSessionStopAttorney is the person proof when a general power of
	// attorney made the caller a person: the enrolled walk's proof, which a
	// live grant admits with the grant named.
	proveSessionStopAttorney = func(root string, pid int64, now time.Time) (humanauthority.Proof, error) {
		proof, err := humanauthority.Prove(root, pid, nil, now)
		if err != nil {
			return proof, err
		}
		if proof.Helm == nil || proof.Helm.Grant == "" || !proof.TerminalValidFor(root) {
			return humanauthority.Proof{}, fmt.Errorf("the power of attorney did not admit this session stop")
		}
		return proof, nil
	}
)

// authorizeSessionStop mints one quiet-stop authorization for the current
// announced main session at a proven human terminal. A refusal returns its
// sentence and exit code and writes nothing. When the same person's
// authorization already holds, it returns that authorization, exit 0 and the
// sentence saying what already holds, and writes nothing (R-129-ui).
func authorizeSessionStop(stateRoot, by string) (goal.SessionStop, string, int) {
	callerPID := int64(os.Getpid())
	classification, err := classifySessionStopCaller(stateRoot, callerPID)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: caller classification failed: %v", err), 3
	}
	if classification.Class != lease.ClassHuman {
		return goal.SessionStop{}, sessionStopRefusal(stateRoot, by, fmt.Errorf("%s", humanauthority.OutcomeAgent)), 3
	}

	now := sessionStopNow().UTC()
	prove := proveSessionStopHuman
	if classification.Attorney != nil {
		prove = proveSessionStopAttorney
	}
	humanProof, err := prove(stateRoot, int64(os.Getppid()), now)
	if err != nil {
		return goal.SessionStop{}, sessionStopRefusal(stateRoot, by, err), 3
	}
	holder, err := currentSessionStopHolder(stateRoot)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: the current checkout holder cannot be read: %v", err), 1
	}
	view, err := classifySessionStopView(stateRoot, callerPID)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: the current lease epoch cannot be read: %v", err), 1
	}
	if holder.MainId == "" || holder.SessionId == "" || view.ClaimEpoch == nil || *view.ClaimEpoch < 1 {
		return goal.SessionStop{}, "session stop refused: the current checkout holder lacks a main identity, announced session, or lease epoch", 1
	}

	marker, err := (&goal.Store{Root: stateRoot}).WriteSessionStop(goal.SessionStop{
		SchemaVersion: 3,
		SessionId:     holder.SessionId,
		HolderMainId:  holder.MainId,
		ClaimEpoch:    *view.ClaimEpoch,
		By:            by,
		WrittenAt:     now.Format(time.RFC3339),
		ExpiresAt:     now.Add(sessionStopLifetime).Format(time.RFC3339),
		Human: goal.SessionStopProcessRef{
			Pid: humanProof.InvokerRef.PID, PidStartedAt: humanProof.InvokerRef.PIDStartedAt,
			PidStartTicks: humanProof.InvokerRef.StartTicks, BootID: humanProof.InvokerRef.BootID,
		},
	}, humanProof)
	var already goal.AlreadyHolds
	if errors.As(err, &already) {
		return marker, already.Reason, 0
	}
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop: %v", err), 1
	}
	return marker, "", 0
}

// sessionStopRefusal says, in two lines, why this shell may not stop the
// session quietly and the one command that resolves it, with the person's
// name filled in (by, else the enrolled person's, NAME only when nobody is
// known): enroll the terminal, or stop from a terminal the person opened.
func sessionStopRefusal(stateRoot, by string, err error) string {
	person := strings.TrimSpace(by)
	if person == "" {
		person = "NAME"
		if enrollment, readErr := humanauthority.ReadEnrollment(stateRoot); readErr == nil && enrollment.Human != "" {
			person = enrollment.Human
		}
	}
	remedy := humanauthority.RemedyFor(stateRoot, err, by, []string{"metasystem", "session", "stop", "--by", person})
	reason := "only a person can stop a session quietly: " + remedy.Reason + ", so nothing was done"
	if len(remedy.Argv) == 0 {
		return reason
	}
	return reason + "\nrun: " + shellCommand(remedy.Argv) + "  (" + remedy.Then + ")"
}
