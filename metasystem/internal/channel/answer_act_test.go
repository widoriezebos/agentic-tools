package channel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func contractRequest(root string) AskRequest {
	return AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "machine", Facts: []string{"the build stopped"}, Now: time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC), Contract: true,
		Plain: &Plain{Decision: "g: the build of unit u stopped at 01:00 (a build step failed)", IfNothing: "it waits; nothing builds or lands until you choose"},
		Options: []Option{
			{Label: "Stop here", Consequence: "the work stays in its worktree; nothing builds, lands or concludes. This question stays open."},
			{Label: "Continue", Consequence: "the builder corrects and finishes the unit from the prepared brief; one more round of the goal's budget. Undo: metasystem work stop g.", Act: []string{"metasystem", "work", "revise", "g", "--work", "u", "--after", "1", "--brief", "brief.md", "--reason", "the machinery stopped: a build step failed"}},
		},
		Recommendation: "Continue"}
}

func noQuestionFiles(t *testing.T, root string) {
	t.Helper()
	if files, _ := filepath.Glob(filepath.Join(channelRoot(root), "questions", "*.json")); len(files) != 0 {
		t.Fatalf("a refused ask wrote %v", files)
	}
}

func TestContractAskRefusesWordsAPersonCannotRead(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		amend func(*AskRequest)
	}{
		{"no decision", func(r *AskRequest) { r.Plain.Decision = " " }},
		{"no if-nothing", func(r *AskRequest) { r.Plain.IfNothing = "" }},
		{"an option without a consequence", func(r *AskRequest) { r.Options[0].Consequence = "" }},
		{"a decision of 281 runes", func(r *AskRequest) { r.Plain.Decision = strings.Repeat("é", 281) }},
		{"a consequence of 201 runes", func(r *AskRequest) { r.Options[1].Consequence = strings.Repeat("é", 201) }},
		{"a recommendation that is no label", func(r *AskRequest) { r.Recommendation = "Maybe" }},
		{"a repeated label", func(r *AskRequest) { r.Options[0].Label = "continue" }},
		{"a 10-hex run", func(r *AskRequest) { r.Plain.Decision += " at abcdef0123" }},
		{"a path", func(r *AskRequest) { r.Options[0].Consequence = "the work stays in /Users/x" }},
		{"a pid", func(r *AskRequest) { r.Plain.IfNothing = "pid 6378 waits" }},
		{"a generation", func(r *AskRequest) { r.Plain.Decision = "the seat restarted at generation 2" }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			request := contractRequest(root)
			row.amend(&request)
			if _, err := Ask(request); err == nil || err.Error() == "" {
				t.Fatalf("the contract ask was accepted: %v", err)
			}
			noQuestionFiles(t, root)
		})
	}
	t.Run("a legacy ask keeps today's text", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		q, err := Ask(AskRequest{RepoRoot: root, About: "lane", Kind: "other", Machine: "m", Facts: []string{"metasystem landing resume"}, Now: time.Unix(10, 0),
			Plain: &Plain{Decision: "the lane on pid 6378 at /Users/x stopped at abcdef0123", Act: "metasystem landing resume", IfNothing: "the lane does not do this"}})
		want := "The lane on pid 6378 at /Users/x stopped at abcdef0123.\nTo go on: metasystem landing resume\nIf you do nothing: the lane does not do this."
		if err != nil || q.Text != want {
			t.Fatalf("legacy ask: %q %v, want %q", q.Text, err, want)
		}
	})
}

func TestContractQuestionRendersInContractOrder(t *testing.T) {
	t.Parallel()
	for _, codeOff := range []bool{false, true} {
		root := t.TempDir()
		answer := "Reply with the word and your code: Continue / Stop here"
		if codeOff {
			answer = "Reply: Continue / Stop here"
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.human.answer-code=off\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		request := contractRequest(root)
		request.Plain.Decision = "D" + strings.Repeat("é", 278) + "."
		q, err := Ask(request)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Join([]string{request.Plain.Decision, "",
			"★ Continue — " + request.Options[1].Consequence,
			"Stop here — " + request.Options[0].Consequence, "",
			"If you do nothing: it waits; nothing builds or lands until you choose.", answer}, "\n")
		if q.Text != want {
			t.Fatalf("contract text (code off %t):\n%s\nwant:\n%s", codeOff, q.Text, want)
		}
		if got := ReplyInstructionsAt(root, q); got != answer {
			t.Fatalf("question show's answer line = %q, want %q", got, answer)
		}
	}
}
