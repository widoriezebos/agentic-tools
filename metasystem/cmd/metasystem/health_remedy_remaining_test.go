package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func TestSystemCheckRemainingRemediesFromOneOwner(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	source, err := parser.ParseFile(token.NewFileSet(), "intent_process.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range source.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "publicHealthRemedy" {
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if _, ok := node.(*ast.SwitchStmt); ok {
					t.Error("the command renderer keeps a second per-role remedy owner")
				}
				return true
			})
		}
	}
	output := filepath.Join(t.TempDir(), "verdicts.json")
	produce := exec.Command(*remedyClearProducer, "-test.run=^TestHealthRemedyTableOwnsEveryRole$", "-test.timeout=30m", "-remaining-remedy-output="+output)
	produce.Dir = "../../internal/steward"
	var producerErrors bytes.Buffer
	produce.Stderr = &producerErrors
	input, err := produce.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	outputPipe, err := produce.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := produce.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		input.Close()
		if err := produce.Wait(); err != nil {
			t.Errorf("real health bed: %v\n%s", err, producerErrors.String())
		}
	}()
	if line, err := bufio.NewReader(outputPipe).ReadString('\n'); err != nil || line != "health beds ready\n" {
		t.Fatalf("health beds were not ready: %q %v", line, err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Root    string
		Verdict steward.HealthVerdict
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, item := range cases {
		verdict := item.Verdict
		for _, reader := range []struct{ class, audience string }{{lease.ClassHuman, "human"}, {lease.ClassMain, "agent"}} {
			b := newProcessBed(t)
			b.facts.root = item.Root
			b.writeEngine("engine fixture")
			if err := os.MkdirAll(filepath.Join(b.root(), "scripts", "agents"), 0755); err != nil {
				t.Fatal(err)
			}
			owners := b.owners()
			if verdict.Stopped {
				if code, result := b.runJSON(owners, "system", "stop"); code != 0 {
					t.Fatalf("close the command bed's real fence: exit %d %+v", code, result)
				}
			} else if err := os.Remove(stopfence.TransitionPath(b.root())); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			b.class = reader.class
			owners.processes.healthNow = func(string) (time.Time, error) { return verdict.ObservedAt, nil }
			owners.processes.health = func(root, installation string, now time.Time) steward.HealthVerdict {
				if root != item.Root || installation != item.Root || !now.Equal(verdict.ObservedAt) {
					t.Fatalf("system check did not read this bed's current scope and clock: %s %s %s", root, installation, now)
				}
				return verdict
			}
			code, result := b.runJSON(owners, "system", "check")
			data, err := json.Marshal(result.Data)
			if err != nil {
				t.Fatal(err)
			}
			var preview struct {
				Line           string
				PublicRemedies []struct {
					Role        steward.HealthRole
					Public      []string
					Instruction string
				}
			}
			if err := json.Unmarshal(data, &preview); err != nil {
				t.Fatal(err)
			}
			var unhealthy []steward.RoleVerdict
			for _, role := range verdict.Roles {
				if role.Status != steward.HealthAlive {
					unhealthy = append(unhealthy, role)
				}
			}
			if code != verdict.ExitCode() || len(preview.PublicRemedies) != len(unhealthy) {
				t.Fatalf("system check lost the real verdict: exit %d, %+v; result: %+v", code, preview, result)
			}
			for i, row := range preview.PublicRemedies {
				role := unhealthy[i]
				act, plain := role.PublicRemedy(reader.audience, healthRemedyAudience, verdict.Stopped)
				if row.Role != role.Role || !slices.Equal(row.Public, act) || row.Instruction != plain {
					t.Errorf("%s stopped=%t: system check contradicts %s's cause: %+v want %v %q", reader.audience, verdict.Stopped, role.Role, row, act, plain)
				}
				if verdict.Stopped && strings.Join(row.Public, " ") == "metasystem session start" {
					t.Errorf("the closed fence offered an act it refuses: %+v", row)
				}
				if len(row.Public) > 0 {
					command, _, ok := resolveIntentArgv(row.Public[1:])
					if !ok || command.audience != "both" && command.audience != reader.audience || slices.Contains([]string{"list", "show", "check", "status"}, command.action) {
						t.Errorf("%s was given no repairing public act: %v", reader.audience, row.Public)
					}
				} else if row.Instruction == "" {
					t.Errorf("%s has no remedy", role.Role)
				}
				if !verdict.Stopped && role.Status == steward.HealthDead && slices.Contains([]steward.HealthRole{steward.RoleStewardRunner, steward.RoleSupervisionOwner, steward.RoleRepoWatcher, steward.RoleCensusFreshness, steward.RoleNarratorFreshness, steward.RoleSessionMain, steward.RoleHookFreshness}, role.Role) && !strings.Contains(preview.Line, string(role.Role)+"=dead ("+role.Reason+"; remedy: metasystem session start)") {
					t.Errorf("system check hook preview did not give %s the agent act: %s", role.Role, preview.Line)
				}
				if role.RemedyFacts[0].Cause == steward.CauseUnavailable && slices.Contains([]steward.HealthRole{steward.RoleStewardRunner, steward.RoleSupervisionOwner, steward.RoleRepoWatcher, steward.RoleCensusFreshness, steward.RoleNarratorFreshness, steward.RoleSessionMain, steward.RoleHookFreshness}, role.Role) {
					want := "metasystem system start"
					if reader.audience == "agent" {
						want = "metasystem session start"
					}
					if strings.Join(row.Public, " ") != want {
						t.Errorf("%s needs its own process act %q, got %+v", reader.audience, want, row)
					}
				}
			}
			t.Logf("%s stopped=%t: system check exit %d; %d cause-owned remedies", reader.audience, verdict.Stopped, code, len(preview.PublicRemedies))
		}
	}
}

func TestSystemCheckLedgerAttentionNotYetProduced(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	for _, evidence := range []struct {
		name, contents string
		cause          steward.RemedyCause
		instruction    string
	}{
		{"missing", "", steward.CauseUnavailable, "nothing to do: the armed steward examines the move on its next tick"},
		{"no canonical tip", `{"schema":2}`, steward.CauseUnavailable, "nothing to do: the armed steward examines the move on its next tick"},
		{"malformed", "{broken", steward.CauseUnreadable, "a person repairs the unreadable evidence the reason above names, then runs metasystem system check"},
	} {
		b := newProcessBed(t)
		if evidence.contents != "" {
			path := filepath.Join(b.root(), "artifacts", "agents", "steward", "ledger-attention.json")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(evidence.contents), 0600); err != nil {
				t.Fatal(err)
			}
		}
		owners := b.owners()
		for _, class := range []string{lease.ClassHuman, lease.ClassMain} {
			b.class = class
			code, result := b.runJSON(owners, "system", "check")
			data, err := json.Marshal(result.Data)
			if err != nil {
				t.Fatal(err)
			}
			var preview struct {
				Verdict        steward.HealthVerdict
				PublicRemedies []struct {
					Role        steward.HealthRole
					Public      []string
					Instruction string
				}
			}
			if err := json.Unmarshal(data, &preview); err != nil {
				t.Fatal(err)
			}
			index := slices.IndexFunc(preview.Verdict.Roles, func(role steward.RoleVerdict) bool { return role.Role == steward.RoleLedgerAttention })
			if index < 0 {
				t.Fatal("system check omitted ledger attention")
			}
			role := preview.Verdict.Roles[index]
			if code != preview.Verdict.ExitCode() || role.Status != steward.HealthUnknown || len(role.RemedyFacts) != 1 || role.RemedyFacts[0].Cause != evidence.cause || role.Remedy != evidence.instruction {
				t.Errorf("%s for %s: exit %d, ledger attention %+v", evidence.name, class, code, role)
			}
			index = slices.IndexFunc(preview.PublicRemedies, func(row struct {
				Role        steward.HealthRole
				Public      []string
				Instruction string
			}) bool {
				return row.Role == steward.RoleLedgerAttention
			})
			if index < 0 || len(preview.PublicRemedies[index].Public) != 0 || preview.PublicRemedies[index].Instruction != evidence.instruction {
				t.Errorf("%s for %s: wrong public ledger remedy: %+v", evidence.name, class, preview.PublicRemedies)
			}
		}
	}
}

func TestSystemCheckMissingNarratorInstallationNeedsStart(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	for _, malformed := range []bool{false, true} {
		b := newProcessBed(t)
		if malformed {
			path := steward.RepoIdentityPath(b.root())
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		owners := b.owners()
		for _, class := range []string{lease.ClassHuman, lease.ClassMain} {
			b.class = class
			code, result := b.runJSON(owners, "system", "check")
			data, err := json.Marshal(result.Data)
			if err != nil {
				t.Fatal(err)
			}
			var preview struct {
				Verdict        steward.HealthVerdict
				PublicRemedies []struct {
					Role        steward.HealthRole
					Public      []string
					Instruction string
				}
			}
			if err := json.Unmarshal(data, &preview); err != nil {
				t.Fatal(err)
			}
			index := slices.IndexFunc(preview.Verdict.Roles, func(role steward.RoleVerdict) bool { return role.Role == steward.RoleNarratorFreshness })
			if index < 0 {
				t.Fatal("system check omitted narrator freshness")
			}
			cause, reason := steward.CauseUnavailable, "no steward installation generation is recorded"
			want := []string{"metasystem", "system", "start"}
			if class == lease.ClassMain {
				want[1] = "session"
			}
			instruction := ""
			if malformed {
				cause, reason = steward.CauseUnreadable, "the steward installation generation is unreadable"
				want = nil
				instruction = "a person repairs the unreadable evidence the reason above names, then runs metasystem system check"
			}
			role := preview.Verdict.Roles[index]
			if code != preview.Verdict.ExitCode() || role.Status != steward.HealthUnknown || role.Reason != reason || len(role.RemedyFacts) != 1 || role.RemedyFacts[0].Cause != cause {
				t.Errorf("malformed=%t for %s: exit %d, narrator freshness %+v", malformed, class, code, role)
			}
			index = slices.IndexFunc(preview.PublicRemedies, func(row struct {
				Role        steward.HealthRole
				Public      []string
				Instruction string
			}) bool {
				return row.Role == steward.RoleNarratorFreshness
			})
			if index < 0 || !slices.Equal(preview.PublicRemedies[index].Public, want) || preview.PublicRemedies[index].Instruction != instruction {
				t.Errorf("malformed=%t for %s: wrong public narrator remedy: %+v", malformed, class, preview.PublicRemedies)
			}
		}
	}
}
