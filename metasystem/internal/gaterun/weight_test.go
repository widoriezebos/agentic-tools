package gaterun

import (
	"strings"
	"testing"
)

// The weight formula's laws: engine source weighs triple, coordination
// state weighs nothing, and diff size folds in at one point per
// hundred lines.
func TestLandingWeight(t *testing.T) {
	cases := []struct {
		name    string
		numstat string
		want    int64
	}{
		{"one script", "3\t1\tscripts/agents/sync-transport.sh", 1},
		{"one engine file", "10\t2\tinternal/gaterun/weight.go", 3},
		{"engine test weighs like a script", "10\t2\tinternal/gaterun/weight_test.go", 1},
		{"goal ledger weighs nothing", "12\t0\tplans/goals/counselor.md", 0},
		{"receipts weigh nothing", "1\t0\tplans/receipts.log", 0},
		{"artifacts weigh nothing", "5\t5\tartifacts/agents/battery-weight.json", 0},
		{"nested goal ledger weighs nothing", "12\t0\tmetasystem/plans/goals/counselor.md", 0},
		{"nested receipts weigh nothing", "1\t0\tmetasystem/plans/receipts.log", 0},
		{"nested artifacts weigh nothing", "5\t5\tmetasystem/artifacts/agents/battery-weight.json", 0},
		{"a product plans file still weighs", "4\t0\tmetasystem/plans/counselor-design.md", 1},
		{"mixed landing", strings.Join([]string{
			"10\t2\tinternal/covenant/covenant.go",
			"40\t0\tinternal/covenant/covenant_test.go",
			"6\t1\tscripts/validate-metasystem.sh",
			"9\t0\tplans/goals/counselor.md",
		}, "\n"), 5},
		{"big sweep folds gently", "450\t250\tdocs/glossary.md", 8},
	}
	for _, c := range cases {
		if got := LandingWeight(c.numstat); got != c.want {
			t.Fatalf("%s: weight %d, want %d", c.name, got, c.want)
		}
	}
}

func TestWeightAccumulatorLifecycle(t *testing.T) {
	root := t.TempDir()

	state, due, err := WeightAdd(root, "abc1234", "10\t2\tinternal/a/a.go", 10)
	if err != nil {
		t.Fatal(err)
	}
	if state.Accumulated != 3 || state.Landings != 1 || due {
		t.Fatalf("first landing misfolded: %+v due=%v", state, due)
	}

	// Crossing the threshold reports due and STAYS due until reset —
	// the nudge repeats, it never blocks.
	state, due, err = WeightAdd(root, "def5678", strings.Repeat("5\t5\tinternal/b/b.go\n", 3), 10)
	if err != nil {
		t.Fatal(err)
	}
	if state.Accumulated != 12 || !due {
		t.Fatalf("threshold crossing missed: %+v due=%v", state, due)
	}
	if _, due = WeightCheck(root, 10); !due {
		t.Fatal("check must report due after crossing")
	}

	// A zero threshold disables the nudge without losing the count.
	if _, due = WeightCheck(root, 0); due {
		t.Fatal("a zero threshold must never report due")
	}

	// Only a milestone battery resets the window.
	state, err = WeightReset(root, "def5678")
	if err != nil {
		t.Fatal(err)
	}
	if state.Accumulated != 0 || state.LastCommit != "def5678" {
		t.Fatalf("reset misrecorded: %+v", state)
	}
	if _, due = WeightCheck(root, 10); due {
		t.Fatal("check must be quiet after reset")
	}
}
