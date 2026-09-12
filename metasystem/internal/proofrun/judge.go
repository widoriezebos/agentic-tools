package proofrun

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// JudgeCompatibilityVersion is bumped by hand when the runner, the identity
// composition or the result vocabulary changes meaning in a way the engine
// sources' bytes do not show. It is the first part of every judge key.
const JudgeCompatibilityVersion = "judge/v1"

// judgeSources are what the engine binary is built from: the command and
// internal trees, the module file and its checksums. Their git ids at the
// policy base commit are the second part of the judge key. The whole binary
// is the judge (coverage verdicts live in internal/audit, the worker in
// cmd/metasystem), so no hand list of judge packages can miss one; a
// records, plans, docs or scripts landing keeps every id.
var judgeSources = []string{"cmd", "internal", "go.mod", "go.sum"}

// DefaultJudgeKey is the key of a request that computed none (fixtures, or a
// checkout without the engine sources): the version binds, the sources read
// absent.
func DefaultJudgeKey() string {
	parts := []string{JudgeCompatibilityVersion}
	for range judgeSources {
		parts = append(parts, "absent")
	}
	return strings.Join(parts, ":")
}

// ComputeJudgeKey names the judge that will read a candidate: the
// compatibility version and the git ids of the engine sources at the policy
// base commit, beneath the installation prefix. A rebuild of the same source
// keeps the key; an edit to any engine source changes it. A path the commit
// does not carry reads absent, so an adopted application or a fixture
// repository still gets a stable key. A repository or commit that cannot be
// read yields a key that matches nothing (a fresh token), never one that
// matches another unreadable run: a key must never make a test run fail,
// and never let two unknown judges look alike.
func ComputeJudgeKey(ctx context.Context, projectRoot, policyBaseCommit, installationPrefix string) string {
	if projectRoot == "" || policyBaseCommit == "" {
		return unreadableJudgeKey()
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := judgeGit(bounded, projectRoot, "rev-parse", "--verify", "--quiet", policyBaseCommit+"^{commit}"); err != nil {
		return unreadableJudgeKey()
	}
	parts := []string{JudgeCompatibilityVersion}
	for _, source := range judgeSources {
		id, err := judgeSourceID(bounded, projectRoot, policyBaseCommit, path.Join(installationPrefix, source))
		if err != nil {
			return unreadableJudgeKey()
		}
		parts = append(parts, id)
	}
	return strings.Join(parts, ":")
}

// judgeSourceID is the git id of one engine source at the commit, "absent"
// when the commit carries no such path (ls-tree lists nothing), and an
// error when git could not answer at all. Paths are read against the
// repository top, whatever directory the engine runs in.
func judgeSourceID(ctx context.Context, projectRoot, commit, sourcePath string) (string, error) {
	listing, err := judgeGit(ctx, projectRoot, "ls-tree", "-z", "--full-tree", commit, "--", sourcePath)
	if err != nil {
		return "", err
	}
	if listing == "" {
		return "absent", nil
	}
	head, _, _ := strings.Cut(strings.TrimRight(listing, "\x00"), "\t")
	fields := strings.Fields(head)
	if len(fields) != 3 || !validTreeDigest(fields[2]) {
		return "", errors.New("judge source listing is not a git tree entry")
	}
	return fields[2], nil
}

func judgeGit(ctx context.Context, projectRoot string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", projectRoot}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func unreadableJudgeKey() string {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return JudgeCompatibilityVersion + ":unreadable:" + time.Now().UTC().Format(time.RFC3339Nano)
	}
	return JudgeCompatibilityVersion + ":unreadable:" + hex.EncodeToString(token)
}
