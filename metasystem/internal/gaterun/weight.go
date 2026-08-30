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

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
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

type weightLock struct{ file *os.File }

func acquireWeightLock(root string) (*weightLock, error) {
	if err := os.MkdirAll(filepath.Dir(WeightLockPath(root)), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(WeightLockPath(root), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return &weightLock{file: file}, nil
}

func (lock *weightLock) release() {
	_ = unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	_ = lock.file.Close()
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
				return fmt.Errorf("applied validation weight decision has no predecessor generation")
			}
			if !decision.ResetDecision.Apply || decision.ResetDecision.WouldRefuse ||
				!decision.DischargeDecision.Apply || decision.DischargeDecision.WouldRefuse {
				return fmt.Errorf("applied validation weight decision lacks both consequences")
			}
		} else if !decision.ResetDecision.WouldRefuse && !decision.DischargeDecision.WouldRefuse {
			return fmt.Errorf("inert validation weight decision has no would-refuse outcome")
		}
	}
	seenProofs := map[string]bool{}
	for _, proof := range state.ConsumedProofs {
		if proof.RunID == "" || proof.GoalID == "" || proof.GoalRevision == 0 || proof.ObligationRevision == 0 || proof.ConsumedAt == "" ||
			!proof.ResetDecision.Apply || proof.ResetDecision.WouldRefuse || !proof.DischargeDecision.Apply || proof.DischargeDecision.WouldRefuse {
			return fmt.Errorf("consumed validation proof is incomplete")
		}
		if _, err := time.Parse(time.RFC3339, proof.ConsumedAt); err != nil {
			return fmt.Errorf("consumed validation proof timestamp is invalid")
		}
		key := fmt.Sprintf("%s\x00%s\x00%d\x00%d", proof.RunID, proof.GoalID, proof.ObligationRevision, proof.WeightGeneration)
		if seenProofs[key] {
			return fmt.Errorf("consumed validation proof is duplicated")
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
			return fmt.Errorf("applied validation weight decision has no exact consumed proof")
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
	weight, err := LandingWeight(numstat, prefix)
	if err != nil {
		return WeightState{}, false, err
	}
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

func WeightCheck(root string, threshold int64) (WeightState, bool, error) {
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightState{}, false, err
	}
	defer lock.release()
	state, err := loadWeight(root, weightNow())
	return state, err == nil && threshold > 0 && state.Accumulated >= threshold, err
}

// WeightDischarge is the reset action boundary. DRAFT and OBSERVE record an
// inert would-refuse; only exact current human authority can reset weight.
func WeightDischarge(root, goalID string, obligationRevision uint64, runID string) (WeightDischargeResult, error) {
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightDischargeResult{}, err
	}
	defer lock.release()
	now := weightNow()
	state, err := loadWeight(root, now)
	if err != nil {
		return WeightDischargeResult{}, err
	}
	for _, proof := range state.ConsumedProofs {
		if proof.RunID == runID && proof.GoalID == goalID && proof.ObligationRevision == obligationRevision {
			return WeightDischargeResult{}, fmt.Errorf("REFUSED-PROOF-CONSUMED: run %s already discharged weight generation %d", runID, proof.WeightGeneration)
		}
	}
	binding, err := dispatch.ResolveGoalBinding(root, goalID, now)
	if err != nil {
		return WeightDischargeResult{}, fmt.Errorf("weight discharge requires the exact accepted obligation revision: %w", err)
	}
	if binding.File.Obligation == nil || binding.File.Obligation.Revision != obligationRevision {
		return WeightDischargeResult{}, fmt.Errorf("weight discharge requires accepted obligation revision %d", obligationRevision)
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
	policy, policyErr := config.CorrelationPolicy(root)
	if policyErr != nil || policy == "" || obligation.ReviewPolicy != policy || obligation.ReviewOutcome != "human-approved" ||
		!resetDecision.Apply || !dischargeDecision.Apply {
		return result, fmt.Errorf("weight discharge refused: current human authorization and policy do not permit reset-weight")
	}
	projection := dispatch.ProjectBudget(root, binding.File, now)
	if projection.Status != dispatch.BudgetKnown {
		return result, fmt.Errorf("weight discharge refused: governed proof accounting is unknown: record=%s reason=%s", projection.Unknown.Record, projection.Unknown.Reason)
	}
	record, err := (&run.Store{Root: root}).Read(runID)
	if err != nil || record == nil || record.Status != run.StatusGreen || record.GoalId != goalID || record.Governed == nil ||
		record.Governed.ObligationRevision != obligationRevision || record.Governed.WeightGeneration == nil ||
		record.Governed.Observation == nil || record.Governed.Observation.AssumptionState != run.AssumptionMatch || record.Governed.Exhausted {
		return result, fmt.Errorf("weight discharge refused: run %s is not an exact green governed proof", runID)
	}
	if *record.Governed.WeightGeneration != state.Generation {
		return result, fmt.Errorf("REFUSED-PROOF-STALE: run %s proves weight generation %d, current generation is %d", runID, *record.Governed.WeightGeneration, state.Generation)
	}
	if !sameWeightEpoch(record.Governed.BudgetEpoch, projection.WeightEpoch) {
		return result, fmt.Errorf("REFUSED-PROOF-STALE: run %s is not bound to the current obligation budget epoch", runID)
	}
	source := fmt.Sprintf("%s-r%d-weight-g%d-%s", goalID, obligationRevision, state.Generation, runID)
	if _, err := retrodebt.Raise(root, retrodebt.KindObligation, source, now); err != nil {
		return result, fmt.Errorf("weight discharge refused: retro obligation could not be raised: %w", err)
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

func sameWeightEpoch(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
