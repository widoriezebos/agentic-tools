package census

import "fmt"

// find-ancestor walks up the process tree from a pid and returns the first
// ancestor whose argv matches a runtime signature — the agent session a
// delegate or wrapper runs under. arm uses it to infer arming identity when
// --pid is omitted.

// ProcInfo is one process's tree facts, as `ps -o ppid,pgid,command` yields.
type ProcInfo struct {
	PPID    int64
	PGID    int64
	Command string
}

// ProcTree reads one process's facts, or returns ok=false when the pid is
// gone or unreadable (the walk stops there). The kernel-backed reader shells
// to ps in production; tests inject a fixed tree.
type ProcTree interface {
	Info(pid int64) (info ProcInfo, ok bool)
}

// Ancestor is the matched agent ancestor.
type Ancestor struct {
	Pid     int64
	PGID    int64
	Runtime string
	Argv    string
}

// FindAncestor walks parents from pid (exclusive of pid 1 and below) and
// returns the first signature-matched ancestor. It is loop-safe: a pid seen
// twice ends the walk. It starts AT pid, classifies each level, stops at the
// first match, and errors when none is found.
func FindAncestor(tree ProcTree, pid int64, signatures []Signature) (Ancestor, error) {
	current := pid
	seen := map[int64]bool{}
	for current > 1 && !seen[current] {
		seen[current] = true
		info, ok := tree.Info(current)
		if !ok {
			break
		}
		if runtime := Runtime(info.Command, signatures); runtime != "" {
			return Ancestor{Pid: current, PGID: info.PGID, Runtime: runtime, Argv: info.Command}, nil
		}
		current = info.PPID
	}
	return Ancestor{}, fmt.Errorf("no immediate agent-signature ancestor was found")
}
