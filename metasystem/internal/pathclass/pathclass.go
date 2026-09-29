// Package pathclass owns the manifest that classifies repository paths by
// change law. Consumers use the parsed manifest directly when their paths are
// already installation-relative and the resolver when they start with a
// repository path.
package pathclass

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// SourcePath is the installation-relative path of the manifest's source.
// The engine reads its compiled-in copy; a landing reads the file at this
// path in its base tree, so a candidate never judges itself by its own policy.
const SourcePath = "internal/pathclass/path-classes.txt"

// LegacySourcePath is where a base tree from before the manifest was
// compiled in keeps it; a landing reads it only when SourcePath is absent
// from its base, which is true of exactly one landing: the move itself.
const LegacySourcePath = "scripts/agents/path-classes.txt"

//go:embed path-classes.txt
var manifestSource []byte

// Source is the compiled-in manifest bytes.
func Source() []byte { return append([]byte(nil), manifestSource...) }

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
	Floors  map[string]bool
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

// Load parses the manifest compiled into this engine.
func Load() (*Manifest, error) {
	return Parse(manifestSource)
}

// Parse validates and parses one complete manifest.
func Parse(data []byte) (*Manifest, error) {
	manifest := &Manifest{owners: make(map[string]string), Floors: make(map[string]bool)}
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
		case "floor":
			if err := validPath(key, true); err != nil {
				return nil, fmt.Errorf("path class manifest line %d: %w", lineNumber, err)
			}
			if fields[1] != "tier-1-refused" {
				return nil, fmt.Errorf("path class manifest line %d has unknown floor rule %q", lineNumber, fields[1])
			}
			if manifest.Floors[key] {
				return nil, fmt.Errorf("path class manifest line %d duplicates floor:%s", lineNumber, key)
			}
			manifest.Floors[key] = true
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

// TierOneRefused reports whether an installation-relative path is protected
// by the tier-1 floor. Directory rows protect every descendant.
func (m *Manifest) TierOneRefused(key string) bool {
	key = cleanKey(key)
	for floor := range m.Floors {
		if rowMatches(floor, key) {
			return true
		}
	}
	return false
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
	return fmt.Sprintf("path %s has no class in the engine's path-class policy (%s); no classified ancestor", key, SourcePath)
}
