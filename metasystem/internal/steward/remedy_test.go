package steward

import (
	"encoding/json"
	"flag"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var remedyBedRoot = flag.String("remedy-bed-root", "", "health command bed's isolated state root")
var remedyBedLedger = flag.String("remedy-bed-ledger", "", "health command bed's accepted ledger snapshot")
var remedyBedOutput = flag.String("remedy-bed-output", "", "health command bed's verdict output")

func TestHealthRemedyTableNamesNoRead(t *testing.T) {
	t.Parallel()
	// The command bed enters the real health preview with only the process,
	// clock and accepted repository read supplied by this fixture.
	if *remedyBedRoot != "" {
		b := newHealthBedAt(t, *remedyBedRoot, "human", "")
		b.base = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
		b.dead(healthBedRef(52001))
		b.dead(b.runner)
		data, err := os.ReadFile(*remedyBedLedger)
		if err != nil {
			t.Fatal(err)
		}
		var files map[string][]byte
		if err := json.Unmarshal(data, &files); err != nil {
			t.Fatal(err)
		}
		for path := range files {
			if !strings.HasPrefix(path, "plans/goals/") && !strings.HasPrefix(path, "records/goals/") {
				delete(files, path)
			}
		}
		tree, problems := goal.ParseTreeFiles(files)
		if len(problems) != 0 {
			t.Fatalf("ledger: %v", problems)
		}
		ledger := newHealthLedger(b.root, b.base)
		ledger.readWorld = func(string) bool { return true }
		ledger.readEndpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{Root: b.root, Remote: "local"}, nil }
		ledger.project = func(goal.Endpoint, bool, time.Time) (goal.Projection, error) {
			return goal.Projection{Root: b.root, Tree: tree, Horizon: goal.ApprovalHorizon{Now: b.base}}, nil
		}
		verdict := previewHealthAtWithEvaluation(b.root, b.root, b.base, b.probe,
			func(repo, installation string, now time.Time, prober identity.Prober, hook bool) ([]RoleVerdict, SpendObservation) {
				return evaluateHealthRolesWithLedger(repo, repo, installation, now, prober, hook, measureSpend, ledger)
			})
		data, err = json.Marshal(verdict)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(*remedyBedOutput, data, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, role := range []HealthRole{RoleStewardRunner, RoleSupervisionOwner, RoleRepoWatcher, RoleNarratorFreshness, RoleCensusFreshness, RoleHookFreshness, RoleSessionMain} {
		remedy := remedyFor(role, RemedyFact{})
		for _, audience := range []string{"human", "agent"} {
			act, _ := remedy.Render(audience, nil)
			want := "metasystem system start"
			if audience == "agent" {
				want = "metasystem session start"
			}
			if strings.Join(act, " ") != want {
				t.Errorf("%s for %s: %v want %s", role, audience, act, want)
			}
		}
	}
	facts := []RemedyFact{
		{Cause: CauseBudgetMissing, Goal: "g"}, {Cause: CauseBudgetMalformed, Goal: "g"},
		{Cause: CauseBudgetBreach, Goal: "g"}, {Cause: CauseBudgetUnknown, Record: "record"},
		{Cause: CauseBreachStopOpen, Goal: "g", Stop: "stop"}, {Cause: CauseBreachStopUnresolved, Goal: "g", Stop: "stop"},
		{Cause: CauseEpochMismatch}, {Cause: CauseForeignLineage, Goal: "g"},
		{Cause: CauseStopCapabilityMissing, Goal: "g", Record: "record"},
		{Cause: CauseJobProcessDead, Job: "job"}, {Cause: CauseTrunkRedUnowned, Incident: "incident"},
		{Cause: "unrecognized"},
	}
	for _, role := range append([]HealthRole{RoleTrunkRed}, healthBedFreshRoles...) {
		cases := append([]RemedyFact{{}}, facts...)
		for _, fact := range cases {
			remedy := remedyFor(role, fact)
			if len(remedy.Act) == 0 && remedy.Plain == "" {
				continue
			}
			for _, audience := range []string{"human", "agent"} {
				act, plain := remedy.Render(audience, nil)
				if remedy.Clears == "" {
					t.Errorf("%s/%s has no clearing effect", role, fact.Cause)
				}
				if len(act) > 0 {
					if len(act) < 3 || act[0] != "metasystem" {
						t.Errorf("not public: %v", act)
						continue
					}
					switch act[2] {
					case "list", "show", "check", "status":
						t.Errorf("read is no remedy: %v", act)
					}
				} else if !strings.Contains(plain, "person") && !strings.Contains(plain, "steward's next pass") && !strings.Contains(plain, "lane records the cadence at its next validation") && !strings.Contains(plain, "claiming session refreshes its stop capability at its next") {
					t.Errorf("plain remedy names no actor and clearing time: %q", plain)
				}
			}
		}
	}
}
