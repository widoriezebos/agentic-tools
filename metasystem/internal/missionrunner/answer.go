package missionrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Answer applies a human's answer to an open ask: it validates the ask
// against the state, applies the reason class's unpark rule, marks the ask
// answered, and advances the state — rolling the ask back if the state write
// or anchor refuses. It prints the outcome and returns the exit code.
func (e *Engine) Answer(askID, answer string) int {
	statePath := filepath.Join(e.missionDir(), "state.json")
	state, err := e.verifyState(statePath, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 7
	}
	askPath := filepath.Join(e.missionDir(), "asks", askID+".json")
	if !pathExists(askPath) {
		fmt.Fprintf(os.Stderr, "answer refused: unknown ask %s\n", askID)
		return 3
	}
	ask, err := readDocLabeled(askPath, "mission ask", 3)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	if ask["askId"] != askID || ask["answeredAt"] != nil {
		fmt.Fprintf(os.Stderr, "answer refused: unknown or already answered ask %s\n", askID)
		return 3
	}
	reason, _ := ask["reasonClass"].(string)
	if reason == "stop-loss" {
		return e.answerStopLoss(statePath, state, ask, askPath, askID, answer)
	}
	if state["parkReason"] == "stop-loss" {
		fmt.Fprintln(os.Stderr, "answer refused: a stop-loss park is answered through its stop-loss ask")
		return 3
	}
	if !KnownAskReasons[reason] && reason != "fence" {
		fmt.Fprintf(os.Stderr, "answer refused: unsupported reason class %s\n", valueString(ask["reasonClass"]))
		return 3
	}
	proposed := deepCopyDoc(state)
	streamID, _ := ask["streamId"].(string)
	unpark := func() {
		proposed["status"] = "running"
		proposed["parkReason"] = nil
		proposed["gatePassed"] = false
	}
	switch {
	case reason == "reserved-decision" || reason == "red-test" || reason == "merge-conflict":
		streams, _ := proposed["streams"].(map[string]any)
		stream, _ := streams[streamID].(map[string]any)
		if stream == nil || stream["state"] != "parked-reserved" {
			fmt.Fprintln(os.Stderr, "answer refused: reserved ask does not name a parked-reserved stream")
			return 3
		}
		stream["state"] = "active"
		stream["reason"] = nil
		stream["answeredAsk"] = askID
		unpark()
	case reason == "host-failure" && proposed["status"] == "parked":
		if proposed["parkReason"] != "host-failure" {
			fmt.Fprintln(os.Stderr, "answer refused: host-failure answer does not match the mission park reason")
			return 3
		}
		if !anyActiveStream(proposed) {
			fmt.Fprintln(os.Stderr, "answer refused: host-failure mission has no active stream")
			return 3
		}
		unpark()
	case reason == "fence":
		stdout, stderr, code := runCaptured(e.Root, nil,
			filepath.Join(e.Root, "scripts", "assert-mission.sh"),
			"--preflight", "--file", e.contractPath())
		if code != 0 {
			fmt.Fprintf(os.Stderr, "answer refused: fence contract amendment is not preflight-ready: %s\n", firstDetail(stderr, stdout))
			return 3
		}
		_, values, _, err := e.parseContract(false)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		reached, err := e.fenceReached(values)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		if !reached {
			unpark()
		}
	}
	waiting, _ := proposed["waitingList"].([]any)
	remaining := []any{}
	for _, item := range waiting {
		if item != askID {
			remaining = append(remaining, item)
		}
	}
	proposed["waitingList"] = remaining
	originalAsk := deepCopyDoc(ask)
	ask["answeredAt"] = nowISO()
	ask["answer"] = answer
	if err := atomicWriteJSON(askPath, ask); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	updated, err := e.writeState(statePath, proposed)
	if err == nil {
		err = e.anchor(statePath, filepath.Join(e.missionDir(), "ledger.md"), e.Mission)
	}
	if err != nil {
		// The ask record and the state advance together or not at all: an
		// answered ask against an unmoved state would strand the mission.
		_ = atomicWriteJSON(askPath, originalAsk)
		fmt.Fprintf(os.Stderr, "answer refused: %s\n", err)
		return 3
	}
	fmt.Printf("mission=%s ask=%s applied=yes status=%s\n", e.Mission, askID, valueString(updated["status"]))
	return 0
}

// answerStopLoss applies a human's answer to a stop-loss ask. The vocal
// reset (plans/stop-loss-core.md) applies to a stagnation park alone and in
// binding order: (1) append the reset ledger line under the ledger lock,
// (2) mark the ask answered, (3) apply the unpark state write. A crash after
// (1) leaves the ask open — re-answering appends a second line, lawful and
// harmless; a crash after (2) leaves an answered ask on a parked mission —
// the next resume applies the unpark. Nothing is rolled back: the ledger
// line is the authoritative reset, and every later step can be replayed
// from it. Any other answer keeps the amendment guidance.
func (e *Engine) answerStopLoss(statePath string, state, ask map[string]any, askPath, askID, answer string) int {
	kind, _ := ask["stopLossKind"].(string)
	if !strings.HasPrefix(answer, "reset:") {
		fmt.Fprintln(os.Stderr, "answer refused: amend, price, reseal, and sign the mission budget before stop-loss unpark")
		return 3
	}
	switch kind {
	case mission.StopLossKindStagnation:
		// The one park the vocal reset applies to.
	case mission.StopLossKindCycleBudget:
		fmt.Fprintln(os.Stderr, "answer refused: reset: applies to a stagnation park only; this park is an exhausted sealed cycle budget — amend, price, reseal, and sign the mission budget")
		return 3
	default:
		fmt.Fprintln(os.Stderr, "answer refused: reset: applies to a stagnation park only; amend, price, reseal, and sign the mission budget")
		return 3
	}
	if state["status"] != "parked" || state["parkReason"] != "stop-loss" {
		fmt.Fprintln(os.Stderr, "answer refused: stop-loss reset applies to a mission parked for stop-loss")
		return 3
	}
	reason := strings.TrimSpace(strings.TrimPrefix(answer, "reset:"))
	ledgerPath := filepath.Join(e.missionDir(), "ledger.md")
	if err := mission.AppendReset(ledgerPath, askID, reason); err != nil {
		// Nothing after the ledger line may happen without it: the mission
		// stays parked, loudly.
		fmt.Fprintf(os.Stderr, "answer refused: stop-loss reset was not recorded: %v\n", err)
		return 3
	}
	e.emit("stop-loss-reset", clipSummary(reason), map[string]string{
		"missionId": e.Mission, "askId": askID,
	})
	answered := deepCopyDoc(ask)
	answered["answeredAt"] = nowISO()
	answered["answer"] = answer
	if err := atomicWriteJSON(askPath, answered); err != nil {
		fmt.Fprintf(os.Stderr, "stop-loss reset is recorded but the ask could not be marked answered: %v; answer it again — a second reset line is lawful and harmless\n", err)
		return 3
	}
	proposed := deepCopyDoc(state)
	proposed["status"] = "running"
	proposed["parkReason"] = nil
	proposed["gatePassed"] = false
	waiting, _ := proposed["waitingList"].([]any)
	remaining := []any{}
	for _, item := range waiting {
		if item != askID {
			remaining = append(remaining, item)
		}
	}
	proposed["waitingList"] = remaining
	updated, err := e.writeState(statePath, proposed)
	if err == nil {
		err = e.anchor(statePath, ledgerPath, e.Mission)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "stop-loss reset is recorded and the ask is answered, but the unpark did not apply: %v; the next resume applies it\n", err)
		return 3
	}
	fmt.Printf("mission=%s ask=%s applied=yes status=%s\n", e.Mission, askID, valueString(updated["status"]))
	return 0
}

// fenceReached reports whether the mission's fences are reached, witnessed
// in the flight recorder.
//
// TODO(go-wiring): this repeats the threshold math that lives in the
// mission-fence family; it needs a mission-fence verb that reports whether a
// contract's fences are reached, then the answer path can call that instead.
func (e *Engine) fenceReached(values map[string]string) (bool, error) {
	reached, err := e.fenceReachedInner(values)
	if err != nil {
		return false, err
	}
	e.emit("fence-check", fmt.Sprintf("reached=%v", reached), map[string]string{
		"missionId": e.Mission, "fence": "mission-fences",
	})
	return reached, nil
}

func (e *Engine) fenceReachedInner(values map[string]string) (bool, error) {
	path := e.fencesPath()
	if !pathExists(path) {
		return false, nil
	}
	fences, err := readDocLabeled(path, "mission fence counters", 3)
	if err != nil {
		return false, err
	}
	return fenceReachedAt(fences, values, missionJobStatuses(e.Root, e.Mission), time.Now().UTC())
}
