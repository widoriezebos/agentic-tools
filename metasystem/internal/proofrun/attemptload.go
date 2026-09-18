package proofrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// TestHostLoadEnvironment names the legacy ambient control that production
// sampling deliberately ignores. Binary tests set it to prove it has no effect.
const TestHostLoadEnvironment = "METASYSTEM_TEST_PROOF_HOST_LOAD"

const testHostLoadCommandPrefix = "metasystem-test-proof-host-load="

// loadSeams are the readers the sample is taken from; tests script them.
type loadReaders struct {
	host      func(now time.Time) hostload.Sample
	launchers func(self int64) (int, bool)
	nested    func(self int64) (bool, bool)
	prober    identity.Prober
	pids      func() ([]int64, error)
	parent    func(pid int64) (int64, bool)
}

var loadSeams loadReaders
var commandLoadOptions []loadSampleOption

type loadSampleSettings struct {
	fixtureRaw string
	fixtureSet bool
}

type loadSampleOption func(*loadSampleSettings)

// withTestHostLoad is the explicit same-package test seam for a deterministic
// zero-load host with the named launcher count. Production callers pass no
// option and can never activate the fixture through inherited process state.
func withTestHostLoad(raw string) loadSampleOption {
	return func(settings *loadSampleSettings) {
		settings.fixtureRaw, settings.fixtureSet = raw, true
	}
}

func init() {
	loadSeams = realLoadReaders()
	if command := filepath.Base(os.Args[0]); strings.HasPrefix(command, testHostLoadCommandPrefix) {
		commandLoadOptions = []loadSampleOption{withTestHostLoad(strings.TrimPrefix(command, testHostLoadCommandPrefix))}
	}
}

// TestHostLoadCommandName returns the explicit process name used by command
// fixtures to inject their deterministic sampler. An ordinary process name,
// regardless of its environment, always selects the production readers.
func TestHostLoadCommandName(raw string) string {
	return testHostLoadCommandPrefix + raw
}

func realLoadReaders() loadReaders {
	return loadReaders{
		host: hostload.Read, launchers: countProofLaunchers,
		nested: nestedProofLauncher, prober: identity.KernelProber{},
		pids: identity.AllPids, parent: identity.ParentPid,
	}
}

func testHostLoad(raw string, now time.Time) (hostload.Sample, int, bool) {
	launchers, err := strconv.Atoi(raw)
	if err != nil || launchers < 0 {
		return hostload.Sample{At: now.UTC().Format(time.RFC3339Nano), Detail: "test host load must be a non-negative integer"}, 0, false
	}
	return hostload.Sample{At: now.UTC().Format(time.RFC3339Nano), Available: true, Cores: 18}, launchers, true
}

func sampleNestedProofLauncher(self int64, options ...loadSampleOption) (bool, bool) {
	settings := loadSampleSettings{}
	for _, option := range commandLoadOptions {
		option(&settings)
	}
	for _, option := range options {
		option(&settings)
	}
	if settings.fixtureSet {
		return false, true
	}
	return loadSeams.nested(self)
}

// sampleLoad reads the host, this checkout's other live attempts, and the
// host's other top-level proof launchers at now. The attempt's own launcher
// is the self every path uses (reserve and every finalize alike), so its
// family (the battery's nested bed launchers, its parents) never counts.
func sampleLoad(root, selfAttempt string, launcher int64, now time.Time, options ...loadSampleOption) LoadSample {
	settings := loadSampleSettings{}
	for _, option := range commandLoadOptions {
		option(&settings)
	}
	for _, option := range options {
		option(&settings)
	}
	sample := LoadSample{}
	if settings.fixtureSet {
		sample.Sample, sample.OverlappingHost, sample.OverlapKnown = testHostLoad(settings.fixtureRaw, now)
	} else {
		sample.Sample = loadSeams.host(now)
		if count, known := loadSeams.launchers(launcher); known {
			sample.OverlappingHost, sample.OverlapKnown = count, true
		}
	}
	sample.OverlappingLocal = liveAttemptsOtherThan(root, selfAttempt)
	return sample
}

// SampleLoad exposes the one authoritative load sample used by proof
// admission. Batch ownership uses the same census as an ordinary proof
// attempt; a second sampler would make the two policies disagree.
func SampleLoad(root, selfAttempt string, launcher int64, now time.Time) LoadSample {
	return sampleLoad(root, selfAttempt, launcher, now)
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
	rows, known := readProcessRows()
	if !known {
		return 0, false
	}
	return topLevelLaunchers(rows, self), true
}

func readProcessRows() ([]processRow, bool) {
	pids, err := loadSeams.pids()
	if err != nil {
		return nil, false
	}
	rows := make([]processRow, 0, len(pids))
	for _, pid := range pids {
		row := processRow{pid: pid}
		if parent, ok := loadSeams.parent(pid); ok {
			row.parent = parent
		}
		exact, state, err := loadSeams.prober.Probe(pid)
		if err == nil && state == identity.Alive && exact.ArgvKnown && isProofLauncherArgv(exact.Argv) {
			row.launcher = true
		}
		rows = append(rows, row)
	}
	return rows, true
}

func nestedProofLauncher(self int64) (bool, bool) {
	rows, known := readProcessRows()
	if !known {
		return false, false
	}
	parent, launcher := map[int64]int64{}, map[int64]bool{}
	for _, row := range rows {
		parent[row.pid], launcher[row.pid] = row.parent, row.launcher
	}
	seen := map[int64]bool{}
	for current := parent[self]; current > 0 && !seen[current]; current = parent[current] {
		seen[current] = true
		if launcher[current] {
			return true, true
		}
	}
	return false, true
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
// running a top-level proof battery. An engine test run reserves a top-level
// attempt and runs the groups, so the cap and load attribution count it too.
// The verb pair immediately after the binary keeps shells that merely mention
// the words out of the census.
func isProofLauncherArgv(argv []string) bool {
	return len(argv) >= 3 && filepath.Base(argv[0]) == "metasystem" &&
		(argv[1] == "proof-run" && argv[2] == "launch" || argv[1] == "test" && argv[2] == "run")
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
		if identity.AliveRef(loadSeams.prober, attempt.Launcher.Ref()) == identity.Alive {
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
