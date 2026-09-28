package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var helmNow = time.Date(2026, 9, 28, 19, 14, 3, 0, time.UTC)

// helmTree is a fixed process tree: pid -> (parent, argv, terminal).
type helmTree map[int64]struct {
	parent   int64
	argv     string
	terminal string
}

func (tree helmTree) Read(pid int64) (humanauthority.Snapshot, error) {
	node, ok := tree[pid]
	if !ok {
		return humanauthority.Snapshot{}, os.ErrNotExist
	}
	argv := strings.Fields(node.argv)
	return humanauthority.Snapshot{Exact: identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), Argv: argv, ArgvKnown: true},
		Executable: "/fixture/bin/" + argv[0], ExecutableKnown: true, OwnerUID: 501, OwnerKnown: true,
		ParentPID: node.parent, ParentKnown: true, TerminalID: node.terminal, TerminalKnown: true}, nil
}

// SessionLeader is the nearest ancestor whose parent is launchd (pid 1).
func (tree helmTree) SessionLeader(pid int64) (int64, error) {
	for tree[pid].parent > 1 {
		pid = tree[pid].parent
	}
	return pid, nil
}

// person is zsh (20) under login (10, the session leader on tty-1) under launchd.
func person() helmTree {
	return helmTree{1: {0, "launchd", ""}, 10: {1, "login -pf wido", "tty-1"}, 20: {10, "-zsh", "tty-1"},
		50: {1, "sshd wido", "tty-2"}, 60: {50, "-zsh", "tty-2"}, 70: {10, "codex-agent exec", "tty-1"}, 80: {70, "bash -c", "tty-1"},
		90: {1, "python3 pty.py", "tty-9"}, 95: {90, "zsh", "tty-9"}, 99: {1, "launchd-child", ""}}
}

type helmBed struct {
	t          *testing.T
	root, inst string
	owners     intentOwners
}

func newHelmBed(t *testing.T, invoker int64, enrolled bool) *helmBed {
	t.Helper()
	root := t.TempDir()
	inst := filepath.Join(root, "metasystem")
	adapters := filepath.Join(inst, "scripts", "agents", "adapters")
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, "development"), adapters} {
		helmMust(t, os.MkdirAll(dir, 0o755))
	}
	helmMust(t, os.WriteFile(filepath.Join(root, "development", "metasystem-design.md"), []byte("x\n"), 0o644),
		os.WriteFile(filepath.Join(inst, "metasystem.conf"), []byte(""), 0o644),
		testexec.WriteFile(filepath.Join(adapters, "codex.sh"), []byte("#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match codex-agent'\n"), 0o755))
	if enrolled {
		_, err := humanauthority.Enroll(inst, 20, person(), "Wido", helmNow)
		helmMust(t, err)
	}
	owners := intentOwners{resolver: stateroot.NewResolver(fakeTop(root), noExecutable),
		helm: helmOwners{reader: person(), pid: func() int64 { return invoker }, now: func() time.Time { return helmNow },
			machine: func(string) (string, error) { return "m1e", nil }, account: func() string { return "wido" }, zone: time.FixedZone("CEST", 2*3600)}}
	return &helmBed{t: t, root: root, inst: inst, owners: owners}
}

func helmMust(t *testing.T, errs ...error) {
	t.Helper()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func (b *helmBed) run(args ...string) (int, string) {
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		b.t.Fatalf("no public command %q", args)
	}
	var out bytes.Buffer
	code := runIntentIn(command, rest, &out, &out, b.root, b.owners)
	return code, out.String()
}

func (b *helmBed) signature() (helm.Record, bool) {
	data, err := os.ReadFile(filepath.Join(b.root, ".git", "metasystem", "helm.json"))
	record, _ := helm.Decode(data)
	return record, err == nil
}

func (b *helmBed) log() string {
	data, _ := os.ReadFile(filepath.Join(b.root, ".git", "metasystem", "helm.log"))
	return string(data)
}

func (b *helmBed) wantTake(args []string, code int, enrollment, by string) helm.Record {
	b.t.Helper()
	got, out := b.run(append([]string{"helm", "take", "--reason", "by hand"}, args...)...)
	record, present := b.signature()
	if got != code || present != (code == 0) || (present && (record.Enrollment != enrollment || record.By != by)) {
		b.t.Fatalf("take %v: exit %d (want %d), signature %v %+v, output:\n%s", args, got, code, present, record, out)
	}
	return record
}

func TestHelmTakeRefusesAgentAncestry(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) { newHelmBed(t, 80, true).wantTake(nil, 3, "", "") })
}

func TestHelmTakeRefusesNoTerminal(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) { newHelmBed(t, 99, true).wantTake(nil, 3, "", "") })
}

func TestHelmTakeAtOtherTerminalAttributes(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) {
		record := newHelmBed(t, 60, true).wantTake(nil, 0, "other-terminal", "wido")
		if record.EnrolledAs != "Wido" || record.Leader != "sshd" || record.LeaderRef != "50@500" || record.Machine != "m1e" {
			t.Fatalf("other-terminal take recorded %+v", record)
		}
	})
}

func TestHelmTakeWithUnreadableEnrollment(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		helmMust(t, os.WriteFile(filepath.Join(b.inst, "artifacts", "agents", "authority", "human-terminal.json"), []byte("{"), 0o644))
		if record := b.wantTake(nil, 0, "unreadable", "wido"); record.EnrolledAs != "" {
			t.Fatalf("unreadable enrollment named %q", record.EnrolledAs)
		}
	})
}

func TestHelmTakeRecordsEnrolledAs(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		if record := b.wantTake(nil, 0, "proven", "Wido"); record.Leader != "login" {
			t.Fatalf("proven take recorded %+v", record)
		}
		if record := b.wantTake([]string{"--by", "Ann"}, 0, "proven", "Ann"); record.EnrolledAs != "Wido" {
			t.Fatalf("typed --by lost the enrolled name: %+v", record)
		}
	})
}

// The boundary of HM6-01 (Q-A, accepted): a pseudo-terminal whose session
// leader is python3 under launchd is accepted, and the signature names it.
func TestHelmTakeDetachedLeaderIsAcceptedAndNamed(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) {
		b := newHelmBed(t, 95, true)
		if record := b.wantTake(nil, 0, "other-terminal", "wido"); record.Leader != "python3" {
			t.Fatalf("detached take recorded leader %q", record.Leader)
		}
		if _, out := b.run("helm", "take", "--reason", "by hand"); !strings.Contains(out, "session leader python3 (90@900)") {
			t.Fatalf("the terminal line does not name the leader:\n%s", out)
		}
	})
}

func TestHelmTakeIsIdempotent(t *testing.T) {
	t.Parallel()
	t.Run("HM-2", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		first := b.wantTake(nil, 0, "proven", "Wido")
		before, _ := os.ReadFile(filepath.Join(b.root, ".git", "metasystem", "helm.json"))
		b.owners.helm.now = func() time.Time { return helmNow.Add(time.Hour) }
		if code, out := b.run("helm", "take", "--reason", "by hand"); code != 0 || !strings.HasPrefix(out, "applied: HUMAN AT THE HELM since 21:14 CEST (2026-09-28) by Wido") {
			t.Fatalf("repeat take: %d %s", code, out)
		}
		if after, _ := os.ReadFile(filepath.Join(b.root, ".git", "metasystem", "helm.json")); !bytes.Equal(before, after) || strings.Count(b.log(), "\n") != 1 {
			t.Fatalf("a repeat rewrote the signature or logged twice:\n%s", b.log())
		}
		b.run("helm", "take", "--reason", "a new reason")
		if record, _ := b.signature(); record.Reason != "a new reason" || record.At != first.At {
			t.Fatalf("a new reason did not keep since: %+v", record)
		}
		b.wantTake([]string{"--by", "Ann"}, 0, "proven", "Ann")
		if log := b.log(); !strings.Contains(log, `"by":"Ann"`) || !strings.Contains(log, `"replaced":"Wido"`) {
			t.Fatalf("a second person's take did not log both names:\n%s", log)
		}
	})
}
