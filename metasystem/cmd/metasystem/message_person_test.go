package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
)

// The person-act refusals follow "Messages a Person Reads": line 1 is what
// happened and why here, line 2 the one command with the name filled in, and
// the rest only with --verbose. The trigger was grant add on a terminal that
// is not enrolled (Wido, 2026-09-30).

// notEnrolledOwners is the intent bed with a proof that finds no enrolled
// terminal.
func notEnrolledOwners(bed *intentBed) intentOwners {
	owners := bed.owners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New(humanauthority.OutcomeNotEnrolled + ": human authority has no readable terminal enrollment")
	}
	return owners
}

func TestMessagePersonActRefusalIsTwoLines(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	code, stdout, stderr := bed.run(notEnrolledOwners(bed), "grant", "add", "--tiers", "1", "--acts", "approve", "--until", "2026-12-01")
	want := "metasystem grant add: this terminal isn't enrolled yet, so nothing was done\n" +
		"run: metasystem system enroll --name Wido  (then repeat this command)\n"
	if code == 0 || stdout != "" || stderr != want {
		t.Fatalf("grant add = %d\nstdout %q\nstderr %q\nwant   %q", code, stdout, stderr, want)
	}
	code, _, verbose := bed.run(notEnrolledOwners(bed), "grant", "add", "--tiers", "1", "--acts", "approve", "--until", "2026-12-01", "--verbose")
	if code == 0 || !strings.HasPrefix(verbose, want) || !strings.Contains(verbose, humanauthority.OutcomeNotEnrolled) {
		t.Fatalf("grant add --verbose = %d %q; want the two lines, then the refusal's code", code, verbose)
	}
	code, result := bed.runJSON(notEnrolledOwners(bed), "grant", "add", "--tiers", "1", "--acts", "approve", "--until", "2026-12-01")
	if code == 0 || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem system enroll --name Wido" || len(result.Details) == 0 {
		t.Fatalf("grant add --json = %d %+v", code, result)
	}
}

// An act an agent session may also perform names, for an agent's shell, the
// flag that says which session acts; a person's way is in the details.
func TestMessageAgentShellNamesTheSessionFlag(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	unenrolled := goalSyncTerminalReader(t, bed.root(), "ttys:not_enrolled")
	bed.facts.reader = &unenrolled
	owners := bed.owners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New(humanauthority.OutcomeAgent + ": claude")
	}
	code, _, stderr := bed.run(owners, "goal", "edit", bedGoal, "--next", "Continue.")
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if code == 0 || len(lines) != 2 ||
		lines[0] != "metasystem goal edit: an agent (claude) started this shell and named no session, so nothing was done" ||
		lines[1] != "run: metasystem goal edit "+bedGoal+" --next Continue. --lineage LINEAGE  (as the session that holds the work)" {
		t.Fatalf("goal edit = %d\n%s", code, stderr)
	}
	// goal pause and done read the terminal themselves: the same two lines.
	code, _, stderr = bed.run(owners, "goal", "pause", bedGoal, "--reason", "x")
	lines = strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if code == 0 || len(lines) != 2 || lines[0] != "metasystem goal pause: this terminal isn't enrolled (Wido enrolled another one), so nothing was done" ||
		lines[1] != "run: metasystem system enroll --name Wido  (moves the enrollment here; then repeat this command)" {
		t.Fatalf("goal pause = %d\n%s", code, stderr)
	}
}

// A notice that the helm admitted an act is printed only when the act then
// proceeds; a refused act prints its refusal alone.
func TestMessageAdmissionNoticeWaitsForTheOutcome(t *testing.T) {
	t.Parallel()
	command, _ := findIntentCommand("grant add")
	for _, test := range []struct {
		name    string
		result  intentResult
		verbose bool
		want    string
	}{
		{"refused", intentResult{Outcome: intentRefused, code: 2, Summary: "this terminal isn't enrolled yet, so nothing was done"}, false,
			"metasystem grant add: this terminal isn't enrolled yet, so nothing was done\n"},
		{"confirmed", intentResult{Outcome: intentConfirmed, Summary: "granted p1"}, false,
			"HUMAN AT THE HELM (wido): grant add runs as wido's act\ngranted p1\n"},
		{"confirmed verbose", intentResult{Outcome: intentConfirmed, Summary: "granted p1"}, true,
			"HUMAN AT THE HELM (wido): grant add runs as wido's act\nlogged in /seat/helm-yields.log\ngranted p1\n"},
	} {
		board := &admissionNotices{}
		board.arm()
		var out bytes.Buffer
		release, _ := board.hold()
		board.say(admissionNotice{w: &out, line: "HUMAN AT THE HELM (wido): grant add runs as wido's act", detail: "logged in /seat/helm-yields.log"})
		inv := &intentInvocation{command: command, stdout: &out, stderr: &out, notices: board, input: intentInput{values: map[string][]string{}}}
		if test.verbose {
			inv.input.values["verbose"] = []string{"true"}
		}
		inv.render(test.result)
		release()
		if out.String() != test.want {
			t.Errorf("%s:\n got  %q\n want %q", test.name, out.String(), test.want)
		}
	}
}

// system start and stop by a shell that is not a person's say so in plain
// words and name the command to run at the person's own terminal.
func TestMessageSystemStopByAnAgentSession(t *testing.T) {
	t.Parallel()
	classify := func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}
	top := func(string) (string, error) { return "/seat", nil }
	_, refusal := humanTerminalCheck("/seat", "/seat", "metasystem system stop", top, classify, "metasystem system stop --repo /seat")
	if refusal == nil {
		t.Fatal("a delegate's stop was not refused")
	}
	result := processRefusalResult(nil, refusal, stoptransition.Report{})
	if result.Summary != "an agent started this shell, so nothing was changed" || strings.Join(result.next, " ") != "metasystem system stop --repo /seat" ||
		result.nextReason != "in a terminal you opened yourself" {
		t.Fatalf("stop refusal = %+v", result)
	}
}

// The trigger, end to end on the real engine: at the helm, grant add
// --acts everything from a shell that is not the enrolled terminal prints
// the refusal alone, in two lines, with the helm holder's name filled in; the
// helm's admission notice, which the refusal contradicts, is not printed.
func TestMessageGrantAddAtTheHelmIsOneRefusal(t *testing.T) {
	t.Parallel()
	root := helmLedgerRepo(t, "grant-machine")
	seat, err := helm.Locate(root)
	helmMust(t, err)
	_, err = helm.Write(root, helm.Record{By: "wido", At: time.Now().UTC().Format(time.RFC3339), Reason: "e2e", Checkout: seat.Checkout})
	helmMust(t, err)
	code, out := helmEngine(t, root, "grant", "add", "--acts", "everything", "--for", "24h")
	want := "metasystem grant add: this terminal isn't enrolled (wido enrolled another one), so nothing was done\n" +
		"run: metasystem system enroll --name wido  (moves the enrollment here; then repeat this command)\n"
	if code != 2 || out != want {
		t.Fatalf("grant add at the helm = %d\n%s\nwant\n%s", code, out, want)
	}
	code, out = helmEngine(t, root, "grant", "add", "--acts", "everything", "--for", "24h", "--verbose")
	if code != 2 || !strings.HasPrefix(out, want) || !strings.Contains(out, "HUMAN AT THE HELM (wido): grant add runs as wido's act") {
		t.Fatalf("grant add --verbose at the helm = %d\n%s", code, out)
	}
}
