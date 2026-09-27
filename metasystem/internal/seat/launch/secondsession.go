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
}

// SecondSessionError is a refused second session; Code is the exit status.
type SecondSessionError struct {
	Code   int
	Detail string
}

func (e *SecondSessionError) Error() string { return e.Detail }

var secondSessionName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

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
		return "", &SecondSessionError{Code: 1, Detail: "second-session destination already exists: " + destination}
	}
	if _, err := o.Git("-C", checkout, "worktree", "add", "-q", "-b", "session/"+name, destination, "HEAD"); err != nil {
		return "", fmt.Errorf("second-session: git worktree add failed: %w", err)
	}

	manifest, err := os.CreateTemp("", "metasystem-local-config-paths.")
	if err != nil {
		return "", err
	}
	defer os.Remove(manifest.Name())
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
	newHarness, err := isolate(checkout, destination, manifest.Name(), o.HarnessRoot)
	if err != nil {
		return "", err
	}

	started, err := o.StartedAt(o.Pid)
	if err != nil {
		return "", fmt.Errorf("second-session: this process's start time is unreadable: %w", err)
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
