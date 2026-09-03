// Package pathclass owns the manifest that classifies repository paths by
// change law. Consumers use the parsed manifest directly when their paths are
// already installation-relative and the resolver when they start with a
// repository path.
package pathclass

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const ManifestPath = "scripts/agents/path-classes.txt"

type Class string

const (
	Behavior     Class = "behavior"
	Record       Class = "record"
	Ledger       Class = "ledger"
	Runtime      Class = "runtime"
	Unclassified Class = "unclassified"
	Outside      Class = "outside"
)

type Namespace string

const (
	Install Namespace = "install"
	Repo    Namespace = "repo"
)

type Mode string

const (
	Template Mode = "template"
	Adopted  Mode = "adopted"
)

type row struct {
	namespace Namespace
	key       string
	class     Class
}

// Manifest is immutable after parsing.
type Manifest struct {
	install []row
	repo    []row
	owners  map[string]string
}

// Resolution carries both the answer and the manifest evidence used to reach
// it. Row is empty when no manifest row matched.
type Resolution struct {
	Class     Class
	Namespace Namespace
	Key       string
	Row       string
	Mode      Mode
}

// Load reads the checked-out manifest beneath an installation root.
func Load(installationRoot string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(installationRoot, filepath.FromSlash(ManifestPath)))
	if err != nil {
		return nil, fmt.Errorf("path class manifest is unreadable: %w", err)
	}
	return Parse(data)
}

// Parse validates and parses one complete manifest.
func Parse(data []byte) (*Manifest, error) {
	manifest := &Manifest{owners: make(map[string]string)}
	seen := map[Namespace]map[string]bool{Install: {}, Repo: {}}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("path class manifest line %d must contain one key and one value", lineNumber)
		}
		kind, key, ok := strings.Cut(fields[0], ":")
		if !ok {
			return nil, fmt.Errorf("path class manifest line %d has an unknown row kind", lineNumber)
		}
		switch kind {
		case string(Install), string(Repo):
			namespace := Namespace(kind)
			if err := validPath(key, true); err != nil {
				return nil, fmt.Errorf("path class manifest line %d: %w", lineNumber, err)
			}
			class := Class(fields[1])
			if !validClass(class) {
				return nil, fmt.Errorf("path class manifest line %d has unknown class %q", lineNumber, fields[1])
			}
			if seen[namespace][key] {
				return nil, fmt.Errorf("path class manifest line %d duplicates %s:%s", lineNumber, namespace, key)
			}
			seen[namespace][key] = true
			entry := row{namespace: namespace, key: key, class: class}
			if namespace == Install {
				manifest.install = append(manifest.install, entry)
			} else {
				manifest.repo = append(manifest.repo, entry)
			}
		case "own":
			if err := validPath(key, false); err != nil || !strings.HasPrefix(key, "plans/") {
				return nil, fmt.Errorf("path class manifest line %d has an invalid ownership path", lineNumber)
			}
			if !validGoalID(fields[1]) {
				return nil, fmt.Errorf("path class manifest line %d has invalid goal id %q", lineNumber, fields[1])
			}
			if _, duplicate := manifest.owners[key]; duplicate {
				return nil, fmt.Errorf("path class manifest line %d duplicates own:%s", lineNumber, key)
			}
			manifest.owners[key] = fields[1]
		default:
			return nil, fmt.Errorf("path class manifest line %d has unknown row kind %q", lineNumber, kind)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("path class manifest is unreadable: %w", err)
	}
	return manifest, nil
}

func validClass(class Class) bool {
	switch class {
	case Behavior, Record, Ledger, Runtime:
		return true
	default:
		return false
	}
}

func validPath(key string, directoryAllowed bool) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.ContainsAny(key, "*?[") {
		return errors.New("path is empty, absolute, or contains a glob")
	}
	directory := strings.HasSuffix(key, "/")
	if directory && !directoryAllowed {
		return errors.New("ownership path must name an exact file")
	}
	cleanKey := strings.TrimSuffix(key, "/")
	if cleanKey == "" || path.Clean(cleanKey) != cleanKey {
		return errors.New("path is not a clean relative path")
	}
	for _, segment := range strings.Split(cleanKey, "/") {
		if segment == ".." {
			return errors.New("path contains a .. segment")
		}
	}
	return nil
}

func validGoalID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	for _, character := range id {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

// Class answers in the installation namespace.
func (m *Manifest) Class(key string) Class {
	return m.Resolve(Install, key).Class
}

// Resolve performs longest-prefix resolution within exactly one namespace.
func (m *Manifest) Resolve(namespace Namespace, key string) Resolution {
	key = cleanKey(key)
	rows := m.install
	if namespace == Repo {
		rows = m.repo
	}
	answer := Resolution{Class: Unclassified, Namespace: namespace, Key: key}
	longest := -1
	for _, candidate := range rows {
		if !rowMatches(candidate.key, key) || len(candidate.key) <= longest {
			continue
		}
		longest = len(candidate.key)
		answer.Class = candidate.class
		answer.Row = string(candidate.namespace) + ":" + candidate.key
	}
	return answer
}

// ResolveRepositoryPath classifies a Git-reported repository-relative path
// against the installation prefix, repository mode, and application
// ownership. In an adopted unvendored layout, ownership must win before the
// empty installation prefix can classify every repository path as installed.
func (m *Manifest) ResolveRepositoryPath(mode Mode, ownership stateroot.Ownership, installationPrefix, repositoryPath string) Resolution {
	key := cleanKey(repositoryPath)
	prefix := cleanKey(installationPrefix)
	if mode == Adopted && ownership == stateroot.OwnerApp {
		return Resolution{Class: Outside, Mode: mode}
	}
	if prefix == "" {
		answer := m.Resolve(Install, key)
		answer.Mode = mode
		return answer
	}
	if key == prefix || strings.HasPrefix(key, prefix+"/") {
		installKey := strings.TrimPrefix(strings.TrimPrefix(key, prefix), "/")
		answer := m.Resolve(Install, installKey)
		answer.Mode = mode
		return answer
	}
	if mode == Adopted {
		return Resolution{Class: Outside, Mode: mode}
	}
	answer := m.Resolve(Repo, key)
	answer.Mode = mode
	return answer
}

// GoalOwner returns the goal named by an exact ownership row.
func (m *Manifest) GoalOwner(key string) (string, bool) {
	goal, ok := m.owners[cleanKey(key)]
	return goal, ok
}

func cleanKey(key string) string {
	cleaned := path.Clean(strings.TrimPrefix(filepath.ToSlash(key), "./"))
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func rowMatches(rowKey, key string) bool {
	if !strings.HasSuffix(rowKey, "/") {
		return key == rowKey
	}
	directory := strings.TrimSuffix(rowKey, "/")
	return key == directory || strings.HasPrefix(key, rowKey)
}

// RefusalText is the one fail-closed explanation for an unclassified key.
func RefusalText(key string) string {
	return fmt.Sprintf("path %s has no class in %s; no classified ancestor; add a row for %s or its directory to %s", key, ManifestPath, key, ManifestPath)
}

// ResolvePath discovers the installation and repository around the running
// engine, consults application ownership, and then resolves the appropriate
// manifest namespace.
func ResolvePath(input string) (Resolution, error) {
	absolute, err := absoluteInput(input)
	if err != nil {
		return Resolution{}, err
	}
	installationRoot, err := discoverInstallationRoot()
	if err != nil {
		return Resolution{}, err
	}
	repositoryRoot, err := discoverRepositoryRoot(installationRoot)
	if err != nil {
		return Resolution{}, err
	}
	ownership, modeText, ownerErr := stateroot.Owner(absolute)
	mode := Mode(modeText)
	if ownerErr != nil {
		if ownership == stateroot.OwnerOutside {
			if _, insideRepository := relativeContained(repositoryRoot, absolute); !insideRepository {
				return Resolution{Class: Outside, Mode: mode}, nil
			}
		}
		return Resolution{}, ownerErr
	}
	if ownership == stateroot.OwnerOutside {
		return Resolution{Class: Outside, Mode: mode}, nil
	}
	manifest, err := Load(installationRoot)
	if err != nil {
		return Resolution{}, err
	}
	return resolveAt(manifest, installationRoot, repositoryRoot, mode, ownership, absolute)
}

func resolveAt(manifest *Manifest, installationRoot, repositoryRoot string, mode Mode, ownership stateroot.Ownership, input string) (Resolution, error) {
	if mode == Adopted && ownership == stateroot.OwnerApp {
		return Resolution{Class: Outside, Mode: mode}, nil
	}
	absolute, err := absoluteInput(input)
	if err != nil {
		return Resolution{}, err
	}
	repositoryKey, insideRepository := relativeContained(repositoryRoot, absolute)
	if !insideRepository {
		return Resolution{Class: Outside, Mode: mode}, nil
	}
	if installKey, insideInstallation := relativeContained(installationRoot, absolute); insideInstallation {
		answer := manifest.Resolve(Install, installKey)
		answer.Mode = mode
		return answer, nil
	}
	if mode == Adopted {
		return Resolution{Class: Outside, Mode: mode}, nil
	}
	answer := manifest.Resolve(Repo, repositoryKey)
	answer.Mode = mode
	return answer, nil
}

func absoluteInput(input string) (string, error) {
	absolute, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("path class: resolve %q against the current directory: %w", input, err)
	}
	return filepath.Clean(absolute), nil
}

func relativeContained(root, target string) (string, bool) {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func discoverInstallationRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("path class: locate executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	root, err := filepath.Abs(filepath.Dir(filepath.Dir(executable)))
	if err != nil {
		return "", fmt.Errorf("path class: locate installation: %w", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ManifestPath))); err != nil {
		return "", fmt.Errorf("path class: executable %q is not installed at <installation>/bin/metasystem", executable)
	}
	return root, nil
}

func discoverRepositoryRoot(installationRoot string) (string, error) {
	command := exec.Command("git", "-C", installationRoot, "rev-parse", "--show-toplevel")
	command.Env = scrubGitSteering(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("path class: installation is not inside a Git repository: %s", strings.TrimSpace(string(output)))
	}
	return filepath.Abs(strings.TrimSpace(string(output)))
}

func scrubGitSteering(environment []string) []string {
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, present := strings.Cut(entry, "=")
		if present {
			switch name {
			case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
				"GIT_DISCOVERY_ACROSS_FILESYSTEM", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
				"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
				"GIT_CONFIG_NOSYSTEM", "GIT_GRAFT_FILE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE",
				"GIT_IMPLICIT_WORK_TREE", "GIT_NO_REPLACE_OBJECTS", "GIT_PREFIX":
				continue
			}
		}
		clean = append(clean, entry)
	}
	return clean
}
