// Package pathpattern owns the relative-path grammar shared by testing policy,
// input manifests, and diagnostics.
package pathpattern

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Pattern struct {
	text       string
	components []string
	subtree    bool
}

func Parse(value string) (Pattern, error) {
	value = strings.ReplaceAll(value, `\`, "/")
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsRune(value, 0) ||
		len(value) >= 2 && value[1] == ':' {
		return Pattern{}, fmt.Errorf("invalid relative path pattern %q", value)
	}
	for _, component := range strings.Split(value, "/") {
		if component == ".." {
			return Pattern{}, fmt.Errorf("path pattern %q traverses outside its root", value)
		}
	}
	value = path.Clean(value)
	if value == "." {
		return Pattern{}, fmt.Errorf("path pattern must name a path")
	}
	subtree := strings.HasSuffix(value, "/**")
	base := value
	if subtree {
		base = strings.TrimSuffix(value, "/**")
	}
	components := strings.Split(base, "/")
	for _, component := range components {
		if component == "" || strings.ContainsAny(component, "[]{}") || strings.Contains(component, "**") {
			return Pattern{}, fmt.Errorf("unsupported wildcard syntax in path pattern %q", value)
		}
	}
	return Pattern{text: value, components: components, subtree: subtree}, nil
}

func (p Pattern) String() string { return p.text }

func (p Pattern) Match(candidate string) bool {
	candidate = strings.ReplaceAll(candidate, `\`, "/")
	if candidate == "" || strings.HasPrefix(candidate, "/") {
		return false
	}
	for _, component := range strings.Split(candidate, "/") {
		if component == ".." {
			return false
		}
	}
	candidate = path.Clean(candidate)
	parts := strings.Split(candidate, "/")
	if len(parts) < len(p.components) || !p.subtree && len(parts) != len(p.components) {
		return false
	}
	for i, component := range p.components {
		matched, err := path.Match(component, parts[i])
		if err != nil || !matched {
			return false
		}
	}
	return true
}

// Covers preserves directory declarations already accepted by older testing
// contracts. An exact file cannot contain descendants, so the extra match is
// conservative when the declaration currently names a file.
func (p Pattern) Covers(candidate string) bool {
	if p.Match(candidate) {
		return true
	}
	return !p.subtree && !strings.ContainsAny(p.text, "*?") && strings.HasPrefix(candidate, p.text+"/")
}

// StaticPrefix is the literal path before the first wildcard component.
// A root-level wildcard returns "." for Git's root pathspec.
func (p Pattern) StaticPrefix() string {
	var parts []string
	for _, component := range p.components {
		if strings.ContainsAny(component, "*?") {
			break
		}
		parts = append(parts, component)
	}
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

func (p Pattern) HasComponentWildcard() bool {
	for _, component := range p.components {
		if strings.ContainsAny(component, "*?") {
			return true
		}
	}
	return false
}

// Expand returns relative non-directory entries. A declaration with no current
// match returns an empty list; its declaration still belongs in the digest.
func (p Pattern) Expand(root string) ([]string, error) {
	static := 0
	for static < len(p.components) && !strings.ContainsAny(p.components[static], "*?") {
		static++
	}
	start := root
	if static > 0 {
		start = filepath.Join(root, filepath.FromSlash(strings.Join(p.components[:static], "/")))
	}
	// A static parent symlink must not redirect the walk outside the tree.
	current := root
	for _, component := range p.components[:static] {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 && (current != start || static < len(p.components) || p.subtree) {
			return nil, fmt.Errorf("path pattern %s traverses symlink %s", p.text, current)
		}
	}
	var matches []string
	exactDirectory := false
	err := filepath.WalkDir(start, func(current string, item fs.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) && current == start {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".git" || strings.HasPrefix(relative, ".git/") {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// An installed dependency tree is content on disk, never repository
		// content, and it is skipped by name at any depth exactly as vendor
		// is skipped elsewhere (g1-s8 revision 5, the exclusion slice).
		if item.IsDir() && item.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if item.IsDir() {
			if current == start && static == len(p.components) && !p.subtree {
				exactDirectory = true
			}
			return nil
		}
		if item.Type()&os.ModeSymlink != 0 && p.matchesPrefix(relative) &&
			len(strings.Split(relative, "/")) < len(p.components) {
			return fmt.Errorf("path pattern %s traverses symlink %s", p.text, relative)
		}
		if p.Match(relative) || exactDirectory && strings.HasPrefix(relative, p.text+"/") {
			matches = append(matches, relative)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

func (p Pattern) matchesPrefix(candidate string) bool {
	parts := strings.Split(candidate, "/")
	if len(parts) > len(p.components) {
		return false
	}
	for i, component := range parts {
		if matched, _ := path.Match(p.components[i], component); !matched {
			return false
		}
	}
	return true
}

// Overlaps is conservative for a path that could be a directory: its entries
// can overlap another declaration rooted below it.
func (p Pattern) Overlaps(other Pattern) bool {
	limit := len(p.components)
	if len(other.components) < limit {
		limit = len(other.components)
	}
	for i := 0; i < limit; i++ {
		if !componentsOverlap(p.components[i], other.components[i]) {
			return false
		}
	}
	return true
}

// componentsOverlap explores the small product automaton of two * / ? globs.
func componentsOverlap(a, b string) bool {
	left, right := []rune(a), []rune(b)
	type state struct{ i, j int }
	seen := map[state]bool{}
	var visit func(int, int) bool
	visit = func(i, j int) bool {
		s := state{i, j}
		if seen[s] {
			return false
		}
		seen[s] = true
		if i == len(left) && j == len(right) {
			return true
		}
		if i < len(left) && left[i] == '*' && visit(i+1, j) {
			return true
		}
		if j < len(right) && right[j] == '*' && visit(i, j+1) {
			return true
		}
		if i == len(left) || j == len(right) {
			return false
		}
		if left[i] == '*' || right[j] == '*' || left[i] == '?' || right[j] == '?' || left[i] == right[j] {
			nextI, nextJ := i+1, j+1
			if left[i] == '*' {
				nextI = i
			}
			if right[j] == '*' {
				nextJ = j
			}
			if nextI != i || nextJ != j {
				return visit(nextI, nextJ)
			}
		}
		return false
	}
	return visit(0, 0)
}
