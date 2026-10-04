package gaterun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type WeightDecision struct {
	RunID              string                   `json:"runId"`
	GoalID             string                   `json:"goalId"`
	GoalRevision       uint64                   `json:"goalRevision"`
	ObligationRevision uint64                   `json:"obligationRevision"`
	WeightGeneration   uint64                   `json:"weightGeneration"`
	DecidedAt          string                   `json:"decidedAt"`
	ResetDecision      goal.ConsequenceDecision `json:"resetDecision"`
	DischargeDecision  goal.ConsequenceDecision `json:"dischargeDecision"`
	Applied            bool                     `json:"applied"`
	PriorWeight        int64                    `json:"priorWeight"`
	PriorLandings      int64                    `json:"priorLandings"`
}

type ConsumedProof struct {
	RunID              string                   `json:"runId"`
	GoalID             string                   `json:"goalId"`
	GoalRevision       uint64                   `json:"goalRevision"`
	ObligationRevision uint64                   `json:"obligationRevision"`
	WeightGeneration   uint64                   `json:"weightGeneration"`
	ConsumedAt         string                   `json:"consumedAt"`
	ResetDecision      goal.ConsequenceDecision `json:"resetDecision"`
	DischargeDecision  goal.ConsequenceDecision `json:"dischargeDecision"`
}

// WeightState is a plain landing accumulator. It has no runner, clone,
// checkpoint, or hidden retry lifecycle.
type WeightState struct {
	Schema         int             `json:"schema"`
	Generation     uint64          `json:"generation"`
	Accumulated    int64           `json:"accumulated"`
	Landings       int64           `json:"landings"`
	SinceUTC       string          `json:"sinceUtc"`
	LastCommit     string          `json:"lastCommit"`
	LastDecision   *WeightDecision `json:"lastDecision,omitempty"`
	ConsumedProofs []ConsumedProof `json:"consumedProofs,omitempty"`
}

type WeightDischargeResult struct {
	State    WeightState    `json:"state"`
	Decision WeightDecision `json:"decision"`
}

func recordInertWeightDecision(state WeightState, decision WeightDecision) (WeightState, bool) {
	if !decision.ResetDecision.WouldRefuse && !decision.DischargeDecision.WouldRefuse {
		return state, false
	}
	state.Generation++
	state.LastDecision = &decision
	return state, true
}

func weightPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "validation-weight.json")
}

func WeightLockPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "validation-weight.flock")
}

type weightLock struct{ held *lock.FileLock }

func acquireWeightLock(root string) (*weightLock, error) {
	if err := os.MkdirAll(filepath.Dir(WeightLockPath(root)), 0o755); err != nil {
		return nil, err
	}
	held, err := lock.File(WeightLockPath(root), 0o644, lock.Exclusive)
	if err != nil {
		return nil, err
	}
	return &weightLock{held: held}, nil
}

func (lock *weightLock) release() {
	_ = lock.held.Release()
}

var weightNow = func() time.Time { return time.Now().UTC() }

func initialWeight(now time.Time) WeightState {
	return WeightState{Schema: 1, SinceUTC: now.UTC().Format(time.RFC3339)}
}

func validateWeight(state WeightState) error {
	if state.Schema != 1 || state.Accumulated < 0 || state.Landings < 0 || state.SinceUTC == "" {
		return fmt.Errorf("validation weight state is incomplete")
	}
	if _, err := time.Parse(time.RFC3339, state.SinceUTC); err != nil {
		return fmt.Errorf("validation weight sinceUtc is invalid: %w", err)
	}
	if state.Accumulated > 0 && state.Landings == 0 {
		return fmt.Errorf("validation weight has no landing")
	}
	if decision := state.LastDecision; decision != nil {
		if decision.RunID == "" || decision.GoalID == "" || decision.GoalRevision == 0 || decision.ObligationRevision == 0 || decision.DecidedAt == "" ||
			decision.PriorWeight < 0 || decision.PriorLandings < 0 {
			return fmt.Errorf("validation weight decision is incomplete")
		}
		if _, err := time.Parse(time.RFC3339, decision.DecidedAt); err != nil {
			return fmt.Errorf("validation weight decision timestamp is invalid")
		}
		if decision.Applied {
			if decision.WeightGeneration >= state.Generation {
				return fmt.Errorf("validation weight record: its applied reset is not older than the current count")
			}
			if !decision.ResetDecision.Apply || decision.ResetDecision.WouldRefuse ||
				!decision.DischargeDecision.Apply || decision.DischargeDecision.WouldRefuse {
				return fmt.Errorf("applied validation weight decision lacks both consequences")
			}
		} else if !decision.ResetDecision.WouldRefuse && !decision.DischargeDecision.WouldRefuse {
			return fmt.Errorf("validation weight record: a reset that was held back names no reason")
		}
	}
	seenProofs := map[string]bool{}
	for _, proof := range state.ConsumedProofs {
		if proof.RunID == "" || proof.GoalID == "" || proof.GoalRevision == 0 || proof.ObligationRevision == 0 || proof.ConsumedAt == "" ||
			!proof.ResetDecision.Apply || proof.ResetDecision.WouldRefuse || !proof.DischargeDecision.Apply || proof.DischargeDecision.WouldRefuse {
			return fmt.Errorf("validation weight record: a used-up validation run is incomplete")
		}
		if _, err := time.Parse(time.RFC3339, proof.ConsumedAt); err != nil {
			return fmt.Errorf("validation weight record: a used-up validation run has an invalid time")
		}
		key := fmt.Sprintf("%s\x00%s\x00%d\x00%d", proof.RunID, proof.GoalID, proof.ObligationRevision, proof.WeightGeneration)
		if seenProofs[key] {
			return fmt.Errorf("validation weight record: one validation run is used up twice")
		}
		seenProofs[key] = true
	}
	if decision := state.LastDecision; decision != nil && decision.Applied {
		matched := false
		for _, proof := range state.ConsumedProofs {
			if proof.RunID == decision.RunID && proof.GoalID == decision.GoalID && proof.GoalRevision == decision.GoalRevision &&
				proof.ObligationRevision == decision.ObligationRevision && proof.WeightGeneration == decision.WeightGeneration &&
				proof.ConsumedAt == decision.DecidedAt {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("validation weight record: its applied reset names no used-up validation run")
		}
	}
	return nil
}

func loadWeight(root string, now time.Time) (WeightState, error) {
	data, err := os.ReadFile(weightPath(root))
	if os.IsNotExist(err) {
		return initialWeight(now), nil
	}
	if err != nil {
		return WeightState{}, err
	}
	var state WeightState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return WeightState{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return WeightState{}, fmt.Errorf("validation weight has trailing JSON")
	}
	return state, validateWeight(state)
}

func writeWeight(root string, state WeightState) error {
	if err := validateWeight(state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(weightPath(root), string(data)+"\n", root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("validation weight published with directory durability unknown")
	}
	return nil
}

func LandingWeight(numstat []byte, prefix string) (int64, error) {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return 0, err
	}
	separator := byte(0)
	if !bytes.ContainsRune(numstat, 0) {
		separator = '\n'
	}
	var weight, lines int64
	for len(numstat) > 0 {
		index := bytes.IndexByte(numstat, separator)
		var row []byte
		if index < 0 {
			row, numstat = numstat, nil
		} else {
			row, numstat = numstat[:index], numstat[index+1:]
		}
		first := bytes.IndexByte(row, '\t')
		if first < 0 {
			continue
		}
		secondRelative := bytes.IndexByte(row[first+1:], '\t')
		if secondRelative < 0 {
			continue
		}
		second := first + 1 + secondRelative
		path := string(row[second+1:])
		included, err := policy.Includes(behaviorsurface.Landing, path, prefix)
		if err != nil || !included {
			if err != nil {
				return 0, err
			}
			continue
		}
		normalized, err := behaviorsurface.NormalizePath(path, prefix)
		if err != nil {
			return 0, err
		}
		if normalized == "" {
			normalized, err = behaviorsurface.NormalizePath(path, "")
			if err != nil {
				return 0, err
			}
		}
		perFile := int64(1)
		if strings.HasSuffix(normalized, ".go") && !strings.HasSuffix(normalized, "_test.go") {
			perFile = 3
		}
		weight += perFile
		for _, field := range [][]byte{row[:first], row[first+1 : second]} {
			if string(field) == "-" {
				continue
			}
			value, err := strconv.ParseInt(string(field), 10, 64)
			if err != nil || value < 0 {
				return 0, fmt.Errorf("invalid numstat count %q for %q", field, path)
			}
			lines += value
		}
	}
	return weight + lines/100, nil
}

func WeightAdd(root, commit string, numstat []byte, prefix string, threshold int64) (WeightState, bool, error) {
	return WeightAddScaled(root, commit, numstat, prefix, threshold, 1)
}

// WeightAddScaled folds one landing's measured weight, multiplied by the
// owning goal's risk scale, into the accumulator. The scale is the goal's
// highest risk answer (1 to 3): the goal's answers never choose per-landing
// depth, they bring the deep cadence run sooner (ruling R-3). A scale below
// 1 counts as 1.
func WeightAddScaled(root, commit string, numstat []byte, prefix string, threshold, scale int64) (WeightState, bool, error) {
	weight, err := LandingWeight(numstat, prefix)
	if err != nil {
		return WeightState{}, false, err
	}
	if scale < 1 {
		scale = 1
	}
	weight *= scale
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightState{}, false, err
	}
	defer lock.release()
	state, err := loadWeight(root, weightNow())
	if err != nil {
		return WeightState{}, false, err
	}
	state.Generation++
	state.Accumulated += weight
	state.Landings++
	state.LastCommit = commit
	if err := writeWeight(root, state); err != nil {
		return WeightState{}, false, err
	}
	return state, threshold > 0 && state.Accumulated >= threshold, nil
}

// WeightThresholdKey is the setting that names the validation weight
// threshold.
const WeightThresholdKey = "validation.weight-threshold"

// WeightThreshold is the installation at root's validation weight threshold:
// its metasystem.conf layers, else the compiled default; an unusable value
// reads as 60.
func WeightThreshold(root string) int64 {
	value, code, _ := config.Get(config.GetParams{Key: WeightThresholdKey, ConfPath: filepath.Join(root, "metasystem.conf")})
	if code != 0 {
		return int64(config.MustIntDefault(WeightThresholdKey))
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 60
	}
	return parsed
}

// WeightCheckAt reads cadence weight using its caller's clock.
func WeightCheckAt(root string, threshold int64, now time.Time) (WeightState, bool, error) {
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightState{}, false, err
	}
	defer lock.release()
	state, err := loadWeight(root, now)
	return state, err == nil && threshold > 0 && state.Accumulated >= threshold, err
}

type weightDischargeReads struct {
	ResolveGoalBinding func(string, string, time.Time) (dispatch.GoalBinding, error)
}

func weightDischargeAtWith(root, goalID string, obligationRevision uint64, runID string, now time.Time, reads weightDischargeReads) (WeightDischargeResult, error) {
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightDischargeResult{}, err
	}
	defer lock.release()
	state, err := loadWeight(root, now)
	if err != nil {
		return WeightDischargeResult{}, err
	}
	for _, proof := range state.ConsumedProofs {
		if proof.RunID == runID && proof.GoalID == goalID && proof.ObligationRevision == obligationRevision {
			return WeightDischargeResult{}, weightRefusal(WeightAlreadyReset, fmt.Errorf("run %s already reset the validation weight (at count %d); a run resets it once", runID, proof.WeightGeneration))
		}
	}
	binding, err := reads.ResolveGoalBinding(root, goalID, now)
	if err != nil {
		return WeightDischargeResult{}, fmt.Errorf("weight discharge requires the exact accepted obligation revision: %w", err)
	}
	if binding.File.Obligation == nil || binding.File.Obligation.Revision != obligationRevision {
		return WeightDischargeResult{}, weightRefusal(WeightResetNotAllowed, fmt.Errorf("weight discharge requires accepted obligation revision %d", obligationRevision))
	}
	obligation := binding.File.Obligation
	resetDecision := obligation.Decide(goal.EffectResetWeight)
	dischargeDecision := obligation.Decide(goal.EffectDischargeObligation)
	result := WeightDischargeResult{State: state, Decision: WeightDecision{RunID: runID, GoalID: goalID,
		GoalRevision: binding.Revision, ObligationRevision: obligationRevision, WeightGeneration: state.Generation,
		DecidedAt: now.Format(time.RFC3339), ResetDecision: resetDecision,
		DischargeDecision: dischargeDecision,
		PriorWeight:       state.Accumulated, PriorLandings: state.Landings}}
	if recorded, ok := recordInertWeightDecision(state, result.Decision); ok {
		state = recorded
		result.State = state
		return result, writeWeight(root, state)
	}
	// The policy slot is the installation's configuration: a root that holds
	// no metasystem.conf is refused, never read as an empty slot.
	installation, policyErr := stateroot.ParseInstallation(root)
	policy := ""
	if policyErr == nil {
		policy, policyErr = config.CorrelationPolicy(installation)
	}
	if policyErr != nil {
		// A read that failed, not the policy's answer: it may read later.
		return result, fmt.Errorf("the validation weight was not reset: the review policy can't be read: %w", policyErr)
	}
	if policy == "" || obligation.ReviewPolicy != policy ||
		!resetDecision.Apply || !dischargeDecision.Apply {
		return result, weightRefusal(WeightResetNotAllowed, fmt.Errorf("the validation weight was not reset: the goal's obligation and the review policy do not allow it"))
	}
	projection := dispatch.ProjectBudget(root, binding.File, now)
	if projection.Status != dispatch.BudgetKnown {
		return result, fmt.Errorf("the validation weight was not reset: the goal's test-run budget cannot be read (%s: %s)", projection.Unknown.Record, projection.Unknown.Reason)
	}
	record, err := (&run.Store{Root: root}).Read(runID)
	if err != nil || record == nil {
		return result, fmt.Errorf("the validation weight was not reset: run %s's record can't be read: %v", runID, err)
	}
	if record.Status != run.StatusGreen || record.GoalId != goalID || record.Governed == nil ||
		record.Governed.ObligationRevision != obligationRevision || record.Governed.WeightGeneration == nil ||
		record.Governed.Observation == nil || record.Governed.Observation.AssumptionState != run.AssumptionMatch || record.Governed.Exhausted {
		return result, weightRefusal(WeightRunNotGreen, fmt.Errorf("the validation weight was not reset: run %s is not a green validation run of this goal", runID))
	}
	if *record.Governed.WeightGeneration != state.Generation {
		return result, weightRefusal(WeightGenerationMoved, fmt.Errorf("the validation weight was not reset: run %s validated count %d, and the count is now %d", runID, *record.Governed.WeightGeneration, state.Generation))
	}
	if !sameWeightEpoch(record.Governed.BudgetEpoch, projection.WeightEpoch) {
		return result, weightRefusal(WeightResetNotAllowed, fmt.Errorf("the validation weight was not reset: run %s ran under an earlier budget of the obligation", runID))
	}
	// The compiled default names testing.json; an installation that names
	// no contract (an explicit empty value) is not migrated.
	if contract, present, lookupErr := config.CommittedLookup(filepath.Join(root, "metasystem.conf"), "testing.contract"); lookupErr != nil {
		return result, fmt.Errorf("the validation weight was not reset: metasystem.conf cannot say which testing contract applies: %w", lookupErr)
	} else if present && strings.TrimSpace(contract) != "" {
		// The attempt store that can't be read is a read that failed; what
		// it says about the run (no attempt, not a success, not sufficient)
		// is the run's answer.
		if _, readErr := proofrun.ReadAttempts(root); readErr != nil {
			return result, fmt.Errorf("the validation weight was not reset: run %s's test attempts can't be read: %w", runID, readErr)
		}
		attempt, testingResult, proofErr := proofrun.GovernedTestResult(root, runID)
		if proofErr != nil || attempt.GoalID != goalID || attempt.GoalRevision != binding.Revision || attempt.ReservationOwner == nil ||
			attempt.ReservationOwner.RunGeneration != record.Generation || attempt.ReservationOwner.ObligationRevision != obligationRevision ||
			testingResult.Purpose != testpolicy.PurposeCadence || testingResult.RequiredMode != testpolicy.ModeDeep || testingResult.ExecutedMode != testpolicy.ModeDeep {
			return result, weightRefusal(WeightRunNotGreen, fmt.Errorf("the validation weight was not reset: run %s did not run the full deep test set for this goal: %v", runID, proofErr))
		}
		if err := proofrun.RequireResultGroups(testingResult, testpolicy.CadenceCatchGroupIDs()); err != nil {
			return result, weightRefusal(WeightRunNotGreen, fmt.Errorf("the validation weight was not reset: %w", err))
		}
	}
	source := fmt.Sprintf("%s-r%d-weight-g%d-%s", goalID, obligationRevision, state.Generation, runID)
	if _, err := retrodebt.Raise(root, retrodebt.KindObligation, source, now); err != nil {
		return result, fmt.Errorf("the validation weight was not reset: the retro obligation could not be raised: %w", err)
	}
	result.Decision.Applied = true
	state.ConsumedProofs = append(state.ConsumedProofs, ConsumedProof{RunID: runID, GoalID: goalID,
		GoalRevision: binding.Revision, ObligationRevision: obligationRevision, WeightGeneration: state.Generation,
		ConsumedAt: now.Format(time.RFC3339), ResetDecision: resetDecision, DischargeDecision: dischargeDecision})
	state.Generation++
	state.Accumulated = 0
	state.Landings = 0
	state.SinceUTC = now.Format(time.RFC3339)
	state.LastDecision = &result.Decision
	result.State = state
	return result, writeWeight(root, state)
}

// The typed refusals of a weight discharge: the run or the policy says no,
// as opposed to a read that failed. A caller reads them with errors.As; any
// other error is a read that may succeed later.
const (
	WeightGenerationMoved = "WEIGHT_GENERATION_MOVED"
	WeightResetNotAllowed = "WEIGHT_RESET_NOT_ALLOWED"
	WeightRunNotGreen     = "WEIGHT_RUN_NOT_GREEN"
	WeightAlreadyReset    = "WEIGHT_ALREADY_RESET"
)

// WeightRefusal is a discharge the run or the policy refused, by code.
type WeightRefusal struct {
	Code string
	Err  error
}

func (refusal *WeightRefusal) Error() string { return refusal.Err.Error() }
func (refusal *WeightRefusal) Unwrap() error { return refusal.Err }

func weightRefusal(code string, err error) error {
	return &WeightRefusal{Code: code, Err: err}
}

func sameWeightEpoch(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
