package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type attorneyAdmitBed struct {
	root     string
	entries  []goal.PowerOfAttorneyEntry
	class    lease.Classification
	holder   lease.CurrentHolderView
	machine  string
	fixture  bool
	logErr   error
	lockErr  error
	readErr  error
	logLines []string
	locks    int
	stderr   bytes.Buffer
}

var attorneyAdmitNow = time.Date(2026, 9, 30, 7, 0, 0, 0, time.UTC)

func newAttorneyAdmitBed(t *testing.T) *attorneyAdmitBed {
	root := t.TempDir()
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := goal.PowerOfAttorneyEntry{ID: "01M-grant", By: "human:wido", Verbs: []string{goal.GeneralAct}, For: "m1e",
		Checkout: canonical, Lineage: "lin-main", Since: "2026-09-30T06:00:00Z", Until: "2026-10-01T06:00:00Z"}
	return &attorneyAdmitBed{root: root, entries: []goal.PowerOfAttorneyEntry{entry},
		class:   lease.Classification{Class: lease.ClassMain, MainId: "main-1"},
		holder:  lease.CurrentHolderView{MainId: "main-1", OwnerLineage: "lin-main"},
		machine: "m1e"}
}

func (b *attorneyAdmitBed) admitter() *attorneyAdmitter {
	return &attorneyAdmitter{owners: attorneyAdmitOwners{
		grants: func(string) ([]goal.PowerOfAttorneyEntry, error) { return b.entries, b.readErr },
		machine: func(string) (string, error) {
			return b.machine, nil
		},
		classify: func(string, int64) (lease.Classification, error) { return b.class, nil },
		holder:   func(string) (lease.CurrentHolderView, error) { return b.holder, nil },
		fixture:  func(string) bool { return b.fixture },
		verb:     func() string { return "goal approve g1" },
		stderr:   &b.stderr,
		appendLog: func(_ string, line string) error {
			if b.logErr != nil {
				return b.logErr
			}
			b.logLines = append(b.logLines, line)
			return nil
		},
		lockShared: func(string) error { b.locks++; return b.lockErr },
	}}
}

// W4: the admitter admits only the lease-holding main of the checkout and
// lineage the grant names, on this machine, outside fixture mode, and only
// after its one log line is written; it binds the act to the grant.
func TestAttorneyAdmitsOnlyTheHolderOfTheBoundCheckout(t *testing.T) {
	t.Parallel()
	b := newAttorneyAdmitBed(t)
	grant, ok := b.admitter().admit(b.root, 80, attorneyAdmitNow)
	if !ok || grant.Grant != "01M-grant" || grant.By != "wido" || grant.Class != lease.ClassMain || grant.Until != "2026-10-01T06:00:00Z" {
		t.Fatalf("the holder was not admitted: %+v %v", grant, ok)
	}
	if len(b.logLines) != 1 || !strings.Contains(b.logLines[0], "answered grant=01M-grant by=wido") || !strings.Contains(b.logLines[0], `act="goal approve g1"`) || !strings.Contains(b.logLines[0], "main=main-1") {
		t.Fatalf("one log line naming the grant: %q", b.logLines)
	}
	if !strings.Contains(b.stderr.String(), "POWER OF ATTORNEY (wido, grant 01M-grant): goal approve g1 runs as wido's act") {
		t.Fatalf("the stderr line: %q", b.stderr.String())
	}
	if b.locks != 1 {
		t.Fatalf("the grantee locked %d times", b.locks)
	}

	refusals := map[string]func(*attorneyAdmitBed){
		"not the holder":    func(b *attorneyAdmitBed) { b.holder.MainId = "main-2" },
		"another lineage":   func(b *attorneyAdmitBed) { b.holder.OwnerLineage = "lin-other" },
		"delegate":          func(b *attorneyAdmitBed) { b.class = lease.Classification{Class: lease.ClassDelegate} },
		"steward":           func(b *attorneyAdmitBed) { b.class = lease.Classification{Class: lease.ClassSteward} },
		"supervision":       func(b *attorneyAdmitBed) { b.class = lease.Classification{Class: lease.ClassSupervision} },
		"another checkout":  func(b *attorneyAdmitBed) { b.entries[0].Checkout = "/elsewhere" },
		"another machine":   func(b *attorneyAdmitBed) { b.machine = "m1b" },
		"fixture mode":      func(b *attorneyAdmitBed) { b.fixture = true },
		"unreadable ledger": func(b *attorneyAdmitBed) { b.readErr = errors.New("no root record") },
		"log append fails":  func(b *attorneyAdmitBed) { b.logErr = errors.New("disk full") },
		"lock fails":        func(b *attorneyAdmitBed) { b.lockErr = errors.New("no lock") },
		"expired":           func(b *attorneyAdmitBed) { b.entries[0].Until = "2026-09-30T07:00:00Z" },
		"revoked": func(b *attorneyAdmitBed) {
			b.entries[0].Revoked, b.entries[0].RevokedBy = "2026-09-30T06:30:00Z", "human:wido"
		},
		"scoped entry only": func(b *attorneyAdmitBed) {
			b.entries[0] = goal.PowerOfAttorneyEntry{ID: "s", Verbs: []string{"approve"}}
		},
		"two live for here": func(b *attorneyAdmitBed) {
			second := b.entries[0]
			second.ID = "01M-second"
			b.entries = append(b.entries, second)
		},
		"no clock":           nil,
		"relative checkout?": func(b *attorneyAdmitBed) { b.entries[0].Checkout = "seat" },
	}
	for label, mutate := range refusals {
		bed := newAttorneyAdmitBed(t)
		now := attorneyAdmitNow
		if mutate == nil {
			now = time.Time{}
		} else {
			mutate(bed)
		}
		if grant, ok := bed.admitter().admit(bed.root, 80, now); ok {
			t.Errorf("%s: admitted %+v", label, grant)
		}
		grantee := label == "log append fails" || label == "lock fails"
		if !grantee && bed.locks != 0 {
			t.Errorf("%s: a caller the grant does not name took the grant lock", label)
		}
		if bed.stderr.Len() != 0 || (label != "log append fails" && len(bed.logLines) != 0) {
			t.Errorf("%s: a refusal left a trace: %q %v", label, bed.stderr.String(), bed.logLines)
		}
	}

	// One process is one act: a second proof logs no second line.
	again := b.admitter()
	if _, ok := again.admit(b.root, 80, attorneyAdmitNow); !ok {
		t.Fatal("second admitter refused")
	}
	if _, ok := again.admit(b.root, 80, attorneyAdmitNow); !ok || len(b.logLines) != 2 {
		t.Fatalf("a repeat proof in one process logged again: %q", b.logLines)
	}
}

// A revoke on a checkout no grantee act ever locked neither waits nor makes
// the lock file; one a grantee holds shared waits, bounded, and says so.
func TestGrantLockExclusiveTouchesOnlyALockAGranteeMade(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lock := &grantLock{}
	held, err := lock.exclusive(root, time.Second, func(time.Duration) { t.Fatal("waited on a lock no one made") })
	if err != nil || !held {
		t.Fatalf("no lock file = %v %v", held, err)
	}
	if _, err := os.Stat(grantLockPath(root)); !os.IsNotExist(err) {
		t.Fatalf("a revoke made the lock file: %v", err)
	}
	grantee := &grantLock{}
	if err := grantee.shared(root); err != nil {
		t.Fatal(err)
	}
	waits := 0
	held, err = (&grantLock{}).exclusive(root, 250*time.Millisecond, func(time.Duration) { waits++ })
	if err != nil || held || waits == 0 {
		t.Fatalf("a grantee's hold = %v %v after %d waits", held, err, waits)
	}
}
