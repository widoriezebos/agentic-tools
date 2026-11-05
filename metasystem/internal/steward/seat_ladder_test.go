package steward

// The seat ladder's proofs (g1-s77 section 3: tests 1 to 6, 12, 17, 19, 21).
// The bed is one accepted goal tree served through the tick's real readers,
// a declared worker census, and a recording seat launcher.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

const seatBedMachine = "bed-m1"

// The steward reads and counts seats under the lineage the launch lane sets
// in the seat's environment: one lineage per installation, never two
// spellings (D-seat).
func TestSeatLineageIsTheLaunchLanes(t *testing.T) {
	t.Parallel()
	if SeatLineage != launch.SeatOwnerLineage {
		t.Fatalf("the steward's seat lineage %q is not the launch lane's %q", SeatLineage, launch.SeatOwnerLineage)
	}
}

// fakeSeatLauncher records every seat start and answers each launch's state
// as the test declares it.
type fakeSeatLauncher struct {
	starts   []SeatLaunchSpec
	states   map[string]SeatLaunchState
	startErr error
	// off is why this installation starts no seat; empty allows one.
	off string
}

func (f *fakeSeatLauncher) SeatAllowed(string) (bool, string, error) { return f.off == "", f.off, nil }

func (f *fakeSeatLauncher) StartSeat(spec SeatLaunchSpec) error {
	f.starts = append(f.starts, spec)
	if f.startErr != nil {
		return f.startErr
	}
	if f.states == nil {
		f.states = map[string]SeatLaunchState{}
	}
	f.states[spec.ID] = SeatLaunchState{Found: true, State: "running"}
	return nil
}

func (f *fakeSeatLauncher) SeatLaunch(id string) (SeatLaunchState, error) {
	return f.states[id], nil
}

type seatBed struct {
	t        *testing.T
	root     string
	now      time.Time
	goals    map[string]*goal.GoalFile
	order    []string
	launcher *fakeSeatLauncher
	tips     map[string]string
	fence    string
	settings goal.GateSettings
	// questions are the open channel questions the ladder reads.
	questions []goal.OpenQuestion
	situation func(string, time.Time) Situation
	page      func(string, time.Time) (SituationRecord, error)
	busy      bool
	boardHome string
}

func newSeatBed(t *testing.T, goals ...*goal.GoalFile) *seatBed {
	t.Helper()
	root := t.TempDir()
	testprovider.Register(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("launch.seat.runtime=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStewardRecord(t, ledgerAttentionStatePath(root), map[string]any{
		"schema": ledgerAttentionStateSchema, "lastOutcome": "local",
	})
	b := &seatBed{t: t, root: root, now: time.Now().UTC().Truncate(time.Second),
		goals: map[string]*goal.GoalFile{}, launcher: &fakeSeatLauncher{}, tips: map[string]string{},
		settings: goal.GateSettings{HumanFromTier: 2, AutoAfter: 4 * time.Hour, AutoAfterText: "4h"}}
	for _, file := range goals {
		b.put(file)
	}
	return b
}

func (b *seatBed) put(file *goal.GoalFile) {
	if _, ok := b.goals[file.Id]; !ok {
		b.order = append(b.order, file.Id)
	}
	b.goals[file.Id] = file
}

func (b *seatBed) drop(id string) { delete(b.goals, id) }

func (b *seatBed) projection(now time.Time) goal.Projection {
	b.t.Helper()
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	files := map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(rootRecord)}
	for id, file := range b.goals {
		files["plans/goals/"+id+".md"] = goal.RenderFile(file)
	}
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) != 0 {
		b.t.Fatalf("the bed's goal files did not parse: %v", problems)
	}
	return goal.Projection{Root: b.root, Tree: tree, Horizon: goal.ApprovalHorizon{Now: now}}
}

func (b *seatBed) seatDependencies() *seatDependencies {
	d := &seatDependencies{
		ProviderHome: testprovider.Home(b.root),
		Launcher:     b.launcher,
		Units:        func(string, string) ([]UnitStage, error) { return nil, nil },
		Lane:         func(string, string) (plain.Entry, bool, error) { return plain.Entry{}, false, nil },
		Main:         func(string) (string, error) { return "main", nil },
		Contains:     func(string, string, string) (bool, error) { return false, nil },
		Jobs:         func(string) ([]map[string]any, error) { return nil, nil },
		Refusals:     func() ([]launch.Refusal, error) { return nil, nil },
		Launches:     func() ([]launch.Record, error) { return nil, nil },
		Threads:      func() ([]board.Thread, error) { return nil, nil },
		Project:      func(string, time.Time) (goal.Projection, error) { return b.projection(b.now), nil },
		Tips: func(_ string, goals []string) (map[string]string, error) {
			tips := map[string]string{}
			for _, id := range goals {
				if tip, ok := b.tips[id]; ok {
					tips[id] = tip
				}
			}
			return tips, nil
		},
		Machine:  func(string) (string, error) { return seatBedMachine, nil },
		Gate:     func(string) (goal.GateSettings, error) { return b.settings, nil },
		Fence:    func(string) (bool, string, error) { return b.fence != "", b.fence, nil },
		Classify: outage.ClassifyProviderResult,
		Now:      func() time.Time { return b.now },
		OpenQuestions: func(string) []goal.OpenQuestion {
			return append([]goal.OpenQuestion(nil), b.questions...)
		},
	}
	if b.situation != nil {
		d.Situation = func(root string, now time.Time) (SituationRecord, error) {
			return SituationPage(root, func(at time.Time) Situation { return b.situation(root, at) }, now, SituationFresh)
		}
	}
	if b.page != nil {
		d.Situation = b.page
	}
	if b.boardHome != "" {
		d.Decisions = func() ([]humanauthority.Decision, int, error) { return humanauthority.ReadDecisions(b.boardHome) }
		d.Threads = func() ([]board.Thread, error) { return board.Threads(b.boardHome) }
	}
	return d
}

func (b *seatBed) dependencies() openWorkDependencies {
	return openWorkDependencies{
		NewWorld: func(string) bool { return true },
		ReadClaimableBudgetedWork: func(string, time.Time) (goal.ClaimableBudgetedWork, error) {
			return goal.ClaimableWorkFromProjection(b.projection(b.now), seatBedMachine, identity.KernelProber{})
		},
		Seat: b.seatDependencies(),
		Busy: func(string, goal.ClaimableBudgetedWork, time.Time) (bool, string, int) {
			return b.busy, "running build", 0
		},
	}
}

func (b *seatBed) tickWith(census WorkerCensus) TickResult {
	b.t.Helper()
	path := EvidencePath(b.root)
	prev, err := LoadEvidence(path)
	if err != nil {
		b.t.Fatal(err)
	}
	result, err := decideTickWithDependencies(b.root, TickConfig{Now: b.now, ProviderHome: testprovider.Home(b.root), Situation: b.situation}, census, prev, Marks{HeadOid: "h", OpidDigest: "d"}, b.dependencies())
	if err != nil {
		b.t.Fatal(err)
	}
	if err := SaveEvidence(b.root, path, result.Evidence); err != nil {
		b.t.Fatal(err)
	}
	return result
}

func (b *seatBed) tick(w Workers) TickResult {
	b.t.Helper()
	return b.tickWith(fakeCensus{workers: w})
}

func (b *seatBed) start(selection *SeatSelection) SeatRecord {
	b.t.Helper()
	if selection == nil {
		b.t.Fatal("no seat selection to start")
	}
	record, err := startSeatWithDependencies(b.root, *selection, *b.seatDependencies())
	if err != nil {
		b.t.Fatal(err)
	}
	return record
}

// startUnder starts the selection as the runner's pass does, re-reading the
// tick's grounds under the start's lock with the census given (SOL-A-03).
func (b *seatBed) startUnder(selection SeatSelection, census WorkerCensus, cached ...SituationRecord) (SeatRecord, error) {
	b.t.Helper()
	dependencies := *b.seatDependencies()
	dependencies.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
		openWork := b.dependencies()
		if len(cached) > 0 {
			openWork.Seat.Situation = func(string, time.Time) (SituationRecord, error) { return cached[0], nil }
		}
		return seatRecheck(b.root, TickConfig{Now: b.now, ProviderHome: testprovider.Home(b.root), Situation: b.situation}, census, openWork, records)
	}
	return startSeatWithDependencies(b.root, selection, dependencies)
}

// end declares a seat launch terminal with the result document given.
func (b *seatBed) end(id, state, result string) {
	b.t.Helper()
	dir := filepath.Join(b.root, "launch-state", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.t.Fatal(err)
	}
	path := filepath.Join(dir, "result.json")
	if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
		b.t.Fatal(err)
	}
	b.launcher.states[id] = SeatLaunchState{Home: testprovider.Home(b.root), Runtime: "claude", Model: "fixture-model", Found: true, Terminal: true, State: state, ResultPath: path, FinishedAt: b.now.Format(time.RFC3339Nano)}
}

func (b *seatBed) records() []SeatRecord {
	b.t.Helper()
	records, err := readSeatRecords(b.root)
	if err != nil {
		b.t.Fatal(err)
	}
	return records
}

var deadWorkers = Workers{CensusComplete: true}

func seatReadyGoal(id, next string) *goal.GoalFile {
	return approvedStewardGoal(id, "Serve "+id, next, "2026-08-23T00:00:00Z")
}

func seatClaimedGoal(id, lineage string) *goal.GoalFile {
	file := seatReadyGoal(id, "Continue it.")
	file.State = goal.StateClaimed
	file.Revision = 3
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: seatBedMachine, ClaimEpoch: 1}
	file.Claimed = &goal.ClaimRecord{Machine: seatBedMachine, Lineage: lineage, At: "2026-08-23T01:00:00Z", Revision: 3}
	file.History = append(file.History, goal.HistoryLine{At: "2026-08-23T01:00:00Z",
		Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC1", seatBedMachine, lineage), Verb: "claim",
		Actor: seatBedMachine + "+" + lineage, Targets: []string{id}, Keep: -1})
	return file
}

// seatLandingGoal is a goal the seat lineage built and marked land-ready at
// the time given, at the tier given.
func seatLandingGoal(id string, tier uint8, landedReadyAt time.Time) *goal.GoalFile {
	file := seatClaimedGoal(id, SeatLineage)
	file.Tier = tier
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, tier, *file.Budget)
	at := landedReadyAt.UTC().Format(time.RFC3339)
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC2", seatBedMachine, SeatLineage)
	file.History = append(file.History, goal.HistoryLine{At: at, Opid: opid, Verb: "land-ready",
		Actor: seatBedMachine + "+" + SeatLineage, Targets: []string{id}, Keep: -1})
	file.Revision++
	file.Landing = &goal.LandingRecord{At: at, Opid: opid}
	return file
}

const seatReviewTip = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"
const seatReviewRecord = "plans/reviews/review-of-held.md"

func seatHumanLine(file *goal.GoalFile, at time.Time, reason string) {
	file.History = append(file.History, goal.HistoryLine{At: at.UTC().Format(time.RFC3339),
		Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC3", seatBedMachine, "human"), Verb: "review",
		Actor: "human:Wido", Targets: []string{file.Id}, Reason: reason, Keep: -1})
	file.Revision++
}

func seatCleared(file *goal.GoalFile, at time.Time) {
	seatHumanLine(file, at, goal.ReviewLine{Verdict: goal.VerdictClearToLand, Tip: seatReviewTip, Record: seatReviewRecord, By: "Wido"}.Reason())
}

func seatHeldBySitting(file *goal.GoalFile, at time.Time) {
	seatHumanLine(file, at, goal.SittingReason(true, seatReviewRecord, "Wido"))
}

func pendingMessages(t *testing.T, root string) []string {
	t.Helper()
	pending, err := PendingNotifications(root)
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, n := range pending {
		messages = append(messages, n.Message)
	}
	return messages
}

func stewardshipBed(t *testing.T, files ...*goal.GoalFile) (*seatBed, string) {
	t.Helper()
	b := newSeatBed(t, files...)
	b.now = time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	home := t.TempDir()
	b.boardHome = home
	src := withEmptyHost(t, SituationSources{Now: func() time.Time { return b.now },
		LaneHome: func() (string, error) { return home, nil }, BoardHome: func() (string, error) { return home, nil },
		ClaimLaneState: func(string, string, string) (string, error) { return "", nil }})
	src.Machine = func(string) (string, error) { return seatBedMachine, nil }
	src.Project = func(goal.Endpoint, bool, time.Time) (goal.Projection, error) { return b.projection(b.now), nil }
	b.situation = func(root string, _ time.Time) Situation {
		s := readSituation(t, root, src)
		// The seat bed keeps its accepted ledger in memory, without Git.
		s.Next, _ = goal.Next(b.projection(b.now), seatBedMachine)
		s.Asks = nil
		s.readAsks(src, b.now)
		return s
	}
	return b, home
}

func stewardAsk(t *testing.T, b *seatBed, home, id string, deadline time.Duration) {
	t.Helper()
	composePublish(t, home, board.Request{ID: id, Kind: board.KindAsk, From: board.Sender{Machine: "peer"},
		To: board.Address{Machine: seatBedMachine}, Text: "Please decide", Deadline: deadline}, b.now.Add(-time.Hour))
}

func stewardSelected(t *testing.T, result TickResult) SeatSelection {
	t.Helper()
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Kind != "steward" || result.Seat.Goal != "" || result.Seat.Held {
		t.Fatalf("expected stewardship with no goal: %+v %+v", result.Decision, result.Seat)
	}
	return *result.Seat
}

func TestAStewardSessionsProgressIsItsHandledDecision(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, outcome, dispose, lineage, machine string
		before, reply, receipt, progress         bool
	}{
		{name: "confirmed", outcome: "confirmed", progress: true},
		{name: "partial", outcome: "partial", progress: true},
		{name: "unchanged", outcome: "unchanged", progress: true},
		{name: "refused", outcome: "refused"},
		{name: "failed", outcome: "failed"},
		{name: "wait", dispose: humanauthority.DisposeWait, progress: true},
		{name: "ask", dispose: humanauthority.DisposeAsk, progress: true},
		{name: "before", outcome: "confirmed", before: true},
		{name: "another lineage", outcome: "confirmed", lineage: "builder"},
		{name: "another machine", outcome: "confirmed", machine: "elsewhere"},
		{name: "read receipt", receipt: true},
		{name: "reply", reply: true, progress: true},
		{name: "other lineage reply", reply: true, lineage: "builder"},
		{name: "other machine reply", reply: true, machine: "elsewhere"},
		{name: "old reply", reply: true, before: true},
	} {
		for _, kind := range []string{"steward", ""} {
			t.Run(row.name+"/"+kind, func(t *testing.T) {
				t.Parallel()
				b, home := stewardshipBed(t, seatReadyGoal("ready", "Build it."))
				selection := SeatSelection{Kind: kind, Subject: "subject"}
				if kind == "" {
					selection.Goal = "ready"
				}
				record := b.start(&selection)
				at := b.now.Add(time.Second)
				if row.before {
					at = b.now.Add(-time.Second)
				}
				lineage, machine := SeatLineage, seatBedMachine
				if row.lineage != "" {
					lineage = row.lineage
				}
				if row.machine != "" {
					machine = row.machine
				}
				switch {
				case row.reply:
					composePublish(t, home, board.Request{ID: "ask", Kind: board.KindAsk, From: board.Sender{Machine: "peer"}, To: board.Address{Machine: machine}, Text: "Decide"}, at.Add(-time.Minute))
					composePublish(t, home, board.Request{Kind: board.KindReply, Thread: "ask", From: board.Sender{Machine: machine, Lineage: lineage}, To: board.Address{Machine: "peer"}, Text: "Handled"}, at)
				case row.receipt:
					if err := RecordSituationRead(b.root, record.LaunchID, at, FirstThing{Kind: "ask", Subject: "subject"}); err != nil {
						t.Fatal(err)
					}
				default:
					d := humanauthority.Decision{ID: "decision", At: at, Lineage: lineage, Machine: machine, Outcome: row.outcome}
					if row.dispose != "" {
						d.Disposition = &humanauthority.Disposition{Kind: row.dispose, WaitUntil: at.Add(time.Hour)}
					}
					if err := humanauthority.AppendDecision(home, d); err != nil {
						t.Fatal(err)
					}
				}
				b.now = b.now.Add(time.Minute)
				b.end(record.LaunchID, "completed", `{"is_error":false}`)
				b.tick(deadWorkers)
				got := b.records()[0]
				if (got.Outcome == SeatProgress) != row.progress {
					t.Fatalf("progress=%v: %+v", row.progress, got)
				}
			})
		}
	}
}

func stewardStopBed(t *testing.T, state string) (*seatBed, *string, *string) {
	t.Helper()
	b, _ := stewardshipBed(t)
	b.situation = nil
	subject, occurrence := "overdue-goal held", "first"
	b.page = func(string, time.Time) (SituationRecord, error) {
		return SituationRecord{Composition: Composition{First: FirstThing{Kind: "row", Subject: subject, Command: "metasystem decision record --occurrence " + occurrence + " --dispose wait"}}}, nil
	}
	for i, ago := range []time.Duration{20 * time.Minute, 10 * time.Minute} {
		record := SeatRecord{Schema: 1, LaunchID: fmt.Sprintf("ended-%d", i), Kind: "steward", Subject: subject, Occurrence: occurrence,
			Machine: seatBedMachine, StartedAt: b.now.Add(-ago).Format(seatStartedAtLayout), ReapedAt: b.now.Add(-ago + time.Minute).Format(time.RFC3339), LaunchState: state, Outcome: SeatNoProgress}
		if err := writeSeatRecord(b.root, record); err != nil {
			t.Fatal(err)
		}
	}
	return b, &subject, &occurrence
}

func TestANewOccurrenceStartsInsideTheHour(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"same", "occurrence", "subject"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			b, subject, occurrence := stewardStopBed(t, "completed")
			if change == "occurrence" {
				*occurrence = "second"
			}
			if change == "subject" {
				*subject = "overdue-goal another"
			}
			r := b.tick(deadWorkers)
			if change == "same" {
				want := "the steward started twice for " + *subject + " without progress in the last hour; metasystem machine revive " + b.root + " lifts the stop, or a new occurrence starts again"
				if r.Seat != nil || !strings.Contains(r.Decision.Reason, want) {
					t.Fatalf("same occurrence must name its working remedy: %+v", r.Decision)
				}
				return
			}
			if record, err := b.startUnder(stewardSelected(t, r), fakeCensus{workers: deadWorkers}); err != nil || record.LaunchID == "" {
				t.Fatalf("new stewardship work must start: %+v %v", record, err)
			}
		})
	}
}

type stewardPersonTree struct{}

func (stewardPersonTree) SessionLeader(int64) (int64, error) { return 10, nil }
func (stewardPersonTree) Read(pid int64) (humanauthority.Snapshot, error) {
	parent, command, terminal := int64(0), "launchd", ""
	switch pid {
	case 10:
		parent, command, terminal = 1, "login", "tty-1"
	case 20:
		parent, command, terminal = 10, "zsh", "tty-1"
	case 1:
	default:
		return humanauthority.Snapshot{}, os.ErrNotExist
	}
	return humanauthority.Snapshot{Exact: identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), ArgvKnown: true, Argv: []string{command}},
		Executable: "/fixture/bin/" + command, ExecutableKnown: true, OwnerUID: 501, OwnerKnown: true,
		ParentPID: parent, ParentKnown: true, TerminalID: terminal, TerminalKnown: true}, nil
}

func TestAPersonReviveLiftsTheStewardStop(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"completed", "failed", "cancelled", "missing"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, _, _ := stewardStopBed(t, state)
			b.root = canonicalPath(b.root)
			if r := b.tick(deadWorkers); r.Seat != nil || !strings.Contains(r.Decision.Reason, "hour") {
				t.Fatalf("expected the steward stop: %+v", r.Decision)
			}
			closed := personRevivesTheSteward(t, b, state)
			if _, ok := closed.(SeatRevivalClosed); !ok {
				t.Fatalf("person's revive must close the steward launch: %v", closed)
			}
			data, err := os.ReadFile(EvidencePath(b.root))
			if err != nil || !strings.Contains(string(data), `"steward-stop-cleared"`) || !strings.Contains(string(data), `"by": "Wido"`) || !strings.Contains(string(data), b.root) || !strings.Contains(string(data), b.now.Format(time.RFC3339)) {
				t.Fatalf("missing durable person clearance: %s %v", data, err)
			}
			want := "steward launch ended-1 ended " + state
			if state == "missing" {
				want = "steward launch ended-1 record is missing"
			}
			want += "; the steward stop is lifted by your word; the steward's next tick selects again"
			if closed.Error() != want {
				t.Fatalf("person's revive must explain the steward close: got %q, want %q", closed, want)
			}
			b.now = b.now.Add(time.Second)
			if record, err := b.startUnder(stewardSelected(t, b.tick(deadWorkers)), fakeCensus{workers: deadWorkers}); err != nil || record.LaunchID == "" {
				t.Fatalf("person's revive did not lift the stop: %+v %v", record, err)
			}
		})
	}
}

// personRevivesTheSteward runs a person's machine revive at the enrolled
// terminal against the bed's last steward launch in the given state and
// returns what the revive answered.
func personRevivesTheSteward(t *testing.T, b *seatBed, state string) error {
	t.Helper()
	bin := filepath.Join(b.root, "fixture-engine")
	if err := testexec.WriteFile(bin, []byte("fixture engine"), 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(b.root), InstallIdentity{RepoIdentity: b.root, Generation: 1, InstallPath: bin, InstallDigest: digest}); err != nil {
		t.Fatal(err)
	}
	d := *b.seatDependencies()
	d.Census = fakeCensus{workers: deadWorkers}
	d.RegistryPath = filepath.Join(b.root, "host-registry")
	frame, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": b.root, "ownerTag": "revive", "at": b.now.Format(time.RFC3339), "generation": 1, "watcherTag": "w", "reaperTag": "r", "retiredThrough": 0})
	if err := registry.AppendFrame(d.RegistryPath, frame); err != nil {
		t.Fatal(err)
	}
	if _, err := humanauthority.Enroll(b.root, 20, stewardPersonTree{}, "Wido", b.now); err != nil {
		t.Fatal(err)
	}
	d.Person, err = humanauthority.Prove(b.root, 20, stewardPersonTree{}, b.now)
	if err != nil || !d.Person.EnrolledTerminalFor(b.root) {
		t.Fatalf("person proof: %+v %v", d.Person, err)
	}
	b.launcher.states = map[string]SeatLaunchState{}
	if state != "missing" {
		b.launcher.states["ended-1"] = SeatLaunchState{Found: true, Terminal: true, State: state}
	}
	_, err = startSeatWithDependencies(b.root, SeatSelection{}, d)
	return err
}

// Past the three-start cap, a person's revive lifts the steward cap as well:
// the ladder counts only steward records started after the clearance.
func TestAPersonReviveLiftsTheStewardCap(t *testing.T) {
	t.Parallel()
	b, subject, occurrence := stewardStopBed(t, "failed")
	third := SeatRecord{Schema: 1, LaunchID: "ended-2", Kind: "steward", Subject: *subject, Occurrence: *occurrence,
		Machine: seatBedMachine, StartedAt: b.now.Add(-30 * time.Minute).Format(seatStartedAtLayout), ReapedAt: b.now.Add(-29 * time.Minute).Format(time.RFC3339), LaunchState: "failed", Outcome: SeatNoProgress}
	if err := writeSeatRecord(b.root, third); err != nil {
		t.Fatal(err)
	}
	b.root = canonicalPath(b.root)
	if r := b.tick(deadWorkers); r.Seat != nil || !strings.Contains(r.Decision.Reason, "no steward starts for this occurrence") {
		t.Fatalf("expected the steward cap: %+v", r.Decision)
	}
	if closed := personRevivesTheSteward(t, b, "failed"); closed == nil || !strings.Contains(closed.Error(), "the steward stop is lifted by your word") {
		t.Fatalf("person's revive must lift the stop: %v", closed)
	}
	b.now = b.now.Add(time.Second)
	if record, err := b.startUnder(stewardSelected(t, b.tick(deadWorkers)), fakeCensus{workers: deadWorkers}); err != nil || record.LaunchID == "" {
		t.Fatalf("person's revive did not lift the cap: %+v %v", record, err)
	}
}

func TestAnOutstandingAskStartsTheStewardBesideAWaitingClaim(t *testing.T) {
	t.Parallel()
	for _, deadline := range []time.Duration{time.Minute, 0} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			t.Parallel()
			held := seatClaimedGoal("held", SeatLineage)
			held.NextStep = "WAITING ON THE HUMAN: which one?"
			b, home := stewardshipBed(t, held)
			before := string(goal.RenderFile(held))
			composePublish(t, home, board.Request{ID: "ask-held", Kind: board.KindAsk, From: board.Sender{Machine: "peer"},
				To: board.Address{Goal: "held"}, Text: "Please decide", Deadline: deadline}, b.now.Add(-time.Hour))
			selection := stewardSelected(t, b.tick(deadWorkers))
			if selection.Subject != "ask-held" || strings.Join(selection.SeatHeld, ",") != "held" {
				t.Fatalf("held claims: %+v", selection)
			}
			record, err := b.startUnder(selection, fakeCensus{workers: deadWorkers})
			if err != nil || record.LaunchID == "" || record.Kind != "steward" || record.Subject != "ask-held" || len(b.launcher.starts) != 1 {
				t.Fatalf("start: %+v %v", record, err)
			}
			if string(goal.RenderFile(held)) != before || len(pendingMessages(t, b.root)) != 0 {
				t.Fatal("stewardship changed a claim or queued a notice")
			}
		})
	}
}

type stewardCensus struct {
	calls   int
	workers Workers
}

func (c *stewardCensus) Workers(string) (Workers, error) { c.calls++; return c.workers, nil }

func TestAStewardStartNeedsNoGoal(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"empty", "running", "due held delegate", "no reader", "unreadable"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b, home := stewardshipBed(t)
			b.busy = row == "running" || row == "due held delegate"
			if row == "due held delegate" {
				b.put(seatClaimedGoal("held", SeatLineage))
			}
			stewardAsk(t, b, home, "ask-seat", 0)
			census := &stewardCensus{workers: deadWorkers}
			if row == "no reader" || row == "unreadable" {
				b.situation = nil
				if row == "unreadable" {
					b.page = func(string, time.Time) (SituationRecord, error) {
						return SituationRecord{}, fmt.Errorf("page unreadable")
					}
				}
				if r := b.tickWith(census); r.Seat != nil || census.calls != 0 {
					t.Fatalf("unavailable page admitted a seat: %+v; census %d", r.Decision, census.calls)
				}
				return
			}
			if b.busy {
				census.workers.Live = 1
			}
			selection := stewardSelected(t, b.tickWith(census))
			if census.calls != 1 {
				t.Fatalf("census calls: %d", census.calls)
			}
			if _, err := b.startUnder(selection, census); err != nil || len(b.launcher.starts) != 1 {
				t.Fatalf("start: %v", err)
			}
			if row == "due held delegate" {
				b.end(b.records()[0].LaunchID, "completed", `{"type":"result","is_error":false}`)
				composePublish(t, home, board.Request{Kind: board.KindReply, Thread: "ask-seat", To: board.Address{Machine: "peer"}, From: board.Sender{Machine: seatBedMachine}, Text: "Handled"}, b.now)
				b.now = b.now.Add(3 * time.Minute)
				if r := b.tickWith(census); r.Seat != nil || len(b.launcher.starts) != 1 {
					t.Fatalf("busy goal succeeded after reply: %+v", r)
				}
			}
		})
	}
}

func TestNoStewardStartsOnDoubtOrBesideAMain(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"main", "helm", "doubt", "off", "fence", "clear"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b, home := stewardshipBed(t, seatClaimedGoal("held", SeatLineage))
			b.goals["held"].NextStep = "WAITING ON THE HUMAN: which one?"
			stewardAsk(t, b, home, "ask-seat", 0)
			workers := deadWorkers
			switch row {
			case "main":
				workers.Live, workers.LiveSeatMains = 1, 1
			case "doubt":
				workers.CensusComplete = false
			case "off":
				b.launcher.off = "off"
			case "fence":
				b.fence = "closed"
			}
			result := b.tick(workers)
			if row == "clear" || row == "helm" {
				selection := stewardSelected(t, result)
				if row == "helm" {
					takeHelmFixture(t, b.root)
				}
				_, err := b.startUnder(selection, fakeCensus{workers: workers})
				if err != nil {
					t.Fatal(err)
				}
			} else if result.Seat != nil {
				t.Fatalf("guard admitted a seat: %+v", result)
			}
			want := 0
			if row == "clear" {
				want = 1
			}
			if len(b.launcher.starts) != want || len(b.records()) != want {
				t.Fatalf("starts: %d; records: %d", len(b.launcher.starts), len(b.records()))
			}
		})
	}
}

func TestTheStewardCapCountsPerOccurrence(t *testing.T) {
	t.Parallel()
	b, home := stewardshipBed(t)
	stewardAsk(t, b, home, "ask-one", 0)
	for range 3 {
		selection := stewardSelected(t, b.tick(deadWorkers))
		record, err := b.startUnder(selection, fakeCensus{workers: deadWorkers})
		if err != nil {
			t.Fatal(err)
		}
		b.end(record.LaunchID, "completed", `{"type":"result","is_error":false}`)
		b.now = b.now.Add(70 * time.Minute)
	}
	r := b.tick(deadWorkers)
	if r.Decision.Action != ActNotify || r.Seat != nil || !strings.Contains(r.Decision.Reason, "ask-one") {
		t.Fatalf("cap: %+v", r)
	}
	composePublish(t, home, board.Request{Kind: board.KindReply, Thread: "ask-one", To: board.Address{Machine: "peer"}, From: board.Sender{Machine: seatBedMachine}, Text: "Handled"}, b.now)
	stewardAsk(t, b, home, "ask-two", 0)
	stewardSelected(t, b.tick(deadWorkers))
	t.Run("different subjects start inside the hour", func(t *testing.T) {
		t.Parallel()
		b, home := stewardshipBed(t)
		for _, id := range []string{"one", "two"} {
			stewardAsk(t, b, home, id, 0)
			selection := stewardSelected(t, b.tick(deadWorkers))
			record, err := b.startUnder(selection, fakeCensus{workers: deadWorkers})
			if err != nil {
				t.Fatal(err)
			}
			if ev, err := LoadEvidence(EvidencePath(b.root)); err != nil || ev.AbnormalCount != 0 {
				t.Fatalf("different subject reused a restart: %+v %v", ev, err)
			}
			b.end(record.LaunchID, "completed", `{"type":"result","is_error":false}`)
			composePublish(t, home, board.Request{Kind: board.KindReply, Thread: id, To: board.Address{Machine: "peer"}, From: board.Sender{Machine: seatBedMachine}, Text: "Handled"}, b.now)
			b.now = b.now.Add(10 * time.Minute)
		}
		stewardAsk(t, b, home, "three", 0)
		if record, err := b.startUnder(stewardSelected(t, b.tick(deadWorkers)), fakeCensus{workers: deadWorkers}); err != nil || record.LaunchID == "" {
			t.Fatalf("different subject must start: %+v %v", record, err)
		}
	})
	t.Run("a goal ending is a different start", func(t *testing.T) {
		t.Parallel()
		b, home := stewardshipBed(t, seatClaimedGoal("held", SeatLineage))
		first := b.tick(deadWorkers)
		record, err := b.startUnder(*first.Seat, fakeCensus{workers: deadWorkers})
		if err != nil {
			t.Fatal(err)
		}
		b.end(record.LaunchID, "completed", `{"type":"result","is_error":false}`)
		b.goals["held"].NextStep = "WAITING ON THE HUMAN: which one?"
		b.now = b.now.Add(time.Minute)
		stewardAsk(t, b, home, "ask-held", 0)
		selection := stewardSelected(t, b.tick(deadWorkers))
		record, err = b.startUnder(selection, fakeCensus{workers: deadWorkers})
		if err != nil {
			t.Fatal(err)
		}
		ev, err := LoadEvidence(EvidencePath(b.root))
		if err != nil || ev.AbnormalCount != 0 {
			t.Fatalf("goal ending spent the steward's restart history: %+v %v", ev, err)
		}
	})
	t.Run("a routed occurrence has its own count", func(t *testing.T) {
		t.Parallel()
		b, _ := stewardshipBed(t)
		b.situation = nil
		occurrence := "first"
		b.page = func(string, time.Time) (SituationRecord, error) {
			return SituationRecord{Composition: Composition{First: FirstThing{Kind: "row", Subject: "overdue-goal held", Command: "metasystem decision record --occurrence " + occurrence + " --dispose wait"}}}, nil
		}
		for range 3 {
			selection := stewardSelected(t, b.tick(deadWorkers))
			record, err := b.startUnder(selection, fakeCensus{workers: deadWorkers})
			if err != nil || record.Occurrence != occurrence {
				t.Fatalf("occurrence start: %+v %v", record, err)
			}
			b.end(record.LaunchID, "completed", `{"type":"result","is_error":false}`)
			b.now = b.now.Add(70 * time.Minute)
		}
		if r := b.tick(deadWorkers); r.Seat != nil || !strings.Contains(r.Decision.Reason, "overdue-goal held") {
			t.Fatalf("occurrence cap: %+v", r.Decision)
		}
		occurrence = "second"
		if selection := stewardSelected(t, b.tick(deadWorkers)); selection.Occurrence != occurrence {
			t.Fatalf("new occurrence: %+v", selection)
		}
	})
	t.Run("earlier goal restarts keep the rolling hour limit", func(t *testing.T) {
		t.Parallel()
		b, home := stewardshipBed(t, seatClaimedGoal("held", SeatLineage))
		first := b.tick(deadWorkers)
		record, err := b.startUnder(*first.Seat, fakeCensus{workers: deadWorkers})
		if err != nil {
			t.Fatal(err)
		}
		b.end(record.LaunchID, "completed", `{"type":"result","is_error":false}`)
		b.goals["held"].NextStep = "WAITING ON THE HUMAN: which one?"
		b.now = b.now.Add(time.Minute)
		stewardAsk(t, b, home, "ask-seat", 0)
		ev := Evidence{CurrentSeat: record.LaunchID, AbnormalCount: 2, Abnormal: [2]AbnormalRestart{
			{At: b.now.Add(-20 * time.Minute), Class: "failed"},
			{At: b.now.Add(-10 * time.Minute), Class: "cancelled"},
		}}
		if err := SaveEvidence(b.root, EvidencePath(b.root), ev); err != nil {
			t.Fatal(err)
		}
		if r := b.tick(deadWorkers); r.Seat != nil || !strings.Contains(r.Decision.Reason, "two automatic abnormal restarts were attempted in the rolling hour") {
			t.Fatalf("earlier restart limit: %+v", r.Decision)
		}
	})
}

func TestAStewardStartRereadsItsSubjectUnderTheLock(t *testing.T) {
	t.Parallel()
	for _, another := range []bool{false, true} {
		t.Run(fmt.Sprint(another), func(t *testing.T) {
			t.Parallel()
			b, home := stewardshipBed(t)
			stewardAsk(t, b, home, "ask-seat", 0)
			selection := stewardSelected(t, b.tick(deadWorkers))
			data, err := os.ReadFile(situationPagePath(b.root))
			if err != nil {
				t.Fatal(err)
			}
			var cached SituationRecord
			if err := json.Unmarshal(data, &cached); err != nil || cached.Composition.First.Subject != "ask-seat" {
				t.Fatalf("cached subject: %+v %v", cached, err)
			}
			composePublish(t, home, board.Request{Kind: board.KindReply, Thread: "ask-seat", To: board.Address{Machine: "peer"}, From: board.Sender{Machine: seatBedMachine}, Text: "Handled"}, b.now)
			if another {
				stewardAsk(t, b, home, "ask-other", 0)
			}
			record, err := b.startUnder(selection, fakeCensus{workers: deadWorkers}, cached)
			if err == nil || !strings.Contains(err.Error(), "grounds changed") || record.LaunchID != "" || len(b.records()) != 0 || len(b.launcher.starts) != 0 {
				t.Fatalf("stale start: %+v %v", record, err)
			}
		})
	}
}

// Test 1.
func TestReadyWorkWithNoSeatStartsOne(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "alpha" || result.Seat.Held {
		t.Fatalf("ready work with no seat starts one naming it: %+v %+v", result.Decision, result.Seat)
	}
	result = bed.tick(Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true})
	if result.Decision.Action != ActNotify || result.Seat != nil {
		t.Fatalf("a live seat main owns ready work through its own Stop: %+v %+v", result.Decision, result.Seat)
	}
}

// Test 2.
func TestReadyWorkNeverSpawnsOnDoubt(t *testing.T) {
	t.Parallel()
	for name, workers := range map[string]Workers{
		"incomplete census": {},
		"one untracked":     {Untracked: 1, CensusComplete: true},
		"one unprovable":    {Unprovable: 1, CensusComplete: true},
	} {
		t.Run(name, func(t *testing.T) {
			bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
			result := bed.tick(workers)
			if result.Decision.Action != ActNotify || result.Seat != nil || result.Decision.Verdict != VerdictUnknown {
				t.Fatalf("doubt about the workers never starts a seat: %+v %+v", result.Decision, result.Seat)
			}
		})
	}
}

// capSeats records n reaped seats named for the goal under its approval that
// ended without progress.
func capSeats(t *testing.T, bed *seatBed, id string, n int) {
	t.Helper()
	opid := bed.goals[id].Approved.Opid
	for i := 0; i < n; i++ {
		record := SeatRecord{Schema: 1, LaunchID: "seat-cap-" + id + string(rune('a'+i)), Goal: id, ApprovalOpid: opid,
			Machine: seatBedMachine, StartedAt: bed.now.Add(time.Duration(i-10) * time.Minute).Format(time.RFC3339),
			ReapedAt: bed.now.Format(time.RFC3339), LaunchState: "completed", Outcome: SeatNoProgress}
		if err := writeSeatRecord(bed.root, record); err != nil {
			t.Fatal(err)
		}
	}
}

// Test 3.
func TestReadyWorkHonoursTheGuardsInOrder(t *testing.T) {
	t.Parallel()
	t.Run("an unreaped seat launch holds before everything", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		bed.start(&SeatSelection{Goal: "alpha", Ready: []string{"alpha"}})
		if _, err := testprovider.Record(bed.root, "overloaded", "API Error: 529", "test", bed.now); err != nil {
			t.Fatal(err)
		}
		result := bed.tick(Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true})
		if result.Decision.Action != ActNone || !strings.Contains(result.Decision.Reason, "seat-") {
			t.Fatalf("an unreaped seat launch is ActNone naming it: %+v", result.Decision)
		}
	})
	t.Run("an outage notifies before the cap", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		capSeats(t, bed, "alpha", 3)
		if _, err := testprovider.Record(bed.root, "overloaded", "API Error: 529", "test", bed.now); err != nil {
			t.Fatal(err)
		}
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNotify || !strings.Contains(result.Decision.Reason, "provider") || result.Seat != nil {
			t.Fatalf("a standing outage holds with a notification before the cap: %+v", result.Decision)
		}
	})
	t.Run("a closed fence is ActNone with its reason", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		capSeats(t, bed, "alpha", 3)
		bed.fence = "the checkout was stopped by Wido"
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNone || !strings.Contains(result.Decision.Reason, bed.fence) || result.Seat != nil {
			t.Fatalf("a closed fence starts nothing and says why: %+v", result.Decision)
		}
		bed.fence = ""
		if _, err := startSeatWithDependencies(bed.root, SeatSelection{Goal: "alpha"}, *bed.seatDependencies()); err != nil {
			t.Fatal(err)
		}
		bed.fence = "the checkout was stopped by Wido"
		before := len(bed.launcher.starts)
		if _, err := startSeatWithDependencies(bed.root, SeatSelection{Goal: "alpha"}, *bed.seatDependencies()); err == nil || !strings.Contains(err.Error(), bed.fence) || len(bed.launcher.starts) != before {
			t.Fatalf("the steward's pre-start read refuses at a closed fence: %v", err)
		}
	})
	t.Run("a capped named goal notifies naming the reset", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		opid := bed.goals["alpha"].Approved.Opid
		capSeats(t, bed, "alpha", 2)
		if capped, err := SeatCapped(bed.root, "alpha", opid); err != nil || capped {
			t.Fatalf("two seats without progress do not cap: %v %v", capped, err)
		}
		capSeats(t, bed, "alpha", 3)
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNotify || result.Seat != nil ||
			!strings.Contains(result.Decision.Reason, "3 seats ended without progress on alpha") ||
			!strings.Contains(result.Decision.Reason, personRemedyMark+"metasystem goal approve alpha\nmetasystem goal approve alpha approves it afresh") ||
			strings.Contains(result.Decision.Reason, "unapprove") {
			t.Fatalf("a capped goal starts no seat and names the reset: %+v", result.Decision)
		}
		if capped, err := SeatCapped(bed.root, "alpha", opid); err != nil || !capped {
			t.Fatalf("the steward's own records read capped: %v %v", capped, err)
		}
		if capped, err := SeatCapped(bed.root, "alpha", "another-approval"); err != nil || capped {
			t.Fatalf("another approval is not capped: %v %v", capped, err)
		}
	})
	t.Run("the helm holds the start", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		takeHelmFixture(t, bed.root)
		if _, err := startSeatWithDependencies(bed.root, SeatSelection{Goal: "alpha"}, *bed.seatDependencies()); err != nil {
			t.Fatal(err)
		}
		if len(bed.launcher.starts) != 0 {
			t.Fatalf("a seat at the helm starts nothing: %+v", bed.launcher.starts)
		}
	})
	t.Run("the delegate revival cap is not consulted for seats", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		if err := SaveEvidence(bed.root, EvidencePath(bed.root), Evidence{Marks: Marks{HeadOid: "h", OpidDigest: "d"}, DryRevivals: 3}); err != nil {
			t.Fatal(err)
		}
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil {
			t.Fatalf("DryRevivals at its cap still starts a seat: %+v", result.Decision)
		}
	})
}

// Test 4.
func TestAForeignClaimOnThisMachineStartsNoSeatAndOursStartsASuccessor(t *testing.T) {
	t.Parallel()
	t.Run("a person's claim beside ready work notifies naming it", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."), seatClaimedGoal("held", "coordinator"))
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, "held") {
			t.Fatalf("a foreign claim on this machine starts no seat: %+v", result.Decision)
		}
	})
	t.Run("the seat lineage's claim beside ready work starts its successor", func(t *testing.T) {
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."), seatClaimedGoal("held", SeatLineage))
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "held" || !result.Seat.Held {
			t.Fatalf("our dead seat's claim is succeeded first: %+v %+v", result.Decision, result.Seat)
		}
	})
	t.Run("the seat lineage's claim alone is owned work that starts its successor", func(t *testing.T) {
		bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
		result := bed.tick(deadWorkers)
		if !strings.Contains(result.OpenWork, "claimed goals") || result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "held" {
			t.Fatalf("owned seat work proven dead starts the successor seat: %q %+v %+v", result.OpenWork, result.Decision, result.Seat)
		}
	})
	t.Run("a person's owned claim keeps today's path", func(t *testing.T) {
		bed := newSeatBed(t, seatClaimedGoal("held", "coordinator"))
		result := bed.tick(deadWorkers)
		if result.Seat != nil {
			t.Fatalf("a person's claim is the owned-work ladder's business: %+v %+v", result.Decision, result.Seat)
		}
	})
	t.Run("a live seat main starts nothing", func(t *testing.T) {
		bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
		result := bed.tick(Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true})
		if result.Decision.Action == ActRevive || result.Seat != nil {
			t.Fatalf("a live seat main is never displaced: %+v", result.Decision)
		}
	})
}

// Test 5.
func TestOnlyHumanWordGoalsStartNoSeat(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t,
		seatReadyGoal("first", "WAITING ON THE HUMAN: which design."),
		seatReadyGoal("second", "RULING NEEDED on the scope."))
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActNotify || result.Seat != nil ||
		!strings.Contains(result.Decision.Reason, "first") || !strings.Contains(result.Decision.Reason, "second") {
		t.Fatalf("goals waiting on a human word start no seat and are named: %+v", result.Decision)
	}
	bed.put(seatReadyGoal("third", "Build it."))
	result = bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "third" {
		t.Fatalf("the free goal is named, not the head: %+v %+v", result.Decision, result.Seat)
	}
	record := bed.start(result.Seat)
	brief, err := os.ReadFile(bed.launcher.starts[0].Brief)
	if err != nil {
		t.Fatal(err)
	}
	if record.Goal != "third" || !strings.Contains(string(brief), "metasystem goal claim third") || strings.Contains(string(brief), "first") {
		t.Fatalf("the brief and the record name the free goal: %+v\n%s", record, brief)
	}
}

// Test 6.
func TestTickReadsTheCensusForClaimableWork(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	calls := &atomic.Int32{}
	result := bed.tickWith(countingCensus{calls: calls})
	if calls.Load() != 1 || !strings.Contains(result.OpenWork, "claimable") {
		t.Fatalf("claimable work reads the census once: calls=%d %q", calls.Load(), result.OpenWork)
	}
}

// seatUsageLimitResult is the Claude CLI's usage-limit ending, which the
// classifier names provider-limit; seatOverloadResult is the overload ending.
const (
	seatUsageLimitResult = `{"type":"result","is_error":true,"result":"Claude AI usage limit reached|1759262400"}`
	seatOverloadResult   = `{"type":"result","is_error":true,"result":"API Error: 529 {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\"}}"}`
)

// Test 12. A seat ended by the provider's usage limit, and one ended by an
// overload, are provider weather: the mark is fed with the classifier's
// class (internal/outage, test 18), the ladder holds, and past the horizon
// the successor continues the held goal with the count at zero.
func TestProviderLimitAfterClaimRecoversWithoutAPerson(t *testing.T) {
	t.Parallel()
	for _, weather := range []struct{ name, result, class string }{
		{"usage limit", seatUsageLimitResult, outage.ProviderLimit},
		{"overload", seatOverloadResult, "overloaded"},
	} {
		t.Run(weather.name, func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatReadyGoal("held", "Build it."))
			result := bed.tick(deadWorkers)
			record := bed.start(result.Seat)
			// Seat one claims G and its provider stops it.
			bed.put(seatClaimedGoal("held", SeatLineage))
			bed.end(record.LaunchID, "failed", weather.result)
			result = bed.tick(deadWorkers)
			mark, standing := testprovider.StandingAt(bed.root, bed.now)
			if !standing || mark.Source != record.LaunchID || mark.LastClass != weather.class {
				t.Fatalf("a provider-limit ending feeds the outage mark with class %s: %+v %v", weather.class, mark, standing)
			}
			if result.Decision.Action != ActNotify || result.Seat != nil {
				t.Fatalf("the ladder holds while the mark stands: %+v", result.Decision)
			}
			records := bed.records()
			if len(records) != 1 || records[0].Outcome != SeatProviderLimit {
				t.Fatalf("the seat is reaped as provider-limit: %+v", records)
			}
			bed.now = bed.now.Add(outage.Horizon + time.Minute)
			result = bed.tick(deadWorkers)
			if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "held" || !result.Seat.Held {
				t.Fatalf("past the horizon the successor starts and continues G: %+v %+v", result.Decision, result.Seat)
			}
			if count := SeatNoProgressCount(bed.records(), "held", bed.goals["held"].Approved.Opid); count != 0 {
				t.Fatalf("provider weather never counts: %d", count)
			}
		})
	}

	t.Run("held work is selected before the human-word filter", func(t *testing.T) {
		bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage), seatReadyGoal("other", "WAITING ON THE HUMAN: which one."))
		result := bed.tick(deadWorkers)
		if !strings.Contains(result.OpenWork, "claimable") {
			t.Fatalf("the reader answers claimable work: %q", result.OpenWork)
		}
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "held" {
			t.Fatalf("the held goal's successor starts whatever the ready set holds: %+v %+v", result.Decision, result.Seat)
		}
		record := bed.start(result.Seat)
		brief, err := os.ReadFile(bed.launcher.starts[0].Brief)
		if err != nil {
			t.Fatal(err)
		}
		if record.Goal != "held" || !strings.Contains(string(brief), "metasystem goal claim held") || strings.Contains(string(brief), "other") {
			t.Fatalf("the brief and the record name G, never H: %+v\n%s", record, brief)
		}
	})
}

// seatLadderTick judges recorded seats at the per-goal retry boundary.
// Automatic process restarts have a separate seat-wide allowance.
func seatLadderTick(t *testing.T, bed *seatBed) TickResult {
	t.Helper()
	state := reapSeatLaunches(bed.root, *bed.seatDependencies(), bed.now)
	if state.Err != nil || state.Unreaped != "" {
		t.Fatalf("seat records are not settled: %+v", state)
	}
	projection := bed.projection(bed.now)
	work, err := goal.ClaimableWorkFromProjection(projection, seatBedMachine, identity.KernelProber{})
	if err != nil {
		t.Fatal(err)
	}
	world := SeatWorldFrom(work, projection.Tree.Live, bed.settings, bed.tips, bed.now)
	decision, selection := PlanSeat(world, state.Records, 3, false)
	return TickResult{Decision: decision, Seat: selection}
}

// seatCycle runs one seat that claims the goal, writes a blocker and releases
// it: the ledger moves, the branch does not.
func seatCycle(t *testing.T, bed *seatBed, want string, blocker string) SeatRecord {
	t.Helper()
	result := seatLadderTick(t, bed)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != want {
		t.Fatalf("seat for %s: %+v %+v", want, result.Decision, result.Seat)
	}
	record := SeatRecord{Schema: 1, LaunchID: fmt.Sprintf("fixture-seat-%d", len(bed.launcher.starts)+1),
		Goal: want, ApprovalOpid: bed.goals[want].Approved.Opid, Machine: seatBedMachine,
		Tips: map[string]string{want: bed.tips[want]}, StartedAt: bed.now.Format(seatStartedAtLayout)}
	if err := writeSeatRecord(bed.root, record); err != nil {
		t.Fatal(err)
	}
	if err := bed.launcher.StartSeat(SeatLaunchSpec{ID: record.LaunchID}); err != nil {
		t.Fatal(err)
	}
	file := bed.goals[want]
	file.NextStep = blocker
	file.Revision++
	for _, verb := range []string{"claim", "release"} {
		file.History = append(file.History, goal.HistoryLine{At: bed.now.Format(time.RFC3339),
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC4", seatBedMachine, SeatLineage), Verb: verb,
			Actor: seatBedMachine + "+" + SeatLineage, Targets: []string{want}, Keep: -1})
	}
	bed.end(record.LaunchID, "completed", `{"type":"result","is_error":false,"result":"released"}`)
	bed.now = bed.now.Add(time.Minute)
	return record
}

// Test 17. The reset by goal unapprove then goal approve is proven over the
// real verbs in cmd/metasystem (TestSeatCapResetsOnlyOnUnapproveThenApprove).
func TestClaimBlockerReleaseCyclesStopAtTheCap(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("a-stuck", "Build it."), seatReadyGoal("b-next", "Build it."))
	for i := 0; i < 3; i++ {
		seatCycle(t, bed, "a-stuck", "Blocked: the fixture is missing.")
	}
	result := seatLadderTick(t, bed)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "b-next" {
		t.Fatalf("the fourth tick names another free goal: %+v %+v", result.Decision, result.Seat)
	}
	if count := SeatNoProgressCount(bed.records(), "a-stuck", bed.goals["a-stuck"].Approved.Opid); count != 3 {
		t.Fatalf("three no-progress seats count three: %d", count)
	}
	bed.drop("b-next")
	result = seatLadderTick(t, bed)
	if result.Decision.Action != ActNotify || result.Seat != nil ||
		!strings.Contains(result.Decision.Reason, "3 seats ended without progress on a-stuck") {
		t.Fatalf("with no other goal the cap starts none and notifies: %+v", result.Decision)
	}

	t.Run("a seat whose goal branch gained a commit resets the count", func(t *testing.T) {
		t.Parallel()
		bed := newSeatBed(t, seatReadyGoal("a-stuck", "Build it."))
		seatCycle(t, bed, "a-stuck", "Blocked.")
		seatCycle(t, bed, "a-stuck", "Blocked.")
		record := seatCycle(t, bed, "a-stuck", "Blocked.")
		_ = record
		// The third seat's record is reaped at the next tick; before that,
		// its branch gains a commit.
		bed.tips["a-stuck"] = "1111111111111111111111111111111111111111"
		result := seatLadderTick(t, bed)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "a-stuck" {
			t.Fatalf("progress resets the count: %+v %+v", result.Decision, result.Seat)
		}
		if count := SeatNoProgressCount(bed.records(), "a-stuck", bed.goals["a-stuck"].Approved.Opid); count != 0 {
			t.Fatalf("the newest seat made progress: %d", count)
		}
	})
}

func TestAutomaticRevivalStopsAfterClaimReleaseWithoutWorkProgress(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("a-stuck", "Build it."))
	for range 2 {
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "a-stuck" {
			t.Fatalf("seat was not admitted: %+v", result)
		}
		record := bed.start(result.Seat)
		file := bed.goals["a-stuck"]
		file.NextStep = "Blocked: the fixture is missing."
		file.Revision++
		for _, verb := range []string{"claim", "release"} {
			file.History = append(file.History, goal.HistoryLine{At: bed.now.Format(time.RFC3339),
				Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC4", seatBedMachine, SeatLineage), Verb: verb,
				Actor: seatBedMachine + "+" + SeatLineage, Targets: []string{file.Id}, Keep: -1})
		}
		bed.end(record.LaunchID, "completed", `{"type":"result","is_error":false,"result":"released"}`)
		bed.now = bed.now.Add(time.Minute)
	}
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActNotify || result.Seat != nil ||
		!strings.Contains(result.Decision.Reason, "the automatic revival produced no retained work progress") || len(bed.launcher.starts) != 2 {
		t.Fatalf("claim and release must not authorize another automatic revival: %+v starts=%d", result, len(bed.launcher.starts))
	}
	evidence, err := LoadEvidence(EvidencePath(bed.root))
	if err != nil || evidence.AbnormalCount != 1 || evidence.Abnormal[0].Pending {
		t.Fatalf("the stopped tick changed the retained restart allowance: %+v %v", evidence, err)
	}
}

// redirectedSeat starts another goal after a person parks the old launch's goal.
func redirectedSeat(t *testing.T, gap time.Duration) (*seatBed, SeatRecord) {
	t.Helper()
	bed := newSeatBed(t, seatReadyGoal("g1", "Build it."))
	bed.now = time.Date(2026, time.October, 20, 10, 0, 0, 0, time.UTC)
	start := func(want string) SeatRecord {
		t.Helper()
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != want {
			t.Fatalf("seat for %s was not admitted: %+v", want, result)
		}
		record, err := bed.startUnder(*result.Seat, fakeCensus{workers: deadWorkers})
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	first := start("g1")
	bed.end(first.LaunchID, "failed", `{"type":"result","is_error":true,"result":"blocked"}`)
	bed.now = bed.now.Add(time.Minute)
	second := start("g1")
	file := bed.goals["g1"]
	file.State, file.Claimed = goal.StateParked, nil
	file.Revision++
	file.Parked = &goal.ParkRecord{By: "human:Wido", At: bed.now.Format(time.RFC3339), Because: "Work on the other goal."}
	file.History = append(file.History, goal.HistoryLine{At: bed.now.Format(time.RFC3339),
		Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC4", seatBedMachine, "human"), Verb: "park",
		Actor: "human:Wido", Targets: []string{file.Id}, Keep: -1})
	bed.end(second.LaunchID, "cancelled", `{"type":"result","is_error":false,"result":"stopped"}`)
	bed.put(seatReadyGoal("g2", "Build the other goal."))
	bed.now = bed.now.Add(gap)
	third := start("g2")
	starts := bed.launcher.starts
	if len(starts) != 3 || starts[2].ID != third.LaunchID || third.Goal != "g2" {
		t.Fatalf("the launcher did not start the selected goal: %+v %+v", third, starts)
	}
	ev, err := LoadEvidence(EvidencePath(bed.root))
	if err != nil || ev.AbnormalCount != 1 || ev.CurrentSeat != third.LaunchID {
		t.Fatalf("redirection spent restart history or lost the current launch: %+v %v", ev, err)
	}
	return bed, third
}

func TestAnOldLaunchHoldsOnlyItsOwnGoal(t *testing.T) {
	t.Parallel()
	redirectedSeat(t, 7*24*time.Hour)
}

func TestAnOldRestartEntryDoesNotSpeakForALaterLaunch(t *testing.T) {
	t.Parallel()
	bed, previous := redirectedSeat(t, time.Minute)
	bed.end(previous.LaunchID, "completed", `{"type":"result","is_error":false,"result":"blocked"}`)
	bed.now = bed.now.Add(time.Minute)
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "g2" {
		t.Fatalf("an older launch's restart entry held this launch: %+v", result)
	}
	record, err := bed.startUnder(*result.Seat, fakeCensus{workers: deadWorkers})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := LoadEvidence(EvidencePath(bed.root))
	if err != nil || ev.AbnormalCount != 2 || ev.Abnormal[1].Pending || ev.Abnormal[1].Class != SeatNoProgress ||
		!ev.Abnormal[1].At.Equal(bed.now) || ev.CurrentSeat != record.LaunchID || len(bed.launcher.starts) != 4 {
		t.Fatalf("the later launch did not reserve and finish its own restart: %+v %v", ev, err)
	}
}

func TestAHoldIsBoundToItsApproval(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("a-stuck", "Build it."))
	bed.now = time.Date(2026, time.October, 20, 10, 0, 0, 0, time.UTC)
	for range 2 {
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil {
			t.Fatalf("seat was not admitted: %+v", result)
		}
		record := bed.start(result.Seat)
		file := bed.goals["a-stuck"]
		file.NextStep = "Blocked: the fixture is missing."
		file.Revision++
		for _, verb := range []string{"claim", "release"} {
			file.History = append(file.History, goal.HistoryLine{At: bed.now.Format(time.RFC3339),
				Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC4", seatBedMachine, SeatLineage), Verb: verb,
				Actor: seatBedMachine + "+" + SeatLineage, Targets: []string{file.Id}, Keep: -1})
		}
		bed.end(record.LaunchID, "completed", `{"type":"result","is_error":false,"result":"released"}`)
		bed.now = bed.now.Add(time.Minute)
	}
	result := bed.tick(deadWorkers)
	if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, "no retained work progress") {
		t.Fatalf("the same approval must hold: %+v", result)
	}
	file := bed.goals["a-stuck"]
	oldApproval := file.Approved.Opid
	file.Approved.Opid = goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC5", seatBedMachine, "human")
	for i := range file.History {
		if file.History[i].Opid == oldApproval {
			file.History[i].Opid = file.Approved.Opid
		}
	}
	result = bed.tick(deadWorkers)
	if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "a-stuck" {
		t.Fatalf("the old approval held a newly approved start: %+v", result)
	}
}

// Test 19.
func TestStartNoticeNamesTheStops(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	result := bed.tick(deadWorkers)
	record := bed.start(result.Seat)
	if len(bed.launcher.starts) != 1 || bed.launcher.starts[0].ID != record.LaunchID || bed.launcher.starts[0].StateRoot != bed.root {
		t.Fatalf("one seat launch in the checkout: %+v", bed.launcher.starts)
	}
	want := "steward: started seat " + record.LaunchID + " on " + seatBedMachine + " for alpha; `metasystem work stop " + record.LaunchID +
		"` ends it, `metasystem helm take` keeps the steward from starting another"
	found := false
	for _, message := range pendingMessages(t, bed.root) {
		found = found || message == want
	}
	if !found {
		t.Fatalf("the start notice names the stops:\nwant %q\ngot  %q", want, pendingMessages(t, bed.root))
	}
	data, err := os.ReadFile(filepath.Join(bed.root, "artifacts", "agents", "steward", "seats", record.LaunchID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var kept SeatRecord
	if err := json.Unmarshal(data, &kept); err != nil || kept.Goal != "alpha" || kept.ApprovalOpid != bed.goals["alpha"].Approved.Opid || kept.StartedAt == "" {
		t.Fatalf("the seat record names the goal, its approval and the start: %+v %v", kept, err)
	}
	if _, ok := kept.Tips["alpha"]; !ok {
		t.Fatalf("the seat record keeps each ready goal's branch tip, absent as absent: %+v", kept.Tips)
	}
}

// Test 21, with the leg of Astra's confirmation read: cleared, then held.
func TestALandingClaimWaitingOnReviewStartsNoSuccessor(t *testing.T) {
	t.Parallel()
	waits := func(t *testing.T, file *goal.GoalFile, settings ...goal.GateSettings) {
		t.Helper()
		bed := newSeatBed(t, file)
		if len(settings) > 0 {
			bed.settings = settings[0]
		}
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, file.Id) ||
			!strings.Contains(result.Decision.Reason, "waits") {
			t.Fatalf("a landing waiting on the gate starts no successor and names the wait: %+v %+v", result.Decision, result.Seat)
		}
		if len(bed.records()) != 0 || len(bed.launcher.starts) != 0 {
			t.Fatal("a waiting landing starts nothing and counts nothing")
		}
	}
	starts := func(t *testing.T, file *goal.GoalFile, tip string) {
		t.Helper()
		bed := newSeatBed(t, file)
		if tip != "" {
			bed.tips[file.Id] = tip
		}
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != file.Id {
			t.Fatalf("a due landing starts the successor naming it: %+v %+v", result.Decision, result.Seat)
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	t.Run("at the tier with no word", func(t *testing.T) {
		waits(t, seatLandingGoal("held", 3, now.Add(-time.Hour)))
	})
	t.Run("under a hold", func(t *testing.T) {
		file := seatLandingGoal("held", 1, now.Add(-6*time.Hour))
		seatHeldBySitting(file, now.Add(-5*time.Hour))
		waits(t, file)
	})
	t.Run("cleared, then held", func(t *testing.T) {
		file := seatLandingGoal("held", 3, now.Add(-2*time.Hour))
		seatCleared(file, now.Add(-90*time.Minute))
		seatHumanLine(file, now.Add(-time.Hour), goal.SittingReason(true, "plans/reviews/second-look.md", "Astra"))
		waits(t, file)
	})
	t.Run("below the tier before auto-after", func(t *testing.T) {
		waits(t, seatLandingGoal("held", 1, now.Add(-time.Hour)))
	})
	t.Run("below the tier and eligible", func(t *testing.T) {
		starts(t, seatLandingGoal("held", 1, now.Add(-5*time.Hour)), "")
	})
	t.Run("cleared at the tip", func(t *testing.T) {
		file := seatLandingGoal("held", 3, now.Add(-time.Hour))
		seatCleared(file, now.Add(-30*time.Minute))
		starts(t, file, seatReviewTip)
	})
	// SOL-A-02: the word binds to the tip it was given at, as the gate
	// itself requires; a branch moved since waits for the word again.
	t.Run("cleared at a tip the branch has moved from", func(t *testing.T) {
		file := seatLandingGoal("held", 3, now.Add(-time.Hour))
		seatCleared(file, now.Add(-30*time.Minute))
		bed := newSeatBed(t, file)
		bed.tips["held"] = "0badc0de0badc0de0badc0de0badc0de0badc0de"
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNotify || result.Seat != nil || !strings.Contains(result.Decision.Reason, "a moved tip needs the word again") {
			t.Fatalf("a word given at an older tip starts no successor: %+v %+v", result.Decision, result.Seat)
		}
		if len(bed.records()) != 0 || len(bed.launcher.starts) != 0 {
			t.Fatal("a stale word starts nothing and counts nothing")
		}
	})
	t.Run("a waiting landing beside ready work names the ready goal", func(t *testing.T) {
		bed := newSeatBed(t, seatLandingGoal("held", 3, now.Add(-time.Hour)), seatReadyGoal("alpha", "Build it."))
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "alpha" {
			t.Fatalf("the waiting landing is neither selected nor blocking: %+v %+v", result.Decision, result.Seat)
		}
	})
}

// The runner's pass starts the seat the tick selected, instead of the
// delegate revival.
func TestRunnerStartsTheSelectedSeatInsteadOfARevival(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	var started []SeatSelection
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	revives := 0
	deps := runnerLoopDependencies{
		Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
			return TickResult{Decision: Decision{VerdictIdleBacklogDead, ActRevive, "ready work has no seat"}, Seat: &SeatSelection{Goal: "alpha"}}, nil
		},
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            func() time.Time { return now },
		Sleep: func(d time.Duration) {
			now = now.Add(d)
			if err := stopRunnerLoop(loop.root); err != nil {
				t.Fatal(err)
			}
		},
		StartSeat: func(_ string, _ TickConfig, _ WorkerCensus, selection SeatSelection) (SeatRecord, error) {
			started = append(started, selection)
			return SeatRecord{}, nil
		},
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { revives++; return nil }, 200*time.Millisecond, TickConfig{Now: now, Seat: &fakeSeatLauncher{}}, deps); err != nil {
		t.Fatal(err)
	}
	if len(started) != 1 || started[0].Goal != "alpha" || revives != 0 {
		t.Fatalf("the runner starts the selected seat once and no delegate: %+v revives=%d", started, revives)
	}
}

// SOL-A-01: a held goal of the seat lineage whose next step waits on a human
// word is the Stop path's wait (goal.idleBacklogContinuation): no successor
// starts for it, nor for other ready work beside the working claim, and
// nothing counts.
func TestAHeldGoalWaitingOnAHumanWordStartsNoSuccessor(t *testing.T) {
	t.Parallel()
	waiting := func() *goal.GoalFile {
		file := seatClaimedGoal("held", SeatLineage)
		file.NextStep = "WAITING ON THE HUMAN: which store does it read?"
		return file
	}
	for _, leg := range []struct {
		name  string
		goals []*goal.GoalFile
	}{
		{"alone", []*goal.GoalFile{waiting()}},
		{"beside ready work", []*goal.GoalFile{waiting(), seatReadyGoal("alpha", "Build it.")}},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, leg.goals...)
			result := bed.tick(deadWorkers)
			if result.Decision.Action != ActNotify || result.Seat != nil ||
				!strings.Contains(result.Decision.Reason, "held") || !strings.Contains(result.Decision.Reason, "human word") {
				t.Fatalf("a held goal waiting on a human word starts no successor: %+v %+v", result.Decision, result.Seat)
			}
			if len(bed.records()) != 0 || len(bed.launcher.starts) != 0 {
				t.Fatal("a held goal waiting on a human word starts nothing and counts nothing")
			}
		})
	}
}

// SOL-A-03: the start re-reads, under its lock, the census, the outage mark
// and the ladder over the fresh ledger; a selection they no longer hold up
// starts nothing and leaves no record.
func TestASeatStartRereadsItsGroundsUnderTheLock(t *testing.T) {
	t.Parallel()
	selected := func(t *testing.T) (*seatBed, SeatSelection) {
		t.Helper()
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActRevive || result.Seat == nil || result.Seat.Goal != "alpha" {
			t.Fatalf("the tick selects alpha: %+v %+v", result.Decision, result.Seat)
		}
		return bed, *result.Seat
	}
	refused := func(t *testing.T, bed *seatBed, selection SeatSelection, census WorkerCensus) {
		t.Helper()
		if record, err := bed.startUnder(selection, census); err == nil || record.LaunchID != "" {
			t.Fatalf("a start whose grounds changed went ahead: %+v %v", record, err)
		}
		if len(bed.launcher.starts) != 0 || len(bed.records()) != 0 {
			t.Fatalf("a withdrawn start launched or recorded: %+v %+v", bed.launcher.starts, bed.records())
		}
	}
	t.Run("a live main appeared", func(t *testing.T) {
		t.Parallel()
		bed, selection := selected(t)
		refused(t, bed, selection, fakeCensus{workers: Workers{Live: 1, LiveSeatMains: 1, CensusComplete: true}})
	})
	t.Run("the census became unprovable", func(t *testing.T) {
		t.Parallel()
		bed, selection := selected(t)
		refused(t, bed, selection, fakeCensus{workers: Workers{Unprovable: 1, CensusComplete: true}})
	})
	t.Run("an outage began", func(t *testing.T) {
		t.Parallel()
		bed, selection := selected(t)
		if _, err := testprovider.Record(bed.root, "overloaded", "API Error: 529", "test", bed.now); err != nil {
			t.Fatal(err)
		}
		refused(t, bed, selection, fakeCensus{workers: deadWorkers})
	})
	t.Run("the goal now waits on a human word", func(t *testing.T) {
		t.Parallel()
		bed, selection := selected(t)
		bed.goals["alpha"].NextStep = "WAITING ON THE HUMAN: which one."
		refused(t, bed, selection, fakeCensus{workers: deadWorkers})
	})
	t.Run("nothing changed", func(t *testing.T) {
		t.Parallel()
		bed, selection := selected(t)
		if record, err := bed.startUnder(selection, fakeCensus{workers: deadWorkers}); err != nil || record.Goal != "alpha" || len(bed.launcher.starts) != 1 {
			t.Fatalf("an unchanged selection starts its seat: %+v %v", record, err)
		}
	})
}

// Amendment 1 (Wido, 2026-09-30): a seat is opt-in per seat, and the landing
// lane never starts one. While the launcher answers that no seat may start,
// the ladder is today's: claimable work notifies, owned work keeps its path,
// and no seat is selected, recorded or counted.
func TestASeatThatIsOffStartsNothingAndTheLadderIsTodays(t *testing.T) {
	t.Parallel()
	t.Run("claimable work notifies", func(t *testing.T) {
		t.Parallel()
		bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
		bed.launcher.off = "launch.seat.runtime is off"
		result := bed.tick(deadWorkers)
		if result.Decision.Action != ActNotify || result.Decision.Verdict != VerdictIdleBacklogDead || result.Seat != nil {
			t.Fatalf("an installation whose seat is off keeps today's notification: %+v %+v", result.Decision, result.Seat)
		}
		if len(bed.records()) != 0 || len(bed.launcher.starts) != 0 {
			t.Fatal("a seat that is off is never recorded or started")
		}
	})
	t.Run("owned work keeps its path", func(t *testing.T) {
		t.Parallel()
		bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
		bed.launcher.off = "this checkout is the host's landing lane"
		result := bed.tick(deadWorkers)
		if result.Seat != nil || strings.Contains(result.Decision.Reason, "seat") {
			t.Fatalf("owned work under a seat that is off keeps today's path: %+v %+v", result.Decision, result.Seat)
		}
		if len(bed.records()) != 0 || len(bed.launcher.starts) != 0 {
			t.Fatal("a seat that is off is never recorded or started")
		}
	})
}
