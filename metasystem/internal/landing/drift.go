package landing

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// DriftEntry is one refused porcelain status line.
type DriftEntry struct {
	Kind     string
	Index    byte
	Worktree byte
	Path     string
}

// WorktreeDrift classifies repository status while tolerating only candidate
// index entries and append-shaped working changes to declared registers.
func WorktreeDrift(root string, requireEmptyIndex bool) ([]DriftEntry, []string, error) {
	if root == "" {
		return nil, nil, fmt.Errorf("landing drift requires --root")
	}
	workspace := gittree.Workspace{Dir: root}
	status, err := workspace.Status()
	if err != nil {
		return nil, nil, err
	}
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, nil, err
	}
	drift := []DriftEntry{}
	tolerated := []string{}
	for _, entry := range status {
		kind, tolerateRegister, err := classifyDrift(workspace, prefix, entry, requireEmptyIndex)
		if err != nil {
			return nil, nil, err
		}
		if tolerateRegister {
			tolerated = append(tolerated, entry.Path)
			continue
		}
		if kind != "" {
			drift = append(drift, DriftEntry{
				Kind: kind, Index: entry.Index, Worktree: entry.Worktree, Path: entry.Path,
			})
		}
	}
	return drift, tolerated, nil
}

func classifyDrift(workspace gittree.Workspace, prefix string, entry gittree.StatusEntry, requireEmptyIndex bool) (string, bool, error) {
	if entry.Index == '?' && entry.Worktree == '?' {
		// An untracked path outside the installation prefix is not the
		// landing's drift: the rule this verb replaced listed untracked paths
		// from the installation directory only (git ls-files --others there),
		// and a prefixed layout keeps repository-scope state beside it.
		// Tracked changes stay repository-wide, as git diff was.
		if prefix != "" && !strings.HasPrefix(entry.Path, prefix) {
			return "", false, nil
		}
		return "untracked", false, nil
	}
	if requireEmptyIndex {
		if entry.Index != ' ' {
			return "staged", false, nil
		}
		if entry.Worktree == 'M' {
			register, path := workspaceRegisterPath(prefix, entry.Path)
			if register {
				appendShape, err := isRegisterAppend(workspace, path)
				if err != nil {
					return "", false, err
				}
				if appendShape {
					return "", true, nil
				}
				return "register-not-append", false, nil
			}
		}
		return "unstaged", false, nil
	}

	if entry.Worktree == ' ' && strings.ContainsRune("MADT", rune(entry.Index)) {
		return "", false, nil
	}
	if entry.Worktree == 'M' && strings.ContainsRune(" MA", rune(entry.Index)) {
		register, path := workspaceRegisterPath(prefix, entry.Path)
		if register {
			appendShape, err := isRegisterAppend(workspace, path)
			if err != nil {
				return "", false, err
			}
			if appendShape {
				return "", true, nil
			}
			return "register-not-append", false, nil
		}
	}
	return "unstaged", false, nil
}

func workspaceRegisterPath(prefix, topLevelPath string) (bool, string) {
	if prefix != "" {
		if !strings.HasPrefix(topLevelPath, prefix) {
			return false, ""
		}
		topLevelPath = strings.TrimPrefix(topLevelPath, prefix)
	}
	return isAppendOnlyRegister(topLevelPath), topLevelPath
}

func isRegisterAppend(workspace gittree.Workspace, path string) (bool, error) {
	info, err := os.Lstat(filepath.Join(workspace.Dir, filepath.FromSlash(path)))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	indexTree, err := workspace.StagedTree()
	if err != nil {
		return false, err
	}
	before, existed, err := workspace.FileAt(indexTree, path)
	if err != nil {
		return false, err
	}
	after, err := os.ReadFile(filepath.Join(workspace.Dir, filepath.FromSlash(path)))
	if err != nil {
		return false, err
	}
	if len(after) == 0 || after[len(after)-1] != '\n' {
		return false, nil
	}
	if !existed {
		return true, nil
	}
	if len(before) > 0 && before[len(before)-1] != '\n' {
		return false, nil
	}
	return len(after) > len(before) && bytes.Equal(after[:len(before)], before), nil
}
