package landing

import "github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"

// ProjectionAccess supplies repository location and tree filtering effects.
type ProjectionAccess interface {
	TopLevel(root string) (string, error)
	Prefix(root string) (string, error)
	FilterPrefixes(repo, tree string, paths []string) (string, error)
}

type gitProjectionAccess struct {
	RawSource func(gittree.RawRequest) gittree.RawResult
}

func DefaultProjectionAccess() ProjectionAccess { return gitProjectionAccess{} }

func (a gitProjectionAccess) TopLevel(root string) (string, error) {
	return (gittree.Workspace{Dir: root, RawSource: a.RawSource}).TopLevel()
}
func (a gitProjectionAccess) Prefix(root string) (string, error) {
	return (gittree.Workspace{Dir: root, RawSource: a.RawSource}).Prefix()
}
func (a gitProjectionAccess) FilterPrefixes(repo, tree string, paths []string) (string, error) {
	return (gittree.Workspace{Dir: repo, RawSource: a.RawSource}).FilterTreePrefixes(tree, paths)
}
