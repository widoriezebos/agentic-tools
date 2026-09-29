package diskstore

// Workspaces the engine hands out (design engine-owns-disk-lifetimes Part B,
// 3.6): a registered directory under
// <control>/artifacts/agents/workspaces/<owner>/<name>/, owned by a goal or
// a session, in one of two layouts. A plain workspace is an empty marker
// directory; a copy is a linked worktree of the checkout's common store on
// branch workspace/<owner>/<name>, so its commits live in the common store
// and it has no object store of its own. A request is serialized on its
// reservation key; its release is its own end, separate from the goal's,
// and archives a copy's branch tip before anything is removed.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// WorkspaceClass is the class of every handed-out workspace's record.
const WorkspaceClass = "workspace"

// The two layouts of 3.6.
const (
	LayoutPlain = "plain"
	LayoutCopy  = "copy"
)

// WorkspaceGit runs git in dir, bound to ctx; tests pass their own.
type WorkspaceGit func(ctx context.Context, dir string, args ...string) ([]byte, error)

// WorkspaceOwnerSegment is the owner's path and branch segment: goal-<id>
// or session-<name>.
func WorkspaceOwnerSegment(owner Owner) string { return string(owner.Kind) + "-" + owner.Ref }

// WorkspaceBranch is a copy's branch.
func WorkspaceBranch(owner Owner, name string) string {
	return "workspace/" + WorkspaceOwnerSegment(owner) + "/" + workspaceName(name)
}

// WorkspaceArchiveRef is where a copy's branch tip is archived at release.
func WorkspaceArchiveRef(owner Owner, name string) string {
	return "refs/archive/" + WorkspaceOwnerSegment(owner) + "/" + workspaceName(name) + "/" + WorkspaceBranch(owner, name)
}

// WorkspacePath is where a workspace lives.
func WorkspacePath(control string, owner Owner, name string) string {
	return filepath.Join(control, "artifacts", "agents", "workspaces", WorkspaceOwnerSegment(owner), workspaceName(name))
}

// WorkspaceTmp is the workspace's temporary directory: inside a plain
// workspace, beside a copy (never inside its tree, which must stay clean).
func WorkspaceTmp(record Record) string {
	if record.Layout == LayoutCopy {
		return record.Path + ".tmp"
	}
	return filepath.Join(record.Path, "tmp")
}

func workspaceName(name string) string {
	if name == "" {
		return DefaultWorkspaceName
	}
	return name
}

// WorkspaceRequest asks for one workspace.
type WorkspaceRequest struct {
	Registry Registry
	// Control is the checkout's control root; GitRoot its repository.
	Control, GitRoot string
	Owner            Owner
	Name             string
	// CopyOf is the resolved commit of a copy; empty asks for a plain one.
	CopyOf   string
	CapBytes int64
	Now      time.Time
	Entropy  io.Reader
	Git      WorkspaceGit
}

// Workspace is what a request returns.
type Workspace struct {
	Record  Record
	Tmp     string
	Created bool
}

// WorkspaceConflict is a request whose name is taken by a workspace of
// another revision: an input refusal naming the two ways out.
type WorkspaceConflict struct {
	Existing Record
	Name     string
}

func (c *WorkspaceConflict) Error() string {
	what := "a plain workspace"
	if c.Existing.CopyOf != "" {
		what = "a copy of " + c.Existing.CopyOf
	}
	return fmt.Sprintf("workspace %s is already %s at %s; release it with --release --name %s, or name another with --name",
		workspaceName(c.Name), what, c.Existing.Path, workspaceName(c.Name))
}

type reservation struct {
	Record string `json:"record"`
}

// ObtainWorkspace returns the workspace a request names, creating it when
// none is accepted (3.6's repeat contract). The reservation lock is taken
// before a ulid is allocated or a byte copied; the same request again
// returns the same workspace and writes nothing; an interrupted creation
// is discarded and made anew.
func ObtainWorkspace(ctx context.Context, request WorkspaceRequest) (Workspace, error) {
	if request.Owner.Kind != OwnerGoal && request.Owner.Kind != OwnerSession {
		return Workspace{}, fmt.Errorf("a workspace belongs to a goal or a session, not %s", request.Owner.Kind)
	}
	key, err := ReservationKey(request.Owner, request.Name)
	if err != nil {
		return Workspace{}, err
	}
	entry := request.Registry.ReservationPath(key)
	if err := os.MkdirAll(filepath.Dir(entry), 0o700); err != nil {
		return Workspace{}, err
	}
	held, err := lock.File(entry+".lock", 0o600, lock.Exclusive)
	if err != nil {
		return Workspace{}, fmt.Errorf("workspace reservation lock: %w", err)
	}
	defer held.Release()
	if existing, found, err := readReservation(request.Registry, entry); err != nil {
		return Workspace{}, err
	} else if found {
		switch existing.State {
		case StateAccepted:
			if existing.CopyOf != request.CopyOf {
				return Workspace{}, &WorkspaceConflict{Existing: existing, Name: request.Name}
			}
			return Workspace{Record: existing, Tmp: WorkspaceTmp(existing)}, nil
		case StateReserved:
			if err := discardPartial(ctx, request, existing); err != nil {
				return Workspace{}, fmt.Errorf("an interrupted workspace at %s could not be discarded: %w", existing.Path, err)
			}
		case StateReleasing:
			return Workspace{}, fmt.Errorf("the earlier workspace at %s is still being released; metasystem disk clean finishes it, then repeat this request", existing.Path)
		}
	}
	return createWorkspace(ctx, request, key, entry)
}

// FindWorkspace is the unreleased workspace a goal or session holds under
// name, found through its reservation; it reads only.
func FindWorkspace(registry Registry, owner Owner, name string) (Record, bool, error) {
	key, err := ReservationKey(owner, name)
	if err != nil {
		return Record{}, false, err
	}
	return readReservation(registry, registry.ReservationPath(key))
}

func readReservation(registry Registry, entry string) (Record, bool, error) {
	data, err := os.ReadFile(entry)
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	var found reservation
	if err := json.Unmarshal(data, &found); err != nil {
		return Record{}, false, fmt.Errorf("workspace reservation %s is unreadable: %w", entry, err)
	}
	record, err := registry.Load(found.Record)
	if errors.Is(err, ErrNotFound) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	return record, record.State != StateReleased, nil
}

func createWorkspace(ctx context.Context, request WorkspaceRequest, key, entry string) (Workspace, error) {
	layout := LayoutPlain
	if request.CopyOf != "" {
		layout = LayoutCopy
	}
	path := WorkspacePath(request.Control, request.Owner, request.Name)
	record, err := request.Registry.Register(Registration{Path: path, Git: layout == LayoutCopy, Class: WorkspaceClass, Owner: request.Owner,
		Checkout: request.Control, Lifetime: LifetimeOwner, CapBytes: request.CapBytes, CapKind: CapTarget, Layout: layout,
		Reservation: key, CopyOf: request.CopyOf}, request.Now, request.Entropy)
	if err != nil {
		return Workspace{}, err
	}
	if record.State != StateReserved || record.Reservation != key {
		return Workspace{}, fmt.Errorf("store record %s already holds %s; metasystem disk show names it", record.ID, path)
	}
	data, err := json.Marshal(reservation{Record: record.ID})
	if err != nil {
		return Workspace{}, err
	}
	if _, err := atomicfile.WriteFile(entry, append(data, '\n'), 0o600, filepath.Dir(entry)); err != nil {
		return Workspace{}, err
	}
	if _, err := os.Lstat(path); err == nil {
		return Workspace{}, fmt.Errorf("%s exists but no workspace record holds it; metasystem disk show lists it", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Workspace{}, err
	}
	var mutate func(*Record)
	switch layout {
	case LayoutPlain:
		if err := os.Mkdir(path, 0o700); err != nil {
			return Workspace{}, err
		}
		if err := WriteMarker(record); err != nil {
			return Workspace{}, err
		}
	case LayoutCopy:
		branch := WorkspaceBranch(request.Owner, request.Name)
		if _, err := request.Git(ctx, request.GitRoot, "worktree", "add", "-b", branch, path, request.CopyOf); err != nil {
			return Workspace{}, fmt.Errorf("git worktree add %s: %w", path, err)
		}
		identity, err := ReadGitIdentity(path)
		if err != nil {
			return Workspace{}, err
		}
		mutate = func(record *Record) { record.Identity = identity }
	}
	if err := os.MkdirAll(WorkspaceTmp(record), 0o700); err != nil {
		return Workspace{}, err
	}
	accepted, err := request.Registry.Transition(record.ID, []State{StateReserved}, StateAccepted, mutate)
	if err != nil {
		return Workspace{}, err
	}
	return Workspace{Record: accepted, Tmp: WorkspaceTmp(accepted), Created: true}, nil
}

// discardPartial removes what an interrupted creation left at the recorded
// path, by its layout, and marks the record released. The path is the
// reservation's own; a marker naming another record refuses.
func discardPartial(ctx context.Context, request WorkspaceRequest, record Record) error {
	critical, err := request.Registry.TryCritical(record.ID)
	if err != nil {
		return err
	}
	defer critical.Release()
	switch record.Layout {
	case LayoutCopy:
		if _, err := os.Lstat(filepath.Join(record.Path, ".git")); err == nil {
			if _, err := request.Git(ctx, request.GitRoot, "worktree", "remove", "--force", record.Path); err != nil {
				return err
			}
		}
		if _, err := request.Git(ctx, request.GitRoot, "worktree", "prune"); err != nil {
			return err
		}
		_, _ = request.Git(ctx, request.GitRoot, "branch", "-D", WorkspaceBranch(request.Owner, request.Name))
		if err := RemoveTree(ctx, record.Path); err != nil {
			return err
		}
	default:
		if found, err := readMarker(record.Path); err == nil && found.ID != record.ID {
			return fmt.Errorf("%s carries the marker of record %s", record.Path, found.ID)
		}
		if err := RemoveTree(ctx, record.Path); err != nil {
			return err
		}
	}
	if err := RemoveTree(ctx, WorkspaceTmp(record)); err != nil {
		return err
	}
	record.State = StateReleased
	record.ReleasedBy = "the next request (interrupted creation)"
	return critical.Write(record)
}

// WorkspaceReleaseRequest releases one workspace record.
type WorkspaceReleaseRequest struct {
	Registry Registry
	GitRoot  string
	ID       string
	Git      WorkspaceGit
	// Census is the use census; nil is "not taken".
	Census *UseCensus
	// Discard is a person's authorised discard of a copy's uncommitted work.
	Discard *Discard
	By      string
	Now     time.Time
}

// WorkspaceRelease is a release's outcome: Done, Kept (a content check or a
// live user) or Pending (a held lock, an incomplete census), with the
// reason and the command that settles it.
type WorkspaceRelease struct {
	Done    bool   `json:"done"`
	Already bool   `json:"already,omitempty"`
	Kept    bool   `json:"kept,omitempty"`
	Pending bool   `json:"pending,omitempty"`
	Path    string `json:"path,omitempty"`
	Reason  string `json:"reason"`
	Command string `json:"command,omitempty"`
	Archive string `json:"archive,omitempty"`
	Unique  int    `json:"unique,omitempty"`
}

// ReleaseWorkspace is the one release of a workspace (3.6), whoever asks:
// the verb, a landing's release set, the sweeper at the owner's end. Inside
// the store's critical section it revalidates identity, checks a copy is
// clean (unless a person discards), archives a copy's branch tip and reads
// it back, runs the checkout-use proof, writes releasing, removes the
// layout at the recorded path and the branch, and marks the record
// released. It never calls goalbranch.Sweep. A release of a released or
// absent workspace is success and writes nothing.
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
		if kept := checkWorkspace(ctx, request, record, &outcome); kept {
			return outcome, nil
		}
		record.State = StateReleasing
		if outcome.Archive != "" {
			record.Notes = append(record.Notes, fmt.Sprintf("released %s: branch tip archived as %s (%d commit(s) nothing else contains)",
				request.Now.UTC().Format(time.RFC3339), outcome.Archive, outcome.Unique))
		}
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
	if err := removeWorkspace(ctx, request, record); err != nil {
		return WorkspaceRelease{Pending: true, Path: record.Path, Reason: "removal cut short (" + err.Error() + "); the workspace is releasing and the next pass finishes it",
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

// checkWorkspace runs the checks before anything is written: identity, a
// copy's cleanliness and archive, and the use proof. It reports true when
// outcome says why the workspace stays.
func checkWorkspace(ctx context.Context, request WorkspaceReleaseRequest, record Record, outcome *WorkspaceRelease) bool {
	keep := func(pending bool, reason, command string) bool {
		outcome.Kept, outcome.Pending, outcome.Reason, outcome.Command = !pending, pending, reason, command
		return true
	}
	if err := Revalidate(record); err != nil {
		return keep(true, "the workspace is not the recorded one: "+err.Error(), "metasystem disk show")
	}
	name := filepath.Base(record.Path)
	if record.Layout == LayoutCopy {
		status, err := request.Git(ctx, record.Path, "status", "--porcelain=v1", "--untracked-files=all")
		if err != nil {
			return keep(true, "git status failed: "+err.Error(), "git -C "+record.Path+" status")
		}
		if strings.TrimSpace(string(status)) != "" && request.Discard == nil {
			land := "metasystem work land " + record.Owner.Ref + " for the work, or "
			if record.Owner.Kind != OwnerGoal {
				land = "commit the work, or "
			}
			return keep(false, "the workspace has uncommitted changes", land+discardCommand(record.Owner, name))
		}
		archive, unique, err := archiveWorkspace(ctx, request, record, name)
		if err != nil {
			return keep(true, "the branch tip could not be archived: "+err.Error(), "git -C "+request.GitRoot+" branch --list "+WorkspaceBranch(record.Owner, name))
		}
		outcome.Archive, outcome.Unique = archive, unique
	}
	census := request.Census
	switch {
	case census == nil || !census.Taken:
		return keep(true, "use census not taken", "metasystem disk clean --release "+record.ID)
	case !census.Complete():
		return keep(true, "use census incomplete: "+joinLines(census.GapLines()), "metasystem disk clean --release "+record.ID)
	}
	for _, path := range []string{record.Path, WorkspaceTmp(record)} {
		if holders := census.Holders(path); len(holders) != 0 {
			holder := holders[0]
			return keep(false, fmt.Sprintf("in use by pid %d (uid %d, %s)", holder.Pid, holder.UID, holder.Command),
				fmt.Sprintf("metasystem work workspace %s --release --name %s once pid %d has ended", record.Owner.Ref, name, holder.Pid))
		}
	}
	return false
}

func discardCommand(owner Owner, name string) string {
	if owner.Kind == OwnerGoal {
		return "metasystem work workspace " + owner.Ref + " --release --discard --name " + name
	}
	return "metasystem disk clean --preview, then a person's metasystem work workspace --release --discard"
}

// archiveWorkspace writes the copy's branch tip to its archive ref in the
// common store and reads it back; an archive ref already holding the tip
// is success, one holding another commit leaves it and archives under a
// stamped name. It counts the commits nothing else contains.
func archiveWorkspace(ctx context.Context, request WorkspaceReleaseRequest, record Record, name string) (string, int, error) {
	branch := WorkspaceBranch(record.Owner, name)
	tip, found := revParse(ctx, request, "refs/heads/"+branch)
	if !found {
		return "", 0, nil
	}
	archive := WorkspaceArchiveRef(record.Owner, name)
	if existing, ok := revParse(ctx, request, archive); ok && existing != tip {
		archive += "@" + request.Now.UTC().Format("20060102T150405Z")
	}
	if existing, ok := revParse(ctx, request, archive); !ok || existing != tip {
		if _, err := request.Git(ctx, request.GitRoot, "update-ref", archive, tip); err != nil {
			return "", 0, err
		}
		if readBack, ok := revParse(ctx, request, archive); !ok || readBack != tip {
			return "", 0, fmt.Errorf("%s does not read back as %s", archive, tip)
		}
	}
	args := []string{"rev-list", "--count", tip, "--not", "HEAD"}
	if record.Owner.Kind == OwnerGoal {
		for _, ref := range []string{"refs/heads/goal/" + record.Owner.Ref, "refs/remotes/origin/goal/" + record.Owner.Ref} {
			if _, ok := revParse(ctx, request, ref); ok {
				args = append(args, ref)
			}
		}
	}
	counted, err := request.Git(ctx, request.GitRoot, args...)
	if err != nil {
		return "", 0, err
	}
	unique, err := strconv.Atoi(strings.TrimSpace(string(counted)))
	if err != nil {
		return "", 0, fmt.Errorf("rev-list count %q", counted)
	}
	return archive, unique, nil
}

func revParse(ctx context.Context, request WorkspaceReleaseRequest, ref string) (string, bool) {
	out, err := request.Git(ctx, request.GitRoot, "rev-parse", "--verify", "-q", ref)
	if err != nil {
		return "", false
	}
	sha := strings.TrimSpace(string(out))
	return sha, sha != ""
}

// removeWorkspace removes the layout at the recorded path: a plain tree
// (its marker last), or a copy's linked worktree and then its branch, and
// the temporary directory either way.
func removeWorkspace(ctx context.Context, request WorkspaceReleaseRequest, record Record) error {
	if record.Layout == LayoutCopy {
		if _, err := os.Lstat(filepath.Join(record.Path, ".git")); err == nil {
			args := []string{"worktree", "remove"}
			if record.AuthorizedDiscard != nil {
				args = append(args, "--force")
			}
			if _, err := request.Git(ctx, request.GitRoot, append(args, record.Path)...); err != nil {
				return err
			}
		}
		branch := WorkspaceBranch(record.Owner, filepath.Base(record.Path))
		if _, found := revParse(ctx, request, "refs/heads/"+branch); found {
			if _, err := request.Git(ctx, request.GitRoot, "branch", "-D", branch); err != nil {
				return err
			}
		}
	} else if err := RemoveStore(ctx, record); err != nil {
		return err
	}
	return RemoveTree(ctx, WorkspaceTmp(record))
}

// WorkspaceProof is the sweeper's proof for a handed-out workspace (3.1's
// workspace row): its owner ended by its own row, a copy is clean, then
// the checkout-use proof (RegisteredStores runs it) and, inside the
// store's critical section, the copy's archive and the layout's removal,
// the same steps as ReleaseWorkspace. A store of its owner kind that is not
// a workspace (a goal or session worktree) is pending: its release is
// another owner's.
type WorkspaceProof struct {
	GitRoot string
	Git     WorkspaceGit
	// Ended reports whether the owner has ended, and whether that is known.
	Ended func(Owner) (ended, known bool)
	// Now stamps an archive ref that must not overwrite another commit.
	Now time.Time
}

func (WorkspaceProof) Kind() OwnerKind { return OwnerGoal }

func (p WorkspaceProof) request() WorkspaceReleaseRequest {
	return WorkspaceReleaseRequest{GitRoot: p.GitRoot, Git: p.Git, Now: p.Now}
}

// Observe reads only: the owner's state and a copy's git status.
func (p WorkspaceProof) Observe(ctx context.Context, record Record) Verdict {
	name := filepath.Base(record.Path)
	if record.Class != WorkspaceClass {
		return Verdict{Decision: Pending, Reason: "a " + record.Class + " store of " + string(record.Owner.Kind) + " " + record.Owner.Ref + " is released by its own owner, not this proof",
			Command: "metasystem disk show"}
	}
	ended, known := false, false
	if p.Ended != nil {
		ended, known = p.Ended(record.Owner)
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
	if record.Layout == LayoutCopy {
		status, err := p.Git(ctx, record.Path, "status", "--porcelain=v1", "--untracked-files=all")
		if err != nil {
			return Verdict{Decision: Pending, Reason: "git status failed: " + err.Error(), Command: "git -C " + record.Path + " status"}
		}
		if strings.TrimSpace(string(status)) != "" {
			return Verdict{Decision: Keep, Reason: "the workspace has uncommitted changes and its owner has ended", Command: discardCommand(record.Owner, name)}
		}
	}
	return Verdict{Decision: Release, Reason: string(record.Owner.Kind) + " " + record.Owner.Ref + " has ended"}
}

// Apply archives a copy's branch tip, then removes the layout, inside the
// critical section RegisteredStores holds.
func (p WorkspaceProof) Apply(ctx context.Context, critical *Critical) error {
	record := critical.Record()
	if record.Layout == LayoutCopy {
		archive, unique, err := archiveWorkspace(ctx, p.request(), record, filepath.Base(record.Path))
		if err != nil {
			return err
		}
		if archive != "" {
			record.Notes = append(record.Notes, fmt.Sprintf("released %s: branch tip archived as %s (%d commit(s) nothing else contains)",
				p.Now.UTC().Format(time.RFC3339), archive, unique))
			if err := critical.Write(record); err != nil {
				return err
			}
		}
	}
	return removeWorkspace(ctx, p.request(), record)
}
