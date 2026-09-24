package stoptransition

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type proofInventoryStopProbe struct {
	exact identity.Exact
	state identity.Liveness
}

func (p *proofInventoryStopProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid != p.exact.Pid || p.state == identity.Dead {
		return identity.Exact{}, identity.Dead, nil
	}
	return p.exact, p.state, nil
}

func TestProofReservationBeforeChildPublicationIsStoppable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\ndispatch.cap-max=120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(root, "proof-admission"))
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full",
		"pre-publication-stop", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	child := exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :")
	child.Stdin = releaseRead
	child.ExtraFiles = []*os.File{readyWrite}
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = releaseRead.Close()
		_ = releaseWrite.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	var ready [1]byte
	if _, err := io.ReadFull(readyRead, ready[:]); err != nil || ready[0] != 'x' {
		_ = readyRead.Close()
		_ = releaseWrite.Close()
		_ = child.Process.Kill()
		_ = child.Wait()
		t.Fatalf("proof inventory helper readiness=%q err=%v", ready, err)
	}
	_ = readyRead.Close()
	t.Logf("proof inventory helper pid: %d", child.Process.Pid)
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	finished := false
	t.Cleanup(func() {
		_ = releaseWrite.Close()
		if !finished {
			_ = child.Process.Kill()
			<-waited
		}
	})
	launcher, err := proofrun.ProcessIdentityForPID(int64(child.Process.Pid), nil)
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(child.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe proof inventory helper: state=%s err=%v", state, err)
	}
	prober := &proofInventoryStopProbe{exact: exact, state: identity.Alive}
	now := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	attempt, decision, err := proofrun.ReserveLocked(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2, CandidateGoalID: "goal-a", CandidateRevision: 2, ReservedMinutes: 2,
		Identity: proofIdentity, Launcher: launcher, Now: now}, "0"))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("proof stop inventory fixture was admission-refused: %+v", decision)
	}
	family := newProofRunFamily(root, 10, &suiteStopGroups{groups: map[int64]bool{}})
	family.prober = prober
	clock := now
	family.now = func() time.Time { return clock }
	var proofStopWaits int
	family.sleep = func(duration time.Duration) {
		proofStopWaits++
		clock = clock.Add(duration)
		prober.state = identity.Dead
	}
	items, err := family.Inventory()
	if err != nil || len(items) != 1 || !strings.Contains(items[0].Key, attempt.AttemptID) {
		t.Fatalf("reserved launcher was absent from stop inventory: items=%+v err=%v", items, err)
	}
	outcome, err := (&Transition{Families: []Family{family}}).stopItem(items[0])
	if err != nil || !outcome.Complete {
		t.Fatalf("reserved launcher did not stop through proofrun identity handling: outcome=%+v err=%v", outcome, err)
	}
	if proofStopWaits != 1 {
		t.Fatalf("proof inventory stop waits = %d, want one artificial-clock wait", proofStopWaits)
	}
	<-waited
	finished = true
	stopped, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stopped.Terminal == nil || stopped.Terminal.Result != proofrun.TerminalCancelled ||
		stopped.CancellationIntent != "checkout stop transition" {
		t.Fatalf("pre-publication reservation did not reach durable cancellation: attempt=%+v err=%v", stopped, err)
	}
}
