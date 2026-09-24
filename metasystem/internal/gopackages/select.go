// Package gopackages selects the Go test packages affected by an exact tree change.
package gopackages

import (
	"errors"
	"fmt"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type Selection struct {
	Tree, ModulePath              string
	Changed, Dependents, Packages []string
	// InputDirs contains each runnable package's transitive in-module import
	// directories, including imports from test and build-tagged files.
	InputDirs map[string][]string
}

type selectionWorkspace interface {
	TreeOf(string) (string, error)
	FileAt(string, string) ([]byte, bool, error)
	ChangedPaths(string, string) ([]string, error)
	Entries(string, []string) (map[string]gittree.Entry, error)
}

type selector struct {
	workspace    selectionWorkspace
	openSnapshot func(string) (string, func() error, error)
}

// Select follows changed packages and their transitive consumers, including
// imports in test files and files hidden by build tags. A module manifest
// change selects the complete current package inventory.
func Select(moduleRoot, base, tree string) (selection Selection, err error) {
	return SelectWithTags(moduleRoot, base, tree, nil)
}

// SelectWithTags uses the same Go build tags as the concrete test groups.
func SelectWithTags(moduleRoot, base, tree string, buildTags []string) (selection Selection, err error) {
	return SelectWithEnvironment(moduleRoot, base, tree, buildTags, os.Environ())
}

// SelectWithEnvironment evaluates platform and tag constraints from the
// environment that will be used by the native Go command.
func SelectWithEnvironment(moduleRoot, base, tree string, buildTags, environment []string) (selection Selection, err error) {
	workspace := gittree.Workspace{Dir: moduleRoot}
	owner := selector{
		workspace: workspace,
		openSnapshot: func(tree string) (string, func() error, error) {
			detached, err := workspace.NewDetachedWorktree(tree)
			if err != nil {
				return "", nil, err
			}
			return detached.Workspace().Dir, detached.Close, nil
		},
	}
	return owner.selectPackages(base, tree, buildTags, environment)
}

func (s selector) selectPackages(base, tree string, buildTags, environment []string) (selection Selection, err error) {
	workspace := s.workspace
	resolve := func(rev string) (string, error) {
		if _, present, readErr := workspace.FileAt(rev, "go.mod"); readErr == nil && present {
			return rev, nil
		}
		return workspace.TreeOf(rev)
	}
	baseTree, err := resolve(base)
	if err != nil {
		return selection, fmt.Errorf("go package base: %w", err)
	}
	candidateTree, err := resolve(tree)
	if err != nil {
		return selection, fmt.Errorf("go package candidate: %w", err)
	}
	paths, err := workspace.ChangedPaths(baseTree, candidateTree)
	if err != nil {
		return selection, err
	}
	selection.Tree = candidateTree
	if len(paths) == 0 {
		return selection, nil
	}
	beforeEntries, err := workspace.Entries(baseTree, paths)
	if err != nil {
		return selection, err
	}
	afterEntries, err := workspace.Entries(candidateTree, paths)
	if err != nil {
		return selection, err
	}
	mod, present, err := workspace.FileAt(candidateTree, "go.mod")
	if err != nil {
		return selection, err
	}
	if !present {
		return selection, fmt.Errorf("go package candidate has no go.mod")
	}
	selection.ModulePath, err = modulePath(mod)
	if err != nil {
		return selection, err
	}
	candidateDir, closeCandidate, err := s.openSnapshot(candidateTree)
	if err != nil {
		return selection, fmt.Errorf("materialize Go package candidate: %w", err)
	}
	defer func() { err = errors.Join(err, closeCandidate()) }()
	imports, err := packageImports(candidateDir, selection.ModulePath)
	if err != nil {
		return selection, err
	}
	runnable, err := runnablePackageInventory(candidateDir, buildTags, environment)
	if err != nil {
		return selection, err
	}
	var baseImports map[string]map[string]bool
	baseLoaded := false
	changed := map[string]bool{}
	full := false
	for _, path := range paths {
		if path == "go.mod" || path == "go.sum" {
			full = true
			break
		}
	}
	for _, path := range paths {
		if full {
			break
		}
		_, before := beforeEntries[path]
		_, after := afterEntries[path]
		if !before && !after {
			continue
		}
		owner := nearestPackageOwner(path, imports, nil)
		if owner == "" && !baseLoaded {
			baseDir, closeBase, bedErr := s.openSnapshot(baseTree)
			if bedErr != nil {
				return selection, fmt.Errorf("materialize Go package base: %w", bedErr)
			}
			defer func() { err = errors.Join(err, closeBase()) }()
			baseImports, err = packageImports(baseDir, selection.ModulePath)
			if err != nil {
				return selection, err
			}
			baseLoaded = true
		}
		if owner == "" {
			owner = nearestPackageOwner(path, imports, baseImports)
		}
		if owner == "" {
			// An asset with no local owner can still be opened by a test.
			// Conservatively include the current inventory, which can reuse
			// individual package results whose real inputs did not change.
			full = true
			continue
		}
		changed[runnablePattern(owner, runnable)] = true
	}
	if full {
		changed = map[string]bool{"./...": true}
	}
	for pkg := range changed {
		selection.Changed = append(selection.Changed, pkg)
	}
	sort.Strings(selection.Changed)
	selection.Dependents = reverseDependents(selection.ModulePath, selection.Changed, imports)
	patterns := append(append([]string{}, selection.Changed...), selection.Dependents...)
	for pkg := range runnable {
		if matchesAny(pkg, patterns) {
			selection.Packages = append(selection.Packages, pkg)
		}
	}
	sort.Strings(selection.Packages)
	selection.InputDirs = make(map[string][]string, len(selection.Packages))
	for _, pkg := range selection.Packages {
		selection.InputDirs[pkg] = dependencyDirs(selection.ModulePath, pkg, imports)
	}
	return selection, nil
}

func nearestPackageOwner(path string, candidate, base map[string]map[string]bool) string {
	dir := filepath.ToSlash(filepath.Dir(path))
	for {
		pkg := "."
		if dir != "." {
			pkg = "./" + dir
		}
		if candidate[pkg] != nil || base[pkg] != nil {
			return pkg
		}
		if dir == "." {
			return ""
		}
		dir = filepath.ToSlash(filepath.Dir(dir))
	}
}

func runnablePattern(owner string, inventory map[string]bool) string {
	if inventory[owner] {
		return owner
	}
	dir := strings.TrimPrefix(owner, "./")
	for {
		if dir == "." {
			return "./..."
		}
		dir = filepath.ToSlash(filepath.Dir(dir))
		pattern := "./..."
		if dir != "." {
			pattern = "./" + dir + "/..."
		}
		for pkg := range inventory {
			if matchesAny(pkg, []string{pattern}) {
				return pattern
			}
		}
	}
}

func runnablePackageInventory(root string, buildTags, environment []string) (map[string]bool, error) {
	context := build.Default
	// The proof environment may scrub ambient GOOS/GOARCH; absent explicit
	// values make the native go tool target this host, not the caller shell.
	context.GOOS, context.GOARCH = runtime.GOOS, runtime.GOARCH
	context.BuildTags = append([]string(nil), buildTags...)
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch name {
		case "GOOS":
			if value != "" {
				context.GOOS = value
			}
		case "GOARCH":
			if value != "" {
				context.GOARCH = value
			}
		case "CGO_ENABLED":
			if value == "" {
				continue
			}
			if value != "0" && value != "1" {
				return nil, fmt.Errorf("go package selector cannot resolve CGO_ENABLED=%q", value)
			}
			context.CgoEnabled = value == "1"
		case "GOEXPERIMENT":
			if value != "" {
				return nil, fmt.Errorf("go package selector cannot resolve GOEXPERIMENT=%q", value)
			}
		case "GOFLAGS":
			if strings.Contains(value, "-tags") && strings.ContainsAny(value, "'\"\\") {
				return nil, fmt.Errorf("go package selector cannot resolve quoted GOFLAGS tags")
			}
			flags := strings.Fields(value)
			for index := 0; index < len(flags); index++ {
				flag := flags[index]
				if flag == "-overlay" || strings.HasPrefix(flag, "-overlay=") || flag == "-modfile" || strings.HasPrefix(flag, "-modfile=") {
					return nil, fmt.Errorf("go package selector cannot resolve GOFLAGS %s", flag)
				}
				if flag != "-tags" && !strings.HasPrefix(flag, "-tags=") {
					continue
				}
				tags := strings.TrimPrefix(flag, "-tags=")
				if flag == "-tags" {
					index++
					if index == len(flags) {
						return nil, fmt.Errorf("go package selector cannot resolve GOFLAGS tags")
					}
					tags = flags[index]
				}
				if len(buildTags) == 0 {
					for _, tag := range strings.Split(tags, ",") {
						if tag == "" {
							continue
						}
						if !validGoSelectorTag(tag) {
							return nil, fmt.Errorf("go package selector cannot resolve GOFLAGS tag %q", tag)
						}
						context.BuildTags = append(context.BuildTags, tag)
					}
				}
			}
		}
	}
	inventory := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == "vendor" || entry.Name() == ".git") {
				return filepath.SkipDir
			}
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(statErr) {
					return statErr
				}
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			return nil
		}
		relative, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := "."
		if relative != "." {
			pkg = "./" + filepath.ToSlash(relative)
		}
		if !runnableGoPackage(pkg) || inventory[pkg] {
			return nil
		}
		if !context.CgoEnabled {
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if parseErr != nil {
				return fmt.Errorf("read Go imports in %s: %w", path, parseErr)
			}
			cgoOnly := false
			for _, imported := range file.Imports {
				cgoOnly = cgoOnly || imported.Path.Value == `"C"`
			}
			if cgoOnly {
				return nil
			}
		}
		matches, err := context.MatchFile(filepath.Dir(path), name)
		if err != nil {
			return fmt.Errorf("match Go build constraints in %s: %w", path, err)
		}
		if matches {
			inventory[pkg] = true
		} else {
			uncertain, err := mayMatchUnprovenGoToolTag(path, context)
			if err != nil {
				return err
			}
			if uncertain {
				// A different declared Go tool may add release, compiler, or
				// architecture feature tags. Run its native discovery rather
				// than silently treating this package as absent.
				inventory[pkg] = true
			}
		}
		return nil
	})
	return inventory, err
}

func validGoSelectorTag(tag string) bool {
	for _, character := range tag {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_' && character != '.' {
			return false
		}
	}
	return tag != ""
}

func mayMatchUnprovenGoToolTag(path string, context build.Context) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var goBuild constraint.Expr
	var legacy []constraint.Expr
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			break
		}
		if !strings.HasPrefix(line, "//go:build ") && !strings.HasPrefix(line, "// +build ") {
			continue
		}
		expression, err := constraint.Parse(line)
		if err != nil {
			return false, fmt.Errorf("parse Go build constraint in %s: %w", path, err)
		}
		if strings.HasPrefix(line, "//go:build ") {
			goBuild = expression
		} else {
			legacy = append(legacy, expression)
		}
	}
	if goBuild != nil {
		possible, unknown := possibleGoConstraint(goBuild, context)
		return possible && unknown, nil
	}
	possible, unknown := true, false
	for _, expression := range legacy {
		may, hasUnknown := possibleGoConstraint(expression, context)
		possible, unknown = possible && may, unknown || hasUnknown
	}
	return possible && unknown, nil
}

// possibleGoConstraint evaluates known platform/user tags exactly and treats
// declared-tool release/compiler/feature tags as unknown. The conservative
// result may include an extra package, but cannot ignore a runnable one.
func possibleGoConstraint(expression constraint.Expr, context build.Context) (canBeTrue, hasUnknown bool) {
	switch part := expression.(type) {
	case *constraint.TagExpr:
		tag := part.Tag
		if strings.HasPrefix(tag, "go1.") || strings.HasPrefix(tag, "goexperiment.") {
			return true, true
		}
		for _, architecture := range []string{"386", "amd64", "arm", "arm64", "loong64", "mips", "mips64", "mips64le", "mipsle", "ppc64", "ppc64le", "riscv64", "s390x", "wasm"} {
			if strings.HasPrefix(tag, architecture+".") {
				return true, true
			}
		}
		switch tag {
		case "gc", "gccgo", "race", "msan", "asan", "purego", "boringcrypto":
			return true, true
		}
		if tag == context.GOOS || tag == context.GOARCH || tag == "cgo" && context.CgoEnabled ||
			context.GOOS == "android" && tag == "linux" || context.GOOS == "illumos" && tag == "solaris" ||
			context.GOOS == "ios" && tag == "darwin" || tag == "unix" && isUnixGOOS(context.GOOS) {
			return true, false
		}
		for _, declared := range context.BuildTags {
			if declared == tag {
				return true, false
			}
		}
		return false, false
	case *constraint.NotExpr:
		possible, unknown := possibleGoConstraint(part.X, context)
		if unknown {
			return true, true
		}
		return !possible, false
	case *constraint.AndExpr:
		left, leftUnknown := possibleGoConstraint(part.X, context)
		right, rightUnknown := possibleGoConstraint(part.Y, context)
		return left && right, leftUnknown || rightUnknown
	case *constraint.OrExpr:
		left, leftUnknown := possibleGoConstraint(part.X, context)
		right, rightUnknown := possibleGoConstraint(part.Y, context)
		return left || right, leftUnknown || rightUnknown
	}
	return false, false
}

func isUnixGOOS(goos string) bool {
	switch goos {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "linux", "netbsd", "openbsd", "solaris":
		return true
	}
	return false
}

func dependencyDirs(module, root string, imports map[string]map[string]bool) []string {
	seen := map[string]bool{root: true}
	queue := []string{root}
	for len(queue) != 0 {
		pkg := queue[0]
		queue = queue[1:]
		for imported := range imports[pkg] {
			dependency := "."
			if imported != module {
				dependency = "./" + strings.TrimPrefix(imported, module+"/")
			}
			if !seen[dependency] && imports[dependency] != nil {
				seen[dependency] = true
				queue = append(queue, dependency)
			}
		}
	}
	result := make([]string, 0, len(seen))
	for pkg := range seen {
		result = append(result, pkg)
	}
	sort.Strings(result)
	return result
}

// ReverseDependents preserves the join gate's exact-tree consumer closure for
// callers that already have a changed package pattern set.
func ReverseDependents(moduleRoot, tree string, changed []string) (dependents []string, err error) {
	workspace := gittree.Workspace{Dir: moduleRoot}
	resolved := tree
	if _, present, readErr := workspace.FileAt(tree, "go.mod"); readErr != nil || !present {
		resolved, err = workspace.TreeOf(tree)
		if err != nil {
			return nil, err
		}
	}
	goMod, present, err := workspace.FileAt(resolved, "go.mod")
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("reverse dependents: candidate has no go.mod")
	}
	module, err := modulePath(goMod)
	if err != nil {
		return nil, err
	}
	detached, err := workspace.NewDetachedWorktree(resolved)
	if err != nil {
		return nil, fmt.Errorf("reverse dependents: materialize tree: %w", err)
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	imports, err := packageImports(detached.Workspace().Dir, module)
	if err != nil {
		return nil, err
	}
	return reverseDependents(module, changed, imports), nil
}

// go list ./... does not enumerate packages under these directory names.
// Keep their imports in the reverse-dependency graph, but do not try to run
// their fixture sources as standalone package tests.
func runnableGoPackage(pkg string) bool {
	if pkg == "." {
		return true
	}
	for _, component := range strings.Split(strings.TrimPrefix(pkg, "./"), "/") {
		if component == "testdata" || component == "vendor" || strings.HasPrefix(component, ".") || strings.HasPrefix(component, "_") {
			return false
		}
	}
	return true
}

func modulePath(data []byte) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		path := fields[1]
		if strings.HasPrefix(path, "\"") || strings.HasPrefix(path, "`") {
			var err error
			path, err = strconv.Unquote(path)
			if err != nil {
				return "", err
			}
		}
		if path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("go.mod has no module directive")
}

func packageImports(root, module string) (map[string]map[string]bool, error) {
	imports := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == "vendor" || entry.Name() == ".git") {
				return filepath.SkipDir
			}
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(statErr) {
					return statErr
				}
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasPrefix(entry.Name(), "_") || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := "."
		if rel != "." {
			pkg = "./" + filepath.ToSlash(rel)
		}
		if imports[pkg] == nil {
			imports[pkg] = map[string]bool{}
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse Go imports in %s: %w", path, err)
		}
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if imported == module || strings.HasPrefix(imported, module+"/") {
				imports[pkg][imported] = true
			}
		}
		return nil
	})
	return imports, err
}

func reverseDependents(module string, changed []string, imports map[string]map[string]bool) []string {
	dependent := map[string]bool{}
	for {
		added := false
		for pkg, used := range imports {
			if dependent[pkg] || matchesAny(pkg, changed) {
				continue
			}
			for imported := range used {
				rel := "."
				if imported != module {
					rel = "./" + strings.TrimPrefix(imported, module+"/")
				}
				if matchesAny(rel, changed) || dependent[rel] {
					dependent[pkg], added = true, true
					break
				}
			}
		}
		if !added {
			break
		}
	}
	var result []string
	for pkg := range dependent {
		result = append(result, pkg)
	}
	sort.Strings(result)
	return result
}

func matchesAny(pkg string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == pkg || pattern == "./..." || pattern == "..." {
			return true
		}
		if strings.HasSuffix(pattern, "/...") {
			prefix := strings.TrimSuffix(pattern, "/...")
			if pkg == prefix || strings.HasPrefix(pkg, prefix+"/") {
				return true
			}
		}
	}
	return false
}
