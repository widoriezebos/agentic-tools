package census

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// run census: the per-interval scan the watcher runs. It classifies every
// in-scope agent-shaped process as ANNOUNCED (a registered main), CUSTODY
// (owned by a live job), or UNTRACKED (nobody can account for it — surfaced,
// never killed), and writes the verdict. This file holds the FIXTURE-driven
// path — the recorded process table plus recorded state/announcements/custody.
// The production enumeration is a separate binding over the same
// classification core.

// Process is one enumerated process (a fixture row, or one live process).
type Process struct {
	Pid     int64 `json:"pid"`
	PPID    int64 `json:"ppid"`
	PGID    int64 `json:"pgid"`
	Started int64 `json:"pidStartedAt"`
	// StartedExactMicro is the kernel-resolution birth token, ADDITIVE
	// beside the whole-second join key (announcements and custody join
	// on seconds; the exact token exists so a consumer can bind to THE
	// process the census observed — KI-23's acknowledgement). Fixture
	// rows may omit it; enumeration backfills seconds*1e6.
	StartedExactMicro int64 `json:"pidStartedAtExactMicro,omitempty"`
	// The clock-step-immune pair (issue #1); zero/empty on fixture rows.
	StartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID     string `json:"bootId,omitempty"`
	Argv       string `json:"argv"`
	Cwd        string `json:"cwd"`
	CwdError   bool   `json:"cwdError"`
	Alive      bool   `json:"alive"`
}

// InventoryItem is one classified process in the verdict.
type InventoryItem struct {
	Key                    string `json:"key"`
	Class                  string `json:"class"`
	Registry               string `json:"registry"`
	Pid                    int64  `json:"pid"`
	PidStartedAt           int64  `json:"pidStartedAt"`
	PidStartedAtExactMicro int64  `json:"pidStartedAtExactMicro,omitempty"`
	PGID                   int64  `json:"pgid"`
	Runtime                string `json:"runtime"`
	InstanceTag            any    `json:"instanceTag"`
	Cwd                    string `json:"cwd"`
	Scope                  string `json:"scope"`
	Argv                   string `json:"argv"`
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

// MainIDRe and CommandHashRe are the identity grammars of the mains
// directory, shared with lease (review lease-census-4).
var MainIDRe = regexp.MustCompile(`^main-[1-9][0-9]*-[1-9][0-9]*-[0-9a-f]{6}$`)
var CommandHashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

var mainIDRe = MainIDRe
var sha256Re = CommandHashRe

// RunFixtureCensus computes the verdict from a recorded bundle rooted at
// metasystemRoot: the process fixture at processFile, plus state.json,
// mains/, and jobs|missions custody records under metasystemRoot/artifacts.
// The clock is injected so the verdict's timestamps are deterministic. The
// fixture table already records each process's cwd, so no resolver runs.
func RunFixtureCensus(metasystemRoot, repo, processFile, fingerprint string, interval int, now time.Time) (Verdict, error) {
	return runCensus(metasystemRoot, repo, fingerprint, interval, now,
		func(root string) ([]Process, error) { return enumerateFixture(root, processFile) }, nil)
}

// runCensus is the one census orchestration, shared by the fixture and
// production runners: realpath the roots, read and verify the supervision
// snapshot, enumerate the process table and compile the configured
// signatures (either failure zeroes the table under the enumeration label),
// classify every signature-matched process, and assemble the verdict.
// enumerate receives the realpathed metasystem root; resolveCwds, when
// non-nil, patches the matched processes' cwds before classification — the
// production path resolves cwds for matched pids only, the cost rule the
// live process table is careful about.
func runCensus(metasystemRoot, repo, fingerprint string, interval int, now time.Time,
	enumerate func(root string) ([]Process, error), resolveCwds func([]int64) map[int64]cwdResult) (Verdict, error) {
	metasystemRoot = realpath(metasystemRoot)
	repoReal := realpath(repo)
	var errors, diagnostics []string
	// The fixture authority for this walk (agnosticism B1): constructed
	// once from the METASYSTEM root — the configuration owner — never
	// the scan scope (B1 critique finding 3: a fake scope must not
	// authorize fixtures for a non-fake metasystem). A refused
	// construction surfaces as an error and fixtures stay refused.
	var fixtureProbe identity.FixtureProbe
	if authorization, err := fixtureauth.New(metasystemRoot); err != nil {
		errors = append(errors, "fixture-authorization:"+err.Error())
	} else {
		fixtureProbe = authorization.Identity()
	}
	counts := map[string]int{"CUSTODY": 0, "ANNOUNCED": 0, "UNTRACKED": 0}
	var generation *int64
	var stateDigest *string

	if ids, gen, digest, err := readSupervisionSnapshot(metasystemRoot); err != nil {
		errors = append(errors, "supervision-state:"+err.Error())
	} else {
		generation, stateDigest = &gen, &digest
		verifySupervisionSnapshot(ids, fixtureProbe, &errors)
	}

	processes, enumErr := enumerate(metasystemRoot)
	runOwners := loadRunOwners(repoReal, processes, &diagnostics)
	var signatures []Signature
	if enumErr != nil {
		errors = append(errors, "enumeration:"+enumErr.Error())
		processes = nil
	} else if sigs, err := configuredSignatures(metasystemRoot); err != nil {
		errors = append(errors, "enumeration:"+err.Error())
		processes = nil
	} else {
		signatures = sigs
	}

	argvs := make([]string, len(processes))
	for i, p := range processes {
		argvs[i] = p.Argv
	}
	matched := Classify(argvs, signatures)
	var cwds map[int64]cwdResult
	if resolveCwds != nil {
		var matchedPids []int64
		for _, a := range matched {
			matchedPids = append(matchedPids, processes[a.Index].Pid)
		}
		cwds = resolveCwds(matchedPids)
	}

	custody := liveCustody(metasystemRoot)
	announced := announcementsList(metasystemRoot, processes, fixtureProbe, &errors)
	var inventory []InventoryItem
	for _, assignment := range matched {
		process := processes[assignment.Index]
		if resolved, ok := cwds[process.Pid]; ok {
			process.Cwd, process.CwdError = resolved.Cwd, resolved.CwdError
		}
		item, ok := classifyProcess(process, assignment.Runtime, repoReal, custody, announced, runOwners, &errors, &diagnostics)
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
// the classification core is identical.
func classifyProcess(process Process, runtime, repoReal string, custody, announced []identityRecord, runOwners []runOwner, errors, diagnostics *[]string) (InventoryItem, bool) {
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
	classification, registry, tag := classifyOwnership(process, custody, announced, runOwners)
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
		Pid: process.Pid, PidStartedAt: process.Started, PidStartedAtExactMicro: process.StartedExactMicro, PGID: process.PGID,
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

// sameProcessIdentity joins a census-enumerated process to a durable
// record: the clock-step-immune pair decides when both sides carry it
// (issue #1 — the btime-derived second drifts on time-synced guests),
// else the legacy seconds join stands.
func sameProcessIdentity(process Process, item identityRecord) bool {
	if item.Pid != process.Pid {
		return false
	}
	if item.StartTicks > 0 && item.BootID != "" && process.StartTicks > 0 && process.BootID != "" {
		return item.StartTicks == process.StartTicks && item.BootID == process.BootID
	}
	return item.Started == process.Started
}

func classifyOwnership(process Process, custody, announced []identityRecord, runOwners []runOwner) (string, string, any) {
	for _, item := range custody {
		if sameProcessIdentity(process, item) {
			return "CUSTODY", item.Registry, item.InstanceTag
		}
	}
	for _, item := range announced {
		if sameProcessIdentity(process, item) {
			return "ANNOUNCED", item.Registry, item.InstanceTag
		}
	}
	// The RUN custody source (monitor facility, MON-03): group-level, with
	// the strongest proof each custody mode can actually carry. Owned
	// processes ride the CUSTODY class with a RUN tag — the label carries
	// the run id, the enum stays closed.
	for _, owner := range runOwners {
		if process.PGID != owner.Pgid {
			continue
		}
		if owner.Draining {
			return "CUSTODY", owner.Registry, "RUN " + owner.Id + " (draining)"
		}
		if owner.LeaderVerified {
			return "CUSTODY", owner.Registry, "RUN " + owner.Id
		}
	}
	return "UNTRACKED", "none", nil
}

// runOwner is one live run record's custody claim, its leader proof
// precomputed against the same enumerated process table the census
// classifies (wrapped: pid+start+argv-nonce three-factor; adopted:
// pid+start two-factor; draining: pgid plus the record's bounded claim).
type runOwner struct {
	Id             string
	Registry       string
	Pgid           int64
	Draining       bool
	LeaderVerified bool
}

// loadRunOwners reads run records, surfacing every unreadable input.
func loadRunOwners(repo string, processes []Process, diagnostics *[]string) []runOwner {
	store := &run.Store{Root: repo}
	records, unreadable := store.List()
	for _, line := range unreadable {
		*diagnostics = append(*diagnostics, "RUN-RECORD-UNREADABLE "+line)
	}
	var owners []runOwner
	for _, record := range records {
		switch record.Status {
		case run.StatusRunning, run.StatusDraining:
		default:
			continue
		}
		if record.Pgid == nil || record.Pid == nil || record.PidStartedAt == nil {
			continue
		}
		if record.Status == run.StatusDraining && record.EndedAt != nil {
			// Draining custody is BOUNDED (critique finding 6): past the
			// wind-down the survivors surface as UNTRACKED — a stopped
			// watcher must not let a dead run own a reused group forever.
			if ended, err := time.Parse("2006-01-02T15:04:05Z", *record.EndedAt); err == nil {
				if time.Since(ended) > time.Duration(record.WindDownMin)*time.Minute {
					continue
				}
			}
		}
		owner := runOwner{
			Id: record.RunId, Registry: run.RecordPath(repo, record.RunId),
			Pgid: *record.Pgid, Draining: record.Status == run.StatusDraining,
		}
		if !owner.Draining {
			for _, process := range processes {
				if process.Pid != *record.Pid || process.Started != *record.PidStartedAt {
					continue
				}
				if record.Custody == run.CustodyWrapped {
					if process.Argv == "" {
						*diagnostics = append(*diagnostics, "RUN-LEADER-ARGV-UNKNOWN "+record.RunId)
					} else if strings.Contains(process.Argv, record.LaunchNonce) {
						owner.LeaderVerified = true
					}
				} else {
					owner.LeaderVerified = true
				}
				break
			}
		}
		owners = append(owners, owner)
	}
	return owners
}

type identityRecord struct {
	Pid         int64
	Started     int64
	StartTicks  int64  // clock-step-immune pair (issue #1); 0 = legacy record
	BootID      string // "" = legacy record
	InstanceTag string
	Registry    string
}

func enumerateFixture(metasystemRoot, processFile string) ([]Process, error) {
	// The ProcessTableProbe authority (agnosticism B1): fixtureauth owns
	// the one fixture-mode predicate.
	if !fixtureauth.FixtureModeRoot(metasystemRoot) {
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
	for i, row := range rows {
		if row.Argv == "" {
			return nil, fmt.Errorf("process enumeration fixture has unreadable argv")
		}
		if row.StartedExactMicro == 0 {
			rows[i].StartedExactMicro = row.Started * 1_000_000
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

// verifySupervisionSnapshot checks each of owner/watcher/reaper: it must be
// alive (by the fixture identity file when set, else the kernel) and its
// command must carry its tag. A dead identity yields supervision-not-live; a
// live one whose command lacks the tag yields supervision-tag-mismatch.
func verifySupervisionSnapshot(ids map[string]identityRecord, probe identity.FixtureProbe, errors *[]string) {
	for _, name := range []string{"owner", "watcher", "reaper"} {
		id := ids[name]
		if !identityAlive(id.Pid, id.Started, probe) {
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

// identityAlive reports whether pid is alive at expectedStart: the fixture
// identity file (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE) takes precedence as
// the START TIME source when installed, but never overrides kernel death —
// a provably dead pid is dead regardless of its fixture entry (the retired
// python checked pid_exists first for exactly this reason: supervision stop
// verification must be able to observe a stopped process).
func identityAlive(pid, expectedStart int64, probe identity.FixtureProbe) bool {
	if _, state, err := kernelProbe(pid); err == nil && state == probeDead {
		return false
	}
	if entry, ok := probeFixture(probe, pid); ok {
		return entry.StartedAt == expectedStart
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

func announcementsList(metasystemRoot string, processes []Process, probe identity.FixtureProbe, errors *[]string) []identityRecord {
	// Fixture first, kernel second — the same precedence every other identity
	// reader uses. A simulated process table ADDS processes for a census to
	// inventory; it does not declare the rest of the machine dead (treating
	// absence from the fixture as death deleted every real main's
	// announcement while a fixture was installed).
	synthetic := map[int64]Process{}
	for _, process := range processes {
		synthetic[process.Pid] = process
	}
	directory := filepath.Join(metasystemRoot, "artifacts", "agents", "mains")
	os.MkdirAll(directory, 0o755)
	paths, _ := filepath.Glob(filepath.Join(directory, "*.json"))
	sort.Strings(paths)
	var live []identityRecord
	for _, path := range paths {
		name := filepath.Base(path)
		if !IsAnnouncementFile(name) {
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
		if err := ValidateAnnouncementKeys(func(visit func(string) bool) {
			for key := range value {
				visit(key)
			}
		}); err != nil {
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
		// A dead main's announcement is pruned, exactly as the retired
		// python census did: the file is the registry of LIVE mains, and a
		// stale one classifies its checkout's next process as a delegate.
		alive := false
		annTicks, _ := jsonInt(value["pidStartTicks"])
		annBootID, _ := value["bootId"].(string)
		// The pair is EXCLUSIVE when both sides carry it (round-3 R3-1:
		// "seconds OR pair" let a recycled pid with the same second but
		// different ticks keep a stale announcement). PRODUCTION
		// processes land in the enumerated map too (round-2 R2-1);
		// fixture rows keep zero pairs so the seconds rule stands there.
		if process, ok := synthetic[pid]; ok {
			if process.Alive && annTicks > 0 && annBootID != "" &&
				process.StartTicks > 0 && process.BootID != "" {
				alive = process.StartTicks == annTicks && process.BootID == annBootID
			} else {
				alive = process.Alive && process.Started == started
			}
		} else if _, fixtureCovered := probeFixture(probe, pid); fixtureCovered || annTicks == 0 || annBootID == "" {
			// Fixture-covered pids and pairless announcements keep the
			// legacy rule (fixture identities never carry pairs).
			alive = identityAlive(pid, started, probe)
		} else if exact, state, err := (identity.KernelProber{}).Probe(pid); err == nil && state == identity.Alive {
			if exact.StartTicks > 0 && exact.BootID != "" {
				alive = exact.StartTicks == annTicks && exact.BootID == annBootID
			} else {
				alive = exact.StartedAt.Unix() == started
			}
		}
		if !alive {
			if err := os.Remove(path); err != nil {
				*errors = append(*errors, "announcement-prune:"+name+":"+err.Error())
			}
			continue
		}
		tag, _ := value["instanceTag"].(string)
		live = append(live, identityRecord{Pid: pid, Started: started,
			StartTicks: annTicks, BootID: annBootID, InstanceTag: tag, Registry: path})
	}
	return live
}

// configuredSignatures builds the ordered signature list from
// metasystem.runtimes, running each adapter.
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
		matches, excludes := ParseSignatureText(text)
		sig, err := CompileSignature(runtime, matches, excludes)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, nil
}

// ParseSignatureText splits an adapter's `signature` output into its match
// and exclude ERE patterns. Exported so the lease's caller-classification can
// build its delegate signatures from the same one parser.
func ParseSignatureText(text string) (matches, excludes []string) {
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

// processCommand reads a pid's command NATIVELY (kernel argv), for the tag
// check in verifySupervisionSnapshot. A dead or unreadable pid yields an error.
func processCommand(pid int64) (string, error) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return "", fmt.Errorf("no command for pid %d", pid)
	}
	command := strings.Join(exact.Argv, " ")
	if command == "" {
		return "", fmt.Errorf("no command for pid %d", pid)
	}
	return command, nil
}
