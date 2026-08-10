package dispatch

import (
	"fmt"
	"path/filepath"
)

// ExpandPermissions turns a permissions envelope into the absolute-rooted
// form a launch is measured against: "." becomes the repository, "<worktree>"
// becomes the job workspace, relative roots resolve against the repository.
// Writable roots demand a worktree and must stay inside it — a delegate never
// writes outside the workspace it was given. A repository-wide network floor
// of deny overrides whatever the envelope asked for; it only ever narrows.
func ExpandPermissions(sourcePath, repo, workspace string, isWorktree bool, preset, networkFloor, outputPath string) error {
	data, err := readJSON(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid permissions envelope: %v", err)
	}
	envelope, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
	}
	expected := []string{"readRoots", "writeRoots", "network", "approvals", "tools"}
	if len(envelope) != len(expected) {
		return fmt.Errorf("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
	}
	for _, key := range expected {
		if _, present := envelope[key]; !present {
			return fmt.Errorf("permissions envelope must contain exactly readRoots, writeRoots, network, approvals, and tools")
		}
	}
	readRoots, readOK := envelope["readRoots"].([]any)
	writeRoots, writeOK := envelope["writeRoots"].([]any)
	if !readOK || !writeOK {
		return fmt.Errorf("permission roots must be arrays")
	}

	repoResolved := resolvePath(repo)
	workspaceResolved := resolvePath(workspace)
	expand := func(value string) string {
		switch {
		case value == ".":
			return repoResolved
		case value == "<worktree>":
			return workspaceResolved
		case filepath.IsAbs(value):
			return resolvePath(value)
		default:
			return resolvePath(filepath.Join(repoResolved, value))
		}
	}
	expandAll := func(items []any) []any {
		expanded := []any{}
		for _, item := range items {
			if value, ok := item.(string); ok {
				expanded = append(expanded, expand(value))
			}
		}
		return expanded
	}
	envelope["readRoots"] = expandAll(readRoots)
	expandedWrite := expandAll(writeRoots)
	envelope["writeRoots"] = expandedWrite

	if len(expandedWrite) > 0 && !isWorktree {
		return fmt.Errorf("writable permissions require --worktree")
	}
	for _, item := range expandedWrite {
		root := item.(string)
		if !pathWithin(resolvePath(root), workspaceResolved) {
			return fmt.Errorf("permission write root escapes the job worktree: %s", root)
		}
	}
	if networkFloor == "deny" {
		envelope["network"] = "deny"
	}
	envelope["preset"] = preset
	return writeRecord(outputPath, envelope)
}
