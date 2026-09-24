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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type treeReader struct {
	snapshots map[int64][]Snapshot
	reads     map[int64]int
	failAt    map[int64]map[int]error
	session   int64
	sessions  map[int64]int64
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

func (reader *treeReader) SessionLeader(pid int64) (int64, error) {
	if session, ok := reader.sessions[pid]; ok {
		return session, nil
	}
	return reader.session, nil
}

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

func withheldSystemSnapshot(pid, parent int64, terminal, executable string) Snapshot {
	snapshot := authoritySnapshot(pid, parent, nil, terminal)
	snapshot.Exact.ArgvKnown = false
	snapshot.Executable = executable
	snapshot.OwnerUID = 0
	return snapshot
}

func platformLauncherExecutable() string {
	if runtime.GOOS == "darwin" {
		return "/sbin/launchd"
	}
	return "/sbin/init"
}

func systemRootSnapshot() Snapshot {
	return withheldSystemSnapshot(1, 0, "", platformLauncherExecutable())
}

func authorityRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match codex-agent'\n"
	if err := testexec.WriteFile(filepath.Join(directory, "codex.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func enrolledReader() *treeReader {
	return &treeReader{
		reads: map[int64]int{}, session: 10,
		snapshots: map[int64][]Snapshot{
			1:  {systemRootSnapshot()},
			10: {authoritySnapshot(10, 1, []string{"terminal-session"}, "tty-1")},
			20: {authoritySnapshot(20, 10, []string{"interactive-shell"}, "tty-1")},
			30: {authoritySnapshot(30, 20, []string{"command-wrapper", "private-argument"}, "tty-1")},
		},
	}
}

func enrollTestTerminal(t *testing.T, root string, reader *treeReader) {
	t.Helper()
	if _, err := Enroll(root, 20, reader, "Wido", time.Unix(1000, 0)); err != nil {
		t.Fatalf("enroll terminal: %v", err)
	}
	reader.reads = map[int64]int{}
}

func TestProofRequiresExactAgentFreeEnrolledAncestry(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	proof, err := Prove(root, 30, reader, time.Unix(1100, 0))
	if err != nil || !proof.Valid() || proof.Outcome != OutcomeProven || proof.Grade != GradeEnrolled || len(proof.Nodes) != 2 {
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
	if !proof.EnrolledTerminalFor(root) {
		t.Fatal("a real enrolled-terminal observation was not classified as one")
	}
	var parsed Proof
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		t.Fatal(err)
	}
	parsed.Grade = ""
	if parsed.Valid() || parsed.AuthorityGrade() != GradeEnrolled {
		t.Fatalf("legacy parsed proof validity or grade changed: valid=%t grade=%q", parsed.Valid(), parsed.AuthorityGrade())
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

func TestProveWithoutEnrollmentIsTheTerminalWalkPlusNotEnrolled(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	now := time.Unix(1100, 0)

	terminalProof, err := ProveTerminal(root, 30, reader, now)
	if err != nil || !terminalProof.TerminalValidFor(root) || terminalProof.ValidFor(root) ||
		terminalProof.Grade != GradeTerminal || terminalProof.TerminalGeneration != 0 ||
		terminalProof.TerminalRef.PID != 10 || terminalProof.InvokerRef.PID != 30 ||
		terminalProof.ObservedTerminalID() != "tty-1" || len(terminalProof.Nodes) != 4 {
		t.Fatalf("agent-free terminal proof mismatch: proof=%+v err=%v", terminalProof, err)
	}
	rootNode := terminalProof.Nodes[len(terminalProof.Nodes)-1]
	if rootNode.Ref.PID != 1 || rootNode.ParentRef.PID != 0 || !rootNode.OwnerKnown || rootNode.OwnerUID != 0 ||
		!rootNode.ArgvWithheld || rootNode.AgentRuntime != nil {
		t.Fatalf("protected process-tree root was not recorded as an argument-withheld non-agent node: %+v", rootNode)
	}
	if _, err := os.Stat(enrollmentPath(root)); !os.IsNotExist(err) {
		t.Fatalf("terminal proof read or wrote an enrollment: %v", err)
	}
	truncated := terminalProof
	truncated.Nodes = truncated.Nodes[:len(truncated.Nodes)-1]
	if truncated.Valid() {
		t.Fatal("terminal proof remained valid after its process-tree root was removed")
	}

	reader.reads = map[int64]int{}
	missing, err := Prove(root, 30, reader, now)
	if err == nil || missing.Outcome != OutcomeNotEnrolled || missing.Grade != "" || missing.Valid() ||
		missing.TerminalGeneration != 0 || missing.TerminalRef.PID != 10 || len(missing.Nodes) != 4 ||
		!strings.Contains(err.Error(), OutcomeNotEnrolled) || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("missing enrollment did not retain the successful terminal walk: proof=%+v err=%v", missing, err)
	}
}

func TestTerminalWalkChecksAgentsAboveTheSessionLeader(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	reader.snapshots[5] = []Snapshot{withheldSystemSnapshot(5, 4, "", "/bin/sh")}
	reader.snapshots[10] = []Snapshot{authoritySnapshot(10, 5, []string{"terminal-session"}, "tty-1")}
	reader.snapshots[4] = []Snapshot{authoritySnapshot(4, 1, []string{"codex-agent", "run"}, "")}

	proof, err := ProveTerminal(root, 30, reader, time.Unix(1150, 0))
	if err == nil || proof.Outcome != OutcomeAgent || proof.Grade != "" || len(proof.Nodes) != 6 {
		t.Fatalf("an agent above the session leader passed terminal authority: proof=%+v err=%v", proof, err)
	}
	agent := proof.Nodes[len(proof.Nodes)-2].AgentRuntime
	if agent == nil || *agent != "codex" {
		t.Fatalf("the agent above the session leader was not recorded with its runtime: %+v", proof.Nodes)
	}
	if !proof.Nodes[len(proof.Nodes)-3].ArgvWithheld {
		t.Fatalf("the protected system node below the agent was not recorded: %+v", proof.Nodes)
	}
	if rootNode := proof.Nodes[len(proof.Nodes)-1]; rootNode.Ref.PID != 1 || rootNode.ParentRef != (ProcessRef{}) {
		t.Fatalf("the agent refusal did not finish the walk at the process-tree root: %+v", proof.Nodes)
	}
}

func TestTerminalWalkKeepsAgentOutcomeAcrossLaterAncestryFailures(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*treeReader)
		want    string
	}{
		{
			name: "unreadable ancestor",
			prepare: func(reader *treeReader) {
				reader.failAt = map[int64]map[int]error{20: {1: os.ErrPermission}}
			},
			want: OutcomeUnreadable,
		},
		{
			name: "changed ancestor",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(20, 10, []string{"interactive-shell"}, "tty-1")
				changed := first
				changed.ParentPID = 11
				reader.snapshots[20] = []Snapshot{first, first, changed}
			},
			want: OutcomeChanged,
		},
		{
			name: "reused ancestor",
			prepare: func(reader *treeReader) {
				first := authoritySnapshot(20, 10, []string{"interactive-shell"}, "tty-1")
				reused := first
				reused.Exact.StartedAt = first.Exact.StartedAt.Add(time.Second)
				reader.snapshots[20] = []Snapshot{first, reused, reused}
			},
			want: OutcomeReused,
		},
		{
			name: "cyclic ancestor",
			prepare: func(reader *treeReader) {
				reader.snapshots[20] = []Snapshot{authoritySnapshot(20, 30, []string{"interactive-shell"}, "tty-1")}
			},
			want: OutcomeCycle,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := authorityRoot(t)
			reader := enrolledReader()
			reader.snapshots[30] = []Snapshot{authoritySnapshot(30, 20, []string{"codex-agent", "run"}, "tty-1")}
			test.prepare(reader)

			proof, err := ProveTerminal(root, 30, reader, time.Unix(1160, 0))
			if err == nil || proof.Outcome != OutcomeAgent || proof.ContinuationOutcome != test.want || len(proof.Nodes) == 0 {
				t.Fatalf("later %s renamed or erased the agent refusal: proof=%+v err=%v", test.want, proof, err)
			}
			if proof.Nodes[0].AgentRuntime == nil || *proof.Nodes[0].AgentRuntime != "codex" ||
				!strings.Contains(err.Error(), OutcomeAgent+": codex") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("agent refusal did not name its runtime and later outcome: proof=%+v err=%v", proof, err)
			}
		})
	}
}

func TestKernelAncestryWalkReachesProcessTreeRoot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	reader := KernelReader{}
	invokerPID := int64(os.Getpid())
	invoker, refusal := stableRead(reader, invokerPID)
	if refusal != nil {
		t.Fatalf("live ancestry could not read its invoker: %v", refusal)
	}
	signatures, _, err := signatureSet(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, _, outcome, continuation, walkErr := walkProcessTree(invokerPID, invoker, reader, signatures, nil)
	if continuation != "" {
		t.Fatalf("live ancestry recorded a later failure after outcome %s: %s", outcome, continuation)
	}
	if len(nodes) == 0 {
		t.Fatalf("live ancestry recorded no process nodes: outcome=%s err=%v", outcome, walkErr)
	}
	last := nodes[len(nodes)-1]
	if last.Ref.PID != 1 || last.ParentRef != (ProcessRef{}) {
		t.Fatalf("live ancestry did not reach the known process-tree root: last=%+v outcome=%s err=%v", last, outcome, walkErr)
	}
	switch outcome {
	case OutcomeProven:
		if walkErr != nil {
			t.Fatalf("agent-free live ancestry returned an error after reaching the root: %v", walkErr)
		}
	case OutcomeAgent:
		if walkErr == nil {
			t.Fatal("agent-descended live ancestry reached the root without its refusal")
		}
		namedRuntime := ""
		for _, node := range nodes {
			if node.AgentRuntime != nil && *node.AgentRuntime != "" {
				namedRuntime = *node.AgentRuntime
				break
			}
		}
		if namedRuntime == "" {
			t.Fatalf("agent-descended live ancestry did not name a real runtime: nodes=%+v", nodes)
		}
		if !strings.Contains(walkErr.Error(), OutcomeAgent+": "+namedRuntime) {
			t.Fatalf("agent-descended live ancestry error did not name runtime %q: %v", namedRuntime, walkErr)
		}
	default:
		t.Fatalf("live ancestry did not complete with an honest root outcome: outcome=%s err=%v nodes=%+v", outcome, walkErr, nodes)
	}
	t.Logf("live ancestry reached process-tree root pid %d with outcome %s after checking %d nodes", last.Ref.PID, outcome, len(nodes))
}

func TestTerminalConstraintEndsAtTheSessionLeader(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	rootSnapshot := systemRootSnapshot()
	rootSnapshot.TerminalID = "another-terminal"
	reader.snapshots[1] = []Snapshot{rootSnapshot}

	proof, err := ProveTerminal(root, 30, reader, time.Unix(1175, 0))
	if err != nil || !proof.TerminalValidFor(root) || len(proof.Nodes) != 4 {
		t.Fatalf("an agent-free ancestor above the session leader was subjected to the terminal constraint: proof=%+v err=%v", proof, err)
	}
	if reader.reads[1] != 3 {
		t.Fatalf("the process-tree root was read %d times, want one parent-identity pin and two stable observations", reader.reads[1])
	}
	for index, node := range proof.Nodes {
		want := index == 2
		if node.TerminalMatch != want {
			t.Fatalf("node %d terminal match = %t, want %t: %+v", index, node.TerminalMatch, want, proof.Nodes)
		}
	}
}

func TestTerminalWalkRefusesAReusedParentBetweenNodes(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	original := authoritySnapshot(20, 10, []string{"interactive-shell"}, "tty-1")
	reused := original
	reused.Exact.StartedAt = original.Exact.StartedAt.Add(time.Second)
	reader.snapshots[20] = []Snapshot{original, reused, reused}

	proof, err := ProveTerminal(root, 30, reader, time.Unix(1180, 0))
	if err == nil || proof.Outcome != OutcomeReused || proof.Valid() {
		t.Fatalf("terminal walk adopted a different process at the recorded parent pid: proof=%+v err=%v", proof, err)
	}
}

func TestAgentShellRefusedForBothGrades(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	reader.snapshots[40] = []Snapshot{authoritySnapshot(40, 20, []string{"codex-agent", "run"}, "tty-1")}
	reader.snapshots[50] = []Snapshot{authoritySnapshot(50, 40, []string{"shell-wrapper"}, "tty-1")}

	terminalProof, terminalErr := ProveTerminal(root, 50, reader, time.Unix(1200, 0))
	if terminalErr == nil || terminalProof.Outcome != OutcomeAgent || terminalProof.Grade != "" || terminalProof.TerminalValidFor(root) {
		t.Fatalf("agent shell passed terminal grade: proof=%+v err=%v", terminalProof, terminalErr)
	}
	reader.reads = map[int64]int{}
	fullProof, fullErr := Prove(root, 50, reader, time.Unix(1200, 0))
	if fullErr == nil || fullProof.Outcome != OutcomeAgent || fullProof.Grade != "" || fullProof.ValidFor(root) {
		t.Fatalf("agent shell passed the missing-enrollment full walk: proof=%+v err=%v", fullProof, fullErr)
	}

	reader.reads = map[int64]int{}
	reader.snapshots[40] = []Snapshot{authoritySnapshot(40, 20, []string{"codex-agent", "run"}, "tty-agent")}
	terminalProof, terminalErr = ProveTerminal(root, 50, reader, time.Unix(1200, 0))
	if terminalErr == nil || terminalProof.Outcome != OutcomeAgent {
		t.Fatalf("an agent on another terminal lost the agent refusal: proof=%+v err=%v", terminalProof, terminalErr)
	}
}

func TestHeadlessCallerFailsBothGrades(t *testing.T) {
	root := authorityRoot(t)
	reader := &treeReader{
		reads: map[int64]int{}, session: 71,
		snapshots: map[int64][]Snapshot{
			70: {authoritySnapshot(70, 71, []string{"command-wrapper"}, "")},
			71: {authoritySnapshot(71, 1, []string{"scheduler"}, "")},
		},
	}

	terminalProof, terminalErr := ProveTerminal(root, 70, reader, time.Unix(1300, 0))
	if terminalErr == nil || terminalProof.Outcome != OutcomeTerminalMissing || terminalProof.Grade != "" || terminalProof.TerminalValidFor(root) {
		t.Fatalf("headless caller passed terminal grade: proof=%+v err=%v", terminalProof, terminalErr)
	}
	reader.reads = map[int64]int{}
	fullProof, fullErr := Prove(root, 70, reader, time.Unix(1300, 0))
	if fullErr == nil || fullProof.Outcome != OutcomeTerminalMissing || fullProof.Grade != "" || fullProof.ValidFor(root) {
		t.Fatalf("headless caller passed the missing-enrollment full walk: proof=%+v err=%v", fullProof, fullErr)
	}
}

func TestUnreadableEnrollmentReturnsNotEnrolledAfterTheTerminalWalk(t *testing.T) {
	root := authorityRoot(t)
	reader := enrolledReader()
	path := enrollmentPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	proof, err := Prove(root, 30, reader, time.Unix(1400, 0))
	if err == nil || proof.Outcome != OutcomeNotEnrolled || proof.Grade != "" || len(proof.Nodes) != 4 ||
		!strings.Contains(err.Error(), "human terminal enrollment is unreadable") {
		t.Fatalf("unreadable enrollment lost the terminal walk or read reason: proof=%+v err=%v", proof, err)
	}
}

func TestEnrollmentNameIsOptionalOnReadAndRequiredOnWrite(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	reader := enrolledReader()
	enrollment, err := Enroll(root, 20, reader, "  Wido van Riezebos  ", time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.Human != "  Wido van Riezebos  " {
		t.Fatalf("enrollment changed the human's exact name: %q", enrollment.Human)
	}
	if _, err := Enroll(root, 20, reader, " \t ", time.Unix(1100, 0)); err == nil || !strings.Contains(err.Error(), "human's name") {
		t.Fatalf("a nameless new enrollment was accepted: %v", err)
	}

	path := enrollmentPath(root)
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]any
	if err := json.Unmarshal(encoded, &legacy); err != nil {
		t.Fatal(err)
	}
	delete(legacy, "human")
	encoded, err = json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	read, err := ReadEnrollment(root)
	if err != nil || read.Human != "" {
		t.Fatalf("a legacy fieldless enrollment did not parse: enrollment=%+v err=%v", read, err)
	}
	reader.reads = map[int64]int{}
	proof, err := Prove(root, 30, reader, time.Unix(1200, 0))
	if err != nil || !proof.EnrolledTerminalFor(root) {
		t.Fatalf("a legacy fieldless enrollment did not still prove: proof=%+v err=%v", proof, err)
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
		login := withheldSystemSnapshot(61287, 1280, "tty-1", "/usr/bin/login")
		shell := authoritySnapshot(61288, 61287, []string{"-zsh"}, "tty-1")
		shell.Executable = "/bin/zsh"
		command := authoritySnapshot(70, 61288, []string{"metasystem", "goal", "set-obligation"}, "tty-1")
		reader := &treeReader{
			reads: map[int64]int{}, session: 61287,
			snapshots: map[int64][]Snapshot{
				1:     {systemRootSnapshot()},
				1280:  {terminal},
				61287: {login},
				61288: {shell},
				70:    {command},
			},
		}
		enrollment, err := Enroll(root, 61288, reader, "Wido", time.Unix(1700, 0))
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
				1:    {systemRootSnapshot()},
				8458: {shell},
				8457: {server},
			},
		}
		enrollment, err := Enroll(root, 8458, reader, "Wido", time.Unix(1900, 0))
		if err != nil {
			t.Fatalf("tmux shell enrollment failed: %v", err)
		}
		if enrollment.TerminalRef.PID != 8458 || enrollment.SessionLeader.PID != 8458 {
			t.Fatalf("tmux enrollment recorded the wrong roots: %+v", enrollment)
		}
	})
}

func TestProtectedSystemImageAdmissionRequiresStableWithheldArgumentsAndRootOwner(t *testing.T) {
	tests := []struct {
		name           string
		first          Snapshot
		second         Snapshot
		wantOutcome    string
		wantPath       bool
		wantOwner      bool
		wantWorkaround bool
		wantReason     string
	}{
		{
			name: "user-owned process with withheld arguments",
			first: func() Snapshot {
				value := withheldSystemSnapshot(91, 0, "tty-1", "/bin/sh")
				value.OwnerUID = 501
				return value
			}(),
			wantOutcome: OutcomeArgvUnreadable, wantPath: true, wantOwner: true, wantWorkaround: true,
			wantReason: "the operating system withholds this process's arguments and the process is not owned by root",
		},
		{
			name:  "system process arguments become readable",
			first: withheldSystemSnapshot(92, 0, "tty-1", "/bin/sh"),
			second: func() Snapshot {
				value := withheldSystemSnapshot(92, 0, "tty-1", "/bin/sh")
				value.Exact.Argv = []string{"sh"}
				value.Exact.ArgvKnown = true
				return value
			}(),
			wantOutcome: OutcomeChanged, wantPath: true, wantOwner: true,
		},
		{
			name: "system process arguments become withheld",
			first: func() Snapshot {
				value := withheldSystemSnapshot(96, 0, "tty-1", "/bin/sh")
				value.Exact.Argv = []string{"sh"}
				value.Exact.ArgvKnown = true
				return value
			}(),
			second:         withheldSystemSnapshot(96, 0, "tty-1", "/bin/sh"),
			wantOutcome:    OutcomeArgvUnreadable,
			wantPath:       true,
			wantOwner:      true,
			wantWorkaround: true,
			wantReason:     "the process's arguments changed from readable to withheld between observations",
		},
		{
			name: "system process executable unreadable",
			first: func() Snapshot {
				value := withheldSystemSnapshot(93, 0, "tty-1", "/bin/sh")
				value.ExecutableKnown = false
				return value
			}(),
			wantOutcome: OutcomeUnreadable, wantOwner: true, wantWorkaround: true,
		},
		{
			name: "system process owner unknown",
			first: func() Snapshot {
				value := withheldSystemSnapshot(94, 0, "tty-1", "/bin/sh")
				value.OwnerKnown = false
				return value
			}(),
			wantOutcome: OutcomeUnreadable, wantPath: true, wantWorkaround: true,
		},
		{
			name:  "system process owner changes",
			first: withheldSystemSnapshot(95, 0, "tty-1", "/bin/sh"),
			second: func() Snapshot {
				value := withheldSystemSnapshot(95, 0, "tty-1", "/bin/sh")
				value.OwnerUID = 501
				return value
			}(),
			wantOutcome: OutcomeChanged, wantPath: true, wantOwner: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := authorityRoot(t)
			values := []Snapshot{test.first}
			if test.second.Exact.Pid != 0 {
				values = append(values, test.second)
			}
			reader := &treeReader{reads: map[int64]int{}, session: test.first.Exact.Pid,
				snapshots: map[int64][]Snapshot{test.first.Exact.Pid: values}}
			_, err := Enroll(root, test.first.Exact.Pid, reader, "Wido", time.Unix(2000, 0))
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

func TestRootOwnedProcessWithWritableExecutableCannotHideArguments(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "writable-system-image")
	if err := testexec.WriteFile(executable, []byte("fixture"), 0o777); err != nil {
		t.Fatal(err)
	}
	snapshot := withheldSystemSnapshot(97, 0, "tty-1", executable)
	reader := &treeReader{reads: map[int64]int{}, session: 97,
		snapshots: map[int64][]Snapshot{97: {snapshot}}}
	proof, err := ProveTerminal(authorityRoot(t), 97, reader, time.Unix(2050, 0))
	if err == nil || proof.Outcome != OutcomeArgvUnreadable ||
		!strings.Contains(err.Error(), "protected from group and other writes") {
		t.Fatalf("writable executable admitted withheld arguments: proof=%+v err=%v", proof, err)
	}
}

func TestProofRecordsWithheldArgumentsForAProtectedSystemNode(t *testing.T) {
	root := authorityRoot(t)
	systemNode := withheldSystemSnapshot(100, 1, "tty-3", "/bin/sh")
	shell := authoritySnapshot(101, 100, []string{"shell"}, "tty-3")
	reader := &treeReader{reads: map[int64]int{}, session: 100,
		snapshots: map[int64][]Snapshot{1: {systemRootSnapshot()}, 100: {systemNode}, 101: {shell}}}
	if _, err := Enroll(root, 100, reader, "Wido", time.Unix(2100, 0)); err != nil {
		t.Fatalf("enroll protected system process fixture: %v", err)
	}
	reader.reads = map[int64]int{}
	proof, err := Prove(root, 101, reader, time.Unix(2200, 0))
	if err != nil || !proof.Valid() || len(proof.Nodes) != 2 {
		t.Fatalf("protected system process proof failed: proof=%+v err=%v", proof, err)
	}
	systemNodeProof := proof.Nodes[1]
	if !systemNodeProof.OwnerKnown || systemNodeProof.OwnerUID != 0 || !systemNodeProof.ArgvWithheld ||
		systemNodeProof.ArgumentDigest != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" || systemNodeProof.AgentRuntime != nil {
		t.Fatalf("protected system audit node lost its withheld-argument facts: %+v", systemNodeProof)
	}
	encoded, err := json.Marshal(systemNodeProof)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"argvWithheld":true`) {
		t.Fatalf("protected system audit node omitted argvWithheld: %s", encoded)
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
	if err != nil || !proof.ValidFor(fixtureRoot) || !proof.TerminalValidFor(fixtureRoot) ||
		proof.AuthorityGrade() != GradeEnrolled || !proof.FixtureOnly {
		t.Fatalf("fixture proof was not valid for its root: proof=%+v err=%v", proof, err)
	}
	if proof.EnrolledTerminalFor(fixtureRoot) {
		t.Fatal("fixture authority was classified as a real enrolled terminal")
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
	if temporaryProof.EnrolledTerminalFor(root) {
		t.Fatal("a temporary word was classified as a real enrolled terminal")
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

func TestChannelProofIsNotAnEnrolledTerminalProof(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	proof, err := VerifiedChannelAnswerProof(root, governance.RecordedChannelAuthority{
		Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer, Provider: "slack", UserID: "UWIDO",
		MessageRef: "1/2", ContextID: "question-1", Step: 42,
	}, time.Unix(1500, 0))
	if err != nil || !proof.ChannelWordFor(root) {
		t.Fatalf("verified channel fixture was not valid: proof=%+v err=%v", proof, err)
	}
	if proof.EnrolledTerminalFor(root) {
		t.Fatal("a verified channel answer was classified as a real enrolled terminal")
	}
}

func TestTemporaryGoalProofWrapperRejectsIncompleteWordPair(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	if _, err := TemporaryGoalProof(root, "", ""); err == nil || !strings.Contains(err.Error(), "requires the supplied word") {
		t.Fatalf("temporary goal proof wrapper did not enforce the word pair: %v", err)
	}
}

func TestTemporaryGoalProofUsesTheRealWallClock(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	horizon, err := time.Parse(reviewByDateLayout, governance.TemporaryGoalAuthorityHorizon)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := horizon.Add(24 * time.Hour)
	before := time.Now().UTC()
	proof, grantErr := TemporaryGoalProof(root, "Wido authorizes this goal mutation", governance.TemporaryGoalAuthorityHorizon)
	after := time.Now().UTC()
	pastErr := "--review-by " + governance.TemporaryGoalAuthorityHorizon + " is in the past"

	validProof := grantErr == nil && proof.TemporaryResumeFor(root) &&
		!proof.CheckedAt.Before(before) && !proof.CheckedAt.After(after) && proof.CheckedAt.Before(expiresAt)
	if after.Before(expiresAt) {
		if !validProof {
			t.Fatalf("temporary grant did not bind the real wall clock: proof=%+v before=%s after=%s err=%v", proof, before, after, grantErr)
		}
		return
	}
	if !before.Before(expiresAt) {
		if grantErr == nil || grantErr.Error() != pastErr {
			t.Fatalf("an expired wall-clock grant did not refuse as past: proof=%+v err=%v", proof, grantErr)
		}
		return
	}
	if !validProof && (grantErr == nil || grantErr.Error() != pastErr) {
		t.Fatalf("a grant straddling expiry had neither a valid proof nor the past refusal: proof=%+v before=%s after=%s err=%v", proof, before, after, grantErr)
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
	t.Parallel()
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
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	proof, err := temporaryGoalProofAt(root, "Wido authorizes this goal mutation", "2026-09-06", now)
	if err != nil || !proof.CheckedAt.Equal(now) {
		t.Fatalf("temporary proof checkedAt = %s, want %s: %v", proof.CheckedAt, now, err)
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
