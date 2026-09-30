package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// Rule H1: a breach stop admits a person, and the stop the delegation
// lifecycle publishes for one (engineHost.BreachStopOrderingHuman) is ordered
// by the enrolled person's name, proven as the sibling human verbs prove it;
// a --by must match it. The machinery custodians keep recording
// the custodian lineage and cannot put a person's name on their act.
func TestBreachStopAdmitsThePersonAndNamesThem(t *testing.T) {
	t.Parallel()
	human := lease.ClassifyResult{Class: lease.ClassHuman}
	if err := authority.Authorize("stop-custodian", map[string]any{"class": human.Class, "holder": human.Holder}, ""); err != nil {
		t.Fatalf("the stop custodian mode refused a person: %v", err)
	}
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	enrolled := func(string, time.Time) (string, error) { return "Wido", nil }
	unproven := func(string, time.Time) (string, error) { return "", errors.New("TERMINAL_NOT_ENROLLED") }
	nameRead := false
	unused := func(string, time.Time) (string, error) { nameRead = true; return "", errors.New("unused") }

	for _, by := range []string{"", "Wido", "human:Wido"} {
		name, err := breachStopOrderingHumanWith(root, human, by, now, enrolled)
		if err != nil || name != "Wido" {
			t.Fatalf("a person's stop with --by %q = %q %v, want the enrolled name Wido", by, name, err)
		}
	}
	if _, err := breachStopOrderingHumanWith(root, human, "Mallory", now, enrolled); err == nil ||
		!strings.Contains(err.Error(), "--by Mallory is not the person enrolled at this terminal") {
		t.Fatalf("a --by naming someone else was not refused: %v", err)
	}
	for _, by := range []string{"", "Wido"} {
		name, err := breachStopOrderingHumanWith(root, human, by, now, unproven)
		if err == nil || name != "" || !strings.Contains(err.Error(), "metasystem system enroll --name ") {
			t.Fatalf("an unproven person (--by %q) was not guided to enrollment: %q %v", by, name, err)
		}
	}
	for _, custodian := range []lease.ClassifyResult{{Class: lease.ClassMain, Holder: true}, {Class: lease.ClassSteward}} {
		name, err := breachStopOrderingHumanWith(root, custodian, "", now, unused)
		if err != nil || name != "" {
			t.Fatalf("%s custodian stop = %q %v, want the custodian lineage", custodian.Class, name, err)
		}
		if _, err = breachStopOrderingHumanWith(root, custodian, "Wido", now, unused); err == nil {
			t.Fatalf("%s custodian put a person's name on its stop", custodian.Class)
		}
	}
	if nameRead {
		t.Fatal("a custodian's stop read the human proof")
	}

}
