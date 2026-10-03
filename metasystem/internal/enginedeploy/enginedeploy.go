// Package enginedeploy is the deploy adapter of this repository's engine. It
// owns the engines directory, where each deployed commit's engine lies in a
// directory of its own that is never rewritten, so a running process's bytes
// cannot change under it, and the pointer, the symbolic link the active
// engine is run through. It answers the deploy contract's operations
// (internal/deploy) for `go run -trimpath ./cmd/devgate deploy OPERATION`.
package enginedeploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
)

// The contract's exit meanings: done, failed with nothing changed, and an
// operation this adapter does not support. Any other exit leaves what is
// active unknown.
const (
	ExitDone        = 0
	ExitFailed      = 1
	ExitUnsupported = 64
	exitUnknown     = 2
)

// stagingMark separates a staging directory's commit from the process that
// builds in it: engines/<commit>.staging-<pid>. linkMark begins the new name
// an activating process writes beside the pointer and renames over it:
// bin/.metasystem.link-<pid>.
const (
	stagingMark = ".staging-"
	linkMark    = ".metasystem.link-"
)

var fullCommit = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// Engines is the engines directory and the pointer under one home directory
// (~/.metasystem).
type Engines struct {
	Home string
	// Build builds the engine of the tree being deployed at out, stamped
	// with its commit.
	Build func(out string) error
	// Log receives what an operation says besides its response.
	Log io.Writer
}

// Pointer is the symbolic link that names the active engine.
func (e Engines) Pointer() string { return filepath.Join(e.Home, "bin", "metasystem") }

// Engine is where the engine of commit lies once it is built.
func (e Engines) Engine(commit string) string { return filepath.Join(e.dir(), commit, "metasystem") }

func (e Engines) dir() string { return filepath.Join(e.Home, "engines") }

// Serve answers one operation: its request is read from in, its response
// written to out, and the contract's exit status returned.
func (e Engines) Serve(operation string, in io.Reader, out io.Writer) int {
	operations := map[string]func(deploy.Request) (deploy.Response, error){
		"build": e.build, "activate": e.activate, "version": e.version, "rollback": e.rollback,
	}
	act, supported := operations[operation]
	if !supported {
		// go run exits 1 for any failing program, so the answer says it
		// failed too: through go run an unsupported operation still reads
		// as one that changed nothing.
		_ = Failed(out, fmt.Errorf("the engine's deploy adapter does not support the operation %q", operation))
		return ExitUnsupported
	}
	var request deploy.Request
	if err := json.NewDecoder(in).Decode(&request); err != nil {
		return Failed(out, fmt.Errorf("the %s request can't be read: %w", operation, err))
	}
	if request.Schema != 1 || request.Operation != operation {
		return Failed(out, fmt.Errorf("the request is schema %d for %q, not schema 1 for %q", request.Schema, request.Operation, operation))
	}
	response, err := act(request)
	if err != nil {
		return Failed(out, err)
	}
	if err := json.NewEncoder(out).Encode(response); err != nil {
		fmt.Fprintf(e.log(), "the %s response can't be written: %v\n", operation, err)
		return exitUnknown
	}
	return ExitDone
}

// Failed answers an operation that changed nothing: a failed response
// naming why, and its exit status.
func Failed(out io.Writer, reason error) int {
	if err := json.NewEncoder(out).Encode(deploy.Response{Outcome: "failed", Reason: reason.Error()}); err != nil {
		return exitUnknown
	}
	return ExitFailed
}

// build builds the commit's engine in a staging directory beside the
// engines, checks that its stamp is the commit and that it starts, and then
// renames the directory to engines/<commit>/. It never changes what is
// active. An engines/<commit>/ that exists is never rewritten: it is reused
// when it holds the same bytes, and refused otherwise.
func (e Engines) build(request deploy.Request) (deploy.Response, error) {
	commit := request.Commit
	if !fullCommit.MatchString(commit) {
		return deploy.Response{}, fmt.Errorf("%q is not a full commit name, so no engine is built for it", commit)
	}
	if err := os.MkdirAll(e.dir(), 0o755); err != nil {
		return deploy.Response{}, err
	}
	clearEnded(e.dir(), stagingMark)
	staging := filepath.Join(e.dir(), commit+stagingMark+strconv.Itoa(os.Getpid()))
	if err := os.RemoveAll(staging); err != nil {
		return deploy.Response{}, err
	}
	if err := os.Mkdir(staging, 0o755); err != nil {
		return deploy.Response{}, err
	}
	// Once renamed, the staging directory is gone and this removes nothing.
	defer func() { _ = os.RemoveAll(staging) }()
	staged := filepath.Join(staging, "metasystem")
	if err := e.Build(staged); err != nil {
		return deploy.Response{}, fmt.Errorf("the engine of %s can't be built: %w", deploy.Short(commit), err)
	}
	stamp, digest, err := inspect(staged)
	if err != nil {
		return deploy.Response{}, fmt.Errorf("the engine built from %s can't be read: %w", deploy.Short(commit), err)
	}
	if stamp != commit {
		return deploy.Response{}, fmt.Errorf("the engine built from %s carries the build stamp %q, not its commit, so it is not deployed", deploy.Short(commit), stamp)
	}
	if err := e.start(staged); err != nil {
		return deploy.Response{}, fmt.Errorf("the engine built from %s does not start, so it is never activated: %w", deploy.Short(commit), err)
	}
	built := deploy.Response{Outcome: "built", Version: commit, Artifact: e.Engine(commit), Digest: digest}
	final := filepath.Dir(built.Artifact)
	renameErr := os.Rename(staging, final)
	if renameErr == nil {
		return built, nil
	}
	if _, err := os.Lstat(final); err != nil {
		return deploy.Response{}, renameErr
	}
	_, existing, err := inspect(built.Artifact)
	switch {
	case err != nil:
		return deploy.Response{}, fmt.Errorf("%s exists and its engine can't be read, and an engine directory is never rewritten: %w", final, err)
	case existing != digest:
		return deploy.Response{}, fmt.Errorf("%s holds an engine with the checksum %s, not this build's %s, and an engine directory is never rewritten", final, existing, digest)
	}
	return built, nil
}

// clearEnded removes what an interrupted build or activation left in dir:
// the names carrying mark and the process id of a process that has ended.
func clearEnded(dir, mark string) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		_, owner, marked := strings.Cut(entry.Name(), mark)
		pid, err := strconv.Atoi(owner)
		if marked && err == nil && pid > 0 && !alive(pid) {
			_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
		}
	}
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// start runs the built engine once, as the check that it starts at all: its
// help page touches no state.
func (e Engines) start(engine string) error {
	command := exec.Command(engine, "help")
	command.Stdout, command.Stderr = io.Discard, e.log()
	return command.Run()
}

// activate makes the request's artifact the active engine. Repeating it
// points at the same engine again.
func (e Engines) activate(request deploy.Request) (deploy.Response, error) {
	artifact := filepath.Clean(request.Artifact)
	stamp, _, err := e.engine(artifact)
	if err != nil {
		return deploy.Response{}, err
	}
	if err := e.point(artifact); err != nil {
		return deploy.Response{}, err
	}
	return deploy.Response{Outcome: "active", Version: stamp}, nil
}

// point writes a new name of the engine's own link beside the pointer and
// renames it over the pointer: a reader resolves the old engine or the new
// one, never neither. The link is engines/<commit>/link, so the link a
// rename replaces keeps a name in its engine's directory: on APFS a lookup
// still traversing a replaced link that has no name left fails with
// EINVAL, and a rename that dropped the pointer's last name would let a
// concurrent reader find no engine.
func (e Engines) point(artifact string) error {
	pointer := e.Pointer()
	if err := os.MkdirAll(filepath.Dir(pointer), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(pointer); err == nil && info.Mode()&fs.ModeSymlink == 0 {
		return fmt.Errorf("%s is not the symbolic link deploys move, so it is left as it is", pointer)
	}
	own := filepath.Join(filepath.Dir(artifact), "link")
	switch target, err := os.Readlink(own); {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.Symlink(artifact, own); err != nil {
			return err
		}
	case err != nil:
		return err
	case target != artifact:
		return fmt.Errorf("%s names %s, not its engine, so it is not activated", own, target)
	}
	clearEnded(filepath.Dir(pointer), linkMark)
	name := filepath.Join(filepath.Dir(pointer), linkMark+strconv.Itoa(os.Getpid()))
	_ = os.Remove(name)
	// Flags 0: the new name is the link itself, never the engine it names.
	if err := unix.Linkat(unix.AT_FDCWD, own, unix.AT_FDCWD, name, 0); err != nil {
		return err
	}
	// A rename onto a name of the same link changes nothing and leaves the
	// new name behind.
	defer func() { _ = os.Remove(name) }()
	return os.Rename(name, pointer)
}

// version reports the engine the pointer names, its stamp and its checksum;
// none while no engine was ever activated. It only reads.
func (e Engines) version(deploy.Request) (deploy.Response, error) {
	target, err := os.Readlink(e.Pointer())
	if errors.Is(err, fs.ErrNotExist) {
		return deploy.Response{Outcome: "none"}, nil
	}
	if err != nil {
		return deploy.Response{}, fmt.Errorf("the pointer %s can't be read as a symbolic link: %w", e.Pointer(), err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(e.Pointer()), target)
	}
	stamp, digest, err := inspect(target)
	if err != nil {
		return deploy.Response{}, fmt.Errorf("the pointer names %s, which can't be read: %w", target, err)
	}
	return deploy.Response{Outcome: "active", Version: stamp, Artifact: target, Digest: digest}, nil
}

// rollback activates the previous deploy's engine again, without building,
// once it is still there with the checksum its deploy recorded.
func (e Engines) rollback(request deploy.Request) (deploy.Response, error) {
	previous := request.Previous
	if previous == nil || previous.Artifact == "" {
		return deploy.Response{}, errors.New("no previous engine is named, so there is nothing to roll back to")
	}
	artifact := filepath.Clean(previous.Artifact)
	_, digest, err := e.engine(artifact)
	if err != nil {
		return deploy.Response{}, err
	}
	if digest != previous.Digest {
		return deploy.Response{}, fmt.Errorf("%s has the checksum %s, not the %s its deploy recorded, so it is not activated", artifact, digest, previous.Digest)
	}
	return e.activate(deploy.Request{Artifact: artifact})
}

// engine reads an engine of the engines directory: only a never-rewritten
// engines/<commit>/metasystem is ever made active.
func (e Engines) engine(artifact string) (stamp, digest string, err error) {
	commit := filepath.Base(filepath.Dir(artifact))
	if !fullCommit.MatchString(commit) || artifact != e.Engine(commit) {
		return "", "", fmt.Errorf("%q is not an engine of %s, so it is not activated", artifact, e.dir())
	}
	stamp, digest, err = inspect(artifact)
	if err != nil {
		return "", "", fmt.Errorf("the engine %s can't be activated: %w", artifact, err)
	}
	return stamp, digest, nil
}

// inspect reads an engine file's build stamp, without running it, and its
// sha256 checksum.
func inspect(path string) (stamp, digest string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("%s is not a file", path)
	}
	if stamp, err = enginebuild.ReadStamp(file); err != nil {
		return "", "", err
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", "", err
	}
	return stamp, "sha256:" + hex.EncodeToString(sum.Sum(nil)), nil
}

func (e Engines) log() io.Writer {
	if e.Log == nil {
		return io.Discard
	}
	return e.Log
}
