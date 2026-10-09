package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestHealthRemedyActsMatchIntentAudience(t *testing.T) {
	t.Parallel()
	facts := []steward.RemedyFact{
		{},
		{Cause: steward.CauseBudgetMissing, Goal: "g"}, {Cause: steward.CauseBudgetMalformed, Goal: "g"},
		{Cause: steward.CauseBudgetBreach, Goal: "g"}, {Cause: steward.CauseBudgetUnknown, Record: "record"},
		{Cause: steward.CauseBreachStopOpen, Goal: "g", Stop: "stop"}, {Cause: steward.CauseBreachStopUnresolved, Goal: "g", Stop: "stop"},
		{Cause: steward.CauseEpochMismatch}, {Cause: steward.CauseForeignLineage, Goal: "g"},
		{Cause: steward.CauseStopCapabilityMissing, Goal: "g", Record: "record"},
		{Cause: steward.CauseJobProcessDead, Job: "job"}, {Cause: steward.CauseTrunkRedUnowned, Incident: "incident"},
		{Cause: "unrecognized"},
	}
	for _, role := range []steward.HealthRole{
		steward.RoleStewardRunner, steward.RoleSupervisionOwner, steward.RoleRepoWatcher,
		steward.RoleNarratorFreshness, steward.RoleCensusFreshness, steward.RoleHookFreshness,
		steward.RoleSessionMain, steward.RoleClaimedGoalBudget, steward.RoleNonterminalJobs, steward.RoleTrunkRed,
	} {
		for _, fact := range facts {
			for _, audience := range []string{"human", "agent"} {
				act, plain := publicHealthRemedy(steward.RoleVerdict{Role: role, Reason: "no validation cadence", RemedyFacts: []steward.RemedyFact{fact}}, false, audience)
				if len(act) == 0 {
					if plain == "" {
						t.Errorf("%s/%s for %s has no remedy", role, fact.Cause, audience)
					}
					continue
				}
				command, _, ok := resolveIntentArgv(act[1:])
				if !ok || act[0] != "metasystem" || command.audience != "both" && command.audience != audience {
					t.Errorf("%s/%s for %s: act %v is not public for its reader (audience %q)", role, fact.Cause, audience, act, command.audience)
				}
				if slices.Contains([]string{"list", "show", "check", "status"}, command.action) {
					t.Errorf("%s/%s for %s: read is no remedy: %v", role, fact.Cause, audience, act)
				}
			}
		}
	}
}

func TestHealthSystemCheckAgentNamesPersonsBudgetAct(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	for _, class := range []string{lease.ClassMain, lease.ClassDelegate} {
		b := newProcessBed(t)
		b.class = class
		setHealthRemedyMissingBudget(t, b)
		owners := b.owners()
		owners.processes.health = healthRemedyPreview(t, b)
		code, result := b.runJSON(owners, "system", "check")
		data, err := json.Marshal(result.Data)
		if err != nil {
			t.Fatal(err)
		}
		var preview struct {
			Verdict        steward.HealthVerdict
			PublicRemedies []struct {
				Role        steward.HealthRole
				Public      []string
				Instruction string
			}
		}
		if err := json.Unmarshal(data, &preview); err != nil {
			t.Fatal(err)
		}
		index := slices.IndexFunc(preview.Verdict.Roles, func(role steward.RoleVerdict) bool { return role.Role == steward.RoleClaimedGoalBudget })
		if code != 1 || index < 0 || preview.Verdict.Roles[index].Status != steward.HealthDead || len(preview.Verdict.Roles[index].RemedyFacts) == 0 || preview.Verdict.Roles[index].RemedyFacts[0].Cause != steward.CauseBudgetMissing {
			t.Fatalf("%s: system check did not observe a missing budget: exit %d, %+v", class, code, preview.Verdict)
		}
		index = slices.IndexFunc(preview.PublicRemedies, func(remedy struct {
			Role        steward.HealthRole
			Public      []string
			Instruction string
		}) bool {
			return remedy.Role == steward.RoleClaimedGoalBudget
		})
		want := "a person runs: metasystem goal budget " + bedGoal + " BOX"
		if index < 0 || len(preview.PublicRemedies[index].Public) != 0 || preview.PublicRemedies[index].Instruction != want {
			t.Fatalf("%s: agent must receive the person's budget instruction %q: %+v", class, want, preview.PublicRemedies)
		}
		if result.Next != nil && strings.Contains(strings.Join(result.Next.Argv, " "), "goal budget") {
			t.Fatalf("%s: person's budget verb offered as agent's next act: %+v", class, result.Next)
		}
		t.Logf("%s: system check exit %d; %s", class, code, want)
	}
}
