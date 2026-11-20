package steward

import (
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// Exercise the decision and locked start used by scheduled and light runners.
func TestCleanEndsReachTheGoalCapWithoutSpendingDeathHistory(t *testing.T) {
	t.Parallel()
	for _, gap := range []time.Duration{time.Minute, 24 * time.Hour} {
		t.Run(gap.String(), func(t *testing.T) {
			b := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
			b.now = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
			units := []UnitStage{{Unit: "U1", Run: "run-1", Stage: "built, ready for review", Continuable: true, Advances: []string{"admitted:run-1", "built:run-1"}}}
			d := *b.seatDependencies()
			d.Units = func(string, string) ([]UnitStage, error) { return units, nil }
			work := b.dependencies()
			work.Seat = &d
			cfg := TickConfig{ProviderHome: testprovider.Home(b.root)}
			tick := func() TickResult {
				t.Helper()
				cfg.Now = b.now
				ev, err := LoadEvidence(EvidencePath(b.root))
				if err != nil {
					t.Fatal(err)
				}
				result, err := decideTickWithDependencies(b.root, cfg, fakeCensus{workers: deadWorkers}, ev, Marks{}, work)
				if err != nil {
					t.Fatal(err)
				}
				if err = SaveEvidence(b.root, EvidencePath(b.root), result.Evidence); err != nil {
					t.Fatal(err)
				}
				return result
			}
			for n := 0; n < 3; n++ {
				result := tick()
				if result.Seat == nil {
					t.Fatalf("end %d: %+v", n, result.Decision)
				}
				d.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
					return seatRecheck(b.root, cfg, fakeCensus{workers: deadWorkers}, work, records)
				}
				record, err := startSeatWithDependencies(b.root, *result.Seat, d)
				if err != nil {
					t.Fatal(err)
				}
				b.end(record.LaunchID, "completed", "RESULT: DONE\n")
				b.now = b.now.Add(gap)
				result = tick()
				ev, err := LoadEvidence(EvidencePath(b.root))
				if err != nil {
					t.Fatal(err)
				}
				if ev.AbnormalCount != 0 {
					t.Fatalf("clean end spent death history: %+v", ev.Abnormal)
				}
				if n < 2 && (result.Seat == nil || !strings.Contains(result.Decision.Reason, "clean")) {
					t.Fatalf("clean end %d: %+v", n+1, result.Decision)
				}
				if n == 2 && (result.Seat != nil || !strings.Contains(result.Decision.Reason, "goal approve held") || !strings.Contains(result.Decision.Reason, "tried")) {
					t.Fatalf("third end must explain the cap: %+v", result.Decision)
				}
			}
		})
	}
}

func TestGoalProgressCannotBeBorrowedFromAnotherGoal(t *testing.T) {
	t.Parallel()
	b := newSeatBed(t, seatClaimedGoal("held", SeatLineage))
	for i := 0; i < 3; i++ {
		r := SeatRecord{Schema: 1, LaunchID: string(rune('a' + i)), Goal: "held", ApprovalOpid: b.goals["held"].Approved.Opid, Outcome: SeatNoProgress, StartedAt: b.now.Add(time.Duration(i) * time.Minute).Format(seatStartedAtLayout), ReapedAt: b.now.Format(time.RFC3339), LaunchState: "completed"}
		if err := writeSeatRecord(b.root, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeSeatRecord(b.root, SeatRecord{Schema: 1, LaunchID: "other", Goal: "other", Outcome: SeatProgress, StartedAt: b.now.Add(time.Hour).Format(seatStartedAtLayout), ReapedAt: b.now.Format(time.RFC3339), LaunchState: "completed"}); err != nil {
		t.Fatal(err)
	}
	result := b.tick(deadWorkers)
	if result.Seat != nil || !strings.Contains(result.Decision.Reason, "3 seats ended without progress") {
		t.Fatalf("another goal reset the cap: %+v", result.Decision)
	}
}

type cleanEndBed struct {
	*seatBed
	units   []UnitStage
	unitErr error
	d       seatDependencies
	work    openWorkDependencies
	cfg     TickConfig
}

func newCleanEndBed(t *testing.T) *cleanEndBed {
	t.Helper()
	b := &cleanEndBed{seatBed: newSeatBed(t, seatClaimedGoal("held", SeatLineage))}
	b.now = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	b.units = []UnitStage{{Unit: "U1", Run: "run-1", Continuable: true, Advances: []string{"admitted:run-1"}}}
	b.d = *b.seatDependencies()
	b.d.Units = func(string, string) ([]UnitStage, error) { return b.units, b.unitErr }
	b.work = b.dependencies()
	b.work.Seat = &b.d
	b.cfg = TickConfig{ProviderHome: testprovider.Home(b.root), Seat: b.launcher, workReads: &b.work}
	b.d.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
		return seatRecheck(b.root, b.cfg, fakeCensus{workers: deadWorkers}, b.work, records)
	}
	return b
}
func (b *cleanEndBed) decision() TickResult {
	b.t.Helper()
	b.cfg.Now = b.now
	ev, err := LoadEvidence(EvidencePath(b.root))
	if err != nil {
		b.t.Fatal(err)
	}
	r, err := decideTickWithDependencies(b.root, b.cfg, fakeCensus{workers: deadWorkers}, ev, Marks{}, b.work)
	if err != nil {
		b.t.Fatal(err)
	}
	if err = SaveEvidence(b.root, EvidencePath(b.root), r.Evidence); err != nil {
		b.t.Fatal(err)
	}
	return r
}
func (b *cleanEndBed) begin() SeatRecord {
	b.t.Helper()
	r := b.decision()
	if r.Seat == nil {
		b.t.Fatalf("no selection: %+v", r.Decision)
	}
	record, err := startSeatWithDependencies(b.root, *r.Seat, b.d)
	if err != nil {
		b.t.Fatal(err)
	}
	return record
}
func (b *cleanEndBed) finish(record SeatRecord) {
	b.t.Helper()
	b.now = b.now.Add(time.Minute)
	b.end(record.LaunchID, "completed", "RESULT: DONE\n")
}

func TestCleanEndProgressUsesOnlyThisGoalsWork(t *testing.T) {
	for _, kind := range []string{"new unit", "built", "checked", "read", "published", "answered", "push", "reply", "other goal", "handoff note"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newCleanEndBed(t)
			for n := 0; n < 4; n++ {
				record := b.begin()
				switch kind {
				case "push":
					b.tips["held"] = fmt.Sprint(n)
				case "reply":
					b.d.Threads = func() ([]board.Thread, error) {
						return []board.Thread{{Messages: []board.Message{{Kind: board.KindReply, From: board.Sender{Machine: seatBedMachine, Lineage: SeatLineage}, At: b.now}}}}, nil
					}
				case "other goal":
					b.put(seatReadyGoal("other", "Build it."))
					b.tips["other"] = fmt.Sprint(n)
				case "handoff note":
					writeTestFile(t, filepath.Join(b.root, "note.md"), []byte(fmt.Sprint(n)))
				case "answered":
					writeStewardRecord(t, filepath.Join(b.root, "artifacts", "agents", "channel", "questions", fmt.Sprintf("q-%d.json", n)), map[string]any{"id": fmt.Sprintf("q-%d", n), "goal": "held", "state": "closed", "openedAt": b.now, "answer": map[string]any{"phase": "closed", "at": b.now}})
				default:
					b.units[0].Advances = append(b.units[0].Advances, fmt.Sprintf("%s:%d", kind, n))
				}
				b.finish(record)
				r := b.decision()
				progress := kind != "push" && kind != "reply" && kind != "other goal" && kind != "handoff note"
				if !progress && n == 2 {
					if r.Seat != nil {
						t.Fatalf("non-work reset cap: %+v", r.Decision)
					}
					break
				}
				if r.Seat == nil {
					t.Fatalf("work progress was lost: %+v", r.Decision)
				}
				if got := SeatNoProgressCount(b.records(), "held", record.ApprovalOpid); progress && got != 0 || !progress && got != n+1 {
					t.Fatalf("count=%d progress=%v", got, progress)
				}
			}
		})
	}
}

func TestCleanEndKeepsAdmissionGuardsAndDeathHistory(t *testing.T) {
	for _, kind := range []string{"pending", "unreadable history", "unknown outcome", "old history", "abnormal predecessor", "question", "other question", "human word", "fence", "foreign claim", "released", "paused", "missing", "start failed", "unreadable units", "no units", "running unit", "cancelled unit", "budget"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newCleanEndBed(t)
			record := b.begin()
			b.finish(record)
			ev, err := LoadEvidence(EvidencePath(b.root))
			if err != nil {
				t.Fatal(err)
			}
			wantStart := false
			switch kind {
			case "pending", "old history", "abnormal predecessor":
				ev.AbnormalCount = 1
				ev.Abnormal[0] = AbnormalRestart{At: b.now.Add(-time.Second), Class: "failed", Pending: kind == "pending"}
				if kind == "old history" {
					ev.AbnormalCount = 2
					ev.Abnormal[0].Class = SeatNoProgress
					ev.Abnormal[1] = AbnormalRestart{At: b.now.Add(-time.Second), Class: SeatNoProgress}
				}
				wantStart = kind != "pending"
			case "question":
				b.questions = []goal.OpenQuestion{{ID: "q", Goal: "held"}}
			case "other question":
				b.questions = []goal.OpenQuestion{{ID: "q", Goal: "other"}, {ID: "machine"}}
				wantStart = true
			case "human word":
				b.goals["held"].NextStep = "WAITING ON THE HUMAN: approve the design."
			case "fence":
				b.fence = "the person stopped this seat"
			case "foreign claim":
				b.goals["held"].Claimed.Lineage = "person"
			case "released":
				b.goals["held"].State = goal.StateParked
				b.goals["held"].Claimed = nil
				b.goals["held"].StopCapability = nil
				b.goals["held"].Parked = &goal.ParkRecord{By: "human:Wido", At: b.now.Format(time.RFC3339), Because: "paused"}
			case "paused":
				b.goals["held"].State = goal.StateParked
				b.goals["held"].Claimed = nil
				b.goals["held"].StopCapability = nil
				b.goals["held"].Parked = &goal.ParkRecord{By: "human:Wido", At: b.now.Format(time.RFC3339), Because: "paused"}
			case "unreadable history":
				ev.AbnormalCount = 1
				ev.Abnormal[0] = AbnormalRestart{Class: "failed"}
			case "unknown outcome":
				state := b.launcher.states[record.LaunchID]
				state.State = "unrecognized"
				b.launcher.states[record.LaunchID] = state
			case "missing":
				delete(b.launcher.states, record.LaunchID)
			case "start failed":
				record.Outcome = SeatStartFailed
				record.ReapedAt = b.now.Format(time.RFC3339)
				if err := writeSeatRecord(b.root, record); err != nil {
					t.Fatal(err)
				}
			case "unreadable units":
				b.unitErr = errors.New("unreadable work")
			case "no units":
				b.units = nil
				wantStart = true
			case "running unit", "cancelled unit":
				b.units[0].Continuable = false
				wantStart = true
			case "budget":
				b.goals["held"].StopCapability.FenceEpoch = 1
				b.goals["held"].StopFence = &goal.StopFence{StopID: "stop-held-r3-f1", Revision: 3, Epoch: 1, CapabilityGeneration: 3, ClosedAt: b.now.Format(time.RFC3339), Reason: goal.StopReasonElapsedLimit}
				b.goals["held"].Budget.ElapsedLimit = "1m"
				b.goals["held"].Approved.Digest = goal.ApprovalDigest(b.goals["held"].Intent, b.goals["held"].Tier, *b.goals["held"].Budget)
			}
			if err := SaveEvidence(b.root, EvidencePath(b.root), ev); err != nil {
				t.Fatal(err)
			}
			r := b.decision()
			if (r.Seat != nil) != wantStart {
				t.Fatalf("start=%v: %+v", wantStart, r.Decision)
			}
			after, err := LoadEvidence(EvidencePath(b.root))
			if err != nil {
				t.Fatal(err)
			}
			if after.Abnormal != ev.Abnormal || after.AbnormalCount != ev.AbnormalCount {
				t.Fatal("decision rewrote death history")
			}
			if wantStart {
				clean := kind != "no units" && kind != "running unit" && kind != "cancelled unit"
				if strings.Contains(r.Decision.Reason, "clean;") != clean {
					t.Fatalf("classification: %+v", r.Decision)
				}
				if _, err := startSeatWithDependencies(b.root, *r.Seat, b.d); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCleanEndLightAndScheduledDecisionsStartExactlyOnce(t *testing.T) {
	for _, gate := range []string{"ready", "not reached", "restarted"} {
		t.Run(gate, func(t *testing.T) {
			t.Parallel()
			b := newCleanEndBed(t)
			record := b.begin()
			b.finish(record)
			b.cfg.Now = b.now
			if gate == "ready" {
				b.cfg.scheduledProgress = &scheduledProgress{collection: true, seat: true}
			}
			if gate == "not reached" {
				b.cfg.scheduledProgress = &scheduledProgress{collection: true}
			}
			light, err := runLightDecisionsWith(b.root, b.cfg, fakeCensus{workers: deadWorkers}, b.work, ReapContinuations)
			if err != nil {
				t.Fatal(err)
			}
			if gate != "ready" {
				if light.Seat != nil || light.Decision.Action != ActHold {
					t.Fatalf("gate lost: %+v", light.Decision)
				}
				light = b.decision()
			}
			if light.Seat == nil || !strings.Contains(light.Decision.Reason, "clean;") {
				t.Fatalf("light/scheduled clean decision: %+v", light.Decision)
			}
			scheduled := b.decision()
			first, err := startSeatWithDependencies(b.root, *light.Seat, b.d)
			if err != nil {
				t.Fatal(err)
			}
			second, err := startSeatWithDependencies(b.root, *scheduled.Seat, b.d)
			if err != nil {
				t.Fatal(err)
			}
			if first.LaunchID == "" || second.LaunchID != "" || len(b.launcher.starts) != 2 {
				t.Fatal("duplicate successor")
			}
			if count := SeatNoProgressCount(b.records(), "held", record.ApprovalOpid); count != 1 {
				t.Fatalf("double reap: %d", count)
			}
		})
	}
}

func TestCleanEndAdmissionIsReadAgainUnderTheStartLock(t *testing.T) {
	t.Parallel()
	b := newCleanEndBed(t)
	record := b.begin()
	b.finish(record)
	selected := b.decision().Seat
	b.questions = []goal.OpenQuestion{{ID: "new-question", Goal: "held"}}
	if _, err := startSeatWithDependencies(b.root, *selected, b.d); err == nil {
		t.Fatal("stale admission started despite a new question")
	}
	if len(b.launcher.starts) != 1 {
		t.Fatal("a second seat started")
	}
}

func TestCleanEndHandoffMustBelongToTheEndedProcess(t *testing.T) {
	for _, kind := range []string{"current", "old session", "other runtime", "after end", "corrupt capture"} {
		t.Run(kind, func(t *testing.T) {
			b := newCleanEndBed(t)
			installHandoffFixtureFiles(t, b.root)
			writeTestFile(t, filepath.Join(b.root, "metasystem.conf"), []byte("launch.seat.runtime=claude\nrole.steward-continuation.runtime=claude\n"))
			b.now = handoffCaptureNow.Add(-time.Minute)
			b.units = nil
			record := b.begin()
			fixture := &handoffGoalFixture{t: t, root: handoffWaitsTestRoot(t, b.root), state: "claimed", file: b.goals["held"]}
			caller := handoffMainCaller()
			result, err := fixture.handoff(b.root, caller, handoffTestRecord(t, b.root, nil), handoffCaptureNow, filepath.Join(b.root, "memory", "receipts.log"))
			if err != nil {
				t.Fatal(err)
			}
			b.finish(record)
			state := b.launcher.states[record.LaunchID]
			state.Child = &caller.Ref
			if kind == "after end" {
				state.FinishedAt = handoffCaptureNow.Add(-time.Second).Format(time.RFC3339Nano)
			}
			if kind == "old session" || kind == "other runtime" {
				state.Session, state.Runtime = "another-session", "fake"
				if kind == "other runtime" {
					state.Session, state.Runtime = caller.Session, "different-exec"
				}
				other := caller.Ref
				other.Pid++
				state.Child = &other
			}
			b.launcher.states[record.LaunchID] = state
			if kind == "corrupt capture" {
				writeTestFile(t, result.StatePath, []byte("{}"))
			}
			decision := b.decision()
			if kind == "corrupt capture" {
				if decision.Seat != nil {
					t.Fatal("corrupt capture admitted a successor")
				}
				return
			}
			clean := strings.Contains(decision.Decision.Reason, "clean;")
			if clean != (kind == "current") || decision.Seat == nil {
				t.Fatalf("handoff classification: %+v", decision.Decision)
			}
			if kind == "current" {
				if _, err := startSeatWithDependencies(b.root, *decision.Seat, b.d); err != nil {
					t.Fatal(err)
				}
				ev, err := LoadEvidence(EvidencePath(b.root))
				if err != nil || ev.AbnormalCount != 0 {
					t.Fatalf("handoff charged as death: %+v %v", ev, err)
				}
			}
		})
	}
}

func TestCleanEndProgressResetAndFreshApproval(t *testing.T) {
	t.Parallel()
	b := newCleanEndBed(t)
	for n := 0; n < 6; n++ {
		record := b.begin()
		if n == 1 {
			b.units[0].Advances = append(b.units[0].Advances, "checked:run-1")
		}
		b.finish(record)
		r := b.decision()
		if n == 4 {
			if r.Seat != nil {
				t.Fatal("three ends after progress did not cap")
			}
			// A fresh approval is the production ladder's cap epoch.
			file := b.goals["held"]
			old := file.Approved.Opid
			file.Approved.Opid = "01ARZ3NDEKTSV4RRFFQ69G5FCC-human-00000001"
			for i := range file.History {
				if file.History[i].Opid == old {
					file.History[i].Opid = file.Approved.Opid
				}
			}
		} else if r.Seat == nil {
			t.Fatalf("premature stop: %+v", r.Decision)
		}
	}
}

func TestCleanEndStatusUsesTheSameDecision(t *testing.T) {
	t.Parallel()
	b := newCleanEndBed(t)
	r := b.begin()
	b.finish(r)
	b.decision()
	ledger := newHealthLedger(b.root, b.now)
	ledger.readWorld = func(string) bool { return true }
	ledger.readEndpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{}, nil }
	ledger.project = func(goal.Endpoint, bool, time.Time) (goal.Projection, error) { return b.projection(b.now), nil }
	role := checkSessionMainWithLedger(b.root, b.root, b.now, healthProbe{}, ledger, b.d)
	if !strings.Contains(role.Reason, "clean; successor starts at the next decision") {
		t.Fatalf("status lost clean classification: %+v", role)
	}
	b.questions = []goal.OpenQuestion{{ID: "wait", Goal: "held"}}
	role = checkSessionMainWithLedger(b.root, b.root, b.now, healthProbe{}, ledger, b.d)
	if !strings.Contains(role.Reason, "open question") || strings.Contains(role.Reason, "clean;") {
		t.Fatalf("status promised a held successor: %+v", role)
	}
}

func TestCleanEndPreservesEarlierFailuresAndSeparateCheckoutCustody(t *testing.T) {
	t.Parallel()
	a, b := newCleanEndBed(t), newCleanEndBed(t)
	ar, br := a.begin(), b.begin()
	a.finish(ar)
	b.now = b.now.Add(time.Minute)
	b.end(br.LaunchID, "failed", "RESULT: FAILED\n")
	// The existing abnormal policy permits one retry, then refuses recurrence.
	retry := b.decision()
	if retry.Seat == nil {
		t.Fatalf("existing first retry changed: %+v", retry.Decision)
	}
	next, err := startSeatWithDependencies(b.root, *retry.Seat, b.d)
	if err != nil {
		t.Fatal(err)
	}
	b.finish(next)
	clean := b.decision()
	if clean.Seat == nil {
		t.Fatalf("an abnormal predecessor blocked a clean end: %+v", clean.Decision)
	}
	next, err = startSeatWithDependencies(b.root, *clean.Seat, b.d)
	if err != nil {
		t.Fatal(err)
	}
	b.now = b.now.Add(time.Minute)
	b.end(next.LaunchID, "failed", "RESULT: FAILED\n")
	// An actual work advance keeps this assertion about the death guard rather
	// than the earlier no-progress cap.
	b.units[0].Advances = append(b.units[0].Advances, "checked:run-1")
	stop := b.decision()
	if stop.Seat != nil || !strings.Contains(stop.Decision.Reason, "same death class") {
		t.Fatalf("clean end erased a failure: %+v", stop.Decision)
	}
	ready := a.decision()
	if ready.Seat == nil {
		t.Fatalf("other checkout stopped clean successor: %+v", ready.Decision)
	}
	if _, err := startSeatWithDependencies(a.root, *ready.Seat, a.d); err != nil {
		t.Fatal(err)
	}
	ev, err := LoadEvidence(EvidencePath(a.root))
	if err != nil || ev.AbnormalCount != 0 {
		t.Fatalf("cross-checkout history: %+v %v", ev, err)
	}
}

func TestCleanEndFenceAndProviderHoldRemainReversible(t *testing.T) {
	for _, kind := range []string{"fence", "provider"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newCleanEndBed(t)
			r := b.begin()
			b.finish(r)
			b.decision() // Reap the earlier successful launch before the later outage.
			b.now = b.now.Add(time.Second)
			if kind == "fence" {
				b.fence = "person stopped this seat"
			} else {
				if _, err := testprovider.Record(b.root, "overloaded", "API Error: 529", "fixture", b.now); err != nil {
					t.Fatal(err)
				}
			}
			if held := b.decision(); held.Seat != nil {
				t.Fatalf("hold ignored: %+v", held.Decision)
			}
			b.fence = ""
			if kind == "provider" {
				if err := outage.Clear(testprovider.Home(b.root), "anthropic", b.now); err != nil {
					t.Fatal(err)
				}
			}
			ready := b.decision()
			if ready.Seat == nil || !strings.Contains(ready.Decision.Reason, "clean;") {
				t.Fatalf("clean state lost across hold: %+v", ready.Decision)
			}
		})
	}
}

func TestUpgradeRetainsTheOldReapersProgressJudgement(t *testing.T) {
	t.Parallel()
	b := newCleanEndBed(t)
	approval := b.goals["held"].Approved.Opid
	b.launcher.states = make(map[string]SeatLaunchState)
	for i, outcome := range []string{SeatNoProgress, SeatNoProgress, SeatProgress, SeatNoProgress} {
		at := b.now.Add(time.Duration(i-5) * time.Minute).Format(seatStartedAtLayout)
		id := fmt.Sprintf("legacy-%d", i)
		// These are the previous engine's bytes, without a goal-work snapshot or verdict.
		data := fmt.Sprintf(`{"schema":1,"launchId":%q,"goal":"held","approvalOpid":%q,"startedAt":%q,"reapedAt":%q,"launchState":"completed","outcome":%q}`, id, approval, at, at, outcome)
		writeTestFile(t, seatRecordPath(b.root, id), []byte(data))
		b.end(id, "completed", "RESULT: DONE")
	}
	record := b.begin()
	b.finish(record)
	if r := b.decision(); r.Seat == nil || SeatNoProgressCount(b.records(), "held", approval) != 2 {
		t.Fatalf("upgrade lost the legacy reset or strike: %+v", r.Decision)
	}
}

// An active seat can cross the upgrade before its first reap. Its recorded
// tip baseline remains usable under the old progress measure.
func TestUpgradeReapsAnOldStartUnderItsOriginalMeasure(t *testing.T) {
	t.Parallel()
	b := newCleanEndBed(t)
	r := b.begin()
	r.WorkAtStart = nil
	r.Tips = map[string]string{"held": "before"}
	if err := writeSeatRecord(b.root, r); err != nil {
		t.Fatal(err)
	}
	b.tips["held"] = "after"
	b.finish(r)
	got := b.decision()
	last := b.records()[0]
	if got.Seat == nil || last.Outcome != SeatProgress || last.GoalProgress != nil || SeatNoProgressCount(b.records(), r.Goal, r.ApprovalOpid) != 0 {
		t.Fatal("legacy start lost its progress judgement on reap")
	}
}

func TestUnreadableNewWorkBaselineStillCountsTheEnd(t *testing.T) {
	t.Parallel()
	b := newCleanEndBed(t)
	b.tips["held"] = "original-tip"
	b.unitErr = errors.New("unit store unavailable")
	record := b.begin()
	b.unitErr = nil
	b.tips["held"] = "new-tip"
	b.finish(record)
	b.decision()
	last := b.records()[0]
	if last.Outcome != SeatProgress || last.GoalProgress == nil || *last.GoalProgress || SeatNoProgressCount(b.records(), record.Goal, record.ApprovalOpid) != 1 {
		t.Fatalf("current-engine start with unreadable work borrowed the legacy measure: %+v", last)
	}
}
