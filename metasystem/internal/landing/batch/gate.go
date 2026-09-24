package batch

import (
	"bytes"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type gateStep struct {
	Name string
	Args []string
}
type gateStepResult struct {
	RunID    string
	ExitCode int
	Detail   string
}
type batchGateExec func(unitTree string, step gateStep) gateStepResult

type GateStep = gateStep
type GateStepResult = gateStepResult
type GateExecutor = batchGateExec
type patchChange map[string]bool // path -> deleted
type gateChange struct {
	Deleted, BaseAbsent bool
	baseKnown           bool
}
type gateChanges map[string]gateChange

func changedGoPackages(root, tree string, changes gateChanges) ([]string, error) {
	workspaceRoot := unitGateModuleRoot(root)
	if workspaceRoot == "" {
		workspaceRoot = root
	}
	return changedGoPackagesWithWorkspace(root, tree, changes, gittree.Workspace{Dir: workspaceRoot})
}

func changedGoPackagesWithWorkspace(root, tree string, changes gateChanges, workspace gittree.Workspace) ([]string, error) {
	set := map[string]bool{}
	moduleRoot := unitGateModuleRoot(root)
	modulePrefix := ""
	if moduleRoot != "" {
		prefix, err := workspace.Prefix()
		if err != nil {
			return nil, err
		}
		modulePrefix = prefix
		resolved, err := moduleTree(workspace, tree)
		if err != nil {
			return nil, err
		}
		tree = resolved
	} else {
		for changed := range changes {
			if strings.HasPrefix(filepath.ToSlash(changed), "metasystem/") {
				modulePrefix = "metasystem/"
				break
			}
		}
	}
	directoryExists := func(dir string) (bool, error) {
		if root == "" {
			return true, nil
		}
		workspaceRoot := moduleRoot
		path := strings.TrimPrefix(filepath.ToSlash(dir), "./")
		if workspaceRoot == "" {
			workspaceRoot = root
			path = filepath.ToSlash(filepath.Join(strings.TrimSuffix(modulePrefix, "/"), path))
		}
		if path == "" || path == "." {
			path = strings.TrimSuffix(modulePrefix, "/")
		}
		entries, err := workspace.Entries(tree, []string{path})
		return len(entries) != 0, err
	}
	for changed, change := range changes {
		changed = strings.TrimPrefix(filepath.ToSlash(changed), modulePrefix)
		if strings.HasSuffix(changed, ".go") {
			dir := filepath.ToSlash(filepath.Dir(changed))
			pkg := "."
			if dir != "." {
				pkg = "./" + dir
			}
			if change.Deleted {
				if change.BaseAbsent {
					continue
				}
				parent := filepath.ToSlash(filepath.Dir(dir))
				for parent != "." {
					exists, err := directoryExists(parent)
					if err != nil {
						return nil, err
					}
					if exists {
						break
					}
					parent = filepath.ToSlash(filepath.Dir(parent))
				}
				pkg = "./..."
				if parent != "." {
					pkg = "./" + parent + "/..."
				}
			}
			set[pkg] = true
		}
	}
	packages := make([]string, 0, len(set))
	for pkg := range set {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages, nil
}

func patchChangedPaths(patch []byte) patchChange {
	return patchGateChanges(patch).paths()
}

func (changes gateChanges) paths() patchChange {
	paths := patchChange{}
	for path, change := range changes {
		paths[path] = change.Deleted
	}
	return paths
}

func patchGateChanges(patch []byte) gateChanges {
	set := gateChanges{}
	old := ""
	header := false
	update := func(path string, deleted, baseAbsent, baseKnown bool) {
		path = strings.TrimPrefix(strings.TrimPrefix(path, "a/"), "b/")
		if path == "" || path == "/dev/null" {
			return
		}
		change, present := set[path]
		if !present || baseKnown && !change.baseKnown {
			change.BaseAbsent, change.baseKnown = baseAbsent, baseKnown
		}
		change.Deleted = deleted
		set[path] = change
	}
	for _, line := range bytes.Split(patch, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			if _, changed, ok := strings.Cut(string(line), " b/"); ok {
				update(decodePatchPath(changed), false, false, false)
			}
			header, old = true, ""
		} else if header && bytes.HasPrefix(line, []byte("rename from ")) {
			update(decodePatchPath(string(line[len("rename from "):])), true, false, true)
		} else if header && bytes.HasPrefix(line, []byte("rename to ")) {
			update(decodePatchPath(string(line[len("rename to "):])), false, true, true)
		} else if header && bytes.HasPrefix(line, []byte("--- ")) {
			old = decodePatchPath(string(line[4:]))
		} else if header && bytes.HasPrefix(line, []byte("+++ ")) {
			changed := decodePatchPath(string(line[4:]))
			deleted := changed == "/dev/null"
			if deleted {
				changed = old
			}
			update(changed, deleted, old == "/dev/null", true)
			header = false
		}
	}
	return set
}

// ChangedPaths derives a unit's path manifest from its certified patch.
func ChangedPaths(patch []byte) map[string]bool { return patchChangedPaths(patch) }

func decodePatchPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) {
		if decoded, err := strconv.Unquote(raw); err == nil {
			return decoded
		}
	}
	return raw
}
