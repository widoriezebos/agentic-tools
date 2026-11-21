package humanauthority

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func loginSnapshot(pid, parent int64, name, tty string, uid uint32, denied bool) Snapshot {
	s := authoritySnapshot(pid, parent, []string{name}, tty)
	s.OwnerUID = uid
	s.KernelName = name
	s.ExecutableDenied = denied
	if denied {
		s.Executable, s.ExecutableKnown = "", false
	}
	s.Exact.StartTicks, s.Exact.BootID = pid*100, "boot-A"
	return s
}

func loginTable() *treeReader {
	return &treeReader{reads: map[int64]int{}, session: 40, snapshots: map[int64][]Snapshot{
		50: {loginSnapshot(50, 40, "sh", "pts/4", 501, false)},
		40: {loginSnapshot(40, 30, "bash", "pts/4", 501, false)},
		30: {loginSnapshot(30, 20, "sshd-session", "", 501, true)},
		20: {loginSnapshot(20, 1, "sshd-session", "", 0, true)},
		1:  {loginSnapshot(1, 0, "systemd", "", 0, true)},
	}}
}

func TestLoginTerminalProof(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000, 0)
	for _, tc := range []struct {
		name        string
		edit        func(*treeReader)
		want, later string
	}{
		{"ssh", nil, OutcomeProven, ""},
		{"listener", func(r *treeReader) {
			r.snapshots[20][0].ParentPID = 15
			r.snapshots[15] = []Snapshot{loginSnapshot(15, 1, "sshd", "", 0, true)}
		}, OutcomeProven, ""},
		{"tmux", func(r *treeReader) { r.snapshots[30] = []Snapshot{loginSnapshot(30, 1, "tmux", "", 501, false)} }, OutcomeProven, ""},
		{"pane shell job requested externally", func(r *treeReader) { r.snapshots[30] = []Snapshot{loginSnapshot(30, 1, "tmux", "", 501, false)} }, OutcomeProven, ""},
		{"runtime below leader", func(r *treeReader) { r.snapshots[50][0].Exact.Argv = []string{"codex", "exec"} }, OutcomeAgent, ""},
		{"runtime above leader", func(r *treeReader) { r.snapshots[30][0].Exact.Argv = []string{"codex", "exec"} }, OutcomeAgent, ""},
		{"monitor signature", func(r *treeReader) { r.snapshots[20][0].Exact.Argv = []string{"codex", "exec"} }, OutcomeAgent, ""},
		{"root signature then later failure", func(r *treeReader) {
			r.snapshots[20][0].Exact.Argv = []string{"codex", "exec"}
			r.snapshots[1][0].OwnerKnown = false
		}, OutcomeAgent, OutcomeUnreadable},
		{"headless", func(r *treeReader) { r.snapshots[50][0].TerminalID = "" }, OutcomeTerminalMissing, ""},
		{"orphan", func(r *treeReader) { r.snapshots[30][0].ParentPID = 1 }, OutcomeUnreadable, ""},
		{"wrong child", func(r *treeReader) { r.snapshots[30][0].KernelName = "sshd" }, OutcomeUnreadable, ""},
		{"missing child name", func(r *treeReader) { r.snapshots[30][0].KernelName = "" }, OutcomeUnreadable, ""},
		{"missing parent name", func(r *treeReader) { r.snapshots[20][0].KernelName = "" }, OutcomeUnreadable, ""},
		{"unknown parent uid", func(r *treeReader) { r.snapshots[20][0].OwnerKnown = false }, OutcomeUnreadable, ""},
		{"other account", func(r *treeReader) { r.snapshots[30][0].OwnerUID = 502 }, OutcomeUnreadable, ""},
		{"additional hidden account node", func(r *treeReader) { r.snapshots[20][0].OwnerUID = 501 }, OutcomeUnreadable, ""},
		{"readable account above child", func(r *treeReader) {
			r.snapshots[20][0].ParentPID = 15
			r.snapshots[15] = []Snapshot{loginSnapshot(15, 1, "bash", "", 501, false)}
		}, OutcomeUnreadable, ""},
		{"unrelated root service", func(r *treeReader) { r.snapshots[30] = []Snapshot{loginSnapshot(30, 1, "service", "", 0, true)} }, OutcomeUnreadable, ""},
		{"detached root monitor", func(r *treeReader) { r.snapshots[40][0].ParentPID = 20 }, OutcomeUnreadable, ""},
		{"not permission denied", func(r *treeReader) { r.snapshots[1][0].ExecutableDenied = false }, OutcomeUnreadable, ""},
		{"head of session withheld", func(r *treeReader) { r.session = 30; r.snapshots[30][0].TerminalID = "pts/4" }, OutcomeUnreadable, ""},
		{"child both withheld", func(r *treeReader) { r.snapshots[30][0].Exact.ArgvKnown = false }, OutcomeArgvUnreadable, ""},
		{"pid one both withheld", func(r *treeReader) { r.snapshots[1][0].Exact.ArgvKnown = false }, OutcomeArgvUnreadable, ""},
		{"pid one protected image", func(r *treeReader) { r.snapshots[1] = []Snapshot{withheldSystemSnapshot(1, 0, "", "/bin/sh")} }, OutcomeProven, ""},
		{"same-second root birth on a different boot", func(r *treeReader) {
			s := r.snapshots[1][0]
			s.Exact.StartTicks = 0
			r.snapshots[1] = []Snapshot{s}
			s.Exact.BootID = "boot-B"
			r.snapshots[1] = append(r.snapshots[1], s)
		}, OutcomeReused, ""},
		{"parent birth changes", func(r *treeReader) {
			s := r.snapshots[20][0]
			s.Exact.StartTicks++
			r.snapshots[20] = append(r.snapshots[20], s)
		}, OutcomeReused, ""},
		{"name changes between reads", func(r *treeReader) {
			s := r.snapshots[30][0]
			s.KernelName = "bash"
			r.snapshots[30] = append(r.snapshots[30], r.snapshots[30][0], s)
		}, OutcomeChanged, ""},
		{"denial changes between reads", func(r *treeReader) {
			s := r.snapshots[30][0]
			s.ExecutableKnown = true
			s.ExecutableDenied = false
			s.Executable = "/sbin/sshd-session"
			r.snapshots[30] = append(r.snapshots[30], r.snapshots[30][0], s)
		}, OutcomeChanged, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := loginTable()
			if tc.edit != nil {
				tc.edit(r)
			}
			p, err := ProveTerminal(root, 50, r, now)
			if runtime.GOOS != "linux" {
				if tc.name == "ssh" && (p.Outcome != OutcomeUnreadable || err == nil || !strings.Contains(err.Error(), enrollmentAncestryWhere)) {
					t.Fatalf("baseline refusal: %v", err)
				}
				return
			}
			good := tc.want == OutcomeProven
			if (err == nil) != good || p.Outcome != tc.want || p.ContinuationOutcome != tc.later || p.TerminalValidFor(root) != good || p.ValidFor(root) {
				t.Fatalf("outcome %s, continuation %s, valid %v, error %v", p.Outcome, p.ContinuationOutcome, p.TerminalValidFor(root), err)
			}
			if tc.name != "ssh" {
				return
			}
			if len(p.Nodes) != 5 || p.Nodes[2].ExecutableWithheld != "ssh-session" || p.Nodes[3].ExecutableWithheld != "ssh-monitor" || p.Nodes[4].ExecutableWithheld != "pid-one" {
				t.Fatalf("missing linked evidence: %+v", p.Nodes)
			}
			for _, n := range p.Nodes[2:] {
				if n.ExecutableDigest != "" || n.ArgumentDigest == "" {
					t.Fatal("incorrect digests for hidden image")
				}
			}
			data, _ := json.Marshal(p)
			var decoded Proof
			if err := json.Unmarshal(data, &decoded); err != nil || decoded.TerminalValidFor(root) {
				t.Fatal("serialized proof granted authority", err)
			}
			if p.TerminalValidFor(t.TempDir()) {
				t.Fatal("proof changed checkout")
			}
			p.Nodes[2].ExecutableWithheld = "pid-one"
			if p.TerminalValidFor(root) {
				t.Fatal("wrong rule accepted")
			}
		})
	}
}

func TestLoginEnrollmentGeneration(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("Linux login evidence")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := loginTable()
	now := time.Unix(2000, 0)
	e, err := Enroll(root, 40, r, "Wido", now)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := os.ReadFile(enrollmentPath(root))
	if err != nil {
		t.Fatal(err)
	}
	// The schema-1 reader in the older binary only knows these fields.
	var old struct {
		Schema                        int
		EnrolledAt                    time.Time
		Generation                    uint64
		Human, TerminalID             string
		TerminalRef, SessionLeaderRef ProcessRef
	}
	decoder := json.NewDecoder(bytes.NewReader(prior))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&old); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("old decoder: %v", err)
	}
	p, err := Prove(root, 50, r, now)
	if err != nil || !p.EnrolledTerminalFor(root) {
		t.Fatal("enrolled child:", err)
	}
	for _, change := range []string{"reboot", "pid reuse", "tty reuse"} {
		fresh := loginTable()
		switch change {
		case "reboot":
			for _, values := range fresh.snapshots {
				values[0].Exact.BootID = "boot-B"
			}
		case "pid reuse":
			fresh.snapshots[40][0].Exact.StartTicks++
		case "tty reuse":
			fresh.snapshots[40][0].Exact.Pid = 41
			fresh.snapshots[41] = fresh.snapshots[40]
			delete(fresh.snapshots, 40)
			fresh.snapshots[50][0].ParentPID = 41
			fresh.session = 41
		}
		p, err := Prove(root, 50, fresh, now)
		if err == nil || p.EnrolledTerminalFor(root) {
			t.Fatal(change, " kept the old enrollment")
		}
	}
	repeat, err := Enroll(root, 40, r, "Wido", now.Add(time.Hour))
	if err != nil || !repeat.Repeat || repeat.Generation != e.Generation || !repeat.EnrolledAt.Equal(e.EnrolledAt) {
		t.Fatal("repeat:", err)
	}
	after, err := os.ReadFile(enrollmentPath(root))
	if err != nil || !bytes.Equal(prior, after) {
		t.Fatal("repeat changed record", err)
	}
	r.snapshots[40][0].TerminalID = "pts/5"
	r.snapshots[40][0].Exact.StartTicks++
	r.snapshots[30] = []Snapshot{loginSnapshot(30, 1, "tmux", "", 501, false)}
	next, err := Enroll(root, 40, r, "Wido", now.Add(time.Hour))
	if err != nil || next.Generation != e.Generation+1 || reflect.DeepEqual(next.Login, e.Login) {
		t.Fatal("replacement lost its own evidence", err)
	}
}

func TestLoginEnrolledWalkKeepsExecutableRequirement(t *testing.T) {
	t.Parallel()
	if !linuxLogin {
		t.Skip("Linux login evidence")
	}
	r := loginTable()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000, 0)
	if _, err := Enroll(root, 40, r, "Wido", now); err != nil {
		t.Fatal(err)
	}
	// The audit evidence has no say in the next act.
	e, err := ReadEnrollment(root)
	if err != nil {
		t.Fatal(err)
	}
	e.Login.Nodes[1].ExecutableWithheld = "unknown rule"
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enrollmentPath(root), data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Prove(root, 50, r, now)
	if err != nil || !p.EnrolledTerminalFor(root) {
		t.Fatal("the audit evidence decided the act", err)
	}
	r.snapshots[40][0].Executable = ""
	r.snapshots[40][0].ExecutableKnown = false
	r.snapshots[40][0].ExecutableDenied = true
	p, err = Prove(root, 50, r, now)
	if err == nil || p.Outcome != OutcomeUnreadable {
		t.Fatal("the enrolled walk admitted a hidden program", err)
	}
}
