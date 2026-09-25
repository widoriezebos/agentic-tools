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
	"strings"
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
		// The interface never launches a machine under an authorization
		// nobody typed. The verb still takes the terminal-only path — arming
		// from the caller's own enrolled terminal, with no pair at all — but
		// that path belongs to a human standing at a terminal, and a browser
		// is not one. So the word and the date travel with every launch and
		// with every retry, including a retry whose enrollment step may
		// already be done: the resume decides that from the clone's own
		// identity and binary, which this side cannot read.
		if strings.TrimSpace(asked.Word) == "" || strings.TrimSpace(asked.ReviewBy) == "" {
			return launch.Record{}, &launch.Refusal{
				Code:    launch.CodeWordRequired,
				Message: "a machine launched from this interface is enrolled under your own words: give the word and the review date",
			}
		}
		if err := humanauthority.ValidateTemporaryWordPair(asked.Word, asked.ReviewBy); err != nil {
			return launch.Record{}, &launch.Refusal{Code: launch.CodeWordInvalid, Message: err.Error()}
		}
		// The one rule this side has that the engine's validator does not: a
		// review date already behind the human who chose it is a review due
		// the moment the machine joins. It is judged against the CLIENT's own
		// day, which travels with the request: this server and the browser
		// can be on different dates for several hours of every day, and the
		// date on screen is the one a human answered.
		if !launch.ValidDay(asked.ClientToday) {
			return launch.Record{}, &launch.Refusal{
				Code:    launch.CodeReviewDatePast,
				Message: "this request does not say what day it was made on, so the review date cannot be judged against it",
			}
		}
		if launch.ReviewDateBefore(asked.ReviewBy, asked.ClientToday) {
			return launch.Record{}, &launch.Refusal{
				Code:    launch.CodeReviewDatePast,
				Message: "the review-by date " + asked.ReviewBy + " is before " + asked.ClientToday + "; choose a date this machine's enrollment can be reviewed by",
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
		// The record is written before anything runs, so a page opened a
		// second later already has something to read — and it says `starting`
		// until the verb writes its own process identity into it. A record
		// with no identity is nothing to reconcile: judging it by a pid it
		// does not carry would mark every launch dead in the moment between
		// this answer and the child's first write.
		if err := launch.SaveAt(path, record, roots.Checkout); err != nil {
			return launch.Record{}, err
		}
		if err := spawnLaunch(roots, asked, record, path); err != nil {
			// A launch that could not be started is a failed launch and not a
			// starting one. Nothing else will ever write this record, so the
			// reason is written here.
			ended := time.Now().UTC().Format(time.RFC3339)
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
		// A retry is a fresh start of the same launch: the previous run's
		// process identity is not this one's, and leaving it in place would
		// let the reconciliation judge this retry by a pid that has gone.
		record.Outcome = launch.OutcomeStarting
		record.Process = launch.Process{}
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
		Outcome: launch.OutcomeStarting, ReviewBy: asked.ReviewBy,
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
