package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// migratedHumanTerminalRoot is one strictly synthetic checkout whose ledger
// the real migration owner published under a person at a terminal. Only the
// terminal fact is simulated, through the existing fixture table in an exact
// metasystem.runtimes=fake root; pids names every process whose caller the
// owners classify (this process for an owner child, its parent for an
// in-process owner).
func migratedHumanTerminalRoot(t *testing.T, machine string, pids ...int) string {
	t.Helper()
	root, digest := legacyHumanTerminalRoot(t, machine, pids...)
	if stderr, code := captureStderr(t, func() int {
		return runGoalMigrate([]string{"--root", root, "--source-digest", digest, "--sync-mode", "local",
			"--identity", "01J5XM00000000000000000000", "--by", "fixture-human"})
	}); code != 0 {
		t.Fatalf("real migration of the synthetic root failed: code=%d stderr=%s", code, stderr)
	}
	materializeLocalLedger(t, root)
	return root
}

// legacyHumanTerminalRoot is the same synthetic checkout before its
// migration: a committed legacy goals file and its accepted baseline, with
// the digest a person reviews.
func legacyHumanTerminalRoot(t *testing.T, machine string, pids ...int) (string, string) {
	t.Helper()
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "coordinator human fixture")
	runReceiptGit(t, root, "config", "user.email", "coordinator-human@example.invalid")
	runReceiptGit(t, root, "config", "metasystem.goal.machine", machine)
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	for _, directory := range []string{"bin", "scripts/agents", "plans"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The ledger fence enrols the shipped production guard; its caller
	// question is the one simulated fact, answered as the same person.
	guardBytes, err := os.ReadFile("../../scripts/agents/pre-commit-guard.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh"), guardBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"if [[ ${1:-} == lease && ${2:-} == classify ]]; then printf '%s\\n' '{\"class\":\"HUMAN\"}'; exit 0; fi\n" +
		"if [[ ${1:-} == json && ${2:-} == get ]]; then printf '%s\\n' HUMAN; exit 0; fi\nexit 1\n"
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "# Goals\n\n## Goal-free: declared 2026-09-09T00:00:00Z by human over " + strings.Repeat("ab", 32) + "\n"
	digest := bytesSHA256([]byte(legacy))
	baseline, err := json.Marshal(map[string]any{"schemaVersion": 1, "ledger": legacy, "sha256": digest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals-accepted.json"), baseline, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "human initialization")
	entries := make([]string, 0, len(pids))
	for _, pid := range pids {
		entries = append(entries, fmt.Sprintf(`"%d": {"terminal": true}`, pid))
	}
	table := filepath.Join(t.TempDir(), "terminal-table.json")
	if err := os.WriteFile(table, []byte("{"+strings.Join(entries, ", ")+"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	return root, digest
}

func materializeLocalLedger(t *testing.T, root string) {
	t.Helper()
	// A synced checkout carries the accepted ledger's files, not the legacy
	// file the migration replaced.
	backlog := runReceiptGit(t, root, "show", goal.LocalLedgerBranch+":plans/goals/backlog.md")
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), []byte(backlog+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
}

// TestIntentCoordinatorGitAdapterDeclaresAndWithdrawsThroughTheBrainOwner:
// settings coordinator --declare, bare and --withdraw run the real brain
// owner, freshly built, as its own process. The owner classifies its caller
// by walking the child's ancestry, and here that caller is a person at a
// terminal (the existing fixture table, in an exact fake-runtime root), so
// the owner's own authority admits the act and its real transition is
// observed: the declaration record and the per-run registry entry, then
// their withdrawal. Git is the claim because the owner process reads the
// migrated ledger identity and the claim projection from the Git ledger
// itself; no per-test fake repository crosses that process boundary.
func TestIntentCoordinatorGitAdapterDeclaresAndWithdrawsThroughTheBrainOwner(t *testing.T) {
	engine := intentTestEngine(t)
	root := migratedHumanTerminalRoot(t, "coordinator-machine", os.Getpid(), os.Getppid())
	registry := t.TempDir()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", registry)
	ledger := goal.ExistingLedgerIdentity(root)
	if ledger != "01J5XM00000000000000000000" {
		t.Fatalf("migrated ledger identity = %q", ledger)
	}

	var calls [][]string
	delivery := &intentDeliveryOwners{
		process: func(process intentProcess) intentProcessResult {
			calls = append(calls, process.argv)
			return runIntentOwnerProcess(process)
		},
		executable: func() (string, error) { return engine, nil },
	}
	owners := defaultIntentOwners()
	owners.resolver, owners.delivery = stateroot.NewResolver(fakeTop(root), noExecutable), delivery
	settings, _ := findIntentCommand("settings")
	run := func(args ...string) (int, intentResult) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(settings, append(args, "--json"), &stdout, &stderr, root, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	lastOwner := func(before int) []string {
		t.Helper()
		if len(calls) != before+1 {
			t.Fatalf("owner runs = %v, want exactly one more than %d", calls, before)
		}
		return calls[len(calls)-1]
	}

	if code, result := run("coordinator"); code != 0 || !strings.Contains(result.Summary, "no coordinator is declared") || len(calls) != 0 {
		t.Fatalf("undeclared read: %d %+v %v", code, result, calls)
	}

	// Control: without the terminal fact the same owner, argv
	// and root refuse on authority alone and declare nothing.
	humanTable := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	agentTable := filepath.Join(t.TempDir(), "no-terminal-table.json")
	if err := os.WriteFile(agentTable, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", agentTable)
	code, result := run("coordinator", "--declare", "--by", "Wido")
	lastOwner(0)
	if result.Outcome != intentRefused || code == 0 || result.Summary != "brain declare is a human act; run it from an agent-free terminal" ||
		brain.Read(root, ledger).State != brain.Undeclared {
		t.Fatalf("declare without the terminal fact: %d %+v", code, result)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", humanTable)
	calls = nil

	code, result = run("coordinator", "--declare", "--by", "Wido")
	argv := lastOwner(0)
	if want := []string{engine, "brain", "declare", "--root", root, "--by", "Wido"}; !slicesEqual(argv, want) {
		t.Fatalf("declare owner argv = %v, want %v", argv, want)
	}
	expectOutcome(t, "declare", code, result, intentConfirmed)
	if result.Summary != "this checkout is declared its ledger's coordinator" {
		t.Fatalf("declare summary: %+v", result)
	}
	state := brain.Read(root, ledger)
	if state.State != brain.Declared || state.Record == nil || state.Record.DeclaredBy != "Wido" ||
		state.Record.Ledger != ledger || state.Record.Machine != "coordinator-machine" {
		t.Fatalf("the owner's actual declaration: %+v", state)
	}
	if entries := registryFiles(t, registry); len(entries) == 0 {
		t.Fatal("the owner registered no declaration in the isolated registry home")
	}

	if code, result := run("coordinator"); code != 0 || result.Outcome != intentConfirmed ||
		!strings.Contains(result.Summary, "this checkout is the coordinator of ledger "+ledger+", declared by Wido") || len(calls) != 1 {
		t.Fatalf("declared read: %d %+v", code, result)
	}

	// A second declaration is the owner's own refusal, not the adapter's.
	code, result = run("coordinator", "--declare", "--by", "Wido")
	lastOwner(1)
	if result.Outcome != intentRefused || code == 0 || !strings.Contains(result.Summary, "already the brain of ledger "+ledger) {
		t.Fatalf("repeat declare: %d %+v", code, result)
	}

	code, result = run("coordinator", "--withdraw", "--by", "Wido")
	argv = lastOwner(2)
	if want := []string{engine, "brain", "withdraw", "--root", root, "--by", "Wido"}; !slicesEqual(argv, want) {
		t.Fatalf("withdraw owner argv = %v, want %v", argv, want)
	}
	expectOutcome(t, "withdraw", code, result, intentConfirmed)
	if state := brain.Read(root, ledger); state.State != brain.Undeclared {
		t.Fatalf("the owner's actual withdrawal left %+v", state)
	}
	if code, result := run("coordinator"); code != 0 || !strings.Contains(result.Summary, "no coordinator is declared") {
		t.Fatalf("read after withdrawal: %d %+v", code, result)
	}
}

func slicesEqual(left, right []string) bool {
	return len(left) == len(right) && slicesHasPrefix(left, right)
}

func registryFiles(t *testing.T, home string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files = append(files, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

// runIntentRealOwner runs one public command from root, its owner verbs
// being the freshly built engine as their own processes; every owner argv
// is recorded exactly as the adapter composed it.
func runIntentRealOwner(t *testing.T, root string, calls *[][]string, args ...string) (int, intentResult) {
	t.Helper()
	engine := intentTestEngine(t)
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	owners.delivery = &intentDeliveryOwners{
		process: func(process intentProcess) intentProcessResult {
			*calls = append(*calls, append([]string(nil), process.argv...))
			return runIntentOwnerProcess(process)
		},
		executable: func() (string, error) { return engine, nil },
	}
	command, _ := findIntentCommand(args[0])
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append(args[1:], "--json"), &stdout, &stderr, root, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
	}
	return code, result
}

// TestIntentRepairGoalsGitAdapterAcceptsRewoundRemoteHistory: repair goals
// --accept-remote-history --by NAME runs the real goal repair owner as its
// own process, with exactly the adapter's argv. The canonical ledger was
// rewound behind this clone's accepted tip; outside a declared coordinator
// the owner admits the named person, fetches the remote and moves only the
// accepted ref to the fetched tip: the canonical branch is not rewound or
// pushed, and local main is untouched. Git is the claim because fetch and
// ref movement happen inside the owner process.
func TestIntentRepairGoalsGitAdapterAcceptsRewoundRemoteHistory(t *testing.T) {
	t.Parallel()
	root, upstream, base := goalBranchMainCLIFixtureBelow(t, "m1", ".")
	goalSyncMutationGit(t, root, "commit", "-q", "--allow-empty", "-m", "later ledger history")
	later := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "push", "-q", "upstream", "HEAD:main")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, later)
	goalSyncMutationGit(t, upstream, "update-ref", "refs/heads/main", base)

	var calls [][]string
	code, result := runIntentRealOwner(t, root, &calls, "repair", "goals", "--accept-remote-history", "--by", "Wido")
	if len(calls) != 1 || !slicesEqual(calls[0][1:], []string{"goal", "repair", "--accept-remote", "--by", "Wido", "--root", root}) {
		t.Fatalf("owner argv = %v", calls)
	}
	expectOutcome(t, "accept remote history", code, result, intentConfirmed)
	owner := fmt.Sprint(result.Data.(map[string]any)["owner"])
	if !strings.Contains(owner, "advanced=true tip="+base) {
		t.Fatalf("the owner's own report: %+v", result)
	}
	if got := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef); got != base {
		t.Fatalf("accepted ref = %s, want the fetched tip %s (was %s)", got, base, later)
	}
	if got := goalSyncMutationGit(t, upstream, "rev-parse", "refs/heads/main"); got != base {
		t.Fatalf("canonical main moved to %s", got)
	}
	if got := goalSyncMutationGit(t, root, "rev-parse", "refs/heads/main"); got != later {
		t.Fatalf("local main moved to %s", got)
	}
}

// TestIntentRepairGoalsGitAdapterAcceptsHandEditsThroughReconcile: repair
// goals --accept-edits --by NAME runs the real goal reconcile owner as its
// own process with the adapter's exact argv. A hand edit of a goal page in
// the checkout is republished to the accepted ledger as one edit on that
// goal; a repeat finds nothing new. Git is the claim because the owner
// process reads its base and publishes the ledger commit itself.
func TestIntentRepairGoalsGitAdapterAcceptsHandEditsThroughReconcile(t *testing.T) {
	root, _, base := goalBranchMainCLIFixtureBelow(t, "m1", ".")
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	page := filepath.Join(root, "plans", "goals", "standing-validation.md")
	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	file.NextStep = "Hand-edited next step, accepted by a person."
	writeTestingFixtureFile(t, page, goal.RenderFile(file), 0o644)

	var calls [][]string
	code, result := runIntentRealOwner(t, root, &calls, "repair", "goals", "--accept-edits", "--by", "Wido")
	if len(calls) != 1 || !slicesEqual(calls[0][1:], []string{"goal", "reconcile", "--root", root, "--by", "Wido"}) {
		t.Fatalf("owner argv = %v", calls)
	}
	expectOutcome(t, "accept edits", code, result, intentConfirmed)
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	published := projection.Tree.Live["standing-validation"]
	if projection.Tip == base || published == nil || published.NextStep != file.NextStep {
		t.Fatalf("the owner's publication: tip %s (base %s) goal %+v", projection.Tip, base, published)
	}
	last := published.History[len(published.History)-1]
	owner := result.Data.(map[string]any)["owner"].(map[string]any)
	if last.Verb != "edit" || last.Actor != "human:Wido" || owner["tip"] != projection.Tip || owner["rows"] != float64(1) {
		t.Fatalf("the republished edit: history %+v, owner %+v, tip %s", last, owner, projection.Tip)
	}
	// Nothing new is reconciled twice.
	code, result = runIntentRealOwner(t, root, &calls, "repair", "goals", "--accept-edits", "--by", "Wido")
	if owner := result.Data.(map[string]any)["owner"].(map[string]any); code != 0 || owner["rows"] != float64(0) {
		t.Fatalf("repeat accept-edits: %d %+v", code, result)
	}
	if again, err := goal.Project(endpoint, false, time.Now().UTC()); err != nil || again.Tip != projection.Tip {
		t.Fatalf("a clean repeat published: %s %v", again.Tip, err)
	}

	// repair goals --refresh completes a refresh that died after its
	// publication: the page still holds the hand edit as it was before the
	// publication synthesized its revision and history, and the base says
	// refreshDue with that durably captured snapshot. It takes no person and
	// reads no edit as authority.
	writeTestingFixtureFile(t, page, goal.RenderFile(file), 0o644)
	snapshot, err := goal.CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.WriteBase(root, goal.BaseRecord{Commit: projection.Tip, WrittenAt: "2026-09-26T00:00:00Z", RefreshDue: true, Snapshot: snapshot.Files}); err != nil {
		t.Fatal(err)
	}
	before := len(calls)
	code, result = runIntentRealOwner(t, root, &calls, "repair", "goals", "--refresh")
	if len(calls) != before+1 || !slicesEqual(calls[before][1:], []string{"goal", "reconcile", "--root", root, "--refresh-only"}) {
		t.Fatalf("refresh owner argv = %v", calls[before:])
	}
	expectOutcome(t, "refresh", code, result, intentConfirmed)
	restored, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("the owner did not rematerialize the page: %v", err)
	}
	if parsed, problems := goal.ParseFile(restored); len(problems) != 0 || parsed.NextStep != file.NextStep || parsed.Revision != published.Revision ||
		parsed.Revision == file.Revision || len(parsed.History) != len(published.History) {
		t.Fatalf("rematerialized page: %+v %v", parsed, problems)
	}
	if record, exists, err := goal.ReadBase(root); err != nil || !exists || record.RefreshDue || record.Commit != projection.Tip {
		t.Fatalf("base after refresh: %+v %v %v", record, exists, err)
	}
	code, result = runIntentRealOwner(t, root, &calls, "repair", "goals", "--refresh")
	if result.Outcome != intentRefused || code == 0 || !strings.Contains(result.Summary, "no refresh is pending") {
		t.Fatalf("second refresh: %d %+v", code, result)
	}
}

// TestIntentRepairGoalsGitAdapterUpgradesOnlyTheReviewedDigest: repair
// goals --upgrade runs the real goal migrate owner as its own process, only
// on the digest the person reviewed and with exactly the adapter's argv,
// and the owner publishes the native ledger while main keeps the human
// initialization commit. The owner's authority here is the named person
// and the reviewed digest: it classifies its caller only inside a declared
// coordinator checkout, and no coordinator can be declared before a ledger
// exists. Git is the claim because the owner process publishes the ledger.
func TestIntentRepairGoalsGitAdapterUpgradesOnlyTheReviewedDigest(t *testing.T) {
	root, digest := legacyHumanTerminalRoot(t, "upgrade-machine")
	mainBefore := runReceiptGit(t, root, "rev-parse", "refs/heads/main")
	var calls [][]string
	code, result := runIntentRealOwner(t, root, &calls, "repair", "goals", "--upgrade", "--by", "Wido")
	if result.Outcome != intentRefused || code == 0 || len(calls) != 0 || result.Data.(map[string]any)["sourceDigest"] != digest {
		t.Fatalf("unreviewed upgrade: %d %+v %v", code, result, calls)
	}
	code, result = runIntentRealOwner(t, root, &calls, "repair", "goals", "--upgrade", "--by", "Wido",
		"--source-digest", digest, "--sync-mode", "local", "--identity", "01J5XA00000000000000000000")
	want := []string{"goal", "migrate", "--root", root, "--source-digest", digest, "--by", "Wido", "--identity", "01J5XA00000000000000000000", "--sync-mode", "local"}
	if len(calls) != 1 || !slicesEqual(calls[0][1:], want) {
		t.Fatalf("owner argv = %v, want %v", calls, want)
	}
	expectOutcome(t, "upgrade", code, result, intentConfirmed)
	if identity := goal.ExistingLedgerIdentity(root); identity != "01J5XA00000000000000000000" {
		t.Fatalf("the owner's published ledger identity = %q", identity)
	}
	if got := runReceiptGit(t, root, "rev-parse", "refs/heads/main"); got != mainBefore {
		t.Fatalf("main moved from %s to %s", mainBefore, got)
	}
	if tip := runReceiptGit(t, root, "rev-parse", goal.LocalLedgerBranch); tip == mainBefore {
		t.Fatal("the local ledger did not advance")
	}
}

// TestIntentRepairGoalsGitAdapterAcceptsRemoteHistoryInADeclaredCoordinator:
// inside a declared coordinator checkout the repair owner carries a named
// person's word only from a person at a terminal. The declaration is the
// real brain owner's record; the terminal fact is the existing fixture
// table in an exact fake-runtime root. Without that fact the owner refuses
// the adapter's argv and moves nothing; with it the owner accepts the
// rewound remote history, moving only the accepted ref.
func TestIntentRepairGoalsGitAdapterAcceptsRemoteHistoryInADeclaredCoordinator(t *testing.T) {
	root, upstream, base := goalBranchMainCLIFixtureBelow(t, "m1", ".")
	// The fixture announced this process as a seat holding the goal; a
	// person's own shell carries no such announcement, so it is cleared
	// before the terminal fact is pinned (as the wall resolution beds do).
	announcements, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.json"))
	if len(announcements) == 0 {
		t.Fatal("the fixture announced no seat; the control below would prove nothing about it")
	}
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	stageHumanTerminal(t, root, int64(os.Getpid()))
	humanTable := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	registry := t.TempDir()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", registry)
	ledger := goal.ExistingLedgerIdentity(root)
	machine, err := goal.ResolveMachine(root)
	if ledger == "" || err != nil {
		t.Fatalf("fixture ledger %q machine %v", ledger, err)
	}
	if _, err := brain.Declare(brain.DeclareOptions{StateRoot: root, RegistryHome: registry, LedgerIdentity: ledger,
		Machine: machine, DeclaredBy: "Wido", Now: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "commit", "-q", "--allow-empty", "-m", "later ledger history")
	later := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "push", "-q", "upstream", "HEAD:main")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, later)
	goalSyncMutationGit(t, upstream, "update-ref", "refs/heads/main", base)

	emptyTable := filepath.Join(t.TempDir(), "no-terminal-table.json")
	if err := os.WriteFile(emptyTable, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", emptyTable)
	var calls [][]string
	code, result := runIntentRealOwner(t, root, &calls, "repair", "goals", "--accept-remote-history", "--by", "Wido")
	if result.Outcome != intentRefused || code == 0 || !strings.Contains(result.Summary, "the brain never carries a human's word into goal repair") ||
		goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef) != later {
		t.Fatalf("declared coordinator, no terminal fact: %d %+v", code, result)
	}

	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", humanTable)
	code, result = runIntentRealOwner(t, root, &calls, "repair", "goals", "--accept-remote-history", "--by", "Wido")
	want := []string{"goal", "repair", "--accept-remote", "--by", "Wido", "--root", root}
	if len(calls) != 2 || !slicesEqual(calls[0][1:], want) || !slicesEqual(calls[1][1:], want) {
		t.Fatalf("owner argv = %v", calls)
	}
	expectOutcome(t, "declared accept remote history", code, result, intentConfirmed)
	if got := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef); got != base {
		t.Fatalf("accepted ref = %s, want %s", got, base)
	}
	if got := goalSyncMutationGit(t, upstream, "rev-parse", "refs/heads/main"); got != base {
		t.Fatalf("canonical main moved to %s", got)
	}
	if got := goalSyncMutationGit(t, root, "rev-parse", "refs/heads/main"); got != later {
		t.Fatalf("local main moved to %s", got)
	}
	if state := brain.Read(root, ledger); state.State != brain.Declared {
		t.Fatalf("the declaration changed: %+v", state)
	}
}

// TestIntentCarriedGoalDeliveryGitAdapter drives the public land G
// --exception through the whole-owner landing bed's real candidate
// composition, the real goal carry owner as its own process (the test-only
// process seam records the adapter's exact argv, then appends the existing
// fixture enrolled-human flag and a lineage), the landing owner's staging of
// the candidate and its RECEIPT line into a main checkout that is behind
// origin, and the repository's unmodified land.sh --carried --staged-only.
// The carried commit refuses until the person's public test run proves the
// staged candidate; the shown continuation then lands it under the same
// word. Git is the claim because the candidate, the carry word and the
// carried commit are physical history.
func TestIntentCarriedGoalDeliveryGitAdapter(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	f := b.f
	mainBefore := f.remote(t, "refs/heads/main")
	code, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	carry := -1
	for index, argv := range b.calls {
		if len(argv) > 2 && argv[1] == "goal" && argv[2] == "carry" {
			carry = index
		}
	}
	if carry < 0 || !slicesHasPrefix(b.calls[carry][1:], []string{"goal", "carry", "--root", f.mainRoot, "--id", "standing-validation", "--by", "Wido", "--tree"}) ||
		slices.Contains(b.calls[carry], "--fixture-human-authority") {
		t.Fatalf("the carry owner did not run with the adapter's own argv: %v; result %+v", b.calls, result)
	}
	data, _ := result.Data.(map[string]any)
	opid, _ := data["exception"].(string)
	if opid == "" {
		t.Fatalf("no exception was recorded: %d %+v", code, result)
	}
	// Nothing proved the candidate yet: the carried commit's own battery
	// check refuses, the candidate stays staged, and the same word continues.
	if code == 0 || result.Outcome != intentPartial || len(b.lands) != 1 || !strings.Contains(string(b.lands[0].stderr), "carry-battery-unverified") {
		t.Fatalf("the unproved carried landing did not stop at its battery check: %d %+v", code, result)
	}
	if main := f.remote(t, "refs/heads/main"); main == mainBefore {
		t.Fatalf("origin main did not move with the recorded word")
	}
	code, result = b.shown(b.provePublicly(result))
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the proved carried landing did not complete: %d %+v", code, result)
	}
	if b.owner("goal", "carry") != 1 {
		t.Fatalf("the continuation recorded another exception: %v", b.calls)
	}
	main := f.remote(t, "refs/heads/main")
	carried := goalSyncMutationGit(t, f.mainRoot, "log", "--format=%H", "--fixed-strings", "--grep=Carry: "+opid, main)
	if len(strings.Fields(carried)) != 1 {
		t.Fatalf("origin main %s has %d commits carrying %s", main, len(strings.Fields(carried)), opid)
	}
	message := goalSyncMutationGit(t, f.mainRoot, "log", "-1", "--format=%B", carried)
	if !strings.Contains(message, "Landing-Provenance: carried opid="+opid) {
		t.Fatalf("carried commit %s has no carried provenance:\n%s", carried, message)
	}
	if owned := goalSyncMutationGit(t, f.mainRoot, "show", main+":metasystem/owned.go"); !strings.Contains(owned, "const Landed = 1") {
		t.Fatalf("origin main lacks the goal's product: %q", owned)
	}
	ledger := goalSyncMutationGit(t, f.mainRoot, "show", main+":metasystem/memory/receipts.log")
	rows := 0
	for _, line := range strings.Split(ledger, "\n") {
		if strings.Contains(line, "|RECEIPT|") && strings.Contains(line, "|goal=standing-validation|") {
			rows++
			if !strings.Contains(line, "outcome=reworked") || !strings.Contains(line, "verify=skipped") || !strings.Contains(line, opid) {
				t.Fatalf("the goal's receipt row is not the truthful preparation row: %s", line)
			}
		}
	}
	if rows != 1 || strings.Contains(ledger, "pending") || strings.Contains(ledger, "PENDING") || !strings.HasPrefix(ledger, "1|1970-01-01T00:00:00Z|RECEIPT|type=seed") {
		t.Fatalf("origin main's receipt ledger has %d goal rows or lost its prior rows:\n%s", rows, ledger)
	}
	// The word is consumed: the same request resumes through recovery and
	// stages nothing.
	code, result = b.land("standing-validation", "--using-exception", opid)
	data, _ = result.Data.(map[string]any)
	if consumption, _ := data["consumption"].(string); code != 0 || result.Outcome != intentConfirmed || consumption == "none" || data["staged"] != nil {
		t.Fatalf("the consumed word did not resume without staging: %d %+v", code, result)
	}
}
