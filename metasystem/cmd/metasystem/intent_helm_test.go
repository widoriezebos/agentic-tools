package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
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
		50: {1, "sshd wido", "tty-2"}, 60: {50, "-zsh", "tty-2"}, 70: {10, "/opt/homebrew/bin/codex exec", "tty-1"}, 80: {70, "bash -c", "tty-1"},
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
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, "development"), filepath.Join(inst, "scripts", "agents")} {
		helmMust(t, os.MkdirAll(dir, 0o755))
	}
	helmMust(t, os.WriteFile(filepath.Join(root, "development", "metasystem-design.md"), []byte("x\n"), 0o644),
		os.WriteFile(filepath.Join(inst, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644))
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
	// The agent ancestor (pid 70, "/opt/homebrew/bin/codex exec") matches the
	// built-in codex signature of the Go runtime registry, the same
	// registry-backed ProveTerminal every human verb uses; no adapter script
	// is planted.
	t.Run("HM-1", func(t *testing.T) {
		bed := newHelmBed(t, 80, true)
		bed.wantTake(nil, 3, "", "")
		// The refusal says so in the reader's terms, not the proof's code (EM-38).
		if _, out := bed.run("helm", "take", "--reason", "by hand"); !strings.Contains(out, "an agent started this shell") || strings.Contains(out, humanauthority.OutcomeAgent) {
			t.Fatalf("take under an agent ancestry must say an agent started this shell:\n%s", out)
		}
	})
}

func TestHelmTakeRefusesNoTerminal(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) { newHelmBed(t, 99, true).wantTake(nil, 3, "", "") })
}

func TestHelmTakeAtOtherTerminalAttributes(t *testing.T) {
	t.Parallel()
	t.Run("HM-1", func(t *testing.T) {
		// The take enrolls the terminal (intent_helm_enroll_test.go) under
		// the enrolled name, and records the leader it proved.
		record := newHelmBed(t, 60, true).wantTake(nil, 0, "proven", "Wido")
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
		if record := b.wantTake(nil, 0, "proven", "Wido"); record.Leader != "python3" {
			t.Fatalf("detached take recorded leader %q", record.Leader)
		}
		if _, out := b.run("helm", "take", "--reason", "by hand"); !strings.Contains(out, "session leader python3 (90@900)") {
			t.Fatalf("the terminal line does not name the leader:\n%s", out)
		}
	})
}

func TestHelmTakeIsIdempotent(t *testing.T) {
	t.Parallel()
	t.Run("HM-2", witnessHelmTakeRepeat)
}

// witnessHelmTakeRepeat is HM-2's take leg, also U-idem's helm take witness.
func witnessHelmTakeRepeat(t *testing.T) {
	{
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
	}
}

func (b *helmBed) wantReturn(code int, fragment string) string {
	b.t.Helper()
	got, out := b.run("helm", "return")
	if got != code || !strings.Contains(out, fragment) {
		b.t.Fatalf("return: exit %d (want %d), want %q in:\n%s", got, code, fragment, out)
	}
	return out
}

func TestHelmReturnIsIdempotent(t *testing.T) {
	t.Parallel()
	t.Run("HM-2", witnessHelmReturnRepeat)
}

// witnessHelmReturnRepeat is HM-2's return leg, also U-idem's helm return
// witness.
func witnessHelmReturnRepeat(t *testing.T) {
	b := newHelmBed(t, 20, true)
	b.wantTake(nil, 0, "proven", "Wido")
	b.wantReturn(0, "the machinery is at the helm again")
	b.wantReturn(0, "the machinery is at the helm; nothing to return")
	if strings.Count(b.log(), "\n") != 2 {
		t.Fatalf("a repeat return wrote a record:\n%s", b.log())
	}
}

func TestHelmReturnByAgentAllowed(t *testing.T) {
	t.Parallel()
	t.Run("HM-2", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		b.wantTake(nil, 0, "proven", "Wido")
		b.owners.helm.pid = func() int64 { return 80 }
		if b.wantReturn(0, "returned the helm Wido held"); helm.Active(b.root).Active {
			t.Fatal("an agent's return left the helm taken")
		}
	})
}

func TestReturnRemovesFirstMalformed(t *testing.T) {
	t.Parallel()
	t.Run("HM-9", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		seat, err := helm.Write(b.root, helm.Record{By: "Wido"})
		helmMust(t, err)
		if b.wantReturn(0, "returned the helm unknown held: malformed"); helm.Active(b.root).Active || !strings.Contains(b.log(), `"by":"unknown"`) {
			t.Fatalf("malformed signature at %s not returned; log:\n%s", seat.Signature, b.log())
		}
	})
}

func TestReturnRemovesFirstUnreadable(t *testing.T) {
	t.Parallel()
	t.Run("HM-9", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		seat, err := helm.Write(b.root, helm.Record{By: "Wido", At: helmNow.Format(time.RFC3339), Reason: "r"})
		helmMust(t, err, os.Chmod(seat.Signature, 0))
		if b.wantReturn(0, "returned the helm unknown held: open "); helm.Active(b.root).Active {
			t.Fatal("a mode-000 signature was not removed")
		}
		helmMust(t, os.MkdirAll(filepath.Join(seat.Signature, "blocker"), 0o700))
		b.wantReturn(1, seat.Signature+" cannot be removed")
		if b.log() != "" && strings.Count(b.log(), "\n") != 1 {
			t.Fatalf("a failed removal went on to log:\n%s", b.log())
		}
	})
}

func TestReturnWithoutGitStillReturns(t *testing.T) {
	t.Parallel()
	t.Run("HM-9", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		b.wantTake(nil, 0, "proven", "Wido")
		b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return "", errors.New(`exec: "git": executable file not found in $PATH`) }, noExecutable)
		if b.wantReturn(0, "running work: unavailable: "); helm.Active(b.root).Active {
			t.Fatal("return without git left the helm taken")
		}
	})
}

func TestReturnRemovesOnlySignature(t *testing.T) {
	t.Parallel()
	t.Run("HM-9", func(t *testing.T) {
		b := newHelmBed(t, 20, true)
		seat := helm.Seat{Yields: filepath.Join(b.root, ".git", "metasystem", "helm-yields.log")}
		b.wantTake(nil, 0, "proven", "Wido")
		helm.RecordYield(b.root, helm.Yield{At: helmNow, Boundary: "stop-hook"})
		b.wantReturn(0, "yields since the take: 1")
		for _, path := range []string{seat.Yields, filepath.Join(b.root, ".git", "metasystem", "helm.log"), filepath.Join(b.inst, "metasystem.conf")} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("return removed %s: %v", path, err)
			}
		}
	})
}

func TestHelmStatusLinesInLocalTime(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 60, true)
	b.wantTake(nil, 0, "proven", "Wido")
	inv := &intentInvocation{owners: b.owners}
	result := inv.withHelm(intentResult{Summary: "status of " + b.root, text: []string{"machinery: stopped"}, Data: map[string]any{}}, b.root)
	want := "HUMAN AT THE HELM since 21:14 CEST (2026-09-28) by Wido: by hand — metasystem helm return ends it"
	if result.Summary != want || !strings.HasPrefix(result.text[0], "the helm holder's terminal is enrolled as Wido (session leader sshd") || result.text[len(result.text)-1] != "machinery: stopped" {
		t.Fatalf("status lines: %q %q", result.Summary, result.text)
	}
}
