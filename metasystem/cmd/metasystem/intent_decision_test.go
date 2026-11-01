package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestDecisionListAndShowReadTheLog(t *testing.T) {
	t.Parallel()
	bed := recordsLayoutBed(t)
	bed.owners.textEnv = func(io.Writer) textui.Env { return layoutEnv(t, layoutNow, "", bed.cwd) }
	home := t.TempDir()
	bed.owners.decisions.home = func() (string, error) { return home, nil }
	first := humanauthority.Decision{ID: "first", At: layoutNow.Add(-time.Hour), By: "Wido", Grant: "grant-1", Class: "ordering", Text: "G first", Rejected: "leave G last", Undo: "goal prioritize G 2", Context: "queue", Argv: []string{"goal", "prioritize", "G", "3"}, Outcome: "confirmed", Words: "G prioritized", Effect: json.RawMessage(`{"owner":{"tip":"abc123","detail":"published"}}`)}
	second := first
	second.ID, second.At, second.Words, second.Effect = "second", layoutNow, "G unchanged", json.RawMessage(`{"targets":[]}`)
	for _, d := range []humanauthority.Decision{first, second} {
		if err := humanauthority.AppendDecision(home, d); err != nil {
			t.Fatal(err)
		}
	}
	run := func(words ...string) string {
		t.Helper()
		code, out, errout := runLayoutCase(t, layoutCase{args: words}, bed)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s%s", words, code, out, errout)
		}
		return out
	}
	list := run("decision", "list")
	for _, words := range []string{"Checkout lease: decisions", "Decisions in a person's name", "first", "second", "09:58", "10:58", "ordering", "confirmed", "G prioritized", "G unchanged"} {
		if !strings.Contains(list, words) {
			t.Errorf("list lacks %q: %s", words, list)
		}
	}
	if strings.Index(list, "second") >= strings.Index(list, "first") {
		t.Errorf("list is not newest first: %s", list)
	}
	var listed struct {
		Data struct {
			Log     []humanauthority.Decision `json:"log"`
			Records []any                     `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(run("decision", "list", "--json")), &listed); err != nil || len(listed.Data.Log) != 2 || listed.Data.Log[0].ID != second.ID || len(listed.Data.Records) != 1 {
		t.Fatalf("list data: %+v, %v", listed, err)
	}
	show := run("decision", "show", first.ID)
	for _, words := range []string{"Decision first: confirmed", "Wido", "grant-1", "ordering", first.Text, first.Rejected, first.Undo, first.Context, "goal prioritize G 3", "abc123", "published"} {
		if !strings.Contains(show, words) {
			t.Errorf("show lacks %q: %s", words, show)
		}
	}
	var shown struct {
		Data struct {
			Decision humanauthority.Decision `json:"decision"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(run("decision", "show", first.ID, "--json")), &shown); err != nil || shown.Data.Decision.ID != first.ID {
		t.Fatalf("show data: %+v, %v", shown, err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, shown.Data.Decision.Effect); err != nil || compact.String() != string(first.Effect) {
		t.Fatalf("show effect: %s, %v", shown.Data.Decision.Effect, err)
	}
	if show := run("decision", "show", second.ID); !strings.Contains(show, "none") {
		t.Errorf("empty targets: %s", show)
	}
	second.ID, second.Effect = "targeted", json.RawMessage(`{"targets":[{"kind":"goal","id":"G"}]}`)
	if err := humanauthority.AppendDecision(home, second); err != nil {
		t.Fatal(err)
	}
	if show := run("decision", "show", second.ID); !strings.Contains(show, "goal G") {
		t.Errorf("named targets: %s", show)
	}
	// A project record wins an id shared with the host log.
	first.ID = layoutDecisionID
	if err := humanauthority.AppendDecision(home, first); err != nil {
		t.Fatal(err)
	}
	if show := run("decision", "show", layoutDecisionID); !strings.Contains(show, "Checkout lease: decisions") || strings.Contains(show, "grant-1") {
		t.Errorf("project record: %s", show)
	}
	bed.owners.decisions.home = func() (string, error) { return humanauthority.DecisionLogPath(home), nil }
	var unreadable struct {
		Details []string `json:"details"`
		Data    struct {
			Records []any `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(run("decision", "list", "--json")), &unreadable); err != nil || len(unreadable.Details) != 1 || len(unreadable.Data.Records) != 1 {
		t.Fatalf("unreadable log: %+v, %v", unreadable, err)
	}
}
