package humanauthority

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type treeReader struct {
	snapshots map[int64][]Snapshot
	reads     map[int64]int
	failAt    map[int64]map[int]error
	session   int64
}

func (reader *treeReader) Read(pid int64) (Snapshot, error) {
	values := reader.snapshots[pid]
	if len(values) == 0 {
		return Snapshot{}, os.ErrNotExist
	}
	index := reader.reads[pid]
	if failures := reader.failAt[pid]; failures != nil {
		if err := failures[index]; err != nil {
			reader.reads[pid]++
			return Snapshot{}, err
		}
	}
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
		OwnerUID:        501,
		OwnerKnown:      true,
		ParentPID:       parent, ParentKnown: true, TerminalID: terminal, TerminalKnown: true,
	}
}

func systemLoginSnapshot(pid, parent int64, terminal string) Snapshot {
	snapshot := authoritySnapshot(pid, parent, nil, terminal)
	snapshot.Exact.ArgvKnown = false
	snapshot.Executable = "/usr/bin/login"
	snapshot.OwnerUID = 0
	return snapshot
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

func TestTerminalAppLoginAndTmuxSessionShapesEnroll(t *testing.T) {
	t.Run("Terminal app login is the session leader", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("the admitted system login program list is intentionally empty outside Darwin")
		}
		root := authorityRoot(t)
		terminal := authoritySnapshot(1280, 1, []string{"/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal"}, "")
		terminal.Executable = "/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal"
		login := systemLoginSnapshot(61287, 1280, "tty-1")
		shell := authoritySnapshot(61288, 61287, []string{"-zsh"}, "tty-1")
		shell.Executable = "/bin/zsh"
		command := authoritySnapshot(70, 61288, []string{"metasystem", "goal", "set-obligation"}, "tty-1")
		reader := &treeReader{
			reads: map[int64]int{}, session: 61287,
			snapshots: map[int64][]Snapshot{
				1280:  {terminal},
				61287: {login},
				61288: {shell},
				70:    {command},
			},
		}
		enrollment, err := Enroll(root, 61288, reader, time.Unix(1700, 0))
		if err != nil {
			t.Fatalf("Terminal.app shell enrollment failed: %v", err)
		}
		if enrollment.TerminalRef.PID != 61288 || enrollment.SessionLeader.PID != 61287 {
			t.Fatalf("Terminal.app enrollment recorded the wrong roots: %+v", enrollment)
		}
		reader.reads = map[int64]int{}
		proof, err := Prove(root, 70, reader, time.Unix(1800, 0))
		if err != nil || proof.Outcome != OutcomeProven || !proof.Valid() {
			t.Fatalf("command under the Terminal.app shell was not proven: proof=%+v err=%v", proof, err)
		}
	})

	t.Run("tmux shell is its own session leader", func(t *testing.T) {
		root := authorityRoot(t)
		shell := authoritySnapshot(8458, 8457, []string{"-zsh"}, "tty-2")
		shell.Executable = "/bin/zsh"
		server := authoritySnapshot(8457, 1, []string{"tmux", "new-session"}, "")
		reader := &treeReader{
			reads: map[int64]int{}, session: 8458,
			snapshots: map[int64][]Snapshot{
				8458: {shell},
				8457: {server},
			},
		}
		enrollment, err := Enroll(root, 8458, reader, time.Unix(1900, 0))
		if err != nil {
			t.Fatalf("tmux shell enrollment failed: %v", err)
		}
		if enrollment.TerminalRef.PID != 8458 || enrollment.SessionLeader.PID != 8458 {
			t.Fatalf("tmux enrollment recorded the wrong roots: %+v", enrollment)
		}
	})
}

func TestSystemLoginAdmissionRequiresStableWithheldArgumentsAndRootOwner(t *testing.T) {
	tests := []struct {
		name           string
		first          Snapshot
		second         Snapshot
		darwinOnly     bool
		wantOutcome    string
		wantPath       bool
		wantOwner      bool
		wantWorkaround bool
		wantReason     string
	}{
		{
			name: "root sudo with withheld arguments",
			first: func() Snapshot {
				value := systemLoginSnapshot(90, 1, "tty-1")
				value.Executable = "/usr/bin/sudo"
				return value
			}(),
			wantOutcome: OutcomeArgvUnreadable, wantPath: true, wantOwner: true, wantWorkaround: true,
			wantReason: "the operating system withholds this process's arguments and it is not a known system login program",
		},
		{
			name: "user-owned login with withheld arguments",
			first: func() Snapshot {
				value := systemLoginSnapshot(91, 1, "tty-1")
				value.OwnerUID = 501
				return value
			}(),
			wantOutcome: OutcomeArgvUnreadable, wantPath: true, wantOwner: true, wantWorkaround: true,
		},
		{
			name:       "login arguments become readable",
			darwinOnly: true,
			first:      systemLoginSnapshot(92, 1, "tty-1"),
			second: func() Snapshot {
				value := systemLoginSnapshot(92, 1, "tty-1")
				value.Exact.Argv = []string{"login", "-pf", "wido"}
				value.Exact.ArgvKnown = true
				return value
			}(),
			wantOutcome: OutcomeChanged, wantPath: true, wantOwner: true,
		},
		{
			name: "login arguments become withheld",
			first: func() Snapshot {
				value := systemLoginSnapshot(96, 1, "tty-1")
				value.Exact.Argv = []string{"login", "-pf", "wido"}
				value.Exact.ArgvKnown = true
				return value
			}(),
			second:         systemLoginSnapshot(96, 1, "tty-1"),
			wantOutcome:    OutcomeArgvUnreadable,
			wantPath:       true,
			wantOwner:      true,
			wantWorkaround: true,
			wantReason:     "the process's arguments changed from readable to withheld between observations",
		},
		{
			name: "login executable unreadable",
			first: func() Snapshot {
				value := systemLoginSnapshot(93, 1, "tty-1")
				value.ExecutableKnown = false
				return value
			}(),
			wantOutcome: OutcomeUnreadable, wantOwner: true, wantWorkaround: true,
		},
		{
			name: "login owner unknown",
			first: func() Snapshot {
				value := systemLoginSnapshot(94, 1, "tty-1")
				value.OwnerKnown = false
				return value
			}(),
			wantOutcome: OutcomeUnreadable, wantPath: true, wantWorkaround: true,
		},
		{
			name:       "login owner changes",
			darwinOnly: true,
			first:      systemLoginSnapshot(95, 1, "tty-1"),
			second: func() Snapshot {
				value := systemLoginSnapshot(95, 1, "tty-1")
				value.OwnerUID = 501
				return value
			}(),
			wantOutcome: OutcomeChanged, wantPath: true, wantOwner: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.darwinOnly && runtime.GOOS != "darwin" {
				t.Skip("the admitted system login program list is intentionally empty outside Darwin")
			}
			root := authorityRoot(t)
			values := []Snapshot{test.first}
			if test.second.Exact.Pid != 0 {
				values = append(values, test.second)
			}
			reader := &treeReader{reads: map[int64]int{}, session: test.first.Exact.Pid,
				snapshots: map[int64][]Snapshot{test.first.Exact.Pid: values}}
			_, err := Enroll(root, test.first.Exact.Pid, reader, time.Unix(2000, 0))
			if err == nil || !strings.Contains(err.Error(), test.wantOutcome) {
				t.Fatalf("refusal outcome mismatch: err=%v want=%s", err, test.wantOutcome)
			}
			message := err.Error()
			if !strings.Contains(message, "process pid "+strconv.FormatInt(test.first.Exact.Pid, 10)) {
				t.Fatalf("refusal did not name its pid: %s", message)
			}
			if test.wantPath && !strings.Contains(message, test.first.Executable) {
				t.Fatalf("refusal did not name its executable: %s", message)
			}
			if test.wantOwner && !strings.Contains(message, "owner uid "+strconv.FormatUint(uint64(test.first.OwnerUID), 10)) {
				t.Fatalf("refusal did not name its owner: %s", message)
			}
			if test.wantWorkaround && !strings.HasSuffix(message, enrollmentAncestryWorkaround) {
				t.Fatalf("refusal did not end with the enrollment workaround: %s", message)
			}
			if test.wantReason != "" && !strings.Contains(message, test.wantReason) {
				t.Fatalf("refusal did not explain why the process was not admitted: %s", message)
			}
		})
	}
}

func TestProofRecordsWithheldArgumentsForASystemLoginNode(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the admitted system login program list is intentionally empty outside Darwin")
	}
	root := authorityRoot(t)
	login := systemLoginSnapshot(100, 1, "tty-3")
	shell := authoritySnapshot(101, 100, []string{"-zsh"}, "tty-3")
	launchd := authoritySnapshot(1, 0, []string{"/sbin/launchd"}, "")
	launchd.OwnerUID = 0
	reader := &treeReader{reads: map[int64]int{}, session: 100,
		snapshots: map[int64][]Snapshot{1: {launchd}, 100: {login}, 101: {shell}}}
	if _, err := Enroll(root, 100, reader, time.Unix(2100, 0)); err != nil {
		t.Fatalf("enroll system login fixture: %v", err)
	}
	reader.reads = map[int64]int{}
	proof, err := Prove(root, 101, reader, time.Unix(2200, 0))
	if err != nil || !proof.Valid() || len(proof.Nodes) != 2 {
		t.Fatalf("system login proof failed: proof=%+v err=%v", proof, err)
	}
	loginNode := proof.Nodes[1]
	if !loginNode.ArgvWithheld || loginNode.ArgumentDigest != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" || loginNode.AgentRuntime != nil {
		t.Fatalf("system login audit node lost its withheld-argument facts: %+v", loginNode)
	}
	encoded, err := json.Marshal(loginNode)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"argvWithheld":true`) {
		t.Fatalf("system login audit node omitted argvWithheld: %s", encoded)
	}
}

func TestLinuxDoesNotAdmitLoginWithWithheldArguments(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("this pins the intentionally empty Linux system login program list")
	}
	root := authorityRoot(t)
	login := systemLoginSnapshot(102, 1, "tty-4")
	reader := &treeReader{reads: map[int64]int{}, session: 102,
		snapshots: map[int64][]Snapshot{102: {login}}}
	_, err := Enroll(root, 102, reader, time.Unix(2300, 0))
	if err == nil || !strings.Contains(err.Error(), OutcomeArgvUnreadable) {
		t.Fatalf("Linux admitted /usr/bin/login with withheld arguments: %v", err)
	}
}

func TestProofFailsClosedOnTerminalAndAncestryUncertainty(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*treeReader)
		want    string
	}{
		{
			name: "first process read fails",
			prepare: func(reader *treeReader) {
				delete(reader.snapshots, 30)
			},
			want: OutcomeUnreadable,
		},
		{
			name: "second process read fails",
			prepare: func(reader *treeReader) {
				reader.failAt = map[int64]map[int]error{30: {1: os.ErrNotExist}}
			},
			want: OutcomeUnreadable,
		},
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
			name: "first executable unreadable",
			prepare: func(reader *treeReader) {
				value := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				value.ExecutableKnown = false
				reader.snapshots[30] = []Snapshot{value}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "second argv unreadable",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				second := first
				second.Exact.ArgvKnown = false
				reader.snapshots[30] = []Snapshot{first, second}
			},
			want: OutcomeArgvUnreadable,
		},
		{
			name: "second executable unreadable",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				second := first
				second.ExecutableKnown = false
				reader.snapshots[30] = []Snapshot{first, second}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "first owner unreadable",
			prepare: func(reader *treeReader) {
				value := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				value.OwnerKnown = false
				reader.snapshots[30] = []Snapshot{value}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "second owner unreadable",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				second := first
				second.OwnerKnown = false
				reader.snapshots[30] = []Snapshot{first, second}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "first parent unreadable",
			prepare: func(reader *treeReader) {
				value := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				value.ParentKnown = false
				reader.snapshots[30] = []Snapshot{value}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "second parent unreadable",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1")
				second := first
				second.ParentKnown = false
				reader.snapshots[30] = []Snapshot{first, second}
			},
			want: OutcomeUnreadable,
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
			name: "argument count changed",
			prepare: func(reader *treeReader) {
				reader.snapshots[30] = []Snapshot{
					authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1"),
					authoritySnapshot(30, 20, []string{"wrapper", "added"}, "tty-1"),
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
			name: "terminal observation changed",
			prepare: func(reader *treeReader) {
				reader.snapshots[30] = []Snapshot{
					authoritySnapshot(30, 20, []string{"wrapper"}, "tty-1"),
					authoritySnapshot(30, 20, []string{"wrapper"}, "tty-replaced"),
				}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "parent snapshot unreadable",
			prepare: func(reader *treeReader) {
				reader.failAt = map[int64]map[int]error{20: {0: os.ErrNotExist}}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "session leader changed",
			prepare: func(reader *treeReader) {
				reader.session = 11
			},
			want: OutcomeTerminalMissing,
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

func TestProofRefusesAnEmptyInvokerPID(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	proof, err := Prove(root, 0, reader, time.Unix(1400, 0))
	if err == nil || proof.Outcome != OutcomeTerminalMissing || proof.Valid() {
		t.Fatalf("empty invoker PID did not fail closed: proof=%+v err=%v", proof, err)
	}
}

func TestFixtureGoalProofIsBoundToTheExactFakeRuntimeRoot(t *testing.T) {
	fixtureRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixtureRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	authorization, err := fixtureauth.New(fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := FixtureGoalProof(fixtureRoot, authorization.GoalHumanAuthority(), time.Unix(1500, 0))
	if err != nil || !proof.ValidFor(fixtureRoot) || !proof.FixtureOnly {
		t.Fatalf("fixture proof was not valid for its root: proof=%+v err=%v", proof, err)
	}
	if proof.ValidFor(t.TempDir()) {
		t.Fatal("fixture proof authorized a different root")
	}

	productionRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(productionRoot, "metasystem.conf"), []byte("metasystem.runtimes=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	productionAuthorization, err := fixtureauth.New(productionRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FixtureGoalProof(productionRoot, productionAuthorization.GoalHumanAuthority(), time.Unix(1500, 0)); err == nil {
		t.Fatal("a production root obtained fixture human authority")
	}
}

func TestTemporaryGoalProofIsDurableAndDistinct(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	terminalProof, err := Prove(root, 30, reader, time.Unix(1400, 0))
	if err != nil {
		t.Fatal(err)
	}
	terminalOperationID := "01J5X00000000000000000TERM-mac-m1-00000001"
	if err := RecordSetObligationProof(root, terminalOperationID, terminalProof); err != nil {
		t.Fatalf("record terminal proof: %v", err)
	}
	terminalRecord, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "authority", "proofs", terminalOperationID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(terminalRecord), "temporaryHumanWord") || strings.Contains(string(terminalRecord), "reviewBy") ||
		strings.Contains(string(terminalRecord), "departure") {
		t.Fatalf("an enrolled-terminal record was marked temporary: %s", terminalRecord)
	}

	humanWord := "  words relayed as Wido's authorization  "
	temporaryProof, err := temporaryGoalProofAt(root, humanWord, "2026-09-06", time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if temporaryProof.Departure != "R-32-m1" {
		t.Fatalf("temporary proof named the wrong authorizing ruling: %q", temporaryProof.Departure)
	}
	if temporaryProof.Valid() || temporaryProof.ValidFor(root) {
		t.Fatal("a temporary word became an enrolled-terminal ancestry proof")
	}
	if !temporaryProof.AuthorizesSetObligation(root) || temporaryProof.AuthorizesSetObligation(t.TempDir()) {
		t.Fatal("the temporary word was not scoped to set-obligation in its observed root")
	}
	if !temporaryProof.AuthorizesResume(root) || temporaryProof.AuthorizesResume(t.TempDir()) {
		t.Fatal("the temporary word was not scoped to resume in its observed root")
	}
	if !terminalProof.AuthorizesSetObligation(root) {
		t.Fatal("the ordinary enrolled-terminal proof stopped authorizing set-obligation")
	}
	if temporaryProof.Outcome == terminalProof.Outcome {
		t.Fatal("temporary and enrolled-terminal proofs share an indistinguishable outcome")
	}

	operationID := "01J5X00000000000000000TEMP-mac-m1-00000001"
	if err := RecordProof(root, operationID+"-generic", "goal set-obligation", temporaryProof); err == nil {
		t.Fatal("the generic recorder accepted temporary authority by matching an action string")
	}
	if err := RecordSetObligationProof(root, operationID, temporaryProof); err != nil {
		t.Fatalf("record temporary proof: %v", err)
	}
	encoded, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "authority", "proofs", operationID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Action string `json:"action"`
		Proof  Proof  `json:"proof"`
	}
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	if record.Action != "goal set-obligation" || record.Proof.TemporaryHumanWord != humanWord ||
		record.Proof.ReviewBy != "2026-09-06" || record.Proof.Departure != TemporaryWordRuling ||
		record.Proof.Outcome != OutcomeTemporary {
		t.Fatalf("temporary proof record lost its explicit provenance: %s", encoded)
	}
	if err := RecordResumeProof(root, operationID+"-resume", temporaryProof); err != nil {
		t.Fatalf("record temporary resume proof: %v", err)
	}
	if err := RecordProof(root, operationID+"-split", "goal split", temporaryProof); err == nil {
		t.Fatal("a temporary goal proof was recordable for an unrelated human-only verb")
	}
}

func TestTemporaryGoalProofUsesTheRealWallClock(t *testing.T) {
	root := authorityRoot(t)
	horizon, err := time.Parse(reviewByDateLayout, governance.TemporaryGoalAuthorityHorizon)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	proof, grantErr := TemporaryGoalProof(root, "Wido authorizes this goal mutation", governance.TemporaryGoalAuthorityHorizon)
	after := time.Now().UTC()
	if before.Truncate(24 * time.Hour).After(horizon) {
		if grantErr == nil || !strings.Contains(grantErr.Error(), "is in the past") {
			t.Fatalf("an expired wall-clock grant did not refuse as past: proof=%+v err=%v", proof, grantErr)
		}
		return
	}
	if grantErr != nil || proof.CheckedAt.Before(before) || proof.CheckedAt.After(after) {
		t.Fatalf("temporary grant did not bind the real wall clock: checkedAt=%s before=%s after=%s err=%v", proof.CheckedAt, before, after, grantErr)
	}
}

func TestTemporaryProofRequiresTheWholeRemoteWordPair(t *testing.T) {
	root := authorityRoot(t)
	for _, values := range []struct {
		name     string
		word     string
		reviewBy string
		want     string
	}{
		{name: "missing review date", word: "word", want: "travel together"},
		{name: "missing word", reviewBy: "2026-09-06", want: "travel together"},
		{name: "missing pair", want: "requires the supplied word"},
		{name: "whitespace word", word: " \t\n ", reviewBy: "2026-09-06", want: "non-whitespace"},
		{name: "too short", word: "Wido authorizes", reviewBy: "2026-09-06", want: "at least three words"},
		{name: "non-date review", word: "word", reviewBy: "whenever", want: "real date"},
		{name: "impossible review date", word: "word", reviewBy: "2026-09-31", want: "real date"},
	} {
		if _, err := temporaryGoalProofAt(root, values.word, values.reviewBy, time.Unix(1500, 0)); err == nil || !strings.Contains(err.Error(), values.want) {
			t.Fatalf("incomplete temporary word pair was accepted: %+v", values)
		}
	}
}

func TestTemporaryGoalProofEnforcesReviewExpiryAndHorizon(t *testing.T) {
	root := authorityRoot(t)
	for _, test := range []struct {
		name     string
		now      time.Time
		reviewBy string
		want     string
	}{
		{name: "past", now: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), reviewBy: "2026-09-01", want: "--review-by 2026-09-01 is in the past"},
		{name: "beyond horizon", now: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), reviewBy: "2026-09-07", want: "--review-by 2026-09-07 exceeds temporary goal authority horizon 2026-09-06"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := temporaryGoalProofAt(root, "Wido authorizes this goal mutation", test.reviewBy, test.now); err == nil || err.Error() != test.want {
				t.Fatalf("temporary authority expiry refusal mismatch: %v", err)
			}
		})
	}
}

func TestEnrolledAncestryTakesPrecedenceOverTemporaryFlags(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	proof, err := ProveOrTemporaryGoalAuthority(root, 30, reader, "one", "not-a-date", time.Unix(1600, 0))
	if err != nil {
		t.Fatalf("proved ancestry was shadowed by invalid temporary flags: %v", err)
	}
	if !proof.ValidFor(root) || proof.Outcome != OutcomeProven || proof.TemporaryHumanWord != "" || proof.ReviewBy != "" || proof.Departure != "" {
		t.Fatalf("the strong path did not win cleanly: %+v", proof)
	}
}
