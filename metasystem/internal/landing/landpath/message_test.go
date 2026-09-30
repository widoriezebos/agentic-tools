package landpath

import (
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// "Messages a Person Reads" (docs/design/design-principles.md): a landing
// that stops tells the person what happened and why in one plain line, then
// the one command that resolves it; verdicts, codes, paths lists and the step
// log are details. The triggers were the 2026-09-30 switch-on trial's
// refusals ("the Goal-Item is not held by this machine and lineage
// (would-refuse code=goal-item-not-held) ... lawful classification exits").

// internalWords are words a person never reads on the two lines.
var internalWords = regexp.MustCompile(`(?i)would-refuse|code=|lineage|proof|lawful|carriage|verdict|== STEP|!! STEP|Goal-Item is`)

// twoLines checks the person's stream is exactly a stop's two lines.
func twoLines(t *testing.T, stderr, reason, run string) {
	t.Helper()
	want := reason + "\n" + run + "\n"
	if stderr != want {
		t.Fatalf("stderr:\n%s\nwant:\n%s", stderr, want)
	}
	if internalWords.MatchString(stderr) {
		t.Fatalf("the two lines speak internal words: %q", stderr)
	}
	if first, _, _ := strings.Cut(stderr, "\n"); len([]rune(first)) > 100 {
		t.Fatalf("line 1 is %d characters: %q", len([]rune(first)), first)
	}
}

func TestMessageAgentCommitRefusalIsTwoPlainLines(t *testing.T) {
	t.Parallel()
	cases := []struct{ code, reason, run string }{
		{"goal-item-not-held", "goal g1 is not claimed by this session, so nothing was committed",
			"run: metasystem goal claim g1 --take-over --reason TEXT  (a person takes it over; then repeat this command)"},
		{"register-carriage-not-append-only", "the change rewrites or deletes lines of an append-only record; only new lines may be added",
			"needed first: put the existing lines back, keep only the appended ones, then repeat this command"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			b := newBed(t)
			b.epoch = epochOf(4)
			b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: c.code, Provenance: "none change=x",
				VerdictTrailer: "would-refuse code=" + c.code, Refusal: "detail of " + c.code}
			b.git.on("diff --cached --name-only -z --", func(GitCall) GitResult { return ok("a.go\x00") })
			stop := &Stop{}
			status := Commit(b.owners, CommitRequest{Root: b.root, MessageFile: b.messageFile(), OwnerLineage: "L", Chain: "j1",
				Goal: "g1", GoalSet: true, Stop: stop}, &b.stdout, &b.stderr)
			if status != 1 {
				t.Fatalf("status %d", status)
			}
			twoLines(t, b.stderr.String(), c.reason, c.run)
			if stop.Reason != c.reason {
				t.Fatalf("stop = %+v", stop)
			}
			for _, detail := range []string{"verdict: would-refuse code=" + c.code, "detail of " + c.code, "staged paths:\n  a.go"} {
				if !strings.Contains(b.stdout.String(), detail) {
					t.Fatalf("details lack %q:\n%s", detail, b.stdout.String())
				}
			}
		})
	}
}

// A landing's steps are details: the person reads nothing while it goes
// well, and --verbose shows each step.
func TestMessageLandingStepsAreDetails(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.expect(b.land(LandRequest{StagedOnly: true}), 0)
	if b.stderr.String() != "" {
		t.Fatalf("a landing that went well wrote to the person: %q", b.stderr.String())
	}
	if !strings.Contains(b.stdout.String(), "step: commit\n  ok\n") || strings.Contains(b.stdout.String(), "== STEP") {
		t.Fatalf("details:\n%s", b.stdout.String())
	}
}

// A step that fails names itself and its cause on line 1; its log is a
// detail.
func TestMessageFailedStepNamesItsCause(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.on("fetch", func(GitCall) GitResult { return failed(73, "fatal: unable to access origin\n") })
	stop := &Stop{}
	status := Land(b.owners, LandRequest{Root: b.root, MessageFile: b.messageFile(), StagedOnly: true, Stop: stop}, &b.stdout, &b.stderr)
	if status != 73 {
		t.Fatalf("status %d", status)
	}
	twoLines(t, b.stderr.String(), "the landing stopped at fetch origin: fatal: unable to access origin",
		"needed first: fix what stopped it (--verbose shows the step's log), then repeat this command")
	if !strings.Contains(b.stdout.String(), "step failed: fetch origin (exit 73)") || stop.Reason == "" {
		t.Fatalf("details:\n%s\nstop %+v", b.stdout.String(), stop)
	}
}

// A refusal inside a step is the cause, never the step that carried it.
func TestMessageRefusalInsideAStepIsTheCause(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.epoch = epochOf(4)
	b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "goal-item-not-held", Provenance: "p",
		VerdictTrailer: "would-refuse code=goal-item-not-held"}
	stop := &Stop{}
	status := Land(b.owners, LandRequest{Root: b.root, MessageFile: b.messageFile(), StagedOnly: true, Goal: "g1", GoalSet: true,
		OwnerLineage: "L", Stop: stop}, &b.stdout, &b.stderr)
	if status != 1 {
		t.Fatalf("status %d", status)
	}
	twoLines(t, b.stderr.String(), "goal g1 is not claimed by this session, so nothing was committed",
		"run: metasystem goal claim g1 --take-over --reason TEXT  (a person takes it over; then repeat this command)")
	if len(stop.Run) == 0 || stop.Run[3] != "g1" {
		t.Fatalf("stop = %+v", stop)
	}
}
