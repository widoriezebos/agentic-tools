package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

var (
	intentEngineOnce sync.Once
	intentEnginePath string
	intentEngineErr  error
)

// intentTestEngine is this package built once as the real engine the
// administrative choices run as their owner process.
func intentTestEngine(t *testing.T) string {
	t.Helper()
	intentEngineOnce.Do(func() {
		directory, err := os.MkdirTemp("", "intent-engine-")
		if err != nil {
			intentEngineErr = err
			return
		}
		intentEnginePath = filepath.Join(directory, "metasystem")
		if output, err := exec.Command("go", "build", "-o", intentEnginePath, ".").CombinedOutput(); err != nil {
			intentEngineErr = err
			t.Logf("%s", output)
		}
	})
	if intentEngineErr != nil {
		t.Fatalf("build engine: %v", intentEngineErr)
	}
	return intentEnginePath
}

// TestIntentRepairAuthority: every administrative choice reaches its real
// owner verb (this package's engine, as its own process) with its exact
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
		{[]string{"repair", "goals", "--accept-remote-history", "--by", "Wido"}, []string{"goal", "repair", "--accept-remote", "--by", "Wido", "--root"}, "git fetch"},
		{[]string{"settings", "coordinator", "--declare", "--by", "Wido"}, []string{"brain", "declare", "--root"}, "is a human act; run it from an agent-free terminal"},
		{[]string{"settings", "coordinator", "--withdraw", "--by", "Wido"}, []string{"brain", "withdraw", "--root"}, "is a human act; run it from an agent-free terminal"},
		{[]string{"repair", "mission", "demo", "--problem", "2", "--confirm-restored", strings.Repeat("b", 40), "--by", "Wido", "--reason", "restored"}, []string{"mission", "resolve-taint", "--root"}, "human-reserved act"},
		{[]string{"repair", "mission", "demo", "--problem", "2", "--accept-workspace", "--waive", "claim-a", "--by", "Wido", "--reason", "accepted"}, []string{"mission", "resolve-taint", "--root"}, "human-reserved act"},
	} {
		code, result, ran := owner(row.args...)
		if result.Outcome != intentRefused || code == 0 || len(ran) < len(row.verb)+1 || !slicesHasPrefix(ran[1:], row.verb) || !strings.Contains(result.Summary, row.reason) {
			t.Errorf("%v without a person's proof = %d %+v (owner argv %v)", row.args, code, result, ran)
		}
	}
	if _, result, ran := owner("repair", "mission", "demo", "--problem", "2", "--accept-workspace", "--waive", "claim-a", "--by", "Wido", "--reason", "accepted"); !strings.Contains(strings.Join(ran, " "), "--adopt --waives claim-a --by Wido --reason accepted") {
		t.Errorf("accept-workspace inputs: %v %+v", ran, result)
	}
	// Refused before any owner runs.
	for _, args := range [][]string{
		{"repair", "goals", "--accept-edits", "--refresh", "--by", "Wido"},
		{"repair", "goals", "--accept-edits"},
		{"repair", "goals", "--refresh", "--by", "Wido"},
		{"repair", "goals", "--upgrade", "--by", "Wido", "--sync-mode", "sideways"},
		{"repair", "mission", "demo", "--problem", "0", "--confirm-restored", strings.Repeat("b", 40), "--by", "Wido", "--reason", "r"},
		{"repair", "mission", "demo", "--problem", "2", "--confirm-restored", "not-a-tree", "--by", "Wido", "--reason", "r"},
		{"repair", "mission", "demo", "--problem", "2", "--accept-workspace", "--by", "Wido", "--reason", "r"},
		{"repair", "mission", "demo", "--problem", "2", "--accept-workspace", "--confirm-restored", strings.Repeat("b", 40), "--waive", "c", "--by", "Wido", "--reason", "r"},
		{"settings", "coordinator", "--declare", "--withdraw", "--by", "Wido"},
		{"settings", "coordinator", "--declare"},
		// The retired engine floor has no public spelling left to record.
		{"settings", "compatibility", "--minimum-engine", strings.Repeat("a", 40), "--by", "Wido"},
		{"settings", "--minimum-engine", strings.Repeat("a", 40), "--by", "Wido"},
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
	if code, result, _ := owner("settings", "compatibility"); code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "there is no setting compatibility") {
		t.Errorf("retired compatibility read: %d %+v", code, result)
	}
	if code, result, _ := owner("repair", "goals"); code != 0 || !strings.Contains(result.Summary, "whole installation") {
		t.Errorf("journal recovery names its scope: %d %+v", code, result)
	}
	// The upgrade shows the digest to review, and runs only on it.
	legacy := []byte("# Goals\n")
	b.writeFile(filepath.Join(state, "plans", "goals.md"), string(legacy))
	digest := goal.SourceDigestOf(legacy)
	if _, result, ran := owner("repair", "goals", "--upgrade", "--by", "Wido"); result.Outcome != intentRefused || ran != nil || !strings.Contains(result.Decision, digest) {
		t.Errorf("bare upgrade: %+v %v", result, ran)
	}
	if _, result, ran := owner("repair", "goals", "--upgrade", "--by", "Wido", "--source-digest", strings.Repeat("c", 64)); result.Outcome != intentRefused || ran != nil {
		t.Errorf("stale digest: %+v %v", result, ran)
	}
	if _, result, ran := owner("repair", "goals", "--upgrade", "--by", "Wido", "--source-digest", digest, "--amendments", "amend.md", "--sync-mode", "local"); result.Outcome != intentRefused ||
		!strings.Contains(strings.Join(ran, " "), "goal migrate --root") || !strings.Contains(strings.Join(ran, " "), "--source-digest "+digest+" --by Wido --manifest") {
		t.Errorf("reviewed upgrade reaches the owner, whose authority refuses this caller: %+v %v", result, ran)
	}
	// A legacy installation is sent to the public upgrade, never to an
	// internal command.
	if _, result, _ := owner("goals"); result.Outcome != intentRefused || !strings.Contains(result.Decision, "metasystem repair goals --upgrade") || strings.Contains(result.Decision, "internal") {
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
	command, _ := findIntentCommand("split")
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
// non-human caller), and delivers through land.sh --carried (process seam);
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
	b.handler = func(process intentProcess) intentProcessResult {
		if filepath.Base(process.argv[0]) == "land.sh" {
			if landFails {
				return intentProcessResult{code: 1, stderr: []byte("land.sh: the proof token was not reserved\n")}
			}
			return intentProcessResult{stdout: []byte(`{"landed":true}`)}
		}
		return runIntentOwnerProcess(process)
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
	for _, args := range [][]string{
		{"land", bedGoal, "--exception", "group:unit", "--reason", "flaky host"},
		{"land", bedGoal, "--exception", "group:unit", "--reason", "r", "--by", "Wido", "--transfer"},
		{"land", bedGoal, "--exception", "group:unit", "--reason", "r", "--by", "Wido", "--expires", "5h"},
		{"land", bedGoal, "--using-exception", "op-1", "--reason", "r"},
		{"land", bedGoal, "--queue-only", "--exception", "group:unit"},
		{"land", bedGoal, "--by", "Wido"},
		{"land", "job", "j1", "--exception", "group:unit"},
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
	// then the carry owner's own person decision, then land.sh --carried
	// --staged-only) is proved on real Git by TestIntentCarriedCarryOwnerDecidesThePerson,
	// TestIntentCarriedGoalDeliveryGitAdapter and TestIntentCarriedReplay.
	for _, args := range [][]string{
		{"land", bedGoal, "--exception", "group:unit", "--reason", "flaky host", "--by", "Wido"},
		{"land", bedGoal, "--using-exception", "op-1"},
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
		{name: "silent", want: "the owner exited 3 without a reason; its output is under owner"},
	} {
		result := ownerVerbResult(intentProcessResult{code: 3, stdout: []byte(test.stdout), stderr: []byte(test.stderr)}, nil, "done", nil)
		if result.Outcome != intentRefused || result.code != 3 || result.Summary != test.want || strings.Contains(result.Summary, "changed nothing") {
			t.Fatalf("%s: %+v", test.name, result)
		}
	}
}
