package branch

import "github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"

// BranchReadRepository supplies immutable branch facts and a disposable gate workspace.
// Each read request uses one repository instance throughout its decision and journal flow.
type BranchReadRepository interface {
	Range(repo, endpoint, tip, goal string) ([]Commit, error)
	Subject(repo, commit string) (AttestationSubject, error)
	CommonDir(repo string) (string, error)
	Entries(repo, commit string) ([]Entry, error)
	Detached(repo, commit string) (dir string, close func() error, err error)
}

type defaultBranchReadRepository struct{}

func branchReadRepositoryFor(repository BranchReadRepository) BranchReadRepository {
	if repository == nil {
		return defaultBranchReadRepository{}
	}
	return repository
}

func (defaultBranchReadRepository) Range(repo, endpoint, tip, goal string) ([]Commit, error) {
	return ValidateRange(repo, endpoint, tip, goal)
}

func (defaultBranchReadRepository) Subject(repo, commit string) (AttestationSubject, error) {
	subject, _, err := computeSubject(repo, commit)
	return subject, err
}

func (defaultBranchReadRepository) CommonDir(repo string) (string, error) {
	return gitCommonDir(repo)
}

func (defaultBranchReadRepository) Entries(repo, commit string) ([]Entry, error) {
	return RawEntries(repo, commit)
}

func (defaultBranchReadRepository) Detached(repo, commit string) (string, func() error, error) {
	detached, err := (gittree.Workspace{Dir: repo}).NewDetachedCommitWorktree(commit)
	if err != nil {
		return "", nil, err
	}
	return detached.Workspace().Dir, detached.Close, nil
}
