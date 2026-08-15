package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
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

	// The monitor facility's typed facts (MON-05): job facts for the
	// unwatched rule, run facts for warnings + the green cursor, and the
	// run readers' own failure channel. Live runs also join Busy so the
	// STILL WORKING sentence names them.
	result.Jobs = jobFacts(root, prober)
	runFacts, runBusy, runUnreadable := runFactsFor(root, prober)
	result.Runs = runFacts
	result.Busy = append(result.Busy, runBusy...)
	result.RunUnreadable = runUnreadable

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

// jobFacts reads the delegate job records' monitor-relevant slice.
func jobFacts(root string, prober identity.Prober) []goal.JobFact {
	var facts []goal.JobFact
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // busyJobs already surfaced it
		}
		var record struct {
			JobId     string  `json:"jobId"`
			MainId    *string `json:"mainId"`
			StartedAt string  `json:"startedAt"`
			Status    string  `json:"status"`
		}
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		if !inFlightStatus[record.Status] {
			continue
		}
		if record.JobId == "" {
			record.JobId = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		mainId := ""
		if record.MainId != nil {
			mainId = *record.MainId
		}
		facts = append(facts, goal.JobFact{
			Id: record.JobId, MainId: mainId, StartedAt: record.StartedAt, Status: record.Status,
			WaiterLive: run.LiveWaiter(root, prober, "job", record.JobId, mainId,
				run.WaiterTarget{StartedAt: record.StartedAt}),
		})
	}
	return facts
}

// runFactsFor reads run records into typed facts, Busy items for live
// runs, and the run readers' failure channel — including the attestation
// facts behind Supervised.
func runFactsFor(root string, prober identity.Prober) ([]goal.RunFact, []goal.Item, []string) {
	store := &run.Store{Root: root}
	records, unreadable := store.List()
	attested, attestErr := readRunsPass(root, prober)
	if attestErr != "" {
		unreadable = append(unreadable, attestErr)
	}
	var facts []goal.RunFact
	var busy []goal.Item
	for _, record := range records {
		fact := goal.RunFact{
			Id: record.RunId, Generation: record.Generation, Nonce: record.LaunchNonce,
			Status: record.Status, Acked: record.Acked,
			Hung:        record.HungSince != nil,
			ExpectGreen: record.Expect.Green, ExpectRed: record.Expect.Red,
			ExpectHung: record.Expect.Hung, ExpectUnknown: record.Expect.Unknown,
		}
		if record.MainId != nil {
			fact.MainId = *record.MainId
		}
		if record.TerminalSeq != nil {
			fact.TerminalSeq = *record.TerminalSeq
		}
		if record.Pid != nil && record.PidStartedAt != nil {
			switch identity.AliveRef(prober, identity.Ref{Pid: *record.Pid, StartedAtSec: *record.PidStartedAt}) {
			case identity.Alive:
				fact.ProbeState = "alive"
			case identity.Unknown:
				fact.ProbeState = "unknown"
			default:
				fact.ProbeState = "dead"
			}
		}
		key := fmt.Sprintf("%s.g%d.%s", record.RunId, record.Generation, record.LaunchNonce)
		fact.Supervised = attested[key]
		fact.WaiterLive = run.LiveWaiter(root, prober, "run", record.RunId, fact.MainId,
			run.WaiterTarget{Generation: record.Generation, LaunchNonce: record.LaunchNonce})
		facts = append(facts, fact)
		switch record.Status {
		case run.StatusLaunching, run.StatusRunning, run.StatusDraining:
			busy = append(busy, goal.Item{Kind: "run", Id: record.RunId,
				Detail: clipDetail(fmt.Sprintf("run %s [%s] %s", record.RunId, record.Status, record.Display))})
		}
	}
	return facts, busy, unreadable
}

// readRunsPass loads the watcher's attestation: the set of lifecycle
// triples a FRESH pass by the LIVE armed watcher scanned.
func readRunsPass(root string, prober identity.Prober) (map[string]bool, string) {
	path := filepath.Join(root, "artifacts", "agents", "supervision", "runs-pass.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, path + ": " + err.Error()
	}
	var attestation struct {
		CompletedAt  string `json:"completedAt"`
		WatcherPid   int64  `json:"watcherPid"`
		WatcherStart int64  `json:"watcherStart"`
		ScannedRuns  []struct {
			Id          string `json:"id"`
			Generation  int    `json:"generation"`
			LaunchNonce string `json:"launchNonce"`
		} `json:"scannedRuns"`
	}
	if json.Unmarshal(data, &attestation) != nil {
		return nil, path + ": unparsable attestation"
	}
	// The freshness bound and the ARMED watcher identity come from the
	// supervision state (critique finding 5): a one-shot pass or a
	// future-stamped file supervises nothing.
	armedPid, armedStart, intervalSec, armedOK := armedWatcherIdentity(root)
	if !armedOK {
		return nil, ""
	}
	if attestation.WatcherPid != armedPid || attestation.WatcherStart != armedStart {
		return nil, ""
	}
	completed, err := time.Parse("2006-01-02T15:04:05Z", attestation.CompletedAt)
	if err != nil || completed.After(time.Now().Add(2*time.Second)) ||
		time.Since(completed) > 2*time.Duration(intervalSec)*time.Second {
		return nil, ""
	}
	if identity.AliveRef(prober, identity.Ref{Pid: attestation.WatcherPid, StartedAtSec: attestation.WatcherStart}) != identity.Alive {
		return nil, ""
	}
	out := map[string]bool{}
	for _, scanned := range attestation.ScannedRuns {
		out[fmt.Sprintf("%s.g%d.%s", scanned.Id, scanned.Generation, scanned.LaunchNonce)] = true
	}
	return out, ""
}

// armedWatcherIdentity reads the standing watcher's recorded identity and
// the loaded interval from the supervision state.
func armedWatcherIdentity(root string) (pid, start int64, intervalSec int, ok bool) {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "supervision", "state.json"))
	if err != nil {
		return 0, 0, 0, false
	}
	var state struct {
		Components map[string]struct {
			Pid          int64 `json:"pid"`
			PidStartedAt int64 `json:"pidStartedAt"`
		} `json:"components"`
		IntervalSec int `json:"intervalSec"`
	}
	if json.Unmarshal(data, &state) != nil {
		return 0, 0, 0, false
	}
	watcher, present := state.Components["watcher"]
	if !present || state.IntervalSec <= 0 {
		return 0, 0, 0, false
	}
	return watcher.Pid, watcher.PidStartedAt, state.IntervalSec, true
}
