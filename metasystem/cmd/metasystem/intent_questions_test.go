package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"golang.org/x/sys/unix"
)

type questionProvider struct {
	fail  bool
	posts []string
}

func (p *questionProvider) Post(_ context.Context, _ channel.DestinationConfig, text string, _ *channel.MessageRef) (channel.MessageRef, error) {
	if p.fail {
		return channel.MessageRef{}, errors.New("fixture: the provider is down")
	}
	p.posts = append(p.posts, text)
	return channel.MessageRef{ThreadID: "thread-1", ID: "m1"}, nil
}
func (p *questionProvider) Receive(context.Context, channel.DestinationConfig, []channel.MessageRef, channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	return nil, "", nil
}
func (p *questionProvider) Confirm(context.Context, channel.DestinationConfig, channel.Cursor) error {
	return nil
}
func (p *questionProvider) Credential(context.Context, channel.DestinationConfig) (channel.CredentialIdentity, error) {
	return channel.CredentialIdentity{}, nil
}

// TestIntentQuestionJourney drives show, retry, withdraw, answer and wait
// over the real channel question records and the channel owner's targeted
// retry and withdrawal under its poll lock, with a fake provider; mission
// questions are the mission's own ask records. The mission resume is the
// engine process seam.
func TestIntentQuestionJourney(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	provider := &questionProvider{fail: true}
	owners := b.owners()
	owners.processes.question = channel.ReadQuestion
	owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
		return provider, channel.DestinationConfig{}
	}
	var engineCalls [][]string
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			engineCalls = append(engineCalls, process.argv)
			return intentProcessResult{stdout: []byte(`{"outcome":"resumed"}`)}
		}}
	root := b.root()
	for _, id := range []string{"q1", "q2", "shared"} {
		writeQuestionFixture(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", id+".json"),
			map[string]any{"id": id, "goal": bedGoal, "kind": "other", "state": "open", "facts": []string{"Land it?"}, "options": []any{map[string]any{"label": "yes", "consequence": "land"}}})
	}
	for _, ask := range []map[string]any{
		{"askId": "open-ask", "answeredAt": nil},
		{"askId": "done-ask", "answeredAt": "2026-09-25T10:00:00Z", "answer": "yes"},
		{"askId": "shared", "answeredAt": nil},
	} {
		writeQuestionFixture(t, filepath.Join(root, "artifacts", "agents", "missions", "demo", "asks", ask["askId"].(string)+".json"), ask)
	}
	run := func(args ...string) (int, intentResult) { return b.runJSON(owners, args...) }
	read := func(id string) channel.Question {
		q, err := channel.ReadQuestion(root, id)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}

	if _, result := run("show", "question", "q1"); result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "ask", "--retry", "q1"}) {
		t.Fatalf("an undelivered question offers its retry: %+v", result)
	}
	if code, result := run("ask", "--retry", "q1"); code == 0 || result.Outcome != intentFailed || read("q1").Undelivered != 1 {
		t.Fatalf("failed retry: %d %+v", code, result)
	}
	provider.fail = false
	if code, result := run("ask", "--retry", "q1"); code != 0 || result.Outcome != intentConfirmed || read("q1").Thread == nil || len(provider.posts) != 1 {
		t.Fatalf("retry: %d %+v", code, result)
	}
	if read("q2").Thread != nil || read("q2").Undelivered != 0 {
		t.Fatal("a targeted retry touched another question")
	}
	if code, result := run("ask", "--retry", "q1"); code != 0 || result.Outcome != intentUnchanged || len(provider.posts) != 1 {
		t.Fatalf("a delivered question is not sent again: %d %+v", code, result)
	}
	// Withdrawal: a reason is required, the poll lock is respected.
	if code, result := run("ask", "--withdraw", "q2"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("withdraw without a reason: %d %+v", code, result)
	}
	lock, err := os.OpenFile(filepath.Join(root, "artifacts", "agents", "channel", "lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil || unix.Flock(int(lock.Fd()), unix.LOCK_EX) != nil {
		t.Fatal(err)
	}
	if _, result := run("ask", "--withdraw", "q2", "--reason", "decided already"); result.Outcome != intentInProgress || read("q2").State != "open" {
		t.Fatalf("a withdrawal while a poll holds the lock: %+v", result)
	}
	unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	lock.Close()
	if code, result := run("ask", "--withdraw", "q2", "--reason", "decided already"); code != 0 || read("q2").State != "closed" {
		t.Fatalf("withdraw: %d %+v", code, result)
	}
	if _, result := run("ask", "--withdraw", "q2", "--reason", "again"); result.Outcome != intentUnchanged {
		t.Fatalf("repeat withdraw: %+v", result)
	}
	if _, result := run("ask", "--retry", "q1", "--withdraw", "q1"); result.Outcome != intentRefused {
		t.Fatalf("retry and withdraw together: %+v", result)
	}
	// A channel question is answered in its thread; local text is no proof.
	if _, result := run("answer", "q1", "yes"); result.Outcome != intentRefused || result.Decision != channel.ReplyInstructions(read("q1")) || read("q1").Answer != nil {
		t.Fatalf("channel answer: %+v", result)
	}
	// A shared id is never resolved by precedence.
	if _, result := run("answer", "shared", "yes"); result.Outcome != intentRefused ||
		!slices.Equal(questionCandidates(result.Data), []string{"channel:shared", "demo/shared"}) {
		t.Fatalf("ambiguous question: %+v", result)
	}
	if _, result := run("show", "question", "channel:shared"); result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "channel question shared") {
		t.Fatalf("the explicit channel spelling: %+v", result)
	}
	// The recorded answer again completes the resume; another answer refuses.
	if _, result := run("answer", "demo/done-ask", "no"); result.Outcome != intentRefused || len(engineCalls) != 0 {
		t.Fatalf("conflicting repeat answer: %+v", result)
	}
	if code, result := run("answer", "demo/done-ask", "yes"); code != 0 || len(engineCalls) != 1 || !slices.Equal(engineCalls[0][1:4], []string{"mission", "resume", "--root"}) {
		t.Fatalf("repeated answer resumes: %d %+v %v", code, result, engineCalls)
	}
	if _, result := run("wait", "question", "demo/open-ask"); result.Outcome != intentInProgress || result.Next == nil {
		t.Fatalf("waiting for an unanswered mission question: %+v", result)
	}
}

func writeQuestionFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func questionCandidates(data any) []string {
	var out []string
	object, _ := data.(map[string]any)
	list, _ := object["candidates"].([]any)
	for _, one := range list {
		text, _ := one.(string)
		out = append(out, text)
	}
	return out
}

// TestIntentAskContinuationsAreFollowed follows every continuation ask
// prints: a posted question continues with wait question channel:Q, which
// reaches the channel wait owner for exactly Q and, after a bounded wait,
// keeps its channel qualification; an undelivered stored question continues
// with ask --retry Q, which delivers exactly that question.
func TestIntentAskContinuationsAreFollowed(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	provider := &questionProvider{}
	owners := b.owners()
	owners.processes.question = channel.ReadQuestion
	owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
		return provider, channel.DestinationConfig{}
	}
	root := b.root()
	var asked channel.Question
	owners.processes.ask = func(string, channelAskInput) (channel.Question, []string, int, error) { return asked, nil, 0, nil }
	var engineCalls [][]string
	waitCode := 124
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			engineCalls = append(engineCalls, process.argv)
			return intentProcessResult{code: waitCode, stderr: []byte("the question is not answered yet\n")}
		}}
	run := func(args ...string) (int, intentResult) { return b.runJSON(owners, args...) }
	for _, id := range []string{"posted", "stored"} {
		writeQuestionFixture(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", id+".json"),
			map[string]any{"id": id, "goal": bedGoal, "kind": "other", "state": "open", "facts": []string{"Land it?"}, "options": []any{map[string]any{"label": "yes", "consequence": "land"}}})
	}
	ask := []string{"ask", bedGoal, "--question", "Land it?", "--option", "yes: land it", "--option", "no: wait", "--recommend", "yes"}

	asked = channel.Question{ID: "posted", Goal: bedGoal, State: "open", Thread: &channel.MessageRef{ThreadID: "thread-9", ID: "m9"}}
	_, result := run(ask...)
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "question", "channel:posted"}) {
		t.Fatalf("a posted question continues with its qualified wait: %+v", result)
	}
	_, waited := run(result.Next.Argv[1:]...)
	if len(engineCalls) != 1 || !slices.Contains(engineCalls[0], "wait") || flagValue(engineCalls[0], "--question") != "posted" {
		t.Fatalf("the wait reaches the channel wait owner for exactly the question: %v", engineCalls)
	}
	if waited.Next == nil || !slices.Equal(waited.Next.Argv, []string{"metasystem", "wait", "question", "channel:posted"}) {
		t.Fatalf("a bounded wait keeps the channel qualification: %+v", waited)
	}
	waitCode = 0
	if code, again := run(waited.Next.Argv[1:]...); code != 0 || again.Outcome != intentConfirmed || len(engineCalls) != 2 {
		t.Fatalf("the continued wait reaches the answer: code=%d %+v", code, again)
	}

	asked = channel.Question{ID: "stored", Goal: bedGoal, State: "open", Undelivered: 1}
	_, result = run(ask...)
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "ask", "--retry", "stored"}) {
		t.Fatalf("an undelivered question continues with its exact retry: %+v", result)
	}
	if code, retried := run(result.Next.Argv[1:]...); code != 0 || retried.Outcome != intentConfirmed || len(provider.posts) != 1 {
		t.Fatalf("the retry delivers exactly that question: code=%d %+v posts=%d", code, retried, len(provider.posts))
	}
	if q, err := channel.ReadQuestion(root, "stored"); err != nil || q.Thread == nil {
		t.Fatalf("the stored question is delivered: %+v %v", q, err)
	}
	if q, _ := channel.ReadQuestion(root, "posted"); q.Thread != nil {
		t.Fatal("the retry of one question must not deliver another")
	}
}

// TestIntentResumeArchivedGoalContinuesToReopen: resume of a concluded goal
// is refused toward the public reopen act, and following that continuation
// reopens the goal through its owner with the next step it names.
func TestIntentResumeArchivedGoalContinuesToReopen(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	if code, result := bed.runJSON(bed.owners(), "done", bedGoal, "--reason", "shipped and verified", "--lineage", "m1"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("done = %d %+v", code, result)
	}
	code, result := bed.runJSON(bed.owners(), "resume", bedGoal)
	if code == 0 || result.Outcome != intentRefused || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "reopen", bedGoal, "--next", "TEXT"}) ||
		strings.Contains(result.Decision, "internal") {
		t.Fatalf("resume of a done goal: code=%d %+v", code, result)
	}
	argv := slices.Clone(result.Next.Argv[1:])
	argv[len(argv)-1] = "finish the retry path"
	code, reopened := bed.runJSON(bed.owners(), append(argv, "--lineage", "m1")...)
	if code != 0 || reopened.Outcome != intentConfirmed || bed.goalFile(bedGoal).State == goal.StateDone || bed.goalFile(bedGoal).NextStep != "finish the retry path" {
		t.Fatalf("the printed reopen: code=%d %+v state=%s", code, reopened, bed.goalFile(bedGoal).State)
	}
}

// TestIntentClaimContinuesReservedGoalByItsShow: a held goal whose id is a
// word show reads as a record subject is continued with show --goal G, and
// following that command shows that goal's own record.
func TestIntentClaimContinuesReservedGoalByItsShow(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"designs", "decisions", "record", "design", "question"} {
		bed := newIntentBed(t, false, nil)
		bed.lineage = "m1"
		held := *bed.goalFile(bedGoal)
		held.Id = name
		for index := range held.History {
			held.History[index].Targets = []string{name}
		}
		released := bed.goalFile(bedGoal)
		released.State, released.Claimed = goal.StateApproved, nil
		bed.addGoal(released)
		bed.addGoal(&held)
		code, result := bed.runJSON(bed.owners(), "claim")
		if result.Outcome != intentUnchanged || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "show", "--goal", name}) {
			t.Fatalf("claim while %s is held: code=%d %+v", name, code, result)
		}
		code, shown := bed.runJSON(bed.owners(), result.Next.Argv[1:]...)
		if code != 0 || shown.Outcome != intentConfirmed || len(shown.Targets) == 0 || shown.Targets[0].ID != name || !strings.Contains(shown.Summary, name) {
			t.Fatalf("the printed show for goal %s: code=%d %+v", name, code, shown)
		}
	}
}

// TestIntentAbandonWithSuccessor drives public abandon through the real goal
// owners. A bad successor or a caller without a person's authority abandons
// nothing. An abandonment that recorded no successor is completed by the same
// public command through the carry owner, keeping its original reason and
// abandonment, with no internal command offered; repeating it changes
// nothing.
func TestIntentAbandonWithSuccessor(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	template := *bed.goalFile(bedGoal)
	file := func(id string) goal.GoalFile {
		file := template
		file.Id, file.State, file.Claimed = id, goal.StateApproved, nil
		file.History = append([]goal.HistoryLine(nil), template.History...)
		for index := range file.History {
			file.History[index].Targets = []string{id}
		}
		return file
	}
	for _, id := range []string{"old-goal", "new-goal", "dep-goal"} {
		live := file(id)
		if id == "dep-goal" {
			live.Blocked = []string{"old-goal"}
		}
		bed.addGoal(&live)
	}
	// This build and the fleet floor are per-request facts the abandon owner
	// checks: the ledger records the floor, and this instance's build is it.
	floor := strings.Repeat("a", 40)
	bed.setRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
		History: []goal.HistoryLine{{At: "2026-09-01T08:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB1", "mac-cli", "m1"), Verb: "engine-floor",
			Actor: "human:Wido", Reason: floor + " every enrolled seat runs this engine or newer", Keep: -1}}})
	fleet := bed.owners()
	fleet.dependencies.configureAbandon = func(request *goal.VerbRequest) {
		request.ConfigureAbandon(func() string { return floor }, func(string, string, string) (bool, error) { return true, nil },
			func(string, func(string, string) (bool, error), time.Time) ([]string, error) { return nil, nil })
	}
	lone := file("lone-goal")
	lone.State, lone.Revision = goal.StateAbandoned, lone.Revision+1
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB2", "mac-cli", "m1")
	lone.Abandoned = &goal.AbandonRecord{By: "human:Wido", At: "2026-09-01T09:00:00Z", Revision: lone.Revision, Opid: opid, Because: "no longer needed"}
	lone.History = append(lone.History, goal.HistoryLine{At: "2026-09-01T09:00:00Z", Opid: opid, Verb: "abandon", Actor: "human:Wido",
		Targets: []string{"lone-goal"}, Reason: "no longer needed", Keep: -1})
	rendered := goal.RenderFile(&lone)
	os.MkdirAll(filepath.Join(bed.root(), "records", "goals"), 0o755)
	os.WriteFile(filepath.Join(bed.root(), "records", "goals", "lone-goal.md"), rendered, 0o644)
	bed.repo.commit(bed.repo.accepted).files["records/goals/lone-goal.md"] = rendered
	human := []string{"--by", "Wido", "--fixture-human-authority", "--lineage", "m1"}
	abandon := func(args ...string) (int, intentResult) {
		return bed.runJSON(bed.owners(), append(append([]string{"abandon"}, args...), human...)...)
	}
	for _, successor := range []string{"old-goal", "no-such-goal"} {
		if code, result := abandon("old-goal", "--reason", "superseded", "--successor", successor); code == 0 || result.Outcome == intentConfirmed {
			t.Fatalf("successor %s: code=%d %+v", successor, code, result)
		}
	}
	notPerson := bed.owners()
	notPerson.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("process 42 is not the enrolled terminal")
	}
	if code, result := bed.runJSON(notPerson, "abandon", "old-goal", "--reason", "superseded", "--successor", "new-goal", "--lineage", "m1"); code == 0 || result.Outcome == intentConfirmed {
		t.Fatalf("an abandonment without a person's authority: code=%d %+v", code, result)
	}
	if bed.goalFile("old-goal").State == goal.StateAbandoned {
		t.Fatal("a refused abandonment abandoned the goal")
	}
	if code, result := bed.runJSON(notPerson, "abandon", "lone-goal", "--reason", "no longer needed", "--successor", "new-goal", "--lineage", "m1"); code == 0 ||
		result.Outcome == intentConfirmed || bed.goalFile("lone-goal").Abandoned.Carried != "" {
		t.Fatalf("recording a successor without a person's authority: code=%d %+v", code, result)
	}
	withFleet := func(args ...string) (int, intentResult) {
		return bed.runJSON(fleet, append(append([]string{"abandon"}, args...), human...)...)
	}
	if code, result := withFleet("old-goal", "--reason", "superseded", "--successor", "no-such-goal"); code == 0 || result.Outcome == intentConfirmed ||
		bed.goalFile("old-goal").State == goal.StateAbandoned || !slices.Equal(bed.goalFile("dep-goal").Blocked, []string{"old-goal"}) {
		t.Fatalf("an invalid successor with valid fleet facts: code=%d %+v", code, result)
	}
	code, fresh := withFleet("old-goal", "--reason", "superseded", "--successor", "new-goal")
	old, dep := bed.goalFile("old-goal"), bed.goalFile("dep-goal")
	if code != 0 || fresh.Outcome != intentConfirmed || old.State != goal.StateAbandoned || old.Abandoned == nil || old.Abandoned.Carried != "new-goal" ||
		!slices.Equal(dep.Blocked, []string{"new-goal"}) {
		t.Fatalf("fresh abandonment with a live dependent: code=%d %+v abandoned=%+v dependent=%v", code, fresh, old.Abandoned, dep.Blocked)
	}
	before := len(bed.goalFile("lone-goal").History)
	code, result := abandon("lone-goal", "--reason", "no longer needed", "--successor", "new-goal")
	recovered := bed.goalFile("lone-goal")
	if code != 0 || result.Outcome != intentConfirmed || recovered.Abandoned.Carried != "new-goal" || recovered.Abandoned.Because != "no longer needed" ||
		recovered.Abandoned.Opid != opid || len(recovered.History) != before+1 || strings.Contains(fmt.Sprint(result), "internal") {
		t.Fatalf("recovery names the successor: code=%d %+v abandoned=%+v", code, result, recovered.Abandoned)
	}
	code, result = abandon("lone-goal", "--reason", "no longer needed", "--successor", "new-goal")
	if code != 0 || result.Outcome != intentUnchanged || len(bed.goalFile("lone-goal").History) != before+1 {
		t.Fatalf("a repeated recovery: code=%d %+v", code, result)
	}
}

// TestIntentSettingsKeysAndCheck runs the config keys and validate owners as
// the real engine child on a synthetic configuration of an installation the
// caller names with --repo from elsewhere: settings --keys lists configured
// keys beyond the launch settings, --matching narrows them by prefix, and
// check settings fails on an invalid known setting. Nothing is written.
func TestIntentSettingsKeysAndCheck(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	engine := intentTestEngine(t)
	owners := bed.workOwners()
	owners.delivery = &intentDeliveryOwners{process: runIntentOwnerProcess, executable: func() (string, error) { return engine, nil }}
	conf := filepath.Join(bed.root(), "metasystem.conf")
	original, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(conf, append(original, []byte("\nfixture.alpha.one=1\nfixture.alpha.two=2\nfixture.beta=3\n")...), 0o644)
	caller := t.TempDir()
	run := func(args ...string) (int, intentResult) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--repo", bed.root(), "--json"), &stdout, &stderr, caller, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	code, all := run("settings", "--keys")
	listing := fmt.Sprint(all.Data)
	if code != 0 || all.Outcome != intentConfirmed || !strings.Contains(listing, "fixture.alpha.one") || !strings.Contains(listing, "fixture.beta") {
		t.Fatalf("settings --keys: code=%d %+v", code, all)
	}
	code, matching := run("settings", "--keys", "--matching", "fixture.alpha")
	narrowed := fmt.Sprint(matching.Data)
	if code != 0 || !strings.Contains(narrowed, "fixture.alpha.two") || strings.Contains(narrowed, "fixture.beta") {
		t.Fatalf("settings --keys --matching: code=%d %+v", code, matching)
	}
	// The bed's configuration lacks testing.contract; the validate owner's
	// own reason is the result.
	if code, incomplete := run("check", "settings"); code == 0 || incomplete.Outcome != intentRefused || !strings.Contains(incomplete.Summary, "testing.contract is required") {
		t.Fatalf("check settings on an incomplete configuration: code=%d %+v", code, incomplete)
	}
	invalid := append(append([]byte{}, original...), []byte("\ntesting.contract=missing-contract.json\n")...)
	os.WriteFile(conf, invalid, 0o644)
	if code, refused := run("check", "settings"); code == 0 || refused.Outcome == intentConfirmed || !strings.Contains(refused.Summary, "testing.contract is invalid") ||
		!strings.Contains(refused.Summary, "missing-contract.json") {
		t.Fatalf("check settings on an invalid setting: code=%d %+v", code, refused)
	}
	if after, _ := os.ReadFile(conf); !bytes.Equal(after, invalid) {
		t.Fatal("check settings changed the configuration")
	}
	// The accepted configuration shape of the config owner's own fixture
	// (validConf and minimalTestingContract in internal/config), with only
	// the fake runtime enabled, whose registration needs no directory.
	valid := []byte("metasystem.version=1\nmetasystem.runtimes=fake\ntesting.contract=testing.json\n" +
		"evidence.root=" + t.TempDir() + "\nrole.default.runtime=fake\n" +
		"role.default.model.fake=fake-model\nmodel.tier.1=fake:fake-model\nfixture.alpha.one=1\n")
	contract := []byte(`{"schemaVersion":1,"projectRisk":{"severity":1,"exposure":1,"reversibility":"revert","detection":"immediate","recovery":"bounded"},"surfaces":[{"id":"app","paths":["src/**"],"dependsOn":[],"standard":["section/smoke"],"deep":[],"critical":[]}],"groups":[{"id":"section/smoke","kind":"integration","adapter":"section","cwd":".","inputs":["metasystem.conf"],"outputs":[],"tools":[],"obligations":[],"platforms":["any"],"targetMs":1000,"section":"smoke"}],"always":{"canary":["section/smoke"],"standard":[]},"unknown":["section/smoke"],"cadence":["section/smoke"]}`)
	os.WriteFile(conf, valid, 0o644)
	os.WriteFile(filepath.Join(bed.root(), "testing.json"), contract, 0o644)
	if code, accepted := run("check", "settings"); code != 0 || accepted.Outcome != intentConfirmed {
		t.Fatalf("check settings on a valid configuration: code=%d %+v", code, accepted)
	}
	if after, _ := os.ReadFile(conf); !bytes.Equal(after, valid) {
		t.Fatal("check settings changed the valid configuration")
	}
}
