package main

import (
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// weightThreshold is the installation's validation weight threshold.
func weightThreshold(root string) int64 { return gaterun.WeightThreshold(root) }

// gateWeightAddTo folds one landing's numstat into the validation weight and
// prints the accumulator on the caller's streams.
func gateWeightAddTo(stdout, stderr io.Writer, root, commit, prefix, goalID string, numstat []byte) int {
	scale := goalWeightScale(root, goalID)
	state, due, err := gaterun.WeightAddScaled(root, commit, numstat, prefix, weightThreshold(root), scale)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "validation weight %d over %d landing(s) since %s (this landing scaled by goal risk %d)\n", state.Accumulated, state.Landings, state.SinceUTC, scale)
	if due {
		fmt.Fprintf(stdout, "validation weight reached (threshold %d): run the governed direct validator; findings fix forward\n", weightThreshold(root))
	}
	return 0
}

// goalWeightScale is the owning goal's highest risk answer, 1 when no goal
// is named or its risk cannot be read: weight bookkeeping never refuses a
// concluded landing, it only scales it.
func goalWeightScale(root, goalID string) int64 {
	if goalID == "" {
		return 1
	}
	risk, _, err := testrun.GoalRisk(root, goalID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation weight: goal %s risk not read (%v); the landing weighs unscaled\n", goalID, err)
		return 1
	}
	scale := 1
	for _, answer := range []int{risk.Severity, risk.Novelty, risk.Exposure, risk.Accumulation} {
		if answer > scale {
			scale = answer
		}
	}
	if scale > 3 {
		scale = 3
	}
	return int64(scale)
}
