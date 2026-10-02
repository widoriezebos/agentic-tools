package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// intentTestEngine is this package built once as the real engine the
// administrative choices run as their owner process.
func intentTestEngine(t *testing.T) string {
	t.Helper()
	return testenv.Engine(t)
}

// TestIntentRepairAuthority: every administrative choice reaches its real
// owner (in this process, or this package's engine as its own process for
// the mission runner) with its exact
// inputs, and the owner's own authority check refuses a caller that is not a
// person at an agent-free terminal: this test process is such a caller. The
// public adapter never turns a named person into proof. Malformed and
// conflicting choices refuse before any owner runs, and the upgrade runs
// only on the reviewed digest it shows.
func TestIntentRepairAuthority(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	engine := intentTestEngine(t)
	b.owners.executable = func() (string, error) { return engine, nil }
	b.handler = runIntentOwnerProcess
	// The coordinator and goal repair owners run in this process (design
	// 6.2); each call is recorded as the argv its former child carried.
	b.owners.calls = recordingOwnerCalls([]string{engine, "internal"}, func(argv []string) { b.calls = append(b.calls, argv) })
	owner := func(args ...string) (int, intentResult, []string) {
		t.Helper()
		before := len(b.calls)
		code, result := b.do(args...)
		var ran []string
		if len(b.calls) > before {
			ran = b.calls[len(b.calls)-1]
		}
		return code, result, ran
	}
	state := b.intentBed.root()
	for _, row := range []struct {
		args   []string
		verb   []string
		reason string
	}{
		// Outside a declared coordinator the owner lets any caller accept
		// the remote history of the same ledger; here it reaches its fetch,
		// which has no remote in this bed. This row proves the exact route,
		// not an authority refusal.
		{[]string{"goal", "sync", "--accept-remote-history", "--by", "Wido"}, []string{"goal", "repair", "--accept-remote", "--by", "Wido", "--root"}, "git fetch"},
		{[]string{"settings", "coordinator", "--declare", "--by", "Wido"}, []string{"brain", "declare", "--root"}, "is a human act; run it from an agent-free terminal"},
		{[]string{"mission", "repair", "demo", "--problem", "2", "--confirm-restored", strings.Repeat("b", 40), "--by", "Wido", "--reason", "restored"}, []string{"mission", "resolve-taint", "--root"}, "human-reserved act"},
		{[]string{"mission", "repair", "demo", "--problem", "2", "--accept-workspace", "--waive", "claim-a", "--by", "Wido", "--reason", "accepted"}, []string{"mission", "resolve-taint", "--root"}, "human-reserved act"},
	} {
		code, result, ran := owner(row.args...)
		if result.Outcome != intentRefused || code == 0 || len(ran) < len(row.verb)+1 || !slicesHasPrefix(ran[2:], row.verb) || !strings.Contains(result.Summary, row.reason) {
			t.Errorf("%v without a person's proof = %d %+v (owner argv %v)", row.args, code, result, ran)
		}
	}
	// Withdrawing where nothing is declared is the owner's idempotent repeat
	// (R-129-ui): success at every authority, and nothing is touched.
	if code, result, ran := owner("settings", "coordinator", "--withdraw", "--by", "Wido"); code != 0 || result.Outcome != intentUnchanged ||
		len(ran) < 3 || !slicesHasPrefix(ran[2:], []string{"brain", "withdraw", "--root"}) || !strings.Contains(result.Summary, "nothing to withdraw") {
		t.Errorf("withdraw of an undeclared coordinator = %d %+v (owner argv %v)", code, result, ran)
	}
	if _, result, ran := owner("mission", "repair", "demo", "--problem", "2", "--accept-workspace", "--waive", "claim-a", "--by", "Wido", "--reason", "accepted"); !strings.Contains(strings.Join(ran, " "), "--adopt --waives claim-a --by Wido --reason accepted") {
		t.Errorf("accept-workspace inputs: %v %+v", ran, result)
	}
	// Refused before any owner runs.
	for _, args := range [][]string{
		{"goal", "sync", "--publish", "--refresh", "--by", "Wido"},
		{"goal", "sync", "--publish", "--goal", "g"},
		{"goal", "sync", "--publish", "--by", "Wido"},
		{"goal", "sync", "--goal", "g", "--by", "Wido"},
		{"goal", "sync", "--recover", "--refresh"},
		{"goal", "sync", "--recover", "--by", "Wido"},
		{"goal", "sync", "--refresh", "--by", "Wido"},
		{"goal", "sync", "--upgrade", "--by", "Wido", "--sync-mode", "sideways"},
		{"mission", "repair", "demo", "--problem", "0", "--confirm-restored", strings.Repeat("b", 40), "--by", "Wido", "--reason", "r"},
		{"mission", "repair", "demo", "--problem", "2", "--confirm-restored", "not-a-tree", "--by", "Wido", "--reason", "r"},
		{"mission", "repair", "demo", "--problem", "2", "--accept-workspace", "--by", "Wido", "--reason", "r"},
		{"mission", "repair", "demo", "--problem", "2", "--accept-workspace", "--confirm-restored", strings.Repeat("b", 40), "--waive", "c", "--by", "Wido", "--reason", "r"},
		{"settings", "coordinator", "--declare", "--withdraw", "--by", "Wido"},
		{"settings", "coordinator", "--declare"},
		// The retired engine floor has no public spelling left to record.
		{"settings", "show", "compatibility", "--minimum-engine", strings.Repeat("a", 40), "--by", "Wido"},
		{"settings", "show", "--minimum-engine", strings.Repeat("a", 40), "--by", "Wido"},
	} {
		if code, result, ran := owner(args...); result.Outcome != intentRefused || code == 0 || ran != nil {
			t.Errorf("%v = %d %+v, owner ran %v", args, code, result, ran)
		}
	}
	// Reading needs no person: the coordinator is honestly absent. The
	// retired minimum-engine read is only an unknown setting name now.
	if code, result, ran := owner("settings", "coordinator"); code != 0 || !strings.Contains(result.Summary, "no coordinator is declared") || ran != nil {
		t.Errorf("coordinator read: %d %+v", code, result)
	}
	// settings show reads the launch settings first, and every lane resolves
	// to this bed's only runtime, fake, which the compiled table gives no
	// lane models (it is the fixture harness, not an agent a person runs).
	// The bed names them, as an installation listing fake would.
	confPath := filepath.Join(b.root(), "metasystem.conf")
	conf, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []string{"build", "critique", "design", "read"} {
		conf = append(conf, "launch."+lane+".model.fake=fake-model\n"...)
	}
	if err := os.WriteFile(confPath, conf, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := owner("settings", "show", "compatibility"); code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "there is no setting compatibility") {
		t.Errorf("retired compatibility read: %d %+v", code, result)
	}
	if code, result, _ := owner("goal", "sync", "--recover"); code != 0 || !strings.Contains(result.Summary, "whole installation") {
		t.Errorf("journal recovery names its scope: %d %+v", code, result)
	}
	// The preview runs no owner; this bed has no published base, which it
	// reports rather than reading as "no edits".
	if code, result, ran := owner("goal", "sync"); code == 0 || result.Outcome != intentFailed || ran != nil || !strings.Contains(result.Summary, "published base") {
		t.Errorf("goal sync preview: %d %+v %v", code, result, ran)
	}
	// The upgrade shows the digest to review, and runs only on it.
	legacy := []byte("# Goals\n")
	b.writeFile(filepath.Join(state, "plans", "goals.md"), string(legacy))
	digest := goal.SourceDigestOf(legacy)
	if _, result, ran := owner("goal", "sync", "--upgrade", "--by", "Wido"); result.Outcome != intentRefused || ran != nil || !strings.Contains(resultLine2(result), digest) {
		t.Errorf("bare upgrade: %+v %v", result, ran)
	}
	if _, result, ran := owner("goal", "sync", "--upgrade", "--by", "Wido", "--source-digest", strings.Repeat("c", 64)); result.Outcome != intentRefused || ran != nil {
		t.Errorf("stale digest: %+v %v", result, ran)
	}
	if _, result, ran := owner("goal", "sync", "--upgrade", "--by", "Wido", "--source-digest", digest, "--amendments", "amend.md", "--sync-mode", "local"); result.Outcome != intentRefused ||
		!strings.Contains(strings.Join(ran, " "), "goal migrate --root") || !strings.Contains(strings.Join(ran, " "), "--source-digest "+digest+" --by Wido --manifest") {
		t.Errorf("reviewed upgrade reaches the owner, whose authority refuses this caller: %+v %v", result, ran)
	}
	// A legacy installation is sent to the public upgrade, never to an
	// internal command.
	if _, result, _ := owner("goal", "list"); result.Outcome != intentRefused || !strings.Contains(resultLine2(result), "metasystem goal sync --upgrade") || strings.Contains(resultLine2(result), "internal") {
		t.Errorf("legacy ledger remedy: %+v", result)
	}
}

func slicesHasPrefix(values, prefix []string) bool {
	if len(values) < len(prefix) {
		return false
	}
	for index := range prefix {
		if values[index] != prefix[index] {
			return false
		}
	}
	return true
}

// TestIntentSplitHelpExampleParses: the example split plan in help split is
// accepted, as printed, by the split owner's own closed-grammar parser.
func TestIntentSplitHelpExampleParses(t *testing.T) {
	t.Parallel()
	command, _ := findIntentCommand("goal split")
	var plan []string
	for _, line := range command.details {
		if strings.HasPrefix(line, "  #") || strings.HasPrefix(line, "  -") {
			plan = append(plan, strings.TrimPrefix(line, "  "))
		}
	}
	members, err := goal.ParseMemberDraft([]byte(strings.Join(plan, "\n")+"\n"), "big-goal")
	if err != nil || len(members) != 2 || members[1].Blocked[0] != "first" || len(members[1].Labels) != 2 {
		t.Fatalf("help split example = %+v, %v\n%s", members, err, strings.Join(plan, "\n"))
	}
}

// TestIntentLandException: an exceptional landing computes the candidate
// through the landing owner seam, records the person's exception only
// through the real carry owner (the built engine, which refuses this
// non-human caller), and delivers through the carried landing seam;
// an interrupted delivery continues under the same exception.
func TestIntentLandException(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	engine := intentTestEngine(t)
	b.owners.executable = func() (string, error) { return engine, nil }
	candidate := strings.Repeat("c", 40)
	b.owners.landCandidate = func(args []string) (goalBranchLandPrepOutcome, int, error) {
		return goalBranchLandPrepOutcome{Result: branch.LandResult{Candidate: candidate}}, 0, nil
	}
	landFails := true
	b.handler = runIntentOwnerProcess
	b.owners.landCarried = func(landpath.LandRequest, func(string, string) error, carriedRelease) intentProcessResult {
		if landFails {
			return intentProcessResult{code: 1, stderr: []byte("land: the proof token was not reserved\n")}
		}
		return intentProcessResult{stdout: []byte(`{"landed":true}`)}
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.work.git = func(dir string, args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "rev-parse" && args[1] == candidate+"^{tree}" {
			return []byte(strings.Repeat("e", 40) + "\n"), nil
		}
		return nil, os.ErrNotExist
	}
	run := func(args ...string) (int, intentResult) { return b.runJSON(owners, args...) }
	b.writeJob(map[string]any{"jobId": "j1", "role": "implementer", "status": "completed", "round": 1, "goalId": bedGoal})
	for _, args := range [][]string{
		{"work", "land", bedGoal, "--exception", "group:unit", "--reason", "flaky host"},
		{"work", "land", bedGoal, "--exception", "group:unit", "--reason", "r", "--by", "Wido", "--transfer"},
		{"work", "land", bedGoal, "--exception", "group:unit", "--reason", "r", "--by", "Wido", "--expires", "5h"},
		{"work", "land", bedGoal, "--using-exception", "op-1", "--reason", "r"},
		{"work", "land", bedGoal, "--queue-only", "--exception", "group:unit"},
		{"work", "land", bedGoal, "--by", "Wido"},
		{"work", "land", "j2:j1", "--exception", "group:unit"},
	} {
		if code, result := run(args...); result.Outcome != intentRefused || code != 2 || len(b.calls) != 0 {
			t.Fatalf("%v = %d %+v calls=%v", args, code, result, b.calls)
		}
	}
	// Fetch-before-word (plans/intent-carried-delivery-design.md, final
	// order): the adapter resolves the main checkout and refreshes the code
	// and goal ledger before any carry owner runs. This stub bed has no main
	// checkout, so both a fresh exception and a named one refuse with no
	// owner run and nothing recorded. The real owner routing (goal fetch,
	// then the carry owner's own person decision, then the carried landing
	// of the staged candidate) is proved on real Git by TestIntentCarriedCarryOwnerDecidesThePerson,
	// TestIntentCarriedGoalDeliveryGitAdapter and TestIntentCarriedReplay.
	for _, args := range [][]string{
		{"work", "land", bedGoal, "--exception", "group:unit", "--reason", "flaky host", "--by", "Wido"},
		{"work", "land", bedGoal, "--using-exception", "op-1"},
	} {
		if code, result := run(args...); result.Outcome != intentRefused || code == 0 || len(b.calls) != 0 || !strings.Contains(result.Summary, "nothing was recorded") {
			t.Fatalf("%v before the refresh = %d %+v calls=%v", args, code, result, b.calls)
		}
	}
	_ = landFails
}

// TestIntentOwnerRejectionReasonIsKept: an owner that refuses with a JSON
// result on stdout and nothing on stderr keeps its own reason as the public
// summary; an owner that gives no reason is reported as such, never as a
// refusal that changed nothing.
func TestIntentOwnerRejectionReasonIsKept(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, stdout, stderr, want string
	}{
		{name: "top-level", stdout: `{"outcome":"rejected","reason":"no refresh is pending"}`, want: "no refresh is pending"},
		{name: "nested", stdout: `{"outcome":"refused","refusal":{"code":"X","detail":"the ledger moved"}}`, want: "the ledger moved"},
		{name: "reason-over-stderr", stdout: `{"reason":"the owner's reason"}`, stderr: "usage noise\n", want: "the owner's reason"},
		{name: "stderr", stdout: `{"outcome":"rejected"}`, stderr: "refused: lock held\n", want: "refused: lock held"},
		{name: "text", stdout: "== STEP: one\n!! STEP FAILED: two\n", want: "!! STEP FAILED: two"},
		{name: "silent", want: "the command stopped (exit 3) without saying why; --json shows its output"},
	} {
		result := ownerVerbResult(intentProcessResult{code: 3, stdout: []byte(test.stdout), stderr: []byte(test.stderr)}, nil, "done", nil)
		if result.Outcome != intentRefused || result.code != 3 || result.Summary != test.want || strings.Contains(result.Summary, "changed nothing") {
			t.Fatalf("%s: %+v", test.name, result)
		}
	}
}
