package main

// Idempotency rows for the question, incident, mission, receipt and
// experiment objects (R-129-ui, unit U-idem). Each stateful witness runs its
// action twice and proves the second run is success that recorded nothing.

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
)

func init() {
	registerIdempotency("question ask", idemStateful, "the exact open question again (same goal, kind, text, options, recommendation, token and budget) is found at the channel owner and nothing is asked again; any other parameter is a new question", witnessQuestionAsk)
	registerIdempotency("question retry", idemStateful, "a delivered question is not sent again", witnessQuestionRetry)
	registerIdempotency("question withdraw", idemStateful, "a closed question is not closed, posted or written again", witnessQuestionWithdraw)
	registerIdempotency("question answer", idemStateful, "the recorded answer again records nothing; a different answer is refused", witnessQuestionAnswer)
	registerIdempotency("question show", idemRead, "", nil)
	registerIdempotency("question list", idemRead, "", nil)
	registerIdempotency("question wait", idemRead, "", nil)

	registerIdempotency("incident list", idemRead, "", nil)
	registerIdempotency("incident claim", idemStateful, "the same owner and fix goal already recorded publishes nothing; another owner or goal is a change", witnessIncidentClaim)
	registerIdempotency("incident close", idemStateful, "a closed incident publishes nothing", witnessIncidentClose)

	registerIdempotency("mission start", idemStateful, "a mission whose runner is live is already running; nothing is armed or spawned", witnessMissionLaunch("start"))
	registerIdempotency("mission status", idemRead, "", nil)
	registerIdempotency("mission resume", idemStateful, "a mission whose runner is live is already running; nothing is armed or spawned", witnessMissionLaunch("resume"))
	registerIdempotency("mission repair", idemStateful, "the recorded resolution again writes and anchors nothing; another resolution is refused", witnessMissionRepair)

	registerIdempotency("receipt add", idemCreation, "each receipt records one completed task, so a second call is another task's record; an identical correction that already stands appends nothing at the receipt owner", nil)
	registerIdempotency("receipt status", idemRead, "", nil)
	registerIdempotency("receipt retro", idemCreation, "each call records that one retro ran and resets the cadence from that moment; a second retro is a second event", nil)

	registerIdempotency("experiment record", idemCreation, "each record is one measurement at one commit; the exact entry already standing at the same commit is a no-op at the frontier owner (internal/report TestFrontierRecordRepeatIsUnchanged)", nil)
	registerIdempotency("experiment challenge", idemRead, "", nil)
	registerIdempotency("experiment status", idemRead, "", nil)
	registerIdempotency("experiment check", idemRead, "", nil)
}

// questionFiles is every stored channel question, by file name and bytes.
func questionFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	entries, _ := os.ReadDir(filepath.Join(root, "artifacts", "agents", "channel", "questions"))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "channel", "questions", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files
}

func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for name, data := range a {
		if b[name] != data {
			return false
		}
	}
	return true
}

// channelQuestionOwners points the question commands at the real channel
// owner with a fake provider.
func channelQuestionOwners(b *processBed, provider *questionProvider) intentOwners {
	owners := b.owners()
	owners.processes.question = channel.ReadQuestion
	owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
		return provider, channel.DestinationConfig{}
	}
	owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
		return askChannelQuestionVia(root, in, channelAskSurface{
			identity: func(string) (string, string, error) { return "mac-cli", "", nil },
			load:     func(string) (phase.Loaded, error) { return phase.Loaded{Provider: provider}, nil },
			cursor:   func(string) (string, bool, error) { return "", false, nil },
		})
	}
	return owners
}

func witnessQuestionAsk(t *testing.T) {
	b := newProcessBed(t)
	provider := &questionProvider{}
	owners := channelQuestionOwners(b, provider)
	ask := []string{"question", "ask", bedGoal, "--question", "Land slice 2 now?", "--option", "yes: land it", "--option", "no: wait", "--recommend", "yes"}
	if code, result := b.runJSON(owners, ask...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first ask = %d %+v", code, result)
	}
	before, publications := questionFiles(t, b.root()), b.publications()
	code, result := b.runJSON(owners, ask...)
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already open") {
		t.Fatalf("repeat ask = %d %+v", code, result)
	}
	if len(before) != 1 || !sameFiles(before, questionFiles(t, b.root())) || len(provider.posts) != 1 || b.publications() != publications {
		t.Fatalf("a repeated ask recorded something: %d questions, %d posts, %d publications (was %d)", len(questionFiles(t, b.root())), len(provider.posts), b.publications(), publications)
	}
}

func witnessQuestionRetry(t *testing.T) {
	b := newProcessBed(t)
	provider := &questionProvider{}
	owners := channelQuestionOwners(b, provider)
	writeQuestionFixture(t, filepath.Join(b.root(), "artifacts", "agents", "channel", "questions", "q1.json"),
		map[string]any{"id": "q1", "goal": bedGoal, "kind": "other", "state": "open", "facts": []string{"Land it?"}, "undelivered": 1})
	if code, result := b.runJSON(owners, "question", "retry", "q1"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first retry = %d %+v", code, result)
	}
	before := questionFiles(t, b.root())
	code, result := b.runJSON(owners, "question", "retry", "q1")
	if code != 0 || result.Outcome != intentUnchanged || len(provider.posts) != 1 || !sameFiles(before, questionFiles(t, b.root())) {
		t.Fatalf("repeat retry = %d %+v, %d posts", code, result, len(provider.posts))
	}
}

func witnessQuestionWithdraw(t *testing.T) {
	b := newProcessBed(t)
	provider := &questionProvider{}
	owners := channelQuestionOwners(b, provider)
	writeQuestionFixture(t, filepath.Join(b.root(), "artifacts", "agents", "channel", "questions", "q1.json"),
		map[string]any{"id": "q1", "goal": bedGoal, "kind": "other", "state": "open", "facts": []string{"Land it?"}, "thread": map[string]any{"threadId": "t", "id": "m"}})
	if code, result := b.runJSON(owners, "question", "withdraw", "q1", "--reason", "decided in review"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first withdraw = %d %+v", code, result)
	}
	before, posts := questionFiles(t, b.root()), len(provider.posts)
	code, result := b.runJSON(owners, "question", "withdraw", "q1", "--reason", "decided in review")
	if code != 0 || result.Outcome != intentUnchanged || len(provider.posts) != posts || !sameFiles(before, questionFiles(t, b.root())) {
		t.Fatalf("repeat withdraw = %d %+v, posts %d (was %d)", code, result, len(provider.posts), posts)
	}
	// The channel owner itself makes the same no-op, whoever calls it.
	if _, err := channel.Withdraw(b.root(), "q1", "again", provider, channel.DestinationConfig{}); err != nil || len(provider.posts) != posts || !sameFiles(before, questionFiles(t, b.root())) {
		t.Fatalf("the owner's repeat withdrawal recorded something: %v", err)
	}
}

// alreadyRunningEngine is an engine stand-in that answers a mission resume
// the way the runner does when the mission's runner is live.
func alreadyRunningEngine(calls *[][]string) *intentDeliveryOwners {
	return processBackedDelivery(&intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			*calls = append(*calls, process.argv)
			return intentProcessResult{stdout: []byte(missionrunner.AlreadyRunningPrefix + "demo (runner pid 42); nothing was started\n")}
		}})
}

func witnessQuestionAnswer(t *testing.T) {
	b := newProcessBed(t)
	owners := b.owners()
	var calls [][]string
	owners.delivery = alreadyRunningEngine(&calls)
	askPath := filepath.Join(b.root(), "artifacts", "agents", "missions", "demo", "asks", "done-ask.json")
	writeQuestionFixture(t, askPath, map[string]any{"askId": "done-ask", "answeredAt": "2026-09-25T10:00:00Z", "answer": "yes"})
	recorded, err := os.ReadFile(askPath)
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		code, result := b.runJSON(owners, "question", "answer", "demo/done-ask", "yes")
		if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already recorded") {
			t.Fatalf("answer %d = %d %+v", run, code, result)
		}
	}
	if again, _ := os.ReadFile(askPath); !bytes.Equal(again, recorded) {
		t.Fatal("a repeated answer rewrote the question")
	}
	for _, call := range calls {
		if len(call) < 4 || call[3] != "resume" {
			t.Fatalf("a repeated answer reached the engine for %v", call)
		}
	}
	if code, result := b.runJSON(owners, "question", "answer", "demo/done-ask", "no"); code != 1 || result.Outcome != intentRefused {
		t.Fatalf("a different answer = %d %+v; want the conflict refused", code, result)
	}
}

// seedIncident publishes one open trunk-red entry into the bed's ledger.
func seedIncident(t *testing.T, b *intentBed, id string) {
	t.Helper()
	stamp := "2026-09-17T09:00:00Z"
	register := goal.RenderTrunkRed([]goal.TrunkRedEntry{{
		ID: id, Identity: id, Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
		Sightings: []goal.TrunkRedSighting{{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: stamp,
			Opid: goal.Opid("01J5X0000000000000000000X1", "mac-cli", "m1")}},
		Owner: goal.TrunkRedOwner{Machine: "mac-other", Since: stamp, How: "joiner"}, Holds: []string{"batch-1"}, Opened: stamp,
	}})
	parent := b.repo.accepted
	tip, err := b.repo.Build(goal.Opid("01J5X0000000000000000000X2", "mac-cli", "m1"), parent,
		[]goal.Change{{Path: "plans/goals/trunk-red.json", Content: register}}, "incident fixture")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := b.repo.Publish(parent, tip); err != nil || outcome != goal.CASLanded {
		t.Fatalf("publish incident fixture: %v %v", outcome, err)
	}
	if err := b.repo.AcceptedCAS(parent, tip); err != nil {
		t.Fatal(err)
	}
}

func witnessIncidentClaim(t *testing.T) {
	b := newIntentBed(t, false, nil)
	b.lineage = "m1"
	seedIncident(t, b, "tr-fast-idem000001")
	claim := []string{"incident", "claim", "tr-fast-idem000001", "--goal", bedGoal, "--by", "Wido", "--to", "mac-fix"}
	if code, result := b.runJSON(b.owners(), claim...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first claim = %d %+v", code, result)
	}
	publications, accepted := b.publications(), b.repo.accepted
	code, result := b.runJSON(b.owners(), claim...)
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already owned by mac-fix") ||
		b.publications() != publications || b.repo.accepted != accepted {
		t.Fatalf("repeat claim = %d %+v, %d publications (was %d)", code, result, b.publications(), publications)
	}
}

func witnessIncidentClose(t *testing.T) {
	b := newIntentBed(t, false, nil)
	seedIncident(t, b, "tr-fast-idem000002")
	closeArgs := []string{"incident", "close", "tr-fast-idem000002", "--reason", "the flake is fixed at its source"}
	if code, result := b.runJSON(b.owners(), closeArgs...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first close = %d %+v", code, result)
	}
	publications, accepted := b.publications(), b.repo.accepted
	code, result := b.runJSON(b.owners(), closeArgs...)
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already closed") ||
		b.publications() != publications || b.repo.accepted != accepted {
		t.Fatalf("repeat close = %d %+v, %d publications (was %d)", code, result, b.publications(), publications)
	}
}

// witnessMissionLaunch runs start or resume twice through the mission
// runner owner over a live runner (this test process, carrying the lease's
// tag in its own command line), then once through the router, whose engine
// answers as that owner did.
func witnessMissionLaunch(mode string) func(*testing.T) {
	return func(t *testing.T) {
		selfCommand := liveRunnerTag(t)
		root := t.TempDir()
		engine := missionrunner.NewEngine(root, "mr-idem")
		dir := filepath.Join(root, "artifacts", "agents", "missions", "mr-idem")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite := func(name, content string) {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		mustWrite("state.json", `{"status":"running"}`)
		mustWrite("lease.json", `{"missionId":"mr-idem","pid":`+strconv.Itoa(os.Getpid())+`,"pgid":1,"instanceTag":"`+selfCommand+`","startedAt":"x","renewedAt":"x"}`)
		engine.ArmSupervision = func([]string) (string, string, int) {
			t.Error("an already running mission was armed")
			return "", "", 1
		}
		snapshot := func() map[string]string { return snapshotTree(t, root) }
		if code := engine.Launch(mode, false); code != 0 {
			t.Fatalf("first %s over a live runner = %d", mode, code)
		}
		before := snapshot()
		if code := engine.Launch(mode, false); code != 0 {
			t.Fatalf("repeat %s over a live runner = %d", mode, code)
		}
		if !sameFiles(before, snapshot()) {
			t.Fatalf("a repeated %s over a live runner wrote a file", mode)
		}

		b := newProcessBed(t)
		owners := b.owners()
		var calls [][]string
		owners.delivery = alreadyRunningEngine(&calls)
		code, result := b.runJSON(owners, "mission", mode, "demo")
		if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already running") || len(calls) != 1 {
			t.Fatalf("mission %s over a running mission = %d %+v", mode, code, result)
		}
	}
}

// liveRunnerTag is a tag this test process's own command line carries, so a
// lease naming this pid and tag is a live runner to the mission owner.
func liveRunnerTag(t *testing.T) string {
	t.Helper()
	exact, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive || len(exact.Argv) == 0 {
		t.Skip("own argv unreadable on this host")
	}
	command := strings.Join(exact.Argv, " ")
	tag := command[strings.LastIndex(command, "/")+1:]
	if index := strings.Index(tag, " "); index > 0 {
		tag = tag[:index]
	}
	if tag == "" {
		t.Skip("no usable self tag")
	}
	return tag
}

// witnessMissionRepair drives the router over an engine that answers as the
// mission owner does for the recorded resolution again (proven at the owner
// by internal/missionrunner TestResolveTaintRestore and
// TestResolveTaintTailCompletion): exit 0, one already-resolved line, and
// nothing written. The public result is unchanged both times.
func witnessMissionRepair(t *testing.T) {
	b := newProcessBed(t)
	owners := b.owners()
	var calls [][]string
	owners.delivery = processBackedDelivery(&intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			calls = append(calls, process.argv)
			return intentProcessResult{stdout: []byte("mission=demo taint=2 " + missionrunner.TaintAlreadyResolved + "(restore by Wido); nothing was recorded\n")}
		}})
	before := snapshotTree(t, b.root())
	tree := strings.Repeat("a", 40)
	for run := 1; run <= 2; run++ {
		code, result := b.runJSON(owners, "mission", "repair", "demo", "--problem", "2", "--confirm-restored", tree, "--by", "Wido", "--reason", "restored from the snapshot")
		if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already resolved") {
			t.Fatalf("repair %d = %d %+v", run, code, result)
		}
	}
	if len(calls) != 2 || !sameFiles(before, snapshotTree(t, b.root())) {
		t.Fatalf("a repeated repair recorded something: %d owner calls", len(calls))
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			data, _ := os.ReadFile(path)
			files[path] = string(data)
		}
		return nil
	})
	return files
}
