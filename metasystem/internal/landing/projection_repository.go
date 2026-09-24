package landing

import "github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"

// ProjectionAccess supplies repository location and tree filtering effects.
type ProjectionAccess interface {
	TopLevel(root string) (string, error)
	Prefix(root string) (string, error)
	FilterPrefixes(repo, tree string, paths []string) (string, error)
}

type gitProjectionAccess struct{}

func DefaultProjectionAccess() ProjectionAccess { return gitProjectionAccess{} }

func (gitProjectionAccess) TopLevel(root string) (string, error) {
	return (gittree.Workspace{Dir: root}).TopLevel()
}
func (gitProjectionAccess) Prefix(root string) (string, error) {
	return (gittree.Workspace{Dir: root}).Prefix()
}
func (gitProjectionAccess) FilterPrefixes(repo, tree string, paths []string) (string, error) {
	return (gittree.Workspace{Dir: repo}).FilterTreePrefixes(tree, paths)
}
