package gaterun

// The battery is not a per-change gate: it is an expensive correctness
// proof that becomes sensible only once enough FEATURE WEIGHT has
// accumulated since it last ran. Every landing adds measured weight —
// behavior-surface breadth, engine depth, diff size — and when the
// accumulated total crosses the declared threshold, the landing
// boundary says so. The say-so is a nudge toward the milestone
// battery, never a blocker: what that battery finds is fixed forward.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WeightState is the accumulator, machine-local under artifacts/.
type WeightState struct {
	Accumulated int64  `json:"accumulated"`
	Landings    int64  `json:"landings"`
	SinceUTC    string `json:"sinceUtc"`
	LastCommit  string `json:"lastCommit"`
}

// LandingWeight measures one landing from its numstat rows. Engine
// source weighs triple: a Go change reaches every consumer of the
// binary, while a script or doc reaches its own callers. Coordination
// state (the goal ledger, receipts, artifacts) weighs nothing — no
// proof consumes it, so it can never summon the battery.
func LandingWeight(numstat string) int64 {
	var weight, lines int64
	for _, row := range strings.Split(numstat, "\n") {
		fields := strings.Fields(row)
		if len(fields) < 3 {
			continue
		}
		path := fields[len(fields)-1]
		switch {
		case strings.HasPrefix(path, "plans/goals/"),
			path == "plans/receipts.log",
			strings.HasPrefix(path, "artifacts/"):
			continue
		}
		perFile := int64(1)
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			perFile = 3
		}
		weight += perFile
		var add, del int64
		fmt.Sscanf(fields[0], "%d", &add)
		fmt.Sscanf(fields[1], "%d", &del)
		lines += add + del
	}
	// Diff size folds in gently: one point per hundred changed lines,
	// so a mechanical sweep cannot dwarf a small deep change.
	return weight + lines/100
}

func weightPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "battery-weight.json")
}

func loadWeight(root string) WeightState {
	var state WeightState
	data, err := os.ReadFile(weightPath(root))
	if err == nil {
		json.Unmarshal(data, &state)
	}
	if state.SinceUTC == "" {
		state.SinceUTC = time.Now().UTC().Format(time.RFC3339)
	}
	return state
}

func saveWeight(root string, state WeightState) error {
	path := weightPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// WeightAdd folds one landing into the accumulator and reports the
// running total against the threshold.
func WeightAdd(root, commit, numstat string, threshold int64) (WeightState, bool, error) {
	state := loadWeight(root)
	state.Accumulated += LandingWeight(numstat)
	state.Landings++
	state.LastCommit = commit
	if err := saveWeight(root, state); err != nil {
		return state, false, err
	}
	return state, threshold > 0 && state.Accumulated >= threshold, nil
}

// WeightCheck reports without mutating: due when the accumulator has
// crossed the threshold.
func WeightCheck(root string, threshold int64) (WeightState, bool) {
	state := loadWeight(root)
	return state, threshold > 0 && state.Accumulated >= threshold
}

// WeightReset marks a milestone battery run: the accumulator returns
// to zero and the window restarts. Only a green battery earns this.
func WeightReset(root, commit string) (WeightState, error) {
	state := WeightState{
		SinceUTC:   time.Now().UTC().Format(time.RFC3339),
		LastCommit: commit,
	}
	return state, saveWeight(root, state)
}
