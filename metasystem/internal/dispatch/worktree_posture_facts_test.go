package dispatch

import "fmt"

type postureFact[T any] struct {
	value T
	err   error
}

type postureRefKey struct{ worktree, ref string }
type postureRangeKey struct{ worktree, from, to string }
type postureTreeKey struct{ worktree, treeish string }
type posturePathKey struct{ worktree, treeish, path string }

// postureFacts only answers facts declared by its test. An undeclared query
// fails at the dependency boundary instead of consulting the local repository.
type postureFacts struct {
	heads       map[string]postureFact[string]
	refs        map[postureRefKey]postureFact[string]
	behind      map[postureRangeKey]postureFact[int64]
	unmerged    map[string]postureFact[map[string]struct{}]
	tracked     map[string]postureFact[map[string]struct{}]
	untracked   map[string]postureFact[map[string]struct{}]
	touched     map[postureRangeKey]postureFact[map[string]struct{}]
	prefixes    map[string]postureFact[string]
	directories map[postureTreeKey]postureFact[map[string]bool]
	presence    map[posturePathKey]postureFact[bool]
}

func newPostureFacts() *postureFacts {
	return &postureFacts{
		heads: map[string]postureFact[string]{}, refs: map[postureRefKey]postureFact[string]{},
		behind: map[postureRangeKey]postureFact[int64]{}, unmerged: map[string]postureFact[map[string]struct{}]{},
		tracked: map[string]postureFact[map[string]struct{}]{}, untracked: map[string]postureFact[map[string]struct{}]{},
		touched: map[postureRangeKey]postureFact[map[string]struct{}]{}, prefixes: map[string]postureFact[string]{},
		directories: map[postureTreeKey]postureFact[map[string]bool]{}, presence: map[posturePathKey]postureFact[bool]{},
	}
}

func posturePaths(paths ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}
	return set
}

func factValue[K comparable, T any](facts map[K]postureFact[T], key K) (T, error) {
	fact, ok := facts[key]
	if !ok {
		var zero T
		return zero, fmt.Errorf("undeclared worktree posture fact: %v", key)
	}
	return fact.value, fact.err
}

func (f *postureFacts) Head(worktree string) (string, error) {
	return factValue(f.heads, worktree)
}
func (f *postureFacts) ResolveRef(worktree, ref string) (string, error) {
	return factValue(f.refs, postureRefKey{worktree, ref})
}
func (f *postureFacts) BehindCount(worktree, from, to string) (int64, error) {
	return factValue(f.behind, postureRangeKey{worktree, from, to})
}
func (f *postureFacts) UnmergedPaths(worktree string) (map[string]struct{}, error) {
	return factValue(f.unmerged, worktree)
}
func (f *postureFacts) TrackedPaths(worktree string) (map[string]struct{}, error) {
	return factValue(f.tracked, worktree)
}
func (f *postureFacts) UntrackedPaths(worktree string) (map[string]struct{}, error) {
	return factValue(f.untracked, worktree)
}
func (f *postureFacts) TrunkTouchedPaths(worktree, from, to string) (map[string]struct{}, error) {
	return factValue(f.touched, postureRangeKey{worktree, from, to})
}
func (f *postureFacts) InstallPrefix(repoRoot string) (string, error) {
	return factValue(f.prefixes, repoRoot)
}
func (f *postureFacts) TreeDirectories(worktree, treeish string) (map[string]bool, error) {
	return factValue(f.directories, postureTreeKey{worktree, treeish})
}
func (f *postureFacts) PathPresent(worktree, treeish, path string) (bool, error) {
	return factValue(f.presence, posturePathKey{worktree, treeish, path})
}
