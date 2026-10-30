package steward

import (
	"bytes"
	"errors"
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
	return "Goal " + ready + " is ready at priority 1 and waits for this machine's one claim, which your goal " + held +
		" holds. Next: " + command + ", then metasystem goal claim " + ready + "."
}

func TestTheHolderIsAskedOncePerPairThroughTheTick(t *testing.T) {
	t.Parallel()
	release := "metasystem goal release held-h --reason TEXT"

	t.Run("one ask per pair, kept past the board's sweep", func(t *testing.T) {
		t.Parallel()
		root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
		home := filepath.Join(t.TempDir(), ".metasystem")
		lane := claimQuotaLane(t, root, nil)
		cfg := TickConfig{Now: claimQuotaClock, ClaimLaneState: lane, AskHome: home}
		if _, role := claimQuotaTickWith(t, root, cfg); role.Status != HealthDead {
			t.Fatalf("the first tick's claim-quota = %+v", role)
		}
		asks := claimQuotaAsks(t, home, "held-h")
		if len(asks) != 1 || asks[0].PeerText() != claimQuotaAskText("ready-r", "held-h", release) || asks[0].From.Machine != "m1e" {
			t.Fatalf("the holder's mailbox holds %d asks, want one naming the ready goal, the claim and the release: %+v", len(asks), asks)
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

		// A second ready priority-1 goal behind the same claim is a new pair.
		second := claimQuotaReady("ready-s", 1, "m1e")
		second.Sequence = 2
		claimQuotaAccept(t, root, second)
		cfg.Now, cfg.ClaimLaneState = claimQuotaClock.Add(4*time.Hour), claimQuotaLane(t, root, nil)
		claimQuotaTickWith(t, root, cfg)
		asks = claimQuotaAsks(t, home, "held-h")
		if len(asks) != 1 || asks[0].PeerText() != claimQuotaAskText("ready-s", "held-h", release) {
			t.Fatalf("the new pair got %d asks, want one for ready-s: %+v", len(asks), asks)
		}
		if receipts := claimQuotaReceipts(t, root); len(receipts) != 2 {
			t.Fatalf("the steward kept %d receipts, want 2: %v", len(receipts), receipts)
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
	helmRoles := []HealthRole{RoleClaimQuota, "steward-duty:disk-sweep", "steward-duty:tick-timing", "steward-duty:question-replies"}
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
	want := []HealthRole{RoleClaimQuota, "steward-duty:disk-sweep", "steward-duty:tick-timing", "steward-duty:question-replies"}
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
