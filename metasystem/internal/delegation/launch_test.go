package delegation_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// --- locks ------------------------------------------------------------

// A live holder of the chain lock refuses a new launch after the scaled cap
// with a LOCK_BUSY outcome naming the holder.
func TestLaunchChainLockNamesItsHolderWhenBusy(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("parent", map[string]any{"status": "completed", "sessionId": "s-1", "round": 1})
	message := b.writeFile("message.md", "follow up\n")
	holder := filepath.Join(b.root, "artifacts", "agents", "locks", "parent.d")
	if err := os.MkdirAll(holder, 0o755); err != nil {
		t.Fatal(err)
	}
	// The holder is this very process under another tag it really carries
	// in its argv, so the owner-lock probe keeps it.
	selfArgv := strings.Join(os.Args, " ")
	owner, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "instanceTag": selfArgv})
	if err := os.WriteFile(filepath.Join(holder, "owner.json"), append(owner, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	b.writeFile("artifacts/agents/supervision/last-census.json", `{}`)
	result := b.run("follow-up", "--job", "parent", "--message", message)
	if result.ExitCode == 0 {
		t.Fatal("a busy chain admitted a follow-up")
	}
	if !strings.Contains(b.stderr.String(), "LOCK_BUSY rank=chain key=parent") && !strings.Contains(b.stderr.String(), "dispatch refused") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
}

// actionable-metrics O13: a malformed goal id refuses before anything else
// of the dispatch runs.
func TestDispatchRefusesAMalformedGoalID(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	brief := b.writeFile("brief.md", "review this\n")
	result := b.run("dispatch", "--role", "implementer", "--brief", brief, "--goal", "Invalid_goal", "--destructive-reach", "MECHANICAL")
	requireExit(t, result, 2, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "invalid goal id: Invalid_goal") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if len(b.calls("guard.")) != 0 || len(b.calls("lease.")) != 0 {
		t.Fatalf("a malformed goal reached the guard or the lease: %v", b.doubles.Log.Calls())
	}
}

// Every dispatch refusal that precedes its selection's validation names
// the problem before any owner is reached.
func TestDispatchSelectionRefusals(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	brief := b.writeFile("brief.md", "Working Mode: implement\n")
	for _, tc := range []struct {
		argv []string
		code int
		want string
	}{
		{[]string{"dispatch", "--steward-intent", "n1", "--role", "implementer"}, 2, "--steward-intent admits no other selection flags"},
		{[]string{"dispatch", "--brief", brief, "--destructive-reach", "MECHANICAL"}, 2, "Usage:"},
		{[]string{"dispatch", "--role", "implementer", "--brief", brief}, 2, "Usage:"},
		{[]string{"dispatch", "--role", "implementer", "--brief", brief, "--destructive-reach", "SOMETIMES"}, 2, "Usage:"},
		{[]string{"dispatch", "--role", "nonesuch", "--brief", brief, "--destructive-reach", "MECHANICAL"}, 1, "unknown dispatch role: nonesuch"},
		{[]string{"dispatch", "--role", "implementer", "--brief", brief, "--destructive-reach", "MECHANICAL", "--bogus"}, 2, "Usage:"},
	} {
		result := b.run(tc.argv...)
		if result.ExitCode != tc.code || !strings.Contains(b.stderr.String(), tc.want) {
			t.Fatalf("%v: exit %d stderr %q; want %d with %q", tc.argv, result.ExitCode, b.stderr.String(), tc.code, tc.want)
		}
	}
}

// A brief whose headers do not parse refuses with the operator's repair
// sentence before the lease is read, and releases the checkout guard.
func TestDispatchRefusesInvalidBriefHeadersWithTheRepairSentence(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeFile("metasystem.conf", "metasystem.runtimes=fake\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\n")
	partial := b.writeFile("partial.md", "Working Mode:\n")
	result := b.run("dispatch", "--role", "implementer", "--brief", partial, "--destructive-reach", "MECHANICAL")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "brief headers are invalid; Working Mode must be filled and Boundary and Ceiling must appear together") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if len(b.calls("lease.")) != 0 || len(b.calls("guard.Acquire")) != len(b.calls("guard.Release")) {
		t.Fatalf("an invalid brief reached the lease or kept the guard: %v", b.doubles.Log.Calls())
	}
}

// The whole fresh dispatch: admission, reservation, record, launch under
// the ranked locks, the ownership proof, the start gate, and the handshake
// wait; stdout names the job and stderr the exact waiter command.
func TestDispatchIntegrationLaunchesAnImplementerRound(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--job-id", "happy")
	requireExit(t, result, 0, b.stderr.String())
	if string(result.Stdout) != "happy\n" || !strings.Contains(b.stderr.String(), "wait for it with: metasystem work wait j2:happy") {
		t.Fatalf("stdout %q stderr %q", result.Stdout, b.stderr.String())
	}
	record := b.record("happy")
	if record["status"] != "running" || record["sessionId"] != "fake-session-happy" || record["pid"] == nil {
		t.Fatalf("record %v", record)
	}
}

// A stopped checkout answers a dispatch or a follow-up with its typed
// REFUSED-STOPPED verdict before census admission reads supervision state
// the stop made stale (the retired script's fence-before-census order).
func TestAStoppedCheckoutRefusesBeforeCensusAdmission(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	if err := stopfence.Write(b.root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 7,
		ChangedAt: "2026-09-27T10:00:00Z", Checkout: b.root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71}},
	}); err != nil {
		t.Fatal(err)
	}
	brief := b.writeFile("brief.md", "Working Mode: implement\n")
	b.writeRecord("parent", map[string]any{"status": "completed", "round": 1, "sessionId": "s"})
	for _, argv := range [][]string{
		{"dispatch", "--role", "implementer", "--brief", brief, "--destructive-reach", "MECHANICAL"},
		{"follow-up", "--job", "parent", "--message", brief},
	} {
		result := b.run(argv...)
		requireExit(t, result, 1, b.stderr.String())
		if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-STOPPED" || !strings.Contains(string(result.Stdout), "REFUSED-STOPPED") {
			t.Fatalf("%s: outcome %v stdout %q", argv[0], outcome, result.Stdout)
		}
		if strings.Contains(b.stderr.String(), "census") {
			t.Fatalf("%s consulted the census before the fence: %q", argv[0], b.stderr.String())
		}
	}
}
