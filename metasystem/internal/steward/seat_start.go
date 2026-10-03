package steward

// Starting and reaping the seat mains (g1-s77, D-seat and D-retry). The
// launch itself is the launch lane's seat kind, reached through SeatLauncher.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	Brief     string
	Tag       string
}

// SeatLaunchState is one seat launch as its launch record says.
type SeatLaunchState struct {
	Found      bool
	Terminal   bool
	State      string
	ResultPath string
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
		Tips:    gitGoalTips,
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
// a provider-limit ending feeds the outage mark and never counts; otherwise
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
				if _, err := outage.Record(repoRoot, class, evidence, SeatLineage, now); err != nil {
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
	owned := work == WorkOwned
	if work != WorkClaimable && !owned {
		return Decision{}, nil, false
	}
	// A seat is opt-in per seat and the landing lane never starts one
	// (Amendment 1): while none may start, the ladder is today's.
	allowed, _, err := dependencies.Launcher.SeatAllowed(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "whether this installation starts a seat cannot be read: " + err.Error()}, nil, true
	}
	if !allowed {
		return Decision{}, nil, false
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
	if owned && !seatStepDue(world) && holdsForeign(world) {
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
	openWork := defaultTickContinuationDependencies().openWork
	openWork.Seat = &dependencies
	dependencies.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
		return seatRecheck(root, cfg, census, openWork, records)
	}
	return startSeatWithDependencies(root, selection, dependencies)
}

// seatBrief is what the seat main reads on stdin.
func seatBrief(selection SeatSelection) string {
	why := "it is approved and ready"
	if selection.Held {
		why = "this seat already holds it; the main before you ended"
	}
	return fmt.Sprintf(`# Seat session

You are this seat's session: `+"`metasystem goal claim %s`"+` takes or continues this work.
Work it and land it; stop when nothing is claimable.

The steward started you for goal %s: %s.

Nobody sits at this terminal: your process ends when your turn ends, and every background job you started ends with it. Never end a turn to wait for a job, a critique, a test run or a reply; wait inside the turn with `+"`metasystem work wait`"+` (bounded by --timeout) and carry on. End your turn only when the goal is handed in to land, it is blocked on a person's answer you asked with `+"`metasystem question ask`"+`, or nothing is claimable.
`, selection.Goal, selection.Goal, why)
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
	if err := writeExclusiveBrief(briefPath, seatBrief(selection)); err != nil {
		return SeatRecord{}, err
	}
	machine, err := dependencies.Machine(repoRoot)
	if err != nil || machine == "" {
		machine = "this machine"
	}
	goals := append(append([]string{selection.Goal}, selection.Ready...), selection.SeatHeld...)
	read, err := dependencies.Tips(repoRoot, goals)
	if err != nil {
		return SeatRecord{}, err
	}
	tips := map[string]string{}
	for _, id := range goals {
		tips[id] = read[id]
	}
	record := SeatRecord{Schema: 1, LaunchID: id, Goal: selection.Goal, Held: selection.Held, ApprovalOpid: selection.ApprovalOpid,
		Machine: machine, Tips: tips, StartedAt: dependencies.Now().UTC().Format(seatStartedAtLayout)}
	if err := writeSeatRecord(repoRoot, record); err != nil {
		return SeatRecord{}, err
	}
	if err := dependencies.Launcher.StartSeat(SeatLaunchSpec{ID: id, StateRoot: repoRoot, Brief: briefPath, Tag: nonce}); err != nil {
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
	Project  func(root string, now time.Time) (goal.Projection, error)
	Tips     func(root string, goals []string) (map[string]string, error)
	Machine  func(root string) (string, error)
	Gate     func(root string) (goal.GateSettings, error)
	Fence    func(root string) (closed bool, reason string, err error)
	Classify func(path string) (class, evidence string, ok bool)
	Now      func() time.Time
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
	_, providerOutage := outage.StandingAt(repoRoot, cfg.now())
	d, selection, _, err := decideNowWithSeat(repoRoot, cfg, census, Evidence{}, providerOutage, openWork, &seatTickState{Records: records})
	return d, selection, err
}
