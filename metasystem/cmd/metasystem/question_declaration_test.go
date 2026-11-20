package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestQ2AgentCommandOptionsNeedDeclarations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, consequence string
		agent, refused    bool
	}{
		{"steward accepts", "run metasystem goal accept-risk " + bedGoal + " --finding S-1 --review critic-r2", true, true},
		{"continuation cancels", "run metasystem work stop run:stuck", true, true},
		{"quoted command", "run `bin/metasystem\tgoal accept-risk " + bedGoal + " --finding S-1 --review critic-r2`", true, true},
		{"human accepts", "run metasystem goal accept-risk fix-docs", false, false},
		{"human cancels", "run metasystem work stop run:stuck", false, false},
		{"no command", "accept the risk and carry on", true, false},
		{"other verb", "run metasystem goal pause fix-docs", true, false},
		{"stop goal", "Undo: metasystem work stop fix-docs", true, false},
		{"stop job", "run metasystem work stop j2:job", true, false},
		{"different executable", "other-metasystem goal accept-risk fix-docs", true, false},
		{"different verb", "metasystem goal accept-risk-later fix-docs", true, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b, owners := newQ2CommandBed(t)
			owners.agent.caller = func(*intentInvocation, string) string { return lease.ClassHuman }
			if row.agent {
				b.lineage = "steward-seat"
			}
			code, result := b.runJSON(owners, "question", "ask", bedGoal, "--question", "Decide?", "--option", "Accept: "+row.consequence)
			qs, bad := channel.WalkQuestions(b.root())
			if row.refused {
				if code != 2 || result.Outcome != intentRefused || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "--option-act Accept: metasystem ") || len(qs) != 0 || len(bad) != 0 {
					t.Fatalf("undeclared command: %d %+v, questions=%d unreadable=%d", code, result, len(qs), len(bad))
				}
				code, retry := b.runJSON(owners, result.Next.Argv[1:]...)
				qs, bad = channel.WalkQuestions(b.root())
				if code != 0 || len(qs) != 1 || len(bad) != 0 || qs[0].Options[0].NamedAct == nil {
					t.Fatalf("suggested declaration failed: %d %+v questions=%d unreadable=%d", code, retry, len(qs), len(bad))
				}
			} else if code != 0 || len(qs) != 1 {
				t.Fatalf("ordinary ask changed: %d %+v", code, result)
			}
		})
	}
}

func TestQ2UnnamedAgentCommandOptionNeedsDeclaration(t *testing.T) {
	t.Parallel()
	b, owners := newQ2CommandBed(t)
	owners.agent.caller = func(*intentInvocation, string) string { return lease.ClassDelegate }
	code, result := b.runJSON(owners, "question", "ask", bedGoal, "--question", "Cancel?", "--option", "Cancel: run metasystem work stop run:stuck")
	if code != 2 || result.Outcome != intentRefused || result.Next == nil {
		t.Fatalf("unnamed agent bypassed declaration: %d %+v", code, result)
	}
}

func TestQ2DeclarationBelongsToItsOption(t *testing.T) {
	t.Parallel()
	b, owners := newQ2CommandBed(t)
	b.lineage = "continuation"
	command := "metasystem goal accept-risk " + bedGoal + " --review critic-r2 --finding S-1"
	args := []string{"question", "ask", bedGoal, "--question", "Accept?", "--option", "Accept: run " + command, "--option", "Wait: leave it open"}
	code, result := b.runJSON(owners, append(args, "--option-act", "Wait: "+command)...)
	if code != 2 || result.Outcome != intentRefused {
		t.Fatalf("another option's declaration hid the omission: %d %+v", code, result)
	}
	code, result = b.runJSON(owners, append(args, "--option-act", "Accept: "+command)...)
	if code != 0 {
		t.Fatalf("declared ask: %d %+v", code, result)
	}
}

func TestQ2WantsCannotAccompanyOptionAct(t *testing.T) {
	t.Parallel()
	b, owners := newQ2CommandBed(t)
	code, result := b.runJSON(owners, "question", "ask", bedGoal, "--question", "Accept?", "--option", "Accept: keep the risk", "--option-act", "Accept: metasystem goal accept-risk "+bedGoal+" --review critic-r2 --finding S-1", "--wants", "approval token")
	qs, _ := channel.WalkQuestions(b.root())
	if code == 0 || result.Outcome != intentRefused || len(qs) != 0 || !strings.Contains(result.Summary+strings.Join(result.Details, " "), "--wants") {
		t.Fatalf("mixed authority ask: %d %+v", code, result)
	}
}

func TestQ2UndeclarableCommandOptionsRemainAskable(t *testing.T) {
	t.Parallel()
	command := "metasystem goal accept-risk " + bedGoal + " --review critic-r2 --finding S-1"
	for _, row := range []struct {
		name          string
		target, extra []string
		command       string
	}{
		{name: "machine", target: []string{"--about", "machine"}},
		{name: "lane", target: []string{"--about", "lane"}},
		{name: "another goal", target: []string{bedGoal}, command: "metasystem goal accept-risk another --review critic-r2 --finding S-1"},
		{name: "another goal run", target: []string{bedGoal}, command: "metasystem work stop run:foreign"},
		{name: "missing run", target: []string{bedGoal}, command: "metasystem work stop run:missing"},
		{name: "missing finding", target: []string{bedGoal}, command: "metasystem goal accept-risk " + bedGoal + " --review critic-r2 --finding missing"},
		{name: "incomplete command", target: []string{bedGoal}, command: "metasystem goal accept-risk " + bedGoal},
		{name: "duplicate command", target: []string{bedGoal}, extra: []string{"--option", "Accept and rebuild: " + command}},
		{name: "duplicate command with another reason", target: []string{bedGoal}, extra: []string{"--option", "Accept and rebuild: " + command + " --reason 'bounded exposure'"}},
		{name: "duplicate run", target: []string{bedGoal}, command: "metasystem work stop run:stuck", extra: []string{"--option", "Cancel and rebuild: metasystem work stop run:stuck"}},
		{name: "carry", target: []string{bedGoal}, extra: []string{"--kind", "carry", "--wants", "carry workspace=" + strings.Repeat("a", 40) + " goal=" + bedGoal + " past=Wido"}},
		{name: "stop", target: []string{bedGoal}, extra: []string{"--kind", "stop", "--budget", "1d/10/720m/1/3"}},
		{name: "empty wants", target: []string{bedGoal}, extra: []string{"--wants", ""}},
		{name: "wants", target: []string{bedGoal}, extra: []string{"--wants", "approval token"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b, owners := newQ2CommandBed(t)
			b.lineage = "steward-seat"
			cmd := command
			if row.command != "" {
				cmd = row.command
			}
			args := append([]string{"question", "ask"}, row.target...)
			args = append(args, "--question", "Decide?", "--option", "Accept: "+cmd)
			args = append(args, row.extra...)
			code, result := b.runJSON(owners, args...)
			qs, bad := channel.WalkQuestions(b.root())
			if code != 0 || len(qs) != 1 || len(bad) != 0 {
				t.Fatalf("undeclarable ask: %d %+v questions=%d unreadable=%d", code, result, len(qs), len(bad))
			}
			for _, opt := range qs[0].Options {
				if opt.NamedAct != nil {
					t.Fatal("ordinary ask gained a declaration")
				}
			}
			code, declared := b.runJSON(owners, append(args, "--option-act", "Accept: "+cmd)...)
			qs, bad = channel.WalkQuestions(b.root())
			if code == 0 || declared.Outcome != intentRefused || len(qs) != 1 || len(bad) != 0 {
				t.Fatalf("undeclarable command accepted: %d %+v questions=%d unreadable=%d", code, declared, len(qs), len(bad))
			}
		})
	}
}

func newQ2CommandBed(t *testing.T) (*intentBed, intentOwners) {
	t.Helper()
	b, owners := newQ1CommandBed(t)
	root := t.TempDir()
	for id, goalID := range map[string]string{"stuck": bedGoal, "foreign": "another"} {
		path := filepath.Join(root, id, "run.json")
		data, err := json.Marshal(launch.UnitRunRecord{ID: id, Goal: goalID})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Root: root, Manager: &launch.Manager{}}
	}
	return b, owners
}
