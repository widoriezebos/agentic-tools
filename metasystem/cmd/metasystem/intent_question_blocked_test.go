package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

// TestIntentAskAboutTheLaneNamesNoGoal: question ask --about lane asks with
// no goal, through the channel owner, and continues with the wait; --about
// with an authority kind, with a goal, or with another subject is refused
// before anything is asked.
func TestIntentAskAboutTheLaneNamesNoGoal(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	var inputs []channelAskInput
	owners.processes.ask = func(_ string, in channelAskInput) (channel.Question, []string, int, error) {
		inputs = append(inputs, in)
		return channel.Question{ID: "q-lane", About: in.About, State: "open", Thread: &channel.MessageRef{ThreadID: "t", ID: "m"}}, nil, 0, nil
	}
	run := func(args ...string) (int, intentResult) { return b.runJSON(owners, args...) }
	ask := []string{"question", "ask", "--about", "lane", "--question", "Return the conflicting branch?", "--fact", "merge conflict in internal/goal", "--option", "return: the seat fixes it"}

	code, result := run(ask...)
	if code != 0 || result.Outcome != intentConfirmed || len(inputs) != 1 || inputs[0].Goal != "" || inputs[0].About != "lane" || inputs[0].Kind != "other" {
		t.Fatalf("a lane question: code=%d %+v inputs=%+v", code, result, inputs)
	}
	if result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem question wait channel:q-lane" {
		t.Fatalf("a lane question continues with its wait: %+v", result.Next)
	}
	for _, target := range result.Targets {
		if target.Kind == "goal" {
			t.Fatalf("a lane question names no goal target: %+v", result.Targets)
		}
	}
	for _, refused := range [][]string{
		append(append([]string(nil), ask...), "--kind", "stop", "--budget", "1d/10/720m/1/3"),
		{"question", "ask", bedGoal, "--about", "lane", "--question", "Q?", "--option", "a: b"},
		{"question", "ask", "--about", "seat", "--question", "Q?", "--option", "a: b"},
	} {
		if code, result := run(refused...); code == 0 || result.Outcome != intentRefused {
			t.Fatalf("%v: code=%d %+v; want refused", refused, code, result)
		}
	}
	if len(inputs) != 1 {
		t.Fatalf("a refused ask reached the channel owner: %+v", inputs)
	}
}

// TestIntentQuestionListAnsweredGroupsByRefusal: question list --answered
// reads the ended questions since the window's start, groups them by
// refusal line (the first fact after the question) with counts and the
// median time to answer, most frequent first, leaves open questions out,
// and lists each one with --verbose.
func TestIntentQuestionListAnsweredGroupsByRefusal(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	root := b.root()
	now := time.Now().UTC()
	write := func(id string, record map[string]any) {
		t.Helper()
		record["id"], record["kind"] = id, "other"
		if _, ok := record["goal"]; !ok {
			record["goal"] = ""
		}
		writeQuestionFixture(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", id+".json"), record)
	}
	answered := func(opened time.Time, after time.Duration) map[string]any {
		return map[string]any{"text": "go", "at": opened.Add(after).Format(time.RFC3339Nano), "phase": "closed"}
	}
	at := func(ago time.Duration) time.Time { return now.Add(-ago) }
	write("a1", map[string]any{"about": "lane", "openedAt": at(5 * time.Hour), "state": "closed", "facts": []string{"Return it?", "push refused: HEAD does not contain origin's main"}, "answer": answered(at(5*time.Hour), time.Hour)})
	write("a2", map[string]any{"goal": bedGoal, "openedAt": at(4 * time.Hour), "state": "answered", "facts": []string{"Merge again?", "push refused: HEAD does not contain origin's main"}, "answer": answered(at(4*time.Hour), 3*time.Hour)})
	write("a3", map[string]any{"goal": bedGoal, "openedAt": at(3 * time.Hour), "state": "closed", "facts": []string{"Retry?", "push refused: HEAD does not contain origin's main"}, "answer": answered(at(3*time.Hour), 2*time.Hour)})
	write("w1", map[string]any{"about": "machine", "openedAt": at(2 * time.Hour), "state": "closed", "facts": []string{"Free disk?", "disk full on the proof bed"}})
	write("o1", map[string]any{"goal": bedGoal, "openedAt": at(time.Hour), "state": "open", "facts": []string{"Still open?", "an open refusal"}})
	write("old", map[string]any{"goal": bedGoal, "openedAt": at(10 * 24 * time.Hour), "state": "closed", "facts": []string{"Long ago?", "an old refusal"}, "answer": answered(at(10*24*time.Hour), time.Hour)})

	code, result := b.runJSON(owners, "question", "list", "--answered", "--since", "7d")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("list --answered: code=%d %+v", code, result)
	}
	data, _ := result.Data.(map[string]any)
	groups, _ := data["groups"].([]any)
	got := fmt.Sprint(groups)
	if len(groups) != 2 || !strings.Contains(fmt.Sprint(groups[0]), "push refused: HEAD does not contain origin's main") ||
		!strings.Contains(fmt.Sprint(groups[0]), "count:3") || !strings.Contains(fmt.Sprint(groups[0]), "medianToAnswer:2h0m0s") ||
		!strings.Contains(fmt.Sprint(groups[1]), "disk full on the proof bed") || !strings.Contains(fmt.Sprint(groups[1]), "withdrawn:1") ||
		strings.Contains(got, "an open refusal") || strings.Contains(got, "an old refusal") {
		t.Fatalf("groups = %v", groups)
	}
	if _, listed := data["questions"]; listed {
		t.Fatalf("the summary lists no single question: %+v", data)
	}
	_, verbose := b.runJSON(owners, "question", "list", "--answered", "--since", "7d", "--verbose")
	each, _ := verbose.Data.(map[string]any)["questions"].([]any)
	if len(each) != 4 || strings.Contains(fmt.Sprint(each), "o1") || strings.Contains(fmt.Sprint(each), "old") {
		t.Fatalf("--verbose lists each ended question in the window: %v", each)
	}
	if code, refused := b.runJSON(owners, "question", "list", "--since", "7d"); code == 0 || refused.Outcome != intentRefused {
		t.Fatalf("--since belongs to --answered: code=%d %+v", code, refused)
	}
}
