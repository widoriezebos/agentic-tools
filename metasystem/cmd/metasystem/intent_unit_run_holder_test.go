package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func nonHolderOwners(b *workBed) intentOwners {
	owners := b.workOwners()
	owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
		return func() error {
			return refusal.New("OWNED-ELSEWHERE", "holder=another-session", fmt.Errorf("this checkout is held by another session\nfor a separate copy to write in, run: metasystem session isolate"))
		}
	}
	return owners
}

func withoutRunHolder(t *testing.T, b *workBed, run string) {
	t.Helper()
	r := callerErrorRun(t, b, run)
	r.Holder = nil
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(b.unitRoot, run, "run.json"), data, 0600)
}

func TestACommandWithoutTheClaimWritesNothing(t *testing.T) {
	t.Parallel()
	b, run, build := completedRound(t)
	withoutRunHolder(t, b, run)
	before := workIdemSnapshot(t, b.unitRoot)
	launched := b.starter.launched()
	code, result := collectWithOwners(t, b, run, nonHolderOwners(b))
	assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
	if code != 1 || result.Outcome != intentRefused || resultData(t, result)["state"] != "running" || !slices.Equal(launched, b.starter.launched()) {
		t.Fatalf("non-holder advanced the run: code=%d outcome=%s state=%v launches=%v", code, result.Outcome, resultData(t, result)["state"], b.starter.launched())
	}
	workIdemSameFiles(t, "refused wait", before, workIdemSnapshot(t, b.unitRoot))
	reservation, err := dispatchcore.ReadRecordObject(filepath.Join(b.root(), "artifacts", "agents", "jobs", build+".json"))
	if err != nil || reservation["status"] != "pending" {
		t.Fatalf("non-holder collected the build: %+v %v", reservation, err)
	}
	code, result = collectWithOwners(t, b, run, b.workOwners())
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("holder could not finish: %d %s %v", code, result.Summary, b.starter.launched())
	}
	before = workIdemSnapshot(t, b.unitRoot)
	code, result = collectWithOwners(t, b, run, nonHolderOwners(b))
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" {
		t.Fatalf("non-holder could not read the judgement: %d %s", code, result.Summary)
	}
	workIdemSameFiles(t, "judged wait", before, workIdemSnapshot(t, b.unitRoot))
}

func TestARoundGoesOnUnderItsRunsHolder(t *testing.T) {
	t.Parallel()
	t.Run("proof", func(t *testing.T) {
		t.Parallel()
		b, run, _ := completedRound(t)
		file := b.goalFile(b.id)
		want := launch.RunHolder{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage, ClaimEpoch: 1}
		if r := callerErrorRun(t, b, run); r.Holder == nil || *r.Holder != want {
			t.Fatalf("run did not retain its admitting claim: %+v", r.Holder)
		}
		code, result := collectWithOwners(t, b, run, nonHolderOwners(b))
		if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
			t.Fatalf("round did not continue under its holder: %d %s %v", code, result.Summary, b.starter.launched())
		}
	})
	t.Run("comparison after budget increase", func(t *testing.T) {
		t.Parallel()
		b, args := comparisonAccountingBed(t, 121)
		code, held, _ := b.work(args...)
		if code != 1 || !strings.Contains(resultWords(held), "BUDGET_REFUSED") {
			t.Fatalf("comparison did not reach its budget hold: %d %s", code, held.Summary)
		}
		run := resultData(t, held)["run"].(string)
		if code, r := b.runJSON(b.owners(), "goal", "budget", b.id, "4h/4/240m/1/2", "--by", "Wido", "--fixture-human-authority", "--lineage", "m1"); code != 0 {
			t.Fatalf("budget remedy: %d %s", code, r.Summary)
		}
		if b.goalFile(b.id).StopCapability.ClaimEpoch != 1 {
			t.Fatal("budget act changed the admitting claim epoch")
		}
		assertOwnComparison(t, b, run, nonHolderOwners(b))
	})
}

func TestARunHolderThatNoLongerHoldsNamesWhoContinues(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"epoch", "lineage", "machine", "legacy", "ownership", "timeout", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b, run, _ := completedRound(t)
			owners := nonHolderOwners(b)
			file := b.goalFile(b.id)
			next := []string{"metasystem", "work", "wait", "run:" + run}
			reason := "metasystem work wait run:" + run + "  (from the session that holds this checkout)"
			details := []string{fmt.Sprintf("was %s on %s, now %s on %s", file.Claimed.Lineage, file.Claimed.Machine, file.Claimed.Lineage, file.Claimed.Machine)}
			switch kind {
			case "epoch":
				file.StopCapability.ClaimEpoch = 2
				b.addGoal(file)
			case "lineage", "machine":
				if kind == "lineage" {
					file.Claimed.Lineage = "successor-lineage"
				} else {
					file.Claimed.Machine = "successor-machine"
					file.StopCapability.Machine = file.Claimed.Machine
				}
				b.addGoal(file)
				details[0] = fmt.Sprintf("was %s on %s, now %s on %s", callerErrorRun(t, b, run).Holder.Lineage, callerErrorRun(t, b, run).Holder.Machine, file.Claimed.Lineage, file.Claimed.Machine)
			case "legacy":
				withoutRunHolder(t, b, run)
				next, reason = []string{"metasystem", "session", "isolate"}, "then repeat the claim in the resulting writer session"
				details = []string{"this checkout is held by another session"}
			case "unreadable":
				owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New("ledger unavailable") }
				reason = "after the cause in the details is repaired"
				details = []string{"the claim on goal " + b.id + " cannot be read:", "ledger unavailable"}
			case "ownership", "timeout":
				var cause error = goalHeldElsewhere{goal: b.id, holder: "holder-H", holderLineage: "holder-lineage", machine: file.Claimed.Machine, lineage: file.Claimed.Lineage}
				next, reason = []string{"metasystem", "goal", "claim", b.id, "--take-over", "--reason", "TEXT"}, "a person takes the goal over; or the session holding it builds"
				details = []string{"holder-H"}
				if kind == "timeout" {
					cause = &goal.ReadTimeout{Limit: 4 * time.Second}
					next, reason = []string{"metasystem", "work", "wait", "run:" + run}, "after the cause in the details is repaired"
					details = []string{"timed out after 4s"}
				}
				owners.connection.ownershipCheck = func(string, string, goal.Endpoint) func() error { return func() error { return cause } }
			}
			before := workIdemSnapshot(t, b.unitRoot)
			code, result := collectWithOwners(t, b, run, owners)
			assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
			if code != 1 || result.Outcome != intentRefused || result.Next == nil || !slices.Equal(result.Next.Argv, next) || result.Next.Reason != reason || !slices.Equal(b.starter.launched(), []string{"build"}) {
				t.Fatalf("holder refusal lost its remedy: code=%d next=%+v details=%v launches=%v", code, result.Next, result.Details, b.starter.launched())
			}
			for _, word := range details {
				if !strings.Contains(strings.Join(result.Details, "\n"), word) {
					t.Fatalf("holder refusal lost %q: %v", word, result.Details)
				}
			}
			workIdemSameFiles(t, "holder refusal", before, workIdemSnapshot(t, b.unitRoot))
		})
	}
}

func TestACommandWithoutTheClaimNeverBuilds(t *testing.T) {
	t.Parallel()
	t.Run("build has not started", func(t *testing.T) {
		t.Parallel()
		b := outcomeBed(t, "diff --git a/unit.go b/unit.go\n--- a/unit.go\n+++ b/unit.go\n+new\n")
		conf := filepath.Join(b.root(), "metasystem.conf")
		writeTestingFixtureFile(t, conf, []byte("dispatch.cap-min=broken\n"), 0600)
		code, held, _ := outcomeBuild(t, b)
		if code != 1 {
			t.Fatalf("broken cap admitted build: %d %s", code, held.Summary)
		}
		run := resultData(t, held)["run"].(string)
		step := callerErrorRun(t, b, run).Rounds[0].Steps[0]
		if step.State != launch.StepStarting {
			t.Fatalf("build did not stay starting: %+v", step)
		}
		if _, err := b.manager.Store.Read(step.LaunchID); !os.IsNotExist(err) {
			t.Fatalf("build has a launch record: %v", err)
		}
		writeTestingFixtureFile(t, conf, nil, 0600)
		before := workIdemSnapshot(t, b.unitRoot)
		code, result := collectWithOwners(t, b, run, nonHolderOwners(b))
		assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
		if code != 1 || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "session", "isolate"}) || len(b.starter.launched()) != 0 {
			t.Fatalf("non-holder built: %d %+v %v", code, result, b.starter.launched())
		}
		workIdemSameFiles(t, "unstarted build", before, workIdemSnapshot(t, b.unitRoot))
		if code, result := collectWithOwners(t, b, run, b.workOwners()); code != 0 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
			t.Fatalf("holder could not build: %d %s %v", code, result.Summary, b.starter.launched())
		}
	})
	t.Run("follow-up", func(t *testing.T) {
		t.Parallel()
		b, run, _ := completedRound(t)
		if code, result := collectWithOwners(t, b, run, b.workOwners()); code != 0 {
			t.Fatalf("holder could not finish: %d %s", code, result.Summary)
		}
		before := workIdemSnapshot(t, b.unitRoot)
		code, result := b.runJSON(nonHolderOwners(b), "work", "revise", "run:"+run, "--brief", b.brief("follow-up.md", "Revise the change.\n"))
		assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
		if code != 1 || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "session", "isolate"}) || len(callerErrorRun(t, b, run).Rounds) != 1 || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
			t.Fatalf("non-holder added a round: %d %+v", code, result)
		}
		workIdemSameFiles(t, "follow-up", before, workIdemSnapshot(t, b.unitRoot))
	})
}

func TestRunAuthorityAdmitsOnlyRoundLaunches(t *testing.T) {
	t.Parallel()
	b, run, _ := completedRound(t)
	r := callerErrorRun(t, b, run)
	r.Rounds[0].Steps[0].State = launch.StepPassed
	layout, err := b.owners().resolver.ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	caller := errors.New("caller does not hold the checkout")
	owners := b.workOwners()
	inv := &intentInvocation{owners: owners, layout: layout, cwd: b.root(), stateRoot: b.root()}
	ids := []string{run + "-r1-s2", run + "-r1-s3", run + "-r1-s2-retry", run + "-r1-s2-base", run + "-r1-s2-alone", "20260901t100000-0123456789"}
	for _, holding := range []bool{false, true} {
		inv.owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
			return func() error {
				if holding {
					return nil
				}
				return caller
			}
		}
		for i, id := range ids {
			err := inv.runAuthority(r, launch.StartSpec{ID: id})
			if !holding && i == len(ids)-1 {
				if !errors.Is(err, caller) {
					t.Fatalf("publication check adopted round authority: %v", err)
				}
			} else if err != nil {
				t.Fatalf("holder=%t launch=%s: %v", holding, id, err)
			}
		}
	}
}
