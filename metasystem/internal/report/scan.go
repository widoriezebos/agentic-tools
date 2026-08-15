package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionstate"
)

// Scan fills the verdict's input contract (goal.ScanResult — report
// imports goal, the declared edge; the verdict never imports report).
// Busy comes from CHECKOUT-SCOPED FILE FACTS ONLY: job records, gate-run
// markers, and runner records correlated by the missionstate rule — argv
// matching is retired from this path, so another checkout's activity can
// never suppress this checkout's goal. Every input the scan cannot read
// surfaces in Unreadable; enumeration failure never collapses to idle.
func Scan(root string) goal.ScanResult {
	return scanWithProber(root, identity.KernelProber{})
}

func scanWithProber(root string, prober identity.Prober) goal.ScanResult {
	root = resolveRepo(root)
	var result goal.ScanResult

	// Busy, three classes, all file facts.
	jobItems, jobUnreadable := busyJobs(root)
	result.Busy = append(result.Busy, jobItems...)
	result.Unreadable = append(result.Unreadable, jobUnreadable...)

	gates := gaterun.Survey(root)
	for _, marker := range gates.Live {
		result.Busy = append(result.Busy, goal.Item{
			Kind: "gate", Id: marker.Gate,
			Detail: clipDetail(fmt.Sprintf("gate %s [pid %d]", marker.Gate, marker.Pid)),
		})
	}
	result.Unreadable = append(result.Unreadable, gates.Unreadable...)
	if os.Getenv("METASYSTEM_GATES_RUNNING") == "1" {
		result.Busy = append(result.Busy, goal.Item{Kind: "gate", Id: "fixture", Detail: "gate fixture [env]"})
	}

	missions := missionstate.Survey(root, prober)
	for _, runner := range missions.ActiveMissions() {
		result.Busy = append(result.Busy, goal.Item{Kind: "mission", Id: runner.MissionId, Detail: clipDetail(runner.Detail)})
	}
	result.Unreadable = append(result.Unreadable, missions.Unreadable...)

	// Plans: open steps, human waits, staleness — goals.md never counts
	// (scanner disjointness: only the goal parser reads the ledger).
	if info, err := os.Stat(filepath.Join(root, "plans")); err == nil && info.IsDir() {
		result = scanPlans(root, result)
	}
	return result
}

// scanPlans classifies every plan stream.
func scanPlans(root string, result goal.ScanResult) goal.ScanResult {
	for _, line := range stalePlans(root) {
		result.StalePlans = append(result.StalePlans, goal.Item{Kind: "plan", Id: line, Detail: clipDetail(line)})
	}
	for _, plan := range planFiles(root) {
		text, err := os.ReadFile(plan)
		if err != nil {
			result.Unreadable = append(result.Unreadable, plan+": "+err.Error())
			continue
		}
		name := relName(root, plan)
		if waiting, ok := planField(string(text), "Waiting on the human"); ok && waiting != "" && !unblockedField.MatchString(waiting) {
			result.WaitingOnHuman = append(result.WaitingOnHuman, goal.Item{
				Kind: "plan", Id: name, Detail: clipDetail(fmt.Sprintf("%s waits on the human: %s", name, waiting)),
			})
			continue
		}
		step, ok := planField(string(text), "Next step")
		if !ok || step == "" || settledStep.MatchString(step) {
			continue
		}
		result.Open = append(result.Open, goal.Item{
			Kind: "plan", Id: name, Detail: clipDetail(fmt.Sprintf("OPEN-WORK %s: %s", name, step)),
		})
	}
	return result
}

// busyJobs reads the checkout's delegate job records, surfacing failures.
func busyJobs(root string) ([]goal.Item, []string) {
	var items []goal.Item
	var unreadable []string
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, []string{dir + ": " + err.Error()}
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			unreadable = append(unreadable, path+": "+err.Error())
			continue
		}
		var record struct {
			JobId   string `json:"jobId"`
			Role    string `json:"role"`
			Runtime string `json:"runtime"`
			Status  string `json:"status"`
		}
		if json.Unmarshal(data, &record) != nil {
			unreadable = append(unreadable, path+": unparsable job record")
			continue
		}
		if !inFlightStatus[record.Status] {
			continue
		}
		if record.JobId == "" {
			record.JobId = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if record.Role == "" {
			record.Role = "?"
		}
		if record.Runtime == "" {
			record.Runtime = "?"
		}
		items = append(items, goal.Item{
			Kind: "job", Id: record.JobId,
			Detail: clipDetail(fmt.Sprintf("%s %s [%s, %s]", record.Role, record.JobId, record.Status, record.Runtime)),
		})
	}
	return items, unreadable
}

func clipDetail(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200]
}
