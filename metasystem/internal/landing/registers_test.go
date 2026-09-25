package landing

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestWorkspaceExclusionsAreStableCopies(t *testing.T) {
	t.Parallel()
	want := []string{
		"memory/receipts.log",
		"plans/goals",
		"plans/goals-accepted.json",
		"plans/goals.md",
		"records/counselor",
		"records/goals",
		"records/narrator-digest.log",
	}
	got := WorkspaceExclusions()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WorkspaceExclusions() = %v, want %v", got, want)
	}
	got[0] = "mutated"
	if reflect.DeepEqual(WorkspaceExclusions(), got) {
		t.Fatal("WorkspaceExclusions returned shared mutable state")
	}
}

func TestAppendOnlyRegistersPins(t *testing.T) {
	receiptsRoot, err := stateroot.RelativeRoot(stateroot.Receipts)
	if err != nil {
		t.Fatal(err)
	}
	recordsRoot, err := stateroot.RelativeRoot(stateroot.Records)
	if err != nil {
		t.Fatal(err)
	}
	// Pin 1: the declared registers are exactly the design's two, each under
	// its state root; the counselor registers stay outside (they keep the
	// held-goal rule at the carriage gate and are excluded from the delivery
	// workspace through ledgerPaths, see TestWorkspaceProjection).
	want := []string{
		filepath.ToSlash(filepath.Join(receiptsRoot, "receipts.log")),
		filepath.ToSlash(filepath.Join(recordsRoot, "narrator-digest.log")),
	}
	if !reflect.DeepEqual(appendOnlyRegisters, want) {
		t.Fatalf("append-only registers = %v, want %v", appendOnlyRegisters, want)
	}

	// Pin 2: every declared register is outside the LANDING projection, so
	// commit.sh's comparison and the receipt's filter agree on it.
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, register := range appendOnlyRegisters {
		for _, test := range []struct {
			path   string
			prefix string
		}{
			{path: "metasystem/" + register, prefix: "metasystem/"},
			{path: register},
		} {
			included, err := policy.Includes(behaviorsurface.Landing, test.path, test.prefix)
			if err != nil {
				t.Fatal(err)
			}
			if included {
				t.Fatalf("append-only register %s is included in the landing projection for prefix %q", register, test.prefix)
			}
		}
	}
}
