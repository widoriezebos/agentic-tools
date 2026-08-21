package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// The contract package's own small foundations. The three pure helpers are
// duplicated from mission deliberately (go-production-grade obligation C-3):
// each is a few lines over the standard library with no state, and a package
// created to share them would be exactly the dumping ground the design
// standard forbids. The durable write is NOT duplicated — it delegates to
// the one owner (C-4).

// ContractError is a contract validation or operation failure. The contract
// package carries its own error type rather than reaching back into mission
// for one; no consumer outside internal/mission ever read mission's, so
// nothing observable changes (C-2).
type ContractError struct{ msg string }

func (e *ContractError) Error() string { return e.msg }

func stateErr(format string, args ...any) error {
	return &ContractError{msg: fmt.Sprintf(format, args...)}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func resolvePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func isValidUTF8(data []byte) bool {
	return strings.ToValidUTF8(string(data), "�") == string(data)
}

func atomicWriteText(path, text string) error {
	// Empty anchor until this writer is converted to the two-outcome
	// signature (go-production-grade B5); see the mission copy.
	_, err := atomicfile.WriteText(path, text, "")
	return err
}

// The identifier and count grammars the contract's own validation applies.
// Same rationale as the pure helpers above: two short regexes, duplicated
// rather than shared through a package that would exist only to hold them.
var (
	idRe          = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	positiveIntRe = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// contractGitArgs builds every contract-side git invocation on the
// runner's own surface: object replacement disabled and (via the caller
// setting ScrubbedEnviron) the repository-steering environment stripped,
// so measurement and gate pins read the objects themselves.
func contractGitArgs(repo string, args []string) []string {
	return append([]string{"-C", repo, "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)
}

// gitOutput runs a git subcommand in a checkout, returning its stdout and
// carrying git's own stderr into the failure. Contract preflight reads the
// repository this way (freshness, provenance); mission keeps its own copy
// because its errors are mission errors.
func gitOutput(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", contractGitArgs(repo, args)...)
	cmd.Env = gittree.ScrubbedEnviron()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Bounded like every other external call (B4): a git that never
	// returns must not hang the caller.
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " ")); err != nil {
		return stdout.String(), stateErr("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// gitTry runs a git subcommand and reports its exit code instead of an
// error, for the checks that treat a nonzero status as an answer rather
// than a failure.
func gitTry(repo string, args ...string) (string, int) {
	cmd := exec.Command("git", contractGitArgs(repo, args)...)
	cmd.Env = gittree.ScrubbedEnviron()
	var stdout strings.Builder
	cmd.Stdout = &stdout
	// Bounded like every other external call (B4); a timeout is a failure
	// answer, not an exit code.
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " "))
	if err == nil {
		return stdout.String(), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.String(), exit.ExitCode()
	}
	return stdout.String(), -1
}

// requiredFenceKeys are the fence bounds every contract must declare. The
// contract owns this list because the contract is where they are AUTHORED;
// mission keeps its own copy for the fences it enforces at runtime.
var requiredFenceKeys = []string{
	"fence.wall-clock-hours", "fence.cycles", "fence.jobs", "fence.concurrency", "fence.job-cap-min",
}

// clock is the time source, overridable in tests.
var clock = time.Now

func nowUTC() time.Time { return clock().UTC() }

// relUnderRepo returns path relative to repo, refusing anything outside it.
func relUnderRepo(path, repo string) (string, error) {
	pathAbs := resolvePath(path)
	repoAbs := resolvePath(repo)
	rel, err := filepath.Rel(repoAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", stateErr("mission ledger is outside the repository")
	}
	return filepath.ToSlash(rel), nil
}

// contractNameRe is the contract FILE-NAME grammar: mission-<id>.contract.md.
var contractNameRe = regexp.MustCompile(`^mission-([a-z0-9][a-z0-9-]*)\.contract\.md$`)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func intValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		if strings.ContainsAny(n.String(), ".eE") {
			return 0, false
		}
		i, err := n.Int64()
		return i, err == nil
	case float64:
		return int64(n), n == math.Trunc(n)
	}
	return 0, false
}

// hashRe is the sha256 hex grammar the seal and its tests apply.
var hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
