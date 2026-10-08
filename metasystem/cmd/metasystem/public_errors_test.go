package main

import (
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// The public refusals of the 2026-09-28 error-message audit
// (agentic-tools-evidence/error-messages-20260928/public-audit.md): each
// witness names its audit row. A refusal names what is wrong in the words the
// caller typed and the public command that helps; it never crashes, never
// names an internal command, never exits 0 for work it did not do.

// EM-01: question answer with no question refused by name, never a panic.
func TestQuestionAnswerWithoutAQuestionNamesIt(t *testing.T) {
	b := newIntentBed(t, false, nil)
	code, stdout, stderr := b.run(b.owners(), "question", "answer")
	text := stdout + stderr
	if code != 2 || !strings.Contains(text, "needs the question") || !strings.Contains(text, "metasystem question list") {
		t.Fatalf("question answer without Q: code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// EM-21: an act refused because no actor was proven says why in plain words
// and names the one command that resolves it; it names no refusal code, no
// process ancestry and no internal verb, and --lineage, the agent's remedy,
// is in the action's help and in the refusal's details.
func TestAnActorRefusalSaysWhoMayActInPublicWords(t *testing.T) {
	bed := newIntentBed(t, false, makeQueued)
	unenrolled := goalSyncTerminalReader(t, bed.root(), "ttys:not_enrolled")
	bed.facts.reader = &unenrolled
	owners := bed.owners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil)
	}
	for _, row := range []struct {
		args  []string
		wants []string
	}{
		{[]string{"goal", "pause", bedGoal, "--reason", "x"}, nil},
		{[]string{"goal", "pin", bedGoal, "m1e"}, nil},
		{[]string{"grant", "revoke", "grant-1"}, nil},
		{[]string{"incident", "close", "incident-1", "--reason", "x"}, nil},
	} {
		code, stdout, stderr := bed.run(owners, row.args...)
		text := stdout + stderr
		for _, forbidden := range []string{"TERMINAL_NOT_REACHED", "ancestry", "set-pin", "goal revoke", "trunk-red"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%v names %q: %q", row.args, forbidden, text)
			}
		}
		// "Messages a Person Reads": why here in plain words, and the one
		// command with the enrolled person's name filled in.
		for _, want := range append(row.wants, "✗ this terminal isn't enrolled (Wido enrolled another one), so nothing was done\n",
			"  → metasystem system enroll --name Wido  moves the enrollment here; then repeat this command\n") {
			if code == 0 || !strings.Contains(text, want) {
				t.Errorf("%v: code %d, want %q in %q", row.args, code, want, text)
			}
		}
	}
	if _, out, _ := runCLI("goal", "pause", "--help"); !strings.Contains(out, "--lineage") {
		t.Errorf("goal pause --help omits --lineage: %q", out)
	}
}

// runPublic runs one public command line in this process, as the binary
// does, from the test's directory inside this repository.
func runPublic(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return dispatchWithFamilies(args, stdout, stderr, families())
	})
}

// EM-04, EM-11: a passthrough action answers its mistakes under its public
// name and lists only the options its help documents; EM-06: --repo, which
// every command takes, is taken.
func TestPassthroughActionsAnswerUnderTheirPublicName(t *testing.T) {
	for _, row := range []struct {
		args []string
		want string
	}{
		{[]string{"test", "status", "--bogus"}, "metasystem test status: does not take --bogus; it takes "},
		{[]string{"test", "list", "--bogus"}, "metasystem test list: does not take --bogus; it takes "},
		{[]string{"test", "plan", "--bogus"}, "metasystem test plan: does not take --bogus; it takes "},
		{[]string{"session", "status", "--bogus"}, "metasystem session status: does not take --bogus; it takes "},
		{[]string{"session", "handoff", "--bogus"}, "metasystem session handoff: does not take --bogus; it takes "},
		{[]string{"session", "isolate", "--bogus"}, "metasystem session isolate: does not take --bogus; it takes "},
		{[]string{"experiment", "record", "--bogus"}, "metasystem experiment record: does not take --bogus; it takes "},
		{[]string{"experiment", "status", "--bogus"}, "metasystem experiment status: does not take --bogus; it takes "},
		{[]string{"experiment", "check", "--bogus"}, "metasystem experiment check: does not take --bogus; it takes "},
		{[]string{"experiment", "record", "--score"}, "metasystem experiment record: --score needs a value"},
		{[]string{"receipt", "status", "--bogus"}, "metasystem receipt status: does not take --bogus; it takes "},
		{[]string{"receipt", "add", "--bogus"}, "metasystem receipt add: does not take --bogus; it takes "},
		{[]string{"test", "baseline", "--bogus"}, "metasystem test baseline: does not take --bogus; it takes "},
	} {
		code, stdout, stderr := runPublic(t, row.args...)
		if code != 2 || !strings.Contains(stderr, row.want) {
			t.Errorf("%v: code %d stdout %q stderr %q; want %q", row.args, code, stdout, stderr, row.want)
		}
		for _, internal := range []string{"internal", "policy-child", "test verify", "report ", "context handoff", "validate stop-loss", "Usage of", "flag provided"} {
			if strings.Contains(stderr, internal) {
				t.Errorf("%v names %q: %q", row.args, internal, stderr)
			}
		}
	}
	if code, _, stderr := runPublic(t, "receipt", "status", "--repo", "."); strings.Contains(stderr, "does not take --repo") || code == 2 {
		t.Errorf("receipt status --repo: code %d stderr %q", code, stderr)
	}
}

// EM-12: an action whose help shows --root as optional finds the
// installation from the current directory or --repo, as every command does.
func TestPassthroughActionsFindTheirInstallation(t *testing.T) {
	for _, args := range [][]string{{"test", "list"}, {"test", "list", "--repo", "."}} {
		code, stdout, stderr := runPublic(t, args...)
		if code != 0 || !strings.Contains(stdout, "verb-ratchet") {
			t.Errorf("%v: code %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}
	// EM-02, EM-37: the receipt actions find the installation and its
	// ledger from the current directory, not from where the binary lives.
	for _, args := range [][]string{{"receipt", "status"}, {"receipt", "status", "--repo", "../.."}} {
		_, _, stderr := runPublic(t, args...)
		if strings.Contains(stderr, "is not installed at") || strings.Contains(stderr, "does not parse") || strings.Contains(stderr, "cannot read metasystem configuration") {
			t.Errorf("%v: stderr %q", args, stderr)
		}
	}
	// EM-04: a bare test status names what it needs in public words.
	code, _, stderr := runPublic(t, "test", "status")
	if code != 2 || !strings.Contains(stderr, "metasystem test status") || !strings.Contains(stderr, "--tree") ||
		strings.Contains(stderr, "internal") || strings.Contains(stderr, "test verify") || strings.Contains(stderr, "--root is required") {
		t.Errorf("bare test status: code %d stderr %q", code, stderr)
	}
}

// EM-05: session status's own help example names a Stop report id the
// command takes, and an id it cannot take is refused in public words.
func TestSessionStatusExampleIsAnIDItTakes(t *testing.T) {
	command, _ := findIntentCommand("session status")
	for _, example := range command.examples {
		words := strings.Fields(example)
		for i, word := range words {
			if word == "--id" && i+1 < len(words) {
				if err := report.ValidateStopStatusID(words[i+1]); err != nil {
					t.Errorf("example %q names an id the command refuses: %v", example, err)
				}
			}
		}
	}
	code, _, stderr := runPublic(t, "session", "status", "--id", "r-7f3a")
	if code != 2 || !strings.HasPrefix(stderr, "✗ r-7f3a is not a Stop report id") || strings.Contains(stderr, "report stop-status") {
		t.Errorf("session status --id r-7f3a: code %d stderr %q", code, stderr)
	}
}

// EM-35: test baseline names what it needs with an example, and a --gate
// without its command is named.
func TestTestBaselineNamesWhatItNeeds(t *testing.T) {
	for _, row := range []struct {
		args []string
		want string
	}{
		{[]string{"test", "baseline"}, "\n  → metasystem test baseline --gate 'go test ./...'"},
		{[]string{"test", "baseline", "--gate"}, "--gate needs a value: the gate command that passed, e.g."},
	} {
		code, stdout, stderr := runPublic(t, row.args...)
		if code != 2 || !strings.Contains(stderr, row.want) || strings.Contains(stderr, "Exit codes") {
			t.Errorf("%v: code %d stdout %q stderr %q", row.args, code, stdout, stderr)
		}
	}
}

// EM-19, EM-22, EM-40: system check's remedies are public acts that repair
// the role, never the check itself, and a reason that names its own command
// is not contradicted.
func TestSystemCheckRemediesArePublicActs(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		role        steward.RoleVerdict
		argv        []string
		instruction string
	}{
		{steward.RoleVerdict{Role: steward.RoleSessionMain, Reason: "no session main is announced"}, []string{"metasystem", "system", "start"}, ""},
		{steward.RoleVerdict{Role: steward.RoleCapabilitySnapshots, Reason: "missing or stale capability snapshots: claude"}, nil, "the steward tick probes"},
		{steward.RoleVerdict{Role: steward.RoleTrunkRed, Reason: "no deep validation cadence status is recorded"}, nil, "nothing to do: the armed landing lane records the cadence at its next validation"},
		{steward.RoleVerdict{Role: steward.RoleTrunkRed, Reason: "open trunk-red incident inc-1 has no owner", RemedyFacts: []steward.RemedyFact{{Cause: steward.CauseTrunkRedUnowned, Incident: "inc-1"}}}, []string{"metasystem", "incident", "claim", "inc-1", "--goal", "G"}, ""},
		{steward.RoleVerdict{Role: steward.RoleStopCapabilityEpoch, Reason: "no machine nickname: run  git config metasystem.goal.machine <nickname>  once on this machine"}, nil, "run the command the reason above names"},
	} {
		argv, instruction := publicHealthRemedy(row.role, false)
		if !slices.Equal(argv, row.argv) || !strings.Contains(instruction, row.instruction) || strings.Contains(instruction, "no public command repairs this") {
			t.Errorf("%s (%s) = %v %q", row.role.Role, row.role.Reason, argv, instruction)
		}
	}
}

// EM-03: experiment check names the missing ledger, not a missing option,
// and an unknown option is answered without the usage wall.
func TestExperimentCheckNamesTheMissingLedger(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.md")
	code, _, stderr := runPublic(t, "experiment", "check", "--file", missing)
	if code != 2 || !strings.Contains(flatPage(stderr), "there is no ledger at "+missing) || strings.Contains(stderr, "missing --file") {
		t.Errorf("experiment check --file absent: code %d stderr %q", code, stderr)
	}
	code, _, stderr = runPublic(t, "experiment", "check")
	if code != 2 || !strings.Contains(stderr, "needs the ledger to check; nothing was checked\n  → metasystem experiment check --file LEDGER") {
		t.Errorf("bare experiment check: code %d stderr %q", code, stderr)
	}
	if _, _, stderr := runPublic(t, "experiment", "check", "--bogus"); strings.Contains(stderr, "Exit codes") {
		t.Errorf("experiment check --bogus prints the usage wall: %q", stderr)
	}
}

// EM-10, EM-15: design show takes its goal first, like every show, and an
// unknown goal is named as unknown. EM-14, EM-30: decision show points at the
// decisions.
func TestDesignAndDecisionShowNameWhatIsMissing(t *testing.T) {
	b := newIntentBed(t, false, nil)
	for _, row := range []struct {
		args       []string
		code       int
		want, next string
	}{
		{[]string{"design", "show", "nosuchgoal"}, 1, "no goal nosuchgoal on the accepted ledger", "metasystem goal list --all"},
		{[]string{"design", "show", "--goal", "nosuchgoal"}, 1, "no goal nosuchgoal on the accepted ledger", "metasystem goal list --all"},
		{[]string{"design", "show", bedGoal}, 0, "has no design record", ""},
		{[]string{"decision", "show"}, 2, "decision show takes one record id", "metasystem decision list"},
		{[]string{"decision", "show", "NOSUCH"}, 1, "the project has no record NOSUCH", "metasystem decision list"},
	} {
		code, stdout, stderr := b.run(b.owners(), row.args...)
		text := stdout + stderr
		if code != row.code || !strings.Contains(text, row.want) || row.next != "" && !strings.Contains(text, row.next) {
			t.Errorf("%v: code %d text %q", row.args, code, text)
		}
	}
}

// EM-16, EM-45, EM-50: the option parser's refusals read as plain words:
// a guess only when it is close and runnable, a word count in the caller's
// terms, a switch that takes true or false.
func TestIntentOptionRefusalsReadPlainly(t *testing.T) {
	t.Parallel()
	parse := func(name string, args ...string) *intentInputError {
		command, ok := findIntentCommand(name)
		if !ok {
			t.Fatalf("no %s", name)
		}
		_, problem := parseIntentArgs(command, args)
		return problem
	}
	if problem := parse("goal edit", "g", "--bogus"); problem == nil || strings.Contains(problem.summary, "did you mean") || !strings.Contains(problem.summary, "it takes") {
		t.Errorf("goal edit --bogus: %+v", problem)
	}
	if problem := parse("goal pause", "g", "--reson", "x"); problem == nil || !strings.Contains(problem.summary, "did you mean --reason") {
		t.Errorf("goal pause --reson: %+v", problem)
	}
	if problem := parse("goal list", "x"); problem == nil || problem.summary != "goal list takes no target; unexpected x" {
		t.Errorf("goal list x: %+v", problem)
	}
	if problem := parse("goal show", "g", "extra"); problem == nil || problem.summary != "goal show takes one target; unexpected extra" {
		t.Errorf("goal show g extra: %+v", problem)
	}
	if problem := parse("goal list", "--all=maybe"); problem == nil || problem.summary != `--all takes true or false, not "maybe"` {
		t.Errorf("goal list --all=maybe: %+v", problem)
	}
}

// EM-17, EM-42, EM-48: an unknown or missing question is named plainly,
// with the list of questions as the next step; no file error leaks.
func TestQuestionMissesPointAtTheQuestionList(t *testing.T) {
	b := newIntentBed(t, false, nil)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	for _, args := range [][]string{
		{"question", "retry", "nosuch"},
		{"question", "withdraw", "nosuch", "--reason", "x"},
		{"question", "show", "channel:nosuch"},
		{"question", "show"},
	} {
		code, stdout, stderr := b.run(owners, args...)
		text := stdout + stderr
		if code == 0 || !strings.Contains(text, "metasystem question list") || strings.Contains(text, "no such file") ||
			strings.Contains(text, "settings show") || strings.Contains(text, "show question") {
			t.Errorf("%v: code %d text %q", args, code, text)
		}
	}
}

// EM-13, EM-24: an owner's refusal is printed once: its first line is the
// summary and is not repeated under it.
func TestOwnerRefusalIsPrintedOnce(t *testing.T) {
	t.Parallel()
	result := ownerVerbResult(intentProcessResult{code: 7, stderr: []byte("no mission x has started here\ndetail\n")}, nil, "done", nil)
	if result.Summary != "no mission x has started here" || slices.Contains(result.text, result.Summary) || !slices.Contains(result.text, "detail") {
		t.Fatalf("result %+v", result)
	}
}

// EM-23: a wait this shell may not register says why in plain words, as the
// result, never "its message is on standard error".
func TestAWaitThisShellMayNotRegisterSaysWhy(t *testing.T) {
	b := newIntentBed(t, false, nil)
	code, stdout, stderr := b.run(b.owners(), "test", "wait", "proof:nosuch")
	text := stdout + stderr
	if code == 0 || strings.Contains(text, "its message is on standard error") || strings.Contains(text, "wait registration") ||
		!strings.Contains(text, "only this checkout's main agent session") {
		t.Fatalf("test wait from a non-main shell: code %d text %q", code, text)
	}
}

// EM-36: a repository that is not there is named plainly: no state-root
// words, no git or Go error text.
func TestNotInsideARepositoryIsSaidPlainly(t *testing.T) {
	outside := t.TempDir()
	for _, row := range []struct{ repo, want string }{
		{filepath.Join(outside, "absent"), "does not exist"},
		{outside, "is not inside a Git repository"},
	} {
		code, _, stderr := runPublic(t, "goal", "list", "--repo", row.repo)
		if code == 0 || !strings.Contains(stderr, row.want) || strings.Contains(stderr, "state root") || strings.Contains(stderr, "fatal:") || strings.Contains(stderr, "stat ") {
			t.Errorf("--repo %s: code %d stderr %q", row.repo, code, stderr)
		}
	}
}

// EM-43, EM-49: a file that is not there is named by its path in plain
// words, the goal is checked before the file, and every refusal says
// nothing was done.
func TestMissingFilesAreNamedByPath(t *testing.T) {
	b := newIntentBed(t, false, nil)
	missing := filepath.Join(t.TempDir(), "absent.md")
	for _, row := range []struct {
		args []string
		want string
	}{
		{[]string{"work", "revise", "nosuchgoal", "--brief", missing}, "no goal nosuchgoal on the accepted ledger"},
		{[]string{"work", "revise", bedGoal, "--brief", missing}, "no brief at " + missing},
		{[]string{"design", "write", bedGoal, "--brief", missing}, "no brief at " + missing},
		{[]string{"work", "review"}, "nothing was done"},
	} {
		code, stdout, stderr := b.run(b.owners(), row.args...)
		// A long path takes a line of its own on the page; the words are
		// read with the lines joined.
		text := strings.Join(strings.Fields(stdout+stderr), " ")
		if code == 0 || !strings.Contains(text, row.want) || strings.Contains(text, "no such file") || strings.Contains(text, "open ") {
			t.Errorf("%v: code %d text %q", row.args, code, text)
		}
	}
	code, _, stderr := runPublic(t, "test", "list", "--root", missing)
	if code == 0 || !strings.Contains(flatPage(stderr), missing+" does not exist") || strings.Contains(stderr, "lstat") {
		t.Errorf("test list --root absent: code %d stderr %q", code, stderr)
	}
}
