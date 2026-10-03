package main

// The interface's half of launching a machine: judge, write the record, spawn
// the verb, answer.
//
// The act is long — a clone, a build and an arming — and every other act this
// server makes returns within one request. So this one does not do the work:
// it writes the record first, so that a page opened a second later already
// has something to read, and then starts `seat launch` detached with the
// launching process's METASYSTEM_* variables removed. The lock inside the
// verb decides the race between two requests; what is decided here is only
// whether this request is worth starting at all.
//
// Signed in is enough (g1-s72 D1). This process holds the one accepted
// proof, so it checks that proof itself, stamps the verdict it stands for
// into the record it creates, and leaves the proof as audit evidence before
// anything is spawned. Nothing about the human travels on the verb's argv:
// the clone's arm reads the verdict from the record.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// launchSeams are the three things the starter asks of the world that a test
// replaces: the facts the preflight judges against (a fetch of the fleet's
// presence), the detached spawn, and the server's clock.
type launchSeams struct {
	facts func(launch.Request, launch.Record) (launch.Facts, error)
	spawn func(lifecycle.Roots, launch.Request, launch.Record, string) error
	now   func() time.Time
}

// launchStarter is the Launch the server is built with.
func launchStarter(roots lifecycle.Roots) func(*session.Session, launch.Request) (launch.Record, error) {
	return launchStarterWith(roots, launchSeams{facts: seatLaunchFacts, spawn: spawnLaunch, now: time.Now})
}

// launchProofAction is the action the audit proof of a launch is recorded
// under: the verb the proof's verdict is carried into.
const launchProofAction = "seat launch"

// launchStarterWith is launchStarter over its seams.
//
// The route has already refused anything weaker than a signed-in human, and
// this function asks again rather than trusting it: it is the one process
// holding the proof, and the record it stamps is what the new machine is
// enrolled from.
func launchStarterWith(roots lifecycle.Roots, seams launchSeams) func(*session.Session, launch.Request) (launch.Record, error) {
	return func(signed *session.Session, asked launch.Request) (launch.Record, error) {
		if signed == nil || !signed.Proof.SessionValidFor(roots.StateRoot.Path()) {
			return launch.Record{}, errors.New("this launch comes from a browser that is not signed in for this checkout; sign in again here")
		}
		asked.From = roots.Checkout
		// The interface never forwards a pair: a browser launch is enrolled
		// from the session's verdict on its record, never from words.
		asked.Word, asked.ReviewBy = "", ""
		// A machine already launched and supervised from here is a repeat
		// whose effect holds (R-129-ui): its launch is the answer, and no
		// record is written or spawned.
		if launched, already, err := launch.Launched(roots.Checkout, asked); err == nil && already {
			return launched, nil
		}
		now := seams.now().UTC()
		record, path, err := launchRecordFor(roots, &asked, signed, now)
		if err != nil {
			return launch.Record{}, err
		}
		facts, err := seams.facts(asked, record)
		if err != nil {
			return launch.Record{}, err
		}
		if err := launch.Preflight(asked, facts); err != nil {
			return launch.Record{}, err
		}
		// The proof is the audit evidence every session act leaves, and it is
		// written before the record and the spawn: a proof that cannot be
		// recorded is a launch that does not start, with nothing on disk to
		// say otherwise.
		if err := humanauthority.RecordSessionProof(roots.StateRoot.Path(), launchProofOperation(record.Launch, now), launchProofAction, signed.Proof); err != nil {
			return launch.Record{}, fmt.Errorf("your sign-in for this launch could not be recorded, so nothing was started: %w", err)
		}
		// The record is written before anything runs, so a page opened a
		// second later already has something to read — and it says `starting`
		// until the verb writes its own process identity into it. A record
		// with no identity is nothing to reconcile: judging it by a pid it
		// does not carry would mark every launch dead in the moment between
		// this answer and the child's first write.
		if err := launch.SaveAt(path, record, roots.Checkout); err != nil {
			return launch.Record{}, err
		}
		if err := seams.spawn(roots, asked, record, path); err != nil {
			// A launch that could not be started is a failed launch and not a
			// starting one. Nothing else will ever write this record, so the
			// reason is written here.
			ended := seams.now().UTC().Format(time.RFC3339)
			record.Outcome = launch.OutcomeFailed
			record.EndedAt = &ended
			record.SetStep(launch.Step{
				Step: launch.StepClone, Outcome: launch.StepFailed, At: ended, Words: err.Error(),
			})
			_ = launch.SaveAt(path, record, roots.Checkout)
			return launch.Record{}, err
		}
		return record, nil
	}
}

// launchProofOperation names one launch's audit proof: the launch and the
// moment its verdict was stamped, so a retry under a later session leaves a
// proof of its own beside the first.
func launchProofOperation(id string, at time.Time) string {
	return id + "-" + at.UTC().Format("20060102T150405Z")
}

// launchEnrollment is the verdict a signed-in session's proof stands for,
// stamped from the proof and the server's clock and never from the body.
func launchEnrollment(signed *session.Session, now time.Time) *launch.Enrollment {
	return &launch.Enrollment{
		Kind:     launch.EnrollmentHumanSession,
		Provider: signed.Proof.ChannelProvider,
		Human:    signed.Proof.ChannelUser,
		Session:  signed.Proof.ChannelRef,
		At:       now.UTC().Format(time.RFC3339),
	}
}

// launchDiscarder is the DiscardLaunch the server is built with: the record
// in this checkout is marked, and the clone it names is not touched.
func launchDiscarder(roots lifecycle.Roots) func(id string) (launch.Record, error) {
	return func(id string) (launch.Record, error) {
		return launch.Discard(roots.Checkout, id, time.Now())
	}
}

// launchRecordFor is the record this request is about: the one a retry
// resumes, or a fresh one stamped with the signed-in session's verdict.
func launchRecordFor(roots lifecycle.Roots, asked *launch.Request, signed *session.Session, now time.Time) (launch.Record, string, error) {
	if asked.Resuming() {
		path, err := launch.Path(roots.Checkout, asked.Resume)
		if err != nil {
			return launch.Record{}, "", &launch.Refusal{Code: launch.CodeIDInvalid, Message: err.Error()}
		}
		record, err := launch.LoadAt(path)
		if err != nil {
			return launch.Record{}, "", err
		}
		// A retry is the same launch: its machine and its destination are
		// what the record says, whatever a browser sent.
		asked.Machine, asked.Destination = record.Machine, record.Destination
		// A retry is a fresh start of the same launch: the previous run's
		// process identity is not this one's, and leaving it in place would
		// let the reconciliation judge this retry by a pid that has gone.
		record.Outcome = launch.OutcomeStarting
		record.Process = launch.Process{}
		record.EndedAt = nil
		// A record the interface stamped is stamped again under the session
		// retrying it. One created without an enrollment is never stamped
		// later (S72-02): its resume refuses by name at the enrollment step
		// if it still has to arm, so a pre-slice clone's engine is never
		// handed a flag it does not know.
		if record.Enrollment != nil {
			record.Enrollment = launchEnrollment(signed, now)
		}
		return record, path, nil
	}
	if asked.Destination == "" {
		proposed, err := seatLaunchDestination(roots.Checkout, asked.Machine)
		if err != nil {
			return launch.Record{}, "", &launch.Refusal{Code: launch.CodeDestinationExists, Message: err.Error()}
		}
		asked.Destination = proposed
	}
	id, err := goal.NewOperationULID()
	if err != nil {
		return launch.Record{}, "", err
	}
	path, err := launch.Path(roots.Checkout, id)
	if err != nil {
		return launch.Record{}, "", err
	}
	return launch.Record{
		SchemaVersion: launch.SchemaVersion, Launch: id,
		Machine: asked.Machine, Destination: asked.Destination,
		Outcome:    launch.OutcomeStarting,
		StartedAt:  now.Format(time.RFC3339),
		Steps:      []launch.Step{},
		Enrollment: launchEnrollment(signed, now),
	}, path, nil
}

// spawnLaunch starts the verb detached and returns as soon as it is running.
//
// setsid, because this launch must outlive the request and the server that
// started it; the record is what reports on it afterwards, and the process
// identity in that record is what says whether it is still alive. The
// environment is scrubbed for the reason every step's is: an inherited
// METASYSTEM_EVIDENCE_ROOT would outrank the configuration the verb copies
// into the new machine.
func spawnLaunch(roots lifecycle.Roots, asked launch.Request, record launch.Record, path string) error {
	engine := roots.Installation.Path("bin", "metasystem")
	args := launchArgs(roots, asked, record, path)
	log, err := os.OpenFile(launchLogPath(roots.Checkout, record.Launch), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	quiet, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer quiet.Close()
	command := exec.Command(engine, args...)
	command.Dir = roots.Checkout
	command.Env = launch.Scrubbed(os.Environ())
	command.Stdin = quiet
	command.Stdout = log
	command.Stderr = log
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("the launch could not be started: %w", err)
	}
	// The child is nobody's to wait for: it outlives this request by design,
	// and the record it rewrites is what this server reads it by.
	return command.Process.Release()
}

// launchArgs is the verb's argument list: the record it continues and, for a
// fresh launch, the machine and destination that record already names. No
// pair and nothing about the human: the record carries the verdict.
func launchArgs(roots lifecycle.Roots, asked launch.Request, record launch.Record, path string) []string {
	args := []string{"seat", "launch", "--from", roots.Checkout, "--record", path}
	if asked.Resuming() {
		return append(args, "--resume", record.Launch)
	}
	return append(args, "--machine", asked.Machine, "--destination", asked.Destination)
}

// launchLogPath is where one launch's own output lands. It is beside the
// record and named for the launch, so a human reading a failed card has the
// verb's whole transcript under the path the card names.
func launchLogPath(checkout, id string) string {
	return filepath.Join(launch.Dir(checkout), id+".log")
}
