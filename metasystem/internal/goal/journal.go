package goal

// The transaction journal: the machine-local crash record every
// ledger mutation writes BEFORE acting. Three phases are the entire vocabulary — created,
// pushed, terminal — and recovery is ONE rule for every
// non-terminal entry: refetch, evaluate the opid postcondition, and
// when that is absent let the OWNER'S LIVENESS decide. A
// dead owner's entry is COMPLETED from its stored intent, never
// killed; only the live owner abandons its own never-pushed
// work or expires its own retry loop. Terminals are beliefs, the
// opid is the truth: a pre-rewind push can land after the
// entry terminalized, and any later sighting of the opid in
// canonical history corrects the entry to confirmed-late.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// Phase is a journal entry's lifecycle position. These three names
// are the ONLY journal vocabulary anywhere in the design.
type Phase string

const (
	PhaseCreated  Phase = "created"
	PhasePushed   Phase = "pushed"
	PhaseTerminal Phase = "terminal"
)

// Outcome is a terminal entry's classification, with the evidence.
type Outcome string

const (
	OutcomeConfirmed     Outcome = "confirmed"
	OutcomeConfirmedLate Outcome = "confirmed-late"
	OutcomeLost          Outcome = "lost"
	OutcomeAbandoned     Outcome = "abandoned"
	OutcomeRejected      Outcome = "rejected"
	OutcomeExpired       Outcome = "expired"
)

// FieldDelta is one edit's before/after on one target's field —
// stored so a recovering process can rebuild the edit exactly.
type FieldDelta struct {
	Target string `json:"target"`
	Field  string `json:"field"`
	Old    string `json:"old"`
	New    string `json:"new"`
}

// Intent is the COMPLETE NORMALIZED COMMAND INTENT: verb,
// targets, and every argument — reason, conclusion, keep count, arc
// changes, edit deltas — enough to rebuild the operation without
// the original process.
type Intent struct {
	Verb    string            `json:"verb"`
	Targets []string          `json:"targets,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Deltas  []FieldDelta      `json:"deltas,omitempty"`
}

// OwnerIdentity names the process that owns a non-terminal entry,
// in the shipped clock-step-immune shape: the ticks+bootId pair
// decides where both sides carry it, the start second otherwise.
type OwnerIdentity struct {
	Pid          int64  `json:"pid"`
	StartTicks   int64  `json:"startTicks,omitempty"`
	BootID       string `json:"bootId,omitempty"`
	PidStartedAt int64  `json:"pidStartedAt"`
}

// Entry is one journaled transaction.
type Entry struct {
	Opid    string        `json:"opid"`
	Machine string        `json:"machine"`
	Lineage string        `json:"lineage"`
	Owner   OwnerIdentity `json:"owner"`
	Intent  Intent        `json:"intent"`
	Phase   Phase         `json:"phase"`

	// Step fields, filled as the transaction advances.
	FetchedOid string `json:"fetchedOid,omitempty"`
	TxnCommit  string `json:"txnCommit,omitempty"`

	// The pushed phase's durable fields.
	ExpectedOldTip string `json:"expectedOldTip,omitempty"`
	Attempts       int    `json:"attempts,omitempty"`
	Deadline       string `json:"deadline,omitempty"` // RFC3339

	// The terminal phase's durable fields.
	Outcome  Outcome `json:"outcome,omitempty"`
	Evidence string  `json:"evidence,omitempty"`

	CreatedAt  string `json:"createdAt"`
	TerminalAt string `json:"terminalAt,omitempty"`
}

func journalDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "goal-transactions")
}
func journalLockPath(repoRoot string) string {
	return filepath.Join(journalDir(repoRoot), "journal.flock")
}
func entryPath(repoRoot, opid string) string {
	return filepath.Join(journalDir(repoRoot), opid+".json")
}

// JournalLock serializes every journal transition on this clone.
// Callers hold it across read-decide-write.
type JournalLock struct{ f *os.File }

func AcquireJournalLock(repoRoot string) (*JournalLock, error) {
	if err := os.MkdirAll(journalDir(repoRoot), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(journalLockPath(repoRoot), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &JournalLock{f: f}, nil
}

func (l *JournalLock) Release() {
	if l.f != nil {
		_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
		l.f.Close()
		l.f = nil
	}
}

// SelfOwner captures the calling process's identity for a new entry.
func SelfOwner() (OwnerIdentity, error) {
	self, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return OwnerIdentity{}, fmt.Errorf("the journal cannot read its own process identity")
	}
	return OwnerIdentity{
		Pid: int64(os.Getpid()), StartTicks: self.StartTicks,
		BootID: self.BootID, PidStartedAt: self.StartedAt.Unix(),
	}, nil
}

// OwnerAlive proves the entry's owner by the strongest identity the
// record carries; an identityless record proves nothing and reads
// as alive — doubt never authorizes a takeover.
func OwnerAlive(e Entry) bool {
	if e.Owner.Pid <= 0 {
		return true
	}
	live, state, err := identity.KernelProber{}.Probe(e.Owner.Pid)
	if err != nil || state == identity.Unknown {
		return true
	}
	if state != identity.Alive {
		return false
	}
	if e.Owner.StartTicks > 0 && e.Owner.BootID != "" && live.StartTicks > 0 && live.BootID != "" {
		return live.StartTicks == e.Owner.StartTicks && live.BootID == e.Owner.BootID
	}
	if e.Owner.PidStartedAt > 0 {
		return live.StartedAt.Unix() == e.Owner.PidStartedAt
	}
	return true
}

// callerIsOwner: the running process is the recorded owner.
func callerIsOwner(e Entry) bool {
	if e.Owner.Pid != int64(os.Getpid()) {
		return false
	}
	self, state, err := identity.KernelProber{}.Probe(e.Owner.Pid)
	if err != nil || state != identity.Alive {
		return false
	}
	if e.Owner.StartTicks > 0 && e.Owner.BootID != "" && self.StartTicks > 0 && self.BootID != "" {
		return self.StartTicks == e.Owner.StartTicks && self.BootID == e.Owner.BootID
	}
	return self.StartedAt.Unix() == e.Owner.PidStartedAt
}

// guardTouch: a process other than the owner touches
// an entry only when the owner is provably dead.
func guardTouch(e Entry) error {
	if callerIsOwner(e) {
		return nil
	}
	if !OwnerAlive(e) {
		return nil
	}
	return fmt.Errorf("journal entry %s belongs to live process %d; only its owner may advance it", e.Opid, e.Owner.Pid)
}

func writeEntry(repoRoot string, e Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	path := entryPath(repoRoot, e.Opid)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadEntry loads one entry by opid.
func ReadEntry(repoRoot, opid string) (Entry, error) {
	data, err := os.ReadFile(entryPath(repoRoot, opid))
	if err != nil {
		return Entry{}, fmt.Errorf("no journal entry %s: %w", opid, err)
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, fmt.Errorf("journal entry %s malformed: %w", opid, err)
	}
	return e, nil
}

// CreateEntry journals the intent durably BEFORE any action. The
// caller becomes the owner; a duplicate opid refuses.
func CreateEntry(repoRoot string, opid, machine, lineage string, intent Intent) (Entry, error) {
	if opid == "" || intent.Verb == "" {
		return Entry{}, fmt.Errorf("a journal entry needs an opid and a verb")
	}
	owner, err := SelfOwner()
	if err != nil {
		return Entry{}, err
	}
	lock, err := AcquireJournalLock(repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer lock.Release()
	if _, err := os.Stat(entryPath(repoRoot, opid)); err == nil {
		return Entry{}, fmt.Errorf("journal entry %s already exists", opid)
	}
	e := Entry{
		Opid: opid, Machine: machine, Lineage: lineage, Owner: owner,
		Intent: intent, Phase: PhaseCreated,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeEntry(repoRoot, e); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// RecordSteps fills the step oids as the transaction advances;
// owner-only while the owner lives, monotonic (non-terminal only).
func RecordSteps(repoRoot, opid, fetchedOid, txnCommit string) error {
	lock, err := AcquireJournalLock(repoRoot)
	if err != nil {
		return err
	}
	defer lock.Release()
	e, err := ReadEntry(repoRoot, opid)
	if err != nil {
		return err
	}
	if err := guardTouch(e); err != nil {
		return err
	}
	if e.Phase == PhaseTerminal {
		return fmt.Errorf("journal entry %s is terminal; steps no longer change", opid)
	}
	if fetchedOid != "" {
		e.FetchedOid = fetchedOid
	}
	if txnCommit != "" {
		e.TxnCommit = txnCommit
	}
	return writeEntry(repoRoot, e)
}

// MarkPushed advances created → pushed, durably BEFORE the push
// leaves the process: expected old tip, attempt count, and the
// deadline stamp land first. Monotonic — pushed and terminal
// entries refuse.
func MarkPushed(repoRoot, opid, expectedOldTip string, attempts int, deadline time.Time) error {
	lock, err := AcquireJournalLock(repoRoot)
	if err != nil {
		return err
	}
	defer lock.Release()
	e, err := ReadEntry(repoRoot, opid)
	if err != nil {
		return err
	}
	if err := guardTouch(e); err != nil {
		return err
	}
	if e.Phase != PhaseCreated && e.Phase != PhasePushed {
		return fmt.Errorf("journal entry %s is %s; the machine is monotonic (created → pushed → terminal)", opid, e.Phase)
	}
	e.Phase = PhasePushed
	e.ExpectedOldTip = expectedOldTip
	e.Attempts = attempts
	e.Deadline = deadline.UTC().Format(time.RFC3339)
	return writeEntry(repoRoot, e)
}

// MarkTerminal closes the entry with its outcome and evidence, from
// created or pushed. Never backward.
func MarkTerminal(repoRoot, opid string, outcome Outcome, evidence string) error {
	lock, err := AcquireJournalLock(repoRoot)
	if err != nil {
		return err
	}
	defer lock.Release()
	e, err := ReadEntry(repoRoot, opid)
	if err != nil {
		return err
	}
	if err := guardTouch(e); err != nil {
		return err
	}
	if e.Phase == PhaseTerminal {
		return fmt.Errorf("journal entry %s is already terminal (%s)", opid, e.Outcome)
	}
	e.Phase = PhaseTerminal
	e.Outcome = outcome
	e.Evidence = evidence
	e.TerminalAt = time.Now().UTC().Format(time.RFC3339)
	return writeEntry(repoRoot, e)
}

// CorrectLate is the ONE lawful terminal correction: a
// pre-rewind push landed after the entry terminalized, so the opid
// now sits in canonical history. Confirmed entries stand; every
// other terminal corrects to confirmed-late with the new evidence.
// Liveness does not gate this — the evidence is canonical history,
// not a competing process's claim.
func CorrectLate(repoRoot, opid, evidence string) error {
	lock, err := AcquireJournalLock(repoRoot)
	if err != nil {
		return err
	}
	defer lock.Release()
	e, err := ReadEntry(repoRoot, opid)
	if err != nil {
		return err
	}
	if e.Phase != PhaseTerminal {
		return fmt.Errorf("journal entry %s is %s, not terminal; the ordinary recovery rule applies", opid, e.Phase)
	}
	if e.Outcome == OutcomeConfirmed || e.Outcome == OutcomeConfirmedLate {
		return nil
	}
	e.Outcome = OutcomeConfirmedLate
	e.Evidence = evidence
	e.TerminalAt = time.Now().UTC().Format(time.RFC3339)
	return writeEntry(repoRoot, e)
}

// TakeOver reassigns a provably-dead owner's non-terminal entry to
// the calling process, under the lock — the first step of
// recovery-completes.
func TakeOver(repoRoot, opid string) (Entry, error) {
	lock, err := AcquireJournalLock(repoRoot)
	if err != nil {
		return Entry{}, err
	}
	defer lock.Release()
	e, err := ReadEntry(repoRoot, opid)
	if err != nil {
		return Entry{}, err
	}
	if e.Phase == PhaseTerminal {
		return Entry{}, fmt.Errorf("journal entry %s is terminal; there is nothing to take over", opid)
	}
	if callerIsOwner(e) {
		return e, nil
	}
	if OwnerAlive(e) {
		return Entry{}, fmt.Errorf("journal entry %s belongs to live process %d; a live owner is never displaced", opid, e.Owner.Pid)
	}
	owner, err := SelfOwner()
	if err != nil {
		return Entry{}, err
	}
	e.Owner = owner
	if err := writeEntry(repoRoot, e); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// Entries lists the whole journal. An unreadable store fails the
// read — a hidden pushed entry would unblock mutations it must
// block.
func Entries(repoRoot string) ([]Entry, error) {
	entries, err := os.ReadDir(journalDir(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		e, err := ReadEntry(repoRoot, entry.Name()[:len(entry.Name())-len(".json")])
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// PushedBlocking reports the pushed entry that blocks this clone's
// own mutations (process-independently: any process on the clone
// sees the same durable file). Created and terminal entries never
// block.
func PushedBlocking(repoRoot string) (Entry, bool, error) {
	all, err := Entries(repoRoot)
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range all {
		if e.Phase == PhasePushed {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// Postcondition is what the refetch found for one entry's opid.
type Postcondition string

const (
	// PostconditionPresent: the opid's History line (or commit
	// trailer) is in canonical history — the operation landed.
	PostconditionPresent Postcondition = "present"
	// PostconditionCompetitor: a same-target competitor's opid landed
	// instead — the race was lost.
	PostconditionCompetitor Postcondition = "competitor"
	// PostconditionAbsent: neither — the outcome rides on liveness.
	PostconditionAbsent Postcondition = "absent"
)

// RecoveryAction is what the one recovery rule says to do next.
type RecoveryAction string

const (
	ActionConfirm      RecoveryAction = "confirm"        // terminalize confirmed
	ActionConfirmLate  RecoveryAction = "confirm-late"   // correct a terminalized entry
	ActionLost         RecoveryAction = "lost"           // terminalize lost, name the winner
	ActionComplete     RecoveryAction = "complete"       // dead owner: finish from stored intent
	ActionAbandonOwn   RecoveryAction = "abandon-own"    // live owner retires its never-pushed work
	ActionExpireOwn    RecoveryAction = "expire-own"     // live owner's own deadline passed
	ActionLeaveToOwner RecoveryAction = "leave-to-owner" // a live owner's entry is never touched
	ActionKeepRetrying RecoveryAction = "keep-retrying"  // live owner, inside its deadline
	ActionNothingToDo  RecoveryAction = "nothing"        // already correctly terminal
)

// ClassifyRecovery is the ONE rule, as a pure function over the
// evidence: the opid postcondition decides first; when it
// is absent the owner's liveness decides — a dead owner's entry is
// completed from its stored intent, created and pushed alike; only
// the live owner abandons its own never-pushed work or expires its
// own retry loop at the deadline.
func ClassifyRecovery(e Entry, post Postcondition, ownerAlive, callerOwns, pastDeadline bool) RecoveryAction {
	if e.Phase == PhaseTerminal {
		if post == PostconditionPresent &&
			e.Outcome != OutcomeConfirmed && e.Outcome != OutcomeConfirmedLate {
			return ActionConfirmLate
		}
		return ActionNothingToDo
	}
	switch post {
	case PostconditionPresent:
		return ActionConfirm
	case PostconditionCompetitor:
		return ActionLost
	}
	// Absent: liveness decides.
	if !ownerAlive {
		return ActionComplete
	}
	if !callerOwns {
		return ActionLeaveToOwner
	}
	if e.Phase == PhaseCreated {
		return ActionAbandonOwn
	}
	if pastDeadline {
		return ActionExpireOwn
	}
	return ActionKeepRetrying
}

// PastDeadline reads the pushed entry's own deadline stamp.
func PastDeadline(e Entry, now time.Time) bool {
	if e.Deadline == "" {
		return false
	}
	d, err := time.Parse(time.RFC3339, e.Deadline)
	if err != nil {
		return true
	}
	return now.After(d)
}
