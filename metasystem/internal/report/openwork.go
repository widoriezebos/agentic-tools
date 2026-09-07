package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"golang.org/x/sys/unix"
)

// OpenWork reports plans that name an unblocked next step while nothing is
// running, and plans whose own account of themselves contradicts the job
// records. Continuation is the one part of the loop no prompt can guarantee,
// so this reporter makes stopping with open work a visible fact. It reads only
// the structured fields plans/README.md mandates, so its answer is the same
// under any runtime or none.
func OpenWork(root string) []string {
	root = resolveRepo(root)
	if info, err := os.Stat(filepath.Join(root, "plans")); err != nil || !info.IsDir() {
		return nil
	}
	return append(stalePlans(root), openWork(root)...)
}

// now and grace are the time source and chain grace window, overridable in tests.
var now = time.Now

func graceSeconds() float64 {
	if v := os.Getenv("METASYSTEM_CHAIN_GRACE_SECONDS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 5400
}

var (
	settledStep    = regexp.MustCompile(`(?i)^(none|nothing|n/?a|done|completed?|-|tbd)\b[.\s]*$`)
	templateValue  = regexp.MustCompile(`^<[^<>\r\n]+>$`)
	unblockedField = regexp.MustCompile(`(?i)^(none|nothing)\b`)
	roundSuffix    = regexp.MustCompile(`-r[0-9]+$`)
	roundRank      = regexp.MustCompile(`-r([0-9]+)$`)
)

var (
	inFlightStatus      = map[string]bool{"pending": true, "running": true}
	brainInFlightStatus = map[string]bool{"pending-setup": true, "pending": true, "running": true}
)

func inFlightStatuses(root string) map[string]bool {
	state := brain.Read(root, goal.ExistingLedgerIdentity(root))
	if state.State == brain.Declared || state.State == brain.Corrupt {
		return brainInFlightStatus
	}
	return inFlightStatus
}

func resolveRepo(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return root
}

// planField returns the value of a mandated "- <label>:" line, if present.
func planField(text, label string) (string, bool) {
	re := regexp.MustCompile(`(?m)^-\s*` + regexp.QuoteMeta(label) + `\s*:\s*(.*)$`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

func readJobRecords(root string) []map[string]any {
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts/agents/jobs", "*.json"))
	var records []map[string]any
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record map[string]any
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		record["__path"] = path
		records = append(records, record)
	}
	return records
}

func jobsInFlight(root string) int {
	count := 0
	records := readJobRecords(root)
	statuses := inFlightStatuses(root)
	for _, record := range records {
		if status, _ := record["status"].(string); statuses[status] {
			count++
		}
	}
	if len(openChainsInFlight(records)) > 0 {
		count++
	}
	if count == 0 && gatesRunning(root) {
		count++
	}
	return count
}

func openChainsInFlight(records []map[string]any) []string {
	chains := map[string]*chainEntry{}
	for _, record := range records {
		path, _ := record["__path"].(string)
		jobID, _ := record["jobId"].(string)
		if jobID == "" {
			jobID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		rootJob := roundSuffix.ReplaceAllString(jobID, "")
		entry := chains[rootJob]
		if entry == nil {
			entry = &chainEntry{}
			chains[rootJob] = entry
		}
		if jobID == rootJob {
			if closed, ok := record["chainClosed"].(bool); ok {
				entry.rootOpen = !closed
			}
		}
		rank := 1
		if m := roundRank.FindStringSubmatch(jobID); m != nil {
			rank, _ = strconv.Atoi(m[1])
		}
		if !entry.hasNewest || rank > entry.newestRank {
			entry.hasNewest = true
			entry.newestRank = rank
			entry.status, _ = record["status"].(string)
		}
	}
	var roots []string
	for rootJob, entry := range chains {
		if entry.rootOpen && entry.hasNewest && !dispatch.TerminalStatus(entry.status) {
			roots = append(roots, rootJob)
		}
	}
	sort.Strings(roots)
	return roots
}

// gatesRunning reports whether a gate is in flight — from the fixture override
// when set, otherwise from the gate-run markers this checkout keeps.
func gatesRunning(root string) bool {
	switch os.Getenv("METASYSTEM_GATES_RUNNING") {
	case "1":
		return true
	case "0":
		return false
	}
	return gaterun.Running(root)
}

type chainEntry struct {
	ids        map[string]bool
	closed     bool
	rootOpen   bool
	hasNewest  bool
	newestRank int
	status     string
	mtime      time.Time
}

type openWorkSeenRecord struct {
	SchemaVersion int                                     `json:"schemaVersion"`
	Plans         map[string]map[string]openWorkSeenEntry `json:"plans"`
}

type openWorkSeenEntry struct {
	Line    string `json:"line"`
	FirstAt string `json:"firstAt"`
}

func openWorkSeenPath(root string) string {
	return filepath.Join(resolveRepo(root), "artifacts", "agents", "supervision", "open-work-seen.json")
}

func openWorkSeenWarning(path string, err error) string {
	return fmt.Sprintf("OPEN-WORK-SEEN-RESET %s: %v", path, err)
}

func readOpenWorkSeen(path string) (openWorkSeenRecord, string) {
	empty := openWorkSeenRecord{SchemaVersion: 1, Plans: map[string]map[string]openWorkSeenEntry{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return empty, ""
	}
	if err != nil {
		return empty, openWorkSeenWarning(path, err)
	}
	var record openWorkSeenRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return empty, openWorkSeenWarning(path, err)
	}
	if record.SchemaVersion != 1 {
		return empty, openWorkSeenWarning(path, fmt.Errorf("unexpected schema version %d", record.SchemaVersion))
	}
	if record.Plans == nil {
		return empty, openWorkSeenWarning(path, fmt.Errorf("plans map is null"))
	}
	for plan, entries := range record.Plans {
		if entries == nil {
			return empty, openWorkSeenWarning(path, fmt.Errorf("plan %s has a null entries map", plan))
		}
		for digest, entry := range entries {
			expected := fmt.Sprintf("%x", sha256.Sum256([]byte(entry.Line)))
			if digest != expected {
				return empty, openWorkSeenWarning(path, fmt.Errorf("plan %s contains a line digest mismatch", plan))
			}
		}
	}
	return record, ""
}

// OpenWorkSeenWarning reads the durable marker without changing it. A
// deadline refusal uses this path so it cannot spend an open-work refusal.
func OpenWorkSeenWarning(root string) string {
	_, warning := readOpenWorkSeen(openWorkSeenPath(root))
	return warning
}

// MarkOpenWorkSeen durably records the current open plan lines, prunes lines
// no longer present, and tells the verdict which lines already refused a turn.
func MarkOpenWorkSeen(root string, items []goal.Item, at time.Time) ([]goal.Item, string, error) {
	path := openWorkSeenPath(root)
	if len(items) == 0 {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return items, "", nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, "", fmt.Errorf("prepare open-work seen directory: %w", err)
	}
	lockFile, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open open-work seen lock: %w", err)
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return nil, "", fmt.Errorf("lock open-work seen record: %w", err)
	}
	defer func() { _ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN) }()

	record, warning := readOpenWorkSeen(path)

	marked := append([]goal.Item(nil), items...)
	current := openWorkSeenRecord{SchemaVersion: 1, Plans: map[string]map[string]openWorkSeenEntry{}}
	stamp := at.UTC().Format(time.RFC3339)
	for index := range marked {
		plan := marked[index].Id
		line := marked[index].FullDetail
		if line == "" {
			line = marked[index].Detail
		}
		digest := marked[index].LineDigest
		if digest == "" {
			digest = fmt.Sprintf("%x", sha256.Sum256([]byte(line)))
		}
		marked[index].LineDigest = digest
		entries := record.Plans[plan]
		entry, seen := entries[digest]
		marked[index].PreviouslyRefused = seen
		if !seen {
			entry = openWorkSeenEntry{Line: line, FirstAt: stamp}
		}
		current.Plans[plan] = map[string]openWorkSeenEntry{digest: entry}
	}
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("render open-work seen record: %w", err)
	}
	if _, err := atomicfile.WriteText(path, string(encoded)+"\n", ""); err != nil {
		return nil, "", fmt.Errorf("write open-work seen record: %w", err)
	}
	return marked, warning, nil
}

func stalePlans(root string) []string {
	return stalePlansWithStatuses(root, inFlightStatuses(root))
}

func stalePlansWithStatuses(root string, statuses map[string]bool) []string {
	live := map[string]bool{}
	chains := map[string]*chainEntry{}
	for _, record := range readJobRecords(root) {
		path, _ := record["__path"].(string)
		jobID, _ := record["jobId"].(string)
		if jobID == "" {
			jobID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		status, _ := record["status"].(string)
		if statuses[status] {
			live[jobID] = true
		}
		rootJob := roundSuffix.ReplaceAllString(jobID, "")
		entry := chains[rootJob]
		if entry == nil {
			entry = &chainEntry{ids: map[string]bool{}}
			chains[rootJob] = entry
		}
		entry.ids[jobID] = true
		if jobID == rootJob {
			if closed, _ := record["chainClosed"].(bool); closed {
				entry.closed = true
			}
		}
		rank := 1
		if m := roundRank.FindStringSubmatch(jobID); m != nil {
			rank, _ = strconv.Atoi(m[1])
		}
		if !entry.hasNewest || rank > entry.newestRank {
			mtime := time.Time{}
			if info, err := os.Stat(path); err == nil {
				mtime = info.ModTime()
			}
			entry.hasNewest = true
			entry.newestRank = rank
			entry.status = status
			entry.mtime = mtime
		}
	}

	// An open chain is work in flight for a plan's purposes even between
	// rounds, but bounded by a grace window so an abandoned chain still goes
	// stale. jobsInFlight keeps the strict view; this is only for stale-plan
	// accuracy.
	current := map[string]bool{}
	for job := range live {
		current[job] = true
		current[roundSuffix.ReplaceAllString(job, "")] = true
	}
	grace := graceSeconds()
	nowTime := now()
	for rootJob, entry := range chains {
		if entry.closed || !entry.hasNewest {
			continue
		}
		if entry.status == "completed" && nowTime.Sub(entry.mtime).Seconds() <= grace {
			current[rootJob] = true
			for id := range entry.ids {
				current[id] = true
			}
		}
	}

	var lines []string
	for _, plan := range planFiles(root) {
		text, err := os.ReadFile(plan)
		if err != nil {
			continue
		}
		claim, ok := planField(string(text), "In flight right now")
		if !ok || claim == "" || unblockedField.MatchString(claim) {
			continue
		}
		name := relName(root, plan)
		if strings.Contains(strings.ToLower(claim), "gate") && gatesRunning(root) {
			continue
		}
		if len(current) == 0 {
			lines = append(lines, fmt.Sprintf("STALE-PLAN %s: claims work in flight while no job is running", name))
			continue
		}
		named := false
		for job := range current {
			if job != "" && strings.Contains(claim, job) {
				named = true
				break
			}
		}
		if !named {
			lines = append(lines, fmt.Sprintf("STALE-PLAN %s: names in-flight work that is not among the running jobs or open chains", name))
		}
	}
	return lines
}

func openWork(root string) []string {
	if jobsInFlight(root) > 0 {
		return nil
	}
	var lines []string
	for _, plan := range planFiles(root) {
		text, err := os.ReadFile(plan)
		if err != nil {
			continue
		}
		step, ok := planField(string(text), "Next step")
		if !ok || step == "" || settledStep.MatchString(step) {
			continue
		}
		if templateValue.MatchString(step) {
			lines = append(lines, fmt.Sprintf("TEMPLATE-UNFILLED %s: %s", relName(root, plan), step))
			continue
		}
		if waiting, ok := planField(string(text), "Waiting on the human"); ok && waiting != "" && !unblockedField.MatchString(waiting) {
			continue
		}
		lines = append(lines, fmt.Sprintf("OPEN-WORK %s: %s", relName(root, plan), step))
	}
	return lines
}

// planFiles lists the stream plans, sorted, excluding README.md (the format doc).
func planFiles(root string) []string {
	paths, _ := filepath.Glob(filepath.Join(root, "plans", "*.md"))
	sort.Strings(paths)
	var out []string
	for _, path := range paths {
		// goals.md is the goal ledger, read only by the goal parser
		// (scanner disjointness); README.md is the format doc.
		if base := filepath.Base(path); base != "README.md" && base != "goals.md" {
			out = append(out, path)
		}
	}
	return out
}

func relName(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}
