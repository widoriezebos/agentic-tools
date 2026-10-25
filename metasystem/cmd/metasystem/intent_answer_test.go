package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

func TestUnitStopQuestionsOfferSolutions(t *testing.T) {
	t.Parallel()
	t.Run("a stopped build", func(t *testing.T) {
		t.Parallel()
		b := failedBuildQuestionBed(t)
		enrol(t, b, "Wido")
		q, r := partialQuestion(t, b)
		round := r.Rounds[0]
		act := withoutOption(shellWords(q.UnitStop.Needs), "by")
		if round.Cause == "environment" || round.Cause == "deadline" || !q.Contract || q.Recommendation != "Continue" || len(q.Options) != 2 {
			t.Fatalf("the correction does not offer solutions: cause=%s %+v", round.Cause, q)
		}
		decision := fmt.Sprintf("%s: the build of unit outcome stopped at %s (%s).", strings.ReplaceAll(b.id, "-", " "), q.OpenedAt.Local().Format("15:04"), round.Stop.Class)
		if q.Plain.Decision != decision || q.Plain.IfNothing != "it waits; nothing builds or lands until you choose." {
			t.Fatalf("words = %+v, want %q", q.Plain, decision)
		}
		continueOption, stop := q.Options[0], q.Options[1]
		if continueOption.Label != "Continue" || !slices.Equal(continueOption.Act, act) || !strings.HasSuffix(continueOption.Consequence, "Undo: metasystem work stop "+b.id+".") || stop.Label != "Stop here" || stop.Act != nil {
			t.Fatalf("options = %+v, want Continue with %q", q.Options, act)
		}
		if brief := act[slices.Index(act, "--brief")+1]; !regularFile(brief) || brief != filepath.Join(round.Directory, "correction-brief.md") {
			t.Fatalf("the act's brief is not the prepared one: %s", brief)
		}
		for _, word := range []string{"FILE", "TEXT", "NAME", "--by"} {
			if slices.Contains(continueOption.Act, word) {
				t.Fatalf("the act keeps %s: %q", word, continueOption.Act)
			}
		}
	})
	for _, row := range []struct{ name, diff string }{
		{"a gap", ""},
		{"a size stop", "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n-old\n+new\ndiff --git a/a_test.go b/a_test.go\n--- /dev/null\n+++ b/a_test.go\n+test\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := outcomeBed(t, row.diff)
			if code, result, _ := outcomeBuild(t, b); code != 1 {
				t.Fatalf("build: %d %+v", code, result)
			}
			if qs, _ := channel.WalkOpenQuestions(b.root()); len(qs) != 1 || qs[0].Contract || len(qs[0].Options) != 0 {
				t.Fatalf("%s asks differently than before: %+v", row.name, qs)
			}
		})
	}
	t.Run("a check deadline", func(t *testing.T) {
		t.Parallel()
		b, _, _ := deadlineCheckBed(t, 4, everyCheckDeadline)
		agentBuild(t, b)
		if qs, _ := channel.WalkOpenQuestions(b.root()); len(qs) != 1 || qs[0].Contract || !slices.Contains(qs[0].UnitStop.AcceptableActs, "work-review-retry") {
			t.Fatalf("the retry asks differently than before: %+v", qs)
		}
	})
}
