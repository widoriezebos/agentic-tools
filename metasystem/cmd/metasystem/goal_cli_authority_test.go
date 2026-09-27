package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The goal CLI shell bed's authority scenarios (goal-cli-fixtures.sh
// wrong-terminal, proof-grades and human-lineage), ported to the Git-free
// goal CLI bed with injected process facts instead of real process trees and
// pseudo-terminals.

// gcliAuthorityJournalLineage reads the "lineage" recorded in one goal
// transaction journal of the bed's checkout.
func gcliAuthorityJournalLineage(t *testing.T, root, opid string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "goal-transactions", opid+".json"))
	if err != nil {
		t.Fatalf("read the %s journal: %v", opid, err)
	}
	match := regexp.MustCompile(`"lineage": "([^"]*)"`).FindSubmatch(data)
	if match == nil {
		t.Fatalf("the %s journal records no lineage:\n%s", opid, data)
	}
	return string(match[1])
}

// gcliAuthorityApprovalOpid is the operation id on a goal record's Approved line.
func gcliAuthorityApprovalOpid(t *testing.T, record string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^- Approved: .* opid=([^ ]+) authority=`).FindStringSubmatch(record)
	if match == nil {
		t.Fatalf("the approval recorded no operation identifier:\n%s", record)
	}
	return match[1]
}

// TestGoalCLIAuthorityHumanLineage is the human-lineage scenario: a
// fixture-authorized approval under the owner lineage journals that lineage,
// and with no owner lineage the approval takes its coordinator identity from
// the checkout's local terminal enrollment, terminal-<id>-<generation>.
func TestGoalCLIAuthorityHumanLineage(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	for _, id := range []string{"explicit-lineage-approval", "derived-lineage-approval"} {
		code, stdout, stderr := bed.public("goal", "open", id, "--origin", "human", "--by", "Wido", "--fixture-human-authority",
			"--intent", "Record an approval under a fixture coordinator.", "--next", "Compare its operation identity.",
			"--tier", "3", "--risk", "severity=3,novelty=1,exposure=1,accumulation=1", "--basis", "fixture identity comparison")
		if code != 0 {
			t.Fatalf("open %s = %d stdout=%q stderr=%q", id, code, stdout, stderr)
		}
	}

	code, stdout, stderr := bed.public("goal", "approve", "explicit-lineage-approval", "--by", "Wido", "--fixture-human-authority")
	if code != 0 {
		t.Fatalf("the fixture-lineage approval did not confirm: %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	explicit := gcliAuthorityApprovalOpid(t, bed.goalRecord("explicit-lineage-approval"))
	if got := gcliAuthorityJournalLineage(t, bed.root, explicit); got != "fixture-lineage" {
		t.Fatalf("the explicit approval journal lost fixture-lineage: %q", got)
	}

	// The exact strict enrollment shape humanauthority.ReadEnrollment reads,
	// as the shell bed wrote it, and no owner lineage.
	enrollment := `{
  "schema": 1,
  "enrolledAt": "2026-09-06T08:00:00Z",
  "generation": 7,
  "terminalId": "ttys:fixture",
  "terminalRef": {"pid": 1, "pidStartedAt": 1},
  "sessionLeaderRef": {"pid": 1, "pidStartedAt": 1}
}
`
	if err := os.WriteFile(filepath.Join(bed.root, "artifacts", "agents", "authority", "human-terminal.json"), []byte(enrollment), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.lineage = ""
	code, stdout, stderr = bed.public("goal", "approve", "derived-lineage-approval", "--by", "Wido", "--fixture-human-authority")
	if code != 0 {
		t.Fatalf("the enrollment-derived approval did not confirm: %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	derived := gcliAuthorityApprovalOpid(t, bed.goalRecord("derived-lineage-approval"))
	const derivedLineage = "terminal-ttys-fixture-7"
	sum := sha256.Sum256([]byte(derivedLineage))
	if suffix := derived[strings.LastIndex(derived, "-")+1:]; suffix != hex.EncodeToString(sum[:])[:8] {
		t.Fatalf("the derived approval operation identifier suffix %s does not match lineage hash %s", suffix, hex.EncodeToString(sum[:])[:8])
	}
	if got := gcliAuthorityJournalLineage(t, bed.root, derived); got != derivedLineage {
		t.Fatalf("the derived approval journal did not record %s: %q", derivedLineage, got)
	}
}

// gcliAuthorityEmptyFamily is a process family with nothing running, the
// scratch installation's whole inventory in the shell bed.
type gcliAuthorityEmptyFamily struct{}

func (gcliAuthorityEmptyFamily) Name() string                              { return "run" }
func (gcliAuthorityEmptyFamily) Inventory() ([]stoptransition.Item, error) { return nil, nil }
func (gcliAuthorityEmptyFamily) Stop(stoptransition.Item) (stoptransition.Outcome, error) {
	return stoptransition.Outcome{}, nil
}

// gcliAuthorityProcessCheckout is a checkout carrying its own engine, as the
// shell bed copied bin/metasystem into its clone.
func gcliAuthorityProcessCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("fixture engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestGoalCLIAuthorityWrongTerminal is the wrong-terminal scenario through
// the system stop owner: a caller classified DELEGATE is refused with exit 1
// and the process-verb grammar's two lines, and a human terminal's stop prints
// the three-line stop report; status then reports who closed the fence, when.
func TestGoalCLIAuthorityWrongTerminal(t *testing.T) {
	t.Parallel()
	checkout := gcliAuthorityProcessCheckout(t)
	top := func(string) (string, error) { return checkout, nil }
	scope, scale, code := parseProcessScopeWith("stop", []string{"--repo", checkout}, top)
	if code != 0 {
		t.Fatalf("stop scope = %d", code)
	}
	stopAt := time.Date(2026, 9, 7, 7, 40, 0, 0, time.UTC)
	const stopPid = 4242
	owners := processOwners{
		repositoryTop: top,
		transition: func(scope processScope, scale int) *stoptransition.Transition {
			transition := processTransition(scope, scale)
			transition.Families = []stoptransition.Family{gcliAuthorityEmptyFamily{}}
			transition.Now = func() time.Time { return stopAt }
			transition.Self = func() (identity.Ref, error) { return identity.Ref{Pid: stopPid, StartedAtSec: 100}, nil }
			return transition
		},
	}

	var classified []string
	owners.classify = func(repo, installation string, _ int64) (lease.Classification, error) {
		classified = append(classified, repo+"|"+installation)
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}
	if _, refusal := owners.stop(scope, scale); refusal == nil {
		t.Fatal("a non-terminal caller was allowed to stop the metasystem")
	} else {
		got := "metasystem " + publicProcessVerb(refusal.verb) + ": " + strings.TrimSuffix(refusal.sentence, ".") + ".\n" + refusal.second + "\n"
		want := "metasystem system stop: system stop is a human act at a terminal; this caller is DELEGATE.\n" +
			"at an agent-free terminal, run: metasystem system stop --repo " + checkout + "\n"
		if refusal.code != 1 || refusal.plain != "" || got != want {
			t.Fatalf("wrong-terminal refusal did not match the process-verb grammar: code=%d plain=%q\n got %q\nwant %q", refusal.code, refusal.plain, got, want)
		}
	}
	if len(classified) != 1 || classified[0] != checkout+"|"+checkout {
		t.Fatalf("the stop classified %q, want the checkout's own installation once", classified)
	}
	if _, err := os.Stat(stopfence.TransitionPath(checkout)); !os.IsNotExist(err) {
		t.Fatalf("the refused stop wrote a fence: %v", err)
	}

	owners.classify = func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassHuman}, nil
	}
	report, refusal := owners.stop(scope, scale)
	if refusal != nil {
		t.Fatalf("a human terminal's stop was refused: %+v", refusal)
	}
	want := "checkout " + checkout + "\nnothing is running\nstopped " + checkout + "; start again: metasystem system start --repo " + checkout
	if got := strings.Join(report.Lines, "\n"); report.ExitCode != 0 || got != want {
		t.Fatalf("the fixture-human stop report changed grammar: exit=%d\n got %q\nwant %q", report.ExitCode, got, want)
	}
	fence, err := stopfence.Read(checkout)
	if err != nil {
		t.Fatal(err)
	}
	status, err := owners.status(scope, scale)
	if err != nil {
		t.Fatal(err)
	}
	want = "checkout " + checkout + "\nnothing is running\nstopped since " + fence.ChangedAt + " by stop pid " + "4242" +
		"; start again: metasystem system start --repo " + checkout
	if got := strings.Join(status.Lines, "\n"); status.ExitCode != 0 || got != want || fence.By.Process.Pid != stopPid || fence.ChangedAt != stopAt.Format(time.RFC3339) {
		t.Fatalf("status did not report the fixture-human stop: fence=%+v exit=%d\n got %q\nwant %q", fence, status.ExitCode, got, want)
	}
}

// gcliAuthorityTree is one injected process table: the snapshots the
// human-authority walks read, and each process's session leader. It stands in
// for the shell bed's pseudo-terminal, its signed fake-agent holder and its
// setsid-detached headless shell.
type gcliAuthorityTree struct {
	snapshots map[int64]humanauthority.Snapshot
	leaders   map[int64]int64
}

func (tree gcliAuthorityTree) Read(pid int64) (humanauthority.Snapshot, error) {
	snapshot, ok := tree.snapshots[pid]
	if !ok {
		return humanauthority.Snapshot{}, os.ErrNotExist
	}
	return snapshot, nil
}

func (tree gcliAuthorityTree) SessionLeader(pid int64) (int64, error) {
	if leader, ok := tree.leaders[pid]; ok {
		return leader, nil
	}
	return 0, os.ErrNotExist
}

const (
	gcliAuthorityTerminal = "ttys:proof"
	gcliAuthorityHuman    = int64(30) // a shell at an agent-free terminal
	gcliAuthorityAgent    = int64(50) // a shell under the signed fake agent, same terminal
	gcliAuthorityHeadless = int64(70) // a detached shell with no controlling terminal
)

func gcliAuthorityProcess(pid, parent int64, terminal string, argv ...string) humanauthority.Snapshot {
	return humanauthority.Snapshot{
		Exact:      identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), Argv: argv, ArgvKnown: true},
		Executable: "/fixture/bin/" + argv[0], ExecutableKnown: true, OwnerUID: 501, OwnerKnown: true,
		ParentPID: parent, ParentKnown: true, TerminalID: terminal, TerminalKnown: true,
	}
}

func gcliAuthorityProcessTree() gcliAuthorityTree {
	return gcliAuthorityTree{
		snapshots: map[int64]humanauthority.Snapshot{
			1:  gcliAuthorityProcess(1, 0, "", "launchd"),
			10: gcliAuthorityProcess(10, 1, gcliAuthorityTerminal, "login"),
			20: gcliAuthorityProcess(20, 10, gcliAuthorityTerminal, "bash"),
			30: gcliAuthorityProcess(30, 20, gcliAuthorityTerminal, "metasystem"),
			40: gcliAuthorityProcess(40, 20, gcliAuthorityTerminal, "metasystem-fake-agent", "fixture"),
			50: gcliAuthorityProcess(50, 40, gcliAuthorityTerminal, "bash", "agent-shell"),
			71: gcliAuthorityProcess(71, 1, "", "setsid"),
			70: gcliAuthorityProcess(70, 71, "", "bash", "headless"),
		},
		leaders: map[int64]int64{30: 10, 20: 10, 10: 10, 50: 10, 40: 10, 70: 71, 71: 71},
	}
}

// gcliAuthorityCaller makes the bed's every human proof the real
// humanauthority walk (enrolled grade, and the terminal grade a stopping act
// falls back to) from invoker over the injected process table, against the
// bed's installed agent signatures: the fake runtime's signature matches the
// shell bed's metasystem-fake-agent holder.
type gcliAuthorityCaller struct {
	bed     *goalCLIBed
	tree    gcliAuthorityTree
	invoker int64
}

func newGcliAuthorityCaller(t *testing.T, bed *goalCLIBed, invoker int64) *gcliAuthorityCaller {
	t.Helper()
	adapters := filepath.Join(bed.root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	signature := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match metasystem-fake-agent'\n"
	if err := testexec.WriteFile(filepath.Join(adapters, "fake.sh"), []byte(signature), 0o755); err != nil {
		t.Fatal(err)
	}
	return &gcliAuthorityCaller{bed: bed, tree: gcliAuthorityProcessTree(), invoker: invoker}
}

// prove is the enrolled-grade walk from this caller.
func (c *gcliAuthorityCaller) prove(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
	return humanauthority.Prove(root, c.invoker, c.tree, now)
}

func (c *gcliAuthorityCaller) dependencies(stdout, stderr *bytes.Buffer) syncRequestDependencies {
	dependencies := c.bed.dependencies(stdout, stderr)
	dependencies.proveHuman = func(root string, pid int64, reader humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		return c.prove(root, pid, reader, "", "", now)
	}
	dependencies.proveTerminal = func(root string, _ int64, _ humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
		return humanauthority.ProveTerminal(root, c.invoker, c.tree, now)
	}
	return dependencies
}

// public runs one public command as this caller.
func (c *gcliAuthorityCaller) public(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	owners := c.bed.owners(&stdout, &stderr)
	owners.prove = c.prove
	owners.dependencies = c.dependencies(&stdout, &stderr)
	code := runIntentIn(command, rest, &stdout, &stderr, c.bed.root, owners)
	return code, stdout.String(), stderr.String()
}

// release runs the release owner in process (the stopping act's own request:
// enrolled grade, or the terminal grade it falls back to).
func (c *gcliAuthorityCaller) release(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	dependencies := c.dependencies(&stdout, &stderr)
	builder := func(verb, root, by, lineage string) (goal.VerbRequest, error) {
		return syncStoppingReqWithProofWithDependencies(verb, root, by, lineage, nil, c.bed.commandNow, dependencies)
	}
	code := runGoalReleaseWithDependencies(append([]string{"--root", c.bed.root}, args...), builder, dependencies)
	return code, stdout.String(), stderr.String()
}

// mutation runs the park or unpark owner in process, the owner public goal
// pause and goal resume call, with the arc form public has no spelling for.
func (c *gcliAuthorityCaller) mutation(name string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	owners := c.bed.owners(&stdout, &stderr)
	// The owner reports into a typed report, as the public commands read it,
	// so its refusals reach this caller rather than the process's stderr.
	report := &ownerReport{}
	dependencies := c.dependencies(&stdout, &stderr)
	dependencies.report = report
	code, _ := trySyncMutationWithCompletion(name, append([]string{"--root", c.bed.root}, args...), c.bed.commandNow,
		dependencies, owners.parkBranchCheck, owners.completion)
	if report.failure != nil {
		stderr.WriteString(report.failure.Error() + "\n")
	}
	if report.result != nil {
		stdout.WriteString(string(report.result.Outcome) + " " + report.result.Detail + "\n")
	}
	return code, stdout.String(), stderr.String()
}

// gcliAuthorityStoppingJournals lists the lineages the checkout's goal
// transaction journals record for verb.
func gcliAuthorityStoppingJournals(t *testing.T, root, verb string) []string {
	t.Helper()
	entries, err := goal.Entries(root)
	if err != nil {
		t.Fatal(err)
	}
	var lineages []string
	for _, entry := range entries {
		if entry.Intent.Verb == verb {
			lineages = append(lineages, entry.Lineage)
		}
	}
	return lineages
}

// TestGoalCLIAuthorityProofGrades is the proof-grades scenario. An
// agent-descended caller is refused AGENT_IN_AUTHORITY_CHAIN on release, pause,
// resume and approve and their arc forms; a headless human-word caller is
// refused TERMINAL_NOT_REACHED; a human at an unenrolled terminal may release,
// pause and resume to queued at the terminal grade, each journaled under the
// derived terminal-<id>-0 lineage, while approval, an enrolled-grade act, is
// refused TERMINAL_NOT_ENROLLED.
func TestGoalCLIAuthorityProofGrades(t *testing.T) {
	t.Parallel()
	const agentRefusal = `AGENT_IN_AUTHORITY_CHAIN: [[:alnum:]_-]+`
	agentRefused := func(t *testing.T, label string, code int, stdout, stderr string) {
		t.Helper()
		if code == 0 {
			t.Fatalf("agent shell was allowed to %s: stdout=%q stderr=%q", label, stdout, stderr)
		}
		if !regexp.MustCompile(agentRefusal).MatchString(stdout + stderr) {
			t.Fatalf("agent shell %s failed without its exact authority refusal: code=%d stdout=%q stderr=%q", label, code, stdout, stderr)
		}
	}
	humanRuns := func(t *testing.T, label string, code int, stdout, stderr string) {
		t.Helper()
		if code != 0 {
			t.Fatalf("unenrolled human terminal could not %s: code=%d stdout=%q stderr=%q", label, code, stdout, stderr)
		}
	}
	// The main checkout and the arc checkout, a second clone of the same
	// ledger on machine fixture-arc-machine. Neither has a terminal enrollment.
	main := newGoalCLIBed(t, goalCLISeed{noEnrollment: true})
	arc := newGoalCLIBed(t, goalCLISeed{noEnrollment: true})
	arc.machine = "fixture-arc-machine"

	t.Run("headless human word", func(t *testing.T) {
		bed := newGoalCLIBed(t, goalCLISeed{noEnrollment: true})
		bed.lineage = ""
		headless := newGcliAuthorityCaller(t, bed, gcliAuthorityHeadless)
		code, stdout, stderr := headless.release("--id", "ship-widget", "--by", "Wido")
		if code == 0 || !strings.Contains(stderr, "TERMINAL_NOT_REACHED") {
			t.Fatalf("headless human-word caller did not receive the terminal-not-reached refusal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	t.Run("agent-descended caller", func(t *testing.T) {
		agent := newGcliAuthorityCaller(t, main, gcliAuthorityAgent)
		arcAgent := newGcliAuthorityCaller(t, arc, gcliAuthorityAgent)
		before, arcBefore := main.tip(), arc.tip()
		code, stdout, stderr := agent.release("--id", "ship-widget", "--by", "Wido", "--lineage", "agent-shell")
		agentRefused(t, "release", code, stdout, stderr)
		code, stdout, stderr = agent.public(t, "goal", "pause", "ship-widget", "--reason", "agent stop", "--by", "Wido", "--lineage", "agent-shell")
		agentRefused(t, "park", code, stdout, stderr)
		code, stdout, stderr = arcAgent.release("--id", "ship-widget", "--by", "Wido", "--lineage", "agent-shell", "--arc", "proof-grade")
		agentRefused(t, "release-arc", code, stdout, stderr)
		code, stdout, stderr = arcAgent.mutation("park", "--id", "ship-widget", "--because", "agent arc stop", "--by", "Wido", "--lineage", "agent-shell", "--arc", "proof-grade")
		agentRefused(t, "park-arc", code, stdout, stderr)
		code, stdout, stderr = arcAgent.mutation("unpark", "--id", "ship-widget", "--by", "Wido", "--lineage", "agent-shell", "--arc", "proof-grade")
		agentRefused(t, "unpark-arc", code, stdout, stderr)
		code, stdout, stderr = agent.public(t, "goal", "approve", "fix-docs", "--budget", "1d/10/720m/1/3", "--by", "Wido", "--lineage", "agent-shell")
		agentRefused(t, "approve", code, stdout, stderr)
		if main.tip() != before || arc.tip() != arcBefore {
			t.Fatal("a refused agent act moved the accepted ledger")
		}
	})

	t.Run("human at an unenrolled terminal", func(t *testing.T) {
		main.lineage, arc.lineage = "", ""
		human := newGcliAuthorityCaller(t, main, gcliAuthorityHuman)
		arcHuman := newGcliAuthorityCaller(t, arc, gcliAuthorityHuman)
		agent := newGcliAuthorityCaller(t, main, gcliAuthorityAgent)

		code, stdout, stderr := human.release("--id", "ship-widget", "--by", "Wido")
		humanRuns(t, "release", code, stdout, stderr)
		code, stdout, stderr = human.public(t, "goal", "pause", "ship-widget", "--reason", "human stop", "--by", "Wido")
		humanRuns(t, "park", code, stdout, stderr)
		code, stdout, stderr = agent.public(t, "goal", "resume", "ship-widget", "--by", "Wido", "--lineage", "agent-shell")
		agentRefused(t, "unpark", code, stdout, stderr)
		code, stdout, stderr = human.public(t, "goal", "resume", "ship-widget", "--by", "Wido")
		humanRuns(t, "unpark-to-queued", code, stdout, stderr)
		if state := goalCLILine(main.goalRecord("ship-widget"), "- State: "); state != "- State: queued" {
			t.Fatalf("the human unpark did not return ship-widget to the queue: %q", state)
		}
		code, stdout, stderr = arcHuman.release("--id", "ship-widget", "--by", "Wido", "--arc", "proof-grade")
		humanRuns(t, "release-arc", code, stdout, stderr)
		code, stdout, stderr = arcHuman.mutation("park", "--id", "ship-widget", "--because", "human arc stop", "--by", "Wido", "--arc", "proof-grade")
		humanRuns(t, "park-arc", code, stdout, stderr)
		code, stdout, stderr = arcHuman.mutation("unpark", "--id", "ship-widget", "--by", "Wido", "--arc", "proof-grade")
		humanRuns(t, "unpark-arc-to-queued", code, stdout, stderr)

		derived := "terminal-ttys-proof-0"
		for _, verb := range []string{"release", "park", "unpark"} {
			if lineages := gcliAuthorityStoppingJournals(t, main.root, verb); len(lineages) != 1 || lineages[0] != derived {
				t.Fatalf("the human %s journal did not record derived lineage %s: %q", verb, derived, lineages)
			}
		}

		before := main.tip()
		code, stdout, stderr = human.public(t, "goal", "approve", "fix-docs", "--budget", "1d/10/720m/1/3", "--by", "Wido")
		if code == 0 || !strings.Contains(stdout+stderr, "TERMINAL_NOT_ENROLLED") || main.tip() != before {
			t.Fatalf("unenrolled human approval did not name its enrolled-grade refusal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}
