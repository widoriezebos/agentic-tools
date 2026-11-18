package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/checkenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type proofEnvironmentProber struct{}

func (proofEnvironmentProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Now()}, identity.Alive, nil
}
func TestProofEnvironmentReplacesParent(t *testing.T) {
	t.Setenv("UNDECLARED_PROOF_INPUT", "synthetic-parent")
	t.Setenv("PATH", t.TempDir())
	for _, declared := range []bool{false, true} {
		var entries []string
		if declared {
			entries = []string{"DECLARED_PROOF_INPUT=synthetic-declaration"}
		}
		input, err := checkenv.Admit(nil, entries, false)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		script := `test -z "${UNDECLARED_PROOF_INPUT+x}" && test -n "$OWNED_OVERLAY"`
		if declared {
			script += ` && test "$DECLARED_PROOF_INPUT" = synthetic-declaration`
		} else {
			script += ` && test -z "${DECLARED_PROOF_INPUT+x}"`
		}
		brief := PlainBrief{Argv: []string{"/bin/sh", "-c", script}, Dir: dir, Input: input, Overlays: []string{"OWNED_OVERLAY=present"}}
		body, _ := json.Marshal(brief)
		path := filepath.Join(dir, "brief.json")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		record := Record{AdapterData: map[string]json.RawMessage{}}
		setString(record.AdapterData, "brief", path)
		command, err := (PlainExec{}).Command(record, dir)
		if err != nil || !command.ReplaceEnvironment {
			t.Fatal("proof did not request replacement")
		}
		child, _, err := (OSProcesses{Prober: proofEnvironmentProber{}}).StartChild(command)
		if err != nil {
			t.Fatal(err)
		}
		if code, err := child.Wait(); err != nil || code != 0 {
			t.Fatal("proof acquired parent input or lost declared input/overlay")
		}
		after, _ := os.ReadFile(path)
		if !slices.Equal(body, after) || len(input.Environment()) != len(entries) {
			t.Fatal("runtime input mutated the record")
		}
	}
	for _, environment := range [][]string{nil, {}} {
		_, _, err := (OSProcesses{Prober: proofEnvironmentProber{}}).StartChild(Command{Program: "sh", Directory: t.TempDir(), Environment: environment, ReplaceEnvironment: true, LogPath: filepath.Join(t.TempDir(), "log")})
		if err == nil {
			t.Fatal("bare executable used parent PATH")
		}
	}
}
func TestProofEnvironmentDiagnosticsDoNotChangeIdentities(t *testing.T) {
	f := newUnitFixture(t, "", "branch", "round")
	plan, err := ReadUnitPlan(f.plan)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := checkenv.Admit([]string{"FIRST=one", "PATH=/usr/bin:/bin"}, nil, true)
	b, _ := checkenv.Admit([]string{"SECOND=two", "PATH=/usr/bin:/bin"}, nil, true)
	plan.Proof[0].Input = a
	first, err := namedUnitDigest(plan, plan.Worktree, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	roundA, _ := json.Marshal(proofIdentityCommands(plan))
	plan.Proof[0].Input = b
	second, err := namedUnitDigest(plan, plan.Worktree, DefaultSettings())
	roundB, _ := json.Marshal(proofIdentityCommands(plan))
	if err != nil || first != second || !slices.Equal(roundA, roundB) || CheckEnvironmentDigest(a.Environment(), "base") != CheckEnvironmentDigest(b.Environment(), "base") {
		t.Fatal("diagnostics changed proof identity")
	}
}
