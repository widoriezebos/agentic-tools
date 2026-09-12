package proofrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// LoadSample is the host as one proof attempt saw it at a moment: the load
// averages against the core count, the live attempts of this checkout, and
// the proof launchers running anywhere on the host (any seat).
type LoadSample struct {
	hostload.Sample
	OverlappingLocal int  `json:"overlappingLocal"`
	OverlappingHost  int  `json:"overlappingHost"`
	OverlapKnown     bool `json:"overlapKnown"`
}

// AttemptLoad is the attempt's start sample and, once terminal, its end
// sample: a failure under a crowded box is attributable from the record.
type AttemptLoad struct {
	Start LoadSample  `json:"start"`
	End   *LoadSample `json:"end,omitempty"`
}

// LoadAttribution names a terminal taken under load in the record.
const LoadAttribution = "load"

// loadSeams are the readers the sample is taken from; tests script them.
var loadSeams = struct {
	host      func(now time.Time) hostload.Sample
	launchers func(self int64) (int, bool)
}{host: hostload.Read, launchers: countProofLaunchers}

// sampleLoad reads the host, this checkout's other live attempts, and the
// host's other top-level proof launchers at now. The attempt's own launcher
// is the self every path uses (reserve and every finalize alike), so its
// family (the battery's nested bed launchers, its parents) never counts.
func sampleLoad(root, selfAttempt string, launcher int64, now time.Time) LoadSample {
	sample := LoadSample{Sample: loadSeams.host(now)}
	sample.OverlappingLocal = liveAttemptsOtherThan(root, selfAttempt)
	if count, known := loadSeams.launchers(launcher); known {
		sample.OverlappingHost, sample.OverlapKnown = count, true
	}
	return sample
}

// Loaded reports whether the sample shows a crowded host: the one-minute
// load at or above the cores, or another top-level proof launcher on the
// host with the load already at half the cores. One other launcher on an
// otherwise idle box is overlap, recorded as such, not load: the audit's
// gap (a red under load and a red from a defect reading alike) closes only
// if the label discriminates.
func (s LoadSample) Loaded() bool {
	if s.Saturated() {
		return true
	}
	return s.OverlapKnown && s.OverlappingHost > 0 && s.Available && s.Cores > 0 && s.Load1m >= float64(s.Cores)/2
}

// Describe renders the sample for a rationale or a report line.
func (s LoadSample) Describe() string {
	parts := []string{}
	if s.OverlapKnown {
		parts = append(parts, fmt.Sprintf("%d other proof launcher(s) on the host", s.OverlappingHost))
	} else {
		parts = append(parts, "host overlap unknown")
	}
	parts = append(parts, fmt.Sprintf("%d other live attempt(s) in this checkout", s.OverlappingLocal))
	if s.Available {
		parts = append(parts, fmt.Sprintf("load %.2f/%.2f/%.2f on %d cores", s.Load1m, s.Load5m, s.Load15m, s.Cores))
	} else {
		parts = append(parts, "load unavailable")
	}
	return strings.Join(parts, ", ")
}

// processRow is one live process as the launcher count sees it.
type processRow struct {
	pid, parent int64
	launcher    bool
}

// countProofLaunchers counts the top-level proof launchers alive on the
// host outside self's family; a table that cannot be read reports unknown
// rather than zero.
func countProofLaunchers(self int64) (int, bool) {
	pids, err := identity.AllPids()
	if err != nil {
		return 0, false
	}
	rows := make([]processRow, 0, len(pids))
	for _, pid := range pids {
		row := processRow{pid: pid}
		if parent, ok := identity.ParentPid(pid); ok {
			row.parent = parent
		}
		exact, state, err := (identity.KernelProber{}).Probe(pid)
		if err == nil && state == identity.Alive && exact.ArgvKnown && isProofLauncherArgv(exact.Argv) {
			row.launcher = true
		}
		rows = append(rows, row)
	}
	return topLevelLaunchers(rows, self), true
}

// topLevelLaunchers counts the launchers that have no launcher above them
// and are neither self, nor an ancestor of self, nor a descendant of self.
// A battery's nested bed launchers, and the joined launches inside another
// seat's battery, are one launcher each way: the battery's.
func topLevelLaunchers(rows []processRow, self int64) int {
	parent := map[int64]int64{}
	launcher := map[int64]bool{}
	for _, row := range rows {
		parent[row.pid] = row.parent
		if row.launcher {
			launcher[row.pid] = true
		}
	}
	ancestorLauncher := func(pid int64) bool {
		seen := map[int64]bool{}
		for current := parent[pid]; current > 0 && !seen[current]; current = parent[current] {
			seen[current] = true
			if launcher[current] {
				return true
			}
		}
		return false
	}
	descendsFrom := func(pid, ancestor int64) bool {
		seen := map[int64]bool{}
		for current := pid; current > 0 && !seen[current]; current = parent[current] {
			seen[current] = true
			if current == ancestor {
				return true
			}
		}
		return false
	}
	count := 0
	for pid := range launcher {
		if pid == self || descendsFrom(pid, self) || descendsFrom(self, pid) || ancestorLauncher(pid) {
			continue
		}
		count++
	}
	return count
}

// isProofLauncherArgv recognises a metasystem engine, whatever its path,
// running `proof-run launch`: the verb pair right after the binary, so a
// shell or a test that merely mentions the words is never a launcher.
func isProofLauncherArgv(argv []string) bool {
	return len(argv) >= 3 && filepath.Base(argv[0]) == "metasystem" && argv[1] == "proof-run" && argv[2] == "launch"
}

// liveAttemptsOtherThan counts this checkout's attempts without a terminal
// whose launcher is alive, other than self. Unreadable records are skipped:
// one damaged record must not hide the others.
func liveAttemptsOtherThan(root, self string) int {
	attempts, _ := readAttemptsSkippingUnreadable(root)
	count := 0
	for _, attempt := range attempts {
		if attempt.AttemptID == self || attempt.Terminal != nil {
			continue
		}
		if identity.AliveRef(identity.KernelProber{}, attempt.Launcher.Ref()) == identity.Alive {
			count++
		}
	}
	return count
}

// readAttemptsSkippingUnreadable reads every attempt record it can and
// names the ones it cannot, so a reader of the whole set is never blinded by
// one record (2026-09-12: one record whose control root no longer matched
// its path hid every other attempt from the status verb).
func readAttemptsSkippingUnreadable(root string) ([]Attempt, []string) {
	paths, err := filepath.Glob(filepath.Join(attemptsDir(root), "*.json"))
	if err != nil {
		return nil, []string{err.Error()}
	}
	var attempts []Attempt
	var unreadable []string
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		attempt, err := ReadAttempt(root, id)
		if err != nil {
			unreadable = append(unreadable, err.Error())
			continue
		}
		attempts = append(attempts, attempt)
	}
	return attempts, unreadable
}

// PatienceDefect is one line of the patience-defects record: a group that
// is bounded by consumption and still failed while the host was crowded.
// It is filed as the group's defect to fix, never as a reason to serialize
// seats (R-35-m3, R-92-m1e).
type PatienceDefect struct {
	At           string     `json:"at"`
	AttemptID    string     `json:"attemptId"`
	GoalID       string     `json:"goalId"`
	Group        string     `json:"group"`
	Status       string     `json:"status"`
	ProgressRule string     `json:"progressRule"`
	Load         LoadSample `json:"load"`
}

// PatienceDefectsPath is the append-only record of load-attributed failures
// of consumption-bounded groups under a control root.
func PatienceDefectsPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "proof-runs", "patience-defects.jsonl")
}

// consumptionBounded reads a group's progress rule ("cpu-budget/<b>+zero-
// window/<w>[+<platform suffix>]") and reports whether either bound is
// set; a budget or a window of "none" is no bound.
func consumptionBounded(rule string) bool {
	for _, part := range strings.Split(rule, "+") {
		name, value, found := strings.Cut(part, "/")
		if !found {
			continue
		}
		if (name == "cpu-budget" || name == "zero-window") && value != "" && value != "none" {
			return true
		}
	}
	return false
}

// filePatienceDefects appends one line per failed consumption-bounded
// group of a failed attempt whose end sample shows a crowded host. It runs
// after the terminal is committed and its failure is reported, never fatal:
// the record is the fact, the filing is its hand-off.
func filePatienceDefects(attempt Attempt, end LoadSample) ([]PatienceDefect, error) {
	if attempt.TestResult == nil || attempt.Terminal == nil || attempt.Terminal.Result != TerminalFailed || !end.Loaded() {
		return nil, nil
	}
	var filed []PatienceDefect
	for _, group := range attempt.TestResult.Groups {
		if group.Status != "failed" || !consumptionBounded(group.ProgressRule) {
			continue
		}
		filed = append(filed, PatienceDefect{At: end.At, AttemptID: attempt.AttemptID, GoalID: attempt.GoalID,
			Group: group.ID, Status: group.Status, ProgressRule: group.ProgressRule, Load: end})
	}
	if len(filed) == 0 {
		return nil, nil
	}
	path := PatienceDefectsPath(attempt.ControlRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	for _, defect := range filed {
		line, err := json.Marshal(defect)
		if err != nil {
			return nil, err
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			return nil, err
		}
	}
	return filed, nil
}
