package brain

// Ported from scripts/agents/brain-fixtures.sh (verb redesign U7b part 3),
// scenarios brain-declare-race and brain-second-declaration-refuses. The
// shell bed built Git ledgers and ran two engine processes; the host-pointer
// compare-and-swap it proved lives here, so the port calls Declare and
// Withdraw directly with per-test registry homes and checkouts.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const otherBedLedger = "01J5X0000000000000000OTHER"

func bedCheckout(t *testing.T) string {
	t.Helper()
	path, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// bedDeclare declares with a frozen lock clock and a yielding poll, so a
// contended declaration waits for the holder's release by scheduling rather
// than by wall time, and can never reach its bounded-wait refusal.
func bedDeclare(root, registry, ledger, machine string) error {
	frozen := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	_, err := Declare(DeclareOptions{
		StateRoot: root, RegistryHome: registry, LedgerIdentity: ledger, Machine: machine,
		DeclaredBy: "Wido", Now: frozen,
		lockClock: func() time.Time { return frozen },
		lockSleep: func(time.Duration) { runtime.Gosched() },
	})
	return err
}

func TestBrainBedDeclareRaceNamesTheWinnerAndRefusesALiveForeignLock(t *testing.T) {
	t.Parallel()
	registry := t.TempDir()
	one, two := bedCheckout(t), bedCheckout(t)
	machines := []string{"brain-one", "brain-two"}
	roots := []string{one, two}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for index := range roots {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs[index] = bedDeclare(roots[index], registry, testLedger, machines[index])
		}(index)
	}
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("brain declare race outcomes were %v, %v; want exactly one winner", errs[0], errs[1])
	}
	winner, loserErr := one, errs[1]
	if errs[0] != nil {
		winner, loserErr = two, errs[0]
	}
	if !strings.Contains(loserErr.Error(), "already has a brain") || !strings.Contains(loserErr.Error(), winner) {
		t.Fatalf("race loser did not name the winner: %v", loserErr)
	}
	pointer := PointerPath(registry, testLedger)
	data, err := os.ReadFile(pointer)
	if err != nil || strings.TrimSpace(string(data)) != winner {
		t.Fatalf("race pointer does not name winner %q: %q %v", winner, data, err)
	}
	if removed, err := Withdraw(winner, registry, testLedger); err != nil || !removed {
		t.Fatalf("winner withdraw = %v %v", removed, err)
	}

	// A live foreign holder of the host-pointer lock refuses the next
	// declaration once the bounded wait passes on the injected clock.
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("test process identity unreadable: %v %v", state, err)
	}
	lockDir := pointer + ".lock.d"
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	owner, err := json.Marshal(map[string]any{"pid": os.Getpid(), "pidStartedAt": exact.StartedAt.Unix(), "instanceTag": "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, "owner.json"), append(owner, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	sleeps := 0
	loser := one
	if winner == one {
		loser = two
	}
	_, err = Declare(DeclareOptions{
		StateRoot: loser, RegistryHome: registry, LedgerIdentity: testLedger, Machine: "brain-two",
		DeclaredBy: "Wido", Now: clock,
		lockClock: func() time.Time { return clock },
		lockSleep: func(d time.Duration) { sleeps++; clock = clock.Add(d) },
	})
	if err == nil || !strings.Contains(err.Error(), "another brain declaration is in progress") {
		t.Fatalf("a live foreign lock holder did not refuse the declaration: %v", err)
	}
	if sleeps == 0 {
		t.Fatal("the refusal did not come from the bounded wait")
	}
	if Read(loser, testLedger).State != Undeclared {
		t.Fatal("the refused declaration wrote a record")
	}
}

func TestBrainBedSecondDeclarationRefuses(t *testing.T) {
	t.Parallel()
	homeOne, homeTwo := t.TempDir(), t.TempDir()
	primary, second, other := bedCheckout(t), bedCheckout(t), bedCheckout(t)

	if err := bedDeclare(primary, homeOne, testLedger, "brain-one"); err != nil {
		t.Fatal(err)
	}
	if err := bedDeclare(primary, homeOne, testLedger, "brain-one"); err == nil || !strings.Contains(err.Error(), "already the brain") {
		t.Fatalf("a second declaration of the same checkout was not refused: %v", err)
	}
	if err := bedDeclare(second, homeOne, testLedger, "brain-two"); err == nil || !strings.Contains(err.Error(), primary) {
		t.Fatalf("a second checkout on the same host did not name the declared brain %q: %v", primary, err)
	}
	if Read(second, testLedger).State != Undeclared {
		t.Fatal("the refused second checkout wrote a record")
	}
	// Another host (registry home) and another ledger are independent.
	if err := bedDeclare(second, homeTwo, testLedger, "brain-two"); err != nil {
		t.Fatalf("another host's declaration was refused: %v", err)
	}
	if err := bedDeclare(other, homeOne, otherBedLedger, "other-brain"); err != nil {
		t.Fatalf("another ledger's declaration was refused: %v", err)
	}
	if removed, err := Withdraw(second, homeTwo, testLedger); err != nil || !removed {
		t.Fatalf("second withdraw = %v %v", removed, err)
	}
	if removed, err := Withdraw(primary, homeOne, testLedger); err != nil || !removed {
		t.Fatalf("primary withdraw = %v %v", removed, err)
	}
	if err := bedDeclare(second, homeOne, testLedger, "brain-two"); err != nil {
		t.Fatalf("the withdrawn host pointer was not replaceable: %v", err)
	}
	data, err := os.ReadFile(PointerPath(homeOne, testLedger))
	if err != nil || strings.TrimSpace(string(data)) != second {
		t.Fatalf("second checkout did not replace the withdrawn host pointer: %q %v", data, err)
	}
}
