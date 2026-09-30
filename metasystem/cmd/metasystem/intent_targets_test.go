package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// TestIntentProcessTargets: start, stop and status name the interface and a
// new machine as targets and reach their owners (the interface lifecycle
// seam and the seat launch verb as an engine process seam); options of
// another target refuse before any owner runs.
func TestIntentProcessTargets(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	var verbs []string
	var received []uiIntentOptions
	owners.processes.ui = func(verb string, _ lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error) {
		verbs = append(verbs, verb)
		received = append(received, options)
		return uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"ui " + verb}}, State: lifecycle.Running, Restart: &lifecycle.RestartReport{Started: true}}, nil
	}
	var engine [][]string
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			engine = append(engine, process.argv)
			return intentProcessResult{stdout: []byte(`{"state":"supervised"}`)}
		}}
	for _, args := range [][]string{{"ui", "start"}, {"ui", "stop"}, {"ui", "status"}} {
		if code, result := b.runJSON(owners, args...); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("%v = %d %+v", args, code, result)
		}
	}
	if !slices.Equal(verbs, []string{"start", "stop", "status"}) {
		t.Fatalf("interface verbs = %v", verbs)
	}
	// The options carry a closure, so they compare field by field; each
	// verb is handed the reader of the other seats.
	sameOptions := func(got, want uiIntentOptions) bool {
		return got.listen == want.listen && got.listenSet == want.listenSet && got.waitSeconds == want.waitSeconds && got.seats != nil
	}
	for _, options := range received {
		if !sameOptions(options, uiIntentOptions{waitSeconds: 15}) {
			t.Fatalf("default interface options = %+v", options)
		}
	}
	for _, test := range []struct {
		args []string
		want uiIntentOptions
	}{
		{[]string{"ui", "start", "--listen", "127.0.0.1:9876"}, uiIntentOptions{listen: "127.0.0.1:9876", listenSet: true, waitSeconds: 15}},
		{[]string{"ui", "stop", "--wait-seconds", "0"}, uiIntentOptions{waitSeconds: 0}},
		{[]string{"ui", "restart", "--listen", "[::1]:9876", "--wait-seconds", "40"}, uiIntentOptions{listen: "[::1]:9876", listenSet: true, waitSeconds: 40}},
	} {
		if code, result := b.runJSON(owners, test.args...); code != 0 || result.Outcome != intentConfirmed || !sameOptions(received[len(received)-1], test.want) {
			t.Fatalf("%v = %d %+v options %+v", test.args, code, result, received)
		}
	}
	for _, args := range [][]string{
		{"session", "start", "--listen", "127.0.0.1:9876"},
		{"machine", "start", "m9", "--listen", "127.0.0.1:9876"},
		{"work", "stop", "j2:some-job", "--wait-seconds", "0"},
		{"system", "restart", "--wait-seconds", "1"},
		{"ui", "stop", "--wait-seconds", "-1"},
		{"ui", "stop", "--wait-seconds", "0.5"},
		{"ui", "restart", "--wait-seconds", "9223372036854775807"},
	} {
		if code, result := b.runJSON(owners, args...); code != 2 || result.Outcome != intentRefused || len(verbs) != 6 || len(engine) != 0 {
			t.Fatalf("bad UI options %v reached an owner: %d %+v %v %v", args, code, result, verbs, engine)
		}
	}

	if code, result := b.runJSON(owners, "machine", "start", "m9", "--destination", "/tmp/m9"); code != 0 || len(engine) != 1 ||
		!slices.Equal(engine[0][1:6], []string{"internal", "seat", "launch", "--machine", "m9"}) || !strings.Contains(strings.Join(engine[0], " "), "--destination /tmp/m9") {
		t.Fatalf("start machine = %d %+v %v", code, result, engine)
	}
	for _, args := range [][]string{{"ui", "start", "--temporary-human-word", "yes"}, {"system", "start", "--destination", "/tmp/x"}, {"machine", "list", "ui"}, {"status", "--refresh"}} {
		if code, result := b.runJSON(owners, args...); code != 2 || result.Outcome != intentRefused {
			t.Fatalf("%v = %d %+v", args, code, result)
		}
	}
	if len(verbs) != 6 || len(engine) != 1 {
		t.Fatalf("a refused target reached an owner: %v %v", verbs, engine)
	}
}

// TestIntentCheckPublicRemedies: every unhealthy role check reports carries
// a public command or a plain instruction chosen by the role, never the
// owner's internal command line.
func TestIntentCheckPublicRemedies(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		role    steward.HealthRole
		stopped bool
		want    []string
	}{
		{steward.RoleStewardRunner, false, []string{"metasystem", "session", "start"}},
		{steward.RoleSupervisionOwner, true, []string{"metasystem", "system", "start"}},
		{steward.RoleTrunkRed, false, []string{"metasystem", "incident", "list"}},
		{steward.RoleProofAttempts, false, []string{"metasystem", "test", "run"}},
	} {
		public, _ := publicHealthRemedy(steward.RoleVerdict{Role: row.role, Remedy: "metasystem session start --repo /x"}, row.stopped)
		if !slices.Equal(public, row.want) {
			t.Errorf("%s stopped=%t = %v, want %v", row.role, row.stopped, public, row.want)
		}
	}
	// Typed causes carried by the producer choose the public act.
	for _, row := range []struct {
		fact   steward.RemedyFact
		public []string
	}{
		{steward.RemedyFact{Cause: steward.CauseBudgetMissing, Goal: "g1"}, []string{"metasystem", "goal", "budget", "g1", "BOX"}},
		{steward.RemedyFact{Cause: steward.CauseBudgetBreach, Goal: "g1"}, []string{"metasystem", "goal", "budget", "g1", "BOX"}},
		{steward.RemedyFact{Cause: steward.CauseEpochMismatch, Goal: "g1"}, []string{"metasystem", "session", "start"}},
		{steward.RemedyFact{Cause: steward.CauseForeignLineage, Goal: "g1"}, []string{"metasystem", "goal", "release", "g1"}},
		{steward.RemedyFact{Cause: steward.CauseBudgetUnknown, Record: "artifacts/agents/jobs/x.json"}, nil},
		{steward.RemedyFact{Cause: steward.CauseBreachStopUnresolved, Goal: "g1", Stop: "s1"}, nil},
	} {
		public, instruction := publicHealthRemedy(steward.RoleVerdict{Role: steward.RoleClaimedGoalBudget, RemedyFacts: []steward.RemedyFact{row.fact}}, false)
		if !slices.Equal(public, row.public) || (public == nil && (instruction == "" || strings.Contains(instruction, "--json"))) {
			t.Errorf("%+v = %v %q", row.fact, public, instruction)
		}
	}
	for _, role := range []steward.HealthRole{steward.RoleCensusFreshness, steward.RoleHookFreshness, steward.RoleLedgerAttention} {
		if public, _ := publicHealthRemedy(steward.RoleVerdict{Role: role, Remedy: "x"}, false); len(public) == 0 || slices.Equal(public, []string{"metasystem", "system", "check"}) {
			t.Errorf("%s sends check back to itself: %v", role, public)
		}
	}
	for _, role := range []steward.HealthRole{steward.RoleNonterminalJobs, steward.RoleRetroDebt, steward.RoleSpendFence, "unlisted-role"} {
		public, instruction := publicHealthRemedy(steward.RoleVerdict{Role: role, Remedy: `"/x/bin/metasystem" internal delegate reap`}, false)
		if public != nil || instruction == "" || strings.Contains(instruction, "delegate") || strings.Contains(instruction, "internal") || strings.Contains(instruction, "--json") {
			t.Errorf("%s = %v %q", role, public, instruction)
		}
	}
}
