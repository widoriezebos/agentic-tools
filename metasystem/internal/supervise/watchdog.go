package supervise

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// The end-of-turn supervision health judgment (review cli-2, relocated from
// cmd): what a session should know before it ends — a stale or unsuccessful
// census, untracked agent processes, a fingerprint older than the code, and
// any recorded identity that is no longer running — each with re-arm advice.

// WatchdogReport derives the report lines for a checkout. The report is for
// a human at the end of a session: at most one supervision line (every
// symptom shares the one remedy), at most one untracked line (grouped pids,
// no argv walls — `proc census` has the detail), and NO lines when
// everything is healthy. The page of repeated WATCHDOG lines this replaced
// was ignored by its audience, which is the one failure a report cannot
// survive (human, 2026-08-13). now is injected so staleness is testable.
func WatchdogReport(repo string, now time.Time) []string {
	supervision := filepath.Join(repo, "artifacts", "agents", "supervision")
	var problems []string
	var untracked []string

	last, lastErr := readObjectFile(filepath.Join(supervision, "last-census.json"))
	if lastErr != nil {
		problems = append(problems, "never reported")
	} else {
		completed, _ := intField(last["completedAtEpoch"])
		interval, _ := intField(last["intervalSec"])
		age := now.Unix() - completed
		window := int64(0)
		if interval >= 1 {
			window = 2 * interval
			if window > 180 {
				window = 180
			}
		}
		if verdict, _ := last["verdict"].(string); verdict != "SUCCESS" {
			problems = append(problems, "last census failed")
		} else if interval < 1 || age >= window {
			problems = append(problems, "last census "+humanAge(age)+" old")
		}
		if inventory, ok := last["inventory"].([]any); ok {
			byRuntime := map[string][]string{}
			for _, raw := range inventory {
				item, _ := raw.(map[string]any)
				if item == nil {
					continue
				}
				if class, _ := item["class"].(string); class == "UNTRACKED" {
					pid, _ := intField(item["pid"])
					runtime, _ := item["runtime"].(string)
					if runtime == "" {
						runtime = "unknown"
					}
					byRuntime[runtime] = append(byRuntime[runtime], fmt.Sprint(pid))
				}
			}
			runtimes := make([]string, 0, len(byRuntime))
			for runtime := range byRuntime {
				runtimes = append(runtimes, runtime)
			}
			sort.Strings(runtimes)
			for _, runtime := range runtimes {
				untracked = append(untracked, runtime+" "+strings.Join(byRuntime[runtime], ","))
			}
		}
	}

	state, stateErr := readObjectFile(filepath.Join(supervision, "state.json"))
	owner, ownerOK := stateOwner(state)
	components, componentsOK := stateComponents(state)
	if stateErr != nil || !ownerOK || !componentsOK {
		problems = append(problems, "state unreadable")
	} else if lastErr == nil {
		stateFP, _ := state["fingerprint"].(string)
		lastFP, _ := last["fingerprint"].(string)
		if stateFP != lastFP {
			problems = append(problems, "code changed since arming")
		}
	}

	identities := map[string]map[string]any{}
	if owner != nil {
		identities["owner"] = owner
	}
	for name, raw := range components {
		if item, ok := raw.(map[string]any); ok {
			identities[name] = item
		}
	}
	names := make([]string, 0, len(identities))
	for name := range identities {
		names = append(names, name)
	}
	sort.Strings(names)
	var dead []string
	for _, name := range names {
		item := identities[name]
		pid, pidOK := intField(item["pid"])
		start, startOK := intField(item["pidStartedAt"])
		if !pidOK || !startOK || !census.Alive(pid, start) {
			dead = append(dead, name)
		}
	}
	if len(dead) > 0 {
		problems = append(problems, strings.Join(dead, "+")+" not running")
	}

	var lines []string
	if len(problems) > 0 {
		lines = append(lines, "SUPERVISION DOWN ("+strings.Join(problems, "; ")+") — re-arm: scripts/agents/arm-supervision.sh --repo .")
	}
	if len(untracked) > 0 {
		lines = append(lines, "UNTRACKED agents (not this checkout's work; detail: bin/metasystem proc census): "+strings.Join(untracked, "; "))
	}
	return lines
}

// humanAge renders seconds the way a person reads them: 90s, 14m, 6h2m, 3d.
func humanAge(seconds int64) string {
	if seconds < 120 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	if seconds < 48*3600 {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dd", seconds/86400)
}

func stateOwner(state map[string]any) (map[string]any, bool) {
	if state == nil {
		return nil, false
	}
	owner, ok := state["owner"].(map[string]any)
	return owner, ok
}

func stateComponents(state map[string]any) (map[string]any, bool) {
	if state == nil {
		return nil, false
	}
	components, ok := state["components"].(map[string]any)
	return components, ok
}
