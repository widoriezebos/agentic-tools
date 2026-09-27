package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// Witness R14 (verbs-object-action 3.6, rule H1): a person is never
// dead-ended. Every control-plane authority mode admits the authenticated
// human, and every register refusal a person can meet either asks for a
// corrected word or guides with a runnable public command. The register half
// (every row has a standing; a guide row's message names its command near its
// site) is internal/refusal's TestH1EveryRowHasAStanding; this half binds the
// named commands to the public table, which lives here.
func TestR14AuthorityModesAdmitThePerson(t *testing.T) {
	t.Parallel()
	modes := authority.Modes()
	if len(modes) == 0 {
		t.Fatal("the authority mode set is empty")
	}
	for _, mode := range modes {
		if !authority.ValidMode(mode) {
			t.Errorf("mode %s from authority.Modes is not valid", mode)
		}
		for _, holder := range []bool{false, true} {
			if err := authority.Authorize(mode, map[string]any{"class": lease.ClassHuman, "holder": holder}, ""); err != nil {
				t.Errorf("R14: authority mode %s refuses the authenticated person (holder=%v): %v", mode, holder, err)
			}
		}
	}
}

func TestR14GuideRowsNameRunnablePublicCommands(t *testing.T) {
	t.Parallel()
	public := map[string]bool{}
	for _, command := range publicIntentCommands() {
		public[command.name] = true
	}
	guides := 0
	for _, row := range refusal.Rows {
		if row.Standing() != refusal.StandingGuide || row.Forward == "" {
			continue
		}
		guides++
		if !public[row.Forward] {
			t.Errorf("R14: guide row %s names metasystem %s, which is not a runnable public command", row.Code, row.Forward)
		}
	}
	if guides == 0 {
		t.Fatal("R14: the register has no guide rows naming a public command")
	}
}

// The witness is not vacuous: a forward outside the public table, or a
// hidden entrypoint, is caught by the same lookup.
func TestR14PublicLookupRefusesNonPublicForms(t *testing.T) {
	t.Parallel()
	public := map[string]bool{}
	for _, command := range publicIntentCommands() {
		public[command.name] = true
	}
	for _, form := range []string{"goal set-budget", "goal steal", "goal carry", "job breach-stop"} {
		if public[form] {
			t.Errorf("%q must not be a runnable public command", form)
		}
	}
	for _, command := range intentCommands() {
		if command.hidden && public[command.name] {
			t.Errorf("hidden entry %q counted as public", command.name)
		}
	}
	for _, form := range []string{"goal budget", "goal claim", "work stop", "goal notes"} {
		if !public[form] {
			t.Errorf("%q must be a runnable public command", form)
		}
	}
}
