package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type landingOwnerOrdinaryFixture struct {
	root     string
	pass     func() error
	release  func() error
	cadence  *batchOwnerCadence
	machine  string
	expected []string
	calls    int
}

func newLandingOwnerOrdinaryFixture(t *testing.T, expected ...string) *landingOwnerOrdinaryFixture {
	t.Helper()
	fixture := newLandingOwnerOrdinaryCadenceFixture(t, expected...)
	originalTick := batchOwnerCadenceTick
	tickDone := make(chan struct{}, 1)
	batchOwnerCadenceTick = func(root string, held batchOwnerLease, _ func() time.Time) error {
		defer func() { tickDone <- struct{}{} }()
		if root != fixture.root || held.root != fixture.root || held.pid != int64(os.Getpid()) ||
			held.session != fmt.Sprintf("landing-owner-%d", os.Getpid()) || held.started <= 0 ||
			held.epoch <= 0 || !held.announced {
			t.Errorf("cadence received root %q and lease %+v, want held lease for %q", root, held, fixture.root)
			return nil
		}
		if err := held.require(); err != nil {
			t.Errorf("cadence lease is no longer held: %v", err)
			return nil
		}
		holder, err := lease.CurrentHolder(root)
		if err != nil || holder.Pid != held.pid || holder.ClaimEpoch != held.epoch || holder.SessionId != held.session {
			t.Errorf("cadence holder=%+v error=%v, want pid=%d epoch=%d session=%q", holder, err, held.pid, held.epoch, held.session)
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
		batchOwnerCadenceTick = originalTick
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
	fixture := &landingOwnerOrdinaryFixture{root: root, expected: expected, cadence: newBatchOwnerCadence()}
	var ok bool
	fixture.release, fixture.pass, ok = setupLandingOwnerWithInputs(root, root, fixture.cadence, fixture.resolve)
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

func (fixture *landingOwnerOrdinaryFixture) resolve(root string) (productionBatchOwnerInputs, error) {
	fixture.calls++
	if root != fixture.root {
		return productionBatchOwnerInputs{}, fmt.Errorf("landing-owner input root %q, want canonical root %q", root, fixture.root)
	}
	if fixture.calls > len(fixture.expected) {
		return productionBatchOwnerInputs{}, fmt.Errorf("unexpected landing-owner input resolution %d", fixture.calls)
	}
	if want := fixture.expected[fixture.calls-1]; fixture.machine != want {
		return productionBatchOwnerInputs{}, fmt.Errorf("landing-owner enrollment on resolution %d is %q, want %q", fixture.calls, fixture.machine, want)
	}
	if fixture.machine == "" {
		return productionBatchOwnerInputs{}, fmt.Errorf("no machine nickname is enrolled and hostnames are never published: run  git config metasystem.goal.machine <nickname>  once on this machine")
	}
	lockDir, queueDir, err := batchOwnerFixtureProofLockDirectories(root)
	if err != nil {
		return productionBatchOwnerInputs{}, err
	}
	owner := &ledgerTrunkRedOwner{
		endpoint: goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: landingOwnerUnexpectedRepository{}},
		actor:    goal.Actor{Machine: fixture.machine, Lineage: landingOwnerLineage},
		now:      time.Now,
	}
	return productionBatchOwnerInputs{ledgerOwner: owner, machine: fixture.machine, lockDir: lockDir, queueDir: queueDir}, nil
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
