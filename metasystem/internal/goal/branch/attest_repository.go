package branch

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"os"
	"path/filepath"
	"strings"
)

// attestationReads supplies immutable repository facts for one attestation decision.
type attestationReads interface {
	ReadSubject(repo, commit string) (readsubject.ReadSubject, error)
	RawEntries(repo, commit string) ([]byte, error)
	Range(repo, endpoint, tip, goal string) ([]Commit, error)
	SnapshotFile(repo, snapshot, path string) ([]byte, error)
	TopLevel(repo string) (string, error)
	CommitExists(repo, commit string) error
	Kind(repo, commit, goal string) (KindInfo, error)
	Prefix(repo string) (string, error)
	Transition(repo, before, after string) ([]byte, error)
	TreeEntry(repo, tree, path string) (string, error)
}
type gitAttestationReads struct{}

func (gitAttestationReads) ReadSubject(repo, commit string) (readsubject.ReadSubject, error) {
	read, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{
		RepoRoot: repo, Role: "code-critic", Reviews: "commit:" + commit,
	})
	if err != nil || !present {
		return readsubject.ReadSubject{}, fmt.Errorf("commit subject %s is unreadable: %v", commit, err)
	}
	return read, nil
}
func (gitAttestationReads) RawEntries(repo, commit string) ([]byte, error) {
	return rawEntries(repo, commit)
}
func (gitAttestationReads) Range(repo, endpoint, tip, goal string) ([]Commit, error) {
	return ValidateRange(repo, endpoint, tip, goal)
}
func (gitAttestationReads) SnapshotFile(repo, snapshot, path string) ([]byte, error) {
	if snapshot == "" {
		return nil, fmt.Errorf("snapshot is required for %s", path)
	}
	return gitOutput(repo, "show", snapshot+":"+path)
}
func (gitAttestationReads) TopLevel(repo string) (string, error) {
	return (gittree.Workspace{Dir: repo}).TopLevel()
}
func (gitAttestationReads) CommitExists(r, c string) error {
	_, err := gitOutput(r, "cat-file", "-e", c+"^{commit}")
	return err
}
func (gitAttestationReads) Kind(repo, commit, goal string) (KindInfo, error) {
	return KindOf(repo, commit, goal)
}
func (gitAttestationReads) Prefix(repo string) (string, error) {
	out, err := gitOutput(repo, "rev-parse", "--show-prefix")
	return strings.TrimSpace(string(out)), err
}
func (gitAttestationReads) Transition(repo, before, after string) ([]byte, error) {
	return rawTransition(repo, before, after)
}
func (gitAttestationReads) TreeEntry(repo, tree, path string) (string, error) {
	return treeEntryAt(repo, tree, path)
}

type readCommitEffects struct {
	Inspect       func(CommitRequest) (commitBranchState, error)
	StagedPaths   func(string) ([]string, error)
	AdoptionClean func(CommitRequest, commitBranchState, []string) error
	Patch         func(string, map[string][]byte, string) ([]byte, error)
	Build         func(CommitRequest, commitBranchState, string, string, []byte) (string, error)
	IndexTree     func(string) (string, error)
	RestoreIndex  func(string, string) error
	Stage         func(string, []string) error
	Install       func(CommitRequest, commitBranchState, string) error
}

func gitReadCommitEffects() readCommitEffects {
	return readCommitEffects{
		Inspect: inspectCommitBranch, StagedPaths: stagedPaths, AdoptionClean: adoptionCheckoutClean,
		Patch: prospectiveReadPatch, Build: buildCommitOnto,
		IndexTree: func(repo string) (string, error) {
			out, err := gitOutput(repo, "write-tree")
			return strings.TrimSpace(string(out)), err
		},
		RestoreIndex: func(repo, tree string) error {
			_, err := gitOutput(repo, "read-tree", tree)
			return err
		},
		Stage: func(repo string, paths []string) error {
			args := []string{"add", "--"}
			for _, path := range paths {
				args = append(args, ":(top)"+path)
			}
			_, err := gitOutput(repo, args...)
			return err
		},
		Install: installCommitOnto,
	}
}
func attestationFileAt(r attestationReads, repo, snapshot, path string) ([]byte, error) {
	if snapshot != "" {
		return r.SnapshotFile(repo, snapshot, path)
	}
	root, err := r.TopLevel(repo)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
}
