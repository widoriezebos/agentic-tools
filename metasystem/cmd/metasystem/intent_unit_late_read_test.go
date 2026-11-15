package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// lateFreshRead yields through the public review command with one live read.
func lateFreshRead(t *testing.T, findings []readsubject.Finding) (*workBed, *stopReadStarter, string, string, launch.UnitStep) {
	t.Helper()
	b := newStopWorkBed(t)
	s := &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{}, findings}, corrupt: "missing-return"}
	b.manager.Supervisor = s
	run, _, brief := stopBuild(t, b, "auto")
	s.corrupt, b.starter.hold = "", "read"
	code, result, stderr := stopPublic(t, b, "auto", "work", "review", b.id, "--work", "stopped")
	if code != 3 || result.Summary != "the fresh read of attempt 1 is running" {
		t.Fatalf("fresh read did not yield: %d %s %s", code, result.Summary, stderr)
	}
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	step := record.Rounds[0].Steps[len(record.Rounds[0].Steps)-1]
	if step.Name != "read:fresh" || step.State != launch.StepRunning || record.Rounds[0].UnknownRetries != 1 {
		t.Fatalf("fresh read is not held: %+v", record)
	}
	return b, s, run, brief, step
}

func endLateFreshRead(t *testing.T, b *workBed, step launch.UnitStep) {
	t.Helper()
	if _, err := b.manager.Store.Update(step.LaunchID, func(record *launch.Record) error {
		code := 0
		record.State, record.ExitCode = launch.Completed, &code
		record.FinishedAt = b.manager.Now().UTC().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b.starter.hold = ""
}

func assertLateReadBooked(t *testing.T, b *workBed, run string, step launch.UnitStep, material int) launch.UnitRunRecord {
	t.Helper()
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	round := record.Rounds[0]
	decision := "close"
	if material > 0 {
		decision = "continue"
	}
	fresh := round.Steps[len(round.Steps)-1]
	if fresh.Name != "read:fresh" || fresh.State != launch.StepPassed || fresh.LaunchID != step.LaunchID || len(round.Reads) != 1 || round.Reads[0].ID != step.LaunchID || round.Material != material || round.Stop == nil || round.Stop.Decision != decision || strings.HasPrefix(round.Stop.Handoff, "stopped ") {
		t.Fatalf("late read was not booked: %+v", round)
	}
	return record
}

func TestALateReadWaitsForAnAuthorizedLook(t *testing.T) {
	t.Parallel()
	b, _, run, _, step := lateFreshRead(t, nil)
	endLateFreshRead(t, b, step)
	before := workIdemSnapshot(t, b.unitRoot)
	launched := len(b.starter.launched())
	code, result := collectWithOwners(t, b, run, nonHolderOwners(b))
	assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
	if code != 1 || result.Outcome != intentRefused || len(b.starter.launched()) != launched {
		t.Fatalf("non-holder booked the late read: %d %s", code, result.Summary)
	}
	workIdemSameFiles(t, "refused late-read wait", before, workIdemSnapshot(t, b.unitRoot))
	record := callerErrorRun(t, b, run)
	if fresh := record.Rounds[0].Steps[len(record.Rounds[0].Steps)-1]; fresh.Name != "read:fresh" || fresh.State != launch.StepRunning {
		t.Fatalf("fresh read changed: %+v", fresh)
	}
	if code, result := collectWithOwners(t, b, run, b.workOwners()); code != 0 {
		t.Fatalf("holder could not book the late read: %d %s", code, result.Summary)
	}
	assertLateReadBooked(t, b, run, step, 0)
}

func TestALateFreshReadIsBookedByTheNextWait(t *testing.T) {
	t.Parallel()
	for _, wait := range []bool{true, false} {
		t.Run(fmt.Sprintf("wait-first=%t", wait), func(t *testing.T) {
			t.Parallel()
			b, s, run, _, step := lateFreshRead(t, nil)
			endLateFreshRead(t, b, step)
			launched := len(b.starter.launched())
			if wait { // Regression: wait must book and finish before returning.
				code, result, stderr := stopPublic(t, b, "auto", "work", "wait", b.id, "--work", "stopped")
				if code != 0 {
					t.Fatalf("wait: %d %s %s", code, result.Summary, stderr)
				}
				assertLateReadBooked(t, b, run, step, 0)
			}
			// Preservation: review joins an ended read, or reviews a booked one.
			owners := b.workOwners()
			reviewed := false
			owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) {
				reviewed = true
				return "", errors.New("round review reached the endpoint")
			}
			var out, stderr bytes.Buffer
			code := runIntentIn(mustIntentArgvCommand(t, []string{"work", "review"}), []string{b.id, "--work", "stopped", "--json"}, &out, &stderr, b.root(), owners)
			if code != 1 || !reviewed || !strings.Contains(out.String(), "round review reached the endpoint") {
				t.Fatalf("round review was not reached: code=%d endpoint=%t %s", code, reviewed, stderr.String())
			}
			assertLateReadBooked(t, b, run, step, 0)
			if s.next != 2 || len(b.starter.launched()) != launched {
				t.Fatal("booking started another read")
			}
		})
	}
}

func TestANamedRevisionBooksTheLateReadFirst(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{true, false} {
		t.Run(fmt.Sprintf("person=%t", person), func(t *testing.T) {
			t.Parallel()
			var findings []readsubject.Finding
			if !person {
				findings = []readsubject.Finding{stopFinding("regression", "a.go")}
			}
			b, s, run, brief, step := lateFreshRead(t, findings)
			endLateFreshRead(t, b, step)
			builds := 0
			s.beforeBuild = func() {
				builds++
				record := assertLateReadBooked(t, b, run, step, len(findings))
				if len(record.Rounds) != 2 {
					t.Fatalf("correction was not reserved: %+v", record)
				}
				reservation, err := dispatchcore.ReadRecordObject(filepath.Join(b.root(), "artifacts", "agents", "jobs", step.LaunchID+".json"))
				if err != nil || !dispatchcore.TerminalStatus(fmt.Sprint(reservation["status"])) {
					t.Fatalf("fresh read reservation is still active at build start: %+v %v", reservation, err)
				}
			}
			if !person {
				brief = b.brief("late-correction.md", "Correct the finding.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n| "+step.LaunchID+":1"+" | fixed | a.go:12 |\n")
			}
			args := []string{"work", "revise", b.id, "--work", "stopped", "--brief", brief}
			if person {
				args = append(args, "--reason", "Correct explicitly after the read", "--by", "Wido")
			}
			// Regression: both forms must see the booked decision before admission.
			code, result, stderr := stopPublic(t, b, "auto", args...)
			if code != 0 || builds != 1 {
				t.Fatalf("named correction: %d %s %v %s builds=%d", code, result.Summary, result.Details, stderr, builds)
			}
			assertLateReadBooked(t, b, run, step, len(findings))
		})
	}
}
