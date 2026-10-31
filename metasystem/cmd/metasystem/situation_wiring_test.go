package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// situationCheckout makes the work bed's checkout a machine's accepted
// ledger with an engine, and returns its installation, where the page lives.
func situationCheckout(t *testing.T, bed *workBed) string {
	t.Helper()
	root := bed.root()
	for _, args := range [][]string{{"init", "-q"}, {"config", "metasystem.goal.machine", "mac-cli"}, {"add", "plans/goals/backlog.md"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "ledger"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	layout, err := bed.workOwners().resolver.ResolveLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	install := layout.InstallationRoot.Path()
	engine := filepath.Join(install, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return install
}

// countedSituationOwners are the bed's owners with injected situation
// sources at *clock whose ledger projections are counted; a live launch
// read fails the test.
func countedSituationOwners(t *testing.T, bed *workBed, clock *time.Time, projections *int) intentOwners {
	t.Helper()
	root, owners := bed.root(), bed.workOwners()
	ledger := owners.dependencies.endpoint
	owners.processes.launches = func() *launch.Manager {
		t.Error("the situation read the live launch manager, not the given sources")
		return &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
	}
	board := t.TempDir()
	owners.situation = situationEmptyHost(t, steward.SituationSources{Now: func() time.Time { return *clock },
		Project: func(_ goal.Endpoint, fetch bool, now time.Time) (goal.Projection, error) {
			*projections++
			endpoint, err := ledger(root)
			if err != nil {
				return goal.Projection{}, err
			}
			return goal.Project(endpoint, fetch, now)
		},
		HostWork: func() (int, int, error) { return 0, 0, nil }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		BoardHome: func() (string, error) { return board, nil }, Launches: func() ([]launch.Record, error) { return nil, nil },
		Holder: func(string) (steward.SeatHolder, error) { return steward.SeatHolder{}, nil }})
	return owners
}

// The tick's page is written through the wired reader as the duty writes it
// (SituationPage with no freshness); the verb a minute later prints it and
// reads nothing.
func TestTheWiredTickWritesThePageTheVerbReads(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	install := situationCheckout(t, bed)
	clock, projections := situationNow, 0
	owners := countedSituationOwners(t, bed, &clock, &projections)
	var config steward.TickConfig
	wireStewardSeat(&config, owners)
	if config.Situation != nil {
		if _, err := steward.SituationPage(install, func(at time.Time) steward.Situation { return config.Situation(install, at) }, clock, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(install, "artifacts", "agents", "steward", "situation.json")); err != nil || projections != 1 {
		t.Fatalf("the wired tick wrote no page (%v) or read the ledger %d times, want once", err, projections)
	}
	clock = clock.Add(time.Minute)
	command, _, _ := resolveIntentArgv([]string{"system", "situation"})
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, nil, &stdout, &stderr, bed.root(), owners); code != 0 || !strings.Contains(stdout.String(), ", 1m ago") || projections != 1 {
		t.Fatalf("system situation = %d, ledger read %d times, want the tick's page and no read:\n%s%s", code, projections, stdout.String(), stderr.String())
	}
}
