package delegation_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// p6HoldChainLock makes a live holder own the chain lock: this very
// process under a tag its argv really carries, as a racing wrapper would.
func (b *bed) p6HoldChainLock(chain string) {
	b.t.Helper()
	holder := filepath.Join(b.root, "artifacts", "agents", "locks", chain+".d")
	if err := os.MkdirAll(holder, 0o755); err != nil {
		b.t.Fatal(err)
	}
	owner, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "instanceTag": strings.Join(os.Args, " ")})
	if err := os.WriteFile(filepath.Join(holder, "owner.json"), append(owner, '\n'), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// p6Armed installs the census fingerprint inputs and arms supervision, so
// a stub-Git bed passes a follow-up's fresh-census entry check.
func (b *bed) p6Armed() {
	b.t.Helper()
	b.writeFile("bin/metasystem", "engine bytes\n")
	b.armSupervision()
}

// Lines 4202-4206: a follow-up of a chain whose newest round is a running
// fresh dispatch is refused by name, and no successor is reserved.
func TestFollowUpRefusesARunningFreshRound(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.p6Armed()
	b.writeRecord("active-turn", map[string]any{"status": "running", "role": "design-critic", "round": 1, "dispatchMode": "fresh", "sessionId": "s-1"})
	message := b.writeFile("follow.md", "Working Mode: design\n\nAgain.\n")
	result := b.run("follow-up", "--job", "active-turn", "--message", message)
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "use a fresh dispatch after pending, running, process-lost") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if exists(b.recordPath("active-turn-r2")) || len(b.calls("adapter.Launch")) != 0 {
		t.Fatal("a refused follow-up reserved or launched a successor")
	}
	if exists(filepath.Join(b.root, "artifacts", "agents", "locks", "active-turn.d", "owner.json")) {
		t.Fatal("the refused follow-up kept the chain lock")
	}
}

// Lines 4208-4213: a closed chain refuses a follow-up under its chain lock.
func TestFollowUpRefusesAClosedChain(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.p6Armed()
	b.writeRecord("happy", map[string]any{"status": "completed", "role": "design-critic", "round": 1, "chainClosed": true, "sessionId": "s-1"})
	message := b.writeFile("follow.md", "Working Mode: design\n\nAgain.\n")
	result := b.run("follow-up", "--job", "happy", "--message", message)
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "job chain is closed") || exists(b.recordPath("happy-r2")) {
		t.Fatalf("stderr %q", b.stderr.String())
	}
}

// Lines 4215-4247: close and follow-up serialize on the one chain lock. A
// close that finds a racing follow-up holding it refuses without closing;
// the follow-up side's LOCK_BUSY is TestLaunchChainLockNamesItsHolderWhenBusy,
// and a close that won first is TestFollowUpRefusesAClosedChain.
func TestCloseRefusesWhileAFollowUpHoldsTheChainLock(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("close-race", map[string]any{"status": "completed", "role": "implementer", "round": 1})
	b.p6HoldChainLock("close-race")
	result := b.run("close", "--job", "close-race")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "chain is busy: close-race") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if record := b.record("close-race"); record["chainClosed"] == true {
		t.Fatal("a close that lost the chain lock closed the chain")
	}
}
