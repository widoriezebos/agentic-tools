package launch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type reservationGit struct{}

func (reservationGit) Run(dir string, _ []string, args ...string) ([]byte, error) {
	if strings.Join(args, " ") != "rev-parse --show-toplevel" {
		return nil, errors.New("unexpected Git call")
	}
	return []byte(dir), nil
}

func recoveryReservation(t *testing.T, state string, live identity.Liveness) (*UnitRunner, UnitRunRecord, string) {
	t.Helper()
	m, _, probe, _ := manager(t)
	probe.states[99] = live
	r := &UnitRunner{Manager: m, Root: t.TempDir(), Git: reservationGit{},
		CriticCustody: func(UnitRunRecord, bool) (bool, error) { return false, errors.New("unreadable critic custody") }}
	id, err := newID(m.Now())
	if err != nil {
		t.Fatal(err)
	}
	record := UnitRunRecord{ID: id, State: state, Worktree: t.TempDir(), Mutation: &identity.Ref{Pid: 99, StartedAtSec: 99}}
	if err := writeUnitJSON(filepath.Join(r.runDir(record.ID), "run.json"), record, r.root()); err != nil {
		t.Fatal(err)
	}
	if err := r.reserveTree(record); err != nil {
		t.Fatal(err)
	}
	var path string
	if err := r.treeLocked(record.Worktree, func(p string, _ *treeReservation) error { path = p; return nil }); err != nil {
		t.Fatal(err)
	}
	return r, record, path
}

func TestGateTreeReleasesFinishedDeadMutationWithUnreadableCustody(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"cancelled", "completed"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			r, record, path := recoveryReservation(t, state, identity.Dead)
			if err := r.GateTree(record.Worktree, "", nil); err != nil {
				t.Fatalf("finished dead run still reserves the tree: %v", err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("reservation remains: %v", err)
			}
		})
	}
}

func TestCancelRunReleasesFinishedDeadMutationWithUnreadableCustody(t *testing.T) {
	t.Parallel()
	r, record, path := recoveryReservation(t, "cancelled", identity.Dead)
	if _, err := r.CancelRun(record.ID); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reservation remains: %v", err)
	}
}

func TestGateTreeKeepsLiveOrUnknownMutation(t *testing.T) {
	t.Parallel()
	for _, live := range []identity.Liveness{identity.Alive, identity.Unknown} {
		t.Run(live.String(), func(t *testing.T) {
			t.Parallel()
			r, record, path := recoveryReservation(t, "running", live)
			r.CriticCustody = nil
			var waiting *TreeWaitingError
			if err := r.GateTree(record.Worktree, "", nil); !errors.As(err, &waiting) {
				t.Fatalf("mutation custody was released: %v", err)
			}
			if _, err := r.CancelRun(record.ID); err == nil || !strings.Contains(err.Error(), "pid 99") || !strings.Contains(err.Error(), "start 99") {
				t.Fatalf("stop did not identify the holding process: %v", err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("reservation disappeared: %v", err)
			}
		})
	}
}

func TestGateTreeFinishedDeadMutationKeepsLiveChild(t *testing.T) {
	t.Parallel()
	r, record, path := recoveryReservation(t, "completed", identity.Dead)
	child, err := newID(r.Manager.Now())
	if err != nil {
		t.Fatal(err)
	}
	ref := identity.Ref{Pid: 10, StartedAtSec: 10}
	if err := r.Manager.Store.Create(Record{ID: child, State: Completed, Child: &ref}); err != nil {
		t.Fatal(err)
	}
	record.Rounds = []UnitRound{{Steps: []UnitStep{{LaunchID: child}}}}
	if err := writeUnitJSON(filepath.Join(r.runDir(record.ID), "run.json"), record, r.root()); err != nil {
		t.Fatal(err)
	}
	var waiting *TreeWaitingError
	if err := r.GateTree(record.Worktree, "", nil); !errors.As(err, &waiting) {
		t.Fatalf("surviving child released: %v", err)
	}
	if _, err := r.CancelRun(record.ID); err == nil || !strings.Contains(err.Error(), "pid 10, start 10") {
		t.Fatalf("stop failed to identify surviving child: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("reservation gone: %v", err)
	}
}

func TestGateTreeFinishedLiveMutationKeepsUnreadableCustody(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"cancelled", "completed"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			r, record, path := recoveryReservation(t, state, identity.Alive)
			var waiting *TreeWaitingError
			if err := r.GateTree(record.Worktree, "", nil); !errors.As(err, &waiting) {
				t.Fatalf("terminal state bypassed a surviving mutation: %v", err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("reservation disappeared: %v", err)
			}
		})
	}
}

func TestGateTreeFinishedUnknownMutationKeepsUnreadableCustody(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"cancelled", "completed"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			r, record, path := recoveryReservation(t, state, identity.Unknown)
			var waiting *TreeWaitingError
			if err := r.GateTree(record.Worktree, "", nil); !errors.As(err, &waiting) {
				t.Fatalf("terminal state bypassed unknown mutation custody: %v", err)
			}
			if _, err := r.CancelRun(record.ID); err == nil {
				t.Fatal("stop released unknown mutation custody")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("reservation disappeared: %v", err)
			}
		})
	}
}

func TestAReleasedRebaseLeavesNoOwner(t *testing.T) {
	t.Parallel()
	for _, child := range []bool{false, true} {
		t.Run(map[bool]string{false: "alone", true: "child retained"}[child], func(t *testing.T) {
			r, record, path := recoveryReservation(t, "completed", identity.Dead)
			release, err := r.ReserveMutation(record.Worktree, "goal", "rebase")
			if err != nil {
				t.Fatal(err)
			}
			if child {
				if err := r.treeLocked(record.Worktree, func(path string, owner *treeReservation) error {
					owner.Children = []string{"retained-child"}
					return writeUnitJSON(path, owner, r.root())
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := release(); err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(path)
			if child && err != nil || !child && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("reservation with child=%t: %v", child, err)
			}
			if err := r.treeLocked(record.Worktree, func(_ string, owner *treeReservation) error {
				if !child && owner.Run != "" {
					t.Fatalf("released owner remains: %+v", owner)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStopPassesAnExistingFinishedMutationOwner(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"dead", "live", "unknown", "surviving child"} {
		t.Run(mode, func(t *testing.T) {
			live := identity.Dead
			if mode == "live" {
				live = identity.Alive
			} else if mode == "unknown" {
				live = identity.Unknown
			}
			r, owner, path := recoveryReservation(t, "completed", live)
			r.CriticCustody = func(UnitRunRecord, bool) (bool, error) { return true, nil }
			if mode == "surviving child" {
				child, err := newID(r.Manager.Now())
				if err != nil {
					t.Fatal(err)
				}
				ref := identity.Ref{Pid: 10, StartedAtSec: 10}
				if err := r.Manager.Store.Create(Record{ID: child, State: Completed, Child: &ref}); err != nil {
					t.Fatal(err)
				}
				owner.Rounds = []UnitRound{{Steps: []UnitStep{{LaunchID: child}}}}
				if err := writeUnitJSON(filepath.Join(r.runDir(owner.ID), "run.json"), owner, r.root()); err != nil {
					t.Fatal(err)
				}
			}
			id, err := newID(r.Manager.Now())
			if err != nil {
				t.Fatal(err)
			}
			unit := UnitRunRecord{ID: id, Worktree: owner.Worktree, State: "awaiting-judgement"}
			if err := writeUnitJSON(filepath.Join(r.runDir(id), "run.json"), unit, r.root()); err != nil {
				t.Fatal(err)
			}
			stopped, err := r.CancelRun(id)
			if mode == "dead" {
				if err != nil || stopped.State != "cancelled" {
					t.Fatalf("finished owner blocked stop: %+v %v", stopped, err)
				}
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("reservation remains: %v", err)
				}
			} else {
				if err == nil || stopped.State == "cancelled" {
					t.Fatalf("unproven custody released: %+v %v", stopped, err)
				}
				if mode == "surviving child" && !strings.Contains(err.Error(), "pid 10, start 10") {
					t.Fatalf("surviving child not named: %v", err)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("reservation disappeared: %v", err)
				}
				if err := r.GateTree(owner.Worktree, "", nil); err == nil {
					t.Fatal("GateTree released unproven custody")
				}
			}
		})
	}
}
