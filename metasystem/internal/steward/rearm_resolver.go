package steward

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

const (
	landingRefConfigKey = "metasystem.steward.landing-ref"
	// rearm-resolve-seconds is the longest an external judgment step may be silent.
	rearmResolveSecondsConfig  = "metasystem.steward.rearm-resolve-seconds"
	defaultRearmResolveSeconds = 20
	witnessDigestCacheName     = "rearm-witness-digests.json"
)

type witnessDigestCache struct {
	Entries map[string]string `json:"entries"`
}

// Each rearm resolution carries its own repository answers. The default
// binds the existing Git commands; callers may supply isolated answers.
type rearmResolverDeps struct {
	resolveSeconds       func(string) int
	localLandingRef      func(string) (string, error)
	resolvingRef         func(string, string) (string, error)
	checkoutHead         func(string) (string, error)
	deadlineGit          func(RearmClock, int, string, string, ...string) (string, error)
	witnessGit           func(context.Context, string, []byte, func(), ...string) ([]byte, error)
	witnessTreeDigest    func(context.Context, string, string, behaviorsurface.Policy, RearmClock, int) (string, error)
	writeWitnessCache    func(string, string, map[string]string)
	projectionDiff       func(context.Context, string, string, string, func()) ([]byte, error)
	archivedEngineDigest func(context.Context, string, string, behaviorsurface.Policy, RearmClock, int) (string, error)
	notifyAvailable      func(string) bool
	runnerExcluded       func(string, bool) (string, bool)
}

func defaultRearmResolverDeps() rearmResolverDeps {
	return rearmResolverDeps{
		resolveSeconds: RearmResolveSeconds,
		localLandingRef: func(root string) (string, error) {
			return gitOutputContext(context.Background(), root, "config", "--local", "--no-includes", "--get", landingRefConfigKey)
		},
		resolvingRef: func(root, ref string) (string, error) {
			return gitOutputContext(context.Background(), root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		},
		checkoutHead: func(root string) (string, error) {
			return gitOutputContext(context.Background(), root, "rev-parse", "--verify", "HEAD^{commit}")
		},
		deadlineGit:          gitOutputWithProgressDeadline,
		witnessGit:           runWitnessGitCommand,
		witnessTreeDigest:    digestArchivedTree,
		writeWitnessCache:    writeWitnessDigestCache,
		projectionDiff:       diffEngineProjection,
		archivedEngineDigest: archivedEngineDigestAtCommitWithClock,
		notifyAvailable:      func(root string) bool { _, ok := NotifyCommand(root); return ok },
		runnerExcluded:       runnerExclusion,
	}
}

type classifiedJudgmentError struct {
	message string
	cause   error
}

func (e *classifiedJudgmentError) Error() string { return e.message }
func (e *classifiedJudgmentError) Unwrap() error { return e.cause }

type gitCommandFailure struct {
	cause error
}

func (e *gitCommandFailure) Error() string { return e.cause.Error() }
func (e *gitCommandFailure) Unwrap() error { return e.cause }

// Judgment classes let callers distinguish a negative ownership verdict from
// a stalled decision without coupling that choice to message text.
var (
	ErrNotOwned          = errors.New("destination is not owned")
	ErrProjectionDiffers = fmt.Errorf("ENGINE projection differs: %w", ErrNotOwned)
	ErrJudgmentStalled   = errors.New("steward judgment stalled")
)

func classifiedJudgment(message string, cause error) error {
	return &classifiedJudgmentError{message: message, cause: cause}
}

func gitSaidNo(err error) bool {
	var status interface{ ExitCode() int }
	return errors.As(err, &status) && status.ExitCode() == 1
}

var (
	commitBuildStamp  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	witnessBuildStamp = regexp.MustCompile(`^witness-([0-9a-f]{12})$`)
	witnessCacheKey   = regexp.MustCompile(`^[0-9]+:[0-9a-f]{40}([0-9a-f]{24})?$`)
	witnessDigest     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	witnessObjectID   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	projectionHeader  = regexp.MustCompile(`^:([0-7]{6}) ([0-7]{6}) ([0-9a-fA-F]{40}|[0-9a-fA-F]{64}) ([0-9a-fA-F]{40}|[0-9a-fA-F]{64}) ([A-Za-z]+)$`)
)

// RearmResolveSeconds reads the witness-history deadline in the same
// forgiving shape as the runner cadence: a positive integer wins and every
// absent or malformed value falls back to the default.
func RearmResolveSeconds(installationRoot string) int {
	out, err := exec.Command("git", "-C", installationRoot, "config", "--get", rearmResolveSecondsConfig).Output()
	if err == nil {
		if seconds, parseErr := strconv.Atoi(strings.TrimSpace(string(out))); parseErr == nil && seconds > 0 {
			return seconds
		}
	}
	return defaultRearmResolveSeconds
}

func readOwnedLandingRef(installationRoot string) (string, error) {
	return readOwnedLandingRefWithDeps(defaultRearmResolverDeps(), installationRoot)
}

func readOwnedLandingRefWithDeps(deps rearmResolverDeps, installationRoot string) (string, error) {
	value, err := deps.localLandingRef(installationRoot)
	if err != nil {
		if !gitSaidNo(err) {
			return "", &gitCommandFailure{cause: err}
		}
		value = ""
	}
	if value == "" {
		shown := value
		if shown == "" {
			shown = "<unset>"
		}
		return "", fmt.Errorf("the installation owns no remote-tracking landing ref (%s is %s; expected refs/remotes/<remote>/<branch>)", landingRefConfigKey, shown)
	}
	tail := strings.TrimPrefix(value, "refs/remotes/")
	remote, branch, qualified := strings.Cut(tail, "/")
	if tail == value || !qualified || remote == "" || branch == "" {
		return "", fmt.Errorf("the installation owns no remote-tracking landing ref (%s is %s; expected refs/remotes/<remote>/<branch>)", landingRefConfigKey, value)
	}
	if _, err := deps.resolvingRef(installationRoot, value); err != nil {
		message := fmt.Sprintf("the installation owns no resolving remote-tracking landing ref (%s is %s)", landingRefConfigKey, value)
		if gitSaidNo(err) {
			return "", classifiedJudgment(message, err)
		}
		return "", &gitCommandFailure{cause: err}
	}
	return value, nil
}

func gitOutputContext(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitOutputWithProgressDeadline(clock RearmClock, seconds int, step, root string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := RunRearmStep(context.Background(), clock, time.Duration(seconds)*time.Second, step, func(ctx context.Context, progress func()) error {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		cmd.Stdout = RearmProgressWriter(&stdout, progress)
		cmd.Stderr = RearmProgressWriter(&stderr, progress)
		return cmd.Run()
	})
	if err != nil {
		if errors.Is(err, ErrJudgmentStalled) {
			return "", err
		}
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

type witnessCandidate struct {
	commit, tree, firstParent string
	changes                   []projectionDiffEntry
}

func runWitnessGitCommand(ctx context.Context, root string, input []byte, progress func(), args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = RearmProgressWriter(&stdout, progress), RearmProgressWriter(&stderr, progress)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func parseWitnessLog(raw []byte) ([]witnessCandidate, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	parts := bytes.Split(raw, []byte{0})
	if len(parts[len(parts)-1]) != 0 {
		return nil, fmt.Errorf("parse witness history: output is not NUL-terminated")
	}
	parts = parts[:len(parts)-1]
	var candidates []witnessCandidate
	for position := 0; position < len(parts); {
		header := parts[position]
		if len(header) == 0 || header[0] != 1 {
			return nil, fmt.Errorf("parse witness history: expected commit header, got %q", header)
		}
		fields := strings.Fields(string(header[1:]))
		if len(fields) < 2 || !witnessObjectID.MatchString(fields[0]) || !witnessObjectID.MatchString(fields[1]) {
			return nil, fmt.Errorf("parse witness history: malformed commit header %q", header)
		}
		for _, parent := range fields[2:] {
			if !witnessObjectID.MatchString(parent) {
				return nil, fmt.Errorf("parse witness history: malformed parent %q", parent)
			}
		}
		candidate := witnessCandidate{commit: fields[0], tree: fields[1]}
		if len(fields) > 2 {
			candidate.firstParent = fields[2]
		}
		position++
		for position < len(parts) && (len(parts[position]) == 0 || parts[position][0] != 1) {
			rawHeader := bytes.TrimPrefix(parts[position], []byte{'\n'})
			match := projectionHeader.FindSubmatch(rawHeader)
			if match == nil || len(match[3]) != len(match[4]) {
				return nil, fmt.Errorf("parse witness history: malformed raw entry %q", parts[position])
			}
			position++
			if position >= len(parts) || len(parts[position]) == 0 {
				return nil, fmt.Errorf("parse witness history: missing NUL-terminated path")
			}
			candidate.changes = append(candidate.changes, projectionDiffEntry{
				oldMode: string(match[1]), newMode: string(match[2]),
				oldID: string(match[3]), newID: string(match[4]), path: string(parts[position]),
			})
			position++
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

// witnessGitStep runs one git command under the silence bound: every byte
// the command writes counts as progress, so a long but live walk is not a stall.
func witnessGitStep(clock RearmClock, seconds int, step, root string, input []byte, args ...string) ([]byte, error) {
	return witnessGitStepWithDeps(defaultRearmResolverDeps(), clock, seconds, step, root, input, args...)
}

func witnessGitStepWithDeps(deps rearmResolverDeps, clock RearmClock, seconds int, step, root string, input []byte, args ...string) ([]byte, error) {
	var out []byte
	err := RunRearmStep(context.Background(), clock, time.Duration(seconds)*time.Second, step, func(stepContext context.Context, progress func()) error {
		var runErr error
		out, runErr = deps.witnessGit(stepContext, root, input, progress, args...)
		return runErr
	})
	return out, err
}

func readWitnessCandidates(clock RearmClock, seconds int, installationRoot, ref string) ([]witnessCandidate, error) {
	return readWitnessCandidatesWithDeps(defaultRearmResolverDeps(), clock, seconds, installationRoot, ref)
}

func readWitnessCandidatesWithDeps(deps rearmResolverDeps, clock RearmClock, seconds int, installationRoot, ref string) ([]witnessCandidate, error) {
	args := []string{"log", "--topo-order", "--no-abbrev", "--raw", "-z", "--no-renames", "--no-ext-diff", "--ignore-submodules=none", "--diff-merges=first-parent", "--format=%x01%H %T %P", ref}
	raw, err := witnessGitStepWithDeps(deps, clock, seconds, "list-witness-history", installationRoot, nil, args...)
	if err != nil {
		return nil, err
	}
	return parseWitnessLog(raw)
}

func readWitnessPrefixTrees(clock RearmClock, seconds int, installationRoot, prefix string, candidates []witnessCandidate) error {
	return readWitnessPrefixTreesWithDeps(defaultRearmResolverDeps(), clock, seconds, installationRoot, prefix, candidates)
}

func readWitnessPrefixTreesWithDeps(deps rearmResolverDeps, clock RearmClock, seconds int, installationRoot, prefix string, candidates []witnessCandidate) error {
	var input strings.Builder
	for _, candidate := range candidates {
		fmt.Fprintf(&input, "%s:%s\n", candidate.commit, prefix)
	}
	out, err := witnessGitStepWithDeps(deps, clock, seconds, "resolve-witness-trees", installationRoot, []byte(input.String()), "cat-file", "--batch-check=%(objectname)")
	if err != nil {
		return err
	}
	lines := bytes.Split(bytes.TrimSuffix(out, []byte{'\n'}), []byte{'\n'})
	if len(lines) != len(candidates) {
		return fmt.Errorf("resolve nested witness trees: got %d batch results for %d candidates", len(lines), len(candidates))
	}
	for index, line := range lines {
		if !witnessObjectID.Match(line) {
			return fmt.Errorf("resolve nested witness tree for %s:%s: %s", candidates[index].commit, prefix, line)
		}
		candidates[index].tree = string(line)
	}
	return nil
}

func witnessProjectionClasses(candidates []witnessCandidate, policy behaviorsurface.Policy, prefix string) ([]int, map[int][]int, error) {
	classByCommit := make(map[string]int, len(candidates))
	classes := make([]int, len(candidates))
	nextClass := 0
	for index := len(candidates) - 1; index >= 0; index-- {
		changedPath, undecidable, err := projectionChange(candidates[index].changes, policy, prefix)
		if err != nil {
			return nil, nil, err
		}
		parentClass, parentKnown := classByCommit[candidates[index].firstParent]
		if changedPath == "" && !undecidable && candidates[index].firstParent != "" && parentKnown {
			classes[index] = parentClass
		} else {
			classes[index] = nextClass
			nextClass++
		}
		classByCommit[candidates[index].commit] = classes[index]
	}
	members := make(map[int][]int, nextClass)
	for index, class := range classes {
		members[class] = append(members[class], index)
	}
	return classes, members, nil
}

// resolveWitnessStamp searches the complete reachable history newest first.
// First-parent edges that preserve the ENGINE projection form classes, and
// every member of a class shares one cached or computed digest.
func resolveWitnessStamp(repoRoot, installationRoot, ref, hex12 string) (string, error) {
	return resolveWitnessStampWithClock(SystemRearmClock(), repoRoot, installationRoot, ref, hex12)
}

func resolveWitnessStampWithClock(clock RearmClock, repoRoot, installationRoot, ref, hex12 string) (string, error) {
	return resolveWitnessStampWithDeps(defaultRearmResolverDeps(), clock, repoRoot, installationRoot, ref, hex12)
}

func resolveWitnessStampWithDeps(deps rearmResolverDeps, clock RearmClock, repoRoot, installationRoot, ref, hex12 string) (string, error) {
	seconds := deps.resolveSeconds(installationRoot)
	toplevelBytes, err := witnessGitStepWithDeps(deps, clock, seconds, "resolve-toplevel", installationRoot, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	toplevel := strings.TrimSpace(string(toplevelBytes))
	prefix, err := filepath.Rel(toplevel, installationRoot)
	if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("installation root %q is outside git toplevel %q", installationRoot, toplevel)
	}
	if prefix == "." {
		prefix = ""
	}
	prefix = filepath.ToSlash(prefix)
	policy, err := behaviorsurface.Load()
	if err != nil {
		return "", err
	}
	cachePath := filepath.Join(repoRoot, "artifacts", "agents", "steward", witnessDigestCacheName)
	digests := readWitnessDigestCache(cachePath)
	finish := func(commit string, resultErr error) (string, error) {
		deps.writeWitnessCache(cachePath, repoRoot, digests)
		return commit, resultErr
	}
	candidates, err := readWitnessCandidatesWithDeps(deps, clock, seconds, installationRoot, ref)
	if err != nil {
		return finish("", err)
	}
	if prefix != "" {
		if err := readWitnessPrefixTreesWithDeps(deps, clock, seconds, installationRoot, prefix, candidates); err != nil {
			return finish("", err)
		}
	}
	classes, members, err := witnessProjectionClasses(candidates, policy, prefix)
	if err != nil {
		return finish("", err)
	}
	classDigests := make(map[int]string, len(members))
	for index, candidate := range candidates {
		class := classes[index]
		digest, ready := classDigests[class]
		if !ready {
			for _, member := range members[class] {
				cacheKey := fmt.Sprintf("%d:%s", policy.Version, candidates[member].tree)
				if digest, ready = digests[cacheKey]; ready {
					break
				}
			}
			if !ready {
				newest := candidates[members[class][0]]
				archiveSpec := newest.commit
				if prefix != "" {
					archiveSpec += ":" + prefix
				}
				digest, err = deps.witnessTreeDigest(context.Background(), toplevel, archiveSpec, policy, clock, seconds)
				if err != nil {
					return finish("", err)
				}
			}
			for _, member := range members[class] {
				cacheKey := fmt.Sprintf("%d:%s", policy.Version, candidates[member].tree)
				digests[cacheKey] = digest
			}
			classDigests[class] = digest
		}
		if strings.HasPrefix(digest, hex12) {
			return finish(candidate.commit, nil)
		}
	}
	return finish("", classifiedJudgment(fmt.Sprintf("witness digest matches no commit reachable from %s", ref), ErrNotOwned))
}

func readWitnessDigestCache(path string) map[string]string {
	cache := witnessDigestCache{}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &cache) != nil || cache.Entries == nil {
		return map[string]string{}
	}
	for key, digest := range cache.Entries {
		if !witnessCacheKey.MatchString(key) || !witnessDigest.MatchString(digest) {
			return map[string]string{}
		}
	}
	return cache.Entries
}

func writeWitnessDigestCache(path, repoRoot string, entries map[string]string) {
	data, err := json.MarshalIndent(witnessDigestCache{Entries: entries}, "", "  ")
	if err != nil {
		return
	}
	_, _ = atomicfile.WriteText(path, string(append(data, '\n')), repoRoot)
}

func digestArchivedTree(ctx context.Context, toplevel, tree string, policy behaviorsurface.Policy, clock RearmClock, seconds int) (string, error) {
	directory, err := os.MkdirTemp("", "metasystem-rearm-witness-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", err
	}
	archivePath := filepath.Join(directory, "tree.tar")
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	archiveCommand := exec.CommandContext(ctx, "git", "-C", toplevel, "archive", "--format=tar", tree)
	var archiveError bytes.Buffer
	archiveErr := RunRearmStep(ctx, clock, time.Duration(seconds)*time.Second, "archive-witness-tree", func(stepContext context.Context, progress func()) error {
		archiveCommand = exec.CommandContext(stepContext, "git", "-C", toplevel, "archive", "--format=tar", tree)
		archiveCommand.Stdout = RearmProgressWriter(archive, progress)
		archiveCommand.Stderr = RearmProgressWriter(&archiveError, progress)
		return archiveCommand.Run()
	})
	closeErr := archive.Close()
	if archiveErr != nil {
		return "", fmt.Errorf("git archive %s: %w (%s)", tree, archiveErr, strings.TrimSpace(archiveError.String()))
	}
	if closeErr != nil {
		return "", closeErr
	}
	extract := filepath.Join(directory, "extract")
	if err := os.Mkdir(extract, 0o700); err != nil {
		return "", err
	}
	var extractOutput bytes.Buffer
	extractErr := RunRearmStep(ctx, clock, time.Duration(seconds)*time.Second, "extract-witness-tree", func(stepContext context.Context, progress func()) error {
		cmd := exec.CommandContext(stepContext, "tar", "-xvf", archivePath, "-C", extract)
		cmd.Stdout = RearmProgressWriter(&extractOutput, progress)
		cmd.Stderr = RearmProgressWriter(&extractOutput, progress)
		return cmd.Run()
	})
	if extractErr != nil {
		return "", fmt.Errorf("extract archived tree: %w (%s)", extractErr, strings.TrimSpace(extractOutput.String()))
	}
	return policy.Digest(extract, behaviorsurface.Engine)
}

func resolveLandedBuild(repoRoot, installationRoot, landingRef, stamp string) (string, error) {
	return resolveLandedBuildWithClock(SystemRearmClock(), repoRoot, installationRoot, landingRef, stamp)
}

func resolveLandedBuildWithClock(clock RearmClock, repoRoot, installationRoot, landingRef, stamp string) (string, error) {
	return resolveLandedBuildWithDeps(defaultRearmResolverDeps(), clock, repoRoot, installationRoot, landingRef, stamp)
}

func resolveLandedBuildWithDeps(deps rearmResolverDeps, clock RearmClock, repoRoot, installationRoot, landingRef, stamp string) (string, error) {
	commit := ""
	seconds := deps.resolveSeconds(installationRoot)
	switch {
	case commitBuildStamp.MatchString(stamp):
		out, err := deps.deadlineGit(clock, seconds, "resolve-build-stamp", installationRoot, "rev-parse", "--verify", "--quiet", stamp+"^{commit}")
		if err != nil {
			if errors.Is(err, ErrJudgmentStalled) {
				return "", err
			}
			if gitSaidNo(err) {
				return "", classifiedJudgment(fmt.Sprintf("rebuilt engine carries unresolved build stamp %q; automatic re-arm is bounded to landed commits", stamp), ErrNotOwned)
			}
			return "", err
		}
		commit = out
	case witnessBuildStamp.MatchString(stamp):
		var err error
		commit, err = resolveWitnessStampWithDeps(deps, clock, repoRoot, installationRoot, landingRef, witnessBuildStamp.FindStringSubmatch(stamp)[1])
		if err != nil {
			return "", fmt.Errorf("rebuilt engine was built from %s, which is not proven landed on %s: %w", stamp, landingRef, err)
		}
	default:
		shown := stamp
		if shown == "" {
			shown = "<unreadable>"
		}
		return "", classifiedJudgment(fmt.Sprintf("rebuilt engine carries build stamp %s; automatic re-arm is bounded to landed commits", shown), ErrNotOwned)
	}
	if _, err := deps.deadlineGit(clock, seconds, "compare-build-ancestry", installationRoot, "merge-base", "--is-ancestor", commit, landingRef); err != nil {
		if gitSaidNo(err) {
			return "", classifiedJudgment(fmt.Sprintf("rebuilt engine was built from %s, which is not landed on %s", stamp, landingRef), ErrNotOwned)
		}
		return "", err
	}
	return commit, nil
}

var enrollmentSkewPathspecs = [...]string{"internal", "cmd", "scripts/agents"}

func verifyEnrollmentLandedSource(installationRoot, sourceCommit, landedCommit string) error {
	return verifyEnrollmentLandedSourceWithClock(SystemRearmClock(), installationRoot, sourceCommit, landedCommit)
}

func verifyEnrollmentLandedSourceWithClock(clock RearmClock, installationRoot, sourceCommit, landedCommit string) error {
	return verifyEnrollmentLandedSourceWithDeps(defaultRearmResolverDeps(), clock, installationRoot, sourceCommit, landedCommit)
}

func verifyEnrollmentLandedSourceWithDeps(deps rearmResolverDeps, clock RearmClock, installationRoot, sourceCommit, landedCommit string) error {
	if sourceCommit == landedCommit {
		return nil
	}
	seconds := deps.resolveSeconds(installationRoot)
	_, ancestorErr := deps.deadlineGit(clock, seconds, "compare-enrollment-ancestry", installationRoot, "merge-base", "--is-ancestor", sourceCommit, landedCommit)
	if ancestorErr != nil {
		if gitSaidNo(ancestorErr) {
			return classifiedJudgment(fmt.Sprintf("enrollment records landed source %q but executable stamp source %q is not its ancestor", landedCommit, sourceCommit), ErrNotOwned)
		}
		return ancestorErr
	}
	args := []string{"log", "--format=", "--name-only", "--ancestry-path", "--diff-merges=first-parent", sourceCommit + ".." + landedCommit, "--"}
	args = append(args, enrollmentSkewPathspecs[:]...)
	changedPaths, err := deps.deadlineGit(clock, seconds, "read-enrollment-skew", installationRoot, args...)
	if err != nil {
		return fmt.Errorf("read enrollment engine-and-agent-script skew: %w", err)
	}
	if changedPaths != "" {
		return classifiedJudgment(fmt.Sprintf("enrollment records landed source %q but engine or agent scripts changed after executable stamp source %q", landedCommit, sourceCommit), ErrNotOwned)
	}
	return nil
}

func verifyEnrollmentBuildSource(installationRoot, stamp, sourceCommit, landedCommit string) error {
	return verifyEnrollmentBuildSourceWithClock(SystemRearmClock(), installationRoot, stamp, sourceCommit, landedCommit)
}

func verifyEnrollmentBuildSourceWithClock(clock RearmClock, installationRoot, stamp, sourceCommit, landedCommit string) error {
	return verifyEnrollmentBuildSourceWithDeps(defaultRearmResolverDeps(), clock, installationRoot, stamp, sourceCommit, landedCommit)
}

func verifyEnrollmentBuildSourceWithDeps(deps rearmResolverDeps, clock RearmClock, installationRoot, stamp, sourceCommit, landedCommit string) error {
	err := verifyEnrollmentLandedSourceWithDeps(deps, clock, installationRoot, sourceCommit, landedCommit)
	if err == nil || !witnessBuildStamp.MatchString(stamp) || !errors.Is(err, ErrNotOwned) {
		return err
	}
	// A witness names ENGINE content rather than one commit. Its resolver
	// deliberately chooses the newest matching commit, which may be a
	// ledger-only descendant of the commit recorded by enrollment.
	if reverseErr := verifyEnrollmentLandedSourceWithDeps(deps, clock, installationRoot, landedCommit, sourceCommit); reverseErr == nil {
		return nil
	}
	return err
}

type projectionDiffEntry struct {
	oldMode, newMode string
	oldID, newID     string
	path             string
}

func parseProjectionDiff(raw []byte) ([]projectionDiffEntry, error) {
	var entries []projectionDiffEntry
	for len(raw) > 0 {
		header, rest, found := bytes.Cut(raw, []byte{0})
		if !found {
			return nil, fmt.Errorf("parse ENGINE projection diff: truncated header")
		}
		match := projectionHeader.FindSubmatch(header)
		if match == nil || len(match[3]) != len(match[4]) {
			return nil, fmt.Errorf("parse ENGINE projection diff: malformed header %q", header)
		}
		path, tail, found := bytes.Cut(rest, []byte{0})
		if !found || len(path) == 0 {
			return nil, fmt.Errorf("parse ENGINE projection diff: missing NUL-terminated path")
		}
		entries = append(entries, projectionDiffEntry{string(match[1]), string(match[2]), string(match[3]), string(match[4]), string(path)})
		raw = tail
	}
	return entries, nil
}

func projectionKind(mode string) (byte, bool) {
	switch mode {
	case "000000":
		return 'a', true
	case "100644", "100755":
		return 'f', true
	case "120000":
		return 's', true
	case "160000":
		return 'g', true
	default:
		return 0, false
	}
}

func projectionChange(entries []projectionDiffEntry, policy behaviorsurface.Policy, prefix string) (string, bool, error) {
	for _, entry := range entries {
		oldKind, oldKnown := projectionKind(entry.oldMode)
		newKind, newKnown := projectionKind(entry.newMode)
		if !oldKnown || !newKnown || oldKind == 'g' || newKind == 'g' {
			return "", true, nil
		}
	}
	for _, entry := range entries {
		included, err := policy.Includes(behaviorsurface.Engine, entry.path, prefix)
		if err != nil {
			return "", false, err
		}
		oldKind, _ := projectionKind(entry.oldMode)
		newKind, _ := projectionKind(entry.newMode)
		if included && (oldKind != newKind || entry.oldID != entry.newID) {
			return entry.path, false, nil
		}
	}
	return "", false, nil
}

func diffEngineProjection(ctx context.Context, installationRoot, sourceCommit, destinationCommit string, progress func()) ([]byte, error) {
	args := []string{"diff-tree", "-r", "-z", "--no-commit-id", "--no-abbrev", "--no-renames", "--no-ext-diff", "--ignore-submodules=none", "--relative", sourceCommit, destinationCommit}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", installationRoot}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = RearmProgressWriter(&stdout, progress), RearmProgressWriter(&stderr, progress)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func compareEngineProjection(ctx context.Context, installationRoot, sourceCommit, destinationCommit string, policy behaviorsurface.Policy) error {
	return compareEngineProjectionWithClock(ctx, installationRoot, sourceCommit, destinationCommit, policy, SystemRearmClock(), RearmResolveSeconds(installationRoot))
}

func compareEngineProjectionWithClock(ctx context.Context, installationRoot, sourceCommit, destinationCommit string, policy behaviorsurface.Policy, clock RearmClock, seconds int) error {
	return compareEngineProjectionWithDeps(defaultRearmResolverDeps(), ctx, installationRoot, sourceCommit, destinationCommit, policy, clock, seconds)
}

func compareEngineProjectionWithDeps(deps rearmResolverDeps, ctx context.Context, installationRoot, sourceCommit, destinationCommit string, policy behaviorsurface.Policy, clock RearmClock, seconds int) error {
	var raw []byte
	err := RunRearmStep(ctx, clock, time.Duration(seconds)*time.Second, "compare-engine-projection", func(stepContext context.Context, progress func()) error {
		var diffErr error
		raw, diffErr = deps.projectionDiff(stepContext, installationRoot, sourceCommit, destinationCommit, progress)
		return diffErr
	})
	if err != nil {
		return err
	}
	entries, err := parseProjectionDiff(raw)
	if err != nil {
		return err
	}
	changedPath, undecidable, err := projectionChange(entries, policy, "")
	if err != nil {
		return err
	}
	if !undecidable {
		if changedPath == "" {
			return nil
		}
		message := fmt.Sprintf("enrolled source %s and destination %s have different ENGINE projections (first changed path %q)", sourceCommit, destinationCommit, changedPath)
		return classifiedJudgment(message, ErrProjectionDiffers)
	}
	sourceDigest, err := deps.archivedEngineDigest(ctx, installationRoot, sourceCommit, policy, clock, seconds)
	if err != nil {
		return fmt.Errorf("read enrolled source ENGINE projection: %w", err)
	}
	destinationDigest, err := deps.archivedEngineDigest(ctx, installationRoot, destinationCommit, policy, clock, seconds)
	if err != nil {
		return fmt.Errorf("read destination ENGINE projection: %w", err)
	}
	if sourceDigest != destinationDigest {
		message := fmt.Sprintf("enrolled source %s and destination %s have different ENGINE projections", sourceCommit, destinationCommit)
		return classifiedJudgment(message, ErrProjectionDiffers)
	}
	return nil
}

// VerifySourceAtDestination proves that the pinned enrolled bytes still own
// policy for a captured destination commit. The enrollment remains bound to
// the commit that genuinely supplied the executable; later destination
// commits may reuse it only while the complete ENGINE projection is equal.
func (b *EnrolledBinary) VerifySourceAtDestination(installationRoot, destinationCommit string) error {
	return b.VerifySourceAtDestinationWithClock(SystemRearmClock(), installationRoot, destinationCommit)
}

// VerifySourceAtDestinationWithClock is the injected-clock form used by a
// landed re-arm judgment.
func (b *EnrolledBinary) VerifySourceAtDestinationWithClock(clock RearmClock, installationRoot, destinationCommit string) error {
	return b.verifySourceAtDestinationWithDeps(defaultRearmResolverDeps(), clock, installationRoot, destinationCommit)
}

func (b *EnrolledBinary) verifySourceAtDestinationWithDeps(deps rearmResolverDeps, clock RearmClock, installationRoot, destinationCommit string) error {
	if b == nil || b.file == nil {
		return fmt.Errorf("the enrolled engine is not open")
	}
	if !commitBuildStamp.MatchString(destinationCommit) || len(destinationCommit) != 40 {
		return fmt.Errorf("captured policy destination %q is not a full commit id", destinationCommit)
	}
	stamp := b.BuildStamp()
	sourceCommit, err := resolveLandedBuildWithDeps(deps, clock, b.repoRoot, installationRoot, destinationCommit, stamp)
	if err != nil {
		return err
	}
	// Only an automatic re-arm records landing provenance. A human enrollment
	// still has to prove its actual pinned build stamp, ancestry and complete
	// ENGINE projection below; it does not invent a machine landing record.
	if b.Install.LandedCommit != "" {
		if err := verifyEnrollmentBuildSourceWithDeps(deps, clock, installationRoot, stamp, sourceCommit, b.Install.LandedCommit); err != nil {
			return err
		}
	}
	if b.Install.MintedBy == "machine-rebuild" && (b.Install.LandedCommit == "" || b.Install.LandingRef == "") {
		return fmt.Errorf("machine enrollment is missing its landed source or landing ref")
	}
	if b.Install.LandedCommit == "" && b.Install.LandingRef != "" {
		return fmt.Errorf("enrollment has a landing ref without a landed source")
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return err
	}
	seconds := deps.resolveSeconds(installationRoot)
	err = compareEngineProjectionWithDeps(deps, context.Background(), installationRoot, sourceCommit, destinationCommit, policy, clock, seconds)
	if err != nil {
		return err
	}
	return nil
}

func archivedEngineDigestAtCommit(ctx context.Context, installationRoot, commit string, policy behaviorsurface.Policy) (string, error) {
	return archivedEngineDigestAtCommitWithClock(ctx, installationRoot, commit, policy, SystemRearmClock(), RearmResolveSeconds(installationRoot))
}

func archivedEngineDigestAtCommitWithClock(ctx context.Context, installationRoot, commit string, policy behaviorsurface.Policy, clock RearmClock, seconds int) (string, error) {
	toplevel, err := gitOutputWithProgressDeadline(clock, seconds, "resolve-archive-toplevel", installationRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	toplevel = canonicalPath(toplevel)
	installationRoot = canonicalPath(installationRoot)
	prefix, err := filepath.Rel(toplevel, installationRoot)
	if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("installation root %q is outside git toplevel %q", installationRoot, toplevel)
	}
	archiveSpec := commit
	if prefix != "." {
		archiveSpec = commit + ":" + filepath.ToSlash(prefix)
	}
	return digestArchivedTree(ctx, toplevel, archiveSpec, policy, clock, seconds)
}
