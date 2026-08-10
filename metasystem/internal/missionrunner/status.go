package missionrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Status is the driver-facing view of a mission. It prints one status line
// and returns the exit code drivers branch on: 0 running, 10 completed, 11
// parked, 7 unreadable, 13 abandoned or runner-failed.
func (e *Engine) Status() int {
	statePath := filepath.Join(e.missionDir(), "state.json")
	if !pathExists(statePath) {
		fmt.Printf("mission=%s status=unreadable reason=missing-state\n", e.Mission)
		return 7
	}
	state, err := e.verifyState(statePath, false)
	if err != nil {
		fmt.Printf("mission=%s status=unreadable reason=%s\n", e.Mission, dashed(err.Error()))
		return 7
	}
	reason := valueString(state["parkReason"])
	if reason == "" {
		reason = "none"
	}
	status := valueString(state["status"])
	if status == "running" {
		// "Running" is a claim about a PROCESS, and the state file cannot
		// make it alone: when the runner has died, the mission advances
		// nothing, enforces no fence, and a driver trusting this status polls
		// forever (four and a half hours, the night this was written). The
		// runner record and the kernel decide whether anyone is actually
		// driving.
		recordPath, _, _ := e.runnerPaths()
		if !pathExists(recordPath) {
			fmt.Printf("mission=%s status=abandoned reason=no-runner-record\n", e.Mission)
			return 13
		}
		record, err := readDocLabeled(recordPath, "mission runner record", 3)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		switch record["status"] {
		case "failed":
			failure := valueString(record["error"])
			if failure == "" {
				failure = "unknown"
			}
			fmt.Printf("mission=%s status=runner-failed reason=%s\n", e.Mission, dashed(failure))
			return 13
		case "completed":
			// The previous runner CONCLUDED — it parked or finished and
			// finalized its record — and a human's answer has reopened the
			// mission. "Running with no live runner" is the legitimate
			// awaiting-resume resting state here, not abandonment: the next
			// step is `resume`, by whoever answered. Only a runner that died
			// without concluding is a defect worth stopping a driver for.
		default:
			pid, pidOK := jsonInt(record["pid"])
			recordedStart, startOK := jsonInt(record["pidStartedAt"])
			alive := false
			if pidOK && startOK {
				// A pid whose identity cannot be resolved is a pid that is
				// gone.
				if startedAt, err := processStartedAt(int(pid)); err == nil && startedAt == recordedStart {
					alive = true
				}
			}
			if !alive {
				fmt.Printf("mission=%s status=abandoned reason=runner-process-gone\n", e.Mission)
				return 13
			}
		}
	}
	fmt.Printf("mission=%s status=%s reason=%s\n", e.Mission, status, reason)
	switch status {
	case "running":
		return 0
	case "completed":
		return 10
	case "parked":
		return 11
	default:
		return 7
	}
}

// dashed flattens a message into the one-token reason field of a status line.
func dashed(message string) string {
	return strings.ReplaceAll(message, " ", "-")
}
