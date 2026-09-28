package landpath

// The land driver scenarios of the former Bash fixture bed
// (scripts/agents/land-fixtures.sh), ported to the Go landing path with a
// scripted Git and per-test owners. Each test names the scenario it ports;
// what a scenario proved about an owner (the receipt-line decision, drift
// classification, advance, observation) stays with that owner's tests.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// TestDriverPushRetryRecoversOneMovingOriginRejection ports push-retry: a
// push rejected because origin moved is fetched, rebased, held-checked and
// re-proved, the second push lands, transport mirrors it, the landing is
// weighed once, and an ordinary landing touches no goal owner. The message
// comes from standard input and its owned file is removed.
func TestDriverPushRetryRecoversOneMovingOriginRejection(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	pushes := 0
	b.git.on("push --porcelain origin refs/heads/main:refs/heads/main", func(call GitCall) GitResult {
		pushes++
		if !driverEnvHas(call.Env, "LC_ALL=C") {
			t.Fatalf("push ran without LC_ALL=C: %v", call.Env)
		}
		if pushes == 1 {
			return GitResult{Code: 1, Stdout: []byte("To origin\n!\trefs/heads/main:refs/heads/main\t[rejected] (fetch first)\nDone\n")}
		}
		return ok("To origin\n \trefs/heads/main:refs/heads/main\th0..c1\nDone\n")
	})
	var messageFile string
	b.git.on("commit", func(call GitCall) GitResult {
		for i, arg := range call.Args {
			if arg == "-F" {
				messageFile = call.Args[i+1]
			}
		}
		return b.git.commit(call)
	})
	removed := ""
	b.owners.RemoveFile = func(path string) error {
		b.log.add("remove %s", filepath.Base(path))
		if path == messageFile {
			removed = path
			return os.Remove(path)
		}
		return nil
	}
	status := b.land(LandRequest{MessageFile: "-", Message: []byte("fixture retries a rejected push\n"), Pathspecs: []string{"payload.txt"}})
	b.expect(status, 0)
	if pushes != 2 {
		t.Fatalf("push attempts = %d, want the rejection plus one retry", pushes)
	}
	driverInOrder(t, b.stdout.String(),
		"== STEP: push origin (attempt 1 of 3)",
		"-- retryable rejection: push origin (attempt 1 of 3) (exit 1)",
		"[rejected] (fetch first)",
		"-- origin moved during push; fetching and rebasing before retry 2 of 3",
		"== STEP: fetch origin after push attempt 1",
		"== STEP: rebase onto origin/main after push attempt 1",
		"== STEP: goal held at the rebased base",
		"== STEP: verify shared testing proof after retry rebase",
		"== STEP: push origin (attempt 2 of 3)",
		"== STEP: sync transport")
	if strings.Contains(b.stdout.String(), "attempt 3 of 3") {
		t.Fatalf("a landed push was retried:\n%s", b.stdout.String())
	}
	if got := len(b.git.called("fetch --quiet origin +refs/heads/main:refs/remotes/origin/main")); got != 2 {
		t.Fatalf("fetches = %d, want the landing fetch and the retry fetch", got)
	}
	driverLogOrder(t, b.log, "advance refs/remotes/origin/main", "held base=refs/remotes/origin/main", "verify tree=t1",
		"advance refs/remotes/origin/main", "held base=refs/remotes/origin/main", "verify tree=t1", "transport main")
	if b.log.count("weight") != 1 || b.log.has("notify") || b.log.has("park") {
		t.Fatalf("ordinary landing owners: %v", b.log.calls)
	}
	if !strings.HasPrefix(b.git.message, "fixture retries a rejected push\n") {
		t.Fatalf("the standard-input message was not committed:\n%s", b.git.message)
	}
	if messageFile == "" || removed != messageFile {
		t.Fatalf("owned message %q removed %q", messageFile, removed)
	}
	if _, err := os.Stat(messageFile); !os.IsNotExist(err) {
		t.Fatalf("owned message survived: %v", err)
	}
}

// TestDriverStepFailureSurfacesTheFailingCodeAndStops ports step-failure: a
// fetch that fails with a distinctive code ends the landing with exactly
// that code, prints the step's output and one retained land log, and never
// rebases or pushes; the commit stands locally.
func TestDriverStepFailureSurfacesTheFailingCodeAndStops(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	b.git.on("fetch", func(GitCall) GitResult { return failed(73, "fixture fetch broke with exit 73\n") })
	var spills []string
	b.owners.OutputSpill = func(_, verb, ext string, data []byte) (string, error) {
		spills = append(spills, verb+"."+ext+":"+string(data))
		return "output-reference verb=" + verb + " path=artifacts/agents/output/land-1.log", nil
	}
	b.expect(b.land(LandRequest{Pathspecs: []string{"payload.txt"}, SkipTransport: true}), 73,
		"!! STEP FAILED: fetch origin (exit 73)", "fixture fetch broke with exit 73", "output-reference verb=land")
	if len(spills) != 1 || !strings.HasPrefix(spills[0], "land.log:") || !strings.Contains(spills[0], "fixture fetch broke with exit 73") {
		t.Fatalf("retained logs = %q, want exactly one land log holding the failure", spills)
	}
	if strings.Contains(b.stdout.String(), "== STEP: rebase onto origin/main") || b.log.has("advance") ||
		len(b.git.called("push")) != 0 || b.log.has("transport") {
		t.Fatalf("the chain continued after fetch failed:\n%s\n%v", b.stdout.String(), b.log.calls)
	}
	if b.git.commits != 1 {
		t.Fatalf("commits = %d, want the local commit standing", b.git.commits)
	}
}

// TestDriverNewPlanAcknowledgmentIsTheFlagNotTheEnvironment ports new-plan:
// an inherited METASYSTEM_ALLOW_NEW_PLAN is not the caller's
// acknowledgment, so the pre-commit guard's refusal reaches the caller
// verbatim at the commit step and nothing is fetched; the same staged plan
// lands when the landing itself allows a new plan.
func TestDriverNewPlanAcknowledgmentIsTheFlagNotTheEnvironment(t *testing.T) {
	t.Parallel()
	guardedCommit := func(b *bed) {
		b.git.on("commit", func(call GitCall) GitResult {
			if inherited := driverEnvNamed(call.Env, "METASYSTEM_ALLOW_NEW_PLAN"); len(inherited) > 1 ||
				len(inherited) == 1 && inherited[0] != "METASYSTEM_ALLOW_NEW_PLAN=1" {
				t.Fatalf("commit environment carries %q", inherited)
			}
			guardGit := func(call GitCall) GitResult {
				switch strings.Join(call.Args, " ") {
				case "rev-parse --show-toplevel":
					return ok(b.root + "\n")
				case "diff --cached --name-only", "diff --cached --name-only --diff-filter=AM":
					return ok("plans/new.md\n")
				case "rev-parse --verify HEAD":
					return ok("h0\n")
				case "diff --cached --name-status --diff-filter=A":
					return ok("A\tplans/new.md\n")
				}
				t.Fatalf("unscripted guard git call: %v", call.Args)
				return GitResult{}
			}
			var stdout, stderr strings.Builder
			status := Guard(GuardOwners{Git: guardGit, CallerPID: 100,
				AllowNewPlan: driverEnvHas(call.Env, "METASYSTEM_ALLOW_NEW_PLAN=1"),
				Classify:     func(string, int64) (string, error) { return "HUMAN", nil }},
				b.root, b.root, &stdout, &stderr)
			if status != 0 {
				return GitResult{Code: status, Stdout: []byte(stdout.String()), Stderr: []byte(stderr.String())}
			}
			return b.git.commit(call)
		})
	}

	b := newBed(t)
	b.owners.Environ = func() []string { return []string{"PATH=/usr/bin", "METASYSTEM_ALLOW_NEW_PLAN=1"} }
	driverPathMode(b)
	guardedCommit(b)
	b.expect(b.land(LandRequest{Pathspecs: []string{"plans/new.md"}, SkipTransport: true}), 1,
		"pre-commit guard: refusing to commit NEW plan file(s):\n  plans/new.md", "!! STEP FAILED: commit")
	if b.git.commits != 0 || strings.Contains(b.stdout.String(), "== STEP: fetch origin") || len(b.git.called("fetch")) != 0 {
		t.Fatalf("the refused commit continued:\n%s", b.stdout.String())
	}

	b = newBed(t)
	b.owners.Environ = func() []string { return []string{"PATH=/usr/bin", "METASYSTEM_ALLOW_NEW_PLAN=1"} }
	guardedCommit(b)
	b.expect(b.land(LandRequest{StagedOnly: true, AllowNewPlan: true, SkipTransport: true}), 0)
	if b.git.commits != 1 || len(b.git.called("push --porcelain origin refs/heads/main:refs/heads/main")) != 1 {
		t.Fatalf("the acknowledged plan did not land: %v", b.git.calls)
	}
}

// TestDriverForwardsTheGoalToTheCommitBoundary ports goal: the goal is
// driver input carried once to the receipt line, the proof, the
// observation and the stamped Goal-Item, and wakes the goal's waiters.
func TestDriverForwardsTheGoalToTheCommitBoundary(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	observed := driverObserved(b)
	b.expect(b.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 0)
	if len(*observed) != 1 || (*observed)[0].Goal != "fx" {
		t.Fatalf("observations = %+v, want one for goal fx", *observed)
	}
	if countExact(b.git.message, "Goal-Item: fx") != 1 {
		t.Fatalf("commit message lacks one Goal-Item fx:\n%s", b.git.message)
	}
	driverLogOrder(t, b.log, "receipt-line goal=fx", "verify tree=t1 goal=fx", "observe judge= goal=fx", "verify tree=t1 goal=fx", "notify fx c1")
	if b.log.has("transport") {
		t.Fatalf("--skip-transport mirrored transport: %v", b.log.calls)
	}
}

// TestDriverReceiptLineRefusalGivesTheIndexBack ports receipt-line: a code
// landing whose staged ledger appends no RECEIPT line for its goal is
// refused with the owner's words, commits nothing, and unstages what path
// mode staged; the retry with the ledger named lands both together, and a
// landing the owner exempts lands without a line. Which landings need a
// line is the owner's decision (internal/landing TestReceiptLine*).
func TestDriverReceiptLineRefusalGivesTheIndexBack(t *testing.T) {
	t.Parallel()
	const detail = "the landing changes code (payload.txt) but its staged memory/receipts.log appends no RECEIPT line for goal fx; " +
		`write the line with metasystem receipt add --type implement --outcome shipped --goal fx --built-by coordinator --note "<what landed and how it was verified>" and include memory/receipts.log in the landing`
	decide := func(b *bed, staged *[]string) {
		b.owners.ReceiptLine = func(_, tree, goal, directFix string) (ReceiptDecision, error) {
			b.log.add("receipt-line tree=%s goal=%s", tree, goal)
			for _, path := range *staged {
				if path == "memory/receipts.log" || strings.HasPrefix(path, "plans/") {
					return ReceiptDecision{Encoded: `{"outcome":"appended"}`}, nil
				}
			}
			return ReceiptDecision{Refused: true, Detail: detail, Encoded: `{"outcome":"refused"}`}, nil
		}
	}
	for _, c := range []struct {
		name  string
		paths []string
	}{{"code without its line", []string{"payload.txt"}}} {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			driverPathMode(b)
			decide(b, &c.paths)
			b.expect(b.land(LandRequest{Pathspecs: c.paths, Goal: "fx", GoalSet: true, SkipTransport: true}), 2,
				"land refused: "+detail, "!! STEP FAILED: receipt line for the landing (exit 2)")
			if b.git.commits != 0 || len(b.git.called("reset -q -- payload.txt")) != 1 {
				t.Fatalf("refused landing: commits=%d calls=%v", b.git.commits, b.git.calls)
			}
			if !b.log.has("receipt-line tree=t1 goal=fx") {
				t.Fatalf("the owner was not asked for the staged project tree: %v", b.log.calls)
			}
		})
	}
	t.Run("staged set is left staged", func(t *testing.T) {
		b := newBed(t)
		staged := []string{"payload.txt"}
		decide(b, &staged)
		b.expect(b.land(LandRequest{StagedOnly: true, Goal: "fx", GoalSet: true, SkipTransport: true}), 2, "land refused: "+detail)
		if len(b.git.called("reset")) != 0 {
			t.Fatalf("a staged-only refusal reset the caller's index: %v", b.git.calls)
		}
	})
	for _, paths := range [][]string{{"payload.txt", "memory/receipts.log"}, {"plans/existing.md"}} {
		t.Run(strings.Join(paths, "+"), func(t *testing.T) {
			b := newBed(t)
			driverPathMode(b)
			decide(b, &paths)
			b.expect(b.land(LandRequest{Pathspecs: paths, Goal: "fx", GoalSet: true, SkipTransport: true}), 0)
			driverInOrder(t, b.stdout.String(), "== STEP: receipt line for the landing", "-- ok", "== STEP: commit")
			if b.git.commits != 1 || len(b.git.called("add -- "+strings.Join(paths, " "))) != 1 || len(b.git.called("reset")) != 0 {
				t.Fatalf("landing with its line: commits=%d calls=%v", b.git.commits, b.git.calls)
			}
		})
	}
	t.Run("owner failure", func(t *testing.T) {
		b := newBed(t)
		b.owners.ReceiptLine = func(string, string, string, string) (ReceiptDecision, error) {
			return ReceiptDecision{}, fmt.Errorf("ledger unreadable")
		}
		b.expect(b.land(LandRequest{StagedOnly: true, SkipTransport: true}), 1, "landing receipt-line: ledger unreadable")
		if b.git.commits != 0 {
			t.Fatal("a landing whose receipt line could not be judged committed")
		}
	})
}

// TestDriverTierOneReceiptsTheStagedCandidateBeforeTheCommit ports tier-one:
// the declared command runs against the staged candidate (the metasystem
// subtree of the staged tree) before the commit boundary, which receives
// the direct-fix class, the root job and that exact receipt path. The
// receipt's own tree bindings are the receipt owner's (internal/landing
// TestCreateTestReceiptIgnoresLiveWorkspaceMotion).
func TestDriverTierOneReceiptsTheStagedCandidateBeforeTheCommit(t *testing.T) {
	t.Parallel()
	const command = `test "$(cat payload.txt)" = tier-one\ landing`
	b := newBed(t)
	driverPathMode(b)
	b.git.prefix = "metasystem/"
	b.git.on("rev-parse t1:metasystem", func(GitCall) GitResult { return ok("sub1\n") })
	observed := driverObserved(b)
	b.owners.TestReceipt = func(_, tree, got string, stdout, _ io.Writer) int {
		b.log.add("test-receipt tree=%s", tree)
		if got != command {
			t.Fatalf("receipt command = %q", got)
		}
		fmt.Fprintln(stdout, "receipt written")
		return 0
	}
	b.expect(b.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, DirectFix: "tier-1",
		RootJob: "tier-one-root", Tests: command, SkipTransport: true}), 0)
	driverLogOrder(t, b.log, "receipt-line", "test-receipt tree=sub1", "verify tree=t1", "observe")
	want := filepath.Join(b.root, "artifacts", "agents", "landing", "receipts", "sub1.json")
	if len(*observed) != 1 {
		t.Fatalf("observations = %+v", *observed)
	}
	request := (*observed)[0]
	if request.DirectFix != "tier-1" || request.RootJob != "tier-one-root" || request.TestReceipt != want || request.Goal != "fx" || request.Tree != "sub1" {
		t.Fatalf("commit boundary observed %+v, want tier-1, root job tier-one-root and receipt %s", request, want)
	}
	driverInOrder(t, b.stdout.String(), "== STEP: tier-1 test receipt", "-- ok", "== STEP: commit")

	b = newBed(t)
	b.owners.TestReceipt = func(_, _, _ string, stdout, _ io.Writer) int { fmt.Fprintln(stdout, "command failed"); return 5 }
	b.expect(b.land(LandRequest{StagedOnly: true, Goal: "fx", GoalSet: true, DirectFix: "tier-1", RootJob: "r", Tests: "false"}), 5,
		"!! STEP FAILED: tier-1 test receipt (exit 5)", "command failed")
	if b.git.commits != 0 {
		t.Fatal("a failed tier-1 receipt reached the commit")
	}
}

// TestDriverFullWidthChainReceiptGates ports the driver half of
// full-width-chain: conflicting receipt inputs and a full-width chain
// without its receipt are refused before any step; a receipt for another
// tree is refused before the commit; the exact candidate's receipt lands
// with the evaluator's chain provenance and pass verdict.
func TestDriverFullWidthChainReceiptGates(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.expect(b.land(LandRequest{StagedOnly: true, Chain: "full-chain", Goal: "fx", GoalSet: true, Tests: "true", TestReceipt: "receipt.json", SkipTransport: true}), 2,
		"land refused: --tests and --test-receipt cannot be combined; remove --test-receipt for a tier-1 landing, or remove --tests for a receipted chain landing\n"+Usage)
	if strings.Contains(b.stdout.String(), "== STEP:") || len(b.git.calls) != 0 {
		t.Fatalf("conflicting receipt inputs started landing work:\n%s\n%v", b.stdout.String(), b.git.calls)
	}

	b = newBed(t)
	b.owners.JobGateWidth = func(_, chain string) string {
		if chain != "full-chain" {
			t.Fatalf("gate width asked for %q", chain)
		}
		return "full"
	}
	b.expect(b.land(LandRequest{StagedOnly: true, Chain: "full-chain", Goal: "fx", GoalSet: true, SkipTransport: true}), 2,
		"land refused: chain full-chain requires sufficient schema-2 testing evidence")
	if strings.Contains(b.stdout.String(), "== STEP:") || len(b.git.calls) != 0 {
		t.Fatalf("a missing receipt reached verification:\n%s", b.stdout.String())
	}

	b = newBed(t)
	b.owners.JobGateWidth = func(string, string) string { return "full" }
	other := driverWriteReceipt(b, "other.json", `{"schemaVersion":2,"tree":"t0"}`)
	b.expect(b.land(LandRequest{StagedOnly: true, Chain: "full-chain", Goal: "fx", GoalSet: true, TestReceipt: other, SkipTransport: true}), 2,
		"land refused: the receipt at "+other+" names tree t0 but the staged candidate is t1; make the receipt against this exact candidate",
		"!! STEP FAILED: test receipt for staged candidate (exit 2)")
	if strings.Contains(b.stdout.String(), "== STEP: commit") || b.git.commits != 0 {
		t.Fatalf("a mismatched receipt reached the commit:\n%s", b.stdout.String())
	}

	b = newBed(t)
	b.owners.JobGateWidth = func(string, string) string { return "full" }
	receipt := driverWriteReceipt(b, "exact.json", `{"schemaVersion":2,"tree":"t1"}`)
	b.observed = landing.Observation{Mode: "observe", Code: "closed-chain", Provenance: "chain=full-chain change=abc certified-change=abc",
		VerdictTrailer: "pass bar=a", GoalRevision: 4}
	observed := driverObserved(b)
	b.expect(b.land(LandRequest{StagedOnly: true, Chain: "full-chain", Goal: "fx", GoalSet: true, TestReceipt: receipt, SkipTransport: true}), 0)
	if len(*observed) != 1 || (*observed)[0].Chain != "full-chain" || (*observed)[0].TestReceipt != receipt {
		t.Fatalf("commit boundary observed %+v", *observed)
	}
	for _, line := range []string{"Landing-Provenance-Verdict: pass bar=a", "Landing-Provenance: chain=full-chain change=abc certified-change=abc", "Goal-Revision: 4"} {
		if countExact(b.git.message, line) != 1 {
			t.Fatalf("landed message lacks %q:\n%s", line, b.git.message)
		}
	}
	if len(b.git.called("push --porcelain origin refs/heads/main:refs/heads/main")) != 1 {
		t.Fatalf("the receipted chain did not push: %v", b.git.calls)
	}
}

// TestDriverStagingDriftRefusesBeforeTheCommit ports the drift legs of
// full-width-chain and ledger-move-lands: whatever the drift owner reports
// after staging (an unstaged product edit, a rewritten register, an
// untracked path) refuses the landing with the owner's lines before the
// commit, and tolerated register appends land. The classification itself
// is the drift owner's (internal/landing TestWorktreeDrift).
func TestDriverStagingDriftRefusesBeforeTheCommit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, drift, text string
	}{
		{"unstaged product", "unstaged\t M\tpayload.txt\n", "land refused: unstaged changes remain after staging; transport requires a clean tree after commit\n  unstaged\t M\tpayload.txt"},
		{"rewritten register", "register-not-append\t M\tmemory/receipts.log\n", "land refused: unstaged changes remain after staging; transport requires a clean tree after commit\n  register-not-append\t M\tmemory/receipts.log"},
		{"untracked path", "untracked\t??\tnotes.txt\n", "land refused: untracked paths remain after staging; transport requires a clean tree after commit\n  untracked\t??\tnotes.txt"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			b.owners.Drift = func(_ string, requireEmpty bool, stdout, _ io.Writer) int {
				if requireEmpty {
					t.Fatal("the staging check asked for an empty index")
				}
				fmt.Fprint(stdout, c.drift)
				return 1
			}
			b.expect(b.land(LandRequest{StagedOnly: true, Chain: "full-chain-2", Goal: "fx", GoalSet: true, SkipTransport: true}), 2,
				c.text, "!! STEP FAILED: stage caller paths (exit 2)")
			if strings.Contains(b.stdout.String(), "== STEP: commit") || b.git.commits != 0 {
				t.Fatalf("drift reached the commit:\n%s", b.stdout.String())
			}
		})
	}
	b := newBed(t)
	b.owners.Drift = func(_ string, requireEmpty bool, stdout, _ io.Writer) int {
		b.log.add("drift empty=%t", requireEmpty)
		return 0
	}
	b.expect(b.land(LandRequest{StagedOnly: true, SkipTransport: true}), 0)
	driverLogOrder(t, b.log, "drift empty=false", "observe", "drift empty=true", "advance")

	b = newBed(t)
	b.owners.Drift = func(_ string, requireEmpty bool, stdout, _ io.Writer) int {
		if requireEmpty {
			fmt.Fprintln(stdout, "unstaged\t M\tpayload.txt")
			return 1
		}
		return 0
	}
	b.expect(b.land(LandRequest{StagedOnly: true, SkipTransport: true}), 1,
		"land refused: commit succeeded but the tree is not clean, so transport will not start\nunstaged\t M\tpayload.txt",
		"!! STEP FAILED: verify clean after commit (exit 1)")
	if b.git.commits != 1 || len(b.git.called("fetch")) != 0 {
		t.Fatalf("an unclean tree after commit went on: %v", b.git.calls)
	}
}

// TestDriverContendedRegisterStopsAtTheRebase ports the contended-register
// leg of full-width-chain: when advance refuses a register both sides
// changed, the landing fails at its rebase step with advance's words, the
// commit stands locally and origin is not pushed. What advance carries and
// refuses is the advance owner's (internal/landing
// TestAdvanceRefusesContendedRegister, TestAdvanceLeavesRegistersUntouched).
func TestDriverContendedRegisterStopsAtTheRebase(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	refusal := "advance refused: advance-register-contended: records/narrator-digest.log " + strings.Repeat("a", 40)
	b.owners.Advance = func(_, _ string, stdout, _ io.Writer) int { fmt.Fprintln(stdout, refusal); return 1 }
	b.expect(b.land(LandRequest{Pathspecs: []string{"memory/receipts.log"}, Goal: "fx", GoalSet: true, DirectFix: "register-carriage", SkipTransport: true}), 1,
		"!! STEP FAILED: rebase onto origin/main (exit 1)", refusal)
	driverInOrder(t, b.stdout.String(), "== STEP: commit", "== STEP: fetch origin", "== STEP: rebase onto origin/main")
	if b.git.commits != 1 || len(b.git.called("push")) != 0 || b.log.has("held") {
		t.Fatalf("contended register: commits=%d calls=%v log=%v", b.git.commits, b.git.calls, b.log.calls)
	}
}

// TestDriverBrainFenceRefusesEveryLandingAct ports brain-land-refuses: on a
// checkout declared the brain, or whose declaration is unreadable, every
// landing and every declared commit is refused with the fence's words and
// exit 2 before any lease, step or Git effect; a plain undeclared commit is
// not a landing and proceeds. The fence's own texts are the brain owner's
// (internal/brain TestFenceLandTexts).
func TestDriverBrainFenceRefusesEveryLandingAct(t *testing.T) {
	t.Parallel()
	for _, detail := range []string{
		"land refused: this checkout is declared the brain; the brain never lands",
		"this checkout's brain declaration is unreadable (not JSON); until a human repairs it nothing here dispatches, lands",
	} {
		fence := func(b *bed) {
			b.owners.BrainFence = func(_, act string) (string, error) {
				b.log.add("brain-fence act=%s", act)
				return detail, nil
			}
		}
		for name, request := range map[string]LandRequest{
			"land chain":  {Chain: "brain-root", Pathspecs: []string{"payload.txt"}},
			"land tier-1": {DirectFix: "tier-1", Pathspecs: []string{"payload.txt"}},
		} {
			t.Run(name, func(t *testing.T) {
				b := newBed(t)
				fence(b)
				b.expect(b.land(request), 2, detail)
				if len(b.git.calls) != 0 || strings.Contains(b.stdout.String(), "== STEP") || b.log.has("require-holder") || !b.log.has("brain-fence act=land") {
					t.Fatalf("a fenced landing had effects: %v %v\n%s", b.git.calls, b.log.calls, b.stdout.String())
				}
			})
		}
		for name, request := range map[string]CommitRequest{
			"commit chain":             {Chain: "brain-root"},
			"commit register-carriage": {DirectFix: "register-carriage"},
		} {
			t.Run(name, func(t *testing.T) {
				b := newBed(t)
				fence(b)
				b.expect(b.commit(request), 2, detail)
				if len(b.git.calls) != 0 || b.log.has("require-holder") || !b.log.has("brain-fence act=land") {
					t.Fatalf("a fenced commit had effects: %v %v", b.git.calls, b.log.calls)
				}
			})
		}
		t.Run("plain commit", func(t *testing.T) {
			b := newBed(t)
			fence(b)
			b.epoch = epochOf(2)
			b.expect(b.commit(CommitRequest{OwnerLineage: "fixture-lineage"}), 0)
			if b.log.has("brain-fence") || b.git.commits != 1 {
				t.Fatalf("a plain commit on the brain: %v commits=%d", b.log.calls, b.git.commits)
			}
		})
	}
	b := newBed(t)
	b.owners.BrainFence = func(string, string) (string, error) { return "", fmt.Errorf("unreadable") }
	b.expect(b.land(LandRequest{StagedOnly: true}), 1, "land refused: brain fence failed")
}

// TestDriverUndeclaredNodeLandsGoalFreeCarriage ports
// brain-absent-node-proceeds: a node that is not the brain lands a
// register-carriage record without a goal, carrying the evaluator's
// Goal-free provenance, and pushes. When a Goal-free ledger admits a
// goal-less agent landing is the evaluator's (internal/landing
// TestObservationRequiresAGoalFromANonHumanActor).
func TestDriverUndeclaredNodeLandsGoalFreeCarriage(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	b.epoch = epochOf(3)
	b.observed = landing.Observation{Mode: "observe", Code: "register-carriage",
		Provenance: "chain= direct-fix class=register-carriage change=abc goal-free", VerdictTrailer: "pass bar=register-carriage"}
	observed := driverObserved(b)
	b.expect(b.land(LandRequest{Pathspecs: []string{"records/misc/brain-absent-node.txt"}, DirectFix: "register-carriage",
		OwnerLineage: "fixture-lineage", SkipTransport: true}), 0)
	if len(*observed) != 1 || (*observed)[0].Goal != "" || (*observed)[0].DirectFix != "register-carriage" || (*observed)[0].Actor != "m1+fixture-lineage" {
		t.Fatalf("observations = %+v", *observed)
	}
	if countExact(b.git.message, "Landing-Provenance: chain= direct-fix class=register-carriage change=abc goal-free") != 1 ||
		strings.Contains(b.git.message, "Goal-Item:") {
		t.Fatalf("node landing message:\n%s", b.git.message)
	}
	if len(b.git.called("push --porcelain origin refs/heads/main:refs/heads/main")) != 1 {
		t.Fatalf("the node did not push: %v", b.git.calls)
	}
}

// TestDriverWorkspaceReceiptAcrossPeerMoves ports ledger-move-lands,
// records-move-lands and input-move-refuses: a schema-2 receipt with a
// workspace identity is accepted for a candidate whose tree moved when the
// workspace projection is unchanged (a peer's ledger-only move) or when
// retained proof verifies the candidate (a records-only move); it is
// refused with the verifier's diagnosis when the proof input moved before
// the commit, and a proof input that moves with the rebase fails the
// post-rebase proof and never reaches origin. The projection and the
// verifier are their owners' (cmd/metasystem
// TestLandingWorkspacePrintsSameIDAcrossLedgerOnlyTrees).
func TestDriverWorkspaceReceiptAcrossPeerMoves(t *testing.T) {
	t.Parallel()
	const receiptBody = `{"schemaVersion":2,"tree":"t0","workspace":{"tree":"w0"}}`
	workspaces := func(b *bed, receipt, candidate string) {
		live := b.owners.Live
		b.owners.Live = func() Judge {
			judge := live()
			judge.Workspace = func(_, tree string) (string, error) {
				b.log.add("workspace tree=%s", tree)
				if tree == "t0" {
					return receipt, nil
				}
				return candidate, nil
			}
			return judge
		}
	}
	t.Run("ledger-only move", func(t *testing.T) {
		b := newBed(t)
		workspaces(b, "w-same", "w-same")
		path := driverWriteReceipt(b, "receipt.json", receiptBody)
		observed := driverObserved(b)
		b.expect(b.land(LandRequest{StagedOnly: true, Chain: "ledger-move-chain", Goal: "fx", GoalSet: true, TestReceipt: path, SkipTransport: true}), 0)
		if b.log.count("verify ") != 2 || !b.log.has("workspace tree=t0") || !b.log.has("workspace tree=t1") {
			t.Fatalf("a ledger-only move was re-verified or not projected: %v", b.log.calls)
		}
		if len(*observed) != 1 || (*observed)[0].TestReceipt != path {
			t.Fatalf("commit boundary observed %+v", *observed)
		}
	})
	t.Run("records-only move", func(t *testing.T) {
		b := newBed(t)
		workspaces(b, "w-before", "w-after")
		path := driverWriteReceipt(b, "receipt.json", receiptBody)
		b.expect(b.land(LandRequest{StagedOnly: true, Chain: "records-move-chain", Goal: "fx", GoalSet: true, TestReceipt: path, SkipTransport: true}), 0,
			"== STEP: test receipt for staged candidate")
		if b.log.count("verify tree=t1 goal=fx") != 3 || strings.Contains(b.stdout.String()+b.stderr.String(), "proof-input-moved-after-receipt") {
			t.Fatalf("records-only move: %v\n%s", b.log.calls, b.stderr.String())
		}
	})
	t.Run("staged input moved", func(t *testing.T) {
		b := newBed(t)
		workspaces(b, "w-before", "w-after")
		b.owners.Verify = func(request VerifyRequest, stdout, _ io.Writer) int {
			for i := 1; i <= 30; i++ {
				fmt.Fprintf(stdout, "verify line %d\n", i)
			}
			fmt.Fprintln(stdout, "proof-input-moved-after-receipt: group policy-protection")
			fmt.Fprintln(stdout, "moved declared paths: payload.txt")
			return 1
		}
		path := driverWriteReceipt(b, "receipt.json", receiptBody)
		b.expect(b.land(LandRequest{StagedOnly: true, Chain: "ledger-move-chain", Goal: "fx", GoalSet: true, TestReceipt: path, SkipTransport: true}), 2,
			"land refused: the receipt at "+path+" names tree t0 but the staged candidate is t1; make the receipt against this exact candidate",
			"proof-input-moved-after-receipt: group policy-protection", "moved declared paths: payload.txt")
		if strings.Contains(b.stderr.String(), "verify line 12\n") || b.git.commits != 0 {
			t.Fatalf("more than twenty verifier lines or a commit:\n%s", b.stderr.String())
		}
	})
	t.Run("input moved with the rebase", func(t *testing.T) {
		b := newBed(t)
		path := driverWriteReceipt(b, "receipt.json", `{"schemaVersion":2,"tree":"t1","workspace":{"tree":"w1"}}`)
		rebased := false
		b.owners.Advance = func(string, string, io.Writer, io.Writer) int { rebased = true; return 0 }
		b.owners.Verify = func(_ VerifyRequest, stdout, _ io.Writer) int {
			if !rebased {
				return 0
			}
			fmt.Fprintln(stdout, "proof-input-moved-after-receipt: group policy-protection; moved declared paths: scripts/application-input.txt")
			return 1
		}
		b.expect(b.land(LandRequest{StagedOnly: true, Chain: "input-move-chain", Goal: "fx", GoalSet: true, TestReceipt: path, SkipTransport: true}), 1,
			"!! STEP FAILED: verify shared testing proof after rebase (exit 1)", "proof-input-moved-after-receipt: group policy-protection",
			"scripts/application-input.txt")
		if b.git.commits != 1 || len(b.git.called("push")) != 0 {
			t.Fatalf("a moved proof input reached origin: %v", b.git.calls)
		}
	})
}

// TestDriverLegacyReceiptIsExactOnly ports the driver half of
// receipt-cutover: a receipt without a schema-2 workspace identity binds
// the exact candidate only. An older receipt names the metasystem subtree
// and lands when it matches; after a peer move it is refused as a mismatch
// without consulting the workspace projection or the verifier. A schema-2
// receipt names the whole-project tree.
func TestDriverLegacyReceiptIsExactOnly(t *testing.T) {
	t.Parallel()
	prefixed := func(b *bed) {
		b.git.prefix = "metasystem/"
		b.git.on("rev-parse t1:metasystem", func(GitCall) GitResult { return ok("sub1\n") })
		live := b.owners.Live
		b.owners.Live = func() Judge {
			judge := live()
			judge.Workspace = func(string, string) (string, error) {
				t.Fatal("an exact-only receipt consulted the workspace projection")
				return "", nil
			}
			return judge
		}
	}
	for name, body := range map[string]string{
		"old receipt":                  `{"tree":"sub1","exitStatus":0}`,
		"schema-2 without a workspace": `{"schemaVersion":2,"tree":"t1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			b := newBed(t)
			prefixed(b)
			path := driverWriteReceipt(b, "receipt.json", body)
			b.expect(b.land(LandRequest{StagedOnly: true, Chain: "cutover-exact-chain", Goal: "fx", GoalSet: true, TestReceipt: path, SkipTransport: true}), 0)
			if b.git.commits != 1 || len(b.git.called("push --porcelain")) != 1 {
				t.Fatalf("the exact receipt did not land: %v", b.git.calls)
			}
		})
	}
	b := newBed(t)
	prefixed(b)
	b.owners.Verify = func(VerifyRequest, io.Writer, io.Writer) int {
		t.Fatal("an exact-only receipt consulted the verifier")
		return 1
	}
	path := driverWriteReceipt(b, "receipt.json", `{"tree":"sub0"}`)
	b.expect(b.land(LandRequest{StagedOnly: true, Chain: "cutover-moved-chain", Goal: "fx", GoalSet: true, TestReceipt: path, SkipTransport: true}), 2,
		"land refused: the receipt at "+path+" names tree sub0 but the staged candidate is sub1; make the receipt against this exact candidate")
	if b.git.commits != 0 {
		t.Fatal("a mismatched old receipt committed")
	}
	for name, body := range map[string]string{"missing": "", "no tree field": `{"schemaVersion":1}`} {
		t.Run(name, func(t *testing.T) {
			b := newBed(t)
			path := filepath.Join(b.root, "absent.json")
			if body != "" {
				path = driverWriteReceipt(b, "receipt.json", body)
			}
			b.expect(b.land(LandRequest{StagedOnly: true, Chain: "c1", TestReceipt: path, SkipTransport: true}), 2,
				"land refused: the receipt at "+path+" cannot be read as a landing receipt ("+name+")")
		})
	}
}
