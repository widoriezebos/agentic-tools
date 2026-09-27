package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// Rule H1: job breach-stop admits a person, and the stop it publishes is
// ordered by that person's name; the machinery custodians keep recording the
// custodian lineage and cannot put a person's name on their act.
func TestBreachStopAdmitsThePersonAndNamesThem(t *testing.T) {
	t.Parallel()
	human := lease.ClassifyResult{Class: lease.ClassHuman}
	if err := authority.Authorize("stop-custodian", map[string]any{"class": human.Class, "holder": human.Holder}, ""); err != nil {
		t.Fatalf("the stop custodian mode refused a person: %v", err)
	}
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	failingProof := func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("TERMINAL_NOT_ENROLLED")
	}
	proofCalled := false
	unusedProof := func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		proofCalled = true
		return humanauthority.Proof{}, errors.New("unused")
	}
	root := t.TempDir()

	name, err := breachStopOrderingHuman(root, human, "Wido", now, unusedProof)
	if err != nil || name != "Wido" || proofCalled {
		t.Fatalf("a person's --by = %q %v (proof read %v), want Wido without a proof read", name, err, proofCalled)
	}

	name, err = breachStopOrderingHuman(root, human, "", now, failingProof)
	if err == nil || name != "" || !strings.Contains(err.Error(), "--by NAME") ||
		!strings.Contains(err.Error(), "metasystem system enroll --name NAME") {
		t.Fatalf("an unnamed person was not guided to --by or enrollment: %q %v", name, err)
	}

	for _, custodian := range []lease.ClassifyResult{{Class: lease.ClassMain, Holder: true}, {Class: lease.ClassSteward}} {
		name, err = breachStopOrderingHuman(root, custodian, "", now, unusedProof)
		if err != nil || name != "" {
			t.Fatalf("%s custodian stop = %q %v, want the custodian lineage", custodian.Class, name, err)
		}
		if _, err = breachStopOrderingHuman(root, custodian, "Wido", now, unusedProof); err == nil {
			t.Fatalf("%s custodian put a person's name on its stop", custodian.Class)
		}
	}
	if proofCalled {
		t.Fatal("a custodian's stop read the human proof")
	}
}
