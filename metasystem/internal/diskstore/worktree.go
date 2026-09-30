package diskstore

// Goal and session worktrees (design engine-owns-disk-lifetimes Part B,
// 3.1's goal and session rows, 3.2 "Goal" and "Seat", U5e). Each is a linked
// worktree of the checkout's common store, registered in the checkout
// registry with no file inside the tree: its identity is its recorded path,
// the gitdir its .git file names and that file's inode. Its release follows
// the workspace rules as landed (Rounds B3 to B3-4): the status printing
// nothing with ignored files counted, no submodule, no skip-worktree or
// assume-unchanged entry, readable reflogs, every tip (per-worktree refs
// included) archived and read back, a clear use census and a pre-removal
// walk that can read and write everything, the git directory included; all
// inside the store's critical section, judged again on every attempt.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// GoalWorktreeClass is a goal's worktree <checkout>-<goal> on goal/<id>.
	GoalWorktreeClass = "goal worktree"
	// SessionWorktreeClass is a second session's worktree on session/<name>.
	SessionWorktreeClass = "session worktree"
)

// FindLinkedWorktrees lists the unreleased records of class owned by owner.
// An unreadable registry or record is an error: a caller holds rather than
// act on part of the list (fail-closed rule 1).
func FindLinkedWorktrees(registry Registry, class string, owner Owner) ([]Record, error) {
	records, unreadable := registry.Inventory()
	for _, bad := range unreadable {
		return nil, fmt.Errorf("store registry holds an unreadable record %s: %s", bad.Path, bad.Reason)
	}
	var found []Record
	for _, record := range records {
		if record.Class == class && record.Owner == owner && record.State != StateReleased {
			found = append(found, record)
		}
	}
	return found, nil
}

// ReserveLinkedWorktree records a linked worktree before git creates it at
// path (3.1: registered before it receives bytes); a repeat for the same
// path and owner returns the record. AcceptLinkedWorktree or
// IdentifyLinkedWorktree completes it once git has made it.
func ReserveLinkedWorktree(registry Registry, path, class string, owner Owner, checkout, bootstrap string, now time.Time, entropy io.Reader) (Record, error) {
	return registry.Register(Registration{Path: path, Git: true, Class: class, Owner: owner, Checkout: checkout,
		Lifetime: LifetimeOwner, CapKind: CapNone, Layout: LayoutCopy, Bootstrap: bootstrap}, now, entropy)
}

// AcceptLinkedWorktree records the worktree's identity from its .git file
// and marks the record accepted; an accepted record is success.
func AcceptLinkedWorktree(registry Registry, id string) (Record, error) {
	record, err := registry.Load(id)
	if err != nil {
		return Record{}, err
	}
	if record.State == StateAccepted {
		return record, nil
	}
	identity, err := ReadGitIdentity(record.Path)
	if err != nil {
		return Record{}, err
	}
	return registry.Transition(id, []State{StateReserved}, StateAccepted, func(record *Record) { record.Identity = identity })
}

// IdentifyLinkedWorktree records the identity of a worktree git has made
// while the record stays reserved (a second session, accepted only once
// its main announces itself).
func (r Registry) IdentifyLinkedWorktree(id string) (Record, error) {
	critical, err := r.lockExclusive(id)
	if err != nil {
		return Record{}, err
	}
	defer critical.Release()
	record := critical.Record()
	if record.State != StateReserved && record.State != StateAccepted {
		return record, fmt.Errorf("store %s is %s; its identity is settled", id, record.State)
	}
	identity, err := ReadGitIdentity(record.Path)
	if err != nil {
		return Record{}, err
	}
	if record.Identity == identity {
		return record, nil
	}
	record.Identity = identity
	return record, critical.Write(record)
}

// AbandonLinkedWorktree settles a reservation whose creation failed with
// nothing at its path: the record becomes history, and nothing is removed.
// A path that exists keeps the record reserved (a person decides).
func AbandonLinkedWorktree(registry Registry, id, why string) error {
	record, err := registry.Load(id)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(record.Path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the reserved worktree %s exists; its record stays reserved", record.Path)
	}
	_, err = registry.Transition(id, []State{StateReserved}, StateReleased, func(record *Record) {
		record.ReleasedBy = "owner"
		record.Notes = append(record.Notes, "creation failed, nothing was made: "+why)
	})
	return err
}

// lockExclusive takes a record's lock exclusively, waiting (an owner's own
// act), and reloads the record.
func (r Registry) lockExclusive(id string) (*Critical, error) {
	file, err := os.OpenFile(r.LockPath(id), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := flockRetry(file, unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	r.acquired(file)
	record, err := r.Load(id)
	if err != nil {
		_ = unlockAndClose(file)
		return nil, err
	}
	return &Critical{registry: r, file: file, record: record}, nil
}

// LinkedRelease releases a set of linked-worktree stores through one
// removal their owner runs (goal done's sweep removes every worktree of the
// goal at once): each store's critical section is held from the judgement
// through the removal.
type LinkedRelease struct {
	GitRoot string
	Git     WorkspaceGit
	// Census is a census already taken; TakeCensus takes one when needed.
	Census     *UseCensus
	TakeCensus func() *UseCensus
	Now        time.Time
	By         string
	// Remove is the owner's removal (goal done's sweep); it runs only once
	// every store is judged, archived and releasing.
	Remove func(ctx context.Context) error
}

// ReleaseLinkedWorktrees releases the given stores inside their critical
// sections, all or none: a held lock, an unproven store, content, a live
// user or a tip that cannot be archived keeps every one of them and runs no
// removal. It returns the outcome of the set.
func ReleaseLinkedWorktrees(ctx context.Context, registry Registry, ids []string, request LinkedRelease) (WorkspaceRelease, error) {
	var sections []*Critical
	defer func() {
		for _, critical := range sections {
			_ = critical.Release()
		}
	}()
	for _, id := range ids {
		critical, err := registry.TryCritical(id)
		var held *HeldError
		switch {
		case errors.As(err, &held):
			return WorkspaceRelease{Pending: true, Reason: "an engine verb is inside the worktree (its record lock is held)",
				Command: "metasystem disk clean, once that verb has ended"}, nil
		case err != nil:
			return WorkspaceRelease{}, err
		}
		sections = append(sections, critical)
	}
	return releaseLinked(ctx, sections, request)
}

// releaseLinked is the one release sequence of linked-worktree stores whose
// critical sections the caller holds.
func releaseLinked(ctx context.Context, sections []*Critical, request LinkedRelease) (WorkspaceRelease, error) {
	outcome := WorkspaceRelease{}
	keep := func(record Record, verdict Verdict) (WorkspaceRelease, error) {
		outcome.Path = record.Path
		outcome.Kept, outcome.Pending = verdict.Decision == Keep, verdict.Decision == Pending
		outcome.Reason, outcome.Command = verdict.Reason, verdict.Command
		return outcome, nil
	}
	var live []*Critical
	for _, critical := range sections {
		record := critical.Record()
		switch record.State {
		case StateReleased:
			continue
		case StateReserved:
			return keep(record, Verdict{Decision: Pending, Reason: "the worktree is still being created", Command: "metasystem disk show"})
		}
		if gone, err := worktreeGone(ctx, request, record); err != nil {
			return keep(record, Verdict{Decision: Pending, Reason: "whether the worktree still exists cannot be read: " + err.Error(), Command: "metasystem disk show"})
		} else if gone {
			// Nothing is removed: the record of a worktree already gone
			// becomes history.
			record.State, record.ReleasedBy = StateReleased, request.By
			record.Notes = append(record.Notes, "the worktree was already gone at "+request.Now.UTC().Format(time.RFC3339))
			if err := critical.Write(record); err != nil {
				return outcome, err
			}
			continue
		}
		if record.State == StateReleasing {
			if err := revalidateReleasing(record); err != nil {
				return keep(record, Verdict{Decision: Pending, Reason: "a releasing worktree changed: " + err.Error() + "; a person decides", Command: "metasystem disk show"})
			}
		} else if err := Revalidate(record); err != nil {
			return keep(record, Verdict{Decision: Pending, Reason: "the worktree is not the recorded one: " + err.Error(), Command: "metasystem disk show"})
		}
		if verdict := judgeContent(ctx, request.Git, record, false); verdict.Decision != Release {
			return keep(record, verdict)
		}
		if unreadable := unreadableIn(record.Path, record.Identity.Gitdir); unreadable != "" {
			return keep(record, Verdict{Decision: Keep, Reason: "the worktree holds what cannot be read, so its removal could stop halfway: " + unreadable,
				Command: "metasystem disk show"})
		}
		live = append(live, critical)
	}
	if len(live) == 0 {
		outcome.Done, outcome.Already, outcome.Reason = true, true, "already released"
		return outcome, nil
	}
	census := request.Census
	if census == nil && request.TakeCensus != nil {
		census = request.TakeCensus()
	}
	for _, critical := range live {
		if verdict := judgeUse(census, critical.Record()); verdict.Decision != Release {
			return keep(critical.Record(), verdict)
		}
	}
	for _, critical := range live {
		archives, unique, err := archiveIfCopy(ctx, critical, request.GitRoot, request.Git, request.Now)
		outcome.Archives, outcome.Unique = append(outcome.Archives, archives...), outcome.Unique+unique
		if err != nil {
			return keep(critical.Record(), Verdict{Decision: Pending, Reason: "the tips could not be archived (" + err.Error() + "); nothing was removed", Command: "metasystem disk clean"})
		}
	}
	if len(outcome.Archives) > 0 {
		outcome.Archive = outcome.Archives[0]
	}
	for _, critical := range live {
		record := critical.Record()
		if record.State != StateReleasing {
			record.State = StateReleasing
			if err := critical.Write(record); err != nil {
				return outcome, err
			}
		}
	}
	if request.Remove == nil {
		return keep(live[0].Record(), Verdict{Decision: Pending, Reason: "no removal is known for this worktree", Command: "metasystem disk show"})
	}
	removeErr := request.Remove(ctx)
	for _, critical := range live {
		record := critical.Record()
		if _, err := os.Lstat(filepath.Join(record.Path, ".git")); err == nil {
			// The removal left this worktree (a refusal, a cut-short run):
			// it stays the owner's, and the next attempt judges it afresh.
			record.State = StateAccepted
			note := "a release left the worktree in place"
			if removeErr != nil {
				note += ": " + removeErr.Error()
			}
			record.Notes = appendNote(record.Notes, note)
			if err := critical.Write(record); err != nil {
				return outcome, err
			}
			continue
		}
		record.State, record.ReleasedBy = StateReleased, request.By
		if err := critical.Write(record); err != nil {
			return outcome, err
		}
	}
	if removeErr != nil {
		outcome.Path = live[0].Record().Path
		outcome.Kept, outcome.Reason = true, removeErr.Error()
		return outcome, removeErr
	}
	for _, critical := range live {
		if critical.Record().State != StateReleased {
			return keep(critical.Record(), Verdict{Decision: Pending, Reason: "the removal left the worktree in place", Command: "metasystem disk clean"})
		}
	}
	outcome.Done, outcome.Reason = true, "released"
	return outcome, nil
}

// worktreeGone reports a recorded worktree whose directory is absent and
// which git no longer lists: there is nothing left to remove.
func worktreeGone(ctx context.Context, request LinkedRelease, record Record) (bool, error) {
	if _, err := os.Lstat(record.Path); !errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return worktreeUnlisted(ctx, request.Git, request.GitRoot, record.Path)
}

// worktreeUnlisted reports a path git's worktree list does not name.
func worktreeUnlisted(ctx context.Context, git WorkspaceGit, gitRoot, path string) (bool, error) {
	listed, err := worktreeListed(ctx, git, gitRoot, path)
	return !listed, err
}

// appendNote adds note once.
func appendNote(notes []string, note string) []string {
	if containsString(notes, note) {
		return notes
	}
	return append(notes, note)
}

// GoalWorktreeProof is the sweeper's proof for a goal's worktree (3.1's goal
// row, 3.2 "Goal"): the goal is concluded by the accepted ledger, the
// worktree's content keeps nothing (the workspace rules), goal done's sweep
// plan refuses nothing, and, inside the critical section, the use census is
// clear; then the tips are archived and the same sweep goal done runs
// removes the branch and the worktree. A refusal or a live user keeps it
// with the text and the command.
type GoalWorktreeProof struct {
	GitRoot string
	Git     WorkspaceGit
	// Ended reports whether the goal has ended, whether that is known, and
	// what it was read from.
	Ended func(Owner) (ended, known bool, basis string)
	// Plan is goal done's sweep plan for the goal, read only: the refusal
	// the sweep would return, "" when it would proceed.
	Plan func(ctx context.Context, goalID string) (refusal string, err error)
	// Sweep is goal done's sweep of the goal's branch and worktrees.
	Sweep func(ctx context.Context, goalID string) error
	Now   time.Time
}

func (GoalWorktreeProof) Kind() OwnerKind { return OwnerGoal }

// Observe reads only: the goal's state, the worktree's content and the
// sweep plan.
func (p GoalWorktreeProof) Observe(ctx context.Context, record Record) Verdict {
	if record.Class != GoalWorktreeClass {
		return Verdict{Decision: Pending, Reason: "a " + record.Class + " store is not a goal worktree", Command: "metasystem disk show"}
	}
	goalID := record.Owner.Ref
	if record.State == StateReserved {
		return Verdict{Decision: Pending, Reason: "goal " + goalID + "'s worktree is still being created", Command: "metasystem goal show " + goalID}
	}
	ended, known, basis := false, false, ""
	if p.Ended != nil {
		ended, known, basis = p.Ended(record.Owner)
	}
	switch {
	case !known:
		return Verdict{Decision: Pending, Reason: "whether goal " + goalID + " has ended cannot be read", Command: "metasystem goal show " + goalID}
	case !ended:
		return Verdict{Decision: Keep, Reason: "goal " + goalID + " is open; its worktree ends when the goal concludes", Command: "metasystem goal show " + goalID}
	}
	if gone, err := worktreeGone(ctx, LinkedRelease{GitRoot: p.GitRoot, Git: p.Git}, record); err != nil {
		return Verdict{Decision: Pending, Reason: "whether the worktree still exists cannot be read: " + err.Error(), Command: "metasystem disk show"}
	} else if gone {
		return Verdict{Decision: Release, Reason: "goal " + goalID + "'s worktree is already gone; its record is settled"}
	}
	if verdict := judgeContent(ctx, p.Git, record, false); verdict.Decision != Release {
		return verdict
	}
	if p.Plan == nil {
		return Verdict{Decision: Pending, Reason: "this engine cannot read goal " + goalID + "'s sweep plan", Command: "metasystem disk show"}
	}
	refusal, err := p.Plan(ctx, goalID)
	switch {
	case err != nil:
		return Verdict{Decision: Pending, Reason: "goal " + goalID + "'s sweep plan cannot be read: " + err.Error(), Command: "metasystem disk clean"}
	case refusal != "":
		return Verdict{Decision: Keep, Reason: refusal, Command: "metasystem work land " + goalID + " for the work it holds, or git -C " + record.Path + " status"}
	}
	return Verdict{Decision: Release, Reason: "goal " + goalID + " has ended (" + basis + ") and its branch holds nothing unlanded"}
}

// Apply is never reached: RegisteredStores hands the store to Release.
func (GoalWorktreeProof) Apply(context.Context, *Critical) error {
	return errors.New("a goal worktree is released through its own release sequence")
}

// Release is the sweeper's release inside the critical section
// RegisteredStores holds: the goal's sweep plan read again, then the one
// release sequence with the pass's census and goal done's sweep.
func (p GoalWorktreeProof) Release(ctx context.Context, critical *Critical, census *UseCensus) Verdict {
	record := critical.Record()
	if verdict := p.Observe(ctx, record); verdict.Decision != Release {
		return verdict
	}
	outcome, err := releaseLinked(ctx, []*Critical{critical}, LinkedRelease{GitRoot: p.GitRoot, Git: p.Git, Census: census, Now: p.Now, By: "sweeper",
		Remove: func(ctx context.Context) error { return p.Sweep(ctx, record.Owner.Ref) }})
	switch {
	case outcome.Done:
		return Verdict{Decision: Release, Reason: "goal ended; its worktree and branch released"}
	case err != nil:
		return Verdict{Decision: Keep, Reason: "goal done's sweep refused: " + err.Error(), Command: "metasystem work land " + record.Owner.Ref + ", or git -C " + record.Path + " status"}
	case outcome.Kept:
		return Verdict{Decision: Keep, Reason: outcome.Reason, Command: outcome.Command}
	}
	return Verdict{Decision: Pending, Reason: outcome.Reason, Command: outcome.Command}
}

// MainLiveness is what a second session's announcements say of its main.
type MainLiveness int

const (
	// MainNone: no main has announced itself in the session's checkout.
	MainNone MainLiveness = iota
	MainAlive
	MainDead
	// MainUnknown: an announcement cannot be read, or its process's
	// liveness cannot be proven.
	MainUnknown
)

// SessionWorktreeProof is the sweeper's proof for a second session's
// worktree (3.1's session row, 3.2 "Seat"): reserved at creation with its
// bootstrap, accepted once a main has announced itself in the worktree,
// released once every announced main is dead, the worktree's content keeps
// nothing and the use census is clear (a surviving delegate or shell in the
// worktree keeps it). A reserved record whose bootstrap died with no main
// announced is reported after the grace, never released. Every tip of
// session/<name> is archived before removal, as the workspace rules say.
type SessionWorktreeProof struct {
	GitRoot string
	Git     WorkspaceGit
	// Main reads the announced mains of the session's checkout (the
	// worktree at path).
	Main func(path string) (MainLiveness, string)
	// BootstrapDead reports whether the recorded bootstrap process has
	// ended; known false is unreadable.
	BootstrapDead func(ref string) (dead, known bool)
	// Grace is disk.session-bootstrap-hours.
	Grace time.Duration
	Now   time.Time
}

func (SessionWorktreeProof) Kind() OwnerKind { return OwnerSession }

// Observe reads only.
func (p SessionWorktreeProof) Observe(ctx context.Context, record Record) Verdict {
	if record.Class != SessionWorktreeClass {
		return Verdict{Decision: Pending, Reason: "a " + record.Class + " store is not a session worktree", Command: "metasystem disk show"}
	}
	name := record.Owner.Ref
	if gone, err := worktreeGone(ctx, LinkedRelease{GitRoot: p.GitRoot, Git: p.Git}, record); err != nil {
		return Verdict{Decision: Pending, Reason: "whether the worktree still exists cannot be read: " + err.Error(), Command: "metasystem disk show"}
	} else if gone {
		return Verdict{Decision: Release, Reason: "session " + name + "'s worktree is already gone; its record is settled"}
	}
	liveness, why := MainUnknown, "this engine cannot read the session's announcements"
	if p.Main != nil {
		liveness, why = p.Main(record.Path)
	}
	if liveness == MainNone && record.State == StateReserved {
		dead, known := false, false
		if p.BootstrapDead != nil && record.Bootstrap != "" {
			dead, known = p.BootstrapDead(record.Bootstrap)
		}
		switch {
		case !known:
			return Verdict{Decision: Pending, Reason: "session " + name + " has announced no main and its bootstrap's liveness cannot be read", Command: "metasystem disk show"}
		case dead && p.Now.Sub(record.Created) >= p.Grace:
			return Verdict{Decision: Keep, Reason: fmt.Sprintf("session %s's bootstrap ended with no main announced (reserved since %s); it is reported, never released",
				name, record.Created.UTC().Format(time.RFC3339)), Command: "git -C " + record.Path + " status, then remove the worktree yourself if nothing in it is needed"}
		}
		return Verdict{Decision: Keep, Reason: "session " + name + " is starting (no main has announced itself yet)", Command: "metasystem disk show"}
	}
	switch liveness {
	case MainAlive:
		return Verdict{Decision: Keep, Reason: "session " + name + "'s main is alive (" + why + ")", Command: "metasystem session stop --by NAME, at " + record.Path}
	case MainUnknown:
		return Verdict{Decision: Pending, Reason: "session " + name + "'s main cannot be proven ended: " + why, Command: "metasystem disk show"}
	}
	if verdict := judgeContent(ctx, p.Git, record, false); verdict.Decision != Release {
		return verdict
	}
	return Verdict{Decision: Release, Reason: "every announced main of session " + name + " has ended"}
}

// Apply is never reached: RegisteredStores hands the store to Release.
func (SessionWorktreeProof) Apply(context.Context, *Critical) error {
	return errors.New("a session worktree is released through its own release sequence")
}

// Release is the sweeper's release inside the critical section: a record
// still reserved whose main announced itself is accepted first (the session
// state machine), then the one release sequence removes exactly the
// recorded worktree and its branch, every tip archived first.
func (p SessionWorktreeProof) Release(ctx context.Context, critical *Critical, census *UseCensus) Verdict {
	record := critical.Record()
	if verdict := p.Observe(ctx, record); verdict.Decision != Release {
		return verdict
	}
	if record.State == StateReserved {
		record.State = StateAccepted
		record.Notes = appendNote(record.Notes, "a main announced itself; accepted by the sweeper")
		if err := critical.Write(record); err != nil {
			return Verdict{Decision: Pending, Reason: err.Error(), Command: "metasystem disk show"}
		}
	}
	if gone, err := worktreeGone(ctx, LinkedRelease{GitRoot: p.GitRoot, Git: p.Git}, record); err == nil && gone {
		outcome, err := releaseLinked(ctx, []*Critical{critical}, LinkedRelease{GitRoot: p.GitRoot, Git: p.Git, Census: census, Now: p.Now, By: "sweeper"})
		if err != nil || !outcome.Done {
			return Verdict{Decision: Pending, Reason: outcome.Reason, Command: "metasystem disk show"}
		}
		return Verdict{Decision: Release, Reason: "the worktree was already gone; its record is settled"}
	}
	outcome, err := releaseInSection(ctx, critical, WorkspaceReleaseRequest{GitRoot: p.GitRoot, Git: p.Git, Census: census, By: "sweeper", Now: p.Now})
	switch {
	case err != nil:
		return Verdict{Decision: Pending, Reason: err.Error(), Command: "metasystem disk show"}
	case outcome.Done:
		return Verdict{Decision: Release, Reason: "session ended; its worktree and branch released"}
	case outcome.Kept:
		return Verdict{Decision: Keep, Reason: outcome.Reason, Command: outcome.Command}
	}
	return Verdict{Decision: Pending, Reason: outcome.Reason, Command: outcome.Command}
}

// ClassProofs is one owner kind's proofs by store class: a goal owns
// handed-out workspaces and its worktree, a session its workspaces and its
// worktree, and each class has its own proof.
type ClassProofs struct {
	Owner   OwnerKind
	ByClass map[string]OwnerProof
}

func (c ClassProofs) Kind() OwnerKind { return c.Owner }

func (c ClassProofs) proof(record Record) OwnerProof { return c.ByClass[record.Class] }

// Observe is the class's proof, or pending for a class with none.
func (c ClassProofs) Observe(ctx context.Context, record Record) Verdict {
	if proof := c.proof(record); proof != nil {
		return proof.Observe(ctx, record)
	}
	return Verdict{Decision: Pending, Reason: fmt.Sprintf("this engine has no proof for a %s store of %s %s", record.Class, record.Owner.Kind, record.Owner.Ref),
		Command: "metasystem disk show"}
}

// Apply is the class's proof's Apply.
func (c ClassProofs) Apply(ctx context.Context, critical *Critical) error {
	if proof := c.proof(critical.Record()); proof != nil {
		return proof.Apply(ctx, critical)
	}
	return errors.New("no proof for the store's class")
}

// Release is the class's proof's own release sequence; a class whose proof
// has none is pending (every class ClassProofs serves has one).
func (c ClassProofs) Release(ctx context.Context, critical *Critical, census *UseCensus) Verdict {
	if releaser, ok := c.proof(critical.Record()).(SelfReleaser); ok {
		return releaser.Release(ctx, critical, census)
	}
	return Verdict{Decision: Pending, Reason: "no release sequence for a " + critical.Record().Class + " store", Command: "metasystem disk show"}
}

// BootstrapRef records a second session's bootstrap process: its pid and
// kernel start time in epoch seconds.
func BootstrapRef(pid, startedAtSec int64) string {
	return fmt.Sprintf("pid=%d;started=%d", pid, startedAtSec)
}

// ParseBootstrapRef reads BootstrapRef's form.
func ParseBootstrapRef(value string) (pid, startedAtSec int64, err error) {
	if _, err := fmt.Sscanf(value, "pid=%d;started=%d", &pid, &startedAtSec); err != nil || pid <= 0 || startedAtSec <= 0 {
		return 0, 0, fmt.Errorf("bootstrap %q is not a recorded process", value)
	}
	return pid, startedAtSec, nil
}
