// Package run owns the run record: tracked long-running work with
// terminal-state watching.
// A run is NON-JOB work — a suite, a cohort, any detached process —
// registered as a record with kernel identity, watched by the standing
// supervision watcher, wakeable through a blocking waiter, and spoken for
// by the turn verdict.
//
// Discipline mirrors the delegate-job record layer: one directory lock
// spans every mutation (lease run-held deliberately does not fence HUMAN
// callers, which is why this lock exists), per-record compare-and-swap
// over (status, generation), legal transitions only, refusals loud, and
// three-way liveness in which Unknown concludes nothing.
package run

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Bounds, every one at the source.
const (
	MaxIdBytes       = 64
	MaxDisplayBytes  = 200
	MaxLogPathBytes  = 512
	MaxPatternBytes  = 256
	MaxExpectBytes   = 240
	MinStaleMin      = 1
	MaxStaleMin      = 1440
	MinWindDownMin   = 1
	MaxWindDownMin   = 120
	DefaultWindDown  = 10
	LaunchFenceMin   = 2
	PruneAgeDays     = 14
	PatternTailBytes = 64 * 1024
)

// Statuses and the custody / evidence / verdict enums.
const (
	StatusLaunching    = "launching"
	StatusRunning      = "running"
	StatusDraining     = "draining"
	StatusGreen        = "green"
	StatusRed          = "red"
	StatusEndedUnknown = "ended-unknown"
	StatusLaunchFailed = "launch-failed"

	CustodyWrapped           = "wrapped"
	CustodyAdoptedVerified   = "adopted-verified"
	CustodyAdoptedUnverified = "adopted-unverified"

	EvidenceSidecar = "exit-sidecar"
	EvidencePattern = "pattern"
	EvidenceNone    = "none"
)

var runIdRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var nonceRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Evidence is how a conclusion is proven.
type Evidence struct {
	Mode           string `json:"mode"`
	VerdictPattern string `json:"verdictPattern,omitempty"`
}

// Expect carries the continuation notes — plain text for the next
// reader, never executable.
type Expect struct {
	Green   string `json:"green"`
	Red     string `json:"red"`
	Hung    string `json:"hung"`
	Unknown string `json:"unknown"`
}

const (
	AssumptionMatch       = "match"
	AssumptionDrift       = "drift"
	AssumptionUnavailable = "unavailable"
	BreakerClosed         = "CLOSED"
	BreakerExhausted      = "EXHAUSTED"
	BreakerAssumption     = "ASSUMPTION_FAILED"
)

// AssumptionObservation is filled by terminalization and rechecked by the
// steward. Unavailable evidence is a typed failure, never a healthy default.
type AssumptionObservation struct {
	ObservedAt        string   `json:"observedAt"`
	Platform          string   `json:"platform"`
	ToolchainIdentity string   `json:"toolchainIdentity"`
	SurfaceDigest     string   `json:"surfaceDigest"`
	ActiveJobs        uint64   `json:"activeJobs"`
	DurationSeconds   uint64   `json:"durationSeconds"`
	AssumptionState   string   `json:"assumptionState"`
	DriftedFields     []string `json:"driftedFields"`
}

// GovernedAttempt binds a recurring run to one immutable obligation and the
// existing four-field budget. Admission and terminalization write the two
// halves of this same record.
type GovernedAttempt struct {
	GoalRevision         uint64                           `json:"goalRevision"`
	ObligationRevision   uint64                           `json:"obligationRevision"`
	WeightGeneration     *uint64                          `json:"weightGeneration"`
	Recurrence           governance.RecurrenceClass       `json:"recurrence"`
	ExecutionCostMinutes uint64                           `json:"executionCostMinutes"`
	ObservedCostMinutes  *uint64                          `json:"observedCostMinutes,omitempty"`
	AttemptOrdinal       uint64                           `json:"attemptOrdinal"`
	ReservedBefore       uint64                           `json:"reservedBefore"`
	Budget               goalbudget.Budget                `json:"budget"`
	BudgetStartedAt      string                           `json:"budgetStartedAt"`
	BudgetEpoch          *uint64                          `json:"budgetEpoch,omitempty"`
	CorrelationPolicy    string                           `json:"correlationPolicy"`
	ExpectedAssumptions  governance.ObligationAssumptions `json:"expectedAssumptions"`
	AdmissionDecision    governance.ConsequenceDecision   `json:"admissionDecision"`
	Observation          *AssumptionObservation           `json:"observation,omitempty"`
	Breaker              string                           `json:"breaker"`
	Exhausted            bool                             `json:"exhausted"`
	ExhaustionReason     string                           `json:"exhaustionReason,omitempty"`
	RetroDebtRaised      bool                             `json:"retroDebtRaised,omitempty"`
}

// TerminalGovernedRunIDReuseError is the stable refusal outcome for an ID
// that has already carried a terminal governed attempt. It remains detectable
// after the corresponding run evidence is pruned.
type TerminalGovernedRunIDReuseError struct {
	RunID  string
	Record string
}

func (err *TerminalGovernedRunIDReuseError) Error() string {
	return fmt.Sprintf("REFUSED-TERMINAL-GOVERNED-ID: run id %s already owns terminal governed state in %s", err.RunID, err.Record)
}

// GovernedAdmissionRequest and Result keep policy outside this package. The
// command layer wires dispatch's accepted-goal evaluator into the run store.
type GovernedAdmissionRequest struct {
	GoalID             string
	ObligationRevision uint64
	StandingShared     bool
}

type GovernedAdmissionResult struct {
	Attempt GovernedAttempt
}

// Record is the run record, schema v1.
type Record struct {
	SchemaVersion int              `json:"schemaVersion"`
	RunId         string           `json:"runId"`
	Kind          string           `json:"kind"`
	Display       string           `json:"display"`
	Custody       string           `json:"custody"`
	Generation    int              `json:"generation"`
	Pid           *int64           `json:"pid"`
	PidStartedAt  *int64           `json:"pidStartedAt"`
	PidStartTicks int64            `json:"pidStartTicks,omitempty"`
	BootID        string           `json:"bootId,omitempty"`
	Pgid          *int64           `json:"pgid"`
	LaunchNonce   string           `json:"launchNonce"`
	Log           string           `json:"log"`
	StartedAt     string           `json:"startedAt"`
	MainId        *string          `json:"mainId"`
	OwnerLineage  *string          `json:"ownerLineage"`
	ClaimEpoch    *int64           `json:"claimEpoch"`
	SessionId     string           `json:"sessionId"`
	GoalId        string           `json:"goalId"`
	Governed      *GovernedAttempt `json:"governed,omitempty"`
	StaleAfterMin int              `json:"staleAfterMin"`
	HungSince     *string          `json:"hungSince"`
	WindDownMin   int              `json:"windDownMin"`
	Evidence      Evidence         `json:"evidence"`
	Expect        Expect           `json:"expect"`
	Status        string           `json:"status"`
	// ProvisionalVerdict freezes at draining ENTRY: adopted-pattern
	// evidence is evaluated once there, so descendants writing the log
	// later cannot change it.
	ProvisionalVerdict *string `json:"provisionalVerdict"`
	// TerminalSeq is assigned under the runs lock at terminalization:
	// the total order the green cursor rides.
	TerminalSeq *int64  `json:"terminalSeq"`
	Acked       bool    `json:"acked"`
	Error       *string `json:"error"`
	ExitCode    *int64  `json:"exitCode"`
	// EndedAt stamps at draining entry: the leader's death IS the run's
	// end; the wind-down clock runs from it.
	EndedAt *string `json:"endedAt"`
}

// Terminal reports whether a status is final.
func Terminal(status string) bool {
	switch status {
	case StatusGreen, StatusRed, StatusEndedUnknown, StatusLaunchFailed:
		return true
	}
	return false
}

// Store binds one checkout. Prober and Now are test seams; CurrentEpoch
// is the in-lock lease-epoch reader: a
// mutation carrying lease coordinates must prove its epoch is STILL the
// checkout's epoch inside the runs lock — authorization at the command
// layer is point-in-time and a stalled child can outlive its main.
type Store struct {
	Root         string
	Prober       identity.Prober
	Now          func() time.Time
	CurrentEpoch func() (*int64, bool)
	// Getpgid is the kernel process-group reader, a seam for tests
	// whose recorded pids are synthetic.
	Getpgid func(pid int64) (int64, error)
	// AllPids is the kernel process-table reader, a seam for tests whose
	// recorded pids are synthetic; production leaves it nil. The table and
	// group reader describe one world, so tests override both or neither.
	AllPids func() ([]int64, error)
	// GroupPresent is the kernel's direct process-group existence proof.
	// Production leaves it nil; tests can pin absent, present, or unknown.
	GroupPresent func(pgid int64) (present, certain bool)
	// AdmitGoverned is mandatory only for a standing shared run or a run that
	// explicitly names an obligation revision.
	AdmitGoverned   func(GovernedAdmissionRequest) (GovernedAdmissionResult, error)
	ObserveGoverned func(*Record, time.Time) AssumptionObservation
}

func (s *Store) getpgid(pid int64) (int64, error) {
	if s.Getpgid != nil {
		return s.Getpgid(pid)
	}
	pg, err := unix.Getpgid(int(pid))
	return int64(pg), err
}

func (s *Store) allPids() ([]int64, error) {
	if s.AllPids != nil {
		return s.AllPids()
	}
	return identity.AllPids()
}

func (s *Store) groupPresent(pgid int64) (present, certain bool) {
	if s.GroupPresent != nil {
		return s.GroupPresent(pgid)
	}
	switch err := unix.Kill(int(-pgid), 0); err {
	case nil, unix.EPERM:
		return true, true
	case unix.ESRCH:
		return false, true
	default:
		return false, false
	}
}

// checkEpoch refuses a stale-epoch mutation; callers hold the runs lock.
// HUMAN callers (nil epoch) are exempt by contract.
func (s *Store) checkEpoch(caller Caller) error {
	if caller.ClaimEpoch == nil {
		return nil
	}
	if s.CurrentEpoch == nil {
		return nil // no seam wired: local library use (tests) — the CLI always wires it
	}
	current, ok := s.CurrentEpoch()
	if !ok || current == nil {
		return fmt.Errorf("cannot verify the lease epoch; refusing the mutation")
	}
	if *current != *caller.ClaimEpoch {
		return fmt.Errorf("the caller's lease epoch %d is stale (current %d); refusing", *caller.ClaimEpoch, *current)
	}
	return nil
}

func (s *Store) prober() identity.Prober {
	if s.Prober != nil {
		return s.Prober
	}
	return identity.KernelProber{}
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) nowISO() string { return s.now().UTC().Format("2006-01-02T15:04:05Z") }

// Dir is the checkout's runs directory.
func Dir(root string) string { return filepath.Join(root, "artifacts", "agents", "runs") }

// RecordPath returns a run's record file.
func RecordPath(root, id string) string { return filepath.Join(Dir(root), id+".json") }

// SidecarPath is the generation-scoped exit sidecar (excluded from the
// record glob by its .g<n>.exit.json suffix).
func SidecarPath(root, id string, generation int) string {
	return filepath.Join(Dir(root), fmt.Sprintf("%s.g%d.exit.json", id, generation))
}

// RecordFiles lists record files, excluding sidecars by suffix grammar.
func RecordFiles(root string) []string {
	paths, _ := filepath.Glob(filepath.Join(Dir(root), "*.json"))
	var out []string
	for _, path := range paths {
		if strings.Contains(filepath.Base(path), ".g") && strings.HasSuffix(path, ".exit.json") {
			continue
		}
		out = append(out, path)
	}
	return out
}

// withLock runs fn under the runs-directory lock — the operation-spanning
// fence for EVERY mutation.
func (s *Store) withLock(fn func() error) error {
	dir := Dir(s.Root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("runs lock cannot be opened: %w", err)
	}
	defer f.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("runs lock is busy; refusing after 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

// nextTerminalSeq allocates from the counter file — callers hold the lock.
func (s *Store) nextTerminalSeq() (int64, error) {
	path := filepath.Join(Dir(s.Root), ".terminal-seq")
	data, err := os.ReadFile(path)
	seq := int64(0)
	if err == nil {
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if parseErr != nil || parsed < 0 {
			return 0, fmt.Errorf("terminal-seq counter is malformed; refusing to reset the total order")
		}
		seq = parsed
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	seq++
	if err := atomicWrite(path, []byte(strconv.FormatInt(seq, 10)+"\n")); err != nil {
		return 0, err
	}
	return seq, nil
}

// Read returns a record without judging it. Missing is (nil, nil).
func (s *Store) Read(id string) (*Record, error) {
	data, err := os.ReadFile(RecordPath(s.Root, id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("run record %s is unparsable: %v", id, err)
	}
	if problems := Validate(&record); len(problems) > 0 {
		return nil, fmt.Errorf("run record %s is structurally illegal: %s", id, problems[0])
	}
	return &record, nil
}

// cas rewrites a record only when its (status, generation) still match —
// callers hold the lock. Refusals name both sides and emit run-cas-refused
// through the caller's event hook.
func (s *Store) cas(id, expectStatus string, expectGeneration int, mutate func(*Record) error) (*Record, error) {
	record, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("no run record %s", id)
	}
	if record.Status != expectStatus || record.Generation != expectGeneration {
		s.emit("run-cas-refused", map[string]string{
			"runId":    id,
			"expected": fmt.Sprintf("%s.g%d", expectStatus, expectGeneration),
			"found":    fmt.Sprintf("%s.g%d", record.Status, record.Generation),
		})
		return nil, fmt.Errorf("run %s is %s (generation %d), not %s (generation %d)",
			id, record.Status, record.Generation, expectStatus, expectGeneration)
	}
	if err := mutate(record); err != nil {
		return nil, err
	}
	if err := s.write(record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *Store) write(record *Record) error {
	if problems := Validate(record); len(problems) > 0 {
		return fmt.Errorf("refusing to write an illegal run record: %s", problems[0])
	}
	data, err := json.MarshalIndent(record, "", " ")
	if err != nil {
		return err
	}
	return atomicWrite(RecordPath(s.Root, record.RunId), append(data, '\n'))
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".run-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// Validate enforces every bound and structural rule.
func Validate(r *Record) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if !runIdRe.MatchString(r.RunId) {
		add("runId must be kebab ≤%d bytes: %q", MaxIdBytes, r.RunId)
	}
	switch r.Kind {
	case "suite", "cohort", "custom":
	default:
		add("kind must be suite|cohort|custom: %q", r.Kind)
	}
	if len(r.Display) > MaxDisplayBytes {
		add("display exceeds %d bytes", MaxDisplayBytes)
	}
	if r.Governed != nil {
		g := r.Governed
		if r.GoalId == "" || g.GoalRevision == 0 || g.ObligationRevision == 0 || g.WeightGeneration == nil || g.AttemptOrdinal == 0 ||
			g.ExecutionCostMinutes == 0 || g.BudgetStartedAt == "" || g.Budget.Validate() != nil {
			add("governed attempt is missing its goal, revisions, ordinal, cost, budget, or start")
		}
		if g.Recurrence != governance.SingleExperiment && g.Recurrence != governance.StandingSharedProcess {
			add("governed recurrence enum violated: %q", g.Recurrence)
		}
		if g.BudgetEpoch != nil && (g.WeightGeneration == nil || *g.WeightGeneration <= *g.BudgetEpoch) {
			add("governed attempt weight generation does not follow its budget epoch")
		}
		if err := g.ExpectedAssumptions.Validate(); err != nil {
			add("governed expected assumptions are invalid: %v", err)
		}
		if g.Breaker != BreakerClosed && g.Breaker != BreakerExhausted && g.Breaker != BreakerAssumption {
			add("governed breaker enum violated: %q", g.Breaker)
		}
		if Terminal(r.Status) && g.Observation == nil {
			add("terminal governed attempt requires an assumption observation")
		}
		if g.Observation != nil {
			if g.Observation.AssumptionState != AssumptionMatch && g.Observation.AssumptionState != AssumptionDrift &&
				g.Observation.AssumptionState != AssumptionUnavailable {
				add("governed assumption-state enum violated: %q", g.Observation.AssumptionState)
			}
			if _, err := time.Parse(time.RFC3339, g.Observation.ObservedAt); err != nil {
				add("governed observation timestamp is invalid")
			}
		}
	}
	switch r.Custody {
	case CustodyWrapped, CustodyAdoptedVerified, CustodyAdoptedUnverified:
	default:
		add("custody enum violated: %q", r.Custody)
	}
	if r.Generation < 1 {
		add("generation starts at 1")
	}
	if !nonceRe.MatchString(r.LaunchNonce) {
		add("launchNonce must be 32 hex")
	}
	if len(r.Log) > MaxLogPathBytes {
		add("log path exceeds %d bytes", MaxLogPathBytes)
	}
	if r.StaleAfterMin < MinStaleMin || r.StaleAfterMin > MaxStaleMin {
		add("staleAfterMin out of range 1..1440: %d", r.StaleAfterMin)
	}
	if r.WindDownMin < MinWindDownMin || r.WindDownMin > MaxWindDownMin {
		add("windDownMin out of range 1..120: %d", r.WindDownMin)
	}
	switch r.Evidence.Mode {
	case EvidenceSidecar, EvidencePattern, EvidenceNone:
	default:
		add("evidence mode enum violated: %q", r.Evidence.Mode)
	}
	if len(r.Evidence.VerdictPattern) > MaxPatternBytes {
		add("verdictPattern exceeds %d bytes", MaxPatternBytes)
	}
	if r.Evidence.VerdictPattern != "" {
		if _, err := regexp.Compile(r.Evidence.VerdictPattern); err != nil {
			add("verdictPattern is not valid RE2: %v", err)
		}
		if r.Custody == CustodyWrapped {
			add("verdictPattern is for adopted records only")
		}
	}
	for name, value := range map[string]string{
		"green": r.Expect.Green, "red": r.Expect.Red,
		"hung": r.Expect.Hung, "unknown": r.Expect.Unknown,
	} {
		if len(value) > MaxExpectBytes {
			add("expect.%s exceeds %d bytes", name, MaxExpectBytes)
		}
	}
	switch r.Status {
	case StatusLaunching:
		if r.Pid != nil || r.Pgid != nil {
			add("a launching record carries no identity yet")
		}
	case StatusRunning, StatusDraining:
		if r.Pid == nil || r.PidStartedAt == nil || r.Pgid == nil {
			add("a %s record requires full identity", r.Status)
		}
	case StatusGreen, StatusRed, StatusEndedUnknown, StatusLaunchFailed:
		if r.TerminalSeq == nil {
			add("a terminal record requires terminalSeq")
		}
	default:
		add("status enum violated: %q", r.Status)
	}
	if r.Status == StatusDraining && r.ProvisionalVerdict == nil {
		add("draining requires a frozen provisionalVerdict")
	}
	if (r.Status == StatusDraining || Terminal(r.Status)) && r.Status != StatusLaunchFailed && r.EndedAt == nil {
		add("%s requires endedAt", r.Status)
	}
	return problems
}

// mintNonce returns 32 hex from the system's entropy.
func mintNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// OwnerDigest is the waiter key's owner component: the mainId, or
// human:<uid> for HUMAN callers (the OS user id is the canonical stable
// human key).
func OwnerDigest(mainId string) string {
	owner := mainId
	if owner == "" {
		owner = "human:" + strconv.Itoa(os.Getuid())
	}
	sum := sha256.Sum256([]byte(owner))
	return hex.EncodeToString(sum[:])[:12]
}

// LifecycleTag is the tagged digest encoding for runs (the
// job form is pinned beside it in internal/goal).
func (r *Record) LifecycleTag() string {
	return fmt.Sprintf("run:%s.g%d.%s", r.RunId, r.Generation, r.LaunchNonce)
}

// emit sends a flight-recorder event; failures are logged facts elsewhere,
// never control flow. The seam is a var for tests.
var emitEvent = func(root, event string, fields map[string]string) {}

func (s *Store) emit(event string, fields map[string]string) {
	emitEvent(s.Root, event, fields)
}

// SetEmitter wires the flight-recorder seam; the command layer installs
// the real events emitter, tests install probes. Events narrate — they
// are never the wake authority.
func SetEmitter(fn func(root, event string, fields map[string]string)) {
	if fn != nil {
		emitEvent = fn
	}
}
