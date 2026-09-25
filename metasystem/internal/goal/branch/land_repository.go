package branch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

type landingScope uint8

const (
	landingAll landingScope = iota
	landingContract
	landingWithoutContract
)

type landingRepository struct {
	reads       attestationReads
	human       func(repo, name string) ([]byte, error)
	open        func(repo, base string) (dir string, close func(), err error)
	reset       func(dir, base string) error
	head        func(dir string) (string, error)
	index       func(dir string) (string, error)
	entry       func(repo, tree, path string) (mode, blob string, present bool, err error)
	patch       func(repo, before, after string) ([]byte, error)
	apply       func(dir string, patch []byte, threeWay bool) error
	transition  func(repo, before, after string, scope landingScope) ([]byte, error)
	message     func(repo, commit string) ([]byte, error)
	stage       func(dir string) error
	commit      func(dir string, who landingIdentity, stamp string, message []byte) error
	tree        func(dir string) (string, error)
	exists      func(repo, snapshot, path string) error
	filterExact func(repo, tree string, paths []string) (string, error)
	ancestor    func(repo, ancestor, descendant string) error
	firstParent func(repo, from, to string) ([]byte, error)
	trailers    func(repo, commit string) ([]byte, error)
	contracts   contractgit.CommitAccess
	projection  landing.ProjectionAccess
}

func (r landingRepository) complete() bool {
	return r.reads != nil && r.human != nil && r.open != nil && r.reset != nil &&
		r.head != nil && r.index != nil && r.entry != nil && r.patch != nil &&
		r.apply != nil && r.transition != nil && r.message != nil && r.stage != nil &&
		r.commit != nil && r.tree != nil && r.exists != nil && r.filterExact != nil &&
		r.ancestor != nil && r.firstParent != nil &&
		r.trailers != nil && r.contracts != nil && r.projection != nil
}

func gitLandingRepository() landingRepository {
	return landingRepository{
		reads: gitAttestationReads{},
		human: func(repo, name string) ([]byte, error) {
			return gitOutput(repo, "config", "--get", "goal.human."+name)
		},
		open: func(repo, base string) (string, func(), error) {
			scratch, err := os.MkdirTemp("", "goal-land-prep-*")
			if err != nil {
				return "", nil, err
			}
			worktree := filepath.Join(scratch, "worktree")
			if _, err := gitOutput(repo, "worktree", "add", "--quiet", "--detach", worktree, base); err != nil {
				_ = os.RemoveAll(scratch)
				return "", nil, err
			}
			return worktree, func() {
				_, _ = gitOutput(repo, "worktree", "remove", "--force", worktree)
				_ = os.RemoveAll(scratch)
			}, nil
		},
		reset: func(dir, base string) error {
			_, err := gitOutput(dir, "reset", "--hard", base)
			return err
		},
		head: func(dir string) (string, error) {
			out, err := gitOutput(dir, "rev-parse", "HEAD^{commit}")
			return strings.TrimSpace(string(out)), err
		},
		index: func(dir string) (string, error) {
			out, err := gitOutput(dir, "write-tree")
			return strings.TrimSpace(string(out)), err
		},
		entry: treeEntry,
		patch: func(repo, before, after string) ([]byte, error) {
			return gitOutput(repo, "diff", "--binary", "--full-index", before, after)
		},
		apply: func(dir string, patch []byte, threeWay bool) error {
			args := []string{"apply", "--index"}
			if threeWay {
				args = append(args, "--3way")
			}
			_, err := gitInput(dir, patch, append(args, "-")...)
			return err
		},
		transition: func(repo, before, after string, scope landingScope) ([]byte, error) {
			switch scope {
			case landingContract:
				return transitionRaw(repo, before, after, "--", "metasystem/testing.json")
			case landingWithoutContract:
				return transitionRaw(repo, before, after, "--", ".", ":(exclude)metasystem/testing.json")
			default:
				return transitionRaw(repo, before, after)
			}
		},
		message: func(repo, commit string) ([]byte, error) {
			return gitOutput(repo, "show", "-s", "--format=%B", commit)
		},
		stage: func(dir string) error {
			_, err := gitOutput(dir, "add", "-A")
			return err
		},
		commit: func(dir string, who landingIdentity, stamp string, message []byte) error {
			env := []string{"GIT_AUTHOR_NAME=" + who.Name, "GIT_AUTHOR_EMAIL=" + who.Email,
				"GIT_COMMITTER_NAME=" + who.Name, "GIT_COMMITTER_EMAIL=" + who.Email,
				"GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp}
			_, err := gitInputEnv(dir, env, message, "commit", "--quiet", "-F", "-")
			return err
		},
		tree: func(dir string) (string, error) {
			out, err := gitOutput(dir, "rev-parse", "HEAD^{tree}")
			return strings.TrimSpace(string(out)), err
		},
		exists: func(repo, snapshot, path string) error {
			_, err := gitOutput(repo, "cat-file", "-e", snapshot+":"+path)
			return err
		},
		filterExact: func(repo, tree string, paths []string) (string, error) {
			return (gittree.Workspace{Dir: repo}).FilterTree(tree, paths)
		},
		ancestor: func(repo, ancestor, descendant string) error {
			_, err := gitOutput(repo, "merge-base", "--is-ancestor", ancestor, descendant)
			return err
		},
		firstParent: func(repo, from, to string) ([]byte, error) {
			return gitOutput(repo, "rev-list", "--first-parent", "--reverse", from+".."+to)
		},
		trailers: func(repo, commit string) ([]byte, error) {
			return gitOutput(repo, "show", "-s", "--format=%B", commit)
		},
		contracts:  contractgit.DefaultCommitAccess(),
		projection: landing.DefaultProjectionAccess(),
	}
}

func (r landingRepository) status(repo, endpoint, tip, goal string) (Status, error) {
	return inspectStatus(repo, endpoint, tip, goal, statusDependencies{
		validatedRange: r.reads.Range,
		kind:           r.reads.Kind,
		attestation: func(repo, snapshot, endpoint, goal, unit, commit string) (Attestation, error) {
			return validateAttestation(r.reads, repo, snapshot, endpoint, goal, unit, commit, map[string]bool{})
		},
		localTip: func(string, string) (string, bool, error) {
			return "", false, fmt.Errorf("landing status cannot read a local tip")
		},
	})
}
