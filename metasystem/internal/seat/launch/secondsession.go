package launch

// A second session: an isolated writer worktree beside the checkout, for a
// main that finds this checkout held by another. The worktree gets its own
// branch, the adapters' declared local configuration copied and audited for
// isolation (validate.SessionIsolation), and a bootstrap arming of its own
// supervision, so the new session starts in a governed checkout of its own.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto/rand"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// SecondSessionOptions is one second-session request. The seams are the
// process and repository effects; production wiring lives in the engine verb.
type SecondSessionOptions struct {
	HarnessRoot string // the installation the request runs from
	Name        string // the worktree and branch name; empty mints one
	Pid         int64  // this process: the bootstrap arming's session identity

	// Git runs git with the arguments and returns its standard output.
	Git func(args ...string) (string, error)
	// StartedAt is a process's kernel start time in epoch seconds.
	StartedAt func(pid int64) (int64, error)
	// Token is a short random hex word for a minted name.
	Token func() (string, error)
	// Now is the clock a minted name is stamped with.
	Now func() time.Time
	// Isolate copies the manifest's local configuration and audits it,
	// returning the new checkout's harness root; nil is validate.SessionIsolation.
	Isolate func(sourceRoot, destinationRoot, manifestPath, harnessRoot string) (string, error)
	// ArmSupervision runs the new harness's engine `up` entry with the arguments.
	ArmSupervision func(newHarness string, args []string) error
	// Registry is the checkout registry the worktree is recorded in; empty
	// is the harness root's.
	Registry diskstore.Registry
}

// SecondSessionError is a refused second session; Code is the exit status.
type SecondSessionError struct {
	Code   int
	Detail string
}

func (e *SecondSessionError) Error() string { return e.Detail }

// SecondSessionIsolated answers a named isolation whose worktree already
// exists (R-129-ui): success with its path. It travels as the error value of
// SecondSession only so that one signature carries it; callers treat it as
// success.
type SecondSessionIsolated struct {
	Path   string
	Branch string
}

func (e *SecondSessionIsolated) Error() string {
	return "second-session: " + e.Path + " is already this checkout's isolated worktree on " + e.Branch
}

var secondSessionName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// sessionWorktreeExists reports whether destination is one of checkout's
// worktrees with branch checked out, as git worktree list says.
func sessionWorktreeExists(git func(args ...string) (string, error), checkout, destination, branch string) bool {
	listed, err := git("-C", checkout, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	want := canonicalSessionPath(destination)
	for _, block := range strings.Split(listed, "\n\n") {
		path, onBranch := "", ""
		for _, line := range strings.Split(block, "\n") {
			if value, found := strings.CutPrefix(line, "worktree "); found {
				path = value
			}
			if value, found := strings.CutPrefix(line, "branch "); found {
				onBranch = value
			}
		}
		if path != "" && canonicalSessionPath(path) == want && onBranch == "refs/heads/"+branch {
			return true
		}
	}
	return false
}

// SecondSession creates the worktree and returns its canonical path.
func SecondSession(o SecondSessionOptions) (string, error) {
	top, err := o.Git("-C", o.HarnessRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("second-session: %s is not inside a git checkout: %w", o.HarnessRoot, err)
	}
	checkout := canonicalSessionPath(strings.TrimSuffix(top, "\n"))
	name := o.Name
	if name == "" {
		token, err := o.Token()
		if err != nil {
			return "", err
		}
		name = filepath.Base(checkout) + "-session-" + o.Now().UTC().Format("20060102t150405z") + "-" + token
	}
	if !secondSessionName.MatchString(name) {
		return "", &SecondSessionError{Code: 2, Detail: "second-session name must contain only letters, numbers, dot, underscore, and hyphen"}
	}
	destination := filepath.Join(filepath.Dir(checkout), name)
	if _, err := os.Lstat(destination); err == nil {
		// A named isolation that already exists as this checkout's worktree
		// on its session branch is a repeat whose effect holds (R-129-ui):
		// its path is the answer, and nothing is created or armed again.
		if o.Name != "" && sessionWorktreeExists(o.Git, checkout, destination, "session/"+name) {
			return canonicalSessionPath(destination), &SecondSessionIsolated{Path: canonicalSessionPath(destination), Branch: "session/" + name}
		}
		return "", &SecondSessionError{Code: 1, Detail: "second-session destination already exists: " + destination}
	}
	// The worktree is registered before git writes a byte of it, reserved
	// with its bootstrap until a main announces itself (Part B 3.2 "Seat").
	started, err := o.StartedAt(o.Pid)
	if err != nil {
		return "", fmt.Errorf("second-session: this process's start time is unreadable: %w", err)
	}
	bootstrap := diskstore.BootstrapRef(o.Pid, started)
	registry := o.Registry
	if registry.Dir == "" {
		registry = diskstore.CheckoutRegistry(o.HarnessRoot)
	}
	record, err := diskstore.ReserveLinkedWorktree(registry, destination, diskstore.SessionWorktreeClass,
		diskstore.Owner{Kind: diskstore.OwnerSession, Ref: name}, o.HarnessRoot, bootstrap, o.Now().UTC(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("second-session: the worktree cannot be registered: %w", err)
	}
	if _, err := o.Git("-C", checkout, "worktree", "add", "-q", "-b", "session/"+name, destination, "HEAD"); err != nil {
		_ = diskstore.AbandonLinkedWorktree(registry, record.ID, err.Error())
		return "", fmt.Errorf("second-session: git worktree add failed: %w", err)
	}
	// Its identity is its .git file; a worktree whose identity cannot be
	// read keeps a record that never proves it, so it is never removed.
	_, _ = registry.IdentifyLinkedWorktree(record.ID)

	manifest, doneManifest, err := diskstore.ScratchFile("metasystem-local-config-paths.")
	if err != nil {
		return "", err
	}
	defer doneManifest()
	if _, err := manifest.WriteString(Manifest()); err != nil {
		manifest.Close()
		return "", err
	}
	if err := manifest.Close(); err != nil {
		return "", err
	}
	isolate := o.Isolate
	if isolate == nil {
		isolate = validate.SessionIsolation
	}
	before := diskstore.PresentPaths(destination, LocalConfigPaths)
	newHarness, err := isolate(checkout, destination, manifest.Name(), o.HarnessRoot)
	if err != nil {
		return "", err
	}
	// What the engine placed in the worktree, and the directory its mains
	// announce themselves in, are the engine's (Round D3 F-1): unchanged,
	// they never keep the worktree at its release.
	installation, relErr := filepath.Rel(canonicalSessionPath(destination), canonicalSessionPath(newHarness))
	if relErr != nil || strings.HasPrefix(installation, "..") {
		installation = "."
	}
	if err := registry.RecordEngineContent(record.ID, destination, diskstore.Placed(destination, LocalConfigPaths, before),
		[]diskstore.EngineDir{diskstore.MainAnnouncementsDir(installation)}); err != nil {
		return "", fmt.Errorf("second-session: what the engine placed in the worktree cannot be recorded: %w", err)
	}

	pid := strconv.FormatInt(o.Pid, 10)
	if err := o.ArmSupervision(newHarness, []string{
		"--repo", destination,
		"--session", "second-session-bootstrap-" + name + "-" + pid,
		"--pid", pid,
		"--start-time", strconv.FormatInt(started, 10),
		"--tag", "metasystem-main-bootstrap-" + name + "-" + pid,
	}); err != nil {
		return "", fmt.Errorf("second-session: arming the new session's supervision failed: %w", err)
	}
	return canonicalSessionPath(destination), nil
}

func canonicalSessionPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}
