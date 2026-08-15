package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
	"os"
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
		// The delivery walk's ONE resume (D64 phase 2): a devin host
		// candidate the session rule rejected post-envelope is skipped by
		// digest and the walk continues — a wrong-session stdout can delay
		// but never destroy a valid lower-channel result. Any failure of
		// the resumed candidate falls back to the ORIGINAL error; there is
		// never a second resume.
		var fault *SessionFault
		recollect := host.DeliveryRecollector(turn.Runtime)
		if errors.As(err, &fault) && recollect != nil {
			if resumed, resumeErr := resumeDelivery(root, turn, turnPath, turnDir, recollect); resumeErr == nil && resumed != nil {
				validation = resumed
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
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

// resumeDelivery re-collects a host turn's delivery past the candidate
// the runner just rejected — through the runtime's REGISTERED
// recollection capability, never a runtime name (agnosticism audit
// class 5) — then validates the new candidate through the same
// return-level path. The digest computation, validation, and
// ReturnValidation construction are runner-owned.
func resumeDelivery(root string, turn Turn, turnPath, turnDir string, recollect host.RecollectFn) (*ReturnValidation, error) {
	rejectedPath := filepath.Join(turnDir, "reply-accepted.json")
	rejectedBytes, err := os.ReadFile(rejectedPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(rejectedBytes)
	result, err := recollect(host.RecollectParams{
		Root:           root,
		TurnRecordPath: turnPath,
		TurnDir:        turnDir,
		Workspace:      root,
		RejectDigests:  []string{hex.EncodeToString(sum[:])},
	})
	if err != nil || !result.Delivered {
		return nil, fmt.Errorf("no further delivery candidate qualified")
	}
	returned, err := validateReturnAt(turn, result.ReplyPath, returnCompletenessCheck(root))
	if err != nil {
		return nil, err
	}
	return &ReturnValidation{Returned: returned, RawPath: filepath.Join(turnDir, "raw.out"), ReturnPath: result.ReplyPath}, nil
}
