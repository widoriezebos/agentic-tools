package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestAnswerDoesNotMistakeBookingForCollection(t *testing.T) {
	t.Parallel()
	b, runner, run, launchID := endedIntentBuild(t, launch.Completed, "RESULT: DONE\n")
	if skipped := steward.BookEndedWork(b.root(), 5*time.Minute, runner); skipped != 0 {
		t.Fatal(skipped)
	}
	enrol(t, b, "Wido")
	channelFor(t, b.root())
	owners := collectionOwners(t, b)
	collectionOwnership(t, b, b.goalFile(b.id).Claimed.Lineage, false)
	q, err := channel.Ask(channel.AskRequest{RepoRoot: b.root(), About: "machine", Kind: "other", Machine: "mac-cli", Contract: true, Now: b.manager.Now(), Facts: []string{"finished round"},
		Decision: &channel.Decision{Kind: "uncollected-round", Subject: b.id, State: launchID}, Recommendation: "Collect it",
		Plain: &channel.Plain{Decision: "The round finished and waits.", IfNothing: "it waits."}, Options: []channel.Option{
			{Label: "Collect it", Act: []string{"metasystem", "work", "wait", "run:" + run, "--timeout", "1m"}, Consequence: "I collect the round. Undo: metasystem work stop on the goal."},
			{Label: "Leave it", Consequence: "it waits."}}})
	if err != nil {
		t.Fatal(err)
	}
	q = recordChoice(t, b.root(), q, func(q *channel.Question) { q.Answer.Text, q.Answer.Lineage = "collect it", "steward" })
	record, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.Plan, []byte("unreadable plan"), 0600); err != nil {
		t.Fatal(err)
	}
	code, result := collectWithOwners(t, b, run, owners, "--answer", q.ID)
	state, _ := answerState(t, result)
	if code == 0 || state != "refused" {
		t.Fatalf("booking was reported as collection after a refused wait: %d %s", code, result.Summary)
	}
	record, err = runner.Status(run)
	if err != nil || record.State != launch.UnitStepEnded || record.Rounds[0].Steps[0].EndBooking.CollectedAt != "" {
		t.Fatalf("refused wait changed the booked step: state=%s error=%v", record.State, err)
	}
}

func witnessNextSessionsStopCollectsTheBookedRound(t *testing.T) {
	b, runner, run, launchID := endedIntentBuild(t, launch.Completed, "RESULT: DONE\n")
	// The build command returned before its launch ended. No collector remains.
	if skipped := steward.BookEndedWork(b.root(), 5*time.Minute, runner); skipped != 0 {
		t.Fatal(skipped)
	}
	endpoint, err := b.dependencies().endpoint(b.root())
	if err != nil {
		t.Fatal(err)
	}
	actor := goal.Actor{Machine: "mac-cli", Lineage: b.goalFile(b.id).Claimed.Lineage}
	collectionOwnership(t, b, actor.Lineage, true)
	holder, err := lease.CurrentHolder(b.root())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("stop hook", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", home)
		if err := os.Symlink(filepath.Dir(b.unitRoot), filepath.Join(home, ".metasystem")); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := runReportTurnVerdictWithInputs([]string{"--root", b.root(), "--session", "replacement-session", "--main-id", holder.MainId},
			func(string) (goal.Endpoint, error) { return endpoint, nil }, func(string) (string, error) { return actor.Machine, nil }, &stdout, &stderr)
		var verdict goal.Verdict
		err = json.Unmarshal(stdout.Bytes(), &verdict)
		if code != 0 {
			t.Fatalf("stop hook command: %d %s", code, stderr.String())
		}
		if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" || !strings.Contains(verdict.Display, "metasystem work wait run:"+run+" --timeout 30m") {
			t.Fatalf("next session was not told to collect: %+v %v", verdict, err)
		}
	})
	code, result, _ := b.work("work", "wait", "run:"+run)
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" || resultData(t, result)["outcome"] != "green" {
		t.Fatalf("replacement collection: %d %s", code, result.Summary)
	}
	record, err := runner.Status(run)
	if err != nil || record.Rounds[0].Steps[0].LaunchID != launchID || record.Rounds[0].Steps[0].EndBooking.CollectedAt == "" {
		t.Fatalf("collection lost the original launch: state=%s error=%v", record.State, err)
	}
	t.Run("collected", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", home)
		if err := os.Symlink(filepath.Dir(b.unitRoot), filepath.Join(home, ".metasystem")); err != nil {
			t.Fatal(err)
		}
		runs, err := steward.UncollectedRuns(b.root(), []string{b.id})
		if err != nil || len(runs) != 0 {
			t.Fatalf("collected round still offered: %+v %v", runs, err)
		}
	})
}

func endedIntentBuild(t *testing.T, state launch.State, message string) (*workBed, *launch.UnitRunner, string, string) {
	t.Helper()
	b := newWorkBed(t)
	b.starter.hold = "build"
	code, result, _ := b.work(append([]string{"work", "build", b.id, "booked", "--brief", b.brief("brief.md", "Build the unit.\n"), "--lines", "20"}, workCheck...)...)
	if code != 3 {
		t.Fatalf("build did not wait: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	runner := &launch.UnitRunner{Root: b.unitRoot, Installation: b.root(), Git: workGit{b}, Manager: b.manager}
	record, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	id := record.Rounds[0].Steps[0].LaunchID
	b.manager.Sleep(time.Minute)
	_, err = b.manager.Store.Update(id, func(execution *launch.Record) error {
		exit := 0
		if state == launch.Failed {
			exit = 1
		}
		execution.State, execution.ExitCode, execution.FinishedAt = state, &exit, b.manager.Now().UTC().Format(time.RFC3339Nano)
		execution.Supervisor, execution.Child, execution.ProcessGroup = nil, nil, nil
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	directory, _ := b.manager.Store.StateDir(id)
	if err := os.WriteFile(filepath.Join(directory, "last-message.txt"), []byte(message), 0600); err != nil {
		t.Fatal(err)
	}
	b.manager.Sleep(6 * time.Minute)
	b.starter.hold = ""
	return b, runner, run, id
}

func TestWorkWaitContinuesTheStewardsBookedStep(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		state            launch.State
		message, outcome string
	}{
		{"done", launch.Completed, "RESULT: DONE\n", "green"},
		{"no result block", launch.Completed, "built the unit\n", "green"},
		{"partial", launch.Completed, "RESULT: PARTIAL\nOPEN: missing part\n", "build-partial"},
		{"reported failure", launch.Completed, "RESULT: FAILED\nOPEN: missing part\n", "build-failed"},
		{"failed", launch.Failed, "", "build-failed"},
		{"cancelled", launch.Cancelled, "", "build-failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, runner, run, id := endedIntentBuild(t, tc.state, tc.message)
			before := len(b.starter.launched())
			budgetBefore := dispatchcore.ProjectBudget(b.root(), b.goalFile(b.id), b.manager.Now())
			if budgetBefore.ActiveJobs != 1 {
				t.Fatalf("fixture has no open job: %+v", budgetBefore)
			}
			if skipped := steward.BookEndedWork(b.root(), 5*time.Minute, runner); skipped != 0 {
				t.Fatalf("booking skipped %d records", skipped)
			}
			record, err := runner.Status(run)
			if err != nil {
				t.Fatal(err)
			}
			if record.State != launch.UnitStepEnded || record.Rounds[0].Outcome != "" || len(b.starter.launched()) != before {
				t.Fatalf("booking continued or judged: %+v", record)
			}
			budget := dispatchcore.ProjectBudget(b.root(), b.goalFile(b.id), b.manager.Now())
			execution, _ := b.manager.Store.Read(id)
			start, _ := time.Parse(time.RFC3339Nano, execution.StartedAt)
			end, _ := time.Parse(time.RFC3339Nano, execution.FinishedAt)
			wantMinutes := uint64((end.Sub(start) + time.Minute - 1) / time.Minute)
			if budget.Status != dispatchcore.BudgetKnown || budget.ActiveJobs != 0 || budget.OpenCapMinutes != 0 || budget.ObservedJobMinutes != wantMinutes {
				t.Fatalf("booking did not release the exact charge: %+v want minutes %d", budget, wantMinutes)
			}
			if _, open := steward.OpenJob(b.root()); open {
				t.Fatal("engine follow still blocked")
			}
			code, status, _ := b.work("work", "status", b.id)
			if code != 0 || !strings.Contains(resultWords(status), "step ended") {
				t.Fatalf("work status hides booked end: %d %+v", code, status)
			}
			code, result, _ := b.work("work", "wait", b.id, "--work", "booked")
			record, err = runner.Status(run)
			if err != nil {
				t.Fatal(err)
			}
			if record.State != "awaiting-judgement" || record.Rounds[0].Outcome != tc.outcome || code != 0 && tc.outcome == "green" {
				t.Fatalf("wait did not continue: code=%d outcome=%+v record=%+v", code, result, record)
			}
			buildCount := 0
			for _, kind := range b.starter.launched() {
				if kind == "build" {
					buildCount++
				}
			}
			if buildCount != 1 || record.Rounds[0].Steps[0].EndBooking == nil || record.Rounds[0].Steps[0].EndBooking.CollectedAt == "" {
				t.Fatal("wait lost booking or rebuilt")
			}
			if tc.outcome != "green" && len(b.starter.launched()) != before {
				t.Fatal("failed build launched a successor")
			}
			if strings.Contains(tc.message, "OPEN:") {
				data, err := os.ReadFile(filepath.Join(record.Rounds[0].Directory, "open.md"))
				if err != nil || !strings.Contains(string(data), "missing part") {
					t.Fatal("normal collection lost partial result", err)
				}
			}
		})
	}
}

func TestWorkStopAfterStewardBookingKeepsThePersonsDecision(t *testing.T) {
	t.Parallel()
	b, runner, run, id := endedIntentBuild(t, launch.Completed, "RESULT: DONE\n")
	if skipped := steward.BookEndedWork(b.root(), 5*time.Minute, runner); skipped != 0 {
		t.Fatal(skipped)
	}
	path := filepath.Join(b.root(), "artifacts/agents/jobs", id+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, result, _ := b.work("work", "stop", "j2:"+id)
	if code != 0 || !strings.Contains(result.Summary, "already stopped") {
		t.Fatalf("repeat stop: %d %+v", code, result)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("repeat stop rewrote booked job")
	}
	code, result, _ = b.work("work", "stop", "j2:"+id, "--reason", "confirm the completed job stays stopped")
	if code != 0 {
		t.Fatalf("repeat stop with reason: %d %+v", code, result)
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("reason on repeated stop rewrote the terminal job")
	}
	owners := b.workOwners()
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	code, result = b.runJSON(owners, "work", "stop", "run:"+run, "--reason", "release the retained worktree")
	if code != 0 {
		t.Fatalf("stop run: %d %+v", code, result)
	}
	overrides, _ := filepath.Glob(filepath.Join(b.root(), "artifacts/agents/channel/unit-stop-overrides/*.json"))
	foundReason := false
	for _, override := range overrides {
		data, _ := os.ReadFile(override)
		foundReason = foundReason || bytes.Contains(data, []byte("release the retained worktree"))
	}
	if !foundReason {
		t.Fatal("the person's stop reason was not retained")
	}
	path = filepath.Join(b.unitRoot, run, "run.json")
	before, _ = os.ReadFile(path)
	steward.BookEndedWork(b.root(), 5*time.Minute, runner)
	after, _ = os.ReadFile(path)
	record, err := runner.Status(run)
	if err != nil || record.State != "cancelled" || !bytes.Equal(before, after) {
		t.Fatal("booking overruled the person's stop", err)
	}
}

func TestLiveWorkWaitJoinsStewardBooking(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	code, result, _ := b.work(append([]string{"work", "build", b.id, "booked", "--brief", b.brief("brief.md", "Build the unit.\n"), "--lines", "20"}, workCheck...)...)
	if code != 3 {
		t.Fatalf("build: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	runner := &launch.UnitRunner{Root: b.unitRoot, Installation: b.root(), Git: workGit{b}, Manager: b.manager}
	record, _ := runner.Status(run)
	id := record.Rounds[0].Steps[0].LaunchID
	sleep := b.manager.Sleep
	booked := false
	b.manager.Sleep = func(d time.Duration) {
		if booked {
			sleep(d)
			return
		}
		booked = true
		sleep(time.Minute)
		if _, err := b.manager.Store.Update(id, func(execution *launch.Record) error {
			exit := 0
			execution.State, execution.ExitCode, execution.FinishedAt = launch.Completed, &exit, b.manager.Now().UTC().Format(time.RFC3339Nano)
			execution.Supervisor, execution.Child, execution.ProcessGroup = nil, nil, nil
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		sleep(6 * time.Minute)
		if skipped := steward.BookEndedWork(b.root(), 5*time.Minute, runner); skipped != 0 {
			t.Fatalf("concurrent booking skipped %d", skipped)
		}
	}
	code, result, _ = b.work("work", "wait", b.id, "--work", "booked")
	if !booked || code != 0 || !strings.Contains(result.Summary, "steward recorded") {
		t.Fatalf("live wait did not join booking: %d %+v", code, result)
	}
	if len(b.starter.launched()) != 1 {
		t.Fatal("racing wait started a successor from stale state")
	}
	b.manager.Sleep = sleep
	b.starter.hold = ""
	code, result, _ = b.work("work", "wait", b.id, "--work", "booked")
	if code != 0 || resultData(t, result)["outcome"] != "green" {
		t.Fatalf("next wait did not collect: %d %+v", code, result)
	}
}

func TestSystemStatusShowsBookedWorkUntilCollection(t *testing.T) {
	t.Parallel()
	b, runner, run, _ := endedIntentBuild(t, launch.Completed, "RESULT: DONE\n")
	if skipped := steward.BookEndedWork(b.root(), 5*time.Minute, runner); skipped != 0 {
		t.Fatal(skipped)
	}
	p := &processBed{intentBed: b.intentBed, class: lease.ClassHuman, launchDir: b.manager.Store.Root, home: t.TempDir()}
	owners := b.workOwners()
	owners.processes = p.owners().processes
	code, status := b.runJSON(owners, "system", "status")
	data, _ := json.Marshal(status.Data)
	if code != 0 || !bytes.Contains(data, []byte("ended; awaiting collection")) || !bytes.Contains(data, []byte("work wait run:"+run)) {
		t.Fatalf("system status lost uncollected work: %d %+v %s", code, status, data)
	}
	if code, result, _ := b.work("work", "wait", "run:"+run); code != 0 {
		t.Fatalf("collect: %d %s", code, result.Summary)
	}
	code, status = b.runJSON(owners, "system", "status")
	data, _ = json.Marshal(status.Data)
	if code != 0 || bytes.Contains(data, []byte("work wait run:"+run)) {
		t.Fatalf("system status still offers collected work: %d %s", code, data)
	}
}
