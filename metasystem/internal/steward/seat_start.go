package steward

// Starting and reaping the seat mains (g1-s77, D-seat and D-retry). The
// launch itself is the launch lane's seat kind, reached through SeatLauncher.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// SeatLaunchSpec is the seat start the steward asks for: the launch lane's
// seat kind in the checkout, the brief on stdin, the id chosen here so the
// record exists before the launch does. StateRoot is the steward's own root,
// the installation whose fence the seat binds to; the launcher runs the seat
// at the top of the checkout that holds it.
type SeatLaunchSpec struct {
	ID        string
	StateRoot string
	// Installation is the root whose metasystem.conf holds the seat's
	// launch settings; the state root keeps the checkout and the fence.
	Installation string
	Brief        string
	Tag          string
}

// SeatLaunchState is one seat launch as its launch record says.
type SeatLaunchState struct {
	Found      bool
	Terminal   bool
	State      string
	ResultPath string
	FinishedAt string
}

// SeatLauncher starts and reads seat launches; the command layer supplies the
// launch manager.
type SeatLauncher interface {
	StartSeat(SeatLaunchSpec) error
	SeatLaunch(id string) (SeatLaunchState, error)
	// SeatAllowed says whether this installation starts seats at all, and
	// why not (Amendment 1): a seat is opt-in per seat, and the host's
	// landing lane never starts one.
	SeatAllowed(stateRoot string) (bool, string, error)
}

// Seat outcomes as the reap records them.
const (
	SeatProviderLimit = "provider-limit"
	SeatProgress      = "progress"
	SeatNoProgress    = "no-progress"
	SeatStartFailed   = "start-failed"
)

// SeatRecord is one seat start under artifacts/agents/steward/seats.
type SeatRecord struct {
	Schema       int               `json:"schema"`
	LaunchID     string            `json:"launchId"`
	Goal         string            `json:"goal"`
	Held         bool              `json:"held"`
	ApprovalOpid string            `json:"approvalOpid"`
	Machine      string            `json:"machine"`
	Tips         map[string]string `json:"tips"`
	StartedAt    string            `json:"startedAt"`
	ReapedAt     string            `json:"reapedAt,omitempty"`
	LaunchState  string            `json:"launchState,omitempty"`
	Outcome      string            `json:"outcome,omitempty"`
	Evidence     string            `json:"evidence,omitempty"`
	RetryAfter   string            `json:"retryAfter,omitempty"`
	ResetRetry   bool              `json:"resetRetry,omitempty"`
}

// seatStartedAtLayout is fixed-width, so the records' start times order
// lexically as they order in time.
const seatStartedAtLayout = "2006-01-02T15:04:05.000000000Z"

func seatsDir(repoRoot string) string { return filepath.Join(runnerDir(repoRoot), "seats") }

func seatRecordPath(repoRoot, id string) string {
	return filepath.Join(seatsDir(repoRoot), id+".json")
}

func writeSeatRecord(repoRoot string, record SeatRecord) error {
	if err := os.MkdirAll(seatsDir(repoRoot), 0o755); err != nil {
		return err
	}
	return writeJSONAtomic(seatRecordPath(repoRoot, record.LaunchID), record)
}

// readSeatRecords reads every seat record, oldest first. An unreadable record
// fails the read: a hidden seat would let a second one start.
func readSeatRecords(repoRoot string) ([]SeatRecord, error) {
	entries, err := os.ReadDir(seatsDir(repoRoot))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the seat records cannot be read: %w", err)
	}
	var records []SeatRecord
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(seatsDir(repoRoot), name))
		if err != nil {
			return nil, fmt.Errorf("seat record %s cannot be read: %w", name, err)
		}
		var record SeatRecord
		if err := json.Unmarshal(data, &record); err != nil || record.LaunchID == "" {
			return nil, fmt.Errorf("seat record %s is malformed", name)
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].StartedAt != records[j].StartedAt {
			return records[i].StartedAt < records[j].StartedAt
		}
		return records[i].LaunchID < records[j].LaunchID
	})
	return records, nil
}

// defaultSeatDependencies are the production readers around a launcher.
func defaultSeatDependencies(launcher SeatLauncher) seatDependencies {
	return seatDependencies{
		Launcher: launcher,
		Project: func(root string, now time.Time) (goal.Projection, error) {
			stateRoot, err := goal.ResolveStateRoot(root)
			if err != nil {
				return goal.Projection{}, err
			}
			endpoint, err := goal.ResolveEndpoint(stateRoot)
			if err != nil {
				return goal.Projection{}, err
			}
			return goal.Project(endpoint, false, now)
		},
		Tips:  gitGoalTips,
		Units: func(string, string) ([]UnitStage, error) { return nil, errors.New("unit reader unavailable") },
		Main: func(root string) (string, error) {
			out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}").Output()
			return strings.TrimSpace(string(out)), err
		},
		Contains: func(root, sha, main string) (bool, error) { return plain.ContainedIn(root, main)(sha) },
		Lane: func(_ string, id string) (plain.Entry, bool, error) {
			home, err := board.Home()
			if err != nil {
				return plain.Entry{}, false, err
			}
			record, registered, incomplete, err := lane.ReadGuarded(home)
			if incomplete != nil {
				return plain.Entry{}, false, incomplete
			}
			if err != nil || !registered {
				return plain.Entry{}, false, err
			}
			return plain.Latest(record.Install, id)
		},
		Jobs:     seatJobRecords,
		Refusals: (launch.Store{}).Refusals,
		Launches: (launch.Store{}).List,
		Threads: func() ([]board.Thread, error) {
			home, err := board.Home()
			if err != nil {
				return nil, err
			}
			return board.Threads(home)
		},
		Machine: goal.ResolveMachine,
		OpenQuestions: func(root string) []goal.OpenQuestion {
			stateRoot, err := goal.ResolveStateRoot(root)
			if err != nil {
				return nil
			}
			return channel.GoalOpenQuestions(stateRoot)
		},
		Gate: func(root string) (goal.GateSettings, error) {
			stateRoot, err := goal.ResolveStateRoot(root)
			if err != nil {
				return goal.GateSettings{}, err
			}
			return goal.ResolveGateSettings(filepath.Join(stateRoot, "metasystem.conf"))
		},
		Fence: func(root string) (bool, string, error) {
			closed, record, err := stopfence.Closed(root)
			if err != nil || !closed {
				return false, "", err
			}
			description, err := stopfence.ClosedDescription(record, root)
			if err != nil {
				return true, "", err
			}
			return true, description, nil
		},
		Classify: outage.ClassifyProviderResult,
		Now:      time.Now,
		Sleep:    time.Sleep,
	}
}

// gitGoalTips reads each goal branch tip, refs/heads/goal/<id>; an absent
// branch is absent from the answer.
func gitGoalTips(root string, goals []string) (map[string]string, error) {
	tips := map[string]string{}
	for _, id := range goals {
		out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "refs/heads/goal/"+id).Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				continue
			}
			return nil, fmt.Errorf("goal branch %s cannot be read: %w", id, err)
		}
		if tip := strings.TrimSpace(string(out)); tip != "" {
			tips[id] = tip
		}
	}
	return tips, nil
}

// seatTickState is what the tick's seat pass read before its decision: the
// unreaped seat launch, if any, and every seat record after the reap.
type seatTickState struct {
	Unreaped string
	Records  []SeatRecord
	Err      error
}

// reapSeatLaunches closes every seat record whose launch has ended (D-retry):
// a provider-limit ending retries once at the reset edge before feeding the
// outage mark, and never counts; otherwise
// the seat made progress when a recorded goal branch tip moved or appeared,
// or when the seat lineage wrote land-ready or done on this machine since the
// seat started.
func reapSeatLaunches(repoRoot string, dependencies seatDependencies, now time.Time) seatTickState {
	records, err := readSeatRecords(repoRoot)
	if err != nil {
		return seatTickState{Err: err}
	}
	state := seatTickState{}
	var projection *goal.Projection
	for i := range records {
		record := &records[i]
		if record.ReapedAt != "" {
			continue
		}
		launch, err := dependencies.Launcher.SeatLaunch(record.LaunchID)
		if err != nil {
			return seatTickState{Err: fmt.Errorf("seat launch %s cannot be read: %w", record.LaunchID, err)}
		}
		if launch.Found && !launch.Terminal {
			state.Unreaped = record.LaunchID
			continue
		}
		record.LaunchState = launch.State
		switch {
		case !launch.Found:
			record.LaunchState = "missing"
			record.Outcome, record.Evidence = SeatNoProgress, "the launch record is missing"
		default:
			if class, evidence, ok := dependencies.Classify(launch.ResultPath); ok {
				record.Outcome, record.Evidence = SeatProviderLimit, class+": "+evidence
				seen, err := time.Parse(time.RFC3339Nano, launch.FinishedAt)
				if err != nil {
					record.Evidence += "; the launch ending has no readable message time; no outage mark was written"
					break
				}
				if reset, retry := outage.ResetRetryAt(class, evidence, seen); retry && !record.ResetRetry {
					record.RetryAfter = reset.UTC().Format(time.RFC3339)
					if wait := reset.Sub(dependencies.Now()); wait > 0 && wait <= 2*time.Minute {
						dependencies.Sleep(wait)
					}
					break
				}
				if _, err := outage.Record(repoRoot, class, evidence, SeatLineage, seen); err != nil {
					record.Evidence += "; the outage mark was not fed: " + err.Error()
				}
				break
			}
			if projection == nil {
				read, err := dependencies.Project(repoRoot, now)
				if err != nil {
					return seatTickState{Err: fmt.Errorf("the goal ledger cannot be read to judge seat %s: %w", record.LaunchID, err)}
				}
				projection = &read
			}
			progress, evidence, err := seatProgress(repoRoot, *record, *projection, dependencies)
			if err != nil {
				return seatTickState{Err: err}
			}
			record.Outcome, record.Evidence = SeatNoProgress, "no retained work advance"
			if progress {
				record.Outcome, record.Evidence = SeatProgress, evidence
			}
		}
		record.ReapedAt = now.UTC().Format(time.RFC3339)
		if err := writeSeatRecord(repoRoot, *record); err != nil {
			return seatTickState{Err: fmt.Errorf("seat record %s cannot be written: %w", record.LaunchID, err)}
		}
	}
	state.Records = records
	return state
}

// seatProgress is D-retry's retained work advance for one ended seat. Claim,
// release, park and every other history line are not progress.
func seatProgress(repoRoot string, record SeatRecord, projection goal.Projection, dependencies seatDependencies) (bool, string, error) {
	goals := make([]string, 0, len(record.Tips))
	for id := range record.Tips {
		goals = append(goals, id)
	}
	sort.Strings(goals)
	tips, err := dependencies.Tips(repoRoot, goals)
	if err != nil {
		return false, "", err
	}
	for _, id := range goals {
		if now := tips[id]; now != "" && now != record.Tips[id] {
			return true, "goal/" + id + " moved to " + now, nil
		}
	}
	started, err := time.Parse(time.RFC3339Nano, record.StartedAt)
	if err != nil || projection.Tree == nil {
		return false, "", nil
	}
	actor := record.Machine + "+" + SeatLineage
	files := []*goal.GoalFile{}
	for _, file := range projection.Tree.Live {
		files = append(files, file)
	}
	for _, file := range projection.Tree.Done {
		files = append(files, file)
	}
	for _, file := range files {
		if file == nil {
			continue
		}
		for _, line := range file.History {
			if (line.Verb != "land-ready" && line.Verb != "done") || line.Actor != actor {
				continue
			}
			if at, err := time.Parse(time.RFC3339, line.At); err == nil && !at.Before(started.Truncate(time.Second)) {
				return true, line.Verb + " " + file.Id + " by " + actor, nil
			}
		}
	}
	return false, "", nil
}

// decideSeat is D-ladder over claimable work, and over owned work whose claim
// is the seat lineage's. ok is false when the decision is not the seat
// ladder's: today's ladder decides.
func decideSeat(repoRoot string, cfg TickConfig, work OpenWork, shared goal.ClaimableBudgetedWork, workers Workers, providerOutage bool,
	dependencies seatDependencies, state seatTickState) (Decision, *SeatSelection, bool) {
	return seatDecision(repoRoot, cfg, work, shared, workers, providerOutage, dependencies, state)
}

// seatDecision owns the start guards for both the tick and its health reading.
func seatDecision(repoRoot string, cfg TickConfig, work OpenWork, shared goal.ClaimableBudgetedWork, workers Workers, providerOutage bool,
	dependencies seatDependencies, state seatTickState) (Decision, *SeatSelection, bool) {
	owned := work == WorkOwned || work == WorkWaiting
	if work != WorkClaimable && !owned {
		return Decision{}, nil, false
	}
	// A seat is opt-in per seat and the landing lane never starts one
	// (Amendment 1): while none may start, the ladder is today's.
	if dependencies.Launcher == nil {
		return Decision{VerdictHealthy, ActNone, "this process has no seat launcher"}, nil, false
	}
	allowed, reason, err := dependencies.Launcher.SeatAllowed(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "whether this installation starts a seat cannot be read: " + err.Error()}, nil, true
	}
	if !allowed {
		return Decision{VerdictHealthy, ActNone, reason}, nil, false
	}
	doubt := !workers.CensusComplete || workers.Untracked > 0 || workers.Unprovable > 0
	if owned {
		if workers.Live > 0 || doubt || !holdsUnderSeatLineage(shared) {
			return Decision{}, nil, false
		}
	}
	if state.Err != nil {
		return Decision{VerdictDegraded, ActNotify, "the seat launches cannot be judged: " + state.Err.Error()}, nil, true
	}
	now := cfg.now()
	projection, err := dependencies.Project(repoRoot, now)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "the goal ledger cannot be read for the seat ladder: " + err.Error()}, nil, true
	}
	settings, err := dependencies.Gate(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "the landing gate's settings cannot be read: " + err.Error()}, nil, true
	}
	var live map[string]*goal.GoalFile
	if projection.Tree != nil {
		live = projection.Tree.Live
	}
	tips, err := dependencies.Tips(repoRoot, shared.Landing)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "the goal branch tips cannot be read for the landing gate: " + err.Error()}, nil, true
	}
	if dependencies.OpenQuestions != nil {
		shared.MarkAskedOpen(dependencies.OpenQuestions(repoRoot))
	}
	world := SeatWorldFrom(shared, live, settings, tips, now)
	if work != WorkWaiting && owned && !seatStepDue(world) && holdsForeign(world) {
		return Decision{}, nil, false
	}
	verdict := VerdictIdleBacklogDead
	if owned {
		verdict = VerdictStalledDead
	}
	if state.Unreaped != "" {
		return Decision{VerdictHealthy, ActNone, "seat launch " + state.Unreaped + " of this steward is not yet reaped"}, nil, true
	}
	if !owned {
		if workers.LiveSeatMains > 0 {
			return Decide(Snapshot{Work: work}), nil, true
		}
		if doubt {
			return Decision{VerdictUnknown, ActNotify, fmt.Sprintf(
				"death not provable (census complete: %v, untracked: %d, unprovable: %d); notifying, never starting a seat on doubt",
				workers.CensusComplete, workers.Untracked, workers.Unprovable)}, nil, true
		}
	}
	if providerOutage {
		return Decision{verdict, ActNotify, "the model provider is overloaded or limited; holding the seat start until the provider recovers"}, nil, true
	}
	closed, reason, err := dependencies.Fence(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "the process-creation fence cannot be read: " + err.Error()}, nil, true
	}
	if closed {
		return Decision{verdict, ActNone, reason}, nil, true
	}
	d, selection := PlanSeat(world, state.Records, cfg.MaxRevivals, owned)
	return d, selection, true
}

func holdsUnderSeatLineage(shared goal.ClaimableBudgetedWork) bool {
	for _, id := range append(append([]string(nil), shared.Claimed...), shared.Landing...) {
		if file, ok := shared.OwnedClaim(id); ok && claimLineage(file) == SeatLineage {
			return true
		}
	}
	return false
}

func seatStepDue(world SeatWorld) bool {
	for _, held := range world.Held {
		if held.Lineage == SeatLineage && held.StepDue {
			return true
		}
	}
	return false
}

func holdsForeign(world SeatWorld) bool {
	for _, held := range world.Held {
		if held.Lineage != SeatLineage {
			return true
		}
	}
	return false
}

// StartSeat starts one seat main naming the selected goal, through the
// launcher the runner was given, once the census given, the outage mark and
// the ladder over the ledger read again still select it.
func StartSeat(repoRoot string, cfg TickConfig, census WorkerCensus, selection SeatSelection) (SeatRecord, error) {
	if cfg.Seat == nil {
		return SeatRecord{}, errors.New("no seat launcher is wired to this steward")
	}
	root := canonicalPath(repoRoot)
	dependencies := defaultSeatDependencies(cfg.Seat)
	if cfg.Units != nil {
		dependencies.Units = cfg.Units
	}
	openWork := defaultTickContinuationDependencies().openWork
	openWork.Seat = &dependencies
	dependencies.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
		return seatRecheck(root, cfg, census, openWork, records)
	}
	return startSeatWithDependencies(root, selection, dependencies)
}

// seatBrief is what the seat main reads on stdin.
func seatBrief(selection SeatSelection, automaticHandoff bool, facts ...string) string {
	why := "it is approved and ready"
	if selection.Held {
		why = "this seat already holds it; the main before you ended"
	}
	boundary, handoff := "", ""
	if automaticHandoff {
		boundary = "; stop advancing at that boundary"
		handoff = "Nobody sits at this terminal: your process ends when your turn ends.\nEvery background job you started ends with it. Never end a turn to wait for a job, a critique, a test run or a reply; wait inside the turn with `metasystem work wait` (bounded by --timeout) and carry on. At the completed unit boundary, write your lessons note in your runtime's configured context.handoff.note-directory, then run `metasystem session handoff --root <installation> --note <that note> --no-delegates`. Read `metasystem session handoff --status --root <installation> --json` until this session's completed boundary carries that handoff nonce; then repeat the same handoff command to confirm durable binding. The steward persists that binding before ending your session. End your turn once the binding is durable. If capture or binding fails, remain alive, report the exact error, repair its cause and retry the same handoff. Otherwise end only when blocked on a person's answer you asked with `metasystem question ask`, or nothing is claimable.\n"
	}
	return fmt.Sprintf("# Seat session\n\n"+
		"You are this seat's session: `metasystem goal claim %s` takes or continues this work.\nWork one unit through its completed outcome and collected, published read%s.\n\n"+
		"The steward started you for goal %s: %s.\n\n",
		selection.Goal, boundary, selection.Goal, why) + handoff + strings.Join(facts, "")
}

func startSeatWithDependencies(repoRoot string, selection SeatSelection, dependencies seatDependencies) (SeatRecord, error) {
	if helm.Active(repoRoot).Active {
		return SeatRecord{}, nil
	}
	if selection.Goal == "" {
		return SeatRecord{}, errors.New("a seat start names its goal")
	}
	arbitration, err := AcquireArbitration(repoRoot)
	if err != nil {
		return SeatRecord{}, err
	}
	defer arbitration.Release()
	closed, reason, err := dependencies.Fence(repoRoot)
	if err != nil {
		return SeatRecord{}, fmt.Errorf("read process-creation fence: %w", err)
	}
	if closed {
		return SeatRecord{}, errors.New(reason)
	}
	records, err := readSeatRecords(repoRoot)
	if err != nil {
		return SeatRecord{}, err
	}
	for _, record := range records {
		if record.ReapedAt == "" {
			return SeatRecord{}, nil
		}
	}
	// The tick decided before the runner's other work; under this lock the
	// census, the outage mark and the goal's eligibility are read again and
	// must still select this goal (SOL-A-03).
	if dependencies.Recheck != nil {
		d, now, err := dependencies.Recheck(records)
		if err != nil {
			return SeatRecord{}, fmt.Errorf("no seat starts for %s: its grounds cannot be read again: %w", selection.Goal, err)
		}
		if d.Action != ActRevive || now == nil || now.Goal != selection.Goal || now.Held != selection.Held {
			return SeatRecord{}, fmt.Errorf("no seat starts for %s: its grounds changed since the tick: %s", selection.Goal, d.Reason)
		}
	}
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return SeatRecord{}, err
	}
	nonce := hex.EncodeToString(raw)
	id := "seat-" + nonce
	if err := os.MkdirAll(seatsDir(repoRoot), 0o755); err != nil {
		return SeatRecord{}, err
	}
	briefPath := filepath.Join(seatsDir(repoRoot), id+".brief.md")
	machine, err := dependencies.Machine(repoRoot)
	if err != nil || machine == "" {
		machine = "this machine"
	}
	goals := append(append([]string{selection.Goal}, selection.Ready...), selection.SeatHeld...)
	read, err := dependencies.Tips(repoRoot, goals)
	tipsError := err
	tips := map[string]string{}
	for _, id := range goals {
		tips[id] = read[id]
	}
	policy, policyErr := config.ResolvePolicy(config.GetParams{Key: "seat.driver", ConfPath: filepath.Join(repoRoot, "metasystem.conf")})
	automaticHandoff := policyErr != nil || policy.Value != "person"
	facts := seatFacts(repoRoot, selection, machine, tips, dependencies)
	if policyErr != nil {
		facts += "Seat driver unavailable: " + policyErr.Error() + "\n"
	}
	if tipsError != nil {
		facts += "Goal tips unavailable; metasystem work status " + selection.Goal + "\n"
	}
	if err := writeExclusiveBrief(briefPath, seatBrief(selection, automaticHandoff, facts)); err != nil {
		return SeatRecord{}, err
	}
	record := SeatRecord{Schema: 1, LaunchID: id, Goal: selection.Goal, Held: selection.Held, ApprovalOpid: selection.ApprovalOpid,
		Machine: machine, Tips: tips, StartedAt: dependencies.Now().UTC().Format(seatStartedAtLayout)}
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Goal == selection.Goal {
			record.ResetRetry = records[i].RetryAfter != ""
			break
		}
	}
	if err := writeSeatRecord(repoRoot, record); err != nil {
		return SeatRecord{}, err
	}
	// The steward's own root holds both its run state and the installation's
	// settings, so it names that root as the seat's installation too.
	if err := dependencies.Launcher.StartSeat(SeatLaunchSpec{ID: id, StateRoot: repoRoot, Installation: repoRoot, Brief: briefPath, Tag: nonce}); err != nil {
		record.ReapedAt = dependencies.Now().UTC().Format(time.RFC3339)
		record.Outcome, record.Evidence = SeatStartFailed, err.Error()
		if writeErr := writeSeatRecord(repoRoot, record); writeErr != nil {
			return record, fmt.Errorf("seat launch %s did not start (%v), and its record could not close: %w", id, err, writeErr)
		}
		return record, fmt.Errorf("seat launch %s did not start: %w", id, err)
	}
	if err := QueueNotification(repoRoot, PendingNotification{
		Nonce: "seat-start-" + id,
		Message: fmt.Sprintf("steward: started seat %s on %s for %s; `metasystem work stop %s` ends it, `metasystem helm take` keeps the steward from starting another",
			id, machine, selection.Goal, id),
	}); err != nil {
		return record, err
	}
	return record, nil
}

// seatDependencies are the seat ladder's readers and its launcher.
type seatDependencies struct {
	Launcher SeatLauncher
	Units    func(root, goalID string) ([]UnitStage, error)
	Main     func(root string) (string, error)
	Contains func(root, sha, main string) (bool, error)
	Lane     func(root, goalID string) (plain.Entry, bool, error)
	Jobs     func(root string) ([]map[string]any, error)
	Refusals func() ([]launch.Refusal, error)
	Launches func() ([]launch.Record, error)
	Threads  func() ([]board.Thread, error)
	Project  func(root string, now time.Time) (goal.Projection, error)
	Tips     func(root string, goals []string) (map[string]string, error)
	Machine  func(root string) (string, error)
	Gate     func(root string) (goal.GateSettings, error)
	Fence    func(root string) (closed bool, reason string, err error)
	Classify func(path string) (class, evidence string, ok bool)
	Now      func() time.Time
	Sleep    func(time.Duration)
	Log      func(string)
	// OpenQuestions reads the open channel questions; a goal one names is
	// left to the person, like a goal waiting on a human word. Nil reads none.
	OpenQuestions func(root string) []goal.OpenQuestion
	// Recheck re-reads, under the start's lock, what the tick's decision
	// rested on and answers the decision now (SOL-A-03); nil re-reads
	// nothing.
	Recheck func(records []SeatRecord) (Decision, *SeatSelection, error)
}

// seatRecheck is the tick's seat decision again over fresh readings: the
// census, the outage mark and the ladder over the ledger read now. The
// records are the seat records the start just read, none unreaped.
func seatRecheck(repoRoot string, cfg TickConfig, census WorkerCensus, openWork openWorkDependencies, records []SeatRecord) (Decision, *SeatSelection, error) {
	_, providerOutage := standingProviderOutage(repoRoot, cfg.now(), nil)
	d, selection, _, err := decideNowWithSeat(repoRoot, cfg, census, Evidence{}, providerOutage, openWork, &seatTickState{Records: records})
	return d, selection, err
}

func standingProviderOutage(repoRoot string, now time.Time, log func(string)) (outage.Mark, bool) {
	mark, standing := outage.StandingAt(repoRoot, now)
	if defect := mark.ResetDefect(now); defect != "" {
		if log != nil {
			log(defect)
		} else {
			fmt.Fprintln(os.Stderr, defect)
		}
	}
	return mark, standing
}

// UnitStage is one unit as the public status reader describes it.
type UnitStage struct{ Unit, Stage, Line, At, Run string }

type seatFactLine struct {
	text, omitted string
	at            time.Time
}

func seatTime(value string) time.Time { at, _ := time.Parse(time.RFC3339, value); return at }

// seatCut counts omitted bytes and keeps the retained text valid UTF-8.
func seatCut(value string, limit int, name, command string) string {
	if len(value) <= limit {
		return value
	}
	note := fmt.Sprintf("\n[%s: %d bytes omitted; %s]\n", name, len(value), command)
	end := max(0, limit-len(note))
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	note = fmt.Sprintf("\n[%s: %d bytes omitted; %s]\n", name, len(value)-end, command)
	return value[:end] + note
}

// seatList keeps newest entries and names every omitted entry or counterpart.
func seatList(lines []seatFactLine, count, limit int, command string) string {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].at.Equal(lines[j].at) {
			return lines[i].omitted < lines[j].omitted
		}
		return lines[i].at.After(lines[j].at)
	})
	keep := min(count, len(lines))
	for {
		var body strings.Builder
		for _, line := range lines[:keep] {
			body.WriteString(line.text + "\n")
		}
		omitted := map[string]int{}
		for _, line := range lines[keep:] {
			omitted[line.omitted]++
		}
		var cuts []string
		for _, name := range slices.Sorted(maps.Keys(omitted)) {
			cuts = append(cuts, fmt.Sprintf("%s: %d omitted", name, omitted[name]))
		}
		note := ""
		if len(cuts) > 0 {
			note = seatCut("["+strings.Join(cuts, "; ")+"; "+command+"]\n", limit, fmt.Sprintf("%d omission notices", len(cuts)), command)
		}
		if body.Len()+len(note) <= limit || keep == 0 {
			return body.String() + note
		}
		keep--
	}
}

func seatFacts(root string, selection SeatSelection, machine string, tips map[string]string, d seatDependencies) string {
	id := selection.Goal
	show, status, landing, inbox := "metasystem goal show "+id, "metasystem work status "+id, "metasystem landing status", "metasystem agent inbox"
	var facts strings.Builder
	unavailable := func(name, command string) { facts.WriteString(name + " unavailable; " + command + "\n") }
	facts.WriteString("\nGoal record:\n")
	projection, err := d.Project(root, d.Now())
	if err != nil || projection.Tree == nil || projection.Tree.Live[id] == nil {
		unavailable("Next step", show)
	} else {
		facts.WriteString("Next step: " + seatCut(projection.Tree.Live[id].NextStep, 2000, "next step", show) + "\n")
	}
	var units []UnitStage
	if units, err = d.Units(root, id); err != nil {
		unavailable("Units", status)
	}
	hand := ""
	entry, found, readErr := d.Lane(root, id)
	if readErr == nil && found && entry.State == plain.StateWaiting {
		main, mainErr := d.Main(root)
		readErr = mainErr
		if readErr == nil {
			var derived []plain.Entry
			derived, readErr = plain.Landed([]plain.Entry{entry}, func(sha string) (bool, error) { return d.Contains(root, sha, main) })
			entry = derived[0]
		}
	}
	if readErr != nil {
		unavailable("Hand-in", landing)
	} else if found {
		hand = entry.State
		if hand == plain.StateWaiting {
			hand = "handed in"
		}
		if entry.Reason != "" {
			hand += ": " + seatCut(entry.Reason, 600, "return text", landing)
		}
		if tips[id] == "" || entry.SHA != tips[id] {
			facts.WriteString("Hand-in at commit " + entry.SHA + ": " + hand + "\n")
			hand = ""
		}
	} else {
		facts.WriteString("Hand-in: none\n")
	}
	var unitLines, refusals []seatFactLine
	for _, unit := range units {
		line := unit.Line
		if line == "" {
			line = unit.Unit + ": " + unit.Stage
		}
		if hand != "" {
			line += "; " + hand
		}
		unitLines = append(unitLines, seatFactLine{line, "units", seatTime(unit.At)})
		if strings.HasPrefix(unit.Stage, "review refused") {
			refusals = append(refusals, seatFactLine{unit.Unit + ": " + unit.Stage + "; metasystem work review " + id + " --work " + unit.Unit, "refusals", seatTime(unit.At)})
		}
	}
	facts.WriteString("Units:\n" + seatList(unitLines, 20, 3000, status))
	jobs, readErr := d.Jobs(root)
	if readErr != nil {
		unavailable("Dispatch refusals", status)
	} else {
		for _, job := range jobs {
			if job["goalId"] != id || job["status"] != "failed" || job["error"] != "dispatch-refused" {
				continue
			}
			at, _ := job["createdAt"].(string)
			open := true
			for _, later := range jobs {
				when, _ := later["createdAt"].(string)
				if later["goalId"] == id && later["role"] == job["role"] && later["reviews"] == job["reviews"] && later["reviewedCommit"] == job["reviewedCommit"] && seatTime(when).After(seatTime(at)) {
					open = false
				}
			}
			if open {
				jobID, _ := job["jobId"].(string)
				refusals = append(refusals, seatFactLine{fmt.Sprintf("job %s, role %v, refusal %v: %v", jobID, job["role"], job["refusalClass"], job["summary"]), "refusals", seatTime(at)})
			}
		}
	}
	refused, refusalErr := d.Refusals()
	launched, launchErr := d.Launches()
	if refusalErr != nil || launchErr != nil {
		unavailable("Launch refusals", status)
	} else {
		latest := map[string]time.Time{}
		for _, record := range launched {
			at := seatTime(record.StartedAt)
			if record.Goal == id && at.After(latest[record.Kind]) {
				latest[record.Kind] = at
			}
		}
		for _, refusal := range refused {
			at := seatTime(refusal.Time)
			if refusal.Goal == id && !latest[refusal.Kind].After(at) {
				refusals = append(refusals, seatFactLine{fmt.Sprintf("launch %s, kind %s, code %s, tag %s", at.In(time.Local).Format(time.RFC3339), refusal.Kind, refusal.Code, refusal.Tag), "refusals", at})
			}
		}
	}
	facts.WriteString("Open refusals:\n" + seatList(refusals, 10, 2000, status))
	if machine == "this machine" {
		unavailable("Messages", inbox)
	} else if threads, readErr := d.Threads(); readErr != nil {
		unavailable("Messages", inbox)
	} else {
		groups := map[string][]board.Message{}
		for _, thread := range threads {
			relevant, other := false, board.Message{}
			for _, message := range thread.Messages {
				if message.To.Machine == machine || message.To.Goal == id || message.From.Machine == machine {
					relevant = true
				}
				if message.From.Machine != machine && message.At.After(other.At) {
					other = message
				}
			}
			if !relevant {
				continue
			}
			for _, message := range thread.Messages {
				counterpart := message.From.Machine
				if counterpart == machine {
					counterpart = message.To.Machine
					if counterpart == "" {
						counterpart = other.From.Machine
					}
					if counterpart == "" {
						counterpart = id
					}
				}
				groups[counterpart] = append(groups[counterpart], message)
			}
		}
		var messageLines []seatFactLine
		retained := 0
		for counterpart, messages := range groups {
			sort.SliceStable(messages, func(i, j int) bool {
				if messages[i].At.Equal(messages[j].At) {
					return messages[i].ID > messages[j].ID
				}
				return messages[i].At.After(messages[j].At)
			})
			for index, message := range messages {
				text, at := "", time.Time{}
				if index < 3 {
					retained++
					at = messages[0].At
					text = board.Render(message, d.Now(), time.Local)
					if len(text) > 600 {
						closing := "\n" + fmt.Sprintf(board.Closing, message.ID)
						text = seatCut(text, 600-len(closing), "message "+message.ID, inbox) + closing
					}
				}
				messageLines = append(messageLines, seatFactLine{text, "messages with " + counterpart, at})
			}
		}
		facts.WriteString("Messages:\n" + seatList(messageLines, retained, 12000-len(seatBrief(selection, true))-facts.Len()-128, inbox))
	}
	return facts.String()
}

func seatJobRecords(root string) ([]map[string]any, error) {
	events, err := seatJSONLines(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(root, "artifacts", "agents", "jobs")
	paths, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []map[string]any
	for _, path := range paths {
		if path.IsDir() || !strings.HasSuffix(path.Name(), ".json") {
			continue
		}
		var record map[string]any
		if err := readRecordJSON(filepath.Join(directory, path.Name()), &record); err != nil {
			return nil, err
		}
		for _, event := range events {
			if event["event"] == "job-refused" && event["jobId"] == record["jobId"] {
				record["summary"] = event["summary"]
			}
		}
		records = append(records, record)
	}

	return records, nil
}

func seatJSONLines(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
}
