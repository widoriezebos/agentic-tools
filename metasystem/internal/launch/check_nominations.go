package launch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type CheckNomination struct{ Package, Pattern, Reason string }
type CheckNominations struct {
	Entries []CheckNomination `json:"entries,omitempty"`
	Skipped []string          `json:"skipped,omitempty"`
}

// ParseCheckNominations reads the builder's RESULT block, not prose elsewhere.
// Each NOMINATE line contains exactly package, run pattern, and reason.
func ParseCheckNominations(message string) CheckNominations {
	var result CheckNominations
	inside := false
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		if buildResultLine.MatchString(strings.Trim(line, "`")) {
			if inside {
				break
			}
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if strings.HasPrefix(line, "```") {
			break
		}
		raw, ok := strings.CutPrefix(line, "NOMINATE:")
		if !ok {
			continue
		}
		fields := strings.Split(raw, ",")
		if len(fields) != 3 {
			result.Skipped = append(result.Skipped, line+": expected package, pattern, reason")
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if fields[0] == "" || fields[1] == "" || fields[2] == "" {
			result.Skipped = append(result.Skipped, line+": all three fields are required")
			continue
		}
		result.Entries = append(result.Entries, CheckNomination{fields[0], strings.TrimPrefix(fields[1], "-run "), fields[2]})
	}
	return result
}

type feedbackCommand struct {
	Package, Pattern string
	Tests            []string
}

// feedbackSelection uses only nominated tests and the changed packages' own
// tests. It never expands imports, symbol readers, or contract canaries.
func feedbackSelection(directory string, paths []string, nominations CheckNominations, environment []string) ([]feedbackCommand, []string, []string, error) {
	context, err := gopackages.EffectiveContext(nil, environment)
	if err != nil {
		return nil, nil, nil, err
	}
	catalog := map[string][]string{}
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, problem error) error {
		if problem != nil {
			return problem
		}
		if entry.IsDir() {
			if path != directory {
				if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata" || entry.Name() == "artifacts" || entry.Name() == "bin" {
					return filepath.SkipDir
				}
				if _, e := os.Stat(filepath.Join(path, "go.mod")); e == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		mayBuild, e := gopackages.FileMayBuild(context, path)
		if e != nil {
			return e
		}
		if !mayBuild {
			return nil
		}
		file, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if e != nil {
			return e
		}
		relative, e := filepath.Rel(directory, filepath.Dir(path))
		if e != nil {
			return e
		}
		pkg := "./" + filepath.ToSlash(relative)
		if relative == "." {
			pkg = "."
		}
		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok && f.Recv == nil && testpolicy.GoTestName(f.Name.Name) {
				catalog[pkg] = append(catalog[pkg], f.Name.Name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	for pkg, names := range catalog {
		catalog[pkg] = slices.Compact(slices.Sorted(slices.Values(names)))
	}
	var commands []feedbackCommand
	selected := []string{}
	skipped := slices.Clone(nominations.Skipped)
	add := func(pkg, pattern string, names []string) {
		tests := []string{}
		for _, name := range names {
			id := pkg + "/" + name
			tests = append(tests, id)
			if !slices.Contains(selected, id) {
				selected = append(selected, id)
			}
		}
		commands = append(commands, feedbackCommand{pkg, pattern, tests})
	}
	for _, n := range nominations.Entries {
		reason := ""
		names := []string{}
		pattern := n.Pattern
		top, _, _ := strings.Cut(pattern, "/")
		re, e := regexp.Compile(top)
		if e == nil {
			_, e = regexp.Compile(pattern)
		}
		if n.Package != "." && (!strings.HasPrefix(n.Package, "./") || filepath.ToSlash(filepath.Clean(n.Package)) != strings.TrimPrefix(n.Package, "./")) {
			reason = "package must be relative to the module"
		} else if strings.HasPrefix(pattern, "-") || strings.ContainsAny(n.Package, ";$`|&\n\r") {
			reason = "invalid package or pattern"
		} else if e != nil {
			reason = "invalid run pattern"
		} else {
			for _, name := range catalog[n.Package] {
				if re.MatchString(name) {
					names = append(names, name)
				}
			}
			if len(names) == 0 {
				reason = "no matching tests"
			}
		}
		if reason != "" {
			skipped = append(skipped, fmt.Sprintf("%s, %s, %s: %s", n.Package, n.Pattern, n.Reason, reason))
			continue
		}
		add(n.Package, pattern, names)
	}
	changed := map[string]bool{}
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(path))
		if path == "." || strings.HasPrefix(path, "../") {
			continue
		}
		for dir := filepath.ToSlash(filepath.Dir(path)); ; dir = filepath.ToSlash(filepath.Dir(dir)) {
			pkg := "./" + dir
			if dir == "." {
				pkg = "."
			}
			if _, ok := catalog[pkg]; ok {
				changed[pkg] = true
				break
			}
			if dir == "." {
				break
			}
		}
	}
	var packages []string
	for pkg := range changed {
		packages = append(packages, pkg)
	}
	slices.Sort(packages)
	for _, pkg := range packages {
		var names, patterns []string
		for _, name := range catalog[pkg] {
			if !slices.Contains(selected, pkg+"/"+name) {
				names = append(names, name)
				patterns = append(patterns, regexp.QuoteMeta(name))
			}
		}
		if len(names) == 0 {
			continue
		}
		pattern := ""
		if len(names) != len(catalog[pkg]) {
			pattern = "^(" + strings.Join(patterns, "|") + ")$"
		}
		add(pkg, pattern, names)
	}
	return commands, selected, skipped, nil
}
