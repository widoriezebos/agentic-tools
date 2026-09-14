package report

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

type openWorkPlanReader func(string) ([]byte, error)
type openWorkDirectoryReader func(string) ([]os.DirEntry, error)

var readOpenWorkPlan openWorkPlanReader = os.ReadFile
var readOpenWorkDirectory openWorkDirectoryReader = os.ReadDir

func openWorkPlanPaths(ctx context.Context, root string) ([]string, error) {
	type result struct {
		entries []os.DirEntry
		err     error
	}
	done := make(chan result, 1)
	reader := readOpenWorkDirectory
	go func() {
		entries, err := reader(filepath.Join(root, "plans"))
		done <- result{entries: entries, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case answer := <-done:
		if os.IsNotExist(answer.err) {
			return nil, nil
		}
		if answer.err != nil {
			return nil, answer.err
		}
		paths := make([]string, 0, len(answer.entries))
		for _, entry := range answer.entries {
			name := entry.Name()
			if entry.IsDir() || filepath.Ext(name) != ".md" || name == "README.md" || name == "goals.md" {
				continue
			}
			paths = append(paths, filepath.Join(root, "plans", name))
		}
		sort.Strings(paths)
		return paths, nil
	}
}

func readOpenWorkPlanContext(ctx context.Context, path string) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	reader := readOpenWorkPlan
	go func() {
		data, err := reader(path)
		done <- result{data: data, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case answer := <-done:
		return answer.data, answer.err
	}
}

// OpenWorkSignature reads only the plan fields that make up the stop gate's
// open-work digest. It deliberately does not scan jobs, runs, the ledger, or
// any other report input, so a wait-cycle check never starts a Git process.
func OpenWorkSignature(ctx context.Context, root string) (string, error) {
	if !filepath.IsAbs(root) {
		absolute, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("resolve open-work root: %w", err)
		}
		root = absolute
	}
	root = filepath.Clean(root)
	result := goal.ScanResult{}
	plans, err := openWorkPlanPaths(ctx, root)
	if err != nil {
		return "", fmt.Errorf("list open-work plans: %w", err)
	}
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		text, err := readOpenWorkPlanContext(ctx, plan)
		if err != nil {
			return "", fmt.Errorf("read open-work plan %s: %w", plan, err)
		}
		if waiting, ok := planField(string(text), "Waiting on the human"); ok && waiting != "" && !unblockedField.MatchString(waiting) {
			continue
		}
		step, ok := planField(string(text), "Next step")
		if !ok || step == "" || settledStep.MatchString(step) || templateValue.MatchString(step) {
			continue
		}
		line := fmt.Sprintf("OPEN-WORK %s: %s", relName(root, plan), step)
		result.Open = append(result.Open, goal.Item{
			Kind: "plan", Id: relName(root, plan), Detail: clipDetail(line), FullDetail: line,
			LineDigest: fmt.Sprintf("%x", sha256.Sum256([]byte(line))),
		})
	}
	return result.OpenWorkSignature(), nil
}

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

// ScanForBrainDeclaration applies the brain's full in-flight set before a
// declaration exists, so quiescence cannot overlook a pending-setup job.
func ScanForBrainDeclaration(root string) goal.ScanResult {
	root = resolveRepo(root)
	return scanWithProberAndStatuses(root, identity.KernelProber{}, brainInFlightStatus)
}

func scanWithProber(root string, prober identity.Prober) goal.ScanResult {
	root = resolveRepo(root)
	return scanWithProberAndStatuses(root, prober, inFlightStatuses(root))
}

func scanWithProberAndStatuses(root string, prober identity.Prober, statuses map[string]bool) goal.ScanResult {
	var result goal.ScanResult
	goalIntents := readGoalIntents(root)

	// Busy, three classes, all file facts.
	jobItems, jobUnreadable := busyJobs(root, statuses, goalIntents)
	result.Busy = append(result.Busy, jobItems...)
	if len(jobItems) == 0 {
		for _, rootJob := range openChainsInFlight(readJobRecords(root)) {
			result.Busy = append(result.Busy, goal.Item{
				Kind: "job", Id: rootJob, Detail: clipDetail(fmt.Sprintf("open delegate chain %s [non-terminal]", rootJob)),
				FullDetail: fmt.Sprintf("open delegate chain %s [non-terminal]", rootJob),
			})
		}
	}
	result.Unreadable = append(result.Unreadable, jobUnreadable...)

	gates := gaterun.Survey(root)
	for _, marker := range gates.Live {
		full := fmt.Sprintf("gate %s [pid %d]", marker.Gate, marker.Pid)
		result.Busy = append(result.Busy, goal.Item{
			Kind: "gate", Id: marker.Gate,
			Detail: clipDetail(full), FullDetail: full,
		})
	}
	result.Unreadable = append(result.Unreadable, gates.Unreadable...)
	if os.Getenv("METASYSTEM_GATES_RUNNING") == "1" {
		result.Busy = append(result.Busy, goal.Item{Kind: "gate", Id: "fixture", Detail: "gate fixture [env]"})
	}

	missions := missionstate.Survey(root, prober)
	for _, runner := range missions.ActiveMissions() {
		result.Busy = append(result.Busy, goal.Item{Kind: "mission", Id: runner.MissionId, Detail: clipDetail(runner.Detail), FullDetail: runner.Detail})
	}
	result.Unreadable = append(result.Unreadable, missions.Unreadable...)

	// The monitor facility's typed facts: job facts for the
	// unwatched rule, run facts for warnings + the green cursor, and the
	// run readers' own failure channel. Live runs also join Busy so the
	// STILL WORKING sentence names them.
	result.Jobs = jobFacts(root, prober, statuses, goalIntents)
	runFacts, runBusy, runUnreadable := runFactsFor(root, prober)
	result.Runs = runFacts
	result.Busy = append(result.Busy, runBusy...)
	result.RunUnreadable = runUnreadable

	questions, questionUnreadable := channel.WalkOpenQuestions(root)
	for _, question := range questions {
		full := fmt.Sprintf("%s goal %s %s from %s since %s: %s", question.ID, question.Goal,
			question.Kind, question.Machine, question.OpenedAt.UTC().Format(time.RFC3339), question.Wants)
		result.Questions = append(result.Questions, goal.Item{
			Kind: "question", Id: question.ID,
			Detail: clipDetail(full), FullDetail: full, RequestedAction: question.Wants, HumanRequired: true,
		})
	}
	for _, detail := range questionUnreadable {
		result.Unreadable = append(result.Unreadable, "question scan: "+detail)
	}
	result = scanDrafts(root, result)

	// Plans: open steps, human waits, staleness — goals.md never counts
	// (scanner disjointness: only the goal parser reads the ledger).
	if info, err := os.Stat(filepath.Join(root, "plans")); err == nil && info.IsDir() {
		result = scanPlans(root, result, statuses)
	}
	return result
}

func scanDrafts(root string, result goal.ScanResult) goal.ScanResult {
	if !goal.NewWorld(root) {
		return result
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		result.Unreadable = append(result.Unreadable, "draft scan: "+err.Error())
		return result
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		result.Unreadable = append(result.Unreadable, "draft scan: "+err.Error())
		return result
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil {
		result.Unreadable = append(result.Unreadable, "draft scan: "+err.Error())
		return result
	}
	for id, item := range projection.Tree.Live {
		if item.State != goal.StateQueued || item.Approved != nil {
			continue
		}
		openedHere := false
		for _, history := range item.History {
			if history.Verb == "open" {
				openedHere = strings.HasPrefix(history.Actor, machine+"+")
				break
			}
		}
		if openedHere {
			result.Drafts = append(result.Drafts, goal.Item{Kind: "draft", Id: id, Detail: clipDetail(item.NextStep), FullDetail: item.NextStep,
				SourcePath: filepath.ToSlash(filepath.Join("plans", "goals", id+".md"))})
		}
	}
	sort.Slice(result.Drafts, func(i, j int) bool { return result.Drafts[i].Id < result.Drafts[j].Id })
	return result
}

// scanPlans classifies every plan stream.
func scanPlans(root string, result goal.ScanResult, statuses map[string]bool) goal.ScanResult {
	for _, line := range stalePlansWithStatuses(root, statuses) {
		result.StalePlans = append(result.StalePlans, goal.Item{Kind: "plan", Id: line, Detail: clipDetail(line), FullDetail: line})
	}
	for _, plan := range planFiles(root) {
		text, err := os.ReadFile(plan)
		if err != nil {
			result.Unreadable = append(result.Unreadable, plan+": "+err.Error())
			continue
		}
		name := relName(root, plan)
		if waiting, ok := planField(string(text), "Waiting on the human"); ok && waiting != "" && !unblockedField.MatchString(waiting) {
			full := fmt.Sprintf("%s waits on the human: %s", name, waiting)
			result.WaitingOnHuman = append(result.WaitingOnHuman, goal.Item{
				Kind: "plan", Id: name, Detail: clipDetail(full), FullDetail: full, SourcePath: name,
				RequestedAction: waiting, HumanRequired: true,
			})
			continue
		}
		step, ok := planField(string(text), "Next step")
		if !ok || step == "" || settledStep.MatchString(step) {
			continue
		}
		if templateValue.MatchString(step) {
			full := fmt.Sprintf("TEMPLATE-UNFILLED %s: %s", name, step)
			result.TemplateUnfilled = append(result.TemplateUnfilled, goal.Item{
				Kind: "plan", Id: name, Detail: clipDetail(full), FullDetail: full, SourcePath: name,
				RequestedAction: step,
			})
			continue
		}
		line := fmt.Sprintf("OPEN-WORK %s: %s", name, step)
		result.Open = append(result.Open, goal.Item{
			Kind: "plan", Id: name, Detail: clipDetail(line), FullDetail: line, SourcePath: name, RequestedAction: step,
			LineDigest: fmt.Sprintf("%x", sha256.Sum256([]byte(line))),
		})
	}
	return result
}

// busyJobs reads the checkout's delegate job records, surfacing failures.
func busyJobs(root string, statuses map[string]bool, goalIntents map[string]string) ([]goal.Item, []string) {
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
			JobId   string  `json:"jobId"`
			Role    string  `json:"role"`
			Runtime string  `json:"runtime"`
			Status  string  `json:"status"`
			MainId  *string `json:"mainId"`
			GoalId  string  `json:"goalId"`
		}
		if json.Unmarshal(data, &record) != nil {
			unreadable = append(unreadable, path+": unparsable job record")
			continue
		}
		if !statuses[record.Status] {
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
		full := fmt.Sprintf("%s %s [%s, %s]", record.Role, record.JobId, record.Status, record.Runtime)
		ownerMainID := ""
		if record.MainId != nil {
			ownerMainID = *record.MainId
		}
		title := goalIntents[record.GoalId]
		if title == "" {
			title = normalizeRoleTitle(record.Role)
		}
		items = append(items, goal.Item{
			Kind: "job", Id: record.JobId,
			Detail: clipDetail(full), FullDetail: full, SourcePath: relName(root, path), OwnerMainId: ownerMainID,
			RequestedAction: title,
		})
	}
	return items, unreadable
}

func clipDetail(s string) string {
	if len(s) <= 200 {
		return s
	}
	end := 200
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// jobFacts reads the delegate job records' monitor-relevant slice.
func jobFacts(root string, prober identity.Prober, statuses map[string]bool, goalIntents map[string]string) []goal.JobFact {
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
			Role      string  `json:"role"`
			GoalId    string  `json:"goalId"`
		}
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		if !statuses[record.Status] {
			continue
		}
		if record.JobId == "" {
			record.JobId = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		mainId := ""
		if record.MainId != nil {
			mainId = *record.MainId
		}
		title := goalIntents[record.GoalId]
		if title == "" {
			title = normalizeRoleTitle(record.Role)
		}
		facts = append(facts, goal.JobFact{
			Id: record.JobId, MainId: mainId, StartedAt: record.StartedAt, Status: record.Status,
			Title: title, Role: record.Role, GoalId: record.GoalId, SourcePath: relName(root, path),
			SourceDigest: fmt.Sprintf("%x", sha256.Sum256(data)), Ownership: "unknown",
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
			Title: record.Display, Role: record.Kind, GoalId: record.GoalId, StartedAt: record.StartedAt,
			SourcePath: filepath.ToSlash(filepath.Join("artifacts", "agents", "runs", record.RunId+".json")),
			Ownership:  "unknown",
		}
		if record.MainId != nil {
			fact.MainId = *record.MainId
		}
		if fact.SourcePath != "" {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fact.SourcePath))); err == nil {
				fact.SourceDigest = fmt.Sprintf("%x", sha256.Sum256(data))
			}
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
			full := fmt.Sprintf("run %s [%s] %s", record.RunId, record.Status, record.Display)
			busy = append(busy, goal.Item{Kind: "run", Id: record.RunId,
				Detail: clipDetail(full), FullDetail: full, SourcePath: fact.SourcePath, OwnerMainId: fact.MainId,
				RequestedAction: record.Display})
		}
	}
	return facts, busy, unreadable
}

func normalizeRoleTitle(role string) string {
	return strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(role))
}

func readGoalIntents(root string) map[string]string {
	intents := map[string]string{}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return intents
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil || projection.Tree == nil {
		return intents
	}
	for id, item := range projection.Tree.Live {
		if item != nil {
			intents[id] = item.Intent
		}
	}
	return intents
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
	// supervision state: a one-shot pass or a
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
