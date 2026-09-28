package main

import (
	"reflect"
	"slices"
	"testing"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// Every remedy the object-action grammar moved from a family form to a
// public form must reach the owner the family form reached. A passthrough
// action must hand its words to the very handler the family verb ran; an
// adapted action is named here with the owner test that drives it.
func TestIntentSwitchedRemediesReachTheFamilyOwner(t *testing.T) {
	t.Parallel()
	scope := processScope{Checkout: "/checkout"}
	for _, test := range []struct {
		printed, family string
		object, action  string
		// handler is the family handler a passthrough action must run.
		handler func([]string) int
		// rest is the argument vector the handler receives.
		rest []string
	}{
		{printed: "metasystem session status --id 7f3a", family: "metasystem report stop-status --id 7f3a",
			object: "session", action: "status", handler: runReportStopStatus, rest: []string{"--id", "7f3a"}},
		// session handoff runs runContextHandoff, or runContextStatus for
		// --status and runContextVerify for --verify NONCE; runSessionHandoff
		// is that split (TestIntentSessionHandoffSplit).
		{printed: "metasystem session handoff --root /i --note memory/handoff.md --no-delegates", family: "metasystem context handoff --root /i --note memory/handoff.md --no-delegates",
			object: "session", action: "handoff", handler: runSessionHandoff, rest: []string{"--root", "/i", "--note", "memory/handoff.md", "--no-delegates"}},
		{printed: "metasystem session handoff --status --root /i", family: "metasystem context status --root /i",
			object: "session", action: "handoff", handler: runSessionHandoff, rest: []string{"--status", "--root", "/i"}},
		{printed: "metasystem session handoff --root /i --verify 3f2a", family: "metasystem context verify --root /i --nonce 3f2a",
			object: "session", action: "handoff", handler: runSessionHandoff, rest: []string{"--root", "/i", "--verify", "3f2a"}},
		// work wait --exit-code runs runJobWatchVerb for j2:J and runRunWatch
		// for --run ID (TestIntentWaitExitCodeRoutes).
		{printed: "metasystem work wait j2:impl-01 --exit-code --repo /c --caller-pid 7", family: "metasystem job watch --root /c --job impl-01 --caller-pid 7", object: "work", action: "wait"},
		{printed: "metasystem work wait --run r1 --exit-code --repo /c", family: "metasystem run watch --id r1 --root /c", object: "work", action: "wait"},
		// Adapted actions: the owner tests named drive each to its owner.
		// work stop j2:J cancels through runDelegateIn --cancel J
		// (TestIntentProcessAndAnswerTargets, "work stop j2:job-c").
		{printed: "metasystem work stop j2:job-c", family: "metasystem delegate --cancel job-c", object: "work", action: "stop"},
		// work finish j2:J is done job J's owner (TestIntentConnectedJourneyRealClose).
		{printed: "metasystem work finish j2:crit1", family: "metasystem done job crit1", object: "work", action: "finish"},
		// work wait wait:ID resumes through the waiter owner with --resume ID
		// (TestIntentWaitGoalEventGitAdapterObservesAPersonsAct).
		{printed: metarun.WaitResumeCommand(metarun.Waiter{WaitID: "w-1"}), family: "metasystem wait --resume w-1", object: "work", action: "wait"},
		// system stop/start call processOwners.stop/arm, the owners the
		// family stop/arm ran (runProcessStopWith, runProcessArmWith).
		{printed: processVerbRetryCommand(scope, "stop"), family: "metasystem stop --repo /checkout", object: "system", action: "stop"},
		{printed: processVerbRetryCommand(scope, "arm"), family: "metasystem arm --repo /checkout", object: "system", action: "start"},
		// session start runs up.Run for the checkout (TestIntentProcessAndAnswerTargets,
		// "session start"); system check reads the steward health verdict.
		{printed: "metasystem session start --repo /checkout", family: "metasystem up --repo /checkout", object: "session", action: "start"},
		{printed: "metasystem system check --repo /checkout", family: "metasystem health --repo /checkout", object: "system", action: "check"},
	} {
		words := shellWords(test.printed)
		command, rest, ok := resolveIntentArgv(words[1:])
		if !ok || command.object != test.object || command.action != test.action {
			t.Errorf("%s (was %s) resolves to %s %s, want %s %s", test.printed, test.family, command.object, command.action, test.object, test.action)
			continue
		}
		if test.handler == nil {
			if command.passthrough != nil {
				t.Errorf("%s is a passthrough; name the family handler it runs", test.printed)
			}
			continue
		}
		if command.passthrough == nil || command.owner == nil || reflect.ValueOf(command.owner).Pointer() != reflect.ValueOf(test.handler).Pointer() {
			t.Errorf("%s does not run the family handler of %s", test.printed, test.family)
		}
		if !slices.Equal(rest, test.rest) {
			t.Errorf("%s hands %q to the handler, want %q", test.printed, rest, test.rest)
		}
	}
}
