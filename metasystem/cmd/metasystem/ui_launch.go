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
// The word reaches the verb's argument list and nothing else. It is not
// written into the record, not into the log, and not into the answer.

import (
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

// launchStarter is the Launch the server is built with.
//
// The signed-in human is the hand this act is made under; the route has
// already refused anything weaker, and nothing about who they are travels
// further than that refusal — the authority the new machine carries is their
// own word, recorded on its identity by the arming verb.
func launchStarter(roots lifecycle.Roots) func(*session.Session, launch.Request) (launch.Record, error) {
	return func(_ *session.Session, asked launch.Request) (launch.Record, error) {
		asked.From = roots.Checkout
		if err := humanauthority.ValidateTemporaryWordPair(asked.Word, asked.ReviewBy); err != nil {
			return launch.Record{}, &launch.Refusal{Code: launch.CodeWordInvalid, Message: err.Error()}
		}
		// The one rule this side has that the engine's validator does not: a
		// review date already behind us is a review due the moment the
		// machine joins, which is not what choosing a date means.
		if asked.ReviewBy != "" && launch.PastReviewDate(asked.ReviewBy, time.Now()) {
			return launch.Record{}, &launch.Refusal{
				Code:    launch.CodeReviewDatePast,
				Message: "the review-by date " + asked.ReviewBy + " has already passed; choose a date this machine's enrollment can be reviewed by",
			}
		}
		record, path, err := launchRecordFor(roots, &asked)
		if err != nil {
			return launch.Record{}, err
		}
		facts, err := seatLaunchFacts(asked, record)
		if err != nil {
			return launch.Record{}, err
		}
		if err := launch.Preflight(asked, facts); err != nil {
			return launch.Record{}, err
		}
		if err := launch.SaveAt(path, record, roots.Checkout); err != nil {
			return launch.Record{}, err
		}
		if err := spawnLaunch(roots, asked, record, path); err != nil {
			return launch.Record{}, err
		}
		return record, nil
	}
}

// launchRecordFor is the record this request is about: the one a retry
// resumes, or a fresh one written before anything runs.
func launchRecordFor(roots lifecycle.Roots, asked *launch.Request) (launch.Record, string, error) {
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
		if asked.ReviewBy == "" {
			asked.ReviewBy = record.ReviewBy
		}
		record.Outcome = launch.OutcomeRunning
		record.EndedAt = nil
		record.ReviewBy = asked.ReviewBy
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
		Outcome: launch.OutcomeRunning, ReviewBy: asked.ReviewBy,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:     []launch.Step{},
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
	engine := filepath.Join(roots.Installation, "bin", "metasystem")
	args := []string{"seat", "launch", "--from", roots.Checkout, "--record", path}
	if asked.Resuming() {
		args = append(args, "--resume", record.Launch)
	} else {
		args = append(args, "--machine", asked.Machine, "--destination", asked.Destination)
	}
	if asked.Word != "" {
		args = append(args, "--temporary-human-word", asked.Word, "--review-by", asked.ReviewBy)
	}
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

// launchLogPath is where one launch's own output lands. It is beside the
// record and named for the launch, so a human reading a failed card has the
// verb's whole transcript under the path the card names.
func launchLogPath(checkout, id string) string {
	return filepath.Join(launch.Dir(checkout), id+".log")
}
