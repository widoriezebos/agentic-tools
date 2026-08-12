package mission

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Contract preflight's process-identity proof (go-production-grade Phase 2½
// Unit A, obligations A-3 and A-4). Written against the pre-move `ps`
// implementation and unchanged across the move to the identity owner.

func selfStartAndTag(t *testing.T) (int64, string) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot probe self: %v %v", state, err)
	}
	if !exact.ArgvKnown || len(exact.Argv) == 0 {
		t.Skip("this process's argv is unreadable; the tag proof needs it")
	}
	// A distinctive substring of our own argv serves as the tag.
	parts := strings.Split(exact.Argv[0], "/")
	return exact.StartedAt.Unix(), parts[len(parts)-1]
}

func TestContractProcessHasTagAcceptsOurselves(t *testing.T) {
	start, tag := selfStartAndTag(t)
	if !contractProcessHasTag(t.TempDir(), int64(os.Getpid()), start, tag) {
		t.Fatalf("our own process failed its identity proof (tag %q)", tag)
	}
}

func TestContractProcessHasTagRejectsAStranger(t *testing.T) {
	start, _ := selfStartAndTag(t)
	if contractProcessHasTag(t.TempDir(), int64(os.Getpid()), start, "tag-that-cannot-appear-91f3c7") {
		t.Fatal("a tag absent from our command line passed the identity proof")
	}
}

// The guards that precede any process read: a nonsense pid, a start time of
// zero, and an empty tag are all refusals before the table is consulted.
func TestContractProcessHasTagGuards(t *testing.T) {
	start, tag := selfStartAndTag(t)
	root := t.TempDir()
	if contractProcessHasTag(root, 1, start, tag) {
		t.Fatal("pid 1 must not pass (pid <= 1 is refused outright)")
	}
	if contractProcessHasTag(root, int64(os.Getpid()), 0, tag) {
		t.Fatal("a zero start time must be refused")
	}
	if contractProcessHasTag(root, int64(os.Getpid()), start, "") {
		t.Fatal("an empty tag must be refused")
	}
	if contractProcessHasTag(root, 1<<30, start, tag) {
		t.Fatal("a pid that cannot exist must be refused")
	}
}

// Without its own fixture variable set, an unreadable process table refuses
// rather than silently passing — and the dispatch side's variable must not
// satisfy this side (A-4).
func TestContractProcessHasTagIgnoresTheDispatchFixtureVariable(t *testing.T) {
	start, tag := selfStartAndTag(t)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "/nonexistent/dispatch-fixture.json")
	if !contractProcessHasTag(t.TempDir(), int64(os.Getpid()), start, tag) {
		t.Fatal("the dispatch fixture variable must not disturb a genuine live proof")
	}
	if contractProcessHasTag(t.TempDir(), 1<<30, start, tag) {
		t.Fatal("the dispatch fixture variable must not satisfy the mission-side proof")
	}
}
