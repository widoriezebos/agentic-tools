package contractgit

import (
	"bytes"
	"strings"
)

// CommitAccess supplies raw commit and index facts. Contract policy stays in transition.go.
type CommitAccess interface {
	Paths(repo, before, after, onlyPath string) ([]byte, error)
	Attributes(repo, tree string, paths []string) ([]byte, error)
	OpenAttributeIndex() (string, func(), error)
	LoadAttributeBase(repo, index, tree string) error
	AttributeEntry(repo, index, commit, path string) ([]byte, error)
	DropAttribute(repo, index, path string) error
	SetAttribute(repo, index, mode, blob, path string) error
	AttributeTree(repo, index string) (string, error)
	File(repo, tree, path string) ([]byte, error)
}

func commitPaths(a CommitAccess, repo, before, after, onlyPath string) ([]string, error) {
	out, err := a.Paths(repo, before, after, onlyPath)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, path := range bytes.Split(out, []byte{0}) {
		if len(path) != 0 {
			paths = append(paths, string(path))
		}
	}
	return paths, nil
}

type gitCommitAccess struct{}

func DefaultCommitAccess() CommitAccess { return gitCommitAccess{} }

func (gitCommitAccess) Paths(repo, before, after, onlyPath string) ([]byte, error) {
	args := []string{"diff-tree", "-r", "--name-only", "-z", "--no-renames", before, after}
	if onlyPath != "" {
		args = append(args, "--", onlyPath)
	}
	return runGit(repo, nil, args...)
}
func (gitCommitAccess) Attributes(repo, tree string, paths []string) ([]byte, error) {
	args := append([]string{"check-attr", "--source=" + tree, "-z", "merge", "--"}, paths...)
	return runGit(repo, nil, args...)
}
func (gitCommitAccess) OpenAttributeIndex() (string, func(), error) { return temporaryIndex() }
func (gitCommitAccess) LoadAttributeBase(repo, index, tree string) error {
	_, err := runGit(repo, []string{"GIT_INDEX_FILE=" + index}, "read-tree", tree)
	return err
}
func (gitCommitAccess) AttributeEntry(repo, index, commit, path string) ([]byte, error) {
	return runGit(repo, []string{"GIT_INDEX_FILE=" + index}, "ls-tree", "-z", commit, "--", path)
}
func (gitCommitAccess) DropAttribute(repo, index, path string) error {
	_, err := runGit(repo, []string{"GIT_INDEX_FILE=" + index}, "update-index", "--force-remove", "--", path)
	return err
}
func (gitCommitAccess) SetAttribute(repo, index, mode, blob, path string) error {
	_, err := runGit(repo, []string{"GIT_INDEX_FILE=" + index}, "update-index", "--add", "--cacheinfo", mode+","+blob+","+path)
	return err
}
func (gitCommitAccess) AttributeTree(repo, index string) (string, error) {
	out, err := runGit(repo, []string{"GIT_INDEX_FILE=" + index}, "write-tree")
	return strings.TrimSpace(string(out)), err
}
func (gitCommitAccess) File(repo, tree, path string) ([]byte, error) {
	return treeFile(repo, tree, path)
}
