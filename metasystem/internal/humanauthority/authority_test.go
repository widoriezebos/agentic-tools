package humanauthority

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type treeReader struct {
	snapshots map[int64][]Snapshot
	reads     map[int64]int
	session   int64
}

func (reader *treeReader) Read(pid int64) (Snapshot, error) {
	values := reader.snapshots[pid]
	if len(values) == 0 {
		return Snapshot{}, os.ErrNotExist
	}
	index := reader.reads[pid]
	if index >= len(values) {
		index = len(values) - 1
	}
	reader.reads[pid]++
	return values[index], nil
}

func (reader *treeReader) SessionLeader(int64) (int64, error) { return reader.session, nil }

func authoritySnapshot(pid, parent int64, argv []string, terminal string) Snapshot {
	executable := "/fixture/bin/unreadable"
	if len(argv) > 0 {
		executable = "/fixture/bin/" + argv[0]
	}
	return Snapshot{
		Exact:           identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), Argv: argv, ArgvKnown: true},
		Executable:      executable,
		ExecutableKnown: true,
		ParentPID:       parent, ParentKnown: true, TerminalID: terminal, TerminalKnown: true,
	}
}

func authorityRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match codex-agent'\n"
	if err := os.WriteFile(filepath.Join(directory, "codex.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func enrolledReader() *treeReader {
	return &treeReader{
		reads: map[int64]int{}, session: 10,
		snapshots: map[int64][]Snapshot{
			10: {authoritySnapshot(10, 1, []string{"terminal-session"}, "tty-1")},
			20: {authoritySnapshot(20, 10, []string{"interactive-shell"}, "tty-1")},
			30: {authoritySnapshot(30, 20, []string{"command-wrapper", "private-argument"}, "tty-1")},
		},
	}
}

func enrollTestTerminal(t *testing.T, root string, reader *treeReader) {
	t.Helper()
	if _, err := Enroll(root, 20, reader, time.Unix(1000, 0)); err != nil {
		t.Fatalf("enroll terminal: %v", err)
	}
	reader.reads = map[int64]int{}
}

func TestProofRequiresExactAgentFreeEnrolledAncestry(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	proof, err := Prove(root, 30, reader, time.Unix(1100, 0))
	if err != nil || !proof.Valid() || proof.Outcome != OutcomeProven || len(proof.Nodes) != 2 {
		t.Fatalf("direct wrapper ancestry was not proven: proof=%+v err=%v", proof, err)
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-argument") || strings.Contains(string(encoded), "command-wrapper") {
		t.Fatalf("proof retained raw argv: %s", encoded)
	}
	if !proof.ValidFor(root) || proof.ValidFor(t.TempDir()) {
		t.Fatal("an observed proof was not bound to its checked root")
	}
	var parsed Proof
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Valid() {
		t.Fatal("parsed proof metadata became a reusable authority token")
	}

	reader.reads = map[int64]int{}
	reader.snapshots[40] = []Snapshot{authoritySnapshot(40, 20, []string{"codex-agent", "run"}, "tty-1")}
	agentProof, err := Prove(root, 40, reader, time.Unix(1200, 0))
	if err == nil || agentProof.Outcome != OutcomeAgent || agentProof.Valid() {
		t.Fatalf("an agent process passed human authority: proof=%+v err=%v", agentProof, err)
	}

	reader.reads = map[int64]int{}
	reader.snapshots[50] = []Snapshot{authoritySnapshot(50, 40, []string{"shell-wrapper"}, "tty-1")}
	agentShellProof, err := Prove(root, 50, reader, time.Unix(1300, 0))
	if err == nil || agentShellProof.Outcome != OutcomeAgent || agentShellProof.Valid() {
		t.Fatalf("a shell below an agent passed human authority: proof=%+v err=%v", agentShellProof, err)
	}
}

func TestProofFailsClosedOnTerminalAndAncestryUncertainty(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*treeReader)
		want    string
	}{
		{
			name: "agent-created terminal",
			prepare: func(reader *treeReader) {
				reader.snapshots[30] = []Snapshot{authoritySnapshot(30, 20, []string{"wrapper"}, "tty-agent")}
			},
			want: OutcomeTerminalMissing,
		},
		{
			name: "argv unreadable",
			prepare: func(reader *treeReader) {
				value := authoritySnapshot(30, 20, nil, "tty-1")
				value.Exact.ArgvKnown = false
				reader.snapshots[30] = []Snapshot{value}
			},
			want: OutcomeArgvUnreadable,
		},
		{
			name: "parent changed",
			prepare: func(reader *treeReader) {
				reader.snapshots[30] = []Snapshot{
					authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1"),
					authoritySnapshot(30, 21, []string{"wrapper"}, "tty-1"),
				}
			},
			want: OutcomeChanged,
		},
		{
			name: "arguments changed",
			prepare: func(reader *treeReader) {
				reader.snapshots[30] = []Snapshot{
					authoritySnapshot(30, 20, []string{"wrapper", "first"}, "tty-1"),
					authoritySnapshot(30, 20, []string{"wrapper", "second"}, "tty-1"),
				}
			},
			want: OutcomeChanged,
		},
		{
			name: "executable changed",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				second := first
				second.Executable = "/fixture/bin/replaced-wrapper"
				reader.snapshots[30] = []Snapshot{first, second}
			},
			want: OutcomeChanged,
		},
		{
			name: "process reused",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				second := first
				second.Exact.StartedAt = first.Exact.StartedAt.Add(time.Second)
				reader.snapshots[30] = []Snapshot{first, second}
			},
			want: OutcomeReused,
		},
		{
			name: "parent reused between nodes",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(20, 10, []string{"interactive-shell"}, "tty-1")
				second := first
				second.Exact.StartedAt = first.Exact.StartedAt.Add(time.Second)
				reader.snapshots[20] = []Snapshot{first, second}
			},
			want: OutcomeReused,
		},
		{
			name: "ancestry cycle",
			prepare: func(reader *treeReader) {
				reader.snapshots[30] = []Snapshot{authoritySnapshot(30, 31, []string{"wrapper"}, "tty-1")}
				reader.snapshots[31] = []Snapshot{authoritySnapshot(31, 30, []string{"other-wrapper"}, "tty-1")}
			},
			want: OutcomeCycle,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := authorityRoot(t)
			reader := enrolledReader()
			enrollTestTerminal(t, root, reader)
			test.prepare(reader)
			proof, err := Prove(root, 30, reader, time.Unix(1400, 0))
			if err == nil || proof.Outcome != test.want || proof.Valid() {
				t.Fatalf("uncertain ancestry did not fail closed: proof=%+v err=%v want=%s", proof, err, test.want)
			}
		})
	}
}
