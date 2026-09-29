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
	"sort"
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
	// Discard is a person's authorised discard of a copy's uncommitted work.
	Discard *Discard
	By      string
	Now     time.Time
	// IgnoredReleaseBytes is disk.workspace-ignored-release-mib in bytes:
	// ignored content up to it goes with a copy; more keeps the copy.
	IgnoredReleaseBytes int64
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

// ReleaseWorkspace releases one workspace inside its critical section:
// identity revalidated, a copy's content judged, the use census read, the
// record written releasing, every tip archived and read back, the layout
// removed at the recorded path with its branch and temporary directory,
// and the record released. It never calls goalbranch.Sweep. A release of
// a released or absent workspace is success and writes nothing.
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
	record := critical.Record()
	outcome := WorkspaceRelease{Path: record.Path}
	if record.Class != WorkspaceClass {
		return outcome, fmt.Errorf("store %s is a %s, not a workspace", record.ID, record.Class)
	}
	switch record.State {
	case StateReleased:
		outcome.Done, outcome.Already, outcome.Reason = true, true, "already released"
		return outcome, nil
	case StateReserved:
		return WorkspaceRelease{Pending: true, Path: record.Path, Reason: "the workspace is still being created", Command: "repeat the request once it returned"}, nil
	case StateAccepted:
		census := request.Census
		if request.TakeCensus != nil {
			census = request.TakeCensus()
		}
		if verdict := judgeWorkspace(ctx, request.Git, record, request.IgnoredReleaseBytes, request.Discard != nil, census); verdict.Decision != Release {
			outcome.Kept, outcome.Pending = verdict.Decision == Keep, verdict.Decision == Pending
			outcome.Reason, outcome.Command = verdict.Reason, verdict.Command
			return outcome, nil
		}
		record.State = StateReleasing
		if request.Discard != nil {
			record.AuthorizedDiscard = request.Discard
		}
		if err := critical.Write(record); err != nil {
			return outcome, err
		}
	case StateReleasing:
		if err := revalidateReleasing(record); err != nil {
			return WorkspaceRelease{Pending: true, Path: record.Path, Reason: "a releasing workspace changed: " + err.Error() + "; a person decides",
				Command: "metasystem disk show"}, nil
		}
	}
	archives, unique, err := releaseWorkspaceLayout(ctx, critical, request.GitRoot, request.Git, request.Now)
	outcome.Archives, outcome.Unique = archives, unique
	if len(archives) > 0 {
		outcome.Archive = archives[0]
	}
	if err != nil {
		return WorkspaceRelease{Pending: true, Path: record.Path, Archives: archives, Reason: "the release stopped (" + err.Error() + "); the workspace is releasing and the next pass finishes it",
			Command: "metasystem disk clean"}, nil
	}
	record = critical.Record()
	record.State, record.ReleasedBy = StateReleased, request.By
	if err := critical.Write(record); err != nil {
		return outcome, err
	}
	outcome.Done, outcome.Reason = true, "released"
	return outcome, nil
}

// releaseWorkspaceLayout archives every tip of a copy and records the
// archive, then removes the layout; the sweeper's proof and the release
// share it, and it runs whatever the record's state, so a release cut
// short archives again (idempotently) before it removes anything.
func releaseWorkspaceLayout(ctx context.Context, critical *Critical, gitRoot string, git WorkspaceGit, now time.Time) ([]string, int, error) {
	record := critical.Record()
	var archives []string
	unique := 0
	if record.Layout == LayoutCopy {
		var err error
		archives, unique, err = archiveWorkspace(ctx, gitRoot, git, record, now)
		if err != nil {
			return nil, 0, fmt.Errorf("the tips could not be archived: %w", err)
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
	}
	return archives, unique, removeWorkspace(ctx, gitRoot, git, record)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// judgeWorkspace is the release's check before anything is written:
// identity; for a copy, its content (modified or untracked files keep it
// unless a person discards; ignored content up to ignoredLimit goes with
// it, more keeps it); then the use census (a live user, or no complete
// census, is pending and retried).
func judgeWorkspace(ctx context.Context, git WorkspaceGit, record Record, ignoredLimit int64, discard bool, census *UseCensus) Verdict {
	if err := Revalidate(record); err != nil {
		return Verdict{Decision: Pending, Reason: "the workspace is not the recorded one: " + err.Error(), Command: "metasystem disk show"}
	}
	if verdict := judgeContent(ctx, git, record, ignoredLimit, discard); verdict.Decision != Release {
		return verdict
	}
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
	return Verdict{Decision: Release, Reason: "clean and unused"}
}

// judgeContent reads a copy's status including ignored files.
func judgeContent(ctx context.Context, git WorkspaceGit, record Record, ignoredLimit int64, discard bool) Verdict {
	if record.Layout != LayoutCopy || discard {
		return Verdict{Decision: Release}
	}
	name := filepath.Base(record.Path)
	status, err := git(ctx, record.Path, "status", "--porcelain=v1", "--untracked-files=all", "--ignored")
	if err != nil {
		return Verdict{Decision: Pending, Reason: "git status failed: " + err.Error(), Command: "git -C " + record.Path + " status"}
	}
	type sized struct {
		path  string
		bytes int64
	}
	var dirty []string
	var ignored []sized
	var ignoredBytes int64
	for _, line := range strings.Split(strings.TrimRight(string(status), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSuffix(line[3:], "/")
		if strings.HasPrefix(line, "!! ") {
			bytes, _, _ := Measure(ctx, filepath.Join(record.Path, path))
			ignored = append(ignored, sized{path, bytes})
			ignoredBytes += bytes
			continue
		}
		dirty = append(dirty, path)
	}
	land := "metasystem work land " + record.Owner.Ref + " for the work, or "
	if record.Owner.Kind != OwnerGoal {
		land = "commit the work, or "
	}
	if len(dirty) > 0 {
		return Verdict{Decision: Keep, Reason: fmt.Sprintf("the workspace has %d uncommitted change(s): %s", len(dirty), firstPaths(dirty)),
			Command: land + discardCommand(record.Owner, name)}
	}
	if ignoredBytes > ignoredLimit {
		sort.Slice(ignored, func(i, j int) bool { return ignored[i].bytes > ignored[j].bytes })
		var largest []string
		for _, item := range ignored {
			largest = append(largest, fmt.Sprintf("%s %s", item.path, formatBytes(item.bytes)))
		}
		return Verdict{Decision: Keep, Reason: fmt.Sprintf("the workspace holds %s of ignored files, over disk.workspace-ignored-release-mib: %s",
			formatBytes(ignoredBytes), firstPaths(largest)), Command: "copy out what matters, then " + discardCommand(record.Owner, name)}
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
	// IgnoredReleaseBytes is disk.workspace-ignored-release-mib in bytes.
	IgnoredReleaseBytes int64
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
	if verdict := judgeContent(ctx, p.Git, record, p.IgnoredReleaseBytes, false); verdict.Decision != Release {
		return verdict
	}
	return Verdict{Decision: Release, Reason: string(record.Owner.Kind) + " " + record.Owner.Ref + " has ended (" + basis + ")"}
}

// Apply archives every tip of a copy, then removes the layout, inside the
// critical section RegisteredStores holds; the release record names what
// the owner's end was read from.
func (p WorkspaceProof) Apply(ctx context.Context, critical *Critical) error {
	record := critical.Record()
	if p.Ended != nil {
		if _, _, basis := p.Ended(record.Owner); basis != "" {
			note := "released at the owner's end, read from " + basis
			if !containsString(record.Notes, note) {
				record.Notes = append(record.Notes, note)
				if err := critical.Write(record); err != nil {
					return err
				}
			}
		}
	}
	_, _, err := releaseWorkspaceLayout(ctx, critical, p.GitRoot, p.Git, p.Now)
	return err
}
