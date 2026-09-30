package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type landingOwnerOrdinaryFixture struct {
	root     string
	pass     func() error
	release  func() error
	cadence  *batchowner.BatchOwnerCadence
	machine  string
	expected []string
	calls    int
}

func newLandingOwnerOrdinaryFixture(t *testing.T, expected ...string) *landingOwnerOrdinaryFixture {
	t.Helper()
	fixture := newLandingOwnerOrdinaryCadenceFixture(t, expected...)
	originalTick := batchowner.BatchOwnerCadenceTick
	tickDone := make(chan struct{}, 1)
	batchowner.BatchOwnerCadenceTick = func(root string, held batchowner.BatchOwnerLease, _ func() time.Time) error {
		defer func() { tickDone <- struct{}{} }()
		if root != fixture.root || held.Root != fixture.root || held.Pid != int64(os.Getpid()) ||
			held.Session != fmt.Sprintf("landing-owner-%d", os.Getpid()) || held.Started <= 0 ||
			held.Epoch <= 0 || !held.Announced {
			t.Errorf("cadence received root %q and lease %+v, want held lease for %q", root, held, fixture.root)
			return nil
		}
		if err := held.Require(); err != nil {
			t.Errorf("cadence lease is no longer held: %v", err)
			return nil
		}
		holder, err := lease.CurrentHolder(root)
		if err != nil || holder.Pid != held.Pid || holder.ClaimEpoch != held.Epoch || holder.SessionId != held.Session {
			t.Errorf("cadence holder=%+v error=%v, want pid=%d epoch=%d session=%q", holder, err, held.Pid, held.Epoch, held.Session)
		}
		return nil
	}
	actualPass := fixture.pass
	fixture.pass = func() error {
		if err := actualPass(); err != nil {
			return err
		}
		<-tickDone
		return nil
	}
	t.Cleanup(func() {
		if err := fixture.release(); err != nil {
			t.Errorf("release landing-owner fixture: %v", err)
		}
		batchowner.BatchOwnerCadenceTick = originalTick
	})
	return fixture
}

func newLandingOwnerOrdinaryCadenceFixture(t *testing.T, expected ...string) *landingOwnerOrdinaryFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	originalLineage, hadLineage := os.LookupEnv("METASYSTEM_OWNER_LINEAGE")
	t.Cleanup(func() {
		if hadLineage {
			_ = os.Setenv("METASYSTEM_OWNER_LINEAGE", originalLineage)
		} else {
			_ = os.Unsetenv("METASYSTEM_OWNER_LINEAGE")
		}
	})
	conf := "metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n" +
		"landing.batch-root=" + root + "\nlanding.batch-max-wait=45m\n"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := &landingOwnerOrdinaryFixture{root: root, expected: expected, cadence: batchowner.NewBatchOwnerCadence()}
	var ok bool
	fixture.release, fixture.pass, ok = setupLandingOwnerWithInputs(t.Output(), root, root, fixture.cadence, fixture.resolve)
	if !ok {
		t.Fatal("landing owner setup stopped the component")
	}
	t.Cleanup(func() {
		if fixture.calls != len(fixture.expected) {
			t.Errorf("landing-owner input resolutions=%d, want %d (%v)", fixture.calls, len(fixture.expected), fixture.expected)
		}
	})
	return fixture
}

func (fixture *landingOwnerOrdinaryFixture) enroll(machine string) {
	fixture.machine = machine
}

func (fixture *landingOwnerOrdinaryFixture) resolve(root string) (batchowner.ProductionBatchOwnerInputs, error) {
	fixture.calls++
	if root != fixture.root {
		return batchowner.ProductionBatchOwnerInputs{}, fmt.Errorf("landing-owner input root %q, want canonical root %q", root, fixture.root)
	}
	if fixture.calls > len(fixture.expected) {
		return batchowner.ProductionBatchOwnerInputs{}, fmt.Errorf("unexpected landing-owner input resolution %d", fixture.calls)
	}
	if want := fixture.expected[fixture.calls-1]; fixture.machine != want {
		return batchowner.ProductionBatchOwnerInputs{}, fmt.Errorf("landing-owner enrollment on resolution %d is %q, want %q", fixture.calls, fixture.machine, want)
	}
	if fixture.machine == "" {
		return batchowner.ProductionBatchOwnerInputs{}, fmt.Errorf("this machine has no name yet; name it once\nrun: git config metasystem.goal.machine <name>")
	}
	lockDir, queueDir, err := batchowner.BatchOwnerFixtureProofLockDirectories(root)
	if err != nil {
		return batchowner.ProductionBatchOwnerInputs{}, err
	}
	owner := &batchowner.LedgerTrunkRedOwner{
		Endpoint: goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: landingOwnerUnexpectedRepository{}},
		Actor:    goal.Actor{Machine: fixture.machine, Lineage: batchowner.LandingOwnerLineage},
		Now:      time.Now,
	}
	return batchowner.ProductionBatchOwnerInputs{LedgerOwner: owner, Machine: fixture.machine, LockDir: lockDir, QueueDir: queueDir}, nil
}

// A component pass without a batch never reads or writes the goal ledger.
type landingOwnerUnexpectedRepository struct{}

func (landingOwnerUnexpectedRepository) Capture(string) (string, error) {
	return "", fmt.Errorf("unexpected landing-owner ledger capture")
}
func (landingOwnerUnexpectedRepository) Accepted() (string, bool, error) {
	return "", false, fmt.Errorf("unexpected landing-owner ledger accepted read")
}
func (landingOwnerUnexpectedRepository) Files(string, ...string) (map[string][]byte, error) {
	return nil, fmt.Errorf("unexpected landing-owner ledger file read")
}
func (landingOwnerUnexpectedRepository) Build(string, string, []goal.Change, string) (string, error) {
	return "", fmt.Errorf("unexpected landing-owner ledger build")
}
func (landingOwnerUnexpectedRepository) Publish(string, string) (goal.CASOutcome, error) {
	return "", fmt.Errorf("unexpected landing-owner ledger publish")
}
func (landingOwnerUnexpectedRepository) AcceptedCAS(string, string) error {
	return fmt.Errorf("unexpected landing-owner ledger accepted update")
}
func (landingOwnerUnexpectedRepository) IsAncestor(string, string) (bool, error) {
	return false, fmt.Errorf("unexpected landing-owner ledger ancestry read")
}
func (landingOwnerUnexpectedRepository) TrailerPresent(string, string) (bool, error) {
	return false, fmt.Errorf("unexpected landing-owner ledger trailer read")
}
func (landingOwnerUnexpectedRepository) CommitWithTrailer(string, string, string) (string, error) {
	return "", fmt.Errorf("unexpected landing-owner ledger commit")
}
func (landingOwnerUnexpectedRepository) CommitTime(string) (time.Time, error) {
	return time.Time{}, fmt.Errorf("unexpected landing-owner ledger commit time")
}
func (landingOwnerUnexpectedRepository) Release(string) error {
	return fmt.Errorf("unexpected landing-owner ledger release")
}
