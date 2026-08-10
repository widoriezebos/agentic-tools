package census

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// run_census port (process-census.py run_census): the per-interval scan the
// watcher runs. It classifies every in-scope agent-shaped process as
// ANNOUNCED (a registered main), CUSTODY (owned by a live job), or UNTRACKED
// (nobody can account for it — surfaced, never killed), and writes the
// verdict. This file ports the FIXTURE-driven path — the recorded process
// table plus recorded state/announcements/custody — which is exactly the
// differential-replay bundle the conformance harness feeds both engines.
// The production ps/lsof enumeration is a separate binding over the same
// classification core.

// Process is one enumerated process (the census Process class / fixture row).
type Process struct {
	Pid      int64  `json:"pid"`
	PPID     int64  `json:"ppid"`
	PGID     int64  `json:"pgid"`
	Started  int64  `json:"pidStartedAt"`
	Argv     string `json:"argv"`
	Cwd      string `json:"cwd"`
	CwdError bool   `json:"cwdError"`
	Alive    bool   `json:"alive"`
}

// InventoryItem is one classified process in the verdict.
type InventoryItem struct {
	Key          string `json:"key"`
	Class        string `json:"class"`
	Registry     string `json:"registry"`
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	PGID         int64  `json:"pgid"`
	Runtime      string `json:"runtime"`
	InstanceTag  any    `json:"instanceTag"`
	Cwd          string `json:"cwd"`
	Scope        string `json:"scope"`
	Argv         string `json:"argv"`
}

// Verdict is the census output (schemaVersion 2).
type Verdict struct {
	SchemaVersion    int             `json:"schemaVersion"`
	Writer           string          `json:"writer"`
	Verdict          string          `json:"verdict"`
	CompletedAt      string          `json:"completedAt"`
	CompletedAtEpoch int64           `json:"completedAtEpoch"`
	DurationMs       int64           `json:"durationMs"`
	IntervalSec      int             `json:"intervalSec"`
	Fingerprint      string          `json:"fingerprint"`
	Generation       *int64          `json:"generation"`
	StateDigest      *string         `json:"stateDigest"`
	Counts           map[string]int  `json:"counts"`
	Inventory        []InventoryItem `json:"inventory"`
	Diagnostics      []string        `json:"diagnostics"`
	Errors           []string        `json:"errors"`
}

var liveStatuses = map[string]bool{
	"pending-setup": true, "pending": true, "running": true,
}

var mainIDRe = regexp.MustCompile(`^main-[1-9][0-9]*-[1-9][0-9]*-[0-9a-f]{6}$`)
var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

// RunFixtureCensus computes the verdict from a recorded bundle rooted at
// metasystemRoot: the process fixture at processFile, plus state.json,
// mains/, and jobs|missions custody records under metasystemRoot/artifacts.
// The clock is injected so the verdict's timestamps are deterministic for
// conformance. This is the census core; the production path substitutes ps
// enumeration and lsof cwd resolution for the fixture, over the same logic.
func RunFixtureCensus(metasystemRoot, repo, processFile, fingerprint string, interval int, now time.Time) (Verdict, error) {
	metasystemRoot = realpath(metasystemRoot)
	repoReal := realpath(repo)
	var errors, diagnostics []string
	counts := map[string]int{"CUSTODY": 0, "ANNOUNCED": 0, "UNTRACKED": 0}
	var generation *int64
	var stateDigest *string

	if ids, gen, digest, err := readSupervisionSnapshot(metasystemRoot); err != nil {
		errors = append(errors, "supervision-state:"+err.Error())
		_ = ids
	} else {
		generation, stateDigest = &gen, &digest
		verifySupervisionSnapshot(ids, &errors)
	}

	processes, enumErr := enumerateFixture(metasystemRoot, processFile)
	var signatures []Signature
	if enumErr != nil {
		errors = append(errors, "enumeration:"+enumErr.Error())
		processes = nil
	} else {
		sigs, err := configuredSignatures(metasystemRoot)
		if err != nil {
			errors = append(errors, "enumeration:"+err.Error())
			processes = nil
		} else {
			signatures = sigs
		}
	}

	custody := liveCustody(metasystemRoot)
	announced := announcementsList(metasystemRoot, &errors)

	var inventory []InventoryItem
	argvs := make([]string, len(processes))
	for i, p := range processes {
		argvs[i] = p.Argv
	}
	for _, assignment := range Classify(argvs, signatures) {
		process := processes[assignment.Index]
		item, ok := classifyProcess(process, assignment.Runtime, repoReal, custody, announced, &errors, &diagnostics)
		if !ok {
			continue
		}
		counts[item.Class]++
		inventory = append(inventory, item)
	}
	return assembleVerdict(verdictLabelFor(errors), fingerprint, interval, generation, stateDigest,
		counts, inventory, diagnostics, errors, now), nil
}

// classifyProcess applies the census's per-process decision: liveness, scope
// (cwd or argv below the repo), and ownership (CUSTODY/ANNOUNCED/UNTRACKED).
// It appends diagnostics/errors and returns the inventory item, or ok=false
// when the process is skipped. Shared by the fixture and production paths so
// the classification core is identical (and conformance-covered).
func classifyProcess(process Process, runtime, repoReal string, custody, announced []identityRecord, errors, diagnostics *[]string) (InventoryItem, bool) {
	if !process.Alive {
		*diagnostics = append(*diagnostics, fmt.Sprintf("RACED-EXIT pid=%d", process.Pid))
		return InventoryItem{}, false
	}
	if process.Started < 0 {
		*errors = append(*errors, fmt.Sprintf("start-time-unreadable:%d", process.Pid))
		return InventoryItem{}, false
	}
	resolvedCwd := ""
	if process.Cwd != "" {
		resolvedCwd = realpath(process.Cwd)
	}
	cwdInScope := resolvedCwd != "" && PathBelow(resolvedCwd, repoReal)
	namedPaths, err := ArgvPaths(process.Argv, resolvedCwd)
	if err != nil {
		*errors = append(*errors, fmt.Sprintf("argv-unreadable:%d:%s", process.Pid, err))
		return InventoryItem{}, false
	}
	argvInScope := false
	for _, path := range namedPaths {
		if PathBelow(path, repoReal) {
			argvInScope = true
			break
		}
	}
	if process.CwdError {
		*diagnostics = append(*diagnostics, fmt.Sprintf("UNRESOLVED-CWD pid=%d argv=%s", process.Pid, process.Argv))
	}
	if !cwdInScope && !argvInScope {
		if process.CwdError {
			*errors = append(*errors, fmt.Sprintf("scope-unresolved:%d", process.Pid))
		}
		return InventoryItem{}, false
	}
	classification, registry, tag := classifyOwnership(process, custody, announced)
	cwd := resolvedCwd
	if cwd == "" {
		cwd = "UNRESOLVED-CWD"
	}
	scope := "argv"
	if cwdInScope {
		scope = "cwd"
	}
	return InventoryItem{
		Key:   fmt.Sprintf("%s|%d|%d", registry, process.Pid, process.Started),
		Class: classification, Registry: registry,
		Pid: process.Pid, PidStartedAt: process.Started, PGID: process.PGID,
		Runtime: runtime, InstanceTag: tag, Cwd: cwd, Scope: scope, Argv: process.Argv,
	}, true
}

func verdictLabelFor(errors []string) string {
	if len(errors) > 0 {
		return "CENSUS-FAILED"
	}
	return "SUCCESS"
}

func assembleVerdict(label, fingerprint string, interval int, generation *int64, stateDigest *string,
	counts map[string]int, inventory []InventoryItem, diagnostics, errors []string, now time.Time) Verdict {
	sort.SliceStable(inventory, func(i, j int) bool { return inventory[i].Pid < inventory[j].Pid })
	if inventory == nil {
		inventory = []InventoryItem{}
	}
	if diagnostics == nil {
		diagnostics = []string{}
	}
	if errors == nil {
		errors = []string{}
	}
	completed := now.UTC()
	return Verdict{
		SchemaVersion: 2, Writer: "watch-background-jobs.sh", Verdict: label,
		CompletedAt:      completed.Format("2006-01-02T15:04:05Z"),
		CompletedAtEpoch: completed.Unix(),
		DurationMs:       0,
		IntervalSec:      interval, Fingerprint: fingerprint,
		Generation: generation, StateDigest: stateDigest,
		Counts: counts, Inventory: inventory, Diagnostics: diagnostics, Errors: errors,
	}
}

func classifyOwnership(process Process, custody, announced []identityRecord) (string, string, any) {
	for _, item := range custody {
		if item.Pid == process.Pid && item.Started == process.Started {
			return "CUSTODY", item.Registry, item.InstanceTag
		}
	}
	for _, item := range announced {
		if item.Pid == process.Pid && item.Started == process.Started {
			return "ANNOUNCED", item.Registry, item.InstanceTag
		}
	}
	return "UNTRACKED", "none", nil
}

type identityRecord struct {
	Pid         int64
	Started     int64
	InstanceTag string
	Registry    string
}

func enumerateFixture(metasystemRoot, processFile string) ([]Process, error) {
	if config.ConfValue(filepath.Join(metasystemRoot, "metasystem.conf"), "metasystem.runtimes", "") != "fake" {
		return nil, fmt.Errorf("METASYSTEM_CENSUS_PROCESS_FILE is allowed only when metasystem.runtimes=fake")
	}
	data, err := os.ReadFile(processFile)
	if err != nil {
		return nil, fmt.Errorf("process enumeration fixture is unreadable: %w", err)
	}
	var rows []Process
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("process enumeration fixture is unreadable: %w", err)
	}
	for _, row := range rows {
		if row.Argv == "" {
			return nil, fmt.Errorf("process enumeration fixture has unreadable argv")
		}
	}
	return rows, nil
}

func readSupervisionSnapshot(metasystemRoot string) (map[string]identityRecord, int64, string, error) {
	statePath := filepath.Join(metasystemRoot, "artifacts", "agents", "supervision", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, 0, "", fmt.Errorf("supervision state is unavailable: %w", err)
	}
	var state struct {
		Generation *int64 `json:"generation"`
		Components map[string]struct {
			Pid         *int64  `json:"pid"`
			Started     *int64  `json:"pidStartedAt"`
			InstanceTag *string `json:"instanceTag"`
		} `json:"components"`
		Owner *struct {
			Pid         *int64  `json:"pid"`
			Started     *int64  `json:"pidStartedAt"`
			InstanceTag *string `json:"instanceTag"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, 0, "", fmt.Errorf("supervision state is unavailable: %w", err)
	}
	if state.Generation == nil || *state.Generation < 1 {
		return nil, 0, "", fmt.Errorf("supervision state has an invalid generation")
	}
	if len(state.Components) != 2 || state.Components["watcher"].Pid == nil ||
		state.Components["reaper"].Pid == nil || state.Owner == nil {
		return nil, 0, "", fmt.Errorf("supervision state has no complete instance set")
	}
	ids := map[string]identityRecord{}
	add := func(name string, pid, started *int64, tag *string) error {
		if pid == nil || *pid < 1 || started == nil || *started < 1 || tag == nil || *tag == "" {
			return fmt.Errorf("supervision state has invalid %s identity", name)
		}
		ids[name] = identityRecord{Pid: *pid, Started: *started, InstanceTag: *tag}
		return nil
	}
	if err := add("owner", state.Owner.Pid, state.Owner.Started, state.Owner.InstanceTag); err != nil {
		return nil, 0, "", err
	}
	for _, name := range []string{"watcher", "reaper"} {
		c := state.Components[name]
		if err := add(name, c.Pid, c.Started, c.InstanceTag); err != nil {
			return nil, 0, "", err
		}
	}
	sum := sha256.Sum256(data)
	return ids, *state.Generation, hex.EncodeToString(sum[:]), nil
}

// verifySupervisionSnapshot ports verify_supervision_snapshot: each of
// owner/watcher/reaper must be alive (by the fixture identity file when set,
// else the kernel) and its command must carry its tag. A dead identity yields
// supervision-not-live; a live one whose command lacks the tag yields
// supervision-tag-mismatch.
func verifySupervisionSnapshot(ids map[string]identityRecord, errors *[]string) {
	for _, name := range []string{"owner", "watcher", "reaper"} {
		id := ids[name]
		if !identityAlive(id.Pid, id.Started) {
			*errors = append(*errors, fmt.Sprintf("supervision-not-live:%s:pid=%d", name, id.Pid))
			continue
		}
		command, err := processCommand(id.Pid)
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("supervision-command-unavailable:%s:pid=%d", name, id.Pid))
			continue
		}
		if !strings.Contains(command, id.InstanceTag) {
			*errors = append(*errors, fmt.Sprintf("supervision-tag-mismatch:%s:pid=%d", name, id.Pid))
		}
	}
}

// identityAlive ports identity_alive: the fixture identity file
// (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE) takes precedence when installed,
// else the kernel start time must equal expected.
func identityAlive(pid, expectedStart int64) bool {
	if fixture := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"); fixture != "" {
		if data, err := os.ReadFile(fixture); err == nil {
			var table map[string]struct {
				Started int64 `json:"pidStartedAt"`
			}
			if json.Unmarshal(data, &table) == nil {
				if entry, ok := table[fmt.Sprint(pid)]; ok {
					return entry.Started == expectedStart
				}
			}
		}
	}
	exact, state, err := kernelProbe(pid)
	if err != nil || state != probeAlive {
		return false
	}
	return exact == expectedStart
}

func liveCustody(metasystemRoot string) []identityRecord {
	agents := filepath.Join(metasystemRoot, "artifacts", "agents")
	type source struct {
		glob          string
		requireStatus bool
	}
	var records []identityRecord
	globs := []source{
		{filepath.Join(agents, "jobs", "*.json"), true},
		{filepath.Join(agents, "missions", "runners", "*.json"), true},
		{filepath.Join(agents, "missions", "*", "turns", "*", "*.json"), false},
	}
	for _, src := range globs {
		paths, _ := filepath.Glob(src.glob)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var value map[string]any
			if json.Unmarshal(data, &value) != nil {
				continue
			}
			status, _ := value["status"].(string)
			if src.requireStatus && !liveStatuses[status] {
				continue
			}
			if !src.requireStatus && !liveStatuses[status] && value["outcome"] != "running" {
				continue
			}
			recordTag, _ := value["instanceTag"].(string)
			if recordTag == "" {
				continue
			}
			candidates := []map[string]any{value}
			if children, ok := value["custodyProcesses"].([]any); ok {
				for _, child := range children {
					if m, ok := child.(map[string]any); ok {
						candidates = append(candidates, m)
					}
				}
			}
			for _, candidate := range candidates {
				pid, pidOK := jsonInt(candidate["pid"])
				started, startedOK := jsonInt(candidate["pidStartedAt"])
				tag, _ := candidate["instanceTag"].(string)
				if pidOK && startedOK && tag == recordTag {
					records = append(records, identityRecord{Pid: pid, Started: started, InstanceTag: tag, Registry: path})
				}
			}
		}
	}
	return records
}

func announcementsList(metasystemRoot string, errors *[]string) []identityRecord {
	directory := filepath.Join(metasystemRoot, "artifacts", "agents", "mains")
	os.MkdirAll(directory, 0o755)
	paths, _ := filepath.Glob(filepath.Join(directory, "*.json"))
	sort.Strings(paths)
	skip := map[string]bool{
		"worktree-lease.json": true, "worktree-commit-token.json": true, "reaped-after-claim.json": true,
	}
	expected := map[string]bool{
		"sessionId": true, "pid": true, "pidStartedAt": true, "pgid": true,
		"runtime": true, "instanceTag": true, "announcedAt": true,
	}
	optional := map[string]bool{"mainId": true, "commandHash": true, "ownerLineage": true}
	var live []identityRecord
	for _, path := range paths {
		name := filepath.Base(path)
		if skip[name] || len(name) > len(".protocol-cursor.json") &&
			name[len(name)-len(".protocol-cursor.json"):] == ".protocol-cursor.json" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("announcement-unreadable:%s:%s", name, err))
			continue
		}
		var value map[string]any
		if json.Unmarshal(data, &value) != nil {
			*errors = append(*errors, "announcement-schema:"+name)
			continue
		}
		ok := true
		for key := range expected {
			if _, present := value[key]; !present {
				ok = false
			}
		}
		for key := range value {
			if !expected[key] && !optional[key] {
				ok = false
			}
		}
		if !ok {
			*errors = append(*errors, "announcement-schema:"+name)
			continue
		}
		pid, pidOK := jsonInt(value["pid"])
		started, startedOK := jsonInt(value["pidStartedAt"])
		if !pidOK || !startedOK {
			*errors = append(*errors, "announcement-identity:"+name)
			continue
		}
		mainID, hasMain := value["mainId"].(string)
		digest, hasDigest := value["commandHash"].(string)
		if value["mainId"] != nil || value["commandHash"] != nil {
			if !hasMain || !mainIDRe.MatchString(mainID) || !hasDigest || !sha256Re.MatchString(digest) {
				*errors = append(*errors, "announcement-identity:"+name)
				continue
			}
		}
		tag, _ := value["instanceTag"].(string)
		live = append(live, identityRecord{Pid: pid, Started: started, InstanceTag: tag, Registry: path})
	}
	return live
}

// configuredSignatures builds the ordered signature list from
// metasystem.runtimes, running each adapter — the port of
// configured_signatures.
func configuredSignatures(metasystemRoot string) ([]Signature, error) {
	confPath := filepath.Join(metasystemRoot, "metasystem.conf")
	selected := splitRuntimes(config.ConfValue(confPath, "metasystem.runtimes", ""))
	if len(selected) == 0 {
		return nil, fmt.Errorf("%s lists no metasystem.runtimes", confPath)
	}
	var out []Signature
	for _, runtime := range selected {
		adapter := filepath.Join(metasystemRoot, "scripts", "agents", "adapters", runtime+".sh")
		info, err := os.Stat(adapter)
		if err != nil || info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("metasystem.runtimes names %q, but its signature adapter is missing or not executable: %s", runtime, adapter)
		}
		text, err := SignatureText(adapter)
		if err != nil {
			return nil, err
		}
		matches, excludes := parseSignatureText(text)
		sig, err := CompileSignature(runtime, matches, excludes)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, nil
}

func parseSignatureText(text string) (matches, excludes []string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line == "" {
			continue
		}
		verb, pattern, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		if verb == "match" {
			matches = append(matches, pattern)
		} else if verb == "exclude" {
			excludes = append(excludes, pattern)
		}
	}
	return matches, excludes
}

func jsonInt(v any) (int64, bool) {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f), true
	}
	return 0, false
}

type probeState int

const (
	probeAlive probeState = iota
	probeDead
	probeUnknown
)

// kernelProbe returns a pid's exact start second and three-way liveness.
func kernelProbe(pid int64) (int64, probeState, error) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	switch state {
	case identity.Alive:
		return exact.StartedAt.Unix(), probeAlive, nil
	case identity.Dead:
		return 0, probeDead, nil
	default:
		return 0, probeUnknown, err
	}
}

// processCommand reads a pid's command via ps (the tag check in
// verify_supervision_snapshot). A dead pid yields an error.
func processCommand(pid int64) (string, error) {
	out, err := exec.Command("ps", "-p", fmt.Sprint(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	command := strings.TrimSpace(string(out))
	if command == "" {
		return "", fmt.Errorf("no command for pid %d", pid)
	}
	return command, nil
}
