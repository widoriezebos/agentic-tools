package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	result, err := RunTick(root, TickConfig{Now: at, ClaimLaneState: lane}, fakeCensus{})
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
