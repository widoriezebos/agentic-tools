package diskstore

// The one release of a handed-out workspace (3.6), whoever asks: the verb,
// a landing's release set, the sweeper at the owner's end. A copy's work
// is judged whole (tracked, untracked and ignored files), and every tip it
// holds (branch, worktree HEAD, reflog) is archived and read back before
// anything is removed, whatever state the record is in.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// WorkspaceReleaseRequest releases one workspace record.
type WorkspaceReleaseRequest struct {
	Registry Registry
	GitRoot  string
	ID       string
	Git      WorkspaceGit
	// Census is a use census already taken; TakeCensus, when set, takes
	// one inside the store's critical section instead. Neither is "not
	// taken".
	Census     *UseCensus
	TakeCensus func() *UseCensus
	// Discard is a person's discard of a copy's uncommitted work, valid
	// for this invocation only.
	Discard *Discard
	By      string
	Now     time.Time
	// LandedTip, set by a landing's release set, is the tip every tip of
	// the workspace must lie in at release time (Round B3-2 R8).
	LandedTip string
}

// WorkspaceRelease is a release's outcome: Done, Kept (its content keeps
// it) or Pending (a held lock, a live user, an incomplete census, a step
// that failed; retried), with the reason and the command that settles it.
type WorkspaceRelease struct {
	Done     bool     `json:"done"`
	Already  bool     `json:"already,omitempty"`
	Kept     bool     `json:"kept,omitempty"`
	Pending  bool     `json:"pending,omitempty"`
	Path     string   `json:"path,omitempty"`
	Reason   string   `json:"reason"`
	Command  string   `json:"command,omitempty"`
	Archive  string   `json:"archive,omitempty"`
	Archives []string `json:"archives,omitempty"`
	Unique   int      `json:"unique,omitempty"`
}

// ReleaseWorkspace releases one workspace inside its critical section. It
// never calls goalbranch.Sweep. A release of a released or absent
// workspace is success and writes nothing.
func ReleaseWorkspace(ctx context.Context, request WorkspaceReleaseRequest) (WorkspaceRelease, error) {
	critical, err := request.Registry.TryCritical(request.ID)
	var held *HeldError
	switch {
	case errors.Is(err, ErrNotFound):
		return WorkspaceRelease{Done: true, Already: true, Reason: "no such workspace; nothing to release"}, nil
	case errors.As(err, &held):
		return WorkspaceRelease{Pending: true, Reason: "an engine verb is inside the workspace (its record lock is held)",
			Command: "metasystem disk clean, once that verb has ended"}, nil
	case err != nil:
		return WorkspaceRelease{}, err
	}
	defer critical.Release()
	if record := critical.Record(); record.Class != WorkspaceClass {
		return WorkspaceRelease{Path: record.Path}, fmt.Errorf("store %s is a %s, not a workspace", record.ID, record.Class)
	}
	return releaseInSection(ctx, critical, request)
}

// releaseInSection is the one release sequence, whatever the record's state
// and whoever asks (Round B3-2 R7): identity revalidated; a copy that is
// still a worktree judged for its content, and every workspace for its
// users, on every attempt (a record already releasing included); for a
// landing's release, its tips checked to lie in the landed tip (R8); every
// tip archived and read back; only then releasing written; the layout
// removed; released written.
func releaseInSection(ctx context.Context, critical *Critical, request WorkspaceReleaseRequest) (WorkspaceRelease, error) {
	record := critical.Record()
	outcome := WorkspaceRelease{Path: record.Path}
	keep := func(verdict Verdict) (WorkspaceRelease, error) {
		outcome.Kept, outcome.Pending = verdict.Decision == Keep, verdict.Decision == Pending
		outcome.Reason, outcome.Command = verdict.Reason, verdict.Command
		return outcome, nil
	}
	switch record.State {
	case StateReleased:
		outcome.Done, outcome.Already, outcome.Reason = true, true, "already released"
		return outcome, nil
	case StateReserved:
		return keep(Verdict{Decision: Pending, Reason: "the workspace is still being created", Command: "metasystem disk show"})
	case StateReleasing:
		if err := revalidateReleasing(record); err != nil {
			return keep(Verdict{Decision: Pending, Reason: "a releasing workspace changed: " + err.Error() + "; a person decides", Command: "metasystem disk show"})
		}
	default:
		if err := Revalidate(record); err != nil {
			return keep(Verdict{Decision: Pending, Reason: "the workspace is not the recorded one: " + err.Error(), Command: "metasystem disk show"})
		}
	}
	discard := request.Discard != nil
	if record.Layout != LayoutCopy || workspaceTreeExists(record) {
		if verdict := judgeContent(ctx, request.Git, record, discard); verdict.Decision != Release {
			return keep(verdict)
		}
	}
	census := request.Census
	if request.TakeCensus != nil {
		census = request.TakeCensus()
	}
	if verdict := judgeUse(census, record); verdict.Decision != Release {
		return keep(verdict)
	}
	if request.LandedTip != "" {
		if verdict := judgeLanded(ctx, request.GitRoot, request.Git, record, request.LandedTip); verdict.Decision != Release {
			return keep(verdict)
		}
	}
	archives, unique, err := archiveIfCopy(ctx, critical, request.GitRoot, request.Git, request.Now)
	outcome.Archives, outcome.Unique = archives, unique
	if len(archives) > 0 {
		outcome.Archive = archives[0]
	}
	if err != nil {
		return keep(Verdict{Decision: Pending, Reason: "the tips could not be archived (" + err.Error() + "); nothing was removed", Command: "metasystem disk clean"})
	}
	record = critical.Record()
	if record.State != StateReleasing {
		record.State = StateReleasing
		if request.Discard != nil {
			// The discard is this invocation's alone: kept as history, never
			// as authority a later caller could release under.
			record.Notes = append(record.Notes, fmt.Sprintf("uncommitted content discarded by %s at %s: %s", request.Discard.By,
				request.Discard.At.UTC().Format(time.RFC3339), request.Discard.Reason))
		}
		if err := critical.Write(record); err != nil {
			return outcome, err
		}
	}
	if err := removeWorkspace(ctx, request.GitRoot, request.Git, record); err != nil {
		return keep(Verdict{Decision: Pending, Reason: "the removal stopped (" + err.Error() + "); the next attempt judges and finishes it", Command: "metasystem disk clean"})
	}
	record = critical.Record()
	record.State, record.ReleasedBy = StateReleased, request.By
	if err := critical.Write(record); err != nil {
		return outcome, err
	}
	outcome.Done, outcome.Reason = true, "released"
	return outcome, nil
}

// workspaceTreeExists reports a copy whose worktree is still there to judge.
func workspaceTreeExists(record Record) bool {
	_, err := os.Lstat(filepath.Join(record.Path, ".git"))
	return err == nil
}

// judgeLanded checks, for a landing's release, that the copy's branch tip
// and worktree HEAD both lie in the landed tip; work beyond it keeps the
// workspace out of the set.
func judgeLanded(ctx context.Context, gitRoot string, git WorkspaceGit, record Record, tip string) Verdict {
	name := filepath.Base(record.Path)
	if record.Layout != LayoutCopy {
		return Verdict{Decision: Keep, Reason: "a plain workspace is never released by a landing", Command: "metasystem work workspace " + record.Owner.Ref + " --release --name " + name}
	}
	var commits []string
	if sha, found := revParseIn(ctx, git, gitRoot, "refs/heads/"+WorkspaceBranch(record.Owner, name)); found {
		commits = append(commits, sha)
	}
	if workspaceTreeExists(record) {
		if sha, found := revParseIn(ctx, git, record.Path, "HEAD"); found {
			commits = append(commits, sha)
		}
	}
	for _, commit := range commits {
		if _, err := git(ctx, gitRoot, "merge-base", "--is-ancestor", commit, tip); err != nil {
			return Verdict{Decision: Keep, Reason: "it holds work beyond the landed tip " + shortSHA(tip) + "; dropped from the landing's release set",
				Command: "metasystem work land " + record.Owner.Ref + ", or metasystem work workspace " + record.Owner.Ref + " --release --name " + name}
		}
	}
	return Verdict{Decision: Release}
}

// archiveIfCopy archives every tip of a copy and records the archive in
// the record's notes; it runs before any removal on every attempt.
func archiveIfCopy(ctx context.Context, critical *Critical, gitRoot string, git WorkspaceGit, now time.Time) ([]string, int, error) {
	record := critical.Record()
	if record.Layout != LayoutCopy {
		return nil, 0, nil
	}
	archives, unique, err := archiveWorkspace(ctx, gitRoot, git, record, now)
	if err != nil {
		return nil, 0, err
	}
	if len(archives) > 0 {
		note := fmt.Sprintf("archived %s: %s (%d commit(s) nothing else contains)", now.UTC().Format(time.RFC3339), strings.Join(archives, ", "), unique)
		if !containsString(record.Notes, note) {
			record.Notes = append(record.Notes, note)
			if err := critical.Write(record); err != nil {
				return archives, unique, err
			}
		}
	}
	return archives, unique, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// judgeUse is the checkout-use proof from a census: a live user, or no
// complete census, is pending and retried.
func judgeUse(census *UseCensus, record Record) Verdict {
	switch {
	case census == nil || !census.Taken:
		return Verdict{Decision: Pending, Reason: "use census not taken", Command: "metasystem disk clean --release " + record.ID}
	case !census.Complete():
		return Verdict{Decision: Pending, Reason: "use census incomplete: " + joinLines(census.GapLines()), Command: "metasystem disk clean --release " + record.ID}
	}
	for _, path := range []string{record.Path, WorkspaceTmp(record)} {
		if holders := census.Holders(path); len(holders) != 0 {
			holder := holders[0]
			return Verdict{Decision: Pending, Reason: fmt.Sprintf("in use by pid %d (uid %d, %s)", holder.Pid, holder.UID, holder.Command),
				Command: fmt.Sprintf("metasystem disk clean once pid %d has ended", holder.Pid)}
		}
	}
	return Verdict{Decision: Release, Reason: "unused"}
}

// judgeContent is a copy's content rule (Round B3-3): the copy is
// released only when `git status --porcelain=v1 -z --ignored=matching
// --untracked-files=all` prints nothing. Any entry (modified, untracked or
// ignored, of any size) or any failure to read keeps it. A person's
// discard for this invocation is the only way past.
func judgeContent(ctx context.Context, git WorkspaceGit, record Record, discard bool) Verdict {
	if record.Layout != LayoutCopy || discard {
		return Verdict{Decision: Release}
	}
	name := filepath.Base(record.Path)
	land := "metasystem work land " + record.Owner.Ref + " for the work, or "
	if record.Owner.Kind != OwnerGoal {
		land = "commit the work, or "
	}
	status, err := git(ctx, record.Path, "status", "--porcelain=v1", "-z", "--ignored=matching", "--untracked-files=all")
	if err != nil {
		return Verdict{Decision: Keep, Reason: "its status cannot be read (" + err.Error() + "); it is kept", Command: land + discardCommand(record.Owner, name)}
	}
	var entries []string
	for _, entry := range strings.Split(string(status), "\x00") {
		if len(entry) > 3 {
			entries = append(entries, entry)
		}
	}
	if len(status) > 0 {
		if len(entries) == 0 {
			entries = []string{strings.TrimSpace(string(status))}
		}
		return Verdict{Decision: Keep, Reason: fmt.Sprintf("the workspace holds %d uncommitted, untracked or ignored entries: %s", len(entries), firstPaths(entries)),
			Command: land + discardCommand(record.Owner, name)}
	}
	return Verdict{Decision: Release}
}

func firstPaths(paths []string) string {
	if len(paths) > 3 {
		return strings.Join(paths[:3], ", ") + fmt.Sprintf(" and %d more", len(paths)-3)
	}
	return strings.Join(paths, ", ")
}

func discardCommand(owner Owner, name string) string {
	if owner.Kind == OwnerGoal {
		return "a person's metasystem work workspace " + owner.Ref + " --release --discard --name " + name + " --reason TEXT"
	}
	return "a person's metasystem work workspace --release --discard"
}

// archiveWorkspace writes every tip of a copy to an archive ref in the
// common store and reads each back: the branch tip, the worktree's own
// HEAD when it lies outside the branch, and every reflog entry of either
// that lies outside what is archived already. An archive ref already
// holding the tip is success; one holding another commit is never
// overwritten (a new name is used). It counts the commits nothing else
// contains.
func archiveWorkspace(ctx context.Context, gitRoot string, git WorkspaceGit, record Record, now time.Time) ([]string, int, error) {
	name := filepath.Base(record.Path)
	branch := WorkspaceBranch(record.Owner, name)
	base := WorkspaceArchiveRef(record.Owner, name)
	type tip struct{ ref, sha string }
	var tips []tip
	if sha, found := revParseIn(ctx, git, gitRoot, "refs/heads/"+branch); found {
		tips = append(tips, tip{base, sha})
	}
	worktree := false
	if _, err := os.Lstat(filepath.Join(record.Path, ".git")); err == nil {
		worktree = true
		if sha, found := revParseIn(ctx, git, record.Path, "HEAD"); found {
			tips = append(tips, tip{base + "-head", sha})
		}
	}
	var reflogs []string
	if worktree {
		if out, err := git(ctx, record.Path, "log", "-g", "--format=%H", "HEAD"); err == nil {
			reflogs = append(reflogs, strings.Fields(string(out))...)
		}
	}
	if out, err := git(ctx, gitRoot, "log", "-g", "--format=%H", "refs/heads/"+branch); err == nil {
		reflogs = append(reflogs, strings.Fields(string(out))...)
	}
	for _, sha := range reflogs {
		tips = append(tips, tip{base + "-reflog-" + shortSHA(sha), sha})
	}
	var archived []string
	var archivedSHAs []string
	for _, candidate := range tips {
		contained := false
		for _, sha := range archivedSHAs {
			if candidate.sha == sha {
				contained = true
				break
			}
			if _, err := git(ctx, gitRoot, "merge-base", "--is-ancestor", candidate.sha, sha); err == nil {
				contained = true
				break
			}
		}
		if contained {
			continue
		}
		ref, err := archiveRef(ctx, gitRoot, git, candidate.ref, candidate.sha, now)
		if err != nil {
			return archived, 0, err
		}
		archived = append(archived, ref)
		archivedSHAs = append(archivedSHAs, candidate.sha)
	}
	if len(archivedSHAs) == 0 {
		return nil, 0, nil
	}
	args := append(append([]string{"rev-list", "--count"}, archivedSHAs...), "--not", "HEAD")
	if record.Owner.Kind == OwnerGoal {
		for _, ref := range []string{"refs/heads/goal/" + record.Owner.Ref, "refs/remotes/origin/goal/" + record.Owner.Ref} {
			if _, ok := revParseIn(ctx, git, gitRoot, ref); ok {
				args = append(args, ref)
			}
		}
	}
	counted, err := git(ctx, gitRoot, args...)
	if err != nil {
		return archived, 0, err
	}
	unique, err := strconv.Atoi(strings.TrimSpace(string(counted)))
	if err != nil {
		return archived, 0, fmt.Errorf("rev-list count %q", counted)
	}
	return archived, unique, nil
}

// archiveRef points ref at sha without overwriting: a ref already at sha
// is success; a ref at another commit leaves it and uses ref@<stamp>, then
// ref@<stamp>-2 and on. The ref is created only where none exists, and
// read back.
func archiveRef(ctx context.Context, gitRoot string, git WorkspaceGit, ref, sha string, now time.Time) (string, error) {
	stamp := now.UTC().Format("20060102T150405.000000000Z")
	for attempt := 0; attempt < 100; attempt++ {
		name := ref
		if attempt == 1 {
			name = ref + "@" + stamp
		} else if attempt > 1 {
			name = fmt.Sprintf("%s@%s-%d", ref, stamp, attempt)
		}
		existing, found := revParseIn(ctx, git, gitRoot, name)
		if found && existing == sha {
			return name, nil
		}
		if found {
			continue
		}
		if _, err := git(ctx, gitRoot, "update-ref", name, sha, ""); err != nil {
			return "", err
		}
		if readBack, ok := revParseIn(ctx, git, gitRoot, name); !ok || readBack != sha {
			return "", fmt.Errorf("%s does not read back as %s", name, sha)
		}
		return name, nil
	}
	return "", fmt.Errorf("no free archive name for %s", ref)
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// removeWorkspace removes the layout at the recorded path: a plain tree
// (its marker last), or a copy's linked worktree and then its branch, and
// the temporary directory either way. A copy whose content was judged
// (or discarded by a person) is removed with --force, which ignored files
// and a discard need.
func removeWorkspace(ctx context.Context, gitRoot string, git WorkspaceGit, record Record) error {
	if record.Layout == LayoutCopy {
		if _, err := os.Lstat(filepath.Join(record.Path, ".git")); err == nil {
			if _, err := git(ctx, gitRoot, "worktree", "remove", "--force", record.Path); err != nil {
				return err
			}
		}
		branch := WorkspaceBranch(record.Owner, filepath.Base(record.Path))
		if _, found := revParseIn(ctx, git, gitRoot, "refs/heads/"+branch); found {
			if _, err := git(ctx, gitRoot, "branch", "-D", branch); err != nil {
				return err
			}
		}
	} else if err := RemoveStore(ctx, record); err != nil {
		return err
	}
	return removeTmp(ctx, record)
}

// WorkspaceProof is the sweeper's proof for a handed-out workspace (3.1's
// workspace row): its owner ended by its own row, a copy's content allows
// it, then the checkout-use proof (RegisteredStores runs it) and, inside
// the store's critical section, the same archive and removal as the
// release. A store of its owner kind that is not a workspace (a goal or
// session worktree) is pending: its release is another owner's.
type WorkspaceProof struct {
	GitRoot string
	Git     WorkspaceGit
	// Ended reports whether the owner has ended, whether that is known, and
	// what it was read from (the ledger tip), for the release record.
	Ended func(Owner) (ended, known bool, basis string)
	Now   time.Time
}

func (WorkspaceProof) Kind() OwnerKind { return OwnerGoal }

// Observe reads only: the owner's state and a copy's content.
func (p WorkspaceProof) Observe(ctx context.Context, record Record) Verdict {
	name := filepath.Base(record.Path)
	if record.Class != WorkspaceClass {
		return Verdict{Decision: Pending, Reason: "a " + record.Class + " store of " + string(record.Owner.Kind) + " " + record.Owner.Ref + " is released by its own owner, not this proof",
			Command: "metasystem disk show"}
	}
	ended, known, basis := false, false, ""
	if p.Ended != nil {
		ended, known, basis = p.Ended(record.Owner)
	}
	switch {
	case !known:
		return Verdict{Decision: Pending, Reason: "whether " + string(record.Owner.Kind) + " " + record.Owner.Ref + " has ended cannot be read", Command: "metasystem goal show " + record.Owner.Ref}
	case !ended:
		command := "metasystem work workspace " + record.Owner.Ref + " --release --name " + name
		if record.Owner.Kind != OwnerGoal {
			command = "metasystem disk show"
		}
		return Verdict{Decision: Keep, Reason: "its " + string(record.Owner.Kind) + " is open; it ends with --release, the landing of its work, or the goal's conclusion", Command: command}
	}
	if verdict := judgeContent(ctx, p.Git, record, false); verdict.Decision != Release {
		return verdict
	}
	return Verdict{Decision: Release, Reason: string(record.Owner.Kind) + " " + record.Owner.Ref + " has ended (" + basis + ")"}
}

// Apply is never reached: RegisteredStores hands a workspace to Release,
// which runs the release's own sequence. It refuses, so a caller that
// skipped Release never removes anything.
func (p WorkspaceProof) Apply(context.Context, *Critical) error {
	return errors.New("a workspace is released through its own release sequence")
}

// Release is the sweeper's release of a workspace whose owner ended: the
// release's own sequence inside the critical section RegisteredStores
// holds, with the pass's census, and a note naming what the owner's end
// was read from.
func (p WorkspaceProof) Release(ctx context.Context, critical *Critical, census *UseCensus) Verdict {
	record := critical.Record()
	if p.Ended != nil {
		if _, _, basis := p.Ended(record.Owner); basis != "" {
			note := "released at the owner's end, read from " + basis
			if !containsString(record.Notes, note) {
				record.Notes = append(record.Notes, note)
				if err := critical.Write(record); err != nil {
					return Verdict{Decision: Pending, Reason: err.Error(), Command: "metasystem disk show"}
				}
			}
		}
	}
	outcome, err := releaseInSection(ctx, critical, WorkspaceReleaseRequest{GitRoot: p.GitRoot, Git: p.Git, Census: census, By: "sweeper", Now: p.Now})
	switch {
	case err != nil:
		return Verdict{Decision: Pending, Reason: err.Error(), Command: "metasystem disk show"}
	case outcome.Done:
		return Verdict{Decision: Release, Reason: "owner ended; released"}
	case outcome.Kept:
		return Verdict{Decision: Keep, Reason: outcome.Reason, Command: outcome.Command}
	}
	return Verdict{Decision: Pending, Reason: outcome.Reason, Command: outcome.Command}
}
