package landing

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

var appendOnlyRegisters = []string{"memory/receipts.log", "records/counselor/accepted-risk-register.jsonl", "records/counselor/carried-landings.jsonl", "records/narrator-digest.log"}

var ledgerPaths = []string{
	"plans/goals",
	"plans/goals-accepted.json",
	"plans/goals.md",
	"records/counselor",
	"records/goals",
}

// WorkspaceExclusions returns the paths written as shared coordination state,
// separate from the product workspace tested for delivery.
func WorkspaceExclusions() []string {
	paths := append([]string(nil), appendOnlyRegisters...)
	paths = append(paths, ledgerPaths...)
	sort.Strings(paths)
	compact := paths[:0]
	for _, candidate := range paths {
		covered := false
		for _, parent := range paths {
			if parent != candidate && strings.HasPrefix(candidate, strings.TrimSuffix(parent, "/")+"/") {
				covered = true
				break
			}
		}
		if !covered {
			compact = append(compact, candidate)
		}
	}
	return compact
}

// TestReceiptProjection records a reproducible projection of a receipt tree.
type TestReceiptProjection struct {
	Excludes []string `json:"excludes"`
	Tree     string   `json:"tree"`
}

// ProjectWorkspaceTree removes workspace exclusions from a whole-project tree.
func ProjectWorkspaceTree(root, tree string) (string, error) {
	installation := gittree.Workspace{Dir: root}
	top, err := installation.TopLevel()
	if err != nil {
		return "", err
	}
	prefix, err := installation.Prefix()
	if err != nil {
		return "", err
	}
	paths := WorkspaceExclusions()
	for index, path := range paths {
		paths[index] = filepath.ToSlash(filepath.Join(prefix, path))
	}
	return (gittree.Workspace{Dir: top}).FilterTreePrefixes(tree, paths)
}

// InstallationWorkspaceTree removes workspace exclusions from a tree already
// scoped to the installation subtree.
func InstallationWorkspaceTree(root, tree string) (string, error) {
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return "", err
	}
	return (gittree.Workspace{Dir: top}).FilterTreePrefixes(tree, WorkspaceExclusions())
}
