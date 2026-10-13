package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestEveryLandingVerbIsGuarded(t *testing.T) {
	t.Parallel()
	for _, command := range publicIntentCommands() {
		if command.object != "landing" {
			continue
		}
		t.Run(command.action, func(t *testing.T) {
			t.Parallel()
			if command.run == nil {
				t.Fatal("landing action bypasses the shared guarded intent runner")
			}
			b := newLaneVerbBed(t)
			if code, out, errout := b.run(t, "landing", "set", b.landingA); code != 0 {
				t.Fatalf("set: %d %s %s", code, out, errout)
			}
			command.run = func(inv *intentInvocation) int { panic("nil error in landingLaneFailure") }
			var out, errout bytes.Buffer
			code := runIntentIn(command, nil, &out, &errout, b.cwd, b.owners())
			if code != 2 || out.Len() != 0 || strings.Count(errout.String(), "\n") != 1 || !strings.Contains(errout.String(), "landing "+command.action+" failed inside the engine: nil error in landingLaneFailure; incident ") {
				t.Fatalf("unguarded %s: %d %q %q", command.action, code, out.String(), errout.String())
			}
			incidents, err := plain.EngineIncidents(filepath.Join(b.landingA, "metasystem"))
			if err != nil || len(incidents) != 1 || incidents[0].Verb != command.action || !strings.Contains(incidents[0].Stack, "landing_engine_guard_test.go") {
				t.Fatalf("incident: %+v %v", incidents, err)
			}
		})
	}
}

func TestLandingEngineIncidentClearIsPersonsActAndRepeats(t *testing.T) {
	t.Parallel()
	landingEngineIncidentClearRepeat(t)
}

func landingEngineIncidentClearRepeat(t *testing.T) {
	t.Helper()
	b := newLaneVerbBed(t)
	if code, out, errout := b.run(t, "landing", "set", b.landingA); code != 0 {
		t.Fatalf("set: %d %s %s", code, out, errout)
	}
	install := filepath.Join(b.landingA, "metasystem")
	incident, err := plain.RecordEngineIncident(install, "merge", []string{"goal-a"}, laneTestNow, "nil error", "the stack")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(plain.Dir(install), "incidents", incident.ID+".json")
	b.person = errors.New("agent session")
	code, _, _ := b.run(t, "landing", "incident", "clear", incident.ID)
	if code == 0 {
		t.Fatal("agent cleared incident")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("agent changed incident", err)
	}
	b.person = nil
	for range 2 {
		code, out, errout := b.run(t, "landing", "incident", "clear", incident.ID)
		if code != 0 || !strings.Contains(out, "Engine incident cleared") {
			t.Fatalf("clear: %d %q %q", code, out, errout)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("incident remains", err)
	}
}

func init() {
	registerIdempotency("landing incident", idemStateful, "clearing an absent incident repeats as success", landingEngineIncidentClearRepeat)
}

func TestLandingGuardPreservesProgressOutput(t *testing.T) {
	t.Parallel()
	var out, errout bytes.Buffer
	code := guardLanding(t.TempDir(), "prove", nil, &out, &errout, func(stdout, _ io.Writer) int {
		_, _ = io.WriteString(stdout, "proving\n")
		if out.String() != "proving\n" {
			t.Fatal("the guard held progress until the command ended")
		}
		return 0
	}, func() (string, error) { return "", errors.New("no lane") })
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("success changed: %d %q", code, errout.String())
	}
}

func TestLandingIncidentClearAllRequiresPersonAndRepeats(t *testing.T) {
	t.Parallel()
	b := newLaneVerbBed(t)
	if code, out, errout := b.run(t, "landing", "set", b.landingA); code != 0 {
		t.Fatalf("set: %d %s %s", code, out, errout)
	}
	install := filepath.Join(b.landingA, "metasystem")
	first, err := plain.RecordEngineIncident(install, "merge", nil, laneTestNow, "first panic", "stack")
	if err != nil {
		t.Fatal(err)
	}
	second, err := plain.RecordEngineIncident(install, "merge", nil, laneTestNow, "second panic", "stack")
	if err != nil || second.ID == first.ID {
		t.Fatalf("same-tick incidents: %+v %v", second, err)
	}
	if code, out, errout := b.run(t, "landing", "incident", "clear", first.ID); code != 0 {
		t.Fatalf("clear one: %d %s %s", code, out, errout)
	}
	incidents, err := plain.EngineIncidents(install)
	if err != nil || len(incidents) != 1 || incidents[0].ID != second.ID {
		t.Fatalf("clear one: %+v %v", incidents, err)
	}
	b.person = errors.New("agent session")
	if code, _, _ := b.run(t, "landing", "incident", "clear", "--all"); code == 0 {
		t.Fatal("agent cleared all")
	}
	incidents, err = plain.EngineIncidents(install)
	if err != nil || len(incidents) != 1 {
		t.Fatalf("agent changed incidents: %+v %v", incidents, err)
	}
	b.person = nil
	for range 2 {
		if code, out, errout := b.run(t, "landing", "incident", "clear", "--all"); code != 0 {
			t.Fatalf("clear all: %d %s %s", code, out, errout)
		}
	}
	incidents, err = plain.EngineIncidents(install)
	if err != nil || len(incidents) != 0 {
		t.Fatalf("clear all: %+v %v", incidents, err)
	}
}
