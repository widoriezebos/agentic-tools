package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

type SessionStopAnnouncementForTest sessionStopAnnouncement

func SessionStopLifecycleTokenForTest(announcement SessionStopAnnouncementForTest) (string, error) {
	return sessionStopLifecycleToken(sessionStopAnnouncement(announcement))
}

func openItem(detail string) Item { return Item{Kind: "plan", Id: detail, Detail: detail} }

const fixtureContextLine = "CONTEXT: 121K of trigger 100K (proof line 150K, maximum 200K, ceiling 240K)"

// The ladder keeps its leading rows (goal-cli-fixtures.sh:1082-1090 reads them
// by position) and the line sits directly above the full-verdict path.
func TestTurnVerdictPrintsTheContextLine(t *testing.T) {
	store := &Store{Root: servingBed(t, "bed-m1", nil), Now: func() time.Time { return time.Unix(1786800000, 0) }}
	verdict, err := store.TurnVerdict(
		ScanResult{Open: []Item{openItem("OPEN-WORK fixture: finish it")}}, "context-line", "", "",
		TurnVerdictOptions{ContextLine: fixtureContextLine})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(verdict.Display, "\n")
	if len(lines) < 4 || lines[0] != "OPEN WORK (1)" || lines[1] != "OPEN-WORK fixture: finish it" ||
		lines[len(lines)-2] != fixtureContextLine || !strings.HasPrefix(lines[len(lines)-1], "Full turn verdict") ||
		strings.Count(verdict.Display, "CONTEXT:") != 1 {
		t.Fatalf("rendered verdict context line = %q", verdict.Display)
	}
}

// supervision-hook-fixtures.sh:1096 compares the whole allowance display.
func TestTurnVerdictHandoffAllowanceStaysExact(t *testing.T) {
	store := testStore(t)
	verdict, err := store.TurnVerdict(ScanResult{}, "context-line", "", "", TurnVerdictOptions{
		ContextLine:     fixtureContextLine,
		HandoffRecorded: func(string) (string, bool, error) { return "recorded", true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Display != "handoff recorded: recorded; end this session" || verdict.ShouldBlock {
		t.Fatalf("handoff allowance display = %q shouldBlock=%v", verdict.Display, verdict.ShouldBlock)
	}
}

const (
	pendingWaitGoalID              = "claimed-wait-goal"
	pendingWaitMainID              = "main-wait"
	pendingWaitLineage             = "lineage-wait"
	pendingWaitSession             = "session-wait"
	pendingWaitAnnouncementSession = "session-placeholder"
	pendingWaitBootID              = "fixture-boot"
)

type pendingWaitVerdictFixture struct {
	root        string
	store       *Store
	row         metarun.Waiter
	scan        ScanResult
	bootElapsed time.Duration
}

func isolatePendingWaitFixtureGit(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		value, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

func writePendingWaitJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pendingWaitClaim(landing bool) *GoalFile {
	claim := &GoalFile{
		Id: pendingWaitGoalID, State: StateClaimed, Intent: "Wait for durable work", Origin: OriginMain,
		NextStep: "Continue after the registered wait.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Claimed: &ClaimRecord{
			Machine: "bed-m1", Lineage: pendingWaitLineage, At: "2026-08-23T01:00:00Z",
			Revision: 2, AccountingRevision: 2,
		},
	}
	if landing {
		claim.Landing = &LandingRecord{At: "2026-08-23T01:30:00Z", Opid: "01J5X0000000000000000000B1-mac-a-1a2b3c4d"}
	}
	return claim
}

func newPendingWaitVerdictFixture(t *testing.T, kind string, readyBacklog bool) *pendingWaitVerdictFixture {
	t.Helper()
	isolatePendingWaitFixtureGit(t)
	landing := kind == "landing"
	files := map[string]*GoalFile{pendingWaitGoalID: pendingWaitClaim(landing)}
	if readyBacklog {
		files["ready-after-wait"] = budgetedQueuedGoal("ready-after-wait", "2026-08-23T00:00:01Z")
	}
	root := servingBed(t, "bed-m1", files)
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	bootElapsed := 2 * time.Hour
	prober := idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0), StartTicks: 410, BootID: pendingWaitBootID},
		42: {Pid: 42, StartedAt: time.Unix(200, 0), StartTicks: 420, BootID: pendingWaitBootID},
	}
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: prober}
	writePendingWaitJSON(t, sessionStopLeasePath(root), sessionStopLease{
		HolderMainId: pendingWaitMainID, Pid: 41, PidStartedAt: 100,
		PidStartTicks: 410, BootID: pendingWaitBootID, ClaimEpoch: 7,
	})
	writePendingWaitJSON(t, filepath.Join(root, "artifacts", "agents", "mains", pendingWaitSession+"-41.json"), sessionStopAnnouncement{
		SessionId: pendingWaitAnnouncementSession, RuntimeSession: pendingWaitSession,
		MainId: pendingWaitMainID, Pid: 41, PidStartedAt: 100,
		PidStartTicks: 410, BootID: pendingWaitBootID, Runtime: "fake", InstanceTag: "fixture-wait-session",
		CommandHash: strings.Repeat("a", 64), AnnouncedAt: now.Add(-time.Minute).Format(time.RFC3339),
		Pgid: 41, OwnerLineage: pendingWaitLineage,
	})

	scan := ScanResult{Open: []Item{{Kind: "plan", Id: pendingWaitGoalID, Detail: "OPEN-WORK claimed-wait-goal: finish after the wait"}}}
	selector := metarun.WaitSelector{Kind: kind, TargetID: kind + "-target"}
	target := metarun.WaiterTarget{}
	switch kind {
	case "job":
		target = metarun.WaiterTarget{OperationID: "operation-wait", Round: 1, StartedAt: "2026-08-23T01:40:00Z"}
		writePendingWaitJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", selector.TargetID+".json"), map[string]any{
			"jobId": selector.TargetID, "operationId": target.OperationID, "round": target.Round,
			"startedAt": target.StartedAt, "status": "running", "goalId": pendingWaitGoalID,
		})
	case "run", "attempt":
		epoch := int64(7)
		runID := kind + "-run"
		runStore := &metarun.Store{Root: root, Now: store.Now}
		nonce, err := runStore.Launch(metarun.Caller{
			Class: "MAIN", MainId: pendingWaitMainID, OwnerLineage: pendingWaitLineage,
			ClaimEpoch: &epoch, SessionId: pendingWaitSession,
		}, metarun.LaunchParams{
			Id: runID, Kind: "custom", Display: "pending wait fixture",
			Log: filepath.Join(root, "artifacts", "agents", "runs", runID+".log"), GoalId: pendingWaitGoalID,
		})
		if err != nil {
			t.Fatal(err)
		}
		runTarget := metarun.WaiterTarget{Generation: 1, LaunchNonce: nonce}
		if kind == "run" {
			selector.TargetID = runID
			target = runTarget
		} else {
			selector.TargetID = "attempt-target"
			proofIdentity := proofrun.BuildProofIdentityForContext(proofrun.ExecutionContext{
				ManifestDigest: strings.Repeat("1", 64), Configuration: strings.Repeat("2", 64),
				Platform: "fixture/fixture", Toolchain: strings.Repeat("3", 64),
			}, "full", "fixture", []string{"gate"}, 1)
			deadline := now.Add(10 * time.Minute)
			attempt, result, reserveErr := proofrun.ReserveLocked(proofrun.AdmissionRequest{
				ControlRoot: root, ExecutionRoot: root, GoalID: pendingWaitGoalID,
				GoalRevision: 2, AccountingRevision: 2, CandidateGoalID: pendingWaitGoalID, CandidateRevision: 2, ReservedMinutes: 10,
				Identity: proofIdentity, Launcher: proofrun.ProcessIdentity{Pid: 90, PidStartedAt: 900},
				Now: now, AttemptID: selector.TargetID,
				ReservationOwner: &proofrun.ReservationOwner{
					ControlRoot: root, RunID: runID, RunGeneration: 1, LaunchNonce: nonce,
					GoalRevision: 2, ObligationRevision: 1, AttemptOrdinal: 1,
					Deadline: deadline.Format(time.RFC3339Nano),
				},
			})
			if reserveErr != nil || result.Disposition != proofrun.DispositionExecuted {
				t.Fatalf("reserve pending attempt: result=%+v err=%v", result, reserveErr)
			}
			target = metarun.WaiterTarget{ProofDigest: attempt.ProofIdentity.IdentityDigest}
		}
	case "landing", "human-act", "channel":
		tip, err := gitIn(root, "rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		tip = strings.TrimSpace(tip)
		selector.Kind, selector.TargetID, selector.GoalID, selector.After = "goal", pendingWaitGoalID, pendingWaitGoalID, tip
		selector.Event = kind
		if kind == "channel" {
			selector.Event, selector.Verb, selector.Question, selector.Poll = "human-act", "answer", "question-wait", "channel"
		}
		target = metarun.WaiterTarget{
			StartedAt:   "ledger:" + tip,
			ProofDigest: ledgerEndpointIdentity(Endpoint{Root: root, Remote: "local", Branch: "refs/heads/main"}),
		}
	default:
		t.Fatalf("unknown pending-wait fixture kind %q", kind)
	}
	epoch := int64(7)
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: strings.Repeat("b", 32), Nonce: strings.Repeat("c", 32),
		Kind: selector.Kind, TargetID: selector.TargetID, OwnerDigest: metarun.OwnerDigest(pendingWaitMainID),
		Pid: 42, PidStartedAt: 200, PidStartTicks: 420, BootID: pendingWaitBootID,
		Session: pendingWaitSession, MainId: pendingWaitMainID, OwnerLineage: pendingWaitLineage,
		ClaimEpoch: &epoch, RuntimeSession: pendingWaitSession, Selector: selector, GoalID: selector.GoalID, Target: target,
		RegisteredAt:      now.Add(-5 * time.Second).Format(time.RFC3339Nano),
		Deadline:          now.Add(time.Hour).Format(time.RFC3339Nano),
		BootDeadlineNanos: (bootElapsed + time.Hour).Nanoseconds(), DeadlineBootID: pendingWaitBootID,
		RemainingNanos: time.Hour.Nanoseconds(), LastObservedAt: now.Add(-5 * time.Second).Format(time.RFC3339Nano),
		LastObservedBootNanos: (bootElapsed - 5*time.Second).Nanoseconds(), OpenWorkSignature: scan.OpenWorkSignature(),
		State: "pending", Delivery: "blocking",
	}
	previousBootClock := turnVerdictBootClock
	turnVerdictBootClock = func() (string, time.Duration, error) { return pendingWaitBootID, bootElapsed, nil }
	t.Cleanup(func() { turnVerdictBootClock = previousBootClock })
	fixture := &pendingWaitVerdictFixture{root: root, store: store, row: row, scan: scan, bootElapsed: bootElapsed}
	fixture.writeRow(t)
	return fixture
}

func (fixture *pendingWaitVerdictFixture) writeRow(t *testing.T) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(metarun.WaitersDir(fixture.root), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	writePendingWaitJSON(t, metarun.WaiterPath(fixture.root, fixture.row.Kind, fixture.row.TargetID, fixture.row.OwnerDigest), fixture.row)
}

func (fixture *pendingWaitVerdictFixture) verdict(t *testing.T, scan ScanResult) Verdict {
	t.Helper()
	verdict, err := fixture.store.TurnVerdict(scan, pendingWaitSession, "", pendingWaitMainID)
	if err != nil {
		t.Fatal(err)
	}
	return verdict
}

func TestSessionAbsentReadsNoWait(t *testing.T) {
	primeOpenSignature := func(fixture *pendingWaitVerdictFixture) {
		t.Helper()
		state, err := fixture.store.loadVerdictState()
		if err != nil {
			t.Fatal(err)
		}
		state.touch(pendingWaitSession, fixture.store.nowISO()).OpenWorkSignature = fixture.scan.OpenWorkSignature()
		if err := fixture.store.saveVerdictState(state); err != nil {
			t.Fatal(err)
		}
	}

	withSessionFixture := newPendingWaitVerdictFixture(t, "job", false)
	primeOpenSignature(withSessionFixture)
	withSession := withSessionFixture.verdict(t, withSessionFixture.scan)
	if !strings.Contains(withSession.Display, "WAITING: registered wait") {
		t.Fatalf("control verdict did not prove that the registered wait was readable: %+v", withSession)
	}

	absentFixture := newPendingWaitVerdictFixture(t, "job", false)
	primeOpenSignature(absentFixture)
	clockReads := 0
	previousBootClock := turnVerdictBootClock
	turnVerdictBootClock = func() (string, time.Duration, error) {
		clockReads++
		return previousBootClock()
	}
	t.Cleanup(func() { turnVerdictBootClock = previousBootClock })
	absent, err := absentFixture.store.TurnVerdict(absentFixture.scan, pendingWaitSession, "", pendingWaitMainID, TurnVerdictOptions{
		SessionAbsent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := "registered waits were not read because the Stop supplied no session"
	if clockReads != 0 || len(absent.Diagnostics) != 1 || absent.Diagnostics[0] != diagnostic ||
		strings.Count(absent.Display, diagnostic) != 1 || strings.Contains(absent.Display, "WAITING: registered wait") {
		t.Fatalf("sessionless Stop read or reported a registered wait: clockReads=%d verdict=%+v", clockReads, absent)
	}
	blockSource := func(source *string) string {
		if source == nil {
			return ""
		}
		return *source
	}
	if absent.ShouldBlock != withSession.ShouldBlock || blockSource(absent.BlockSource) != blockSource(withSession.BlockSource) {
		t.Fatalf("session absence changed the Stop decision: with-session=%+v absent=%+v", withSession, absent)
	}
}

func TestPendingWaitTurnVerdict(t *testing.T) {
	t.Run("work waits suppress matching open work and their own unwatched join", func(t *testing.T) {
		liveDelegate := newPendingWaitVerdictFixture(t, "job", false)
		if err := os.Remove(metarun.WaiterPath(liveDelegate.root, liveDelegate.row.Kind, liveDelegate.row.TargetID, liveDelegate.row.OwnerDigest)); err != nil {
			t.Fatal(err)
		}
		prober := liveDelegate.store.Prober.(idleFixtureProber)
		prober[43] = identity.Exact{Pid: 43, StartedAt: time.Unix(300, 0), StartTicks: 430, BootID: pendingWaitBootID}
		writePendingWaitJSON(t, filepath.Join(liveDelegate.root, "artifacts", "agents", "jobs", liveDelegate.row.TargetID+".json"), map[string]any{
			"jobId": liveDelegate.row.TargetID, "operationId": liveDelegate.row.Target.OperationID,
			"round": liveDelegate.row.Target.Round, "startedAt": liveDelegate.row.Target.StartedAt,
			"status": "running", "goalId": pendingWaitGoalID, "mainId": pendingWaitMainID,
			"ownerLineage": pendingWaitLineage, "pid": 43, "pidStartedAt": 300,
			"pidStartTicks": 430, "bootId": pendingWaitBootID,
		})
		liveScan := liveDelegate.scan
		liveScan.Busy = []Item{{Kind: "job", Id: liveDelegate.row.TargetID, Detail: "delegate job is running"}}
		liveVerdict := liveDelegate.verdict(t, liveScan)
		if liveVerdict.ShouldBlock || liveVerdict.BlockSource != nil {
			t.Fatalf("live delegate control blocked: %+v", liveVerdict)
		}

		for _, kind := range []string{"job", "run", "attempt", "landing"} {
			t.Run(kind, func(t *testing.T) {
				fixture := newPendingWaitVerdictFixture(t, kind, false)
				scan := fixture.scan
				scan.Busy = append([]Item{}, liveScan.Busy...)
				switch kind {
				case "job":
					scan.Jobs = []JobFact{{
						Id: fixture.row.TargetID, MainId: pendingWaitMainID, StartedAt: fixture.row.Target.StartedAt, Status: "running",
					}}
				case "run":
					scan.Runs = []RunFact{{
						Id: fixture.row.TargetID, MainId: pendingWaitMainID, Generation: fixture.row.Target.Generation,
						Nonce: fixture.row.Target.LaunchNonce, Status: metarun.StatusLaunching, Supervised: true,
					}}
				case "attempt":
					attempt, err := proofrun.ReadAttempt(fixture.root, fixture.row.TargetID)
					if err != nil || attempt.ReservationOwner == nil {
						t.Fatalf("read governed attempt: %+v %v", attempt, err)
					}
					scan.Runs = []RunFact{{
						Id: attempt.ReservationOwner.RunID, MainId: pendingWaitMainID,
						Generation: attempt.ReservationOwner.RunGeneration, Nonce: attempt.ReservationOwner.LaunchNonce,
						Status: metarun.StatusLaunching, Supervised: true,
					}}
				}
				verdict := fixture.verdict(t, scan)
				withoutWaitLine := func(display string) string {
					var lines []string
					for _, line := range strings.Split(display, "\n") {
						if !strings.HasPrefix(line, "WAITING: registered wait") && !strings.HasPrefix(line, "Full turn verdict:") {
							lines = append(lines, line)
						}
					}
					return strings.Join(lines, "\n")
				}
				if verdict.ShouldBlock != liveVerdict.ShouldBlock || verdict.BlockSource != liveVerdict.BlockSource ||
					verdict.IdleRefusal != liveVerdict.IdleRefusal || verdict.CountSpent != liveVerdict.CountSpent ||
					withoutWaitLine(verdict.Display) != withoutWaitLine(liveVerdict.Display) ||
					!strings.Contains(verdict.Display, "WAITING: registered wait") ||
					!strings.Contains(verdict.Display, fixture.row.Deadline) || strings.Contains(verdict.Display, "unwatched") {
					t.Fatalf("valid %s wait did not carry work in flight: %+v", kind, verdict)
				}
			})
		}
	})

	t.Run("the live Stop session must match both row sessions", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		fixture.row.Session = "another-logical-session"
		fixture.row.RuntimeSession = "another-logical-session"
		fixture.writeRow(t)
		scan := fixture.scan
		scan.Jobs = []JobFact{{
			Id: fixture.row.TargetID, MainId: pendingWaitMainID, StartedAt: fixture.row.Target.StartedAt, Status: "running",
		}}
		verdict := fixture.verdict(t, scan)
		if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" ||
			strings.Contains(verdict.Display, "WAITING: registered wait") {
			t.Fatalf("another logical session received the wait allowance: %+v", verdict)
		}
	})

	t.Run("a lineage filled after registration accepts the main default only", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		fixture.row.OwnerLineage = pendingWaitMainID
		fixture.writeRow(t)
		if verdict := fixture.verdict(t, fixture.scan); verdict.ShouldBlock || !strings.Contains(verdict.Display, "WAITING: registered wait") {
			t.Fatalf("the pre-fill main lineage was refused: %+v", verdict)
		}
		fixture.row.OwnerLineage = "another-lineage"
		fixture.writeRow(t)
		if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock || strings.Contains(verdict.Display, "WAITING: registered wait") {
			t.Fatalf("an unrelated lineage received the wait allowance: %+v", verdict)
		}
	})

	t.Run("human act suppresses only its matching waiting condition", func(t *testing.T) {
		for _, kind := range []string{"human-act", "channel"} {
			t.Run(kind, func(t *testing.T) {
				fixture := newPendingWaitVerdictFixture(t, kind, false)
				scan := fixture.scan
				scan.WaitingOnHuman = []Item{{Kind: "plan", Id: "plans/waiting.md", Detail: "claimed goal waits on Wido"}}
				if verdict := fixture.verdict(t, scan); verdict.ShouldBlock {
					t.Fatalf("matching %s wait did not suppress saved open work: %+v", kind, verdict)
				}
				scan.WaitingOnHuman = []Item{{Kind: "goal", Id: "another-goal", Detail: "another goal waits on Wido"}}
				if verdict := fixture.verdict(t, scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
					t.Fatalf("another goal's waiting condition gained a %s exemption: %+v", kind, verdict)
				}
				withoutCondition := newPendingWaitVerdictFixture(t, kind, false)
				if verdict := withoutCondition.verdict(t, withoutCondition.scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
					t.Fatalf("unrelated waiting condition gained a %s exemption: %+v", kind, verdict)
				}
			})
		}
	})

	t.Run("changed open signature is never suppressed", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		scan := fixture.scan
		scan.Open[0].Detail += " and newly added work"
		scan.Jobs = []JobFact{{
			Id: fixture.row.TargetID, MainId: pendingWaitMainID, StartedAt: fixture.row.Target.StartedAt, Status: "running",
		}}
		if verdict := fixture.verdict(t, scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" ||
			strings.Contains(verdict.Display, "unwatched") {
			t.Fatalf("changed open work was suppressed: %+v", verdict)
		}
	})

	t.Run("unrelated unwatched work still blocks", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		scan := fixture.scan
		scan.Jobs = []JobFact{{Id: "another-job", MainId: pendingWaitMainID, StartedAt: "another-start", Status: "running"}}
		verdict := fixture.verdict(t, scan)
		if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" {
			t.Fatalf("an unrelated job gained the wait's unwatched exemption: %+v", verdict)
		}
	})

	t.Run("an attempt wait watches only its governed run incarnation", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "attempt", false)
		attempt, err := proofrun.ReadAttempt(fixture.root, fixture.row.TargetID)
		if err != nil || attempt.ReservationOwner == nil {
			t.Fatalf("read governed attempt: %+v %v", attempt, err)
		}
		scan := fixture.scan
		scan.Runs = []RunFact{{
			Id: attempt.ReservationOwner.RunID, MainId: pendingWaitMainID,
			Generation: attempt.ReservationOwner.RunGeneration + 1, Nonce: attempt.ReservationOwner.LaunchNonce,
			Status: metarun.StatusLaunching, Supervised: true,
		}}
		if verdict := fixture.verdict(t, scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" {
			t.Fatalf("attempt wait covered a different run incarnation: %+v", verdict)
		}
	})

	t.Run("only the canonical waiter path is eligible", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		expected := metarun.WaiterPath(fixture.root, fixture.row.Kind, fixture.row.TargetID, fixture.row.OwnerDigest)
		if err := os.Rename(expected, filepath.Join(metarun.WaitersDir(fixture.root), "copied-row.json")); err != nil {
			t.Fatal(err)
		}
		if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
			t.Fatalf("a wait outside WaiterPath changed the decision: %+v", verdict)
		}
	})

	t.Run("malformed waiter is ineligible", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		path := metarun.WaiterPath(fixture.root, fixture.row.Kind, fixture.row.TargetID, fixture.row.OwnerDigest)
		if err := os.WriteFile(path, []byte("{\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
			t.Fatalf("a malformed waiter changed the decision: %+v", verdict)
		}
	})

	t.Run("owner lifecycle is required", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			mutate func(*pendingWaitVerdictFixture)
		}{
			{"dead lease", func(f *pendingWaitVerdictFixture) {
				prober := f.store.Prober.(idleFixtureProber)
				delete(prober, 41)
			}},
			{"missing announcement", func(f *pendingWaitVerdictFixture) {
				path := filepath.Join(f.root, "artifacts", "agents", "mains", pendingWaitSession+"-41.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}},
			{"duplicate main announcement", func(f *pendingWaitVerdictFixture) {
				writePendingWaitJSON(t, filepath.Join(f.root, "artifacts", "agents", "mains", "duplicate-41.json"), sessionStopAnnouncement{
					SessionId: "another-placeholder", RuntimeSession: pendingWaitSession,
					MainId: pendingWaitMainID, Pid: 41, PidStartedAt: 100, PidStartTicks: 410, BootID: pendingWaitBootID,
					Runtime: "fake", InstanceTag: "duplicate", CommandHash: strings.Repeat("a", 64),
					AnnouncedAt: f.store.Now().Add(-time.Minute).Format(time.RFC3339), Pgid: 41, OwnerLineage: pendingWaitLineage,
				})
			}},
			{"announcement birth differs from lease", func(f *pendingWaitVerdictFixture) {
				writePendingWaitJSON(t, filepath.Join(f.root, "artifacts", "agents", "mains", pendingWaitSession+"-41.json"), sessionStopAnnouncement{
					SessionId: pendingWaitAnnouncementSession, RuntimeSession: pendingWaitSession,
					MainId: pendingWaitMainID, Pid: 41, PidStartedAt: 101, PidStartTicks: 410, BootID: pendingWaitBootID,
					Runtime: "fake", InstanceTag: "wrong-birth", CommandHash: strings.Repeat("a", 64),
					AnnouncedAt: f.store.Now().Add(-time.Minute).Format(time.RFC3339), Pgid: 41, OwnerLineage: pendingWaitLineage,
				})
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				fixture := newPendingWaitVerdictFixture(t, "job", false)
				test.mutate(fixture)
				if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
					t.Fatalf("wait without a live owner changed the decision: %+v", verdict)
				}
			})
		}
	})

	t.Run("failed eligibility changes no decision", func(t *testing.T) {
		result := &metarun.WaitResult{SchemaVersion: 2, WaitID: strings.Repeat("b", 32)}
		cases := []struct {
			name   string
			mutate func(*pendingWaitVerdictFixture)
		}{
			{"schema", func(f *pendingWaitVerdictFixture) { f.row.SchemaVersion = 1 }},
			{"state", func(f *pendingWaitVerdictFixture) { f.row.State = "ready" }},
			{"delivery", func(f *pendingWaitVerdictFixture) { f.row.Delivery = "declined" }},
			{"result", func(f *pendingWaitVerdictFixture) { f.row.Result = result }},
			{"wait id", func(f *pendingWaitVerdictFixture) { f.row.WaitID = "short" }},
			{"nonce", func(f *pendingWaitVerdictFixture) { f.row.Nonce = "short" }},
			{"main", func(f *pendingWaitVerdictFixture) { f.row.MainId = "another-main" }},
			{"session", func(f *pendingWaitVerdictFixture) { f.row.Session = "another-session" }},
			{"runtime session", func(f *pendingWaitVerdictFixture) { f.row.RuntimeSession = "another-session" }},
			{"lineage", func(f *pendingWaitVerdictFixture) { f.row.OwnerLineage = "another-lineage" }},
			{"owner digest", func(f *pendingWaitVerdictFixture) { f.row.OwnerDigest = metarun.OwnerDigest("another-main") }},
			{"missing claim epoch", func(f *pendingWaitVerdictFixture) { f.row.ClaimEpoch = nil }},
			{"claim epoch", func(f *pendingWaitVerdictFixture) { epoch := int64(8); f.row.ClaimEpoch = &epoch }},
			{"selector kind", func(f *pendingWaitVerdictFixture) { f.row.Selector.Kind = "run" }},
			{"selector target", func(f *pendingWaitVerdictFixture) { f.row.Selector.TargetID = "another-job" }},
			{"zero target", func(f *pendingWaitVerdictFixture) { f.row.Target = metarun.WaiterTarget{} }},
			{"malformed signature", func(f *pendingWaitVerdictFixture) { f.row.OpenWorkSignature = "not-a-digest" }},
			{"selector extras", func(f *pendingWaitVerdictFixture) { f.row.Selector.Event = "landing" }},
			{"unexpected goal binding", func(f *pendingWaitVerdictFixture) { f.row.GoalID = pendingWaitGoalID }},
			{"dead waiter", func(f *pendingWaitVerdictFixture) { f.row.Pid = 404 }},
			{"bad registered time", func(f *pendingWaitVerdictFixture) { f.row.RegisteredAt = "bad" }},
			{"bad deadline", func(f *pendingWaitVerdictFixture) { f.row.Deadline = "bad" }},
			{"bad observation time", func(f *pendingWaitVerdictFixture) { f.row.LastObservedAt = "bad" }},
			{"observation before registration", func(f *pendingWaitVerdictFixture) {
				f.row.LastObservedAt = f.store.Now().Add(-10 * time.Second).Format(time.RFC3339Nano)
			}},
			{"future observation", func(f *pendingWaitVerdictFixture) {
				f.row.LastObservedAt = f.store.Now().Add(time.Second).Format(time.RFC3339Nano)
			}},
			{"stale observation", func(f *pendingWaitVerdictFixture) {
				f.row.LastObservedAt = f.store.Now().Add(-31 * time.Second).Format(time.RFC3339Nano)
			}},
			{"expired deadline", func(f *pendingWaitVerdictFixture) { f.row.Deadline = f.store.Now().Format(time.RFC3339Nano) }},
			{"deadline before registration", func(f *pendingWaitVerdictFixture) {
				f.row.Deadline = f.store.Now().Add(-6 * time.Second).Format(time.RFC3339Nano)
			}},
			{"deadline over maximum", func(f *pendingWaitVerdictFixture) {
				f.row.Deadline = f.store.Now().Add(25 * time.Hour).Format(time.RFC3339Nano)
			}},
			{"no remaining time", func(f *pendingWaitVerdictFixture) { f.row.RemainingNanos = 0 }},
			{"missing boot", func(f *pendingWaitVerdictFixture) { f.row.BootID = "" }},
			{"changed boot", func(f *pendingWaitVerdictFixture) { f.row.BootID = "another-boot" }},
			{"deadline boot", func(f *pendingWaitVerdictFixture) { f.row.DeadlineBootID = "another-boot" }},
			{"missing boot observation", func(f *pendingWaitVerdictFixture) { f.row.LastObservedBootNanos = 0 }},
			{"future boot observation", func(f *pendingWaitVerdictFixture) {
				f.row.LastObservedBootNanos = (f.bootElapsed + time.Second).Nanoseconds()
			}},
			{"stale boot observation", func(f *pendingWaitVerdictFixture) {
				f.row.LastObservedBootNanos = (f.bootElapsed - 31*time.Second).Nanoseconds()
			}},
			{"boot deadline", func(f *pendingWaitVerdictFixture) { f.row.BootDeadlineNanos = f.bootElapsed.Nanoseconds() }},
			{"boot duration over maximum", func(f *pendingWaitVerdictFixture) {
				f.row.BootDeadlineNanos = f.row.LastObservedBootNanos + (25 * time.Hour).Nanoseconds()
			}},
			{"source incarnation", func(f *pendingWaitVerdictFixture) { f.row.Target.Round++ }},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				fixture := newPendingWaitVerdictFixture(t, "job", false)
				test.mutate(fixture)
				fixture.writeRow(t)
				verdict := fixture.verdict(t, fixture.scan)
				if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
					t.Fatalf("ineligible wait changed today's decision: %+v", verdict)
				}
			})
		}
	})

	t.Run("source must join the claimed goal", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		writePendingWaitJSON(t, filepath.Join(fixture.root, "artifacts", "agents", "jobs", fixture.row.TargetID+".json"), map[string]any{
			"jobId": fixture.row.TargetID, "operationId": fixture.row.Target.OperationID, "round": fixture.row.Target.Round,
			"startedAt": fixture.row.Target.StartedAt, "status": "running", "goalId": "another-goal",
		})
		if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
			t.Fatalf("an unrelated goal wait changed the decision: %+v", verdict)
		}
	})

	t.Run("source must remain pending", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		writePendingWaitJSON(t, filepath.Join(fixture.root, "artifacts", "agents", "jobs", fixture.row.TargetID+".json"), map[string]any{
			"jobId": fixture.row.TargetID, "operationId": fixture.row.Target.OperationID, "round": fixture.row.Target.Round,
			"startedAt": fixture.row.Target.StartedAt, "status": "succeeded", "goalId": pendingWaitGoalID,
		})
		if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
			t.Fatalf("terminal source wait changed the decision: %+v", verdict)
		}
	})

	t.Run("landing wait requires the landing phase", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "human-act", false)
		fixture.row.Selector.Event = "landing"
		fixture.row.GoalID = pendingWaitGoalID
		fixture.writeRow(t)
		if verdict := fixture.verdict(t, fixture.scan); !verdict.ShouldBlock {
			t.Fatalf("a landing wait covered a working claim: %+v", verdict)
		}
	})

	t.Run("warnings and degraded input are unchanged", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		scan := fixture.scan
		scan.Runs = []RunFact{{Id: "red-run", Status: "red", ExpectRed: "inspect red evidence"}}
		verdict := fixture.verdict(t, scan)
		if verdict.ShouldBlock || !strings.Contains(verdict.Display, "went red") || !strings.Contains(verdict.Display, "inspect red evidence") {
			t.Fatalf("a registered wait hid a warning: %+v", verdict)
		}
		scan.Unreadable = []string{"plans/broken.md: unreadable"}
		verdict = fixture.verdict(t, scan)
		if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" ||
			len(verdict.Diagnostics) == 0 || verdict.Diagnostics[len(verdict.Diagnostics)-1] != "plans/broken.md: unreadable" {
			t.Fatalf("a registered wait changed degraded scanner input: %+v", verdict)
		}
	})

	t.Run("closed fence remains terminal", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		if err := stopfence.Write(fixture.root, stopfence.Record{
			SchemaVersion: stopfence.SchemaVersion, State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
			Generation: 1, ChangedAt: "2026-08-23T01:59:00Z", Checkout: fixture.root,
			By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}}, NotStopped: []stopfence.Survivor{},
		}); err != nil {
			t.Fatal(err)
		}
		verdict := fixture.verdict(t, fixture.scan)
		if verdict.ShouldBlock || verdict.LedgerStatus != "stopped" || strings.Contains(verdict.Display, "WAITING: registered wait") {
			t.Fatalf("a registered wait changed the closed fence: %+v", verdict)
		}
	})

	t.Run("human stop authority remains final", func(t *testing.T) {
		fixture := newPendingWaitVerdictFixture(t, "job", false)
		prober := fixture.store.Prober.(idleFixtureProber)
		prober[20] = identity.Exact{Pid: 20, StartedAt: time.Unix(200, 0)}
		proof := *testHumanAuthority(t, fixture.root, fixture.store.Now())
		marker, err := fixture.store.WriteSessionStop(SessionStop{
			SchemaVersion: 3, SessionId: pendingWaitAnnouncementSession, HolderMainId: pendingWaitMainID, ClaimEpoch: 7,
			By: "Wido", WrittenAt: fixture.store.Now().Format(time.RFC3339), ExpiresAt: fixture.store.Now().Add(time.Hour).Format(time.RFC3339),
		}, proof)
		if err != nil {
			t.Fatal(err)
		}
		verdict, err := fixture.store.TurnVerdict(fixture.scan, marker.SessionId, "", marker.HolderMainId)
		if err != nil || verdict.ShouldBlock || !strings.Contains(verdict.Display, "SESSION STOP authorized once") {
			t.Fatalf("registered-wait work changed human stop authority: %+v %v", verdict, err)
		}
	})
}

// The dual-slot block-once state machine — the canonical sequence:
// goal-block, open-work-block, clear, and NO re-block of the unchanged
// goal.
func TestVerdictDualSlotSequence(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "the goal", "Ship it.")

	// 1. Nothing scanned: the goal blocks once, byte-verbatim step.
	v, err := s.TurnVerdict(ScanResult{}, "session-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !v.ShouldBlock || *v.BlockSource != "goal" || !strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("goal did not block first: %+v", v)
	}

	// 2. Open work appears: open-work blocks once.
	scan := ScanResult{Open: []Item{openItem("plans/x.md next: do")}}
	v, _ = s.TurnVerdict(scan, "session-1", "", "")
	if !v.ShouldBlock || *v.BlockSource != "open-work" {
		t.Fatalf("open work did not block: %+v", v)
	}
	// Same signature again: reported, not re-blocked.
	v, _ = s.TurnVerdict(scan, "session-1", "", "")
	if v.ShouldBlock {
		t.Fatalf("unchanged open work re-blocked: %+v", v)
	}

	// 3. Work clears: the unchanged goal does NOT re-block (its revision
	// is spent), and the all-clear names the goal.
	v, _ = s.TurnVerdict(ScanResult{}, "session-1", "", "")
	if v.ShouldBlock {
		t.Fatalf("the spent goal revision re-blocked: %+v", v)
	}
	if !strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "g") {
		t.Fatalf("all-clear does not name the goal: %s", v.Display)
	}

	// 4. The step changes: the new revision blocks once more.
	if _, err := s.SetNext(mainHolder, "Ship it harder."); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "session-1", "", "")
	if !v.ShouldBlock || !strings.Contains(v.Display, "Ship it harder.") {
		t.Fatalf("a re-armed revision did not block: %+v", v)
	}

	// A different session has its own slots.
	v, _ = s.TurnVerdict(ScanResult{}, "session-2", "", "")
	if !v.ShouldBlock {
		t.Fatal("a fresh session inherited a spent revision")
	}
}

// Precedence — busy suppresses everything; human-waits suppress
// the goal clause; stale plans never block; unreadable vetoes both ways.
func TestPrecedenceLadder(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "the goal", "Ship it.")

	v, _ := s.TurnVerdict(ScanResult{Busy: []Item{{Kind: "mission", Id: "m1", Detail: "mission m1 [running]"}}}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "STILL WORKING") || strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("busy precedence wrong: %+v", v)
	}

	v, _ = s.TurnVerdict(ScanResult{WaitingOnHuman: []Item{{Kind: "plan", Id: "w", Detail: "plans/w.md waits on the human"}}}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "WAITING ON THE HUMAN") || strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("human-wait precedence wrong: %+v", v)
	}

	// Stale plans are warning-only: they ride Diagnostics/display via the
	// scanner, never the block path — an all-empty-but-stale scan lets
	// the goal block normally.
	v, _ = s.TurnVerdict(ScanResult{StalePlans: []Item{{Kind: "plan", Id: "old", Detail: "plans/old.md is stale"}}}, "s", "", "")
	if !v.ShouldBlock || *v.BlockSource != "goal" {
		t.Fatalf("stale plans changed the goal outcome: %+v", v)
	}

	// Unreadable vetoes the goal block AND the all-clear.
	v, _ = s.TurnVerdict(ScanResult{Unreadable: []string{"plans/broken.md: permission denied"}}, "s2", "", "")
	if v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "unreadable") {
		t.Fatalf("unreadable veto wrong: %+v", v)
	}
	if len(v.Diagnostics) != 1 {
		t.Fatalf("unreadable path missing from diagnostics: %+v", v.Diagnostics)
	}
}

func TestInfrastructureVerdictNeverBlocks(t *testing.T) {
	assertInfrastructure := func(t *testing.T, verdict Verdict, err error, component, detail string) {
		t.Helper()
		if err != nil || verdict.ShouldBlock || verdict.Class != "infrastructure" || verdict.LedgerStatus != "degraded" ||
			verdict.Component != component || !strings.Contains(verdict.Display, detail) {
			t.Fatalf("%s producer did not allow with its detail: %+v %v", component, verdict, err)
		}
	}
	t.Run("state root", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "development"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "development", "metasystem-design.md"), []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		verdict, err := (&Store{Root: root}).TurnVerdict(ScanResult{}, "state-root", "", "")
		assertInfrastructure(t, verdict, err, "state-root", "missing metasystem.conf")
	})
	t.Run("goal fence", func(t *testing.T) {
		store := testStore(t)
		path := stopfence.TransitionPath(store.Root)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		verdict, err := store.TurnVerdict(ScanResult{}, "goal-fence", "", "")
		assertInfrastructure(t, verdict, err, "goal-fence", "goal fence")
	})
	t.Run("verdict state", func(t *testing.T) {
		store := testStore(t)
		if err := os.MkdirAll(filepath.Dir(statePath(store.Root)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statePath(store.Root), []byte("{broken\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		verdict, err := store.TurnVerdict(ScanResult{}, "verdict-state", "", "")
		assertInfrastructure(t, verdict, err, "verdict-state", "turn verdict state")
	})
	t.Run("status write", func(t *testing.T) {
		store := &Store{Root: servingBed(t, "bed-m1", nil), Now: func() time.Time { return time.Unix(1786800000, 0) }}
		record := brain.Record{Schema: 1, Ledger: ExistingLedgerIdentity(store.Root), Machine: "bed-m1", DeclaredBy: "Wido", DeclaredAt: "2026-09-12T00:00:00Z"}
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(brain.Path(store.Root)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(brain.Path(store.Root), append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		previous := brainStatusWriter
		brainStatusWriter = func(string, brain.Record, time.Time) error { return fmt.Errorf("fixture status failure") }
		t.Cleanup(func() { brainStatusWriter = previous })
		verdict, err := store.TurnVerdict(ScanResult{}, "status-write", "", "")
		assertInfrastructure(t, verdict, err, "status-write", "fixture status failure")
	})
}

func TestTurnVerdictAllowsTheStopUnderARecordedHandoff(t *testing.T) {
	openWork := ScanResult{Open: []Item{openItem("plans/handoff.md next: continue this session")}}

	t.Run("same session precedes open work", func(t *testing.T) {
		store := testStore(t)
		var lookedUp string
		verdict, err := store.TurnVerdict(openWork, "handoff-session", "", "main-1", TurnVerdictOptions{
			HandoffRecorded: func(session string) (string, bool, error) {
				lookedUp = session
				return "handoff-nonce", session == "handoff-session", nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if lookedUp != "handoff-session" {
			t.Fatalf("handoff lookup used session %q", lookedUp)
		}
		if verdict.SchemaVersion != 1 || verdict.ShouldBlock || verdict.BlockSource != nil || verdict.Class != "seat-actionable" ||
			verdict.LedgerStatus != "ok" || verdict.Display != "handoff recorded: handoff-nonce; end this session" {
			t.Fatalf("same-session handoff verdict = %#v", verdict)
		}
	})

	t.Run("closed checkout keeps precedence", func(t *testing.T) {
		store := testStore(t)
		if err := stopfence.Write(store.Root, stopfence.Record{
			State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 1,
			ChangedAt: "2026-09-14T00:00:00Z", Checkout: store.Root,
			By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}},
		}); err != nil {
			t.Fatal(err)
		}
		lookedUp := false
		verdict, err := store.TurnVerdict(openWork, "handoff-session", "", "main-1", TurnVerdictOptions{
			HandoffRecorded: func(string) (string, bool, error) {
				lookedUp = true
				return "handoff-nonce", true, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if lookedUp || verdict.ShouldBlock || verdict.LedgerStatus != "stopped" ||
			strings.Contains(verdict.Display, "handoff recorded") {
			t.Fatalf("closed-checkout precedence changed: lookedUp=%t verdict=%#v", lookedUp, verdict)
		}
	})

	t.Run("foreign session gives no allowance", func(t *testing.T) {
		store := testStore(t)
		verdict, err := store.TurnVerdict(openWork, "current-session", "", "main-1", TurnVerdictOptions{
			HandoffRecorded: func(session string) (string, bool, error) {
				if session == "foreign-session" {
					return "foreign-nonce", true, nil
				}
				return "", false, nil
			},
		})
		if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
			t.Fatalf("foreign handoff changed the current session: %#v %v", verdict, err)
		}
	})

	for _, state := range []struct {
		name  string
		nonce string
	}{
		{name: "cancelled", nonce: "cancelled-nonce"},
		{name: "consumed", nonce: "consumed-nonce"},
		{name: "absent"},
	} {
		t.Run(state.name+" handoff gives no allowance", func(t *testing.T) {
			store := testStore(t)
			verdict, err := store.TurnVerdict(openWork, "handoff-session", "", "main-1", TurnVerdictOptions{
				HandoffRecorded: func(string) (string, bool, error) { return state.nonce, false, nil },
			})
			if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
				t.Fatalf("%s handoff changed the verdict: %#v %v", state.name, verdict, err)
			}
		})
	}

	t.Run("unreadable handoff is infrastructure failure", func(t *testing.T) {
		store := testStore(t)
		verdict, err := store.TurnVerdict(openWork, "handoff-session", "", "main-1", TurnVerdictOptions{
			HandoffRecorded: func(string) (string, bool, error) {
				return "", false, fmt.Errorf("fixture handoff read failed")
			},
		})
		if err != nil || verdict.ShouldBlock || verdict.Class != "infrastructure" ||
			verdict.LedgerStatus != "degraded" || verdict.Component != "handoff-record" ||
			verdict.CauseCode != "handoff-record" || verdict.Display != "handoff record: fixture handoff read failed" {
			t.Fatalf("unreadable handoff verdict = %#v %v", verdict, err)
		}
	})

	t.Run("nil seam gives no allowance", func(t *testing.T) {
		store := testStore(t)
		verdict, err := store.TurnVerdict(openWork, "handoff-session", "", "main-1")
		if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "open-work" {
			t.Fatalf("nil handoff seam changed the verdict: %#v %v", verdict, err)
		}
	})

	t.Run("command supplies steward lookup", func(t *testing.T) {
		_, testFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("turn-verdict test source path is unavailable")
		}
		commandSource, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "cmd", "metasystem", "goal.go"))
		if err != nil {
			t.Fatal(err)
		}
		source := string(commandSource)
		if !strings.Contains(source, "options.HandoffRecorded = func(session string) (string, bool, error)") ||
			!strings.Contains(source, "return steward.LiveHandoffForSession(stateRoot, session)") {
			t.Fatal("runReportTurnVerdict does not supply the state-root-scoped steward handoff lookup")
		}
	})
}

func TestHandoffAllowanceCarriesFrozenFacts(t *testing.T) {
	store := testStore(t)
	scan := ScanResult{Open: []Item{{
		Kind: "plan", Id: "handoff-plan", Detail: "plans/handoff.md next: continue this session",
		RequestedAction: "continue this session", OwnerMainId: "main-1",
	}}}
	verdict, err := store.TurnVerdict(scan, "handoff-session", "", "main-1", TurnVerdictOptions{
		HandoffRecorded: func(string) (string, bool, error) { return "handoff-nonce", true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Facts == nil {
		t.Fatal("handoff allowance has no frozen facts")
	}
	facts := verdict.Facts
	if facts.SchemaVersion != 1 || facts.Identity.Installation != store.Root ||
		facts.Identity.Session != "handoff-session" || facts.Identity.MainId != "main-1" ||
		facts.Identity.ObservedAt != store.nowISO() {
		t.Fatalf("handoff identity facts = %#v", facts.Identity)
	}
	if facts.Verdict.SchemaVersion != verdict.SchemaVersion || facts.Verdict.Class != verdict.Class ||
		facts.Verdict.ShouldBlock != verdict.ShouldBlock || facts.Verdict.LedgerStatus != verdict.LedgerStatus ||
		facts.Verdict.Display != verdict.Display || facts.FullDisplay != verdict.Display {
		t.Fatalf("handoff verdict facts = %#v; verdict = %#v", facts.Verdict, verdict)
	}
	if len(facts.Scan.Open) != 1 || facts.Scan.Open[0].Id != "handoff-plan" {
		t.Fatalf("handoff scan facts = %#v", facts.Scan)
	}
	if facts.Work.ReadSucceeded || facts.Work.Selection != "unknown" || facts.Ownership.State != "unknown" {
		t.Fatalf("handoff allowance fabricated a ledger read: work=%#v ownership=%#v", facts.Work, facts.Ownership)
	}
	if facts.Refusal.HumanStopConsumed {
		t.Fatalf("handoff allowance consumed a human stop: %#v", facts.Refusal)
	}
	if len(facts.Actions) != 0 {
		t.Fatalf("handoff allowance offered an action to continue this session: %#v", facts.Actions)
	}
}

func TestInfrastructurePersistenceFailurePreservesSeatActionableRefusal(t *testing.T) {
	store := &Store{Root: servingBed(t, "bed-m1", nil), Now: func() time.Time { return time.Unix(1786800000, 0) }}
	previous := verdictStateWriter
	verdictStateWriter = func(string, []byte) error { return fmt.Errorf("fixture state write failure") }
	t.Cleanup(func() { verdictStateWriter = previous })
	verdict, err := store.TurnVerdict(ScanResult{Open: []Item{openItem("OPEN-WORK fixture: finish it")}}, "seat-block", "", "")
	if err != nil || !verdict.ShouldBlock || verdict.Class != "seat-actionable" ||
		!strings.Contains(verdict.Display, "observed refusal remains blocking") {
		t.Fatalf("seat-actionable refusal was lost with its state write: %+v %v", verdict, err)
	}
	if _, err := os.Stat(turnVerdictArtifactPath(store.Root, "seat-block")); err != nil {
		t.Fatalf("surviving refusal did not write its turn-verdict artifact: %v", err)
	}
}

func TestInfrastructureStatusWriteFailurePreservesSeatActionableRefusal(t *testing.T) {
	store := &Store{Root: servingBed(t, "bed-m1", nil), Now: func() time.Time { return time.Unix(1786800000, 0) }}
	record := brain.Record{Schema: 1, Ledger: ExistingLedgerIdentity(store.Root), Machine: "bed-m1", DeclaredBy: "Wido", DeclaredAt: "2026-09-12T00:00:00Z"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(store.Root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(store.Root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := brainStatusWriter
	brainStatusWriter = func(string, brain.Record, time.Time) error { return fmt.Errorf("fixture status failure") }
	t.Cleanup(func() { brainStatusWriter = previous })
	verdict, err := store.TurnVerdict(ScanResult{Open: []Item{openItem("OPEN-WORK fixture: finish it")}}, "status-block", "", "")
	if err != nil || !verdict.ShouldBlock || verdict.Class != "seat-actionable" ||
		!strings.Contains(verdict.Display, "observed refusal remains blocking") {
		t.Fatalf("seat-actionable refusal was lost with its status write: %+v %v", verdict, err)
	}
}

// Pre-adoption absence is advisory; post-adoption deletion is
// degraded with the all-clear vetoed and reconcile named.
func TestAbsenceAdvisoryVsDeletionDegraded(t *testing.T) {
	s := testStore(t)
	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.LedgerStatus != "absent" || v.ShouldBlock || !strings.Contains(v.Display, "`goal open` starts one") || !strings.Contains(v.Display, "NOTHING LEFT") {
		t.Fatalf("pre-adoption absence not advisory: %+v", v)
	}

	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	os.Remove(LedgerPath(s.Root))
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.LedgerStatus != "degraded" || v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "reconcile") {
		t.Fatalf("post-adoption deletion not degraded: %+v", v)
	}
}

// The queued-only ledger blocks once naming the first queued
// goal, never a silent all-clear.
func TestQueuedOnlyVerdict(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "a", "goal a", "Do a.")
	mustOpen(t, s, mainHolder, "b", "goal b", "Do b.")
	// Reach queued-only via done --then then reopen of the done goal...
	// simpler: park current with --then b, then park b landing... park
	// requires successor. Reopen path: done a --then b, done b --and-none
	// refuses (queue empty is fine)... Construct directly instead: done
	// a --then b leaves b Current; reopen a; park b --then a... Simplest
	// real path: reopen drops Goal-free after everything concludes.
	if _, err := s.Done(mainHolder, "a", "landed", "b", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(mainHolder, "b", "landed", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reopen(mainHolder, "a", "Round two."); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ := s.ReadLedger()
	if ledger.Current != nil || len(ledger.Queued) != 1 || ledger.Free != nil {
		t.Fatalf("not queued-only: %+v", ledger)
	}

	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.LedgerStatus != "queued-only" || !v.ShouldBlock || !strings.Contains(v.Display, "IDLE WITH BACKLOG") {
		t.Fatalf("queued-only verdict wrong: %+v", v)
	}
	// Legacy backlog is the same every-stop invariant as converted backlog.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !v.ShouldBlock || v.BlockSource == nil || *v.BlockSource != "idle-backlog" {
		t.Fatalf("the second unchanged legacy stop must still block: %+v", v)
	}
}

func TestQueuedOnlyVerdictIgnoresLegacyBlockOnceDigest(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "legacy-queue", "legacy queued goal", "Promote it.")
	if _, err := s.Done(mainHolder, "legacy-queue", "landed", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reopen(mainHolder, "legacy-queue", "Queue it again."); err != nil {
		t.Fatal(err)
	}
	first, digest := s.queuedFrontier()
	if first != "legacy-queue" || digest == "" {
		t.Fatalf("queued frontier missing: first=%q digest=%q", first, digest)
	}
	state := &verdictState{SchemaVersion: 1, Sessions: map[string]*sessionState{
		"legacy-queue-session": {
			LastTouched: s.nowISO(), BlockedGoalRevisions: []string{digest},
		},
	}}
	if err := s.saveVerdictState(state); err != nil {
		t.Fatal(err)
	}

	verdict, err := s.TurnVerdict(ScanResult{}, "legacy-queue-session", "", "")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "idle-backlog" ||
		!strings.Contains(verdict.Display, "legacy-queue") {
		t.Fatalf("a spent legacy queue digest must not suppress the invariant: %+v %v", verdict, err)
	}
}

func TestClaimedSessionReblocksOnceWhenTheSharedQueueChanges(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	mustGit(t, root, "config", "metasystem.goal.machine", "mac-a")
	request := verbReq(root, "01J5X00000000000000000TV10", "mac-a")
	if result, err := Open(request, "claimed-here", "Keep working here.", OriginMain, "Continue it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open claimed goal: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000TV11"
	if result, err := claimApprovedForTest(t, request, "claimed-here", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim goal: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000TV12"
	if result, err := Open(request, "queued-pin", "Wait in the queue.", OriginMain, "Claim later."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open queued goal: %+v %v", result, err)
	}
	store := &Store{Root: root, Now: func() time.Time { return request.Now }, Prober: installIdleLiveClaim(t, root, "lin-1")}
	first, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !first.ShouldBlock {
		t.Fatalf("initial claimed world did not block: %+v %v", first, err)
	}
	spent, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || spent.ShouldBlock {
		t.Fatalf("unchanged claimed world reblocked: %+v %v", spent, err)
	}
	request.Ulid = "01J5X00000000000000000TV13"
	request.Actor.Human = "Wido"
	if result, err := SetPin(request, "queued-pin", "mac-a"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("pin queued goal: %+v %v", result, err)
	}
	changed, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !changed.ShouldBlock || !strings.Contains(changed.Display, "shared goal queue changed") || !strings.Contains(changed.Display, "queued-pin") {
		t.Fatalf("queue change did not reblock the claimed session once: %+v %v", changed, err)
	}
	again, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || again.ShouldBlock {
		t.Fatalf("unchanged queue digest reblocked twice: %+v %v", again, err)
	}
	request.Ulid = "01J5X00000000000000000TV14"
	if result, err := SetPin(request, "queued-pin", "-"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("clear queued pin: %+v %v", result, err)
	}
	cleared, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !cleared.ShouldBlock {
		t.Fatalf("pin clearing did not reblock the claimed session: %+v %v", cleared, err)
	}
	request.Ulid = "01J5X00000000000000000TV15"
	approveGoalForTest(t, request, "queued-pin", testBudget())
	request.Actor = Actor{Machine: "mac-b", Lineage: "turn-verdict-fixture"}
	if result, err := Claim(request, "queued-pin"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim the last queued goal elsewhere: %+v %v", result, err)
	}
	emptied, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !emptied.ShouldBlock || !strings.Contains(emptied.Display, "now empty") {
		t.Fatalf("the final queue departure did not reblock the claimed session: %+v %v", emptied, err)
	}
}

func TestClaimedSessionBaselinesAnUnchangedQueueWithoutFalseChange(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	mustGit(t, root, "config", "metasystem.goal.machine", "mac-a")
	request := verbReq(root, "01J5X00000000000000000TV20", "mac-a")
	if result, err := Open(request, "steady-claim", "Keep the steady claim.", OriginMain, "Continue it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open steady claim: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000TV21"
	if result, err := claimApprovedForTest(t, request, "steady-claim", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim steady goal: %+v %v", result, err)
	}
	store := &Store{Root: root, Now: func() time.Time { return request.Now }, Prober: installIdleLiveClaim(t, root, "lin-1")}
	first, err := store.TurnVerdict(ScanResult{}, "fresh-steady-session", "", "")
	if err != nil || !first.ShouldBlock || strings.Contains(first.Display, "shared goal queue changed") {
		t.Fatalf("a fresh session falsely described its empty queue baseline as a change: %+v %v", first, err)
	}
	state, err := store.loadVerdictState()
	if err != nil {
		t.Fatal(err)
	}
	state.Sessions["fresh-steady-session"].ObservedQueueDigest = ""
	if err := store.saveVerdictState(state); err != nil {
		t.Fatal(err)
	}
	rollout, err := store.TurnVerdict(ScanResult{}, "fresh-steady-session", "", "")
	if err != nil || rollout.ShouldBlock || strings.Contains(rollout.Display, "shared goal queue changed") {
		t.Fatalf("an upgraded pre-existing session falsely described its steady queue as a change: %+v %v", rollout, err)
	}
}

// A goal-free declaration over a moved world blocks
// once; renewal re-arms the all-clear.
func TestGoalFreeStaleness(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "goal-free declared") {
		t.Fatalf("fresh declaration did not read all-clear: %+v", v)
	}

	// The world moves.
	os.MkdirAll(filepath.Join(s.Root, "plans"), 0o755)
	os.WriteFile(filepath.Join(s.Root, "plans", "new-work.md"), []byte("x"), 0o644)
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !v.ShouldBlock || !strings.Contains(v.Display, "predates new work") {
		t.Fatalf("stale declaration did not block: %+v", v)
	}
	// Once per world.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.ShouldBlock {
		t.Fatalf("stale declaration re-blocked: %+v", v)
	}
	// A FURTHER world change blocks once more.
	os.WriteFile(filepath.Join(s.Root, "plans", "even-newer.md"), []byte("y"), 0o644)
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !v.ShouldBlock {
		t.Fatalf("a further world change did not block: %+v", v)
	}
	// Renewal restores the all-clear.
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "goal-free declared") {
		t.Fatalf("renewal did not restore the all-clear: %+v", v)
	}
}

// The sessions map caps at 128 oldest-evicted, session ids
// normalize, and concurrent verdicts serialize under the flock.
func TestSessionMapCapAndHygiene(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	base := time.Unix(1786800000, 0)
	for i := 0; i < 140; i++ {
		tick := base.Add(time.Duration(i) * time.Minute)
		s.Now = func() time.Time { return tick }
		if _, err := s.TurnVerdict(ScanResult{}, fmt.Sprintf("session-%03d", i), "", ""); err != nil {
			t.Fatal(err)
		}
	}
	state, err := s.loadVerdictState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Sessions) != maxSessions {
		t.Fatalf("sessions map holds %d, cap is %d", len(state.Sessions), maxSessions)
	}
	if _, oldest := state.Sessions["session-000"]; oldest {
		t.Fatal("the oldest session survived eviction")
	}

	// A path-shaped session id normalizes to its sha256.
	if _, err := s.TurnVerdict(ScanResult{}, "../../etc/passwd", "", ""); err != nil {
		t.Fatal(err)
	}
	state, _ = s.loadVerdictState()
	for id := range state.Sessions {
		if strings.Contains(id, "/") || strings.Contains(id, "..") {
			t.Fatalf("unnormalized session id stored: %q", id)
		}
	}

	// 30-day expiry drops dormant sessions on any write.
	s.Now = func() time.Time { return base.Add(40 * 24 * time.Hour) }
	if _, err := s.TurnVerdict(ScanResult{}, "fresh", "", ""); err != nil {
		t.Fatal(err)
	}
	state, _ = s.loadVerdictState()
	if len(state.Sessions) != 1 {
		t.Fatalf("expiry kept %d sessions", len(state.Sessions))
	}
}

// The watchdog protocol — changed surfaces, same suppresses,
// clear resets, same surfaces again; concurrent calls surface exactly
// once.
func TestWatchdogProtocol(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	digest := strings.Repeat("a", 64)
	v, _ := s.TurnVerdict(ScanResult{}, "s", digest, "")
	if !v.SurfaceWatchdog {
		t.Fatal("new digest did not surface")
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", digest, "")
	if v.SurfaceWatchdog {
		t.Fatal("same digest surfaced twice")
	}
	// No findings clears the slot...
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.SurfaceWatchdog {
		t.Fatal("clear surfaced")
	}
	// ...so the same digest surfaces again (recover-then-warn-again).
	v, _ = s.TurnVerdict(ScanResult{}, "s", digest, "")
	if !v.SurfaceWatchdog {
		t.Fatal("post-clear digest did not re-surface")
	}

	// Concurrent Stop calls: exactly one surfaces a fresh digest.
	fresh := strings.Repeat("b", 64)
	var wg sync.WaitGroup
	surfaced := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.TurnVerdict(ScanResult{}, "s", fresh, "")
			if err == nil {
				surfaced <- v.SurfaceWatchdog
			}
		}()
	}
	wg.Wait()
	close(surfaced)
	count := 0
	for got := range surfaced {
		if got {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("fresh digest surfaced %d times, want exactly 1", count)
	}
}

// Unreadable-veto tail: inventory failure (as Unreadable) vetoes even when the
// ledger is goal-free-fresh — no all-clear over unknown activity.
func TestInventoryFailureVetoes(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ := s.TurnVerdict(ScanResult{Unreadable: []string{"runners/m1.json: runner liveness unknown"}}, "s", "", "")
	if v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") {
		t.Fatalf("inventory failure did not veto: %+v", v)
	}
}

// Unwatched work blocks once before Busy can hide it,
// keys on lifecycle tags so a reused job id re-arms, and run warnings ride
// above the ladder.
func TestUnwatchedAndWarnings(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	job := JobFact{Id: "j1", MainId: "main-1", StartedAt: "2026-08-15T10:00:00Z", Status: "running"}
	busy := []Item{{Kind: "job", Id: "j1", Detail: "impl j1 [running, codex]"}}

	// Unwatched job, Busy present: the block fires DESPITE Busy.
	v, _ := s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{job}}, "s", "", "main-1")
	if !v.ShouldBlock || *v.BlockSource != "unwatched-work" || !strings.Contains(v.Display, "unwatched") {
		t.Fatalf("unwatched did not block before Busy: %+v", v)
	}
	// Same set again: reported once, no re-block; Busy shows.
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{job}}, "s", "", "main-1")
	if v.ShouldBlock || !strings.Contains(v.Display, "STILL WORKING") {
		t.Fatalf("unwatched re-blocked or Busy hidden: %+v", v)
	}
	// The SAME job id with a NEW startedAt is a new incarnation
	// and re-arms.
	reused := job
	reused.StartedAt = "2026-08-15T11:00:00Z"
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{reused}}, "s", "", "main-1")
	if !v.ShouldBlock {
		t.Fatalf("a reused job id did not re-arm: %+v", v)
	}
	// A raw waiter-liveness fact never watches a job; a foreign main's job
	// still never blocks us.
	watched := reused
	watched.WaiterLive = true
	v, _ = s.TurnVerdict(ScanResult{Jobs: []JobFact{watched}}, "s-raw-waiter", "", "main-1")
	if !v.ShouldBlock || v.BlockSource == nil || *v.BlockSource != "unwatched-work" {
		t.Fatalf("a raw waiter-liveness fact watched a job: %+v", v)
	}
	foreign := JobFact{Id: "j9", MainId: "main-other", StartedAt: "x", Status: "running"}
	v, _ = s.TurnVerdict(ScanResult{Jobs: []JobFact{foreign}}, "s-f", "", "main-1")
	if v.BlockSource != nil && *v.BlockSource == "unwatched-work" {
		t.Fatalf("a foreign job blocked as unwatched: %+v", v)
	}

	// Run warnings above the ladder: red with continuation verbatim,
	// even while Busy (mixed-state display test Busy+RunRed).
	red := RunFact{Id: "r1", MainId: "main-1", Generation: 1, Nonce: "n", Status: "red", ExpectRed: "read the log at /tmp/r1.log"}
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{watched}, Runs: []RunFact{red}}, "s", "", "main-1")
	if !strings.Contains(v.Display, "went red") || !strings.Contains(v.Display, "read the log at /tmp/r1.log") || !strings.Contains(v.Display, "STILL WORKING") {
		t.Fatalf("Busy hid the red warning or the continuation: %s", v.Display)
	}
	// Busy+RunUnreadable: both visible.
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{watched}, RunUnreadable: []string{"runs/x.json: torn"}}, "s", "", "main-1")
	if !strings.Contains(v.Display, "runs/x.json: torn") || !strings.Contains(v.Display, "STILL WORKING") {
		t.Fatalf("Busy hid the run-unreadable line: %s", v.Display)
	}
}

func TestUnwatchedRunIgnoresRawLiveWaiter(t *testing.T) {
	fixture := newPendingWaitVerdictFixture(t, "run", false)
	scan := fixture.scan
	scan.Runs = []RunFact{{
		Id: fixture.row.TargetID, MainId: pendingWaitMainID, Generation: fixture.row.Target.Generation,
		Nonce: fixture.row.Target.LaunchNonce, Status: metarun.StatusRunning, Supervised: true, WaiterLive: true,
	}}
	fixture.row.Session = "another-logical-session"
	fixture.row.RuntimeSession = "another-logical-session"
	fixture.writeRow(t)
	verdict := fixture.verdict(t, scan)
	if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" ||
		strings.Contains(verdict.Display, "WAITING: registered wait") {
		t.Fatalf("a raw live-waiter fact bypassed the session gate: %+v", verdict)
	}

	reused := newPendingWaitVerdictFixture(t, "run", false)
	reusedScan := reused.scan
	reusedScan.Runs = []RunFact{{
		Id: reused.row.TargetID, MainId: pendingWaitMainID, Generation: reused.row.Target.Generation,
		Nonce: reused.row.Target.LaunchNonce, Status: metarun.StatusRunning, Supervised: true, WaiterLive: true,
	}}
	prober := reused.store.Prober.(idleFixtureProber)
	prober[42] = identity.Exact{Pid: 42, StartedAt: time.Unix(200, 2_000)}
	reused.row.PidStartedAtMicro = time.Unix(200, 1_000).UnixMicro()
	reused.row.PidStartTicks = 0
	reused.row.BootID = ""
	reused.writeRow(t)
	verdict = reused.verdict(t, reusedScan)
	if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" ||
		strings.Contains(verdict.Display, "WAITING: registered wait") {
		t.Fatalf("a same-second reused waiter pid bypassed exact-birth authentication: %+v", verdict)
	}
}

func TestTwoConsecutiveStopsStaleRow(t *testing.T) {
	fixture := newPendingWaitVerdictFixture(t, "job", false)
	scan := fixture.scan
	scan.Jobs = []JobFact{{
		Id: fixture.row.TargetID, MainId: pendingWaitMainID, StartedAt: fixture.row.Target.StartedAt, Status: "running",
	}}
	fixture.row.Session = "stale-session"
	fixture.row.RuntimeSession = "stale-session"
	fixture.writeRow(t)

	first := fixture.verdict(t, scan)
	if !first.ShouldBlock || first.BlockSource == nil || *first.BlockSource != "unwatched-work" ||
		!strings.Contains(first.Display, "unwatched") || strings.Contains(first.Display, "WAITING: registered wait") {
		t.Fatalf("the first Stop treated a stale-session row as watched: %+v", first)
	}
	second := fixture.verdict(t, scan)
	if second.ShouldBlock || second.BlockSource != nil || !strings.Contains(second.Display, "OPEN WORK") ||
		strings.Contains(second.Display, "WAITING: registered wait") {
		t.Fatalf("the second Stop did not preserve the block-once fallback without a stale-row allowance: %+v", second)
	}

	prober := fixture.store.Prober.(idleFixtureProber)
	pid := int64(os.Getpid())
	prober[pid] = identity.Exact{Pid: pid, StartedAt: time.Unix(400, 0), StartTicks: 440, BootID: pendingWaitBootID}
	epoch := int64(7)
	ctx, cancel := context.WithCancel(context.Background())
	registered := make(chan struct{})
	result := make(chan metarun.WaitResult, 1)
	go func() {
		result <- (&metarun.Store{Root: fixture.root, Now: fixture.store.Now, Prober: prober}).Wait(ctx, metarun.WaitRequest{
			Selector: fixture.row.Selector,
			Owner: metarun.Caller{
				Class: "MAIN", MainId: pendingWaitMainID, OwnerLineage: pendingWaitLineage,
				ClaimEpoch: &epoch, SessionId: pendingWaitSession,
			},
			RuntimeSession: pendingWaitSession, Timeout: time.Hour, OpenWorkSignature: scan.OpenWorkSignature(),
		}, metarun.WaitOptions{
			Now:       fixture.store.Now,
			BootClock: func() (string, time.Duration, error) { return pendingWaitBootID, fixture.bootElapsed, nil },
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(ctx context.Context, _ metarun.WaitSelector, target metarun.WaiterTarget, _ string) (metarun.SourceObservation, error) {
				if target == (metarun.WaiterTarget{}) {
					return metarun.SourceObservation{Incarnation: fixture.row.Target, Pending: true, Outcome: "running"}, nil
				}
				close(registered)
				<-ctx.Done()
				return metarun.SourceObservation{}, ctx.Err()
			},
		})
	}()
	select {
	case <-registered:
	case early := <-result:
		cancel()
		t.Fatalf("the superseding re-watch ended before registration: %+v", early)
	}

	rewatched := fixture.verdict(t, scan)
	if rewatched.ShouldBlock || strings.Contains(rewatched.Display, "unwatched") ||
		!strings.Contains(rewatched.Display, "WAITING: registered wait") {
		cancel()
		t.Fatalf("the superseding re-watch did not count at the next Stop: %+v", rewatched)
	}
	cancel()
	if stopped := <-result; stopped.ExitCode != metarun.ExitInterrupted {
		t.Fatalf("the superseding re-watch did not stop after cancellation: %+v", stopped)
	}
}

func TestForgedAssociationRefused(t *testing.T) {
	fixture := newPendingWaitVerdictFixture(t, "job", false)
	scan := fixture.scan
	scan.Jobs = []JobFact{{
		Id: fixture.row.TargetID, MainId: pendingWaitMainID, StartedAt: fixture.row.Target.StartedAt, Status: "running",
	}}
	fixture.row.Session = "forged-session"
	fixture.row.RuntimeSession = "forged-session"
	fixture.writeRow(t)
	forged := fixture.verdict(t, scan)
	if !forged.ShouldBlock || strings.Contains(forged.Display, "WAITING: registered wait") {
		t.Fatalf("a forged runtime association received the wait allowance: %+v", forged)
	}

	fixture.row.Session = pendingWaitSession
	fixture.row.RuntimeSession = pendingWaitSession
	fixture.writeRow(t)
	accepted := fixture.verdict(t, scan)
	if accepted.ShouldBlock || !strings.Contains(accepted.Display, "WAITING: registered wait") {
		t.Fatalf("the live Stop session did not receive its own wait allowance: %+v", accepted)
	}
}

func TestHumanRunWatchedSignal(t *testing.T) {
	fixture := newPendingWaitVerdictFixture(t, "run", false)
	prober := fixture.store.Prober.(idleFixtureProber)
	registeredBirth := time.Unix(200, 1_000)
	prober[42] = identity.Exact{Pid: 42, StartedAt: registeredBirth}
	fixture.row.OwnerDigest = metarun.OwnerDigest("")
	fixture.row.MainId = ""
	fixture.row.OwnerLineage = ""
	fixture.row.ClaimEpoch = nil
	fixture.row.Session = ""
	fixture.row.RuntimeSession = ""
	fixture.row.PidStartedAt = registeredBirth.Unix()
	fixture.row.PidStartedAtMicro = registeredBirth.UnixMicro()
	fixture.row.PidStartTicks = 0
	fixture.row.BootID = ""
	fixture.writeRow(t)
	scan := ScanResult{Runs: []RunFact{{
		Id: fixture.row.TargetID, Generation: fixture.row.Target.Generation, Nonce: fixture.row.Target.LaunchNonce,
		Status: metarun.StatusRunning, Supervised: true,
	}}}
	hasWatchRun := func(verdict Verdict) bool {
		if verdict.Facts == nil {
			return false
		}
		for _, action := range verdict.Facts.Actions {
			if action.Kind == "watch-run" && action.TargetId == fixture.row.TargetID {
				return true
			}
		}
		return false
	}

	watched, err := fixture.store.TurnVerdict(scan, "human-watched", "", "")
	if err != nil || strings.Contains(watched.Display, "unwatched") || hasWatchRun(watched) {
		t.Fatalf("the exact live human waiter was not treated as watched: %+v %v", watched, err)
	}
	if err := os.Remove(metarun.WaiterPath(fixture.root, fixture.row.Kind, fixture.row.TargetID, fixture.row.OwnerDigest)); err != nil {
		t.Fatal(err)
	}
	unwatched, err := fixture.store.TurnVerdict(scan, "human-unwatched", "", "")
	if err != nil || !unwatched.ShouldBlock || unwatched.BlockSource == nil || *unwatched.BlockSource != "unwatched-work" || !hasWatchRun(unwatched) {
		t.Fatalf("an unwatched human run did not block and request recovery: %+v %v", unwatched, err)
	}

	fixture.writeRow(t)
	prober[42] = identity.Exact{Pid: 42, StartedAt: time.Unix(200, 2_000)}
	reused, err := fixture.store.TurnVerdict(scan, "human-reused-pid", "", "")
	if err != nil || !reused.ShouldBlock || reused.BlockSource == nil || *reused.BlockSource != "unwatched-work" || !hasWatchRun(reused) {
		t.Fatalf("a same-second reused human waiter pid retained watched authority: %+v %v", reused, err)
	}
}

func TestWatchedJobUsesAuthenticatedWait(t *testing.T) {
	raw := newPendingWaitVerdictFixture(t, "job", false)
	if err := os.Remove(metarun.WaiterPath(raw.root, raw.row.Kind, raw.row.TargetID, raw.row.OwnerDigest)); err != nil {
		t.Fatal(err)
	}
	rawScan := raw.scan
	rawScan.Jobs = []JobFact{{
		Id: raw.row.TargetID, MainId: pendingWaitMainID, StartedAt: raw.row.Target.StartedAt,
		Status: "running", WaiterLive: true,
	}}
	rawVerdict := raw.verdict(t, rawScan)
	if !rawVerdict.ShouldBlock || rawVerdict.BlockSource == nil || *rawVerdict.BlockSource != "unwatched-work" {
		t.Fatalf("a raw live-waiter fact counted as an authenticated job watch: %+v", rawVerdict)
	}

	authenticated := newPendingWaitVerdictFixture(t, "job", false)
	authenticatedScan := authenticated.scan
	authenticatedScan.Jobs = []JobFact{{
		Id: authenticated.row.TargetID, MainId: pendingWaitMainID, StartedAt: authenticated.row.Target.StartedAt,
		Status: "running",
	}}
	authenticatedVerdict := authenticated.verdict(t, authenticatedScan)
	if authenticatedVerdict.ShouldBlock || strings.Contains(authenticatedVerdict.Display, "unwatched") ||
		!strings.Contains(authenticatedVerdict.Display, "WAITING: registered wait") {
		t.Fatalf("the gate-accepted row did not count as the job watch: %+v", authenticatedVerdict)
	}
}

// Greens surface exactly once per session in terminal-sequence
// order, and any unreadable run record freezes the cursor so a delayed
// green is never skipped.
func TestGreenPrefixConsistency(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	greenB := RunFact{Id: "run-b", Status: "green", TerminalSeq: 11, ExpectGreen: "ship B"}
	// Turn 1: B green at seq 11, but run A's record is unreadable —
	// cursor FROZEN, nothing surfaces.
	v, _ := s.TurnVerdict(ScanResult{Runs: []RunFact{greenB}, RunUnreadable: []string{"runs/run-a.json: unreadable"}}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("green surfaced through the freeze: %s", v.Display)
	}
	// Turn 2: A recovered and concluded green at seq 10 — BOTH surface,
	// in order, once.
	greenA := RunFact{Id: "run-a", Status: "green", TerminalSeq: 10, ExpectGreen: "ship A"}
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{greenA, greenB}}, "s", "", "")
	aIdx := strings.Index(v.Display, "run-a finished green")
	bIdx := strings.Index(v.Display, "run-b finished green")
	if aIdx < 0 || bIdx < 0 || aIdx > bIdx {
		t.Fatalf("greens missing or out of order: %s", v.Display)
	}
	// Turn 3: neither resurfaces.
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{greenA, greenB}}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("a surfaced green repeated: %s", v.Display)
	}
	// A fresh session gets its own cursor.
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{greenA, greenB}}, "s2", "", "")
	if !strings.Contains(v.Display, "run-a finished green") {
		t.Fatalf("a fresh session saw no greens: %s", v.Display)
	}
}

// A HUMAN caller (empty mainId) owns human-launched runs (null
// coordinates): the unwatched rule fires for them too.
func TestHumanOwnsHumanRuns(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	humanRun := RunFact{Id: "h-run", MainId: "", Generation: 1, Nonce: "n", Status: "running"}
	v, _ := s.TurnVerdict(ScanResult{Runs: []RunFact{humanRun}}, "hs", "", "")
	if !v.ShouldBlock || *v.BlockSource != "unwatched-work" {
		t.Fatalf("a human's unwatched run did not block: %+v", v)
	}
	// A MAIN caller does NOT own the human's run.
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{humanRun}}, "hs2", "", "main-1")
	if v.BlockSource != nil && *v.BlockSource == "unwatched-work" {
		t.Fatalf("a main owned the human's run: %+v", v)
	}
}

// The green cursor trusts the DISK read inside the
// verdict flock, not the scanner's snapshot — a scan that predates a
// run's conclusion must not let the cursor advance past it, and a torn
// record on disk freezes the cursor even when the scan looked clean.
func TestGreenCursorRereadsDisk(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	writeGreen := func(id string, seq int) {
		record := `{"schemaVersion":1,"runId":"` + id + `","kind":"suite","display":"x","custody":"wrapped",` +
			`"generation":1,"pid":null,"pidStartedAt":null,"pgid":null,` +
			`"launchNonce":"` + strings.Repeat("ef", 16) + `","log":"/tmp/x.log","startedAt":"2026-08-15T10:00:00Z",` +
			`"sessionId":"s","goalId":"","staleAfterMin":30,"windDownMin":10,` +
			`"endedAt":"2026-08-15T10:05:00Z","terminalSeq":` + fmt.Sprintf("%d", seq) + `,` +
			`"evidence":{"mode":"exit-sidecar"},"expect":{"green":"ship ` + id + `","red":"","hung":"","unknown":""},` +
			`"status":"green","acked":false}`
		dir := filepath.Join(s.Root, "artifacts", "agents", "runs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeGreen("disk-run", 5)

	// The scan snapshot is STALE — it never saw disk-run — yet the green
	// surfaces because the verdict re-reads the records under its flock.
	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if !strings.Contains(v.Display, "disk-run finished green") || !strings.Contains(v.Display, "ship disk-run") {
		t.Fatalf("the stale scan hid the on-disk green: %s", v.Display)
	}
	// Once surfaced, never repeated.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("the disk green repeated: %s", v.Display)
	}

	// A torn record ON DISK freezes the cursor even when the scan carries
	// a clean green fact.
	writeGreen("late-run", 6)
	dir := filepath.Join(s.Root, "artifacts", "agents", "runs")
	if err := os.WriteFile(filepath.Join(dir, "torn.json"), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{{Id: "late-run", Status: "green", TerminalSeq: 6, ExpectGreen: "x"}}}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("green surfaced through an on-disk freeze: %s", v.Display)
	}
	if err := os.Remove(filepath.Join(dir, "torn.json")); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !strings.Contains(v.Display, "late-run finished green") {
		t.Fatalf("the frozen green never surfaced after recovery: %s", v.Display)
	}
}
