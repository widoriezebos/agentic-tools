package diskstore

// Workspaces the engine hands out (design engine-owns-disk-lifetimes Part B,
// 3.6): a registered directory under
// <control>/artifacts/agents/workspaces/<owner>/<name>/, owned by a goal or
// a session, in one of two layouts. A plain workspace is an empty marker
// directory; a copy is a linked worktree of the checkout's common store on
// branch workspace/<owner>/<name>, so its commits live in the common store
// and it has no object store of its own. A request is serialized on its
// reservation key and never claims a path it did not create; its release
// is its own end, separate from the goal's, and archives every tip of a
// copy before anything is removed (workspace_release.go).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// WorkspaceArchiveRef is where a copy's branch tip is archived at release;
// its worktree HEAD and reflog tips are archived beneath it.
func WorkspaceArchiveRef(owner Owner, name string) string {
	return "refs/archive/" + WorkspaceOwnerSegment(owner) + "/" + workspaceName(name) + "/" + WorkspaceBranch(owner, name)
}

// WorkspacePath is where a workspace lives.
func WorkspacePath(control string, owner Owner, name string) string {
	return filepath.Join(workspacesRoot(control), WorkspaceOwnerSegment(owner), workspaceName(name))
}

func workspacesRoot(control string) string {
	return filepath.Join(control, "artifacts", "agents", "workspaces")
}

// WorkspaceTmp is the workspace's temporary directory: inside a plain
// workspace; for a copy, whose tree must stay clean, under the workspace
// store's reserved .metasystem-tmp keyed by the record id, which no
// workspace name can reach (names never start with a dot).
func WorkspaceTmp(record Record) string {
	if record.Layout == LayoutCopy {
		return filepath.Join(workspacesRoot(record.Checkout), ".metasystem-tmp", record.ID)
	}
	return filepath.Join(record.Path, "tmp")
}

// ValidWorkspaceName reports a name usable as one path element and one
// branch component (git check-ref-format --branch): letters, digits, dot,
// underscore and dash, not starting with a dot or a dash, not ending with
// a dot or ".lock", no "..", at most 100 characters.
func ValidWorkspaceName(name string) bool {
	if name == "" || len(name) > 100 || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") ||
		strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") || strings.Contains(name, "..") {
		return false
	}
	return strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-')
	}) < 0
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

// WorkspaceUnproven is a path the request did not make and cannot prove
// it made: it is never claimed or removed; a person looks at it.
type WorkspaceUnproven struct {
	Path   string
	Reason string
}

func (e *WorkspaceUnproven) Error() string {
	return fmt.Sprintf("%s %s; nothing there was changed; metasystem disk show lists it for a person to settle", e.Path, e.Reason)
}

type reservation struct {
	Record string `json:"record"`
}

// ObtainWorkspace returns the workspace a request names, creating it when
// none is accepted (3.6's repeat contract). The reservation lock is taken
// before a ulid is allocated or a byte copied; the same request again
// returns the same workspace and writes nothing. A path that exists
// without the request's record is never claimed; a creation that fails
// removes what it proves it made and its reservation.
func ObtainWorkspace(ctx context.Context, request WorkspaceRequest) (Workspace, error) {
	if request.Owner.Kind != OwnerGoal && request.Owner.Kind != OwnerSession {
		return Workspace{}, fmt.Errorf("a workspace belongs to a goal or a session, not %s", request.Owner.Kind)
	}
	if !ValidWorkspaceName(workspaceName(request.Name)) {
		return Workspace{}, fmt.Errorf("%q is not a workspace name: letters, digits, dot, underscore and dash, not starting with a dot or dash", request.Name)
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
				return Workspace{}, err
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
	if _, err := os.Lstat(path); err == nil {
		return Workspace{}, &WorkspaceUnproven{Path: path, Reason: "exists but no workspace record holds it"}
	}
	if layout == LayoutCopy {
		if _, found := revParseIn(ctx, request.Git, request.GitRoot, "refs/heads/"+WorkspaceBranch(request.Owner, request.Name)); found {
			return Workspace{}, &WorkspaceUnproven{Path: path, Reason: "has no worktree, but its branch " + WorkspaceBranch(request.Owner, request.Name) + " exists without a record"}
		}
	}
	record, err := request.Registry.Register(Registration{Path: path, Git: layout == LayoutCopy, Class: WorkspaceClass, Owner: request.Owner,
		Checkout: request.Control, Lifetime: LifetimeOwner, CapBytes: request.CapBytes, CapKind: CapTarget, Layout: layout,
		Reservation: key, CopyOf: request.CopyOf}, request.Now, request.Entropy)
	if err != nil {
		return Workspace{}, err
	}
	if record.State != StateReserved || record.Reservation != key {
		return Workspace{}, &WorkspaceUnproven{Path: path, Reason: "is held by store record " + record.ID}
	}
	data, err := json.Marshal(reservation{Record: record.ID})
	if err != nil {
		return Workspace{}, err
	}
	if _, err := atomicfile.WriteFile(entry, append(data, '\n'), 0o600, filepath.Dir(entry)); err != nil {
		return Workspace{}, err
	}
	accepted, err := makeWorkspace(ctx, request, record)
	if err == nil {
		return Workspace{Record: accepted, Tmp: WorkspaceTmp(accepted), Created: true}, nil
	}
	if cleanup := discardPartial(ctx, request, record); cleanup != nil {
		return Workspace{}, fmt.Errorf("the workspace could not be made (%v), and what was made stays: %w", err, cleanup)
	}
	_ = os.Remove(entry)
	return Workspace{}, fmt.Errorf("the workspace could not be made; nothing is left at %s: %w", path, err)
}

// makeWorkspace creates the layout at the recorded path, its temporary
// directory with its marker, and accepts the record.
func makeWorkspace(ctx context.Context, request WorkspaceRequest, record Record) (Record, error) {
	if err := os.MkdirAll(filepath.Dir(record.Path), 0o700); err != nil {
		return Record{}, err
	}
	var mutate func(*Record)
	switch record.Layout {
	case LayoutPlain:
		if err := os.Mkdir(record.Path, 0o700); err != nil {
			return Record{}, err
		}
		if err := WriteMarker(record); err != nil {
			return Record{}, err
		}
	case LayoutCopy:
		branch := WorkspaceBranch(request.Owner, request.Name)
		if _, err := request.Git(ctx, request.GitRoot, "worktree", "add", "-b", branch, record.Path, request.CopyOf); err != nil {
			return Record{}, fmt.Errorf("git worktree add %s: %w", record.Path, err)
		}
		identity, err := ReadGitIdentity(record.Path)
		if err != nil {
			return Record{}, err
		}
		mutate = func(record *Record) { record.Identity = identity }
	}
	if err := makeTmp(record); err != nil {
		return Record{}, err
	}
	return request.Registry.Transition(record.ID, []State{StateReserved}, StateAccepted, mutate)
}

// makeTmp creates the temporary directory; a copy's carries a marker
// naming its record, so its removal proves whose it is.
func makeTmp(record Record) error {
	tmp := WorkspaceTmp(record)
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return err
	}
	if record.Layout != LayoutCopy {
		return nil
	}
	return writeTmpMarker(record, tmp)
}

func writeTmpMarker(record Record, tmp string) error {
	if found, err := readMarker(tmp); err == nil {
		if found.ID == record.ID && found.Path == tmp {
			return nil
		}
		return fmt.Errorf("%s carries the marker of record %s", tmp, found.ID)
	}
	data, err := json.Marshal(marker{Schema: Schema, ID: record.ID, Path: tmp})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tmp, MarkerName), append(data, '\n'), 0o600)
}

// removeTmp removes a copy's temporary directory only when its marker
// names this record, or it is empty; a plain one goes with its store.
func removeTmp(ctx context.Context, record Record) error {
	if record.Layout != LayoutCopy {
		return nil
	}
	tmp := WorkspaceTmp(record)
	if _, err := os.Lstat(tmp); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if found, err := readMarker(tmp); err == nil && found.ID == record.ID && found.Path == tmp {
		return RemoveTree(ctx, tmp)
	}
	if entries, err := os.ReadDir(tmp); err == nil && len(entries) == 0 {
		return os.Remove(tmp)
	}
	return &WorkspaceUnproven{Path: tmp, Reason: "holds content without the marker of record " + record.ID}
}

// discardPartial removes what an interrupted creation of record made, only
// where it proves it made it: a plain directory carrying this record's
// marker, or empty; a worktree on this record's branch, at the recorded
// commit, clean including ignored files; the branch only at the recorded
// commit. Anything else is refused as unproven and nothing is removed.
func discardPartial(ctx context.Context, request WorkspaceRequest, record Record) error {
	critical, err := request.Registry.TryCritical(record.ID)
	if err != nil {
		return err
	}
	defer critical.Release()
	branch := WorkspaceBranch(request.Owner, request.Name)
	switch record.Layout {
	case LayoutCopy:
		if err := discardPartialCopy(ctx, request, record, branch); err != nil {
			return err
		}
	default:
		if err := discardPartialPlain(ctx, record); err != nil {
			return err
		}
	}
	if err := removeTmp(ctx, record); err != nil {
		return err
	}
	record.State = StateReleased
	record.ReleasedBy = "the next request (interrupted creation)"
	return critical.Write(record)
}

func discardPartialPlain(ctx context.Context, record Record) error {
	info, err := os.Lstat(record.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return &WorkspaceUnproven{Path: record.Path, Reason: "is not the directory the interrupted creation made"}
	}
	if found, err := readMarker(record.Path); err == nil && found.ID == record.ID && found.Path == record.Path {
		return RemoveTree(ctx, record.Path)
	}
	if entries, err := os.ReadDir(record.Path); err == nil && len(entries) == 0 {
		return os.Remove(record.Path)
	}
	return &WorkspaceUnproven{Path: record.Path, Reason: "holds content without the marker of record " + record.ID}
}

func discardPartialCopy(ctx context.Context, request WorkspaceRequest, record Record, branch string) error {
	tip, branchFound := revParseIn(ctx, request.Git, request.GitRoot, "refs/heads/"+branch)
	if branchFound && tip != record.CopyOf {
		return &WorkspaceUnproven{Path: record.Path, Reason: "has branch " + branch + " at " + tip + ", not the recorded " + record.CopyOf}
	}
	if _, err := os.Lstat(filepath.Join(record.Path, ".git")); err == nil {
		head, err := os.ReadFile(filepath.Join(gitdirOf(record.Path), "HEAD"))
		if err != nil || strings.TrimSpace(string(head)) != "ref: refs/heads/"+branch {
			return &WorkspaceUnproven{Path: record.Path, Reason: "is a worktree, but not on the recorded branch " + branch}
		}
		status, err := request.Git(ctx, record.Path, "status", "--porcelain=v1", "--untracked-files=all", "--ignored")
		if err != nil || strings.TrimSpace(string(status)) != "" {
			return &WorkspaceUnproven{Path: record.Path, Reason: "is a worktree with changes the interrupted creation did not make"}
		}
		if _, err := request.Git(ctx, request.GitRoot, "worktree", "remove", record.Path); err != nil {
			return err
		}
	} else if _, err := os.Lstat(record.Path); err == nil {
		if entries, err := os.ReadDir(record.Path); err != nil || len(entries) != 0 {
			return &WorkspaceUnproven{Path: record.Path, Reason: "holds content but no worktree"}
		}
		if err := os.Remove(record.Path); err != nil {
			return err
		}
	}
	if _, err := request.Git(ctx, request.GitRoot, "worktree", "prune"); err != nil {
		return err
	}
	if branchFound {
		if _, err := request.Git(ctx, request.GitRoot, "branch", "-D", branch); err != nil {
			return err
		}
	}
	return nil
}

// gitdirOf is the gitdir a worktree's .git file names; empty when it names
// none.
func gitdirOf(worktree string) string {
	identity, err := ReadGitIdentity(worktree)
	if err != nil {
		return ""
	}
	return identity.Gitdir
}

func revParseIn(ctx context.Context, git WorkspaceGit, dir, ref string) (string, bool) {
	out, err := git(ctx, dir, "rev-parse", "--verify", "-q", ref)
	if err != nil {
		return "", false
	}
	sha := strings.TrimSpace(string(out))
	return sha, sha != ""
}
