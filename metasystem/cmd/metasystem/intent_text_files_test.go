package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
)

// writeInputFile writes a text input under the command's starting directory
// and returns the path relative to it.
func writeInputFile(t *testing.T, cwd, relative, content string) string {
	t.Helper()
	path := filepath.Join(cwd, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return relative
}

// TestIntentTextFilesReachOwners proves a --NAME-file gives its owner the
// file's exact text, that the same text given both ways is one value, and
// that different texts are refused before any owner acts.
func TestIntentTextFilesReachOwners(t *testing.T) {
	t.Parallel()
	// The goal owner keeps a reason to one line; the file's final line end
	// is not part of it.
	reason := "handing it back:  m1b owns the \"read\" now; it's $HOME-safe"

	t.Run("release reason from a file is the recorded reason", func(t *testing.T) {
		t.Parallel()
		bed := newIntentBed(t, false, nil)
		bed.lineage = "m1"
		file := writeInputFile(t, bed.root(), "inputs/reason.txt", reason+"\n")
		before := bed.publications()
		different := writeInputFile(t, bed.root(), "inputs/other.txt", "another reason\n")
		code, result := bed.runJSON(bed.owners(), "release", "--reason", reason, "--reason-file", different)
		bed.expectNoEffect(before, []string{"release", "--reason", "--reason-file"}, code, result)
		if code != 2 || !strings.Contains(result.Summary, "--reason and --reason-file") {
			t.Fatalf("conflicting reasons = %d %+v", code, result)
		}
		code, result = bed.runJSON(bed.owners(), "release", "--reason-file", "inputs/missing.txt")
		bed.expectNoEffect(before, []string{"release", "--reason-file", "missing"}, code, result)

		code, result = bed.runJSON(bed.owners(), "release", "--reason", reason, "--reason-file", file)
		if code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("the same reason given both ways = %d %+v", code, result)
		}
		released := bed.goalFile(bedGoal)
		if last := released.History[len(released.History)-1]; last.Verb != "release" || last.Reason != reason {
			t.Fatalf("recorded reason = %q, want the file's exact text %q", last.Reason, reason)
		}

		// show --history reads the same record back, in text and JSON.
		code, stdout, _ := bed.run(bed.owners(), "show", bedGoal, "--history")
		if code != 0 || !strings.Contains(stdout, "history: ") || !strings.Contains(stdout, " release by ") || !strings.Contains(stdout, reason) {
			t.Fatalf("show --history text = %d %q", code, stdout)
		}
		_, shown := bed.runJSON(bed.owners(), "show", bedGoal, "--history")
		encoded, _ := json.Marshal(shown.Data)
		if !strings.Contains(string(encoded), `"Verb":"release"`) {
			t.Fatalf("show --history JSON has no history: %s", encoded)
		}
		_, plain := bed.runJSON(bed.owners(), "show", bedGoal)
		if encoded, _ := json.Marshal(plain.Data); strings.Contains(string(encoded), `"Verb":"release"`) {
			t.Fatalf("show without --history carries history: %s", encoded)
		}
		code, stdout, _ = bed.run(bed.owners(), "goals", "--all", "--history")
		if code != 0 || !strings.Contains(stdout, "history: ") {
			t.Fatalf("goals --history = %d %q", code, stdout)
		}
	})

	t.Run("ask questions, options and facts keep their text and order", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		question := writeInputFile(t, b.root(), "q/question.md", "Land slice 2 now?\nThe proof is green.\n")
		first := writeInputFile(t, b.root(), "q/yes.md", "yes: land it\n")
		last := writeInputFile(t, b.root(), "q/no.md", "no: wait for review\n")
		fact := writeInputFile(t, b.root(), "q/fact.md", "the read is clean\n")
		code, result := b.runJSON(b.owners(), "ask", "goal-a", "--question", "Other?", "--question-file", question, "--option", "a: b")
		if code != 2 || result.Outcome != intentRefused || len(b.asked) != 0 {
			t.Fatalf("conflicting question = %d %+v, %d asked", code, result, len(b.asked))
		}
		code, result = b.runJSON(b.owners(), "ask", "goal-a", "--question-file", question,
			"--option-file", first, "--option", "later: ask again tomorrow", "--option-file", last, "--fact-file", fact, "--fact", "inline fact")
		if code != 0 || len(b.asked) != 1 {
			t.Fatalf("ask from files = %d %+v", code, result)
		}
		asked := b.asked[0]
		if want := []string{"yes: land it", "later: ask again tomorrow", "no: wait for review"}; !slices.Equal(asked.Options, want) {
			t.Fatalf("options = %q, want %q", asked.Options, want)
		}
		if want := []string{"Land slice 2 now?\nThe proof is green.", "the read is clean", "inline fact"}; !slices.Equal(asked.Facts, want) {
			t.Fatalf("question and facts = %q, want %q", asked.Facts, want)
		}
	})

	t.Run("a mission answer from a file is the answer given", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		askPath := parkedHostFailureMission(t, b.root())
		owners := b.owners()
		var answers []string
		owners.processes.mission = func(root, id string) (*missionrunner.Engine, error) {
			engine := missionrunner.NewEngine(root, id)
			engine.AnchorEffect = func(_, _, answer string) error { answers = append(answers, answer); return nil }
			return engine, nil
		}
		file := writeInputFile(t, b.root(), "answer.md", "retry: the host is back\n")
		code, result := b.runJSON(owners, "answer", "mission", "demo", "host-down", "abort: give up", "--answer-file", file)
		if code != 2 || result.Outcome != intentRefused || missionAskAnswered(askPath) {
			t.Fatalf("conflicting answer = %d %+v", code, result)
		}
		code, result = b.runJSON(owners, "answer", "mission", "demo", "host-down", "--answer-file", file)
		if code != 0 || result.Outcome != intentConfirmed || !missionAskAnswered(askPath) {
			t.Fatalf("answer from a file = %d %+v", code, result)
		}
		data, err := os.ReadFile(askPath)
		if err != nil || !strings.Contains(string(data), "retry: the host is back") || strings.Contains(string(data), "retry: the host is back\\n") {
			t.Fatalf("recorded answer is not the file's text: %s %v (anchored %q)", data, err, answers)
		}
	})
}

// TestIntentGoalsArchivedHistory proves goals --all --history prints the
// history of a done goal it lists, concluded through its real owner, keeps
// the label filter, and that neither --all nor a plain listing prints it.
func TestIntentGoalsArchivedHistory(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	const concluded = "completed proof for archived history"
	if code, result := bed.runJSON(bed.owners(), "done", bedGoal, "--reason", concluded, "--lineage", "m1"); code != 0 {
		t.Fatalf("fixture completion: %d %+v", code, result)
	}
	code, stdout, stderr := bed.run(bed.owners(), "goals", "--all", "--history")
	if code != 0 || !strings.Contains(stdout, bedGoal+" history: ") || !strings.Contains(stdout, concluded) {
		t.Fatalf("goals --all --history = %d %q %q; want the done goal's history", code, stdout, stderr)
	}
	for _, args := range [][]string{{"goals", "--all"}, {"goals", "--history"}, {"goals", "--all", "--history", "--label", "absent-label"}} {
		code, stdout, _ := bed.run(bed.owners(), args...)
		if code != 0 || strings.Contains(stdout, concluded) {
			t.Fatalf("%v = %d %q; archived history must not be printed", args, code, stdout)
		}
	}
}

// TestIntentGoalsReadFlags proves every goals read flag is honored or
// refused, never ignored.
func TestIntentGoalsReadFlags(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	for _, args := range [][]string{
		{"goals", "--tiers", "--label", "ui"},
		{"goals", "--tiers", "--machine", "m1e"},
		{"goals", "--tiers", "--history"},
		{"goals", "--ready", "--history"},
	} {
		code, result := bed.runJSON(bed.owners(), args...)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "does not apply") {
			t.Fatalf("%v = %d %+v; want a refusal", args, code, result)
		}
	}
	code, stdout, stderr := bed.run(bed.owners(), "goals", "--pretty")
	if code != 2 || !strings.Contains(stdout+stderr, "--pretty formats JSON") {
		t.Fatalf("goals --pretty without --json = %d %q %q", code, stdout, stderr)
	}
	if code, result := bed.runJSON(bed.owners(), "goals", "--pretty"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("goals --json --pretty = %d %+v", code, result)
	}
	if code, result := bed.runJSON(bed.owners(), "goals", "--fetch"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("goals --fetch = %d %+v", code, result)
	}
}
