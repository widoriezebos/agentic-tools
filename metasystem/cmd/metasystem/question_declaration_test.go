package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestQ2AgentCommandOptionsNeedDeclarations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, consequence string
		agent, refused    bool
	}{
		{"steward accepts", "run metasystem goal accept-risk fix-docs --finding S-1 --review critic-r2", true, true},
		{"continuation cancels", "run metasystem work stop run:stuck", true, true},
		{"quoted command", "run `bin/metasystem\tgoal accept-risk fix-docs`", true, true},
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
			b, owners := newQ1CommandBed(t)
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
			} else if code != 0 || len(qs) != 1 {
				t.Fatalf("ordinary ask changed: %d %+v", code, result)
			}
		})
	}
}

func TestQ2UnnamedAgentCommandOptionNeedsDeclaration(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	owners.agent.caller = func(*intentInvocation, string) string { return lease.ClassDelegate }
	code, result := b.runJSON(owners, "question", "ask", bedGoal, "--question", "Cancel?", "--option", "Cancel: run metasystem work stop run:stuck")
	if code != 2 || result.Outcome != intentRefused || result.Next == nil {
		t.Fatalf("unnamed agent bypassed declaration: %d %+v", code, result)
	}
}

func TestQ2DeclarationBelongsToItsOption(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
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
	b, owners := newQ1CommandBed(t)
	code, result := b.runJSON(owners, "question", "ask", bedGoal, "--question", "Accept?", "--option", "Accept: keep the risk", "--option-act", "Accept: metasystem goal accept-risk "+bedGoal+" --review critic-r2 --finding S-1", "--wants", "approval token")
	qs, _ := channel.WalkQuestions(b.root())
	if code == 0 || result.Outcome != intentRefused || len(qs) != 0 || !strings.Contains(result.Summary+strings.Join(result.Details, " "), "--wants") {
		t.Fatalf("mixed authority ask: %d %+v", code, result)
	}
}
