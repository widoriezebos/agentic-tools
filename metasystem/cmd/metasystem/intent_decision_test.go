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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestAGrantKeepsItsReason(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, reason, want string
	}{
		{"quoted", `  nights "alone"  `, `nights "alone"`},
		{"omitted", "", "no reason given"},
		{"blank", " \t ", "no reason given"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newGrantEverythingBed(t)
			args := []string{"grant", "add", "--acts", "everything", "--for", "24h"}
			if tc.reason != "" {
				args = append(args, "--reason", tc.reason)
			}
			code, got := b.runJSON(b.owners(), args...)
			data, _ := got.Data.(map[string]any)
			if code != 0 || got.Outcome != intentConfirmed || data["reason"] != tc.want {
				t.Fatalf("grant with reason: %d %+v", code, got)
			}
			code, listed := b.runJSON(b.owners(), "grant", "list")
			data, _ = listed.Data.(map[string]any)
			grants, ok := data["grants"].([]any)
			if code != 0 || !ok || len(grants) != 1 {
				t.Fatalf("grant list: %d %+v", code, listed)
			}
			if view := grants[0].(map[string]any); view["reason"] != tc.want {
				t.Fatalf("listed reason = %v, want %q", view["reason"], tc.want)
			}
			endpoint, err := b.dependencies().endpoint(b.root())
			if err != nil {
				t.Fatal(err)
			}
			_, entries, err := goal.GeneralGrantsAt(endpoint)
			if err != nil || len(entries) != 1 || entries[0].Reason != tc.want {
				t.Fatalf("recorded reason: %+v, %v", entries, err)
			}
			code, got = b.runJSON(b.owners(), args...)
			if code != 0 || got.Outcome != intentUnchanged {
				t.Fatalf("same reason again: %d %+v", code, got)
			}
			args = append(withoutOption(args, "reason"), "--reason", "a different reason")
			code, got = b.runJSON(b.owners(), args...)
			if code != 0 || got.Outcome != intentConfirmed {
				t.Fatalf("changed reason: %d %+v", code, got)
			}
			_, entries, err = goal.GeneralGrantsAt(endpoint)
			if err != nil || len(entries) != 2 {
				t.Fatalf("replacement reason: %+v, %v", entries, err)
			}
			old, current := entries[0], entries[1]
			if old.Revoked == "" {
				old, current = current, old
			}
			if current.Reason != "a different reason" || current.Revoked != "" || old.Reason != tc.want || old.Revoked == "" {
				t.Fatalf("replacement reason: old %+v, current %+v", old, current)
			}
			before := b.repo.accepted
			code, got = b.runJSON(b.owners(), "grant", "add", "--tiers", "1", "--acts", "approve",
				"--until", "2026-09-02", "--reason", "x")
			if code == 0 || got.Outcome != intentRefused || !strings.Contains(got.Summary, "--reason") || b.repo.accepted != before {
				t.Fatalf("scoped reason: %d %+v", code, got)
			}
			root := b.rootRecord()
			root.PowerOfAttorney = []goal.PowerOfAttorneyEntry{{ID: entries[0].ID, By: entries[0].By,
				Tiers: []uint8{1}, Verbs: []string{"approve"}, Since: entries[0].Since, Expires: "2026-09-02"}}
			for _, reason := range []string{`""`, `"x"`, `x`} {
				content := strings.Replace(string(goal.RenderRoot(root)), " verbs=approve", " reason="+reason+" verbs=approve", 1)
				if _, problems := goal.ParseRoot([]byte(content)); len(problems) == 0 {
					t.Fatalf("scoped ledger accepted reason=%s", reason)
				}
			}
		})
	}
}

func TestAnOverruleShowsItsImpactFirst(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"text", "json", "child", "targets", "disposition"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newDecisionAgentBed(t)
			if code, got := b.record([]string{"--class", "ordering"}, []string{"goal", "prioritize", bedGoal, "3"}); code != 0 || got.Outcome != intentConfirmed {
				t.Fatalf("original act: %d %+v", code, got)
			}
			read := func() []humanauthority.Decision {
				t.Helper()
				lines, bad, err := humanauthority.ReadDecisions(b.home)
				if err != nil || bad != 0 {
					t.Fatalf("log: %d %v", bad, err)
				}
				return lines
			}
			original := read()[0]
			if mode == "targets" || mode == "disposition" {
				original.ID = "other"
				original.Effect = json.RawMessage(`{"targets":[{"kind":"goal","id":"G"}]}`)
				if mode == "disposition" {
					original.Argv = nil
					original.Actor, original.By, original.Grant, original.Class = launch.SeatOwnerLineage, "", "", ""
					original.Disposition = &humanauthority.Disposition{Row: "overdue-goal", Subject: bedGoal, Kind: humanauthority.DisposeAsk}
				}
				if err := humanauthority.AppendDecision(b.home, original); err != nil {
					t.Fatal(err)
				}
			}
			before, calls, count := b.repo.accepted, b.calls, len(read())
			b.lineage, b.owners.prove = "", b.prove
			words := []string{"--overrules", original.ID, "--text", "G stays at 2"}
			if mode != "text" {
				words = append(words, "--json")
			}
			if mode == "child" {
				words = append(words, "--undo", "restore priority", "--", "goal", "prioritize", bedGoal, "2")
			}
			impact := ""
			visible := &unitImpactWriter{observe: func(text string) {
				if impact != "" {
					return
				}
				impact = text
				if !strings.HasPrefix(text, "Decision "+original.ID+" (") || len(read()) != count || b.calls != calls || b.repo.accepted != before || b.goalFile(bedGoal).Priority != 3 {
					t.Fatalf("impact must precede the act and append: %q lines=%d calls=%d", text, len(read()), b.calls)
				}
			}}
			var out, errout bytes.Buffer
			var stdout, stderr io.Writer = &out, &errout
			if mode == "text" {
				stdout = visible
			} else {
				stderr = visible
			}
			command, _ := findIntentAction("decision", "record")
			if code := runIntentIn(command, words, stdout, stderr, b.root(), b.owners); code != 0 {
				t.Fatalf("overrule: %d %s %s", code, out.String(), errout.String())
			}
			lines := read()
			last := lines[len(lines)-1]
			if len(lines) != count+1 || impact == "" || last.Overrules != original.ID || last.Impact+"\n" != impact || last.Rejected != "decision "+original.ID+" as recorded" || last.Actor != "human:Wido" || last.Grant != "" {
				t.Fatalf("overrule: %+v impact=%q", last, impact)
			}
			expect := []string{"in Wido's name under grant " + original.Grant, "class ordering"}
			if mode == "disposition" {
				expect = []string{"by " + launch.SeatOwnerLineage, "disposed of overdue-goal for " + bedGoal + ": ask", "it changes nothing that ran"}
			} else {
				expect = append(expect, "ran goal prioritize "+bedGoal+" 3: confirmed", "it does not undo that act", "To undo it: "+original.Undo)
				if mode == "targets" {
					expect = append(expect, "goal G")
				} else {
					var effect struct{ Owner struct{ Tip, Detail string } }
					if err := json.Unmarshal(original.Effect, &effect); err != nil {
						t.Fatal(err)
					}
					expect = append(expect, effect.Owner.Tip, effect.Owner.Detail)
				}
			}
			for _, text := range expect {
				if !strings.Contains(impact, text) {
					t.Errorf("impact lacks %q: %s", text, impact)
				}
			}
			if mode != "text" {
				var got struct {
					Data struct {
						Impact   string
						Decision humanauthority.Decision
					}
				}
				decoder := json.NewDecoder(&out)
				if err := decoder.Decode(&got); err != nil || got.Data.Impact != last.Impact || got.Data.Decision.ID != last.ID {
					t.Fatalf("envelope: %+v %v", got, err)
				}
				if err := decoder.Decode(new(any)); err != io.EOF {
					t.Fatalf("stdout must contain one envelope: %v", err)
				}
			}
			if mode == "child" {
				if b.calls != calls+1 || b.goalFile(bedGoal).Priority != 2 {
					t.Fatal("the replacement act did not run")
				}
			} else if b.calls != calls || b.repo.accepted != before {
				t.Fatal("an overrule without a child changed the ledger")
			}
			for _, row := range []struct {
				flags            []string
				lineage, problem string
			}{
				{[]string{"--overrules", original.ID, "--text", "retain it"}, launch.SeatOwnerLineage, "an overrule is a person's decision"},
				{[]string{"--overrules", original.ID, "--lineage", "seat", "--text", "retain it"}, "", "an overrule is a person's decision"},
				{[]string{"--overrules", "NOSUCH", "--text", "retain it"}, "", "NOSUCH"},
				{[]string{"--overrules", original.ID}, "", "--text"},
			} {
				b.lineage = row.lineage
				before, calls, count := b.repo.accepted, b.calls, len(read())
				code, output, errors := b.run(b.owners, append([]string{"decision", "record", "--json"}, row.flags...)...)
				var got intentResult
				if err := json.Unmarshal([]byte(output), &got); err != nil || code != 2 || got.Outcome != intentRefused || !strings.Contains(got.Summary, row.problem) || errors != "" || len(read()) != count || b.calls != calls || b.repo.accepted != before {
					t.Fatalf("refused overrule: %d %+v %s %v", code, got, errors, err)
				}
				if row.problem == "NOSUCH" && (got.Next == nil || !slices.Equal(got.Next.Argv, []string{"metasystem", "decision", "list"})) {
					t.Fatalf("missing recovery command: %s", output)
				}
			}
		})
	}
}

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
		{flags, []string{"goal", "pin", bedGoal, "m1e", "--repo", t.TempDir()}, "two repositories"},
	} {
		if code, got, _ := run(row.flags, row.act); code != 2 || got.Outcome != intentRefused || !strings.Contains(got.Summary, row.want) {
			t.Errorf("%v %v: %d %+v want %q", row.flags, row.act, code, got, row.want)
		}
	}
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
	act, _ := attorneyActClass(os.Args[1:], nil, "mac-cli", facts.Holder.OwnerLineage)
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	seen := map[string]any{"argv": os.Args[1:], "cwd": cwd, "lineage": os.Getenv("METASYSTEM_OWNER_LINEAGE"), "parent": parent, "grant": admitted.Grant, "class": act, "admitted": ok, "proof": proof.Outcome}
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

type decisionAgentBed struct {
	*grantEverythingBed
	owners       intentOwners
	home, grant  string
	admitter     *attorneyAdmitter
	trace        bytes.Buffer
	process      intentProcess
	child        verbresult.Result
	calls, locks int
}

func newDecisionAgentBed(t *testing.T) *decisionAgentBed {
	t.Helper()
	b := newGrantEverythingBed(t)
	g := b.goalFile(bedGoal)
	makeQueued(g)
	b.addGoal(g)
	code, got := b.runJSON(b.owners(), "grant", "add", "--acts", "everything", "--classes", "ordering,recovery", "--lineage", "seat", "--for", "24h")
	if code != 0 || got.Outcome != intentConfirmed {
		t.Fatalf("person's grant: %d %+v", code, got)
	}
	d := &decisionAgentBed{grantEverythingBed: b, home: t.TempDir(), grant: got.Targets[0].ID}
	b.lineage, b.holder.OwnerLineage = launch.SeatOwnerLineage, launch.SeatOwnerLineage
	d.admitter = &attorneyAdmitter{owners: attorneyAdmitOwners{
		endpoint: b.dependencies().endpoint, machine: b.dependencies().machine,
		decision: func() string { return "" },
		args:     func() []string { return d.process.argv[1:] },
		verb:     func() string { return publicVerb(d.process.argv[1:]) },
		fixture:  func(string) bool { return false },
		holder:   func(string) (lease.CurrentHolderView, error) { return b.holder, nil },
		classify: func(_ string, pid int64) (lease.Classification, error) {
			if pid != int64(os.Getppid()) {
				t.Fatalf("preflight pid=%d, want parent %d", pid, os.Getppid())
			}
			return lease.Classification{Class: lease.ClassMain, MainId: b.holder.MainId}, nil
		},
		lockShared: func(string) error { d.locks++; return nil }, stderr: &d.trace,
		notice: func(n admissionNotice) { fmt.Fprintln(n.w, n.line) },
	}}
	d.owners = b.owners()
	d.owners.decisions = decisionOwners{home: func() (string, error) { return d.home, nil }, grant: d.admitter.grantFor}
	d.owners.attorney.admit = d.admitter.admit
	d.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		p, err := humanauthority.Prove(root, 95, person(), at)
		if err == nil {
			t.Fatal("agent reached the enrolled terminal")
		}
		if grant, ok := d.admitter.admit(root, int64(os.Getppid()), at); ok {
			return humanauthority.HelmProof(root, grant, at)
		}
		return p, err
	}
	d.owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "ENGINE", nil }, ownerEnvelope: func(p intentProcess, verb string) (verbresult.Result, error) {
		d.calls++
		d.process = p
		command, rest, ok := resolveIntentArgv(p.argv[1:])
		if !ok {
			t.Fatalf("child argv: %v", p.argv)
		}
		childOwners := d.owners
		parentDecision := d.admitter.owners.decision
		defer func() { d.admitter.owners.decision = parentDecision }()
		for _, env := range p.env {
			if strings.HasPrefix(env, "METASYSTEM_DECISION=") {
				decision := strings.TrimPrefix(env, "METASYSTEM_DECISION=")
				d.admitter.owners.decision = func() string { return decision }
			}
			if strings.HasPrefix(env, "METASYSTEM_OWNER_LINEAGE=") {
				lineage := strings.TrimPrefix(env, "METASYSTEM_OWNER_LINEAGE=")
				childOwners.dependencies.ownerLineage = func() string { return lineage }
			}
		}
		var out, errout bytes.Buffer
		code := runIntentIn(command, rest, &out, &errout, p.dir, childOwners)
		var err error
		d.child, err = verbresult.Read(out.Bytes(), verb, code, errout.String())
		return d.child, err
	}}
	return d
}

func (b *decisionAgentBed) record(flags, act []string) (int, intentResult) {
	b.t.Helper()
	words := append([]string{"decision", "record", "--json", "--text", "G before H", "--rejected", "leave G at 2", "--undo", "metasystem goal prioritize G 2"}, flags...)
	code, out, errout := b.run(b.owners, append(append(words, "--"), act...)...)
	var got intentResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		b.t.Fatalf("record: %d %s %s %v", code, out, errout, err)
	}
	return code, got
}

func TestADelegatedClassRunsOnlyInsideADecision(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, classes, class string }{
		{"delegated", "run-release,ordering", "run-release"},
		{"unclassed", "", "run-release"},
		{"not named", "ordering", "run-release"},
		{"ordinary", "run-release,ordering", "ordering"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newDecisionAgentBed(t)
			b.lineage = ""
			args := []string{"grant", "add", "--acts", "everything", "--lineage", "seat", "--for", "24h"}
			if row.classes != "" {
				args = append(args, "--classes", row.classes)
			}
			code, got := b.runJSON(b.grantEverythingBed.owners(), args...)
			if code != 0 || got.Outcome != intentConfirmed {
				t.Fatalf("delegated grant: %d %+v", code, got)
			}
			b.grant, b.lineage = got.Targets[0].ID, launch.SeatOwnerLineage
			now, _ := b.commandNow(b.root())
			act := []string{"work", "stop", "run:R"}
			b.process.argv = append([]string{"ENGINE"}, act...)
			for _, token := range []string{"", "two words", " D", "D\n", "D\tE", "D\u00a0E"} {
				b.admitter.owners.decision = func() string { return token }
				if proof, ok := b.admitter.admit(b.root(), int64(os.Getppid()), now); ok {
					t.Fatalf("direct act with decision %q admitted: %+v", token, proof)
				}
			}
			b.admitter.owners.decision = func() string { return "" }
			if b.locks != 0 || b.trace.Len() != 0 {
				t.Fatalf("direct refusal left locks=%d notice=%s", b.locks, b.trace.String())
			}
			if _, err := os.Stat(humanauthority.AttorneyLogPath(b.root())); !os.IsNotExist(err) {
				t.Fatalf("direct refusal wrote an admission log: %v", err)
			}
			var proof *humanauthority.HelmGrant
			prove := b.owners.prove
			b.owners.prove = func(root string, pid int64, reader humanauthority.Reader, by, tty string, at time.Time) (humanauthority.Proof, error) {
				p, err := prove(root, pid, reader, by, tty, at)
				proof = p.Helm
				return p, err
			}
			if row.class == "ordering" {
				act = []string{"goal", "prioritize", bedGoal, "3"}
			}
			code, got = b.record([]string{"--class", row.class}, act)
			lines, bad, err := humanauthority.ReadDecisions(b.home)
			if err != nil || bad != 0 {
				t.Fatalf("decision log: %d %v", bad, err)
			}
			if row.classes == "" || row.classes == "ordering" {
				if code != 2 || got.Outcome != intentRefused || !strings.Contains(got.Summary, "the grant does not carry run-release") || b.calls != 0 || b.locks != 0 || b.trace.Len() != 0 || len(lines) != 0 {
					t.Fatalf("ungranted class reached child: %d %+v calls=%d lines=%+v", code, got, b.calls, lines)
				}
				return
			}
			if b.calls != 1 || len(lines) != 1 || proof == nil || proof.By != "Wido" || proof.Grant != b.grant || lines[0].Class != row.class || lines[0].Outcome != got.Outcome {
				t.Fatalf("decision child: %d %+v proof=%+v lines=%+v", code, got, proof, lines)
			}
			log, err := os.ReadFile(humanauthority.AttorneyLogPath(b.root()))
			if err != nil {
				t.Fatal(err)
			}
			if row.class == "ordering" {
				if code != 0 || got.Outcome != intentConfirmed || proof.Act != "" || proof.Decision != "" || proof.Session != "" || bytes.Contains(log, []byte(" decision=")) {
					t.Fatalf("ordinary admission changed: %d %+v proof=%+v log=%s", code, got, proof, log)
				}
				return
			}
			id := lines[0].ID
			if code == 0 || got.Outcome != intentRefused || !strings.Contains(got.Summary, "this shell acts under the helm or a grant") || proof.Act != "run-release" || proof.Decision != id || proof.Session != b.holder.MainId {
				t.Fatalf("delegated proof or person gate: %d %+v proof=%+v", code, got, proof)
			}
			if !bytes.Contains(log, []byte(" decision="+id+" act=run-release")) || !strings.Contains(b.trace.String(), "runs as the steward's own act under decision "+id+", class run-release") {
				t.Fatalf("delegated trace: %s\n%s", log, b.trace.String())
			}
			t.Logf("decision record --class run-release -- work stop run:R: decision=%s, proof=%+v, outcome=%s", id, proof, got.Outcome)
		})
	}
}

func TestAnAgentsDecisionRecordRunsUnderItsGrant(t *testing.T) {
	t.Parallel()
	b := newDecisionAgentBed(t)
	now, _ := b.commandNow(b.root())
	preflight, ok := b.admitter.grantFor(b.root(), int64(os.Getppid()), now, []string{"goal", "prioritize", "--json", bedGoal, "3"})
	if !ok || preflight.entry.ID != b.grant || preflight.act != "ordering" || b.locks != 0 || b.trace.Len() != 0 {
		t.Fatalf("preflight: %+v ok=%v locks=%d notice=%s", preflight, ok, b.locks, b.trace.String())
	}
	if _, err := os.Stat(humanauthority.AttorneyLogPath(b.root())); !os.IsNotExist(err) {
		t.Fatalf("preflight wrote an admission log: %v", err)
	}
	for i, priority := range []string{"3", "1", "2", "3"} {
		flags := []string{"--class", "ordering"}
		if i == 1 {
			b.holder.MainId = "main-2"
		}
		if i == 2 {
			b.lineage = ""
			flags = append(flags, "--lineage", "lin-x")
		}
		if i == 3 {
			b.lineage = launch.SeatOwnerLineage
			run := b.owners.delivery.ownerEnvelope
			b.owners.delivery.ownerEnvelope = func(p intentProcess, verb string) (verbresult.Result, error) {
				child, err := run(p, verb)
				b.lineage = ""
				if code, got := b.runJSON(b.grantEverythingBed.owners(), "grant", "revoke", b.grant); code != 0 || got.Outcome != intentConfirmed {
					t.Fatalf("revoke after act: %d %+v", code, got)
				}
				return child, err
			}
		}
		act := []string{"goal", "prioritize", bedGoal, priority}
		code, got := b.record(flags, act)
		if code != 0 || got.Outcome != intentConfirmed || b.calls != i+1 {
			t.Fatalf("agent act: %d %+v calls=%d", code, got, b.calls)
		}
		if !slices.Equal(b.process.argv, []string{"ENGINE", "goal", "prioritize", "--json", bedGoal, priority}) || b.process.dir != b.root() {
			t.Fatalf("child: %+v", b.process)
		}
		wantEnv := []string{"METASYSTEM_DECISION=" + got.Targets[0].ID}
		if i == 2 {
			wantEnv = append(wantEnv, "METASYSTEM_OWNER_LINEAGE=lin-x")
		}
		if !slices.Equal(b.process.env, wantEnv) {
			t.Fatalf("child environment: %v", b.process.env)
		}
		lines, bad, err := humanauthority.ReadDecisions(b.home)
		if err != nil || bad != 0 || len(lines) != i+1 {
			t.Fatalf("lines: %+v %d %v", lines, bad, err)
		}
		d := lines[i]
		if d.Actor != b.holder.MainId || d.Lineage != launch.SeatOwnerLineage || d.Grant != b.grant || d.By != "Wido" || d.Class != "ordering" || d.Outcome != intentConfirmed || d.Rejected != "leave G at 2" || !slices.Equal(d.Argv, act) {
			t.Fatalf("agent decision: %+v", d)
		}
		var effect, published struct{ Owner struct{ Tip, Detail string } }
		if err := json.Unmarshal(d.Effect, &effect); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b.child.Data, &published); err != nil {
			t.Fatal(err)
		}
		if effect.Owner != published.Owner || effect.Owner.Tip == "" {
			t.Fatalf("effect=%+v child=%+v", effect, published)
		}
		if i != 3 && effect.Owner.Tip != b.repo.accepted {
			t.Fatalf("effect tip=%s ledger=%s", effect.Owner.Tip, b.repo.accepted)
		}
		history := b.goalFile(bedGoal).History
		if history[len(history)-1].Actor != "human:Wido" || !strings.Contains(b.trace.String(), "grant "+b.grant) {
			t.Fatalf("history=%+v notice=%s", history, b.trace.String())
		}
	}
	t.Run("unrestricted grant", func(t *testing.T) {
		t.Parallel()
		b := newDecisionAgentBed(t)
		b.lineage = ""
		code, got := b.runJSON(b.grantEverythingBed.owners(), "grant", "add", "--acts", "everything", "--lineage", "seat", "--for", "24h")
		if code != 0 || got.Outcome != intentConfirmed {
			t.Fatalf("unrestricted grant: %d %+v", code, got)
		}
		b.lineage = launch.SeatOwnerLineage
		if code, got := b.record([]string{"--class", "ordering"}, []string{"goal", "prioritize", bedGoal, "3"}); code != 0 || got.Outcome != intentConfirmed {
			t.Fatalf("act under unrestricted grant: %d %+v", code, got)
		}
	})
}

func TestAnAgentIsRefusedBeforeTheChild(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name       string
		flags, act []string
		want       string
		revoke     bool
	}{
		{"mismatch", []string{"--class", "recovery"}, []string{"goal", "prioritize", bedGoal, "3"}, "ordering, not recovery", false},
		{"unknown", []string{"--class", "values"}, []string{"goal", "prioritize", bedGoal, "3"}, strings.Join(goal.AttorneyClasses, ", "), false},
		{"unclassified", []string{"--class", "ordering"}, []string{"goal", "budget", bedGoal, "1d/10/720m/1/3"}, "in no class", false},
		{"resume", []string{"--class", "recovery"}, []string{"landing", "resume"}, "in no class", false},
		{"revive", []string{"--class", "recovery"}, []string{"machine", "revive", "mac-cli"}, "in no class", false},
		{"stop-run", []string{"--class", "recovery"}, []string{"work", "stop", "run:ID"}, "run-release, not recovery", false},
		{"revoked", []string{"--class", "ordering"}, []string{"goal", "prioritize", bedGoal, "3"}, "grant", true},
		{"missing", nil, []string{"goal", "prioritize", bedGoal, "3"}, strings.Join(goal.AttorneyClasses, ", "), false},
		{"repository", []string{"--class", "ordering", "--repo", "."}, []string{"goal", "prioritize", bedGoal, "3"}, "own checkout", false},
		{"child-repository", []string{"--class", "ordering"}, []string{"goal", "prioritize", bedGoal, "3", "--repo", "."}, "own checkout", false},
		{"grant-class", []string{"--class", "acks"}, []string{"alert", "ack", "A"}, "acks", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newDecisionAgentBed(t)
			if row.revoke {
				b.lineage = ""
				if code, got := b.runJSON(b.grantEverythingBed.owners(), "grant", "revoke", b.grant); code != 0 {
					t.Fatalf("revoke: %d %+v", code, got)
				}
				b.lineage = launch.SeatOwnerLineage
			}
			before := b.repo.accepted
			code, got := b.record(row.flags, row.act)
			if code != 2 || got.Outcome != intentRefused || !strings.Contains(got.Summary, row.want) {
				t.Errorf("refusal: %d %+v want %q", code, got, row.want)
			}
			lines, bad, err := humanauthority.ReadDecisions(b.home)
			if b.calls != 0 || b.locks != 0 || b.trace.Len() != 0 || b.repo.accepted != before || len(lines) != 0 || bad != 0 || err != nil {
				t.Fatalf("refusal changed state: calls=%d locks=%d lines=%+v err=%v", b.calls, b.locks, lines, err)
			}
		})
	}
}

func TestADispositionIsARecord(t *testing.T) {
	t.Parallel()
	b := newDecisionAgentBed(t)
	agentProver := b.owners.prove
	base := []string{"decision", "record", "--json", "--text", "wait for proof", "--rejected", "act before proof"}
	keys := []string{"--row", "overdue-goal", "--subject", bedGoal, "--occurrence", "2026-09-01T10:00:00Z"}
	run := func(flags, act []string) (int, intentResult) {
		t.Helper()
		words := append(append(slices.Clone(base), keys...), flags...)
		if act != nil {
			words = append(append(words, "--"), act...)
		}
		code, out, errout := b.run(b.owners, words...)
		var got intentResult
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("record: %d %s %s %v", code, out, errout, err)
		}
		return code, got
	}
	// A session may wait or ask after its authority to act has ended.
	b.lineage = ""
	if code, got := b.runJSON(b.grantEverythingBed.owners(), "grant", "revoke", b.grant); code != 0 {
		t.Fatalf("revoke: %d %+v", code, got)
	}
	b.lineage = launch.SeatOwnerLineage
	before := b.repo.accepted
	if code, got := run([]string{"--dispose", "wait", "--trigger", "proof P ends", "--until", "19:30"}, nil); code != 0 || got.Outcome != intentConfirmed {
		t.Fatalf("wait without a grant: %d %+v", code, got)
	}
	lines, bad, err := humanauthority.ReadDecisions(b.home)
	if err != nil || bad != 0 || len(lines) != 1 {
		t.Fatalf("wait log: %+v %d %v", lines, bad, err)
	}
	now, _ := b.commandNow(b.root())
	zone := b.owners.helm.zone
	local := now.In(zone)
	waitUntil := time.Date(local.Year(), local.Month(), local.Day(), 19, 30, 0, 0, zone)
	d := lines[0]
	if d.Actor != launch.SeatOwnerLineage || d.Grant != "" || d.Lineage != launch.SeatOwnerLineage || d.Class != "" || !d.At.Equal(now) || len(d.Argv) != 0 || d.Disposition == nil || d.Disposition.Row != "overdue-goal" || d.Disposition.Subject != bedGoal || d.Disposition.Occurrence != "2026-09-01T10:00:00Z" || d.Disposition.Kind != humanauthority.DisposeWait || d.Disposition.Trigger != "proof P ends" || !d.Disposition.WaitUntil.Equal(waitUntil) || d.Disposition.Decision != d.ID {
		t.Fatalf("wait record: %+v disposition=%+v, want end %s", d, d.Disposition, waitUntil)
	}
	if code, got := run([]string{"--dispose", "ask", "--class", "values", "--lineage", "explicit-seat"}, nil); code != 0 || got.Outcome != intentConfirmed {
		t.Fatalf("ask without a grant: %d %+v", code, got)
	}
	lines, _, _ = humanauthority.ReadDecisions(b.home)
	if len(lines) != 2 || lines[1].Actor != "explicit-seat" || lines[1].Grant != "" || lines[1].Class != "values" || len(lines[1].Argv) != 0 || lines[1].Disposition == nil || lines[1].Disposition.Kind != humanauthority.DisposeAsk || lines[1].Disposition.Trigger != "" || !lines[1].Disposition.WaitUntil.IsZero() || lines[1].Disposition.Decision != lines[1].ID || b.calls != 0 || b.repo.accepted != before {
		t.Fatalf("ask record: %+v child calls=%d", lines, b.calls)
	}
	for _, row := range []struct {
		name       string
		flags, act []string
		missing    string
	}{
		{"too long", []string{"--dispose", "wait", "--trigger", "proof ends", "--until", "2026-09-03"}, nil, "24 hours"},
		{"past", []string{"--dispose", "wait", "--trigger", "proof ends", "--until", "2026-08-30"}, nil, "--until"},
		{"bad time", []string{"--dispose", "wait", "--trigger", "proof ends", "--until", "never"}, nil, "time like"},
		{"no trigger", []string{"--dispose", "wait", "--until", "19:30"}, nil, "--trigger"},
		{"no end", []string{"--dispose", "wait", "--trigger", "proof ends"}, nil, "--until"},
		{"wait child", []string{"--dispose", "wait", "--trigger", "proof ends", "--until", "19:30"}, []string{"goal", "prioritize", bedGoal, "3"}, "no act"},
		{"ask end", []string{"--dispose", "ask", "--until", "19:30"}, nil, "--until"},
		{"ask trigger", []string{"--dispose", "ask", "--trigger", "proof ends"}, nil, "--trigger"},
		{"act missing", []string{"--dispose", "act"}, nil, "act after --"},
		{"wrong kind", []string{"--dispose", "ignore"}, nil, "act, wait or ask"},
	} {
		code, got := run(row.flags, row.act)
		if code != 2 || got.Outcome != intentRefused || !strings.Contains(got.Summary, row.missing) {
			t.Errorf("%s: %d %+v, want %q", row.name, code, got, row.missing)
		}
	}
	for _, key := range []string{"row", "subject", "occurrence", "dispose", "text", "rejected"} {
		words := append(append(slices.Clone(base), keys...), "--dispose", "ask")
		code, out, _ := b.run(b.owners, withoutOption(words, key)...)
		if code != 2 || !strings.Contains(out, "--"+key) {
			t.Errorf("missing %s: %d %s", key, code, out)
		}
	}
	for _, flag := range []string{"trigger", "until"} {
		code, out, _ := b.run(b.owners, append(slices.Clone(base), "--"+flag, "19:30")...)
		if code != 2 || !strings.Contains(out, "--dispose wait") {
			t.Errorf("orphan %s: %d %s", flag, code, out)
		}
	}
	if got, _, err := humanauthority.ReadDecisions(b.home); err != nil || len(got) != 2 || b.calls != 0 || b.repo.accepted != before {
		t.Fatalf("refusals changed state: %d lines, %d calls, %v", len(got), b.calls, err)
	}
	// The person's own proof still identifies a disposition without an act.
	b.lineage = ""
	b.owners.prove = b.prove
	if code, got := run([]string{"--dispose", "ask"}, nil); code != 0 || got.Outcome != intentConfirmed {
		t.Fatalf("person asks: %d %+v", code, got)
	}
	lines, _, _ = humanauthority.ReadDecisions(b.home)
	if len(lines) != 3 || lines[2].Actor != "human:Wido" || lines[2].By != "Wido" || lines[2].Grant != "" {
		t.Fatalf("person's record: %+v", lines)
	}
	if code, got := b.runJSON(b.grantEverythingBed.owners(), "grant", "add", "--acts", "everything", "--classes", "ordering,recovery", "--lineage", "seat", "--for", "24h"); code != 0 || got.Outcome != intentConfirmed {
		t.Fatalf("restore grant: %d %+v", code, got)
	}
	b.lineage = launch.SeatOwnerLineage
	b.owners.prove = agentProver
	for i, act := range [][]string{{"goal", "prioritize", bedGoal, "3"}, {"goal", "pin", "NOSUCH", "m1e"}} {
		code, got := run([]string{"--dispose", "act", "--class", "ordering", "--undo", "restore priority"}, act)
		if i == 0 && (code != 0 || got.Outcome != intentConfirmed) || i == 1 && (code != 1 || got.Outcome != intentRefused) {
			t.Fatalf("act %v: %d %+v", act, code, got)
		}
		lines, _, _ = humanauthority.ReadDecisions(b.home)
		last := lines[len(lines)-1]
		if len(lines) != 4+i || last.Grant == "" || last.Actor != b.holder.MainId || last.Disposition == nil || last.Disposition.Kind != humanauthority.DisposeAct || last.Disposition.Decision != last.ID || humanauthority.ActHandled(last.Outcome) != (i == 0) {
			t.Fatalf("act record: %+v disposition=%+v", last, last.Disposition)
		}
	}
}

func TestAnAgentsDecisionRunsInARealChild(t *testing.T) {
	t.Parallel()
	b := newDecisionAgentBed(t)
	now, _ := b.commandNow(b.root())
	facts := decisionChildFacts{Root: b.root(), Now: now, Parent: int64(os.Getpid()), Agent: true, Entry: b.rootRecord().PowerOfAttorney[0], Holder: b.holder, Owner: map[string]any{"outcome": "confirmed", "tip": "agent-child-tip", "detail": "agent child detail"}}
	raw, _ := json.Marshal(facts)
	path := filepath.Join(t.TempDir(), "facts.json")
	helmMust(t, os.WriteFile(path, raw, 0600))
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "engine")
	helmMust(t, testexec.WriteFile(wrapper, []byte("#!/bin/sh\nexport GO_WANT_BATCH_E2E_COMMAND=1\nexec "+shellQuote(binary)+" decision-child-probe \"$@\"\n"), 0700))
	fixture := testutil.Fixture(t)
	var answer verbresult.Result
	b.lineage = ""
	b.owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return wrapper, nil }, ownerEnvelope: func(p intentProcess, verb string) (verbresult.Result, error) {
		p.env = append(fixture.Env(p.env), "METASYSTEM_DECISION_FACTS="+path)
		var err error
		answer, err = runIntentOwnerEnvelope(p, verb)
		return answer, err
	}}
	code, got := b.record([]string{"--class", "ordering", "--lineage", "lin-x"}, []string{"goal", "prioritize", bedGoal, "3"})
	if code != 0 || got.Outcome != intentConfirmed {
		t.Fatalf("real child: %d %+v", code, got)
	}
	var data struct {
		Seen struct {
			Argv                              []string
			Cwd, Lineage, Grant, Class, Proof string
			Parent                            int64
			Admitted                          bool
		}
	}
	if err := json.Unmarshal(answer.Data, &data); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(data.Seen.Argv, []string{"goal", "prioritize", "--json", bedGoal, "3"}) || data.Seen.Cwd != b.root() || data.Seen.Lineage != "lin-x" || data.Seen.Parent != int64(os.Getpid()) || !data.Seen.Admitted || data.Seen.Grant != b.grant || data.Seen.Class != "ordering" || data.Seen.Proof != humanauthority.OutcomeAgent {
		t.Fatalf("child context: %+v", data.Seen)
	}
	lines, bad, err := humanauthority.ReadDecisions(b.home)
	if err != nil || bad != 0 || len(lines) != 1 || lines[0].Grant != b.grant || !bytes.Contains(lines[0].Effect, []byte(`"tip":"agent-child-tip"`)) || !bytes.Contains(lines[0].Effect, []byte(`"detail":"agent child detail"`)) {
		t.Fatalf("line: %+v %d %v", lines, bad, err)
	}
	lock := &grantLock{}
	acquired, err := lock.exclusive(b.root(), 0, func(time.Duration) { t.Fatal("child kept the lock after exit") })
	if err != nil || !acquired {
		t.Fatalf("child lock after exit: %v %v", acquired, err)
	}
	for _, file := range lock.file {
		helmMust(t, file.Close())
	}
	t.Logf("agent decision: grant=%s class=%s lineage=%s parent=%d effect=%s; child lock released", data.Seen.Grant, data.Seen.Class, data.Seen.Lineage, data.Seen.Parent, lines[0].Effect)
}
