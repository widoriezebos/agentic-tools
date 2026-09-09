package steward

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const directValidationWindowSize = 2

var cadenceCatchGroups = testpolicy.CadenceCatchGroupIDs()

var cadenceFailureLinker = linkCadenceFailure

type validationWindowObservation struct {
	RunID      string   `json:"runId"`
	AttemptID  string   `json:"attemptId"`
	ObservedAt string   `json:"observedAt"`
	Missing    []string `json:"missingGroups"`
	NonGreen   []string `json:"nonGreenGroups"`
}

type validationWindowState struct {
	Schema       int                           `json:"schema"`
	Custodian    string                        `json:"custodian"`
	Observer     string                        `json:"observer"`
	WindowSize   int                           `json:"windowSize"`
	CatchClasses []string                      `json:"catchClassGroupIds"`
	Observations []validationWindowObservation `json:"observations"`
}

func validationWindowPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "direct-validation-window.json")
}

func newValidationWindow() validationWindowState {
	return validationWindowState{Schema: 1, Custodian: "Wido", Observer: "steward",
		WindowSize: directValidationWindowSize, CatchClasses: append([]string(nil), cadenceCatchGroups...)}
}

func loadValidationWindow(repoRoot string) (validationWindowState, error) {
	data, err := os.ReadFile(validationWindowPath(repoRoot))
	if os.IsNotExist(err) {
		return newValidationWindow(), nil
	}
	if err != nil {
		return validationWindowState{}, err
	}
	state := validationWindowState{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return validationWindowState{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || state.Schema != 1 || state.WindowSize != directValidationWindowSize ||
		state.Custodian != "Wido" || state.Observer != "steward" || strings.Join(state.CatchClasses, "\x00") != strings.Join(cadenceCatchGroups, "\x00") {
		return validationWindowState{}, fmt.Errorf("direct-validation observation window has an unknown schema or contract")
	}
	return state, nil
}

func saveValidationWindow(repoRoot string, state validationWindowState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(validationWindowPath(repoRoot), string(data)+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("direct-validation observation window durability is unknown")
	}
	return nil
}

func compareTestingResult(result proofrun.TestResult) (missing, nonGreen []string) {
	rows := map[string]proofrun.GroupResult{}
	for _, group := range result.Groups {
		rows[group.ID] = group
	}
	for _, id := range cadenceCatchGroups {
		group, present := rows[id]
		if !present {
			missing = append(missing, id)
		} else if !group.CollectionComplete || (group.Status != "passed" && group.Status != "reused") {
			nonGreen = append(nonGreen, id+"="+group.Status)
		}
	}
	return missing, nonGreen
}

func observeDirectValidationWindow(repoRoot string, now time.Time) error {
	state, err := loadValidationWindow(repoRoot)
	if err != nil {
		return err
	}
	// Observation and goal linkage are deliberately separate durable acts. A
	// transient failure after the observation is published must be recoverable
	// on the next cadence pass, including after the finite window is full.
	for _, observation := range state.Observations {
		if len(observation.Missing) == 0 && len(observation.NonGreen) == 0 {
			continue
		}
		if err := cadenceFailureLinker(repoRoot, observation, now); err != nil {
			return err
		}
	}
	if len(state.Observations) >= state.WindowSize {
		return nil
	}
	seen := map[string]bool{}
	for _, observation := range state.Observations {
		seen[observation.RunID] = true
	}
	store := &run.Store{Root: repoRoot}
	var candidates []*run.Record
	for _, path := range run.RecordFiles(repoRoot) {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, readErr := store.Read(id)
		if readErr == nil && record != nil && (record.Status == run.StatusGreen || record.Status == run.StatusRed) && record.Kind == "suite" &&
			record.Display == "weight-triggered direct validation" && record.Governed != nil && !seen[id] {
			candidates = append(candidates, record)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return *candidates[i].TerminalSeq < *candidates[j].TerminalSeq
	})
	changed := false
	for _, record := range candidates {
		if len(state.Observations) >= state.WindowSize {
			break
		}
		attempt, result, resultErr := proofrun.LatestGovernedTestResult(repoRoot, record.RunId)
		if resultErr != nil {
			continue
		}
		observation := validationWindowObservation{RunID: record.RunId, AttemptID: attempt.AttemptID, ObservedAt: now.UTC().Format(time.RFC3339)}
		observation.Missing, observation.NonGreen = compareTestingResult(result)
		state.Observations = append(state.Observations, observation)
		changed = true
	}
	if !changed {
		return nil
	}
	if err := saveValidationWindow(repoRoot, state); err != nil {
		return err
	}
	for _, observation := range state.Observations {
		if len(observation.Missing) == 0 && len(observation.NonGreen) == 0 {
			continue
		}
		if err := cadenceFailureLinker(repoRoot, observation, now); err != nil {
			return err
		}
	}
	return nil
}

func linkCadenceFailure(repoRoot string, observation validationWindowObservation, now time.Time) error {
	if !goal.NewWorld(repoRoot) {
		return nil
	}
	store := &run.Store{Root: repoRoot}
	record, err := store.Read(observation.RunID)
	if err != nil || record == nil || record.GoalId == "" {
		return fmt.Errorf("cadence failure %s has no accountable goal: %w", observation.RunID, err)
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil || projection.Tree == nil {
		return fmt.Errorf("read cadence correction goal %s: %w", record.GoalId, err)
	}
	file := projection.Tree.Live[record.GoalId]
	if file == nil || file.Claimed == nil {
		return fmt.Errorf("cadence correction goal %s is not claimed", record.GoalId)
	}
	finding := "cadence-" + observation.RunID
	chain := observation.AttemptID
	for _, existing := range file.ReviewObligations {
		if existing.Finding == finding && existing.Chain == chain {
			return nil
		}
	}
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	request := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage}, Ulid: ulid, Now: now}
	result, err := goal.DeferFindings(request, record.GoalId, []goal.ReviewObligation{{Finding: finding, Chain: chain,
		Artifact: "retained cadence result " + observation.RunID, Test: "focused repair for testing attempt " + observation.AttemptID}})
	if err != nil || (result.Outcome != goal.OutcomeConfirmed && result.Outcome != goal.OutcomeConfirmedLate) {
		return fmt.Errorf("link cadence failure %s to goal %s: outcome=%s: %w", observation.RunID, record.GoalId, result.Outcome, err)
	}
	return nil
}

func directValidationWindowFailures(repoRoot string) []string {
	state, err := loadValidationWindow(repoRoot)
	if err != nil {
		return []string{"direct-validation observation window unavailable: " + err.Error()}
	}
	var failures []string
	for _, observation := range state.Observations {
		if len(observation.Missing) > 0 || len(observation.NonGreen) > 0 {
			failures = append(failures, fmt.Sprintf("direct validation %s catch-class diff missing=%v nonGreen=%v",
				observation.RunID, observation.Missing, observation.NonGreen))
		}
	}
	return failures
}
