package missionrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	if reason == "stop-loss" || state["parkReason"] == "stop-loss" {
		fmt.Fprintln(os.Stderr, "answer refused: stop-loss requires a contract amendment before this minimal runner can unpark")
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
		err = e.anchorState(statePath, filepath.Join(e.missionDir(), "ledger.md"), e.Mission)
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
