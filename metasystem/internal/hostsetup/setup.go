// Package hostsetup plans and applies runtime entry-point registration for an
// existing MetaSystem installation. It consumes the runtime registry rows and
// never selects a model, execution roster, or active host.
package hostsetup

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const (
	pointerBegin = "<!-- BEGIN METASYSTEM HOST POINTERS -->"
	pointerEnd   = "<!-- END METASYSTEM HOST POINTERS -->"
)

type Options struct {
	RepositoryPath string
	Runtimes       []string
	CopySkills     bool
	Check          bool
}

type Result struct {
	Layout   stateroot.Layout
	Runtimes []string
	Changed  []string
}

type actionKind int

const (
	actionFile actionKind = iota
	actionLink
)

type action struct {
	kind actionKind
	path string
	data []byte
	link string
	mode fs.FileMode
}

// Setup validates every selected input and destination before publishing any
// entry. Check mode uses the same plan and refuses when a write is required.
func Setup(options Options) (Result, error) {
	layout, err := stateroot.ResolveLayout(options.RepositoryPath)
	if err != nil {
		return Result{}, err
	}
	selected, err := selectedRuntimes(options.Runtimes)
	if err != nil {
		return Result{}, err
	}
	actions, err := plan(layout, selected, options.CopySkills)
	if err != nil {
		return Result{}, err
	}
	result := Result{Layout: layout, Runtimes: selected}
	for _, item := range actions {
		rel, _ := filepath.Rel(layout.RepositoryRoot, item.path)
		result.Changed = append(result.Changed, filepath.ToSlash(rel))
	}
	if options.Check {
		if len(actions) != 0 {
			return result, fmt.Errorf("host setup check: %d registration path(s) require setup", len(actions))
		}
		return result, nil
	}
	for _, item := range actions {
		if err := publish(layout.RepositoryRoot, item); err != nil {
			return result, err
		}
	}
	return result, nil
}

func selectedRuntimes(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return runtimes.Adoptable(), nil
	}
	if len(requested) == 1 && requested[0] == "none" {
		return nil, nil
	}
	seen := map[string]bool{}
	for _, runtime := range requested {
		if runtime == "none" {
			return nil, fmt.Errorf("runtime none cannot be combined with other runtimes")
		}
		declaration, ok := runtimes.Lookup(runtime)
		if !ok || !declaration.Adoptable {
			return nil, fmt.Errorf("unknown or non-adoptable runtime: %s", runtime)
		}
		if seen[runtime] {
			return nil, fmt.Errorf("runtime selection contains a duplicate: %s", runtime)
		}
		seen[runtime] = true
	}
	return append([]string(nil), requested...), nil
}

func plan(layout stateroot.Layout, selected []string, copySkills bool) ([]action, error) {
	var actions []action
	if layout.Template && len(selected) > 0 {
		for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
			item, needed, err := instructionAction(filepath.Join(layout.RepositoryRoot, name), managedPointers())
			if err != nil {
				return nil, err
			}
			if needed {
				actions = append(actions, item)
			}
		}
	}
	skills, err := skillNames(layout.InstallationRoot)
	if err != nil {
		return nil, err
	}
	plannedTrees := map[string]bool{}
	for _, runtime := range selected {
		rows := runtimes.RegistrationRows(runtime)
		if len(rows) == 0 {
			return nil, fmt.Errorf("runtime %s has no registration declaration", runtime)
		}
		for _, row := range rows {
			switch row.Operation {
			case runtimes.OpTree:
				if plannedTrees[row.Destination] {
					continue
				}
				plannedTrees[row.Destination] = true
				items, err := planSkillTree(layout, row, skills, copySkills)
				if err != nil {
					return nil, err
				}
				actions = append(actions, items...)
			case runtimes.OpSkillProfiles:
				items, err := planProfiles(layout, row, skills)
				if err != nil {
					return nil, err
				}
				actions = append(actions, items...)
			case runtimes.OpCopyFile, runtimes.OpJSONStripKey:
				source := filepath.Join(layout.InstallationRoot, filepath.FromSlash(row.Source))
				shipped, err := os.ReadFile(source)
				if err != nil {
					return nil, fmt.Errorf("host setup: read %s enforcement source: %w", runtime, err)
				}
				destination := filepath.Join(layout.RepositoryRoot, filepath.FromSlash(row.Destination))
				existing := []byte("{}\n")
				mode := fs.FileMode(0o644)
				if info, statErr := os.Lstat(destination); statErr == nil {
					if !info.Mode().IsRegular() {
						return nil, fmt.Errorf("host setup conflict: %s is not a regular file", row.Destination)
					}
					existing, err = os.ReadFile(destination)
					if err != nil {
						return nil, fmt.Errorf("host setup conflict: cannot read %s: %w", row.Destination, err)
					}
					mode = info.Mode().Perm()
				} else if !os.IsNotExist(statErr) {
					return nil, statErr
				}
				registrationAtInstallation := layout.RepositoryRoot == layout.InstallationRoot
				if hooks.CheckSettings(existing, shipped, runtime, layout.InstallationRel, registrationAtInstallation) == nil {
					continue
				}
				merged, err := hooks.MergeSettings(existing, shipped, runtime, layout.InstallationRel, registrationAtInstallation)
				if err != nil {
					return nil, fmt.Errorf("host setup %s: %w", runtime, err)
				}
				if err := hooks.CheckSettings(merged, shipped, runtime, layout.InstallationRel, registrationAtInstallation); err != nil {
					return nil, fmt.Errorf("host setup %s rendered invalid settings: %w", runtime, err)
				}
				if !bytes.Equal(existing, merged) {
					actions = append(actions, action{kind: actionFile, path: destination, data: merged, mode: mode})
				}
			default:
				return nil, fmt.Errorf("host setup: unsupported registration operation %s", row.Operation)
			}
		}
	}
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].path < actions[j].path })
	for _, item := range actions {
		if err := validateParents(layout.RepositoryRoot, item.path); err != nil {
			return nil, err
		}
	}
	return actions, nil
}

func managedPointers() string {
	return pointerBegin + "\n" +
		"MetaSystem's canonical agent contract is `metasystem/AGENTS.md`; use `metasystem/wow.md` to route to task-specific guidance.\n" +
		"This repository's local development facts are in `development/project-rules-local.md`.\n" + pointerEnd
}

func instructionAction(path, block string) (action, bool, error) {
	mode := fs.FileMode(0o644)
	existing := ""
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return action{}, false, fmt.Errorf("host setup conflict: %s is not a regular instruction file", filepath.Base(path))
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return action{}, false, readErr
		}
		existing = string(data)
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return action{}, false, err
	}
	beginCount, endCount := strings.Count(existing, pointerBegin), strings.Count(existing, pointerEnd)
	if beginCount != endCount || beginCount > 1 {
		return action{}, false, fmt.Errorf("host setup conflict: %s has a malformed managed pointer block", filepath.Base(path))
	}
	desired := block + "\n"
	if beginCount == 1 {
		start := strings.Index(existing, pointerBegin)
		end := strings.Index(existing[start:], pointerEnd) + start + len(pointerEnd)
		desired = existing[:start] + block + existing[end:]
		if !strings.HasSuffix(desired, "\n") {
			desired += "\n"
		}
	} else if existing != "" {
		desired = strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
	}
	if desired == existing {
		return action{}, false, nil
	}
	return action{kind: actionFile, path: path, data: []byte(desired), mode: mode}, true, nil
}

func skillNames(installation string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(installation, "skills"))
	if err != nil {
		return nil, fmt.Errorf("host setup: read skill sources: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func planSkillTree(layout stateroot.Layout, row runtimes.RegistrationRow, skills []string, copySkills bool) ([]action, error) {
	var actions []action
	destinationRoot := filepath.Join(layout.RepositoryRoot, filepath.FromSlash(row.Destination))
	for _, name := range skills {
		source := filepath.Join(layout.InstallationRoot, "skills", name)
		destination := filepath.Join(destinationRoot, name)
		if copySkills {
			items, err := planCopiedDirectory(source, destination)
			if err != nil {
				return nil, fmt.Errorf("host setup conflict at %s/%s: %w", row.Destination, name, err)
			}
			actions = append(actions, items...)
			continue
		}
		target, err := filepath.Rel(filepath.Dir(destination), source)
		if err != nil {
			return nil, err
		}
		if info, statErr := os.Lstat(destination); statErr == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return nil, fmt.Errorf("host setup conflict: %s/%s is a foreign file or directory", row.Destination, name)
			}
			existing, err := os.Readlink(destination)
			if err != nil || existing != target {
				return nil, fmt.Errorf("host setup conflict: %s/%s points somewhere else", row.Destination, name)
			}
			continue
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
		actions = append(actions, action{kind: actionLink, path: destination, link: target})
	}
	return actions, nil
}

func planCopiedDirectory(source, destination string) ([]action, error) {
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("existing copied skill is not a directory")
		}
		// An interrupted per-file publication may leave an exact subset. Every
		// surviving entry must still be canonical; only absent source entries are
		// eligible for completion below. Changed or extra content is foreign.
		if err := filepath.WalkDir(destination, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, _ := filepath.Rel(destination, path)
			if rel == "." {
				return nil
			}
			sourcePath := filepath.Join(source, rel)
			sourceInfo, sourceErr := os.Lstat(sourcePath)
			if sourceErr != nil {
				return fmt.Errorf("existing copied skill has an extra entry: %s", rel)
			}
			if entry.IsDir() {
				if !sourceInfo.IsDir() {
					return fmt.Errorf("existing copied skill differs from its source at %s", rel)
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || !sourceInfo.Mode().IsRegular() {
				return fmt.Errorf("existing copied skill differs from its source at %s", rel)
			}
			current, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			canonical, readErr := os.ReadFile(sourcePath)
			if readErr != nil {
				return readErr
			}
			if !bytes.Equal(current, canonical) {
				return fmt.Errorf("existing copied skill differs from its source at %s", rel)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	var actions []action
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("skill source contains an unsupported symlink: %s", path)
		}
		rel, _ := filepath.Rel(source, path)
		destinationPath := filepath.Join(destination, rel)
		if info, statErr := os.Lstat(destinationPath); statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("existing copied skill differs from its source at %s", rel)
			}
			return nil
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		actions = append(actions, action{kind: actionFile, path: destinationPath, data: data, mode: info.Mode().Perm()})
		return nil
	})
	return actions, err
}

func planProfiles(layout stateroot.Layout, row runtimes.RegistrationRow, skills []string) ([]action, error) {
	if row.Mode == "in-place" {
		for _, name := range skills {
			if _, err := os.Stat(filepath.Join(layout.InstallationRoot, strings.ReplaceAll(row.Source, "{skill}", name))); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
		return nil, nil
	}
	var actions []action
	for _, name := range skills {
		source := filepath.Join(layout.InstallationRoot, filepath.FromSlash(strings.ReplaceAll(row.Source, "{skill}", name)))
		info, err := os.Stat(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			if err == nil {
				err = fmt.Errorf("not a regular file")
			}
			return nil, fmt.Errorf("host setup profile source %s: %w", source, err)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		destination := filepath.Join(layout.RepositoryRoot, filepath.FromSlash(strings.ReplaceAll(row.Destination, "{skill}", name)))
		if current, readErr := os.ReadFile(destination); readErr == nil {
			if !bytes.Equal(current, data) {
				return nil, fmt.Errorf("host setup conflict: changed profile %s", strings.TrimPrefix(destination, layout.RepositoryRoot+string(filepath.Separator)))
			}
			continue
		} else if !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("host setup conflict: profile %s is unreadable: %w", destination, readErr)
		}
		actions = append(actions, action{kind: actionFile, path: destination, data: data, mode: info.Mode().Perm()})
	}
	return actions, nil
}

func validateParents(anchor, path string) error {
	for parent := filepath.Dir(path); parent != anchor && strings.HasPrefix(parent, anchor+string(filepath.Separator)); parent = filepath.Dir(parent) {
		if info, err := os.Lstat(parent); err == nil && !info.IsDir() {
			return fmt.Errorf("host setup conflict: parent %s is not a directory", parent)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func publish(_ string, item action) error {
	if item.kind == actionLink {
		if err := os.MkdirAll(filepath.Dir(item.path), 0o755); err != nil {
			return err
		}
		temporary := filepath.Join(filepath.Dir(item.path), "."+filepath.Base(item.path)+".metasystem-link")
		// The deterministic staging name belongs to this publisher. A killed
		// setup may leave its staged link behind; remove only that link so the
		// retry can converge, while refusing a non-link collision.
		if info, err := os.Lstat(temporary); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("host setup conflict: stale link staging path %s is not a symlink", temporary)
			}
			if err := os.Remove(temporary); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Symlink(item.link, temporary); err != nil {
			return err
		}
		defer os.Remove(temporary)
		if err := os.Rename(temporary, item.path); err != nil {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(item.path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(item.path), "."+filepath.Base(item.path)+".metasystem-file-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	failed := func(operation string, operationErr error) error {
		_ = temporary.Close()
		return fmt.Errorf("host setup: %s %s: %w", operation, item.path, operationErr)
	}
	// Set the final mode on the staged inode before publication. A process
	// interruption can therefore leave either the old complete file or the new
	// complete file, never new bytes with a transient CreateTemp mode.
	if err := temporary.Chmod(item.mode); err != nil {
		return failed("stage mode for", err)
	}
	if _, err := temporary.Write(item.data); err != nil {
		return failed("stage", err)
	}
	if err := temporary.Sync(); err != nil {
		return failed("sync", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("host setup: close staged file %s: %w", item.path, err)
	}
	if err := os.Rename(temporaryPath, item.path); err != nil {
		return fmt.Errorf("host setup: publish %s: %w", item.path, err)
	}
	return nil
}
