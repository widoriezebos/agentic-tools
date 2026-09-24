package missionrunner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type wallReads interface {
	Git(root string, args ...string) (stdout, stderr string, code int)
	BlobOID(root string, content []byte) (string, error)
	LedgerTruth(repo string, state map[string]any, path string) (anchored, current string, err error)
	LedgerBlobOID(repo string, state map[string]any, path string) (string, error)
	AuthenticateLedger(repo string, state map[string]any, path string) error
}

type defaultWallReads struct{}

func (defaultWallReads) Git(root string, args ...string) (string, string, int) {
	return gitCaptured(root, args...)
}

func (defaultWallReads) BlobOID(root string, content []byte) (string, error) {
	return blobOID(root, content)
}

func (defaultWallReads) LedgerTruth(repo string, state map[string]any, path string) (string, string, error) {
	return mission.AnchoredLedgerTruth(repo, state, path)
}

func (defaultWallReads) LedgerBlobOID(repo string, state map[string]any, path string) (string, error) {
	return mission.AnchoredLedgerBlobOID(repo, state, path)
}

func (defaultWallReads) AuthenticateLedger(repo string, state map[string]any, path string) error {
	return mission.AuthenticateLiveLedger(repo, state, path)
}

func (e *Engine) wallReads() wallReads {
	if e.wallReadFacts != nil {
		return e.wallReadFacts
	}
	return defaultWallReads{}
}

// wallWorkspace uses gittree's repository answers and effects without
// translating its result types or errors.
type wallWorkspace interface {
	FileAt(tree, path string) ([]byte, bool, error)
	ChangedPaths(fromTree, toTree string) ([]string, error)
	Apply(baseTree string, patch []byte) (string, error)
	Entries(tree string, paths []string) (map[string]gittree.Entry, error)
	Snapshot(baseline string) (string, error)
	SnapshotSeeded(seedCommit, expectedTree string, declaredPaths []string) (string, error)
	FilterTree(tree string, paths []string) (string, error)
	HeadTree() (string, error)
	HeadCommit() (oid string, unborn bool, err error)
	TreeOf(rev string) (string, error)
	RefMap() (map[string]string, error)
	SymbolicHead() (ref string, detached bool, err error)
	WorktreeCensus() ([]gittree.WorktreeRecord, error)
	StagedTree() (string, error)
	TopStagedPosture() (gittree.StagedPosture, error)
	Prefix() (string, error)
	TopLevel() (string, error)
	HistorySteeringFiles() ([]string, error)
	Anchor(mission, tree string) error
	MaterializePaths(tree string, paths []string) error
}

var _ wallWorkspace = gittree.Workspace{}

// wallWorkspace addresses exactly the requested root through this engine.
func (e *Engine) wallWorkspace(root string) wallWorkspace {
	if e.wallWorkspaceFactory != nil {
		return e.wallWorkspaceFactory(root)
	}
	return gittree.Workspace{Dir: root}
}
