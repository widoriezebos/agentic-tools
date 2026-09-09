package census

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestAlivePairSurvivesBtimeDrift:
// on a time-synced guest the btime-derived start SECOND of a live process
// differs between two reads, so seconds-equality false-deaths it. The
// clock-step-immune pair (StartTicks+BootID) does not move, so AlivePair with
// the pair must read the live process alive even when the expected SECOND is
// wrong — while a wrong pair (a genuinely different/reused process) still
// reads dead. Uses this test process's own real identity (Linux only; on
// darwin the pair is absent and the seconds path is exact, which the other
// tests cover).
func TestAlivePairSurvivesBtimeDrift(t *testing.T) {
	pid := int64(os.Getpid())
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probing self failed: state=%v err=%v", state, err)
	}
	if exact.StartTicks == 0 || exact.BootID == "" {
		t.Fatal("Linux process identity has no clock-step-immune pair")
	}
	sec := exact.StartedAt.Unix()

	// The exact truth is alive by every path.
	if !AlivePair(pid, sec, exact.StartTicks, exact.BootID, nil) {
		t.Fatal("exact identity must read alive")
	}
	// A btime step moved the recorded second by a few seconds; the pair is
	// unchanged. Seconds-only reads dead (the bug); the pair reads alive.
	if AlivePair(pid, sec+3, 0, "", nil) {
		t.Fatal("a wrong second with no pair must read dead (the seconds path)")
	}
	if !AlivePair(pid, sec+3, exact.StartTicks, exact.BootID, nil) {
		t.Fatal("a wrong second WITH the matching pair must read alive (drift-immune)")
	}
	// A genuinely different process (wrong ticks) is dead even at the right second.
	if AlivePair(pid, sec, exact.StartTicks+1, exact.BootID, nil) {
		t.Fatal("a mismatched pair (reused pid) must read dead")
	}
}
