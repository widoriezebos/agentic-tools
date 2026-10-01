package missionrunner

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// The mission-runner scenario of dispatch-fixtures.sh drove the real runner
// through the fake host over a real repository. These ports drive the same
// runner in process over the git-free host-cycle bed (buildGitFreeHostCycle:
// Git and the contract source are per-test stubs, the fake host and the
// engine binary are real) and assert what each fixture leg asserted.

// u6bportHostCycle is buildGitFreeHostCycle without its demand that the
// serving-goal source was read: a mission refused before its first turn
// (the fence leg) never assembles a prompt.
func u6bportHostCycle(t *testing.T, behavior string) *Engine {
	t.Helper()
	return u6bportHostCycleWithContract(t, behavior, nil)
}

func u6bportHostCycleWithContract(t *testing.T, behavior string, edit func(string) string) *Engine {
	t.Helper()
	e, f := newGitFreePreflightBedWithContract(t, behavior, nil, edit)
	equipFullCycleFiles(t, e)
	e.goalSource = &mission.GoalSource{Endpoint: goal.Endpoint{
		Root: e.Root, Remote: "origin", Branch: "refs/heads/main", Repository: &hostCycleGoals{t: t},
	}, Machine: "fixture-machine"}
	if err := e.armAndPreflight("start"); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	t.Cleanup(f.done)
	return e
}

// u6bportRequireUnverifiedStart asserts the launcher's refusal: the runner
// never reported a verified host start, which the start verb turns into
// its exit 3.
func u6bportRequireUnverifiedStart(t *testing.T, signal string) {
	t.Helper()
	if !pathExists(signal) {
		return
	}
	doc := readTestDoc(t, signal)
	if doc["verified"] == true {
		t.Fatalf("the runner reported a verified host start: %v", doc)
	}
}

func u6bportTurnDirs(t *testing.T, engine *Engine) []string {
	t.Helper()
	records, err := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 0, len(records))
	for _, record := range records {
		dirs = append(dirs, filepath.Dir(record))
	}
	sort.Slice(dirs, func(i, j int) bool {
		left, _ := readJSONDoc(filepath.Join(dirs[i], "turn.json"))
		right, _ := readJSONDoc(filepath.Join(dirs[j], "turn.json"))
		a, _ := jsonInt(left["cycle"])
		b, _ := jsonInt(right["cycle"])
		return a < b
	})
	return dirs
}

func u6bportOpenAsks(t *testing.T, engine *Engine, reasonClass string) []map[string]any {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "*.json"))
	var open []map[string]any
	for _, file := range files {
		ask, err := readJSONDoc(file)
		if err != nil {
			t.Fatalf("unparseable mission ask %s: %v", file, err)
		}
		if ask["reasonClass"] == reasonClass && ask["answeredAt"] == nil {
			open = append(open, ask)
		}
	}
	return open
}

// runner-cycle: one return-ok turn completes the mission (status exit 10);
// the prompt the runner assembled opens with the seven machine header keys
// in order, the shipped orchestrator preamble byte-for-byte after the header
// break, and passes the turn-prompt checker; the verified start stamped
// the host's own process group and minted tag on the turn; the ledger books the cycle;
// an unconfigured mission books no Patience line; the runner releases its
// lease as it exits. (The contract-improved classification itself is the
// measurement owner's: contract TestContractMeasureClassifiesAgainstPrevious
// and missionrunner TestDrainStallEndToEnd.)
func TestU6bPortReturnOkCycleCompletesWithACheckedPrompt(t *testing.T) {
	t.Parallel()
	engine := u6bportHostCycle(t, "FAKEHOST:return-ok")
	signal := filepath.Join(t.TempDir(), "start.json")
	if code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal); code != 0 {
		t.Fatalf("return-ok mission exited %d", code)
	}
	if code := engine.Status(); code != 10 {
		t.Fatalf("status exit %d, want 10 (completed)", code)
	}
	turns := u6bportTurnDirs(t, engine)
	if len(turns) == 0 {
		t.Fatal("no turn ran")
	}
	// The verified host start stamped the turn: the host leads its own
	// process group under the runner-minted instance tag.
	turn := readTestDoc(t, filepath.Join(turns[0], "turn.json"))
	if tag, _ := turn["instanceTag"].(string); tag != "metasystem-host-"+filepath.Base(turns[0]) {
		t.Fatalf("host instance tag changed shape: %v", turn["instanceTag"])
	}
	if pid, ok := jsonInt(turn["pid"]); !ok || pid < 1 {
		t.Fatalf("turn record lacks the host pid: %v", turn["pid"])
	} else if pgid, _ := jsonInt(turn["pgid"]); pgid != pid {
		t.Fatalf("the host is not its own process-group leader: pid=%v pgid=%v", turn["pid"], turn["pgid"])
	}
	prompt, err := os.ReadFile(filepath.Join(turns[0], "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	header, _, found := bytes.Cut(prompt, []byte("\n\n"))
	if !found {
		t.Fatal("host-turn prompt has no header break")
	}
	var keys []string
	for _, line := range strings.Split(string(header), "\n") {
		key, _, _ := strings.Cut(line, ": ")
		keys = append(keys, key)
	}
	wantKeys := []string{"Mission-Id", "Turn-Id", "Cycle", "Host-Session", "Runtime", "Model", "Reconciliation"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("host-turn prompt header keys %v, want %v", keys, wantKeys)
	}
	preamble, err := protocol.RoleInstructions("orchestrator")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(prompt[len(header)+2:], preamble) {
		t.Fatal("host-turn prompt does not open with the orchestrator preamble after the header break")
	}
	if violation := validate.TurnPrompt(engine.Root, filepath.Join(turns[0], "prompt.md"), turns[0]); violation != nil {
		t.Fatalf("the runner's prompt fails the checker: [%s] %s", violation.Check, violation.Message)
	}
	ledger, err := os.ReadFile(filepath.Join(engine.missionDir(), "ledger.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), "### Cycle 1\n- Classification: ") {
		t.Fatalf("the full cycle did not book its measurement:\n%s", ledger)
	}
	if strings.Contains(string(ledger), "Patience") {
		t.Fatalf("an unconfigured mission booked a Patience line:\n%s", ledger)
	}
	if _, err := os.Stat(filepath.Join(engine.missionDir(), "lease.d")); !os.IsNotExist(err) {
		t.Fatalf("the finished runner retained its lease: %v", err)
	}
}

// runner-bad-prompt: a prompt the turn-prompt checker refuses (here a
// duplicated "## Streams" heading from a stray contract heading) fails the
// turn as prompt-refused before any host launch — no raw.out — and parks the
// mission (status exit 11) without a verified start.
func TestU6bPortPromptCheckerRefusalParksWithoutLaunchingTheHost(t *testing.T) {
	t.Parallel()
	engine := u6bportHostCycleWithContract(t, "FAKEHOST:return-ok", func(contract string) string { return contract + "\n## Streams\n" })
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	u6bportRequireUnverifiedStart(t, signal)
	if code := engine.Status(); code != 11 {
		t.Fatalf("status exit %d, want 11 (parked)", code)
	}
	turns := u6bportTurnDirs(t, engine)
	if len(turns) != 1 {
		t.Fatalf("the refused mission ran %d turns, want 1", len(turns))
	}
	if _, err := os.Stat(filepath.Join(turns[0], "raw.out")); !os.IsNotExist(err) {
		t.Fatalf("prompt-checker refusal launched the fake host: %v", err)
	}
	turn := readTestDoc(t, filepath.Join(turns[0], "turn.json"))
	if detail, _ := turn["detail"].(string); turn["error"] != "prompt-refused" || !strings.Contains(detail, "heading") {
		t.Fatalf("the checker's headings refusal was not recorded on the turn: %v", turn)
	}
}

// runner-ghost: a host return dispatching a job whose record never existed
// is rejected as a dispatched item naming the missing record, and the
// rejection raises an open host-failure ask under the recorded ask id.
func TestU6bPortGhostDispatchIsRejectedWithAnOpenHostFailureAsk(t *testing.T) {
	t.Parallel()
	engine := u6bportHostCycle(t, "FAKEHOST:dispatch-ghost")
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var last map[string]any
	for _, raw := range turnLogOf(state) {
		entry, _ := raw.(map[string]any)
		if kind, _ := entry["kind"].(string); kind == "wall-verification" {
			continue
		}
		last = entry
	}
	if last == nil {
		t.Fatal("the ghost mission recorded no host turn")
	}
	rejected, _ := last["rejected"].([]any)
	if len(rejected) != 1 {
		t.Fatalf("ghost dispatch produced %d rejections, want 1: %v", len(rejected), last["rejected"])
	}
	item, _ := rejected[0].(map[string]any)
	if item["kind"] != "dispatched" {
		t.Fatalf("the ghost rejection is not a dispatched rejection: %v", item)
	}
	if reason, _ := item["reason"].(string); !strings.Contains(reason, "does not exist") {
		t.Fatalf("the ghost rejection reason does not name the missing job: %q", reason)
	}
	askID, _ := item["askId"].(string)
	ask, err := readJSONDoc(filepath.Join(asksDirPath(engine.Root, engine.Mission), askID+".json"))
	if err != nil {
		t.Fatalf("the rejection's ask %q is not on disk: %v", askID, err)
	}
	if ask["reasonClass"] != "host-failure" || ask["answeredAt"] != nil {
		t.Fatalf("the ghost rejection ask is not an open host-failure ask: %v", ask)
	}
}

// runner-fence: a mission whose cycle counter already sits at its sealed
// cycle fence parks on fence before any turn opens, and the batched fence
// ask it raises names the cycles fence only.
func TestU6bPortExhaustedCycleFenceParksBeforeATurn(t *testing.T) {
	t.Parallel()
	engine := u6bportHostCycle(t, "FAKEHOST:return-ok")
	fences := readTestDoc(t, engine.fencesPath())
	fences["cycles"] = 3 // fixtureContract seals fence.cycles=3
	writeJSONFile(t, engine.fencesPath(), fences)
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	u6bportRequireUnverifiedStart(t, signal)
	if code := engine.Status(); code != 11 {
		t.Fatalf("status exit %d, want 11 (parked)", code)
	}
	state := readTestDoc(t, filepath.Join(engine.missionDir(), "state.json"))
	if state["status"] != "parked" || state["parkReason"] != "fence" {
		t.Fatalf("the fence-refused mission is not parked on fence: %v/%v", state["status"], state["parkReason"])
	}
	if turns := u6bportTurnDirs(t, engine); len(turns) != 0 {
		t.Fatalf("a fence-refused cycle opened turns: %v", turns)
	}
	open := u6bportOpenAsks(t, engine, "fence")
	if len(open) != 1 {
		t.Fatalf("the fence park raised %d unanswered fence asks, want 1", len(open))
	}
	question, _ := open[0]["question"].(string)
	if !strings.Contains(question, "`cycles`") || strings.Contains(question, "wall-clock-hours") {
		t.Fatalf("the runner fence refusal was not cycles-only: %q", question)
	}
}

// runner-unverified: a host whose start cannot be verified fails its one
// turn with start-unverified and parks the mission on host-failure with an
// open host-failure ask; acknowledging the ask returns the mission to
// running (status exit 0); resume re-arms supervision, and the resumed run
// completes the mission with a reconciliation turn that names the failed
// prior turn.
func TestU6bPortUnverifiedStartParksThenResumesWithReconciliation(t *testing.T) {
	engine := u6bportHostCycle(t, "FAKEHOST:return-ok")
	t.Setenv("METASYSTEM_FAKE_HOST_START_UNVERIFIED", "1")
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	u6bportRequireUnverifiedStart(t, signal)
	if code := engine.Status(); code != 11 {
		t.Fatalf("status exit %d, want 11 (parked)", code)
	}
	state := readTestDoc(t, filepath.Join(engine.missionDir(), "state.json"))
	if state["parkReason"] != "host-failure" {
		t.Fatalf("the unverified-start mission did not park on host-failure: %v", state["parkReason"])
	}
	turns := u6bportTurnDirs(t, engine)
	if len(turns) != 1 {
		t.Fatalf("the unverified-start mission ran %d turns, want 1", len(turns))
	}
	if turn := readTestDoc(t, filepath.Join(turns[0], "turn.json")); turn["error"] != "start-unverified" {
		t.Fatalf("the failed turn does not carry start-unverified: %v", turn)
	}
	if _, err := os.Stat(filepath.Join(engine.missionDir(), "lease.d")); !os.IsNotExist(err) {
		t.Fatalf("the parked mission retained its runner lease: %v", err)
	}
	open := u6bportOpenAsks(t, engine, "host-failure")
	if len(open) != 1 {
		t.Fatalf("%d unanswered host-failure asks for the unverified start, want 1", len(open))
	}
	askID, _ := open[0]["askId"].(string)
	if code := engine.Answer(askID, "acknowledged"); code != 0 {
		t.Fatalf("acknowledging the host-failure ask exited %d", code)
	}
	if code := engine.Status(); code != 0 {
		t.Fatalf("status after the acknowledgement exited %d, want 0 (running)", code)
	}

	t.Setenv("METASYSTEM_FAKE_HOST_START_UNVERIFIED", "0")
	// Resume re-arms supervision before its run: the launcher's arming
	// step runs the armer again and requires its armed outcome.
	armed := 0
	answer := upAnswered(t, "armed", 0)
	engine.ArmSupervision = func([]string) (verbresult.Result, error) {
		armed++
		return answer, nil
	}
	engine.SupervisionFingerprint = func(string) (string, error) { return "fixture-fingerprint", nil }
	if err := engine.armAndPreflight("resume"); err != nil {
		t.Fatalf("resume preflight: %v", err)
	}
	if armed != 1 {
		t.Fatalf("resume armed supervision %d times, want 1", armed)
	}
	if code := engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-2", signal); code != 0 {
		t.Fatalf("the resumed mission exited %d", code)
	}
	if code := engine.Status(); code != 10 {
		t.Fatalf("status exit %d after resume, want 10 (completed)", code)
	}
	turns = u6bportTurnDirs(t, engine)
	prompt, err := os.ReadFile(filepath.Join(turns[len(turns)-1], "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), "Reconciliation: yes") {
		t.Fatal("the resumed turn did not carry reconciliation")
	}
	if !strings.Contains(string(prompt), "\tfailed\tstart-unverified") {
		t.Fatalf("the resumed turn omitted the failed prior turn from reconciliation:\n%s", prompt)
	}
}

// The runner's end-of-mission chain sweep reaches the delegate lifecycle
// only through Engine.Delegate: reap then close --runner-closed per
// closeable root, and a mission-stamped terminal orphan whose parent walk
// breaks is never named. An engine with no lifecycle wired refuses by name
// rather than skipping the close.
func TestU6bPortChainSweepGoesThroughTheDelegateSeam(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mission := "runner-patience"
	jobs := jobsDirPath(root)
	writeJSONFile(t, filepath.Join(jobs, "pat-lost.json"), map[string]any{"jobId": "pat-lost", "mission": mission,
		"parentJob": "pat-gone", "status": "completed", "role": "design-critic"})
	writeJSONFile(t, filepath.Join(jobs, "chain.json"), map[string]any{"jobId": "chain", "mission": mission, "status": "completed"})
	var argv [][]string
	engine := &Engine{Root: root, Mission: mission, Delegate: func(args ...string) (string, string, int) {
		argv = append(argv, append([]string(nil), args...))
		return "", "", 0
	}}
	if err := engine.closeTerminalChains(); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"reap", "--job", "chain"}, {"close", "--job", "chain", "--runner-closed"}}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("delegate argv %v, want %v", argv, want)
	}

	unwired := &Engine{Root: root, Mission: mission}
	err := unwired.closeTerminalChains()
	if err == nil || !strings.Contains(err.Error(), "chain: the mission runner has no delegate lifecycle wired") {
		t.Fatalf("an unwired engine must refuse the close by name: %v", err)
	}
}

// TestPromptCheckerRunsInTheRunnerNotTheCheckoutEngine is the U9a witness
// that the runner holds each turn prompt to the checker in its own process
// (design 6.2) instead of starting the checkout's engine for `validate
// turn-prompt`: with that engine answering every prompt check with approval,
// a prompt the checker refuses still fails the turn as prompt-refused before
// any host launch.
func TestPromptCheckerRunsInTheRunnerNotTheCheckoutEngine(t *testing.T) {
	t.Parallel()
	// The host-turn instruction is compiled into the engine, so the refused
	// prompt comes from the mission contract: a second required heading.
	engine := u6bportHostCycleWithContract(t, "FAKEHOST:return-ok", func(contract string) string {
		return contract + "\n## Streams\n"
	})
	binary := filepath.Join(engine.Root, "bin", "metasystem")
	real := binary + "-real"
	if err := os.Rename(binary, real); err != nil {
		t.Fatal(err)
	}
	approving := "#!/bin/sh\nif [ \"$1\" = validate ] && [ \"$2\" = turn-prompt ]; then exit 0; fi\nexec '" + real + "' \"$@\"\n"
	if err := testexec.WriteFile(binary, []byte(approving), 0o755); err != nil {
		t.Fatal(err)
	}
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	turns := u6bportTurnDirs(t, engine)
	if len(turns) != 1 {
		t.Fatalf("the refused mission ran %d turns, want 1", len(turns))
	}
	turn := readTestDoc(t, filepath.Join(turns[0], "turn.json"))
	if detail, _ := turn["detail"].(string); turn["error"] != "prompt-refused" || !strings.Contains(detail, "heading") {
		t.Fatalf("the checker's refusal was not the runner's own: %v", turn)
	}
}

// TestFenceAnswerPreflightRunsInTheRunnerNotTheCheckoutEngine is the U9a
// witness that a fence answer preflights the amended contract in the
// runner's own process (design 6.2) instead of starting the checkout's
// engine for `mission contract-preflight`: with that engine approving every
// preflight, a contract whose bytes no longer match their seal is still
// refused as not preflight-ready.
func TestFenceAnswerPreflightRunsInTheRunnerNotTheCheckoutEngine(t *testing.T) {
	t.Parallel()
	engine := u6bportHostCycle(t, "FAKEHOST:return-ok")
	fences := readTestDoc(t, engine.fencesPath())
	fences["cycles"] = 3
	writeJSONFile(t, engine.fencesPath(), fences)
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", filepath.Join(t.TempDir(), "start.json"))
	open := u6bportOpenAsks(t, engine, "fence")
	if len(open) != 1 {
		t.Fatalf("the fence park raised %d fence asks, want 1", len(open))
	}
	binary := filepath.Join(engine.Root, "bin", "metasystem")
	real := binary + "-real"
	if err := os.Rename(binary, real); err != nil {
		t.Fatal(err)
	}
	approving := "#!/bin/sh\nif [ \"$1\" = mission ] && [ \"$2\" = contract-preflight ]; then exit 0; fi\nexec '" + real + "' \"$@\"\n"
	if err := testexec.WriteFile(binary, []byte(approving), 0o755); err != nil {
		t.Fatal(err)
	}
	contractBytes, err := os.ReadFile(engine.contractPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(engine.contractPath(), append(contractBytes, []byte("\nunsealed amendment\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	var errs bytes.Buffer
	engine.Output, engine.Errors = io.Discard, &errs
	askID, _ := open[0]["askId"].(string)
	if code := engine.Answer(askID, "raise the fence"); code != 3 || !strings.Contains(errs.String(), "fence contract amendment is not preflight-ready") {
		t.Fatalf("the unsealed amendment was not refused by the runner's own preflight: %d %q", code, errs.String())
	}
}
