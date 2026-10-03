package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	grants   func(root string) ([]goal.PowerOfAttorneyEntry, error)
	machine  func(root string) (string, error)
	classify func(root string, pid int64) (lease.Classification, error)
	holder   func(root string) (lease.CurrentHolderView, error)
	fixture  func(root string) bool
	verb     func() string
	impact   func() string
	stderr   io.Writer
	// notice tells the person what the grant admitted; nil holds it on the
	// process's admission board until the act's outcome is known.
	notice     func(admissionNotice)
	appendLog  func(root, line string) error
	lockShared func(root string) error
}

func (o attorneyAdmitOwners) withDefaults() attorneyAdmitOwners {
	if o.grants == nil {
		o.grants = acceptedAttorneyEntries
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
	if o.impact == nil {
		o.impact = func() string { return publicImpact(os.Args[1:]) }
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

func (a *attorneyAdmitter) admit(root string, pid int64, now time.Time) (humanauthority.HelmGrant, bool) {
	o := a.owners.withDefaults()
	if now.IsZero() {
		return humanauthority.HelmGrant{}, false
	}
	checkout, err := canonicalCheckout(root)
	if err != nil || o.fixture(checkout) {
		return humanauthority.HelmGrant{}, false
	}
	machine, err := o.machine(checkout)
	if err != nil || machine == "" {
		return humanauthority.HelmGrant{}, false
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
		return humanauthority.HelmGrant{}, false
	}
	class, err := o.classify(checkout, pid)
	if err != nil || class.Class != lease.ClassMain || class.MainId == "" {
		return humanauthority.HelmGrant{}, false
	}
	holder, err := o.holder(checkout)
	if err != nil || holder.MainId != class.MainId || holder.OwnerLineage != entry.Lineage {
		return humanauthority.HelmGrant{}, false
	}
	// Only the grantee takes the grant lock, and only on the checkout the
	// grant binds; the grant is read again under it, so a local revoke that
	// returned is seen and one that starts waits for this act.
	if err := o.lockShared(checkout); err != nil {
		return humanauthority.HelmGrant{}, false
	}
	if again, ok := live(); !ok || again.ID != entry.ID {
		return humanauthority.HelmGrant{}, false
	}
	person := strings.TrimPrefix(entry.By, "human:")
	verb := o.verb()
	impact := o.impact()
	if entry.Lineage == "project-partner" && strings.TrimSpace(impact) == "" {
		_ = o.appendLog(checkout, fmt.Sprintf("%s refused grant=%s by=%s act=%q reason=%q", now.UTC().Format(time.RFC3339), entry.ID, person, verb, "--impact is missing"))
		return humanauthority.HelmGrant{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := entry.ID + "\x00" + verb
	if !a.logged[key] {
		// "answered": the grant answered this act's person check; the act's
		// own checks may still refuse it, and a hard limit logs "refused".
		line := fmt.Sprintf("%s answered grant=%s by=%s act=%q class=%s main=%s pid=%d", now.UTC().Format(time.RFC3339), entry.ID, person, verb, class.Class, class.MainId, pid)
		if impact != "" {
			line += fmt.Sprintf(" impact=%q", impact)
		}
		if err := o.appendLog(checkout, line); err != nil {
			return humanauthority.HelmGrant{}, false
		}
		notice := fmt.Sprintf("POWER OF ATTORNEY (%s, grant %s): %s runs as %s's act", person, entry.ID, verb, person)
		if impact != "" {
			notice = "IMPACT: " + impact + "\n" + notice
		}
		o.notice(admissionNotice{w: o.stderr,
			line:   notice,
			detail: "the grant stood in for the enrolled-terminal check of this seat's main session; logged in " + attorneyLogPath(checkout)})
		if a.logged == nil {
			a.logged = map[string]bool{}
		}
		a.logged[key] = true
	}
	return humanauthority.HelmGrant{By: person, Since: entry.Since, Class: class.Class, Checkout: entry.Checkout, Grant: entry.ID, Until: entry.Until}, true
}

// publicImpact reads the shared option without interpreting the act's text.
func publicImpact(args []string) string {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--impact=") {
			return strings.TrimPrefix(arg, "--impact=")
		}
		if arg == "--impact" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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
