package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// wireAttorneyAdmission wires the person proof's power-of-attorney seam
// once, at startup, beside the helm's.
func wireAttorneyAdmission() {
	humanauthority.AtAttorney = (&attorneyAdmitter{}).admit
}

// attorneyAdmitter answers humanauthority.AtAttorney: while a live general
// power of attorney on the accepted ledger names this machine, this
// checkout's canonical path and the lineage of the session holding its
// lease, the lease-holding main session's act is the granting person's. Any
// read, parse or clock error admits nothing.
type attorneyAdmitter struct {
	owners attorneyAdmitOwners

	mu     sync.Mutex
	logged map[string]bool
}

// attorneyAdmitOwners are the facts the admitter reads; zero values are the
// production readers.
type attorneyAdmitOwners struct {
	args     func() []string
	endpoint func(root string) (goal.Endpoint, error)
	subject  func(root string, ids []string, now time.Time) (attorneySubject, error)
	grants   func(root string) ([]goal.PowerOfAttorneyEntry, error)
	machine  func(root string) (string, error)
	classify func(root string, pid int64) (lease.Classification, error)
	holder   func(root string) (lease.CurrentHolderView, error)
	fixture  func(root string) bool
	verb     func() string
	decision func() string
	stderr   io.Writer
	// notice tells the person what the grant admitted; nil holds it on the
	// process's admission board until the act's outcome is known.
	notice     func(admissionNotice)
	appendLog  func(root, line string) error
	lockShared func(root string) error
}

func (o attorneyAdmitOwners) withDefaults() attorneyAdmitOwners {
	if o.args == nil {
		o.args = func() []string { return os.Args[1:] }
	}
	if o.endpoint == nil {
		o.endpoint = goal.ResolveEndpoint
	}
	if o.subject == nil {
		o.subject = func(root string, ids []string, now time.Time) (attorneySubject, error) {
			end, err := o.endpoint(root)
			if err != nil {
				return nil, err
			}
			return readAttorneySubject(end, ids, now)
		}
	}
	if o.grants == nil {
		o.grants = func(root string) ([]goal.PowerOfAttorneyEntry, error) {
			end, err := o.endpoint(root)
			if err != nil {
				return nil, err
			}
			_, entries, err := goal.GeneralGrantsAt(end)
			return entries, err
		}
	}
	if o.machine == nil {
		o.machine = goal.ResolveMachine
	}
	if o.classify == nil {
		o.classify = func(root string, pid int64) (lease.Classification, error) { return lease.ClassifyAt(root, root, pid) }
	}
	if o.holder == nil {
		o.holder = lease.CurrentHolder
	}
	if o.fixture == nil {
		o.fixture = fixtureauth.FixtureModeRoot
	}
	if o.verb == nil {
		o.verb = func() string { return publicVerb(os.Args[1:]) }
	}
	if o.decision == nil {
		o.decision = func() string { return os.Getenv("METASYSTEM_DECISION") }
	}
	if o.stderr == nil {
		o.stderr = os.Stderr
	}
	if o.notice == nil {
		o.notice = processAdmissionNotices.say
	}
	if o.appendLog == nil {
		o.appendLog = humanauthority.AppendAttorneyLog
	}
	if o.lockShared == nil {
		o.lockShared = processGrantLock.shared
	}
	return o
}

// acceptedAttorneyEntries reads the power-of-attorney entries of the local
// accepted ledger's root record.
func acceptedAttorneyEntries(root string) ([]goal.PowerOfAttorneyEntry, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return nil, err
	}
	_, entries, err := goal.GeneralGrantsAt(endpoint)
	return entries, err
}

// canonicalCheckout is the path a general grant binds: absolute, cleaned,
// symlinks resolved.
func canonicalCheckout(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

type attorneyDecision struct {
	entry  goal.PowerOfAttorneyEntry
	caller lease.Classification
	act    string
	live   func() (goal.PowerOfAttorneyEntry, bool)
}

func (a *attorneyAdmitter) grantFor(root string, pid int64, now time.Time, argv []string) (attorneyDecision, bool) {
	o := a.owners.withDefaults()
	if now.IsZero() {
		return attorneyDecision{}, false
	}
	checkout, err := canonicalCheckout(root)
	if err != nil || o.fixture(checkout) {
		return attorneyDecision{}, false
	}
	machine, err := o.machine(checkout)
	if err != nil || machine == "" {
		return attorneyDecision{}, false
	}
	live := func() (goal.PowerOfAttorneyEntry, bool) {
		entries, err := o.grants(checkout)
		if err != nil {
			return goal.PowerOfAttorneyEntry{}, false
		}
		var found []goal.PowerOfAttorneyEntry
		for _, entry := range entries {
			if !entry.General() || entry.For != machine || entry.Checkout != checkout {
				continue
			}
			if ok, _ := entry.LiveAt(now); ok {
				found = append(found, entry)
			}
		}
		if len(found) != 1 {
			return goal.PowerOfAttorneyEntry{}, false
		}
		return found[0], true
	}
	entry, ok := live()
	if !ok {
		return attorneyDecision{}, false
	}
	class, err := o.classify(checkout, pid)
	if err != nil || class.Class != lease.ClassMain || class.MainId == "" {
		return attorneyDecision{}, false
	}
	holder, err := o.holder(checkout)
	if err != nil || holder.MainId != class.MainId || holder.OwnerLineage != entry.Lineage {
		return attorneyDecision{}, false
	}
	act, ok := attorneyActClass(argv, func(ids []string) (attorneySubject, error) {
		return o.subject(checkout, ids, now)
	}, machine, holder.OwnerLineage)
	if !ok {
		act = ""
	}
	return attorneyDecision{entry: entry, caller: class, act: act, live: live}, true
}

func (a *attorneyAdmitter) admit(root string, pid int64, now time.Time) (humanauthority.HelmGrant, bool) {
	o := a.owners.withDefaults()
	d, ok := a.grantFor(root, pid, now, o.args())
	if !ok || !d.entry.Carries(d.act) {
		return humanauthority.HelmGrant{}, false
	}
	entry, class, checkout := d.entry, d.caller, d.entry.Checkout
	grant := humanauthority.HelmGrant{By: strings.TrimPrefix(entry.By, "human:"), Since: entry.Since,
		Class: class.Class, Checkout: entry.Checkout, Grant: entry.ID, Until: entry.Until}
	if slices.Contains(goal.DelegatedClasses, d.act) {
		decision := o.decision()
		if decision == "" || strings.IndexFunc(decision, unicode.IsSpace) >= 0 {
			return humanauthority.HelmGrant{}, false
		}
		grant.Act, grant.Decision, grant.Session = d.act, decision, class.MainId
	}
	// Only the grantee takes the grant lock, and only on the checkout the
	// grant binds; the grant is read again under it, so a local revoke that
	// returned is seen and one that starts waits for this act.
	if err := o.lockShared(checkout); err != nil {
		return humanauthority.HelmGrant{}, false
	}
	if again, ok := d.live(); !ok || again.ID != entry.ID {
		return humanauthority.HelmGrant{}, false
	}
	person := strings.TrimPrefix(entry.By, "human:")
	verb := o.verb()
	a.mu.Lock()
	defer a.mu.Unlock()
	key := entry.ID + "\x00" + verb + "\x00" + grant.Decision
	if !a.logged[key] {
		// "answered": the grant answered this act's person check; the act's
		// own checks may still refuse it, and a hard limit logs "refused".
		line := fmt.Sprintf("%s answered grant=%s by=%s act=%q class=%s main=%s pid=%d", now.UTC().Format(time.RFC3339), entry.ID, person, verb, class.Class, class.MainId, pid)
		notice := fmt.Sprintf("POWER OF ATTORNEY (%s, grant %s): %s runs as %s's act", person, entry.ID, verb, person)
		if grant.Decision != "" {
			line += " decision=" + grant.Decision + " act=" + grant.Act
			notice = fmt.Sprintf("POWER OF ATTORNEY (%s, grant %s): %s runs as the steward's own act under decision %s, class %s",
				person, entry.ID, verb, grant.Decision, grant.Act)
		}
		if err := o.appendLog(checkout, line); err != nil {
			return humanauthority.HelmGrant{}, false
		}
		o.notice(admissionNotice{w: o.stderr,
			line:   notice,
			detail: "the grant stood in for the enrolled-terminal check of this seat's main session; logged in " + attorneyLogPath(checkout)})
		if a.logged == nil {
			a.logged = map[string]bool{}
		}
		a.logged[key] = true
	}
	return grant, true
}

// attorneyLogPath is the local, append-only record of every answered act.
func attorneyLogPath(root string) string { return humanauthority.AttorneyLogPath(root) }

// grantLock serializes acts admitted under a general grant with a local
// revoke: an admitted act holds the lock shared until its process exits; a
// revoke takes it exclusive, for a bounded time, before it publishes.
type grantLock struct {
	mu   sync.Mutex
	file map[string]*os.File
}

var processGrantLock = &grantLock{}

func grantLockPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "authority", "attorney.lock")
}

func (l *grantLock) open(root string) (*os.File, error) {
	if l.file == nil {
		l.file = map[string]*os.File{}
	}
	if file := l.file[root]; file != nil {
		return file, nil
	}
	path := grantLockPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	l.file[root] = file
	return file, nil
}

// shared takes the lock shared, blocking while a revoke holds it; it stays
// held until the process exits.
func (l *grantLock) shared(root string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := l.open(root)
	if err != nil {
		return err
	}
	return unix.Flock(int(file.Fd()), unix.LOCK_SH)
}

// exclusive takes the lock exclusive, upgrading this process's own shared
// hold, trying until wait has passed. It reports whether it got the lock; a
// revoke proceeds either way, so an admitted act never blocks revocation.
func (l *grantLock) exclusive(root string, wait time.Duration, sleep func(time.Duration)) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	file := l.file[root]
	if file == nil {
		// Only a grantee's act makes the lock; with none, none can hold it.
		existing, err := os.OpenFile(grantLockPath(root), os.O_RDWR, 0)
		if os.IsNotExist(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if l.file == nil {
			l.file = map[string]*os.File{}
		}
		l.file[root] = existing
		file = existing
	}
	const step = 100 * time.Millisecond
	for waited := time.Duration(0); ; waited += step {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return true, nil
		}
		if waited >= wait {
			return false, nil
		}
		sleep(step)
	}
}
