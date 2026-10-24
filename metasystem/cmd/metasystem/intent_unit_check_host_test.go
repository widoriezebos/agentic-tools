package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func loadedHost(at time.Time) hostload.Sample {
	return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: 9}
}

func TestUnitOutcomeSaysTheCheckWaitsForTheHost(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ kind, want string }{
		{"attest", "current host load meets or exceeds host.load-max; the unit check waits for the host"},
		{"build", "current host load meets or exceeds host.load-max; no build was launched"},
	} {
		t.Run(row.kind, func(t *testing.T) {
			t.Parallel()
			inv := &intentInvocation{stateRoot: t.TempDir(), layout: stateroot.Layout{InstallationRoot: stateroot.Installation(t.TempDir())}}
			record := launch.UnitRunRecord{Goal: "goal", Rounds: []launch.UnitRound{
				{Number: 1, Steps: []launch.UnitStep{{Name: "build", Kind: "build", State: launch.StepFailed}}},
				{Number: 2, Steps: []launch.UnitStep{{Name: "build", Kind: "build", State: launch.StepPassed}, {Name: "proof:unit-check", Kind: "attest", State: launch.StepPending}}},
			}}
			newest := &record.Rounds[1]
			if row.kind == "attest" {
				newest.Steps[1].State = launch.StepStarting
			} else {
				newest.Steps[0].State = launch.StepStarting
			}
			err := refusal.New("LAUNCH_BUILD_CAPACITY", "", errors.New("current host load meets or exceeds host.load-max"))
			result := inv.unitOutcome(nil, launch.UnitResult{Record: record}, err, inv.targets("goal"), []string{"metasystem", "work", "wait", "goal"})
			if result.Outcome != intentRefused || result.Summary != row.want {
				t.Fatalf("summary %q, want %q", result.Summary, row.want)
			}
		})
	}
}

func TestIntentPersonReviewStartsTheCheckOnALoadedHost(t *testing.T) {
	t.Parallel()
	t.Run("retry", func(t *testing.T) {
		t.Parallel()
		b, p := failedCommandBed(t, 2)
		code, result := failedCommandBuild(t, b)
		if code != 1 || result.Next == nil {
			t.Fatalf("missing failed-step remedy: %d %+v", code, result)
		}
		args := append([]string(nil), result.Next.Argv[1:]...)
		for i := range args {
			if args[i] == "TEXT" {
				args[i] = "The environment is repaired"
			}
			if args[i] == "NAME" {
				args[i] = "Wido"
			}
		}
		b.manager.CapacitySources.Load = loadedHost
		p.passAfter = 2
		code, admitted, output := b.work(args...)
		after := failedCommandRecord(t, b, admitted)
		if code != 0 || len(p.commands) != 3 || after.Rounds[0].Outcome != "green" {
			t.Fatalf("the person's retry did not start the check on a loaded host: %d %+v commands=%d %s", code, admitted, len(p.commands), output)
		}
	})
	t.Run("size", func(t *testing.T) {
		t.Parallel()
		b := outcomeBed(t, "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+one\n+two\n+three\n")
		code, held, _ := outcomeBuild(t, b)
		if code != 1 || resultData(t, held)["outcome"] != "build-size" {
			t.Fatalf("size did not hold: %d %+v", code, held)
		}
		b.manager.CapacitySources.Load = loadedHost
		var stderr bytes.Buffer
		code, accepted := outcomeReview(t, b, true, &stderr)
		if code != 0 || resultData(t, accepted)["outcome"] != "green" || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
			t.Fatalf("the person's size acceptance did not start the check on a loaded host: %d %+v %v", code, accepted, b.starter.launched())
		}
	})
}

func TestKilledCheckEndsWithItsLaunch(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name          string
		minutes       float64
		lowerBound    bool
		running, next bool
	}{
		{name: "launch-finished", minutes: 30},
		{name: "launch-running", minutes: 150, lowerBound: true, running: true},
		{name: "step-retried", minutes: 30, next: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
			clock := &workClock{now: start}
			bed.manager.Now = clock.Now
			bed.manager.Supervisor = processCostStarter{bed.starter, clock}
			code, built, output := processEvidenceBuild(bed)
			if code != 0 {
				t.Fatalf("build exit %d: %+v %s", code, built, output)
			}
			run := resultData(t, built)["run"].(string)
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil || record.Rounds[0].Steps[0].Kind != "build" {
				t.Fatalf("record: %+v %v", record, err)
			}
			round, original := record.Rounds[0], record.Rounds[0].Steps[0].LaunchID
			killedAt, finishedAt := start.Add(10*time.Minute), start.Add(40*time.Minute)
			launchRecord, err := bed.manager.Store.Update(original, func(r *launch.Record) error {
				r.StartedAt, r.FinishedAt = start.Format(time.RFC3339Nano), finishedAt.Format(time.RFC3339Nano)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if row.running {
				// A finished launch never becomes running again through the store, so the fixture writes the record a live launch has.
				launchRecord.State, launchRecord.FinishedAt = launch.Running, ""
				directory, err := bed.manager.Store.StateDir(original)
				data, _ := json.Marshal(launchRecord)
				if err != nil || os.WriteFile(filepath.Join(directory, "record.json"), data, 0o600) != nil {
					t.Fatalf("running launch: %v", err)
				}
			}
			if row.next {
				retry := launchRecord
				retry.ID = original + "-retry"
				retry.StartedAt, retry.FinishedAt = finishedAt.Add(5*time.Minute).Format(time.RFC3339Nano), finishedAt.Add(10*time.Minute).Format(time.RFC3339Nano)
				if err := bed.manager.Store.Create(retry); err != nil {
					t.Fatal(err)
				}
				record.Rounds[0].Steps[0].LaunchID = retry.ID
				record.Rounds[0].Steps[0].LaunchIDs = []string{original, retry.ID}
				data, _ := json.Marshal(record)
				if err := os.WriteFile(filepath.Join(bed.unitRoot, run, "run.json"), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			clock.mu.Lock()
			clock.now = finishedAt.Add(2 * time.Hour)
			clock.mu.Unlock()
			before := processCostStatus(t, bed)
			killed := launch.CheckExecution{ExecutionID: "check-killed", Goal: bed.id, Run: run, Round: filepath.Base(round.Directory), Role: "builder",
				Steps: []processmeasure.Step{{ID: run + "/" + filepath.Base(round.Directory) + "/check-killed:cheap", Kind: "attest", Parent: original, Start: killedAt.Format(time.RFC3339Nano), Argv: []string{"/bin/sh", "-c", "check"}, Coverage: "unknown"}}}
			data, _ := json.Marshal(killed)
			if err := os.MkdirAll(filepath.Join(round.Directory, "check-killed"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(round.Directory, "check-killed", "observation.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			after := processCostStatus(t, bed)
			if before.Hours["attest"] == nil || after.Hours["attest"] == nil {
				t.Fatalf("attest hours unavailable: before %+v after %+v", before, after)
			}
			if got := (*after.Hours["attest"] - *before.Hours["attest"]) * 60; got < row.minutes-1e-6 || got > row.minutes+1e-6 || after.WorkLowerBound != row.lowerBound {
				t.Fatalf("the killed check counted %g minutes (lower bound %t), want %g (lower bound %t); unknown %v", got, after.WorkLowerBound, row.minutes, row.lowerBound, strings.Join(after.Unknown, "; "))
			}
		})
	}
}
