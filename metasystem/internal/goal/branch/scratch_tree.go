package branch

import "strings"

// ScratchApplyTree is the tree the commit owner's own application of patch
// onto base produces: its disposable scratch worktree and its three-way
// apply, then the index tree. Nothing is committed, no ref moves and the
// caller's checkout is untouched. A replay compares this tree with an
// existing unit's tree to decide whether that unit already holds the change.
func ScratchApplyTree(repo, base string, patch []byte) (string, error) {
	r := gitCommitRepository()
	worktree, closeScratch, err := r.effects.Open(repo, base, false)
	if err != nil {
		return "", err
	}
	defer closeScratch()
	if err := r.effects.Apply(worktree, patch); err != nil {
		return "", operationRefusal(StaleCode, "the change does not apply to %s: %v", base, err)
	}
	out, err := gitOutput(worktree, "write-tree")
	return strings.TrimSpace(string(out)), err
}
