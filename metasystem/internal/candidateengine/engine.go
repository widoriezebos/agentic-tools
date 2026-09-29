// Package candidateengine builds the engine a test run proves with from the
// exact candidate tree, keyed by a build identity over the tree's engine
// projection, toolchain, build environment and platform, and keeps the
// built executables in a validated cache under the proof control root.
package candidateengine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Engine is one candidate engine built from an exact project tree: its
// executable, the executable's digest, the synthetic candidate commit its
// build identity names, and how long its preparation queued.
type Engine struct {
	Path, Digest, Commit string
	QueueDurationMS      int64
	directory            string
}

type cacheRecord struct {
	Version       int    `json:"version"`
	BuildIdentity string `json:"buildIdentity"`
	Digest        string `json:"digest"`
}

// DetachedWorkspace is a materialized candidate tree the build runs in.
type DetachedWorkspace interface {
	Workspace() gittree.Workspace
	Close() error
}

// IO is the Git and worktree seams of a candidate engine preparation.
type IO struct {
	RunGit func(*exec.Cmd) error
	Open   func(gittree.Workspace, string) (DetachedWorkspace, error)
	// Rename publishes a stage into the v2 namespace; nil is os.Rename.
	Rename func(string, string) error
	// BuildArgv is the build owner's command, run in the candidate's
	// installation root; nil is DefaultBuildArgv.
	BuildArgv func(output string) []string
	// Native says the seams are the real Git and worktree ones, so a managed
	// run may materialize the tree in its own scratch root instead.
	Native bool
	// Notes receives what the preparation tells the person running it (a
	// candidate engine kept private to the run); nil tells nothing.
	Notes io.Writer
}

// DefaultBuildArgv runs the candidate tree's own fenced, stamped
// bootstrap build (cmd/devgate), so the build rules that compile a candidate
// are the candidate's, not this engine's.
func DefaultBuildArgv(output string) []string {
	return []string{"go", "run", "-trimpath", "./cmd/devgate", "build", "--trimpath", "--out", output}
}

// Native is the real Git and detached-worktree seams.
func Native() IO {
	return IO{
		RunGit: func(command *exec.Cmd) error { return command.Run() },
		Open: func(workspace gittree.Workspace, tree string) (DetachedWorkspace, error) {
			return workspace.NewDetachedWorktree(tree)
		},
		Native: true,
	}
}

func selected(options []IO) IO {
	if len(options) == 0 {
		return Native()
	}
	return options[0]
}

// BuildEnvironment is environment with every build-owned variable pinned
// and the candidate commit stamped, so a proof build's bytes follow only its
// tree.
func BuildEnvironment(environment []string, stamp string) []string {
	owned := map[string]bool{
		"CGO_ENABLED": true, "GOAMD64": true, "GOARM": true, "GOARM64": true,
		"GOENV": true, "GOEXPERIMENT": true, "GOFLAGS": true, "GOTOOLCHAIN": true,
		"GOWORK": true, "METASYSTEM_BUILD_STAMP": true,
	}
	result := make([]string, 0, len(environment)+10)
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if !owned[key] {
			result = append(result, entry)
		}
	}
	// Microarchitecture feature levels affect compiled bytes, so proof builds use Go's defaults.
	return append(result, "CGO_ENABLED=0", "GOAMD64=v1", "GOARM64=v8.0", "GOARM=7", "GOENV=off",
		"GOEXPERIMENT=", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local", "GOWORK=off", "METASYSTEM_BUILD_STAMP="+stamp)
}

// Proof custody identifies the enclosing run, not the candidate engine's
// compiled behavior. It reaches legacy build scripts but does not fragment
// the cache key across equivalent attempts.
func semanticEnvironment(environment []string) []string {
	nonsemantic := map[string]bool{
		"METASYSTEM_PROOF_CONTROL_ROOT": true, "METASYSTEM_PROOF_ATTEMPT": true,
		"METASYSTEM_PROOF_RECORD_KEY": true, "METASYSTEM_PROOF_CREATION_CLAIM": true,
		"METASYSTEM_PROOF_AUTH_BIN": true, identity.RunOwnerEnv: true,
		identity.FixtureAttemptEnv: true,
	}
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if ok && !nonsemantic[name] {
			result = append(result, entry)
		}
	}
	return result
}

func (build *Engine) Close() error {
	if build == nil || build.directory == "" {
		return nil
	}
	directory := build.directory
	build.directory = ""
	return os.RemoveAll(directory)
}

// Prepare keeps build outputs under the proof control root.
// The existing build identity covers the tracked engine closure, platform and
// toolchain. Every cache hit also checks the published bytes before use.
func Prepare(ctx context.Context, controlRoot string, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, beforeColdBuild func() error, options ...IO) (*Engine, error) {
	io := selected(options)
	buildIdentity, err := BuildIdentityUsing(ctx, workspace, installationPrefix, candidateTree, environment, io)
	if err != nil {
		return nil, err
	}
	scratch := proofrun.ScratchRunFromContext(ctx)
	if scratch == nil {
		// Every candidate engine is built inside a proof run's scratch into
		// the v2 namespace; the legacy candidate-engines/<identity> branch is
		// gone (engine-owns-disk-lifetimes 3.5, DL2-11), and its entries are
		// strays a person removes.
		return nil, fmt.Errorf("a candidate engine is prepared only inside a proof run's scratch (metasystem test run creates one)")
	}
	return prepareScratch(ctx, scratch, controlRoot, workspace, installationPrefix, candidateTree, environment, beforeColdBuild, io, buildIdentity)
}

// v2Retained bounds the v2 namespace; legacy entries beside
// it are never read or evicted (a person removes them).
const v2Retained = 8

// prepareScratch is the run-rooted engine path: a cold build
// compiles inside the run's scratch root and publishes to
// candidate-engines/v2/<identity> by a same-filesystem rename under the
// identity lock; warm or cold, the caller receives a validated private copy
// inside its own root before any v2 eviction runs.
func prepareScratch(ctx context.Context, scratch *proofrun.ScratchRun, controlRoot string, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, beforeColdBuild func() error, io IO, buildIdentity string) (*Engine, error) {
	cacheRoot := filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines", "v2")
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create candidate engine cache: %w", err)
	}
	lockFile, err := os.OpenFile(filepath.Join(cacheRoot, buildIdentity+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("reserve candidate engine identity: %w", err)
	}
	defer lockFile.Close()
	cacheWaitStarted := time.Now()
	for {
		if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		} else if err != unix.EWOULDBLOCK && err != unix.EAGAIN {
			return nil, fmt.Errorf("reserve candidate engine identity: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	queued := time.Since(cacheWaitStarted).Milliseconds()
	entry := filepath.Join(cacheRoot, buildIdentity)
	source := validated(entry, buildIdentity)
	if source == nil {
		if beforeColdBuild != nil {
			if err := beforeColdBuild(); err != nil {
				return nil, err
			}
		}
		lease, err := proofrun.AcquireHostResources(ctx, controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "heavy", nil)
		if err != nil {
			return nil, fmt.Errorf("admit candidate engine build: %w", err)
		}
		defer lease.Close()
		queued += lease.Waited().Milliseconds()
		built, err := Build(proofrun.WithHostResourceLease(ctx, lease), workspace, installationPrefix, candidateTree, environment, io)
		if err != nil {
			return nil, err
		}
		defer built.Close()
		if built.Commit != buildIdentity {
			return nil, fmt.Errorf("candidate engine build identity changed during preparation")
		}
		entry, err = publishScratch(scratch.Dir("engine"), entry, buildIdentity, built, io.Rename, io.Notes)
		if err != nil {
			return nil, err
		}
		if source = validated(entry, buildIdentity); source == nil {
			return nil, fmt.Errorf("published candidate engine artifact failed validation")
		}
	}
	private := filepath.Join(scratch.Dir("engine"), "metasystem")
	if err := CopyArtifact(source.Path, private); err != nil {
		return nil, fmt.Errorf("copy candidate engine into the run: %w", err)
	}
	if sum, err := digest.FileSHA256(private); err != nil || sum != source.Digest {
		return nil, fmt.Errorf("candidate engine copy failed validation: %v", err)
	}
	if entry != filepath.Join(cacheRoot, buildIdentity) {
		return &Engine{Path: private, Digest: source.Digest, Commit: buildIdentity, QueueDurationMS: queued}, nil
	}
	now := time.Now()
	_ = os.Chtimes(filepath.Join(entry, "record.json"), now, now)
	evictV2(cacheRoot, buildIdentity)
	return &Engine{Path: private, Digest: source.Digest, Commit: buildIdentity, QueueDurationMS: queued}, nil
}

// publishScratch stages the built engine inside the run root
// and renames it to entry. On EXDEV the run keeps the stage as its private
// source (engine.unpublished) and nothing is staged outside its root; the
// returned path is where the validated engine now lives.
func publishScratch(stageParent, entry, buildIdentity string, built *Engine, rename func(string, string) error, notes io.Writer) (string, error) {
	stage, err := os.MkdirTemp(stageParent, "stage-")
	if err != nil {
		return "", err
	}
	if err := CopyArtifact(built.Path, filepath.Join(stage, "metasystem")); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(cacheRecord{Version: 1, BuildIdentity: buildIdentity, Digest: built.Digest})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, "record.json"), encoded, 0o600); err != nil {
		return "", err
	}
	if err := os.RemoveAll(entry); err != nil {
		return "", fmt.Errorf("discard invalid candidate engine artifact: %w", err)
	}
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(stage, entry); errors.Is(err, unix.EXDEV) {
		if notes != nil {
			fmt.Fprintln(notes, "metasystem test run: engine.unpublished: candidate engine stays private to this run:", err)
		}
		return stage, nil
	} else if err != nil {
		return "", fmt.Errorf("publish candidate engine artifact: %w", err)
	}
	return entry, nil
}

// evictV2 keeps the newest entries by record mtime; an entry
// is removed only while its own identity lock is taken exclusively, and lock
// files are never removed.
func evictV2(cacheRoot, own string) {
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		return
	}
	type aged struct {
		name string
		at   time.Time
	}
	var candidates []aged
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := os.Stat(filepath.Join(cacheRoot, entry.Name(), "record.json"))
		if err != nil {
			continue
		}
		candidates = append(candidates, aged{entry.Name(), info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].at.After(candidates[j].at) })
	// Oldest first, until the actual survivors fit; a held (live) entry and
	// the caller's own are skipped, never removed.
	survivors := len(candidates)
	for index := len(candidates) - 1; index >= 0 && survivors > v2Retained; index-- {
		candidate := candidates[index]
		if candidate.name == own {
			continue
		}
		lock, err := os.OpenFile(filepath.Join(cacheRoot, candidate.name+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			continue
		}
		if unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil && os.RemoveAll(filepath.Join(cacheRoot, candidate.name)) == nil {
			survivors--
		}
		_ = lock.Close()
	}
}

func validated(entry, buildIdentity string) *Engine {
	var record cacheRecord
	encoded, err := os.ReadFile(filepath.Join(entry, "record.json"))
	if err != nil || json.Unmarshal(encoded, &record) != nil || record.Version != 1 ||
		record.BuildIdentity != buildIdentity || len(record.Digest) != sha256.Size*2 {
		return nil
	}
	path := filepath.Join(entry, "metasystem")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil
	}
	sum, err := digest.FileSHA256(path)
	if err != nil || sum != record.Digest {
		return nil
	}
	return &Engine{Path: path, Digest: sum, Commit: buildIdentity}
}

// CopyArtifact copies an engine executable to a new read-and-execute-only
// file, synced before it is closed.
func CopyArtifact(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o500)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}

// Build materializes the exact project tree, stamps its
// synthetic candidate commit into one proof build, and leaves bin/metasystem
// untouched. The output survives worktree cleanup for the authenticated
// worker and every detached group it launches.
func Build(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, options ...IO) (*Engine, error) {
	io := selected(options)
	scratch := proofrun.ScratchRunFromContext(ctx)
	open := io.Open
	if scratch != nil && io.Native {
		// The tuple is recorded before git worktree add; Git children inherit
		// the writer lock and run with repository hooks off.
		open = func(workspace gittree.Workspace, tree string) (DetachedWorkspace, error) {
			plan, err := scratch.PlanWorktree(workspace, "engine")
			if err != nil {
				return nil, err
			}
			return plan.Create(tree)
		}
	}
	detached, err := open(workspace, candidateTree)
	if err != nil {
		return nil, fmt.Errorf("candidate engine build failed while materializing tree %s: %w", candidateTree, err)
	}
	removeDetached := func() error { return detached.Close() }
	candidateCommit, err := bindMaterializedCommit(ctx, detached.Workspace(), installationPrefix, environment, io)
	if err != nil {
		closeErr := removeDetached()
		return nil, fmt.Errorf("candidate engine build failed while resolving the materialized candidate commit: %v (cleanup: %v)", err, closeErr)
	}
	outputParent := ""
	if scratch != nil {
		outputParent = scratch.Dir("engine")
	}
	directory, err := os.MkdirTemp(outputParent, "metasystem-candidate-engine.*")
	if err != nil {
		closeErr := removeDetached()
		return nil, fmt.Errorf("candidate engine build failed while allocating its private output: %v (cleanup: %v)", err, closeErr)
	}
	build := &Engine{Path: filepath.Join(directory, "metasystem"), Commit: candidateCommit, directory: directory}
	buildArgv := io.BuildArgv
	if buildArgv == nil {
		buildArgv = DefaultBuildArgv
	}
	argv := buildArgv(build.Path)
	fail := func(cause error, output []byte) (*Engine, error) {
		closeErr := removeDetached()
		removeErr := build.Close()
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			cause = fmt.Errorf("%w: %s", cause, detail)
		}
		if closeErr != nil || removeErr != nil {
			cause = fmt.Errorf("%w (cleanup: worktree=%v output=%v)", cause, closeErr, removeErr)
		}
		return nil, fmt.Errorf("candidate engine build failed at commit %s through %s: %w", candidateCommit, strings.Join(argv, " "), cause)
	}
	installationRoot := filepath.Join(detached.Workspace().Dir, filepath.FromSlash(installationPrefix))
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = installationRoot
	// The compiler caches are the resolved machine engine cache, set
	// explicitly; module and user caches stay inherited.
	carried, err := cachedomain.Carry(BuildEnvironment(environment, candidateCommit), "")
	if err != nil {
		return nil, fmt.Errorf("candidate engine build at commit %s: %w", candidateCommit, err)
	}
	command.Env = carried
	if scratch != nil {
		// The run's temp lives in its root.
		command.Env = append(command.Env, "GOTMPDIR="+scratch.Dir("engine"), "TMPDIR="+scratch.Dir("engine"))
	}
	proofrun.AttachHostResourceLease(ctx, command)
	command.WaitDelay = 5 * time.Second
	var combined bytes.Buffer
	command.Stdout, command.Stderr = &combined, &combined
	commandErr := proofrun.RunResourceCommand(ctx, command, proofrun.HostResourceLeaseFromContext(ctx))
	output := combined.Bytes()
	if commandErr != nil {
		return fail(commandErr, output)
	}
	if err := removeDetached(); err != nil {
		_ = build.Close()
		return nil, fmt.Errorf("candidate engine build failed while cleaning its materialized worktree: %w", err)
	}
	info, err := os.Stat(build.Path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		_ = build.Close()
		return nil, fmt.Errorf("candidate engine build failed: %s did not produce a regular executable: %v", strings.Join(argv, " "), err)
	}
	build.Digest, err = digest.FileSHA256(build.Path)
	if err != nil {
		_ = build.Close()
		return nil, fmt.Errorf("candidate engine build failed while hashing its proof output: %w", err)
	}
	return build, nil
}

// BuildIdentity is the synthetic commit naming the candidate's engine
// projection, toolchain closure, build environment and platform, through
// the real Git seams.
func BuildIdentity(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string) (string, error) {
	return BuildIdentityUsing(ctx, workspace, installationPrefix, candidateTree, environment, Native())
}

// BuildIdentityUsing is BuildIdentity through io's Git seam.
func BuildIdentityUsing(ctx context.Context, workspace gittree.Workspace, installationPrefix, candidateTree string, environment []string, io IO) (string, error) {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return "", err
	}
	installationPrefix = strings.Trim(filepath.ToSlash(installationPrefix), "/")
	installationTree := candidateTree
	if installationPrefix != "" {
		installationTree, err = workspace.ResolveTree(candidateTree + ":" + installationPrefix)
		if err != nil {
			return "", fmt.Errorf("resolve candidate installation subtree: %w", err)
		}
	}
	engineTree, err := projectionTree(ctx, workspace.Dir, installationTree, policy.EnginePaths, io)
	if err != nil {
		return "", err
	}
	buildEnvironment := BuildEnvironment(semanticEnvironment(environment), "")
	// The build script receives this explicit environment. Its tracked bytes
	// can consume any supplied value, so the build key covers all of them.
	environmentDigest := digest.SHA256([]byte(strings.Join(buildEnvironment, "\x00")))
	installationRoot := workspace.Dir
	if installationPrefix != "" {
		installationRoot = filepath.Join(workspace.Dir, filepath.FromSlash(installationPrefix))
	}
	toolchainClosure, err := proofrun.ToolchainClosureIdentity(installationRoot, buildEnvironment)
	if err != nil {
		return "", fmt.Errorf("candidate engine toolchain closure: %w", err)
	}
	message := strings.Join([]string{
		"stable candidate proof snapshot",
		"engine-tree=" + engineTree,
		"toolchain-closure=" + toolchainClosure,
		"build-environment=" + environmentDigest,
		"platform=" + runtime.GOOS + "/" + runtime.GOARCH,
		"build-context=CGO_ENABLED=0,GOAMD64=v1,GOARM64=v8.0,GOARM=7,GOENV=off,GOEXPERIMENT=,GOFLAGS=-mod=readonly,GOTOOLCHAIN=local,GOWORK=off,-buildvcs=false,-trimpath",
	}, "\n")
	return commitTree(ctx, workspace.Dir, engineTree, message, io)
}

// scratchGitCommand is a native Git child of a managed run: hooks off and the
// run's writer lock inherited. Unmanaged callers get the plain command.
func scratchGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	scratch := proofrun.ScratchRunFromContext(ctx)
	if scratch == nil {
		return exec.CommandContext(ctx, "git", args...)
	}
	command := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=" + scratch.Dir("no-hooks")}, args...)...)
	command.ExtraFiles = []*os.File{scratch.Writer()}
	return command
}

func projectionTree(ctx context.Context, root, tree string, paths []string, io IO) (string, error) {
	parent := ""
	if scratch := proofrun.ScratchRunFromContext(ctx); scratch != nil {
		// A killed run leaves its projection indexes inside its own root.
		parent = scratch.Dir("engine")
	}
	directory, err := os.MkdirTemp(parent, "metasystem-engine-projection.*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	run := func(index string, stdin []byte, args ...string) ([]byte, error) {
		command := scratchGitCommand(ctx, append([]string{"-C", root, "-c", "core.fileMode=true", "-c", "core.useReplaceRefs=false"}, args...)...)
		command.Env = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + index)
		command.Stdin = bytes.NewReader(stdin)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		if err := io.RunGit(command); err != nil {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	}
	sourceIndex := filepath.Join(directory, "source-index")
	targetIndex := filepath.Join(directory, "target-index")
	if _, err := run(sourceIndex, nil, "read-tree", tree); err != nil {
		return "", fmt.Errorf("seed candidate engine projection: %w", err)
	}
	entries, err := run(sourceIndex, nil, append([]string{"ls-files", "-s", "-z", "--"}, paths...)...)
	if err != nil {
		return "", fmt.Errorf("enumerate candidate engine projection: %w", err)
	}
	if _, err := run(targetIndex, nil, "read-tree", "--empty"); err != nil {
		return "", fmt.Errorf("initialize candidate engine projection: %w", err)
	}
	if len(entries) > 0 {
		if _, err := run(targetIndex, entries, "update-index", "-z", "--index-info"); err != nil {
			return "", fmt.Errorf("write candidate engine projection: %w", err)
		}
	}
	output, err := run(targetIndex, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write candidate engine tree: %w", err)
	}
	engineTree := strings.TrimSpace(string(output))
	if len(engineTree) != 40 && len(engineTree) != 64 {
		return "", fmt.Errorf("candidate engine projection returned invalid tree %q", engineTree)
	}
	return engineTree, nil
}

func commitTree(ctx context.Context, root, tree, message string, io IO) (string, error) {
	gitLine := func(environment []string, args ...string) (string, error) {
		command := scratchGitCommand(ctx, append([]string{"-C", root}, args...)...)
		command.Env = environment
		var combined bytes.Buffer
		command.Stdout, command.Stderr = &combined, &combined
		err := io.RunGit(command)
		output := combined.Bytes()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return strings.TrimSpace(string(output)), nil
	}
	environment := gittree.ScrubbedEnviron()
	commitEnvironment := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
			"GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE":
			continue
		}
		commitEnvironment = append(commitEnvironment, entry)
	}
	commitEnvironment = append(commitEnvironment,
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	commit, err := gitLine(commitEnvironment, "-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid",
		"-c", "author.name=MetaSystem", "-c", "author.email=metasystem@invalid",
		"-c", "committer.name=MetaSystem", "-c", "committer.email=metasystem@invalid",
		"-c", "i18n.commitEncoding=UTF-8", "commit-tree", tree, "-m", message)
	if err != nil {
		return "", err
	}
	return commit, nil
}

func bindMaterializedCommit(ctx context.Context, workspace gittree.Workspace, installationPrefix string, environment []string, io IO) (string, error) {
	root := workspace.Dir
	tree, err := workspace.HeadTree()
	if err != nil {
		return "", err
	}
	commit, err := BuildIdentityUsing(ctx, workspace, installationPrefix, tree, environment, io)
	if err != nil {
		return "", err
	}
	current, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return "", fmt.Errorf("resolve temporary candidate HEAD: %v", err)
	}
	args := []string{"-C", root}
	var inherit []*os.File
	if materialize := workspace.Materialize; materialize != nil {
		// A materializing child inherits the run's writer lock and runs no
		// repository hook (reference-transaction included).
		if materialize.HooksPath != "" {
			args = append(args, "-c", "core.hooksPath="+materialize.HooksPath)
		}
		inherit = materialize.InheritFiles
	}
	command := exec.CommandContext(ctx, "git", append(args, "update-ref", "--no-deref", "HEAD", commit, current)...)
	command.Env = gittree.ScrubbedEnviron()
	command.ExtraFiles = inherit
	var combined bytes.Buffer
	command.Stdout, command.Stderr = &combined, &combined
	if err := io.RunGit(command); err != nil {
		output := combined.Bytes()
		return "", fmt.Errorf("git update-ref --no-deref HEAD: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return commit, nil
}
