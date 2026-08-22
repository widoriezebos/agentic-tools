package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
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
	unblockedField = regexp.MustCompile(`(?i)^(none|nothing)\b`)
	roundSuffix    = regexp.MustCompile(`-r[0-9]+$`)
	roundRank      = regexp.MustCompile(`-r([0-9]+)$`)
)

var inFlightStatus = map[string]bool{"pending": true, "running": true}

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
	for _, record := range readJobRecords(root) {
		if status, _ := record["status"].(string); inFlightStatus[status] {
			count++
		}
	}
	if count == 0 && gatesRunning(root) {
		count++
	}
	return count
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
	hasNewest  bool
	newestRank int
	status     string
	mtime      time.Time
}

func stalePlans(root string) []string {
	live := map[string]bool{}
	chains := map[string]*chainEntry{}
	for _, record := range readJobRecords(root) {
		path, _ := record["__path"].(string)
		jobID, _ := record["jobId"].(string)
		if jobID == "" {
			jobID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		status, _ := record["status"].(string)
		if inFlightStatus[status] {
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
