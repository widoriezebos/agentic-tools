package stoptransition

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestProofReservationBeforeChildPublicationIsStoppable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.cap-max=120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	child := exec.Command("sleep", "60")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = child.Process.Kill()
			<-waited
		}
	})
	launcher, err := proofrun.ProcessIdentityForPID(int64(child.Process.Pid), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2,
		Identity: proofIdentity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	family := newProofRunFamily(root, 10, &suiteStopGroups{groups: map[int64]bool{}})
	items, err := family.Inventory()
	if err != nil || len(items) != 1 || !strings.Contains(items[0].Key, attempt.AttemptID) {
		t.Fatalf("reserved launcher was absent from stop inventory: items=%+v err=%v", items, err)
	}
	outcome, err := (&Transition{Families: []Family{family}}).stopItem(items[0])
	if err != nil || !outcome.Complete {
		t.Fatalf("reserved launcher did not stop through proofrun identity handling: outcome=%+v err=%v", outcome, err)
	}
	<-waited
	finished = true
	stopped, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stopped.Terminal == nil || stopped.Terminal.Result != proofrun.TerminalCancelled ||
		stopped.CancellationIntent != "checkout stop transition" {
		t.Fatalf("pre-publication reservation did not reach durable cancellation: attempt=%+v err=%v", stopped, err)
	}
}
