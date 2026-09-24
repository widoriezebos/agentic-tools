package landing

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// receiptLedgerPath is the receipt ledger by its installation-relative path,
// the register every code landing appends to (landing receipt-line).
const receiptLedgerPath = "memory/receipts.log"

// appendOnlyRegisters names the tracked registers that background writers
// and turn-boundary hooks append to at times a landing does not control:
// the receipt ledger and the narrator digest, the two the design declares
// (plans/landing-receipt-survives-records-drift-design.md, Decision 1). The
// landing rules read this set: the carriage case, the receipt's worktree
// projection, drift and advance. The counselor registers are not in it:
// they are coordination state excluded from the delivery workspace through
// ledgerPaths below, and a change to them keeps its held-goal rule at the
// carriage gate. Paths are workspace-relative, the landing package's one
// path space.
var appendOnlyRegisters = []string{
	receiptLedgerPath,
	"records/narrator-digest.log",
}

// counselorRegisters are the counselor rows a carried landing may append
// to, each line owned by its carried row (candidatePathPolicy in observe.go).
// They are coordination state, excluded from the delivery workspace through
// ledgerPaths below, and not append-only registers of the landing rules: an
// uncarried change to them keeps the held-goal rule at the carriage gate.
var counselorRegisters = []string{
	"records/counselor/accepted-risk-register.jsonl",
	"records/counselor/carried-landings.jsonl",
}

var ledgerPaths = []string{
	"plans/goals",
	"plans/goals-accepted.json",
	"plans/goals.md",
	"records/counselor",
	"records/goals",
}

// AppendOnlyRegisters returns a copy for callers outside the package.
func AppendOnlyRegisters() []string {
	return append([]string(nil), appendOnlyRegisters...)
}

func isAppendOnlyRegister(path string) bool {
	for _, register := range appendOnlyRegisters {
		if path == register {
			return true
		}
	}
	return false
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
	return ProjectWorkspaceTreeWith(root, tree, DefaultProjectionAccess())
}

// ProjectWorkspaceTreeWith applies the installation's workspace exclusions.
func ProjectWorkspaceTreeWith(root, tree string, a ProjectionAccess) (string, error) {
	top, err := a.TopLevel(root)
	if err != nil {
		return "", err
	}
	prefix, err := a.Prefix(root)
	if err != nil {
		return "", err
	}
	paths := WorkspaceExclusions()
	for index, path := range paths {
		paths[index] = filepath.ToSlash(filepath.Join(prefix, path))
	}
	return a.FilterPrefixes(top, tree, paths)
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
