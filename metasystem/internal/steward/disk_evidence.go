package steward

// The evidence path's production seams in the steward's passes (design
// engine-owns-disk-lifetimes 3.5, 3.12): the checkout pass distils and moves
// suite-failure bundles with the host's blob store, the checkout's
// repository as a de-duplication hint, and the checkout's four facts read
// under the pass's context.

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// userHome is the user's home directory, where the host's blob store and
// the default evidence roots live; a fixture passes its own.
func (p DiskPass) userHome() (string, error) {
	if p.UserHome != "" {
		return p.UserHome, nil
	}
	return os.UserHomeDir()
}

// suiteFailureClass is the checkout pass's evidence ageing for top.
func suiteFailureClass(top string, settings diskstore.Settings, pass DiskPass) (diskstore.Class, error) {
	home, err := pass.userHome()
	if err != nil {
		return nil, err
	}
	gitRoot := gitRootAbove(top)
	segmentDir := ""
	if root := settings.EvidenceRoot.Path; root != "" {
		segmentDir = filepath.Join(root, "suite-failures", diskstore.Segment(gitRoot))
	}
	class := diskstore.SuiteFailures{Dir: filepath.Join(top, "artifacts", "agents", "suite-failures"), SegmentDir: segmentDir,
		Installation: top, GitRoot: gitRoot, Blobs: diskstore.BlobStore{Dir: diskstore.BlobStoreDir(home)},
		CompressAbove: settings.Bytes(config.DiskCompressAboveKey), DistillAfter: settings.Duration(config.DiskSuiteDistillKey),
		MoveAfter: settings.Duration(config.DiskSuiteMoveKey), Target: settings.Bytes(config.DiskSuiteTargetKey), Entropy: rand.Reader,
		Known: repositoryKnows(gitRoot), Facts: checkoutFactsReader(top, gitRoot),
		AttemptGoal: func(attempt string) (string, bool) {
			record, err := proofrun.ReadAttempt(top, attempt)
			if err != nil {
				return "", false
			}
			return record.AccountedGoal(), true
		}}
	if pass.SuiteFailureSeams != nil {
		pass.SuiteFailureSeams(&class)
	}
	return class, nil
}

// gitContext runs git in root under ctx with the scrubbed environment.
func gitContext(ctx context.Context, root, stdin string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	out, err := command.Output()
	return string(out), err
}

// repositoryKnows answers which files are byte-identical to an object of
// the repository at gitRoot: one hash-object over the paths, one
// batch-check over the ids. It is a de-duplication hint; the recipe never
// depends on the repository.
func repositoryKnows(gitRoot string) func(ctx context.Context, paths []string) (map[string]bool, error) {
	return func(ctx context.Context, paths []string) (map[string]bool, error) {
		sorted := append([]string(nil), paths...)
		sort.Strings(sorted)
		hashed, err := gitContext(ctx, gitRoot, strings.Join(sorted, "\n")+"\n", "hash-object", "--no-filters", "--stdin-paths")
		if err != nil {
			return nil, err
		}
		ids := strings.Fields(hashed)
		if len(ids) != len(sorted) {
			return nil, fmt.Errorf("hash-object answered %d ids for %d paths", len(ids), len(sorted))
		}
		checked, err := gitContext(ctx, gitRoot, strings.Join(ids, "\n")+"\n", "cat-file", "--batch-check")
		if err != nil {
			return nil, err
		}
		known := map[string]bool{}
		scanner := bufio.NewScanner(strings.NewReader(checked))
		for index := 0; scanner.Scan() && index < len(sorted); index++ {
			if !strings.HasSuffix(scanner.Text(), " missing") {
				known[sorted[index]] = true
			}
		}
		return known, scanner.Err()
	}
}

// checkoutFactsReader reads the checkout's root commit and ledger identity
// under the pass's context; the ledger read is abandoned, not waited for,
// when the context ends first.
func checkoutFactsReader(installation, gitRoot string) func(ctx context.Context) (diskstore.CheckoutFacts, error) {
	return func(ctx context.Context) (diskstore.CheckoutFacts, error) {
		facts := diskstore.CheckoutFacts{GitRoot: gitRoot, Installation: installation}
		roots, err := gitContext(ctx, gitRoot, "", "rev-list", "--max-parents=0", "HEAD")
		if err != nil {
			return facts, err
		}
		if fields := strings.Fields(roots); len(fields) > 0 {
			sort.Strings(fields)
			facts.RootCommit = fields[0]
		}
		identity := make(chan string, 1)
		go func() {
			endpoint, err := goal.ResolveEndpoint(installation)
			if err != nil {
				identity <- ""
				return
			}
			identity <- goal.ExistingLedgerIdentityAtEndpoint(endpoint)
		}()
		select {
		case facts.LedgerIdentity = <-identity:
		case <-ctx.Done():
			return facts, errors.Join(errors.New("the ledger identity was not read within the pass budget"), ctx.Err())
		}
		return facts, nil
	}
}

// gitRootAbove is the nearest directory from dir upward holding a .git
// entry (a directory, or a linked worktree's file), else dir: the git
// top-level the chain mirror hashes into its segment, found without running
// git.
func gitRootAbove(dir string) string {
	for current := dir; ; {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dir
		}
		current = parent
	}
}
