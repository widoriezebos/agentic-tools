package steward

import (
	"bytes"
	"context"
	"encoding/json"
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
	landingRefConfigKey        = "metasystem.steward.landing-ref"
	rearmResolveSecondsConfig  = "metasystem.steward.rearm-resolve-seconds"
	defaultRearmResolveSeconds = 20
	witnessDigestCacheName     = "rearm-witness-digests.json"
)

type witnessDigestCache struct {
	Entries map[string]string `json:"entries"`
}

var (
	witnessTreeDigester      = digestArchivedTree
	witnessDigestCacheWriter = writeWitnessDigestCache
)

var (
	commitBuildStamp  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	witnessBuildStamp = regexp.MustCompile(`^witness-([0-9a-f]{12})$`)
	witnessCacheKey   = regexp.MustCompile(`^[0-9]+:[0-9a-f]{40}([0-9a-f]{24})?$`)
	witnessDigest     = regexp.MustCompile(`^[0-9a-f]{64}$`)
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
	out, err := exec.Command("git", "-C", installationRoot, "config", "--local", "--no-includes", "--get", landingRefConfigKey).Output()
	value := strings.TrimSpace(string(out))
	if err != nil || value == "" {
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
	if _, err := exec.Command("git", "-C", installationRoot, "rev-parse", "--verify", "--quiet", value+"^{commit}").Output(); err != nil {
		return "", fmt.Errorf("the installation owns no resolving remote-tracking landing ref (%s is %s)", landingRefConfigKey, value)
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

func witnessTimeoutError(seconds int) error {
	return fmt.Errorf("witness resolution exceeded the configured %d-second bound (%s)", seconds, rearmResolveSecondsConfig)
}

// resolveWitnessStamp searches the complete reachable history newest first.
// Commits that share a tree share one digest computation, so merge-heavy
// history cannot repeatedly charge the same content walk.
func resolveWitnessStamp(repoRoot, installationRoot, ref, hex12 string) (string, error) {
	seconds := RearmResolveSeconds(installationRoot)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	toplevel, err := gitOutputContext(ctx, installationRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		if ctx.Err() != nil {
			return "", witnessTimeoutError(seconds)
		}
		return "", err
	}
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
		witnessDigestCacheWriter(cachePath, repoRoot, digests)
		return commit, resultErr
	}
	candidateText, err := gitOutputContext(ctx, installationRoot, "rev-list", ref)
	if err != nil {
		if ctx.Err() != nil {
			return finish("", witnessTimeoutError(seconds))
		}
		return finish("", err)
	}
	for _, candidate := range strings.Fields(candidateText) {
		if ctx.Err() != nil {
			return finish("", witnessTimeoutError(seconds))
		}
		treeSpec := candidate + "^{tree}"
		archiveSpec := candidate
		if prefix != "" {
			treeSpec = candidate + ":" + prefix
			archiveSpec = treeSpec
		}
		tree, treeErr := gitOutputContext(ctx, installationRoot, "rev-parse", "--verify", treeSpec)
		if treeErr != nil {
			if ctx.Err() != nil {
				return finish("", witnessTimeoutError(seconds))
			}
			return finish("", treeErr)
		}
		cacheKey := fmt.Sprintf("%d:%s", policy.Version, tree)
		digest, cached := digests[cacheKey]
		if !cached {
			digest, err = witnessTreeDigester(ctx, toplevel, archiveSpec, policy)
			if err != nil {
				if ctx.Err() != nil {
					return finish("", witnessTimeoutError(seconds))
				}
				return finish("", err)
			}
			digests[cacheKey] = digest
		}
		if strings.HasPrefix(digest, hex12) {
			return finish(candidate, nil)
		}
	}
	return finish("", fmt.Errorf("witness digest matches no commit reachable from %s within the %d-second bound", ref, seconds))
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

func digestArchivedTree(ctx context.Context, toplevel, tree string, policy behaviorsurface.Policy) (string, error) {
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
	archiveCommand.Stdout = archive
	var archiveError bytes.Buffer
	archiveCommand.Stderr = &archiveError
	archiveErr := archiveCommand.Run()
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
	cmd := exec.CommandContext(ctx, "tar", "-xf", archivePath, "-C", extract)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("extract archived tree: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return policy.Digest(extract, behaviorsurface.Engine)
}

func resolveLandedBuild(repoRoot, installationRoot, landingRef, stamp string) (string, error) {
	commit := ""
	switch {
	case commitBuildStamp.MatchString(stamp):
		out, err := exec.Command("git", "-C", installationRoot, "rev-parse", "--verify", "--quiet", stamp+"^{commit}").Output()
		if err != nil {
			return "", fmt.Errorf("rebuilt engine carries unresolved build stamp %q; automatic re-arm is bounded to landed commits", stamp)
		}
		commit = strings.TrimSpace(string(out))
	case witnessBuildStamp.MatchString(stamp):
		var err error
		commit, err = resolveWitnessStamp(repoRoot, installationRoot, landingRef, witnessBuildStamp.FindStringSubmatch(stamp)[1])
		if err != nil {
			return "", fmt.Errorf("rebuilt engine was built from %s, which is not proven landed on %s: %w", stamp, landingRef, err)
		}
	default:
		shown := stamp
		if shown == "" {
			shown = "<unreadable>"
		}
		return "", fmt.Errorf("rebuilt engine carries build stamp %s; automatic re-arm is bounded to landed commits", shown)
	}
	if err := exec.Command("git", "-C", installationRoot, "merge-base", "--is-ancestor", commit, landingRef).Run(); err != nil {
		return "", fmt.Errorf("rebuilt engine was built from %s, which is not landed on %s", stamp, landingRef)
	}
	return commit, nil
}
