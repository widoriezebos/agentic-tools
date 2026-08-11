package missionrunner

import (
	"fmt"
	"path/filepath"
)

// File-backed entry points over the pure adjudication and conclusion logic.
// The run loop and the mission-turn verbs share these, so a turn judged
// in-process and a turn judged through the CLI read exactly the same
// artifacts the same way.

// returnCompletenessCheck runs the shipped role checker on a return file, so
// return-schema authority stays in one place, wrapping a refusal in the
// runner's own words.
func returnCompletenessCheck(root string) func(returnPath string) error {
	return func(returnPath string) error {
		stdout, stderr, code := runCaptured(root, nil,
			filepath.Join(root, "scripts", "assert-return-complete.sh"),
			"--role", "orchestrator", "--file", returnPath)
		if code != 0 {
			return fmt.Errorf("orchestrator return is invalid: %s", firstDetail(stderr, stdout))
		}
		return nil
	}
}

// AdjudicateFiles validates a host turn's result envelope and orchestrator
// return against the turn's identity, then judges every claim in the return
// against the mission state and job records, returning the verdict for the
// caller to apply.
func AdjudicateFiles(root, mission, statePath, turnPath, resultPath, turnDir, nowISO string) (*Verdict, error) {
	state, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	turnDoc, err := readDocLabeled(turnPath, "turn record", 3)
	if err != nil {
		return nil, err
	}
	turn, err := TurnFromDoc(turnDoc)
	if err != nil {
		return nil, err
	}
	result, err := readDocLabeled(resultPath, "host result", 3)
	if err != nil {
		return nil, err
	}
	validation, err := ValidateReturn(turn, result, turnDir, returnCompletenessCheck(root))
	if err != nil {
		return nil, err
	}
	verdict, err := Adjudicate(root, mission, turn, state, validation.Returned, nowISO)
	if err != nil {
		return nil, err
	}
	verdict.RawPath = validation.RawPath
	verdict.ReturnPath = validation.ReturnPath
	return verdict, nil
}

// ConcludeFiles proposes the state after an accepted turn from the turn's
// file artifacts: the adjudicated streams, the turn-log entry, the advanced
// cycle count, the refreshed waiting list and fences, and the continue/park/
// complete decision. The caller applies the proposal through the state's
// compare-and-write.
func ConcludeFiles(root, mission, statePath, turnPath, verdictPath, returnPath, resultPath, measurementPath string) (map[string]any, error) {
	inputs := map[string]map[string]any{}
	for label, path := range map[string]string{
		"mission state":        statePath,
		"turn record":          turnPath,
		"adjudication verdict": verdictPath,
		"orchestrator return":  returnPath,
		"host result":          resultPath,
		"measurement":          measurementPath,
	} {
		doc, err := readDocLabeled(path, label, 3)
		if err != nil {
			return nil, err
		}
		inputs[label] = doc
	}
	turn, err := TurnFromDoc(inputs["turn record"])
	if err != nil {
		return nil, err
	}
	state := inputs["mission state"]
	verdict := inputs["adjudication verdict"]
	streams, ok := verdict["streams"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("adjudication verdict has no stream map")
	}
	// The proposal builds on the adjudicated streams, not the streams as the
	// last write left them: the verdict is what this turn already decided.
	state["streams"] = streams
	gatePassed, _ := inputs["measurement"]["gatePassed"].(bool)
	return ConcludeTurn(root, mission, state, turn, TurnConclusion{
		SessionID:      ConclusionSession(turn, inputs["host result"]["sessionId"]),
		Measurement:    inputs["measurement"]["measurement"],
		GatePassed:     gatePassed,
		Accepted:       verdict["accepted"],
		Rejected:       verdict["rejected"],
		Certified:      inputs["orchestrator return"]["certified"],
		FactsForLedger: inputs["orchestrator return"]["factsForLedger"],
		Gaps:           inputs["orchestrator return"]["gaps"],
	})
}
