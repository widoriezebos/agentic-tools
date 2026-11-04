package steward

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

var claimQuotaClock = time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)

// claimQuotaHeld is a goal this bed's machine (m1e) claims and works on.
func claimQuotaHeld(id string) *goal.GoalFile {
	file := approvedStewardGoal(id, "Hold the machine's claim", "Build it.", "2026-08-23T00:00:00Z")
	file.State = goal.StateClaimed
	file.Revision = 3
	file.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-08-23T01:00:00Z", Revision: 3, AccountingRevision: 3}
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: "m1e", ClaimEpoch: 1}
	file.History = append(file.History, goal.HistoryLine{At: "2026-08-23T01:00:00Z",
		Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC1", "m1e", "coordinator"), Verb: "claim",
		Actor: "m1e+coordinator", Targets: []string{id}, Keep: -1})
	return file
}

// claimQuotaReady is an approved goal at the priority given, pinned as given.
func claimQuotaReady(id string, priority uint8, pinned string) *goal.GoalFile {
	file := approvedStewardGoal(id, "Fix the phone", "Build it.", "2026-08-23T00:00:00Z")
	file.Priority, file.Sequence, file.Pinned = priority, 1, pinned
	return file
}

// claimQuotaTickBed is an enrolled Git checkout whose accepted ref carries a
// converted ledger with the given goal files, read by health's production
// ledger readers.
func claimQuotaTickBed(t *testing.T, files map[string][]byte, goals ...*goal.GoalFile) string {
	t.Helper()
	root, _ := seatTickBed(t)
	seatGit(t, root, "config", "goal.sync-remote", "local")
	record := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	if files == nil {
		files = map[string][]byte{}
	}
	files["plans/goals/backlog.md"] = goal.RenderRoot(record)
	for _, file := range goals {
		files["plans/goals/"+file.Id+".md"] = goal.RenderFile(file)
	}
	for relative, data := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seatGit(t, root, "add", "-A")
	seatGit(t, root, "commit", "-qm", "converted ledger")
	seatGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root
}

// claimQuotaLane answers the lane state per held goal; main must be the
// accepted tip the projection read.
func claimQuotaLane(t *testing.T, root string, states map[string]string) func(string, string, string) (string, error) {
	tip := seatGit(t, root, "rev-parse", goal.AcceptedRef)
	return func(gotRoot, id, main string) (string, error) {
		if gotRoot != root || main != tip {
			t.Errorf("lane read root=%q main=%q, want %q at %q", gotRoot, main, root, tip)
		}
		return states[id], nil
	}
}

// claimQuotaTick runs one tick that must complete normally and returns the
// claim-quota verdict recorded by this tick's own observation.
func claimQuotaTick(t *testing.T, root string, at time.Time, lane func(string, string, string) (string, error)) (HealthVerdict, RoleVerdict) {
	t.Helper()
	return claimQuotaTickWith(t, root, TickConfig{Now: at, ClaimLaneState: lane, AskHome: t.TempDir()})
}

// claimQuotaTickWith is claimQuotaTick with the whole configuration given.
func claimQuotaTickWith(t *testing.T, root string, cfg TickConfig) (HealthVerdict, RoleVerdict) {
	t.Helper()
	at := cfg.Now
	result, err := RunTick(root, cfg, fakeCensus{})
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	evidence, err := os.ReadFile(ComponentEvidencePath(root, "steward-tick"))
	if err != nil || !strings.Contains(strings.ReplaceAll(string(evidence), " ", ""), `"outcome":"PASS_COMPLETE"`) {
		t.Fatalf("the tick did not complete normally: %s %v", evidence, err)
	}
	record, err := loadHealthRecord(HealthRecordPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if record.Verdict.Observation != result.Health.Observation || !record.Verdict.ObservedAt.Equal(at) {
		t.Fatalf("health.json holds observation %d at %s, want this tick's %d at %s",
			record.Verdict.Observation, record.Verdict.ObservedAt, result.Health.Observation, at)
	}
	return record.Verdict, claimQuotaRole(t, record.Verdict)
}

func claimQuotaRole(t *testing.T, verdict HealthVerdict) RoleVerdict {
	t.Helper()
	var found []RoleVerdict
	seen := map[HealthRole]bool{}
	for _, role := range verdict.Roles {
		if seen[role.Role] {
			t.Fatalf("role %s appears twice", role.Role)
		}
		seen[role.Role] = true
		if role.Role == RoleClaimQuota {
			found = append(found, role)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the verdict has %d claim-quota roles: %+v", len(found), verdict.Roles)
	}
	return found[0]
}

// claimQuotaBlock is the goal query's answer on the bed's accepted ledger.
func claimQuotaBlock(t *testing.T, root, ready string, states map[string]string) goal.ClaimQuotaBlock {
	t.Helper()
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, claimQuotaClock)
	if err != nil {
		t.Fatal(err)
	}
	return goal.ClaimQuotaBlocker(projection.Tree, ready, "m1e", projection.Tip, func(id, _ string) (string, error) { return states[id], nil })
}

func TestAReadyPriorityOneGoalBehindAClaimIsARowThroughTheTick(t *testing.T) {
	t.Parallel()
	landed := claimQuotaHeld("held-h")
	landed.Landing = &goal.LandingRecord{At: "2026-08-23T02:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC2", "m1e", "coordinator")}
	landed.Revision++
	landed.History = append(landed.History, goal.HistoryLine{At: "2026-08-23T02:00:00Z", Opid: landed.Landing.Opid,
		Verb: "land-ready", Actor: "m1e+coordinator", Targets: []string{"held-h"}, Keep: -1})
	sameArc, arcReady := claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")
	sameArc.Arc, arcReady.Arc = "phone", "phone"
	fenced := claimQuotaHeld("held-h")
	fenced.StopCapability.FenceEpoch = 1
	fenced.StopFence = &goal.StopFence{StopID: "stop-held-h-r3-f1", Revision: 3, Epoch: 1, CapabilityGeneration: 3,
		ClosedAt: "2026-08-23T01:02:00Z", Reason: goal.StopReasonElapsedLimit}
	handed := claimQuotaHeld("held-h")
	handed.Claimed.HandedOver = goal.HandedOver{FromMachine: "m1x", FromLineage: "coordinator", FromEpoch: 1, Batch: "batch-1"}
	questions := map[string][]byte{"artifacts/agents/channel/questions/q-held.json": []byte(
		`{"id":"q-held","goal":"held-h","kind":"decision","machine":"m1e","state":"open","openedAt":"2026-08-23T03:00:00Z"}`)}
	jobs := map[string][]byte{"artifacts/agents/jobs/job-held.json": []byte(`{"jobId":"job-held","goalId":"held-h","status":"running"}`)}
	release := "metasystem goal release held-h --reason TEXT"

	for _, row := range []struct {
		name     string
		goals    []*goal.GoalFile
		files    map[string][]byte
		states   map[string]string
		status   HealthStatus
		activity string
		command  string
	}{
		{name: "idle", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")},
			status: HealthDead, activity: "has no job and no open question", command: release},
		{name: "unpinned", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "")},
			status: HealthDead, activity: "has no job and no open question", command: release},
		{name: "running job", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")}, files: jobs,
			status: HealthDead, activity: "has a running job", command: release},
		{name: "open question", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")}, files: questions,
			status: HealthDead, activity: "waits on an open question", command: release},
		{name: "priority two", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 2, "m1e")}, status: HealthAlive},
		{name: "pinned elsewhere", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1x")}, status: HealthAlive},
		{name: "same arc", goals: []*goal.GoalFile{sameArc, arcReady}, status: HealthAlive},
		{name: "fenced", goals: []*goal.GoalFile{fenced, claimQuotaReady("ready-r", 1, "m1e")}, status: HealthAlive},
		{name: "handed over", goals: []*goal.GoalFile{handed, claimQuotaReady("ready-r", 1, "m1e")}, status: HealthAlive},
		{name: "waiting hand-in", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")},
			states: map[string]string{"held-h": "waiting"}, status: HealthAlive},
		{name: "returned", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")},
			states: map[string]string{"held-h": "returned"}, status: HealthDead, activity: "has no job and no open question", command: "metasystem work land held-h"},
		{name: "landed", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")},
			states: map[string]string{"held-h": "landed"}, status: HealthDead, activity: "has no job and no open question", command: "metasystem goal done held-h --reason TEXT"},
		{name: "two hand-ins waiting", goals: []*goal.GoalFile{landed, claimQuotaHeld("held-i"), claimQuotaReady("ready-r", 1, "m1e")},
			states: map[string]string{"held-i": "waiting"}, status: HealthDead, activity: "has no job and no open question", command: "metasystem work land held-h"},
		{name: "unreadable projection", goals: []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")},
			files: map[string][]byte{"plans/goals/broken.md": []byte("not a goal file\n")}, status: HealthUnknown},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			files := map[string][]byte{}
			for path, data := range row.files {
				files[path] = data
			}
			root := claimQuotaTickBed(t, files, row.goals...)
			tip := seatGit(t, root, "rev-parse", goal.AcceptedRef)
			ledger := map[string]string{}
			for _, file := range row.goals {
				ledger[file.Id] = seatGit(t, root, "show", tip+":plans/goals/"+file.Id+".md")
			}
			verdict, role := claimQuotaTick(t, root, claimQuotaClock, claimQuotaLane(t, root, row.states))
			if role.Status != row.status {
				t.Fatalf("claim-quota = %+v, want %s", role, row.status)
			}
			if after := seatGit(t, root, "rev-parse", goal.AcceptedRef); after != tip {
				t.Fatalf("the tick moved the accepted ledger: %s -> %s", tip, after)
			}
			for id, before := range ledger {
				if after := seatGit(t, root, "show", tip+":plans/goals/"+id+".md"); after != before {
					t.Fatalf("the tick changed goal %s:\n%s\n->\n%s", id, before, after)
				}
			}
			if head := seatGit(t, root, "rev-parse", "HEAD"); head != tip {
				t.Fatalf("the tick committed to the checkout: %s -> %s", tip, head)
			}
			preview := claimQuotaRole(t, PreviewHealth(root, claimQuotaClock.Add(time.Minute), nil))
			if preview.Status != role.Status || preview.Reason != role.Reason {
				t.Fatalf("the preview's claim-quota = %+v, want the tick's recorded %+v", preview, role)
			}
			switch row.status {
			case HealthUnknown:
				if len(role.RemedyFacts) != 1 || role.RemedyFacts[0].Cause != CauseUnreadable {
					t.Fatalf("an unreadable ledger is unknown with an unreadable cause: %+v", role)
				}
				return
			case HealthAlive:
				return
			}
			want := "ready-r (priority 1) is ready but held-h holds this machine's one claim; held-h " + row.activity +
				"; next: " + row.command + ", then metasystem goal claim ready-r"
			if role.Reason != want {
				t.Fatalf("claim-quota reason = %q, want %q", role.Reason, want)
			}
			if len(role.RemedyFacts) != 1 || role.RemedyFacts[0] != (RemedyFact{Cause: CauseClaimQuota, Goal: "held-h", Ready: "ready-r", Command: row.command}) {
				t.Fatalf("claim-quota facts = %+v", role.RemedyFacts)
			}
			if block := claimQuotaBlock(t, root, "ready-r", row.states); block.Command != role.RemedyFacts[0].Command {
				t.Fatalf("the row's command %q is not the query's %q", role.RemedyFacts[0].Command, block.Command)
			}
			if role.FailureEscalation != NoLawfulRemedy || role.Standing || role.ConsecutiveFailures != 1 || !verdict.ShouldAlert {
				t.Fatalf("a blocked priority-1 goal alerts at once: %+v alert=%t", role, verdict.ShouldAlert)
			}
			if !strings.Contains(role.Remedy, "the claiming session of goal held-h runs: "+row.command+", then metasystem goal claim ready-r") {
				t.Fatalf("claim-quota remedy = %q", role.Remedy)
			}
			episodes, err := AlertEpisodes(root)
			if err != nil {
				t.Fatal(err)
			}
			opened := false
			for _, episode := range episodes {
				opened = opened || episode.Digest == verdict.FindingDigest
			}
			if !opened {
				t.Fatalf("no alert episode for finding %s: %+v", verdict.FindingDigest, episodes)
			}
		})
	}

	t.Run("a job starting between two ticks keeps the count", func(t *testing.T) {
		t.Parallel()
		root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
		lane := claimQuotaLane(t, root, nil)
		if _, first := claimQuotaTick(t, root, claimQuotaClock, lane); first.Status != HealthDead || first.ConsecutiveFailures != 1 ||
			!strings.Contains(first.Reason, "has no job and no open question") {
			t.Fatalf("first tick: %+v", first)
		}
		if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "jobs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "jobs", "job-held.json"), jobs["artifacts/agents/jobs/job-held.json"], 0o644); err != nil {
			t.Fatal(err)
		}
		if _, second := claimQuotaTick(t, root, claimQuotaClock.Add(10*time.Minute), lane); second.Status != HealthDead ||
			second.ConsecutiveFailures != 2 || !strings.Contains(second.Reason, "has a running job") {
			t.Fatalf("second tick: %+v", second)
		}
	})
}

// claimQuotaAccept writes the goal files at the bed's accepted ref, as an
// approval or an edit that was accepted does.
func claimQuotaAccept(t *testing.T, root string, goals ...*goal.GoalFile) {
	t.Helper()
	for _, file := range goals {
		if err := os.WriteFile(filepath.Join(root, "plans", "goals", file.Id+".md"), goal.RenderFile(file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seatGit(t, root, "add", "-A")
	seatGit(t, root, "commit", "-qm", "accepted edit")
	seatGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
}

// claimQuotaAsks is every ask in a goal's mailbox on the board under home.
func claimQuotaAsks(t *testing.T, home, held string) []board.Message {
	t.Helper()
	threads, err := board.Threads(home)
	if err != nil {
		t.Fatal(err)
	}
	var asks []board.Message
	for _, thread := range threads {
		for _, message := range thread.Messages {
			if message.Kind == board.KindAsk && message.To.Goal == held {
				asks = append(asks, message)
			}
		}
	}
	return asks
}

func claimQuotaReceipts(t *testing.T, root string) []string {
	t.Helper()
	receipts, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "steward", "asked", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return receipts
}

// claimQuotaCompleted runs one tick that must end PASS_COMPLETE.
func claimQuotaCompleted(t *testing.T, root string, cfg TickConfig) TickResult {
	t.Helper()
	result, err := RunTick(root, cfg, fakeCensus{})
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	evidence, err := os.ReadFile(ComponentEvidencePath(root, "steward-tick"))
	if err != nil || !strings.Contains(strings.ReplaceAll(string(evidence), " ", ""), `"outcome":"PASS_COMPLETE"`) {
		t.Fatalf("the tick did not complete normally: %s %v", evidence, err)
	}
	return result
}

func claimQuotaAskText(ready, held, command string) string {
	return "1 ready goal at priority 1 waits for this machine's one claim, which your goal " + held +
		" holds; the first is " + ready + ". Next: " + command + ", then metasystem goal claim " + ready +
		". The claim stays until you hand in or release; a reply is not needed."
}

func TestNoReleaseIsProposedForWorkInFlight(t *testing.T) {
	t.Parallel()
	root := claimQuotaTickBed(t, map[string][]byte{"artifacts/agents/jobs/job-held.json": []byte(`{"jobId":"job-held","goalId":"held-h","status":"running"}`)},
		claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
	home, stage := t.TempDir(), "reviewed; its read is collected and published (attestation abc)"
	cfg := TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home,
		Units: func(gotRoot, id string) ([]UnitStage, error) {
			if gotRoot != root || id != "held-h" {
				t.Fatalf("units for %s in %s", id, gotRoot)
			}
			return []UnitStage{{Unit: "build", Stage: stage}, {Unit: "old", Stage: "transferred; its destinations remain required"}}, nil
		}}
	check := func(want int) {
		t.Helper()
		claimQuotaTickWith(t, root, cfg)
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != want {
			t.Fatalf("stage %q: %d asks, want %d", stage, len(asks), want)
		}
		if open, err := readClaimQuotaOccurrences(root); err != nil || len(open) != 1 {
			t.Fatalf("the blocking occurrence must stay open: %v %v", open, err)
		}
		cfg.Now = cfg.Now.Add(time.Minute)
	}
	for range 3 {
		check(0)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts/agents/jobs/job-held.json"), []byte(`{"jobId":"job-held","goalId":"held-h","status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, pending := range []string{"built, ready for review", "starting", "running", "stopped (proof-red), needs a revision", "review refused: unavailable", "committed and under independent examination", "reviewed; its read is collected but not yet published (attestation abc)"} {
		stage = pending
		check(0)
	}
	stage = "reviewed; its read is collected and published (attestation abc)"
	check(1)
	check(1)
}

func TestAnEndedOccurrenceClosesItsAsk(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"released", "handed in", "concluded", "abandoned", "released then concluded", "ready work gone", "answered"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			held, ready := claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")
			root := claimQuotaTickBed(t, nil, held, ready)
			home := t.TempDir()
			cfg := TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home}
			claimQuotaTickWith(t, root, cfg)
			ask := claimQuotaAsks(t, home, "held-h")[0]
			if err := board.Mark(ask, "m1e", "coordinator", "inbox", cfg.Now); err != nil {
				t.Fatal(err)
			}
			conclude := func() {
				held.State, held.Conclude, held.Claimed, held.StopCapability = goal.StateDone, "Finished.", nil, nil
				claimQuotaAccept(t, root, held)
			}
			reason := cause
			switch cause {
			case "released", "released then concluded":
				held.State, held.Claimed, held.StopCapability = goal.StateApproved, nil, nil
				reason = "released"
			case "concluded":
				held.State, held.Conclude, held.Claimed, held.StopCapability = goal.StateDone, "Finished.", nil, nil
			case "abandoned":
				held.State, held.Conclude, held.Claimed, held.StopCapability = goal.StateAbandoned, "", nil, nil
				held.Revision++
				held.Abandoned = &goal.AbandonRecord{By: "human:Wido", At: cfg.Now.Format(time.RFC3339), Revision: held.Revision,
					Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC2", "m1e", "coordinator"), Because: "The work is no longer needed."}
				held.History = append(held.History, goal.HistoryLine{At: held.Abandoned.At, Verb: "abandon", Actor: held.Abandoned.By,
					Opid: held.Abandoned.Opid, Targets: []string{held.Id}, Keep: -1, Reason: held.Abandoned.Because})
				reason = "concluded"
			case "handed in":
				held.Landing = &goal.LandingRecord{At: cfg.Now.Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC2", "m1e", "coordinator")}
			case "ready work gone":
				ready.State, ready.Conclude = goal.StateDone, "Finished elsewhere."
				reason = "ready work no longer waits"
			case "answered":
				composePublish(t, home, board.Request{Kind: board.KindReply, From: board.Sender{Machine: "m1e", Lineage: "coordinator"},
					To: board.Address{Machine: "m1e"}, Thread: ask.ID, Text: "no release"}, cfg.Now.Add(time.Minute))
			}
			claimQuotaAccept(t, root, held, ready)
			states := map[string]string{}
			if cause == "handed in" {
				states[held.Id] = "waiting"
			}
			check := func() {
				t.Helper()
				threads, err := board.Threads(home)
				if err != nil || len(threads) != 1 {
					t.Fatalf("ask threads: %+v %v", threads, err)
				}
				thread := threads[0]
				for _, message := range thread.Messages {
					if message.To.Goal == held.Id && message.ID != ask.ID {
						t.Errorf("ending the occurrence sent a new goal message: %s", message.PeerText())
					}
				}
				if cause == "answered" {
					if state, _ := thread.State(cfg.Now); state != board.ThreadReplied || len(thread.Messages) != 2 || thread.Messages[1].PeerText() != "no release" {
						t.Fatalf("the holder's answer was replaced: %+v", thread)
					}
					return
				}
				if open, err := readClaimQuotaOccurrences(root); err != nil || len(open) != 0 {
					t.Fatalf("the occurrence did not end: %v %v", open, err)
				}
				records, err := seatJSONLines(claimQuotaOccurrencePath(root))
				if err != nil || len(records) != 2 || records[1]["end"] != claimQuotaClock.Add(time.Minute).Format(time.RFC3339Nano) {
					t.Fatalf("the occurrence's end record: %+v %v", records, err)
				}
				src := composeSources(t, root, home, &cfg.Now)
				src.ClaimLaneState = cfg.ClaimLaneState
				page := Compose(readSituation(t, root, src))
				if page.First.Kind == "ask" {
					t.Errorf("the ended ask remains the holder's first thing: %+v", page.First)
				}
				var program []string
				for _, part := range page.Parts {
					if part.Name == "program" {
						program = append(program, part.Summary...)
					}
				}
				if !slices.Contains(program, "the occurrence ended: held-h "+reason) {
					t.Errorf("the program does not name the end and its reason: %q", program)
				}
			}
			for i := 1; i <= 6; i++ {
				cfg.Now = claimQuotaClock.Add(time.Duration(i) * time.Minute)
				cfg.ClaimLaneState = claimQuotaLane(t, root, states)
				if _, role := claimQuotaTickWith(t, root, cfg); cause != "answered" && role.Status != HealthAlive {
					t.Fatalf("the ended occurrence's quota verdict: %+v", role)
				}
				check()
				if cause == "released then concluded" && i == 1 {
					conclude()
					reason = "concluded"
				}
			}
			if cause == "answered" {
				return
			}
			if held.State != goal.StateAbandoned && held.State != goal.StateDone {
				conclude()
			}
			owners := func() (board.Ownership, error) {
				return board.Ownership{Concluded: map[string]string{held.Id: string(held.State)}}, nil
			}
			// The first read closes goal messages; the second sees any resulting seat note.
			for range 2 {
				inbox, err := board.Pending(home, "m1e", owners, cfg.Now)
				if err != nil {
					t.Fatal(err)
				}
				inbox.Release()
				for _, message := range inbox.Messages {
					if strings.Contains(board.Render(message, cfg.Now, time.UTC), "was not delivered") {
						t.Errorf("the seat received a failed-delivery note: %s", message.PeerText())
					}
				}
			}
		})
	}
}

func TestTheHolderIsAskedOncePerBlockingOccurrence(t *testing.T) {
	t.Parallel()
	release := "metasystem goal release held-h --reason TEXT"

	t.Run("one ask per occurrence, kept past the board's sweep", func(t *testing.T) {
		t.Parallel()
		goals := []*goal.GoalFile{claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e")}
		for i := 9; i > 0; i-- {
			ready := claimQuotaReady(fmt.Sprintf("ready-%d", i), 1, "m1e")
			ready.Sequence = uint64(i + 1)
			goals = append(goals, ready)
		}
		root := claimQuotaTickBed(t, nil, goals...)
		home := filepath.Join(t.TempDir(), ".metasystem")
		lane := claimQuotaLane(t, root, nil)
		cfg := TickConfig{Now: claimQuotaClock, ClaimLaneState: lane, AskHome: home}
		if _, role := claimQuotaTickWith(t, root, cfg); role.Status != HealthDead {
			t.Fatalf("the first tick's claim-quota = %+v", role)
		}
		asks := claimQuotaAsks(t, home, "held-h")
		want := strings.Replace(claimQuotaAskText("ready-r", "held-h", release), "1 ready goal at priority 1 waits", "10 ready goals at priority 1 wait", 1)
		if len(asks) != 1 || asks[0].PeerText() != want || asks[0].From.Machine != "m1e" {
			t.Fatalf("want one ask naming ten goals and the first in program order: %+v", asks)
		}
		for i := 1; i < 6; i++ {
			cfg.Now = claimQuotaClock.Add(time.Duration(i) * time.Minute)
			claimQuotaTickWith(t, root, cfg)
		}
		if records, err := seatJSONLines(filepath.Join(root, "artifacts/agents/steward/claim-quota.jsonl")); err != nil || len(records) != 1 || records[0]["start"] == nil {
			t.Fatalf("want one occurrence start: %+v %v", records, err)
		}
		if receipts := claimQuotaReceipts(t, root); len(receipts) != 1 {
			t.Fatalf("the steward kept %d receipts, want 1: %v", len(receipts), receipts)
		}
		cfg.Now = claimQuotaClock.Add(10 * time.Minute)
		claimQuotaTickWith(t, root, cfg)
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 1 {
			t.Fatalf("a second tick asked again: %d asks", len(asks))
		}

		// The holder reads and answers; the bridge sweeps the answered thread.
		replyAt := claimQuotaClock.Add(20 * time.Minute)
		if _, err := board.Publish(home, board.Request{Kind: board.KindReply, From: board.Sender{Machine: "m1e", Lineage: "coordinator"},
			To: board.Address{Machine: asks[0].From.Machine}, Thread: asks[0].ID, Text: "releasing it now"}, replyAt); err != nil {
			t.Fatal(err)
		}
		threads, err := board.Threads(home)
		if err != nil {
			t.Fatal(err)
		}
		for _, thread := range threads {
			for _, message := range thread.Messages {
				if err := board.Mark(message, "m1e", "coordinator", "inbox", replyAt); err != nil {
					t.Fatal(err)
				}
			}
		}
		bridge := &board.Bridge{Home: home, MailboxKeep: time.Hour, Now: func() time.Time { return replyAt.Add(2 * time.Hour) }}
		if removed := bridge.SweepMessages(); len(removed) != 2 {
			t.Fatalf("the bridge swept %v, want the ask and its reply", removed)
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 0 {
			t.Fatalf("the sweep left %d asks", len(asks))
		}
		cfg.Now = claimQuotaClock.Add(3 * time.Hour)
		if _, role := claimQuotaTickWith(t, root, cfg); role.Status != HealthDead {
			t.Fatalf("the pair is still blocked: %+v", role)
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 0 {
			t.Fatalf("the swept pair was asked again: %+v", asks)
		}

		// More ready work belongs to the same blocking interval.
		second := claimQuotaReady("ready-s", 1, "m1e")
		second.Sequence = 2
		claimQuotaAccept(t, root, second)
		cfg.Now, cfg.ClaimLaneState = claimQuotaClock.Add(4*time.Hour), claimQuotaLane(t, root, nil)
		claimQuotaTickWith(t, root, cfg)
		asks = claimQuotaAsks(t, home, "held-h")
		if len(asks) != 0 {
			t.Fatalf("more ready work asked the same holder again: %+v", asks)
		}
		if receipts := claimQuotaReceipts(t, root); len(receipts) != 1 {
			t.Fatalf("the steward kept %d receipts, want 1: %v", len(receipts), receipts)
		}
		held := claimQuotaHeld("held-h")
		held.State, held.Conclude, held.Claimed, held.StopCapability = goal.StateDone, "Finished.", nil, nil
		claimQuotaAccept(t, root, held)
		cfg.Now, cfg.ClaimLaneState = cfg.Now.Add(time.Minute), claimQuotaLane(t, root, nil)
		claimQuotaTickWith(t, root, cfg)
		if records, err := seatJSONLines(filepath.Join(root, "artifacts/agents/steward/claim-quota.jsonl")); err != nil || len(records) != 2 || records[1]["end"] == nil {
			t.Fatalf("conclusion did not end the occurrence: %+v %v", records, err)
		}
		claimQuotaAccept(t, root, claimQuotaHeld("held-k"))
		cfg.Now, cfg.ClaimLaneState = cfg.Now.Add(time.Minute), claimQuotaLane(t, root, nil)
		claimQuotaTickWith(t, root, cfg)
		if asks := claimQuotaAsks(t, home, "held-k"); len(asks) != 1 {
			t.Fatalf("new holder: %+v", asks)
		}
		threads, err = board.Threads(home)
		if err != nil {
			t.Fatal(err)
		}
		for _, thread := range threads {
			for _, message := range thread.Messages {
				if message.To.Machine != "" {
					t.Fatalf("unexpected note to the asker: %+v", message)
				}
			}
		}

	})

	t.Run("the current ledger overrules stale facts", func(t *testing.T) {
		t.Parallel()
		for _, state := range []string{"released", "concluded", "another machine"} {
			root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
			home := t.TempDir()
			cfg := TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home}
			health, _ := claimQuotaTickWith(t, root, cfg)
			held := claimQuotaHeld("held-h")
			switch state {
			case "released":
				held.State, held.Claimed, held.StopCapability = goal.StateApproved, nil, nil
			case "concluded":
				held.State, held.Conclude, held.Claimed, held.StopCapability = goal.StateDone, "Finished.", nil, nil
			case "another machine":
				held.Claimed.Machine, held.StopCapability.Machine = "m1x", "m1x"
			}
			claimQuotaAccept(t, root, held)
			if err := tickHealthAsk(cfg)(root, health, cfg.Now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 1 {
				t.Fatalf("%s: stale facts asked: %+v", state, asks)
			}
			if records, err := seatJSONLines(filepath.Join(root, "artifacts/agents/steward/claim-quota.jsonl")); err != nil || len(records) != 2 || records[1]["end"] == nil {
				t.Fatalf("%s: stale facts kept the occurrence: %+v %v", state, records, err)
			}
		}
	})

	t.Run("a stopped health asks nothing", func(t *testing.T) {
		t.Parallel()
		root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
		closeProcessFence(t, root, 4)
		home := filepath.Join(t.TempDir(), ".metasystem")
		result := claimQuotaCompleted(t, root, TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home})
		if !result.Health.Stopped || claimQuotaRole(t, result.Health).Status != HealthDead {
			t.Fatalf("the fenced tick's health = %+v", result.Health)
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 0 || len(claimQuotaReceipts(t, root)) != 0 {
			t.Fatalf("a stopped health asked: %+v", asks)
		}
	})

	t.Run("a seat with no machine enrolled reads none", func(t *testing.T) {
		t.Parallel()
		root := gitRepoWithCurrentGoal(t)
		home := filepath.Join(t.TempDir(), ".metasystem")
		claimQuotaCompleted(t, root, TickConfig{Now: claimQuotaClock, ClaimLaneState: func(string, string, string) (string, error) {
			t.Error("a bootstrap ledger read the lane")
			return "", nil
		}, AskHome: home})
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Fatalf("the tick touched the board: %v", err)
		}
	})
}

// gitFails is a git that answers nothing: the tick cadence is its default.
func gitFails(...string) (string, error) { return "", errors.New("no git") }

func TestTheHelmSeesTheClaimQuotaRowAndAsksOnce(t *testing.T) {
	t.Parallel()
	release := "metasystem goal release held-h --reason TEXT"
	// warmBed is the bed after one full tick with the ready goal at
	// priority 2, at the helm, and the ready goal then raised to priority 1.
	warmBed := func(t *testing.T) (root, home string, baseline healthRecord) {
		t.Helper()
		root = claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 2, "m1e"))
		home = filepath.Join(t.TempDir(), ".metasystem")
		if _, role := claimQuotaTickWith(t, root, TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home}); role.Status != HealthAlive {
			t.Fatalf("the baseline's claim-quota = %+v", role)
		}
		baseline, err := loadHealthRecord(HealthRecordPath(root))
		if err != nil {
			t.Fatal(err)
		}
		takeHelmFixture(t, root)
		claimQuotaAccept(t, root, claimQuotaReady("ready-r", 1, "m1e"))
		return root, home, baseline
	}
	helmTickAt := func(t *testing.T, root, home string, at time.Time) TickResult {
		t.Helper()
		result, err := RunTick(root, TickConfig{Now: at, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home}, fakeCensus{})
		if err != nil {
			t.Fatalf("helm tick: %v", err)
		}
		if result.Decision.Verdict != VerdictHelm || result.Decision.Action != ActNone {
			t.Fatalf("the helm tick decided %+v", result.Decision)
		}
		return result
	}
	readFile := func(t *testing.T, path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return data
	}
	// otherRoles is the record without the roles the helm observes, and
	// without their counters.
	helmRoles := []HealthRole{RoleClaimQuota, "steward-duty:disk-sweep", "steward-duty:tick-timing", "steward-duty:question-replies", "steward-duty:situation"}
	otherRoles := func(record healthRecord) ([]RoleVerdict, HealthObservationState) {
		var roles []RoleVerdict
		for _, role := range record.Verdict.Roles {
			if !slices.Contains(helmRoles, role.Role) {
				roles = append(roles, role)
			}
		}
		state := HealthObservationState{UnknownCounts: map[HealthRole]int{}, FailureCounts: map[HealthRole]int{},
			FailureCauses: map[HealthRole]string{}, FailureEpisodes: map[HealthRole][]time.Time{}}
		for role, count := range record.State.UnknownCounts {
			if !slices.Contains(helmRoles, role) && count != 0 {
				state.UnknownCounts[role] = count
			}
		}
		for role, count := range record.State.FailureCounts {
			if !slices.Contains(helmRoles, role) && count != 0 {
				state.FailureCounts[role] = count
			}
		}
		for role, cause := range record.State.FailureCauses {
			if !slices.Contains(helmRoles, role) {
				state.FailureCauses[role] = cause
			}
		}
		for role, episodes := range record.State.FailureEpisodes {
			if !slices.Contains(helmRoles, role) && len(episodes) != 0 {
				state.FailureEpisodes[role] = episodes
			}
		}
		return roles, state
	}

	t.Run("a warm record", func(t *testing.T) {
		t.Parallel()
		root, home, baseline := warmBed(t)
		narrator := readFile(t, ComponentEvidencePath(root, "narrator"))
		evidence := readFile(t, EvidencePath(root))
		if len(narrator) == 0 || len(evidence) == 0 {
			t.Fatalf("the baseline wrote no narrator record or evidence store")
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 0 {
			t.Fatalf("the baseline asked: %+v", asks)
		}
		baseRoles, baseState := otherRoles(baseline)
		first := claimQuotaClock.Add(time.Hour)
		helmTickAt(t, root, home, first)
		record, err := loadHealthRecord(HealthRecordPath(root))
		if err != nil {
			t.Fatalf("the helm's record does not reload: %v", err)
		}
		role := claimQuotaRole(t, record.Verdict)
		if role.Status != HealthDead || record.Verdict.Observation != baseline.Verdict.Observation+1 ||
			!strings.Contains(role.Reason, "ready-r (priority 1) is ready but held-h holds this machine's one claim") {
			t.Fatalf("the helm's claim-quota = %+v at observation %d", role, record.Verdict.Observation)
		}
		if !record.Verdict.Partial || !record.Verdict.FullObservedAt.Equal(baseline.Verdict.ObservedAt) {
			t.Fatalf("the helm's record is partial=%t full=%s, want the baseline's %s", record.Verdict.Partial,
				record.Verdict.FullObservedAt, baseline.Verdict.ObservedAt)
		}
		if roles, state := otherRoles(record); !reflect.DeepEqual(roles, baseRoles) || !reflect.DeepEqual(state, baseState) {
			t.Fatalf("the helm changed other roles:\n%+v\n%+v\nwant\n%+v\n%+v", roles, state, baseRoles, baseState)
		}
		asks := claimQuotaAsks(t, home, "held-h")
		if len(asks) != 1 || asks[0].PeerText() != claimQuotaAskText("ready-r", "held-h", release) {
			t.Fatalf("the helm sent %d asks: %+v", len(asks), asks)
		}
		if !bytes.Equal(readFile(t, ComponentEvidencePath(root, "narrator")), narrator) || !bytes.Equal(readFile(t, EvidencePath(root)), evidence) {
			t.Fatal("the helm tick narrated or wrote the evidence store")
		}
		if _, fresh := FreshHookHealthPreviewAt(root, root, first.Add(time.Minute), gitFails); fresh {
			t.Fatal("a hook reads the helm's partial record as fresh")
		}
		preview := PreviewHealth(root, first.Add(time.Minute), nil)
		if got := claimQuotaRole(t, preview); got.Status != HealthDead || got.Reason != role.Reason {
			t.Fatalf("the preview's claim-quota = %+v, want the recorded %+v", got, role)
		}

		helmTickAt(t, root, home, claimQuotaClock.Add(2*time.Hour))
		record, err = loadHealthRecord(HealthRecordPath(root))
		if err != nil {
			t.Fatalf("the second helm record does not reload: %v", err)
		}
		if got := claimQuotaRole(t, record.Verdict); got.ConsecutiveFailures != 2 || record.State.FailureCounts[RoleClaimQuota] != 2 ||
			!record.Verdict.FullObservedAt.Equal(baseline.Verdict.ObservedAt) {
			t.Fatalf("the second helm pass = %+v, full %s", got, record.Verdict.FullObservedAt)
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 1 {
			t.Fatalf("the second helm pass asked again: %d asks", len(asks))
		}

		if _, err := helm.Remove(root); err != nil {
			t.Fatal(err)
		}
		full, role := claimQuotaTickWith(t, root, TickConfig{Now: claimQuotaClock.Add(3 * time.Hour), ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home})
		if full.Partial || !full.FullObservedAt.IsZero() || role.ConsecutiveFailures != 3 {
			t.Fatalf("the full tick after the helm = partial %t full %s, claim-quota %+v", full.Partial, full.FullObservedAt, role)
		}
		for _, other := range full.Roles {
			if strings.Contains(other.Reason, "the prior health observation state was unreadable") {
				t.Fatalf("the full tick found the helm's record unreadable: %+v", other)
			}
		}
	})

	t.Run("a cold record under the helm", func(t *testing.T) {
		t.Parallel()
		root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
		home := filepath.Join(t.TempDir(), ".metasystem")
		seeded := HealthVerdict{Roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Standing: true,
			Reason: "the runner is gone", Remedy: "arm the steward"}}}
		if err := fileStandingDefects(root, seeded, healthRoleOrder, claimQuotaClock, noticeFunnel{clock: func() time.Time { return claimQuotaClock }},
			func(string, string) error { return nil }); err != nil {
			t.Fatal(err)
		}
		standingOpen := func() bool {
			episodes, err := AlertEpisodes(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, episode := range episodes {
				if episode.Owner == PatternOwner("health-standing-red") && episode.ScopeID == string(RoleStewardRunner) && !episode.Cleared {
					return true
				}
			}
			return false
		}
		if !standingOpen() {
			t.Fatal("the seeded standing episode is not open")
		}
		takeHelmFixture(t, root)
		for tick := 1; tick <= 3; tick++ {
			helmTickAt(t, root, home, claimQuotaClock.Add(time.Duration(tick)*time.Hour))
		}
		record, err := loadHealthRecord(HealthRecordPath(root))
		if err != nil {
			t.Fatal(err)
		}
		var cold []HealthRole
		for _, role := range record.Verdict.Roles {
			cold = append(cold, role.Role)
		}
		if !reflect.DeepEqual(cold, helmRoles) || record.Verdict.Roles[0].Status != HealthDead ||
			!record.Verdict.Partial || !record.Verdict.FullObservedAt.IsZero() {
			t.Fatalf("the cold helm record = %+v", record.Verdict)
		}
		for _, path := range []string{ComponentEvidencePath(root, "narrator"), EvidencePath(root)} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("the helm wrote %s: %v", path, err)
			}
		}
		if !standingOpen() {
			t.Fatal("the helm cleared a standing episode it did not observe")
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 1 {
			t.Fatalf("the cold helm sent %d asks", len(asks))
		}
	})

	t.Run("the fence closed under the helm", func(t *testing.T) {
		t.Parallel()
		root, home, _ := warmBed(t)
		closeProcessFence(t, root, 4)
		record := readFile(t, HealthRecordPath(root))
		episodes, err := AlertEpisodes(root)
		if err != nil {
			t.Fatal(err)
		}
		helmTickAt(t, root, home, claimQuotaClock.Add(time.Hour))
		if !bytes.Equal(readFile(t, HealthRecordPath(root)), record) {
			t.Fatal("the fenced helm advanced the health record")
		}
		if after, err := AlertEpisodes(root); err != nil || !reflect.DeepEqual(after, episodes) {
			t.Fatalf("the fenced helm changed the alert episodes: %v\n%+v\nwant\n%+v", err, after, episodes)
		}
		if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 0 {
			t.Fatalf("the fenced helm asked: %+v", asks)
		}
	})
}

func TestTheHelmWithNoLaneReaderObservesNoClaimQuota(t *testing.T) {
	t.Parallel()
	root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
	home := filepath.Join(t.TempDir(), ".metasystem")
	ledger := newHealthLedger(root, claimQuotaClock)
	ledger.laneState = claimQuotaLane(t, root, nil)
	seeded, err := observePartialHealth(root, claimQuotaClock, []RoleVerdict{checkClaimQuotaWith(root, root, ledger)})
	if err != nil {
		t.Fatal(err)
	}
	seededRole := claimQuotaRole(t, seeded)
	if seededRole.Status != HealthDead || len(claimQuotaReceipts(t, root)) != 0 {
		t.Fatalf("the seeded claim-quota = %+v with receipts %v", seededRole, claimQuotaReceipts(t, root))
	}
	takeHelmFixture(t, root)
	result, err := RunTick(root, TickConfig{Now: claimQuotaClock.Add(time.Hour), AskHome: home}, fakeCensus{})
	if err != nil || result.Decision.Verdict != VerdictHelm {
		t.Fatalf("helm tick: %+v %v", result.Decision, err)
	}
	record, err := loadHealthRecord(HealthRecordPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := claimQuotaRole(t, record.Verdict); !reflect.DeepEqual(got, seededRole) ||
		record.State.FailureCounts[RoleClaimQuota] != seeded.State.FailureCounts[RoleClaimQuota] {
		t.Fatalf("the helm with no lane reader observed the claim quota: %+v count %d, want %+v count %d", got,
			record.State.FailureCounts[RoleClaimQuota], seededRole, seeded.State.FailureCounts[RoleClaimQuota])
	}
	var roles []HealthRole
	for _, role := range record.Verdict.Roles {
		roles = append(roles, role.Role)
	}
	want := []HealthRole{RoleClaimQuota, "steward-duty:disk-sweep", "steward-duty:tick-timing", "steward-duty:question-replies", "steward-duty:situation"}
	if !reflect.DeepEqual(roles, want) || record.Verdict.Observation != seeded.Observation+1 {
		t.Fatalf("the helm observed %v at observation %d, want %v at %d", roles, record.Verdict.Observation, want, seeded.Observation+1)
	}
	if asks := claimQuotaAsks(t, home, "held-h"); len(asks) != 0 || len(claimQuotaReceipts(t, root)) != 0 {
		t.Fatalf("the helm with no lane reader asked: %+v, receipts %v", asks, claimQuotaReceipts(t, root))
	}
}

func TestTheDeferredHealthCarriesTheAsk(t *testing.T) {
	t.Parallel()
	root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
	home := filepath.Join(t.TempDir(), ".metasystem")
	path := EvidencePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := RunTick(root, TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home}, fakeCensus{})
	if err != nil || result.Decision.Verdict != VerdictDegraded {
		t.Fatalf("the torn store's tick: %+v %v", result.Decision, err)
	}
	record, err := loadHealthRecord(HealthRecordPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if role := claimQuotaRole(t, record.Verdict); role.Status != HealthDead || !record.Verdict.ObservedAt.Equal(claimQuotaClock) {
		t.Fatalf("the deferred health = %+v at %s", role, record.Verdict.ObservedAt)
	}
	asks := claimQuotaAsks(t, home, "held-h")
	if len(asks) != 1 || asks[0].PeerText() != claimQuotaAskText("ready-r", "held-h", "metasystem goal release held-h --reason TEXT") {
		t.Fatalf("the deferred health sent %d asks: %+v", len(asks), asks)
	}
}

func TestABlockingOccurrenceIsTheObservedInterval(t *testing.T) {
	t.Parallel()
	held := claimQuotaHeld("held-h")
	root := claimQuotaTickBed(t, nil, held, claimQuotaReady("ready-r", 1, "m1e"))
	home := t.TempDir()
	cfg := TickConfig{Now: claimQuotaClock, ClaimLaneState: claimQuotaLane(t, root, nil), AskHome: home}
	claimQuotaTickWith(t, root, cfg)
	path := filepath.Join(root, "artifacts/agents/steward/claim-quota.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Now = cfg.Now.Add(time.Minute)
	cfg.ClaimLaneState = func(string, string, string) (string, error) { return "", errors.New("lane unreadable") }
	if _, role := claimQuotaTickWith(t, root, cfg); role.Status != HealthUnknown {
		t.Fatalf("unreadable lane: %+v", role)
	}
	cfg.ClaimLaneState = claimQuotaLane(t, root, nil)
	closeProcessFence(t, root, 4)
	cfg.Now = cfg.Now.Add(time.Minute)
	claimQuotaCompleted(t, root, cfg)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || len(claimQuotaAsks(t, home, "held-h")) != 1 {
		t.Fatalf("unknown or stopped observation changed the occurrence: %s %v", after, err)
	}
	if err := stopfence.Write(root, stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 5}); err != nil {
		t.Fatal(err)
	}
	cfg.Now = cfg.Now.Add(time.Minute)
	claimQuotaTickWith(t, root, cfg)
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("recovery restarted the interval: %s %v", after, err)
	}
	for i, state := range []string{"waiting", "returned", "returned"} {
		if i == 2 {
			held.Claimed.EpisodeAt, held.Claimed.EpisodeRevision = held.Claimed.At, held.Claimed.Revision
			held.Revision++
			held.Claimed.Revision++
			held.Claimed.AccountingRevision = held.Revision
			held.Claimed.At = cfg.Now.Add(time.Minute).UTC().Format(time.RFC3339Nano)
			held.History = append(held.History, goal.HistoryLine{At: held.Claimed.At, Verb: "set-budget", Actor: "m1e+coordinator", Targets: []string{held.Id}, Keep: -1,
				Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC3", "m1e", "coordinator")})
			held.StopCapability.Revision++
			held.StopCapability.Generation++
			claimQuotaAccept(t, root, held)
		}
		cfg.Now = cfg.Now.Add(time.Minute)
		cfg.ClaimLaneState = claimQuotaLane(t, root, map[string]string{"held-h": state})
		health, role := claimQuotaTickWith(t, root, cfg)
		want := HealthDead
		if state == "waiting" {
			want = HealthAlive
		}
		if health.Stopped || role.Status != want {
			t.Fatalf("%s after rebinding=%t: stopped=%t role=%+v", state, i == 2, health.Stopped, role)
		}
	}
	records, err := seatJSONLines(path)
	if err != nil || len(records) != 3 || records[1]["end"] == nil || records[2]["start"] == nil || records[0]["start"] == records[2]["start"] || len(claimQuotaAsks(t, home, "held-h")) != 2 {
		t.Fatalf("waiting, returned and rebinding: %+v %v", records, err)
	}
}
