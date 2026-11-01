package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
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

func TestAPersonsDecisionRecordRunsTheAct(t *testing.T) {
	t.Parallel()
	b := newGrantEverythingBed(t)
	claimed := b.goalFile(bedGoal)
	claimed.Id = "claimed"
	for i := range claimed.History {
		claimed.History[i].Targets = []string{claimed.Id}
	}
	b.addGoal(claimed)
	queued := b.goalFile(bedGoal)
	makeQueued(queued)
	b.addGoal(queued)

	owners := b.owners()
	home := t.TempDir()
	owners.decisions.home = func() (string, error) { return home, nil }
	calls := 0
	var process intentProcess
	var child verbresult.Result
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "ENGINE", nil }}
	owners.delivery.ownerEnvelope = func(p intentProcess, verb string) (verbresult.Result, error) {
		calls++
		process = p
		command, rest, ok := resolveIntentArgv(p.argv[1:])
		if !ok {
			t.Fatalf("child argv: %v", p.argv)
		}
		var out, errout bytes.Buffer
		code := runIntentIn(command, rest, &out, &errout, p.dir, owners)
		var err error
		child, err = verbresult.Read(out.Bytes(), verb, code, errout.String())
		return child, err
	}
	run := func(flags, act []string) (int, intentResult, string) {
		t.Helper()
		words := append([]string{"decision", "record"}, flags...)
		if act != nil {
			words = append(append(words, "--"), act...)
		}
		code, out, errout := b.run(owners, words...)
		var got intentResult
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("record output: %v %s %s", err, out, errout)
		}
		return code, got, errout
	}
	flags := []string{"--json", "--class", "values", "--text", "choose the budget", "--rejected", "keep it", "--undo", "restore it", "--context", "queue"}
	act := []string{"goal", "budget", bedGoal, "1d/10/720m/1/3"}
	code, got, _ := run(flags, act)
	if code != 0 || got.Outcome != intentConfirmed || calls != 1 {
		t.Fatalf("budget: %d %+v calls=%d", code, got, calls)
	}
	if !slices.Equal(process.argv, []string{"ENGINE", "goal", "budget", "--json", bedGoal, "1d/10/720m/1/3"}) || process.dir != b.root() || len(process.env) != 0 {
		t.Fatalf("child context: %+v", process)
	}
	log := func() []humanauthority.Decision {
		t.Helper()
		lines, bad, err := humanauthority.ReadDecisions(home)
		if err != nil || bad != 0 {
			t.Fatalf("log: bad=%d err=%v", bad, err)
		}
		return lines
	}
	d := log()[0]
	now, _ := b.commandNow(b.root())
	if d.Actor != "human:Wido" || d.By != "Wido" || d.Grant != "" || d.Lineage != "" || d.Machine != "mac-cli" || d.Class != "values" || !d.At.Equal(now) || !slices.Equal(d.Argv, act) || d.Text != "choose the budget" || d.Rejected != "keep it" || d.Undo != "restore it" || d.Context != "queue" || d.ID == "" || d.Outcome != intentConfirmed || d.Words != child.Summary {
		t.Fatalf("decision: %+v", d)
	}
	var effect struct {
		Owner struct{ Tip, Detail, Outcome string } `json:"owner"`
	}
	var published struct {
		Owner struct{ Tip, Detail, Outcome string } `json:"owner"`
	}
	if err := json.Unmarshal(d.Effect, &effect); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(child.Data, &published); err != nil {
		t.Fatal(err)
	}
	if effect.Owner.Tip != b.repo.accepted || effect.Owner != published.Owner {
		t.Fatalf("effect=%+v publication=%+v tip=%s", effect, published, b.repo.accepted)
	}
	history := b.goalFile(bedGoal).History
	if history[len(history)-1].Actor != "human:Wido" {
		t.Fatalf("history: %+v", history)
	}
	if len(got.Targets) < 2 || got.Targets[0] != (intentTarget{Kind: "decision", ID: d.ID}) || got.Targets[1].ID != bedGoal {
		t.Fatalf("targets: %+v", got.Targets)
	}
	if code, got, _ := run(flags, act); code != 0 || got.Outcome != intentUnchanged || log()[1].Outcome != intentUnchanged {
		t.Fatalf("repeat: %d %+v", code, got)
	}
	for _, typed := range [][]string{{"goal", "pin", bedGoal, "m1e", "--json"}, {"goal", "pin", bedGoal, "--", "m1e"}} {
		if code, cleared := b.runJSON(owners, "goal", "pin", bedGoal, "--clear"); code != 0 {
			t.Fatalf("clear pin before child: %d %+v", code, cleared)
		}
		if code, got, _ := run(flags, typed); code != 0 || got.Outcome != intentConfirmed {
			t.Fatalf("pin: %d %+v", code, got)
		}
		want := append([]string{"ENGINE", "goal", "pin", "--json"}, typed[2:]...)
		if slices.Contains(typed, "--json") {
			want = append([]string{"ENGINE"}, typed...)
		}
		if !slices.Equal(process.argv, want) || b.goalFile(bedGoal).Pinned != "m1e" {
			t.Fatalf("json placement: %v want %v", process.argv, want)
		}
	}
	if code, got, _ := run(flags, []string{"goal", "pin", "NOSUCH", "m1e"}); code == 0 || code != child.Exit || got.Outcome != intentRefused || log()[4].Outcome != intentRefused || log()[4].Words != child.Summary || !bytes.Contains(log()[4].Effect, []byte(`"owner":`)) {
		t.Fatalf("refused child: %d %+v", code, got)
	}
	before, tip := calls, b.repo.accepted
	for _, row := range []struct {
		flags, act []string
		want       string
	}{
		{flags, nil, "act after --"},
		{append(slices.Clone(flags), "stray"), act, "before --"},
		{flags, []string{"decision", "record"}, "decision record"},
		{flags, []string{"status"}, "public"},
		{flags, []string{"goal", "pin", bedGoal, "m1e", "--json=false"}, "drop --json=false"},
		{flags, []string{"goal", "pin", "--help"}, "help"},
		{flags, []string{"goal", "pin", "--bogus"}, "--bogus"},
		{withoutOption(flags, "text"), act, "--text"},
		{withoutOption(flags, "rejected"), act, "--rejected"},
		{withoutOption(flags, "undo"), act, "--undo"},
		{append(slices.Clone(flags), "--lineage", "seat"), act, "grant check"},
		{flags, []string{"goal", "pin", bedGoal, "m1e", "--repo", t.TempDir()}, "two repositories"},
	} {
		if code, got, _ := run(row.flags, row.act); code != 2 || got.Outcome != intentRefused || !strings.Contains(got.Summary, row.want) {
			t.Errorf("%v %v: %d %+v want %q", row.flags, row.act, code, got, row.want)
		}
	}
	b.lineage = "agent"
	if code, got, _ := run(flags, act); code != 2 || !strings.Contains(got.Summary, "grant check") {
		t.Fatalf("agent: %d %+v", code, got)
	}
	b.lineage = ""
	owners.prove = fixedFixtureGoalAuthority
	if code, got, _ := run(flags, act); code != 2 || got.Outcome != intentRefused {
		t.Fatalf("fixture proof: %d %+v", code, got)
	}
	owners.prove = b.prove
	if calls != before || len(log()) != 5 || b.repo.accepted != tip {
		t.Fatalf("preflight changed state: calls=%d lines=%d", calls, len(log()))
	}
	sub := filepath.Join(b.root(), "sub")
	helmMust(t, os.MkdirAll(sub, 0700), os.MkdirAll(filepath.Join(sub, "sub"), 0700), os.WriteFile(filepath.Join(sub, "sub", "marker.txt"), []byte("marker"), 0600), os.WriteFile(filepath.Join(sub, "reason.txt"), []byte("subdirectory reason"), 0600), os.WriteFile(filepath.Join(b.root(), "reason.txt"), []byte("wrong reason"), 0600))
	command := mustIntentCommand(t, "decision record")
	var out, errout bytes.Buffer
	args := append(slices.Clone(flags), "--repo", "sub/marker.txt", "--", "goal", "release", "claimed", "--reason-file", "reason.txt")
	code = runIntentIn(command, args, &out, &errout, sub, owners)
	if code != 0 || process.dir != sub || flagValue(process.argv, "--repo") != b.root() || b.goalFile("claimed").Claimed != nil {
		t.Fatalf("relative release: %d %s %s process=%+v", code, out.String(), errout.String(), process)
	}
	history = b.goalFile("claimed").History
	if !strings.Contains(fmt.Sprint(history[len(history)-1]), "subdirectory reason") {
		t.Fatalf("release history: %+v", history)
	}
	owners.delivery.ownerEnvelope = func(intentProcess, string) (verbresult.Result, error) {
		return verbresult.Result{Outcome: verbresult.Unknown}, errors.New("broken answer")
	}
	if code, got, _ := run(flags, act); code != 1 || got.Outcome != intentFailed || log()[6].Outcome != intentFailed || !strings.Contains(log()[6].Words, "broken answer") {
		t.Fatalf("unreadable: %d %+v", code, got)
	}
	alert := newAlertBed(t)
	episode, _, err := steward.OpenAlert(b.root(), steward.AlertOpening{Owner: steward.PatternOwner("stagnation"), Work: "4gr18nm8t3nyev9sssda9jgtsq", Since: now.Add(-time.Hour), Now: now, Message: "stalled", Deliver: func(string, string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	owners.alerts = alert.owners().alerts
	owners.landing.home = func() (string, error) { return alert.home, nil }
	owners.delivery.ownerEnvelope = func(p intentProcess, verb string) (verbresult.Result, error) {
		process = p
		command, rest, _ := resolveIntentArgv(p.argv[1:])
		var out, errout bytes.Buffer
		code := runIntentIn(command, rest, &out, &errout, p.dir, owners)
		return verbresult.Read(out.Bytes(), verb, code, errout.String())
	}
	if code, got, _ := run(flags, []string{"alert", "ack", "here/" + episode.EpisodeID}); code != 0 || got.Outcome != intentConfirmed || string(log()[7].Effect) != `{"targets":[]}` {
		t.Fatalf("alert effect: %d %+v lines=%+v", code, got, log())
	}

	if code, got, _ := run(flags, []string{"goal", "show", bedGoal, "--repo", b.root()}); code != 0 || got.Outcome != intentConfirmed || !slices.Equal(process.argv, []string{"ENGINE", "goal", "show", "--json", bedGoal, "--repo", b.root()}) {
		t.Fatalf("child repository: %d %+v", code, got)
	}
	owners.decisions.home = func() (string, error) { return humanauthority.DecisionLogPath(home), nil }
	// The ordinary text stream must retain the entire unsaved record.
	words := append([]string{"decision", "record"}, flags[1:]...)
	code, _, errtext := b.run(owners, append(append(words, "--"), "goal", "prioritize", bedGoal, "3")...)
	if code != 1 || !strings.Contains(errtext, "unsaved decision: ") || !strings.Contains(errtext, "act ran (confirmed)") || b.goalFile(bedGoal).Priority != 3 {
		t.Fatalf("unsaved: %d %s", code, errtext)
	}
	raw := strings.SplitN(strings.SplitN(errtext, "unsaved decision: ", 2)[1], "\n", 2)[0]
	if err := json.Unmarshal([]byte(raw), &d); err != nil || d.Outcome != intentConfirmed {
		t.Fatalf("unsaved JSON: %s %v", raw, err)
	}
	if code, got, _ := run(flags, []string{"goal", "prioritize", bedGoal, "2"}); code != 1 || got.Outcome != intentPartial || len(got.Details) == 0 || !strings.Contains(got.Details[0], `"outcome":"confirmed"`) {
		t.Fatalf("partial result: %d %+v", code, got)
	}
}

type decisionChildFacts struct {
	Root   string
	Now    time.Time
	Parent int64
	Agent  bool
	Entry  goal.PowerOfAttorneyEntry
	Holder lease.CurrentHolderView
	Owner  map[string]any
}

func init() { testHelperCommands["decision-child-probe"] = decisionChildProbe }

func decisionChildProbe(words []string) int {
	os.Args = append([]string{os.Args[0]}, words...)
	raw, err := os.ReadFile(os.Getenv("METASYSTEM_DECISION_FACTS"))
	var facts decisionChildFacts
	if err != nil || json.Unmarshal(raw, &facts) != nil {
		fmt.Fprintln(os.Stderr, "child facts:", err)
		return 1
	}
	tree := person()
	pid, parent := int64(os.Getpid()), int64(os.Getppid())
	tree[parent] = tree[20]
	middle := tree[parent]
	middle.parent, middle.argv = 20, "metasystem decision record"
	if facts.Agent {
		middle.argv = "metasystem-fake-agent exec"
	}
	tree[parent] = middle
	node := middle
	node.parent, node.argv = parent, "metasystem "+strings.Join(words, " ")
	tree[pid] = node
	proof, proofErr := humanauthority.Prove(facts.Root, pid, tree, facts.Now)
	admitter := &attorneyAdmitter{owners: attorneyAdmitOwners{
		grants: func(string) ([]goal.PowerOfAttorneyEntry, error) {
			return []goal.PowerOfAttorneyEntry{facts.Entry}, nil
		},
		machine: func(string) (string, error) { return "mac-cli", nil }, fixture: func(string) bool { return false },
		holder: func(string) (lease.CurrentHolderView, error) { return facts.Holder, nil },
		classify: func(_ string, candidate int64) (lease.Classification, error) {
			if candidate != facts.Parent {
				return lease.Classification{}, fmt.Errorf("unexpected parent %d", candidate)
			}
			return lease.Classification{Class: lease.ClassMain, MainId: facts.Holder.MainId}, nil
		},
	}}
	admitted, ok := admitter.admit(facts.Root, parent, facts.Now)
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	seen := map[string]any{"argv": os.Args[1:], "cwd": cwd, "lineage": os.Getenv("METASYSTEM_OWNER_LINEAGE"), "parent": parent, "grant": admitted.Grant, "admitted": ok, "proof": proof.Outcome}
	if proofErr != nil {
		seen["proofError"] = proofErr.Error()
	}
	outcome, code := verbresult.Confirmed, 0
	if !proof.EnrolledTerminalFor(facts.Root) && !ok {
		outcome, code = verbresult.Refused, 2
	}
	data, _ := json.Marshal(map[string]any{"owner": facts.Owner, "seen": seen})
	if err := verbresult.Write(os.Stdout, verbresult.Result{Verb: words[0] + " " + words[1], Outcome: outcome, Summary: "child publication", Data: data}); err != nil {
		return 1
	}
	return code
}

func TestAPersonsDecisionRunsInARealChild(t *testing.T) {
	t.Parallel()
	for _, agent := range []bool{false, true} {
		t.Run(strconv.FormatBool(agent), func(t *testing.T) {
			t.Parallel()
			b := newGrantEverythingBed(t)
			owners := b.owners()
			home := t.TempDir()
			owners.decisions.home = func() (string, error) { return home, nil }
			now, _ := b.commandNow(b.root())
			facts := decisionChildFacts{Root: b.root(), Now: now, Parent: int64(os.Getpid()), Agent: agent, Owner: map[string]any{"outcome": "confirmed", "tip": "child-tip", "detail": "child detail"}}
			raw, _ := json.Marshal(facts)
			path := filepath.Join(t.TempDir(), "facts.json")
			helmMust(t, os.WriteFile(path, raw, 0600))
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			wrapper := filepath.Join(t.TempDir(), "engine")
			script := "#!/bin/sh\nexport GO_WANT_BATCH_E2E_COMMAND=1\nexec " + shellQuote(binary) + " decision-child-probe \"$@\"\n"
			helmMust(t, testexec.WriteFile(wrapper, []byte(script), 0700))
			fixture := testutil.Fixture(t)
			var answer verbresult.Result
			owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return wrapper, nil }, ownerEnvelope: func(p intentProcess, verb string) (verbresult.Result, error) {
				p.env = append(fixture.Env(p.env), "METASYSTEM_DECISION_FACTS="+path)
				var err error
				answer, err = runIntentOwnerEnvelope(p, verb)
				return answer, err
			}}
			code, out, errout := b.run(owners, "decision", "record", "--json", "--text", "choose", "--rejected", "leave", "--undo", "restore", "--", "goal", "prioritize", bedGoal, "3")
			var result intentResult
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("parent: %d %s %s %v", code, out, errout, err)
			}
			var data struct {
				Seen struct {
					Argv                []string
					Cwd, Lineage, Proof string
					Parent              int64
					Admitted            bool
				}
			}
			if err := json.Unmarshal(answer.Data, &data); err != nil {
				t.Fatal(err)
			}
			want := []string{"goal", "prioritize", "--json", bedGoal, "3"}
			if !slices.Equal(data.Seen.Argv, want) || data.Seen.Cwd != b.root() || data.Seen.Parent != int64(os.Getpid()) || data.Seen.Lineage != "" || data.Seen.Admitted {
				t.Fatalf("child context: %+v", data.Seen)
			}
			lines, bad, err := humanauthority.ReadDecisions(home)
			if err != nil || bad != 0 || len(lines) != 1 || !bytes.Contains(lines[0].Effect, []byte(`"tip":"child-tip"`)) || !bytes.Contains(lines[0].Effect, []byte(`"detail":"child detail"`)) {
				t.Fatalf("child line: %+v %d %v", lines, bad, err)
			}
			if !agent && (code != 0 || result.Outcome != intentConfirmed || data.Seen.Proof != humanauthority.OutcomeProven) || agent && (code != 2 || result.Outcome != intentRefused || data.Seen.Proof != humanauthority.OutcomeAgent) {
				t.Fatalf("proof: agent=%v code=%d result=%+v seen=%+v", agent, code, result, data.Seen)
			}
			t.Logf("decision record --json --text choose --rejected leave --undo restore -- goal prioritize %s 3: outcome=%s, child parent=%d, proof=%s, effect=%s", bedGoal, result.Outcome, data.Seen.Parent, data.Seen.Proof, lines[0].Effect)
		})
	}
}
