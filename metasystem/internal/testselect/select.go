// Package testselect decides which Go packages a set of changed files reaches.
package testselect

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Mode int

const (
	Skip Mode = iota
	Whole
	Named
)

type PackageDecision struct {
	Package string
	Mode    Mode
	Tests   []string
	Tag     string
	Reason  string
}

type Results struct{}

type Request struct {
	ModuleRoot string
	Base       string
	Head       string
	Changed    []string
	Previous   *Results
}

type Selection struct {
	Packages []PackageDecision
}

// Select applies the full package-selection rule to a module change.
func Select(req Request) (Selection, error) {
	return selectWithRunner(req, execRunner{})
}

type commandRunner interface {
	Run(dir, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(dir, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	return command.CombinedOutput()
}

type packageInfo struct {
	path string
	dir  string
}

func selectWithRunner(req Request, runner commandRunner) (Selection, error) {
	if req.Previous != nil {
		return Selection{}, fmt.Errorf("incremental test selection is unavailable: U1b is not built yet")
	}
	root := filepath.Clean(req.ModuleRoot)
	packages, err := discoverPackages(root)
	if err != nil {
		return Selection{}, err
	}
	changed := req.Changed
	if changed == nil {
		changed, err = changedFromGit(root, req.Base, req.Head, runner)
		if err != nil {
			return Selection{}, err
		}
	}
	changed = normalizeChanged(changed)

	if containsModuleFile(changed) {
		return decisionsForAll(packages, Whole, "module files changed"), nil
	}

	packageByDir := make(map[string]string, len(packages))
	packageSet := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		packageByDir[pkg.dir] = pkg.path
		packageSet[pkg.path] = true
	}

	changedPackages := make(map[string]bool)
	linkedChanges := make(map[string]bool)
	for _, changedFile := range changed {
		owner := owningPackage(changedFile, packageByDir)
		if owner == "" {
			continue
		}
		changedPackages[owner] = true
		if !strings.HasSuffix(changedFile, "_test.go") {
			linkedChanges[owner] = true
		}
	}

	importReasons := make(map[string][]string)
	if len(linkedChanges) > 0 {
		modulePath, readErr := readModulePath(filepath.Join(root, "go.mod"))
		if readErr != nil {
			return Selection{}, readErr
		}
		dependencies, listErr := listTestDependencies(root, modulePath, packageSet, runner)
		if listErr != nil {
			return Selection{}, listErr
		}
		for pkg, deps := range dependencies {
			for changedPackage := range linkedChanges {
				if deps[changedPackage] {
					importReasons[pkg] = append(importReasons[pkg], changedPackage)
				}
			}
			sort.Strings(importReasons[pkg])
		}
	}

	anchorReasons, err := findAnchorReaders(root, packages, changed)
	if err != nil {
		return Selection{}, err
	}

	decisions := make([]PackageDecision, 0, len(packages))
	for _, pkg := range packages {
		decision := PackageDecision{Package: pkg.path, Mode: Skip, Tag: "plain", Reason: "not reached by changes"}
		switch {
		case changedPackages[pkg.path]:
			decision.Mode = Whole
			decision.Reason = "changed"
		case len(importReasons[pkg.path]) > 0:
			decision.Mode = Whole
			decision.Reason = "imports " + strings.Join(importReasons[pkg.path], ", ")
		case len(anchorReasons[pkg.path]) > 0:
			decision.Mode = Whole
			decision.Reason = "names a changed file: " + strings.Join(anchorReasons[pkg.path], ", ")
		}
		decisions = append(decisions, decision)
	}
	return Selection{Packages: decisions}, nil
}

func discoverPackages(root string) ([]packageInfo, error) {
	packageDirs := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			packageDirs[filepath.Clean(filepath.Dir(path))] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Go packages: %w", err)
	}
	packages := make([]packageInfo, 0, len(packageDirs))
	for dir := range packageDirs {
		relative, relErr := filepath.Rel(root, dir)
		if relErr != nil {
			return nil, fmt.Errorf("make package path relative: %w", relErr)
		}
		path := "./"
		if relative != "." {
			path += filepath.ToSlash(relative)
		}
		packages = append(packages, packageInfo{path: path, dir: filepath.Clean(relative)})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].path < packages[j].path })
	return packages, nil
}

func skipDirectory(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "testdata", "vendor", "artifacts", "bin":
		return true
	default:
		return false
	}
}

func decisionsForAll(packages []packageInfo, mode Mode, reason string) Selection {
	decisions := make([]PackageDecision, 0, len(packages))
	for _, pkg := range packages {
		decisions = append(decisions, PackageDecision{Package: pkg.path, Mode: mode, Tag: "plain", Reason: reason})
	}
	return Selection{Packages: decisions}
}

func normalizeChanged(changed []string) []string {
	set := make(map[string]bool, len(changed))
	for _, name := range changed {
		name = filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
		if name == "." || name == "" || name == ".." || strings.HasPrefix(name, "../") || filepath.IsAbs(name) {
			continue
		}
		set[name] = true
	}
	result := make([]string, 0, len(set))
	for name := range set {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func containsModuleFile(changed []string) bool {
	for _, name := range changed {
		if name == "go.mod" || name == "go.sum" {
			return true
		}
	}
	return false
}

func owningPackage(changedFile string, packageByDir map[string]string) string {
	dir := filepath.Clean(filepath.Dir(filepath.FromSlash(changedFile)))
	for {
		if pkg := packageByDir[dir]; pkg != "" {
			return pkg
		}
		if dir == "." {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == ".." {
			return ""
		}
		dir = parent
	}
}

func changedFromGit(root, base, head string, runner commandRunner) ([]string, error) {
	prefixOutput, err := runner.Run(root, "git", "rev-parse", "--show-prefix")
	if err != nil {
		return nil, commandError("git rev-parse --show-prefix", prefixOutput, err)
	}
	prefix := filepath.ToSlash(strings.TrimSpace(string(prefixOutput)))
	args := []string{"diff", "--name-only", "--no-renames", base}
	if head != "" {
		args = append(args, head)
	}
	args = append(args, "--")
	output, err := runner.Run(root, "git", args...)
	if err != nil {
		return nil, commandError("git diff", output, err)
	}
	var changed []string
	for _, line := range strings.Split(string(output), "\n") {
		name := filepath.ToSlash(strings.TrimSpace(line))
		if name == "" {
			continue
		}
		if prefix != "" {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = strings.TrimPrefix(name, prefix)
		}
		changed = append(changed, name)
	}
	return changed, nil
}

func commandError(command string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", command, err)
	}
	return fmt.Errorf("%s: %w: %s", command, err, detail)
}

func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read module path: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		modulePath := fields[1]
		if unquoted, unquoteErr := strconv.Unquote(modulePath); unquoteErr == nil {
			modulePath = unquoted
		}
		if modulePath != "" {
			return modulePath, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read module path: %w", err)
	}
	return "", fmt.Errorf("read module path: module directive missing from %s", path)
}

func listTestDependencies(root, modulePath string, packageSet map[string]bool, runner commandRunner) (map[string]map[string]bool, error) {
	output, err := runner.Run(root, "go", "list", "-e", "-test", "-f", `{{.ImportPath}}|{{join .Deps ","}}`, "./...")
	if err != nil {
		return nil, commandError("go list test dependencies", output, err)
	}
	dependencies := make(map[string]map[string]bool)
	for _, rawLine := range bytes.Split(output, []byte{'\n'}) {
		line := string(rawLine)
		if strings.TrimSpace(line) == "" {
			continue
		}
		importPath, rawDeps, ok := strings.Cut(line, "|")
		if !ok {
			return nil, fmt.Errorf("go list test dependencies: malformed output line %q", line)
		}
		pkg, ok := moduleRelativePackage(modulePath, normalizeListedImport(importPath))
		if !ok || !packageSet[pkg] {
			continue
		}
		if dependencies[pkg] == nil {
			dependencies[pkg] = make(map[string]bool)
		}
		if rawDeps == "" {
			continue
		}
		for _, rawDependency := range strings.Split(rawDeps, ",") {
			dependency, inside := moduleRelativePackage(modulePath, normalizeListedImport(rawDependency))
			if inside {
				dependencies[pkg][dependency] = true
			}
		}
	}
	return dependencies, nil
}

func normalizeListedImport(importPath string) string {
	importPath = strings.TrimSpace(importPath)
	if variant := strings.Index(importPath, " ["); variant >= 0 {
		importPath = importPath[:variant]
	}
	importPath = strings.TrimSuffix(importPath, ".test")
	importPath = strings.TrimSuffix(importPath, "_test")
	return importPath
}

func moduleRelativePackage(modulePath, importPath string) (string, bool) {
	if importPath == modulePath {
		return "./", true
	}
	prefix := modulePath + "/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", false
	}
	return "./" + strings.TrimPrefix(importPath, prefix), true
}

func findAnchorReaders(root string, packages []packageInfo, changed []string) (map[string][]string, error) {
	basenames := make(map[string]bool)
	for _, name := range changed {
		base := filepath.Base(filepath.FromSlash(name))
		if base != "." && base != "" {
			basenames[base] = true
		}
	}
	readers := make(map[string][]string)
	for _, pkg := range packages {
		dir := root
		if pkg.dir != "." {
			dir = filepath.Join(root, pkg.dir)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("read package %s: %w", pkg.path, err)
		}
		matched := make(map[string]bool)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				return nil, fmt.Errorf("parse anchors in %s: %w", path, parseErr)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, unquoteErr := strconv.Unquote(literal.Value)
				if unquoteErr != nil {
					return true
				}
				for base := range basenames {
					if containsFileLine(value, base) {
						matched[base] = true
					}
				}
				return true
			})
		}
		for base := range matched {
			readers[pkg.path] = append(readers[pkg.path], base)
		}
		sort.Strings(readers[pkg.path])
	}
	return readers, nil
}

func containsFileLine(literal, basename string) bool {
	anchor := basename + ":"
	for offset := 0; offset < len(literal); {
		index := strings.Index(literal[offset:], anchor)
		if index < 0 {
			return false
		}
		lineStart := offset + index + len(anchor)
		if lineStart < len(literal) && literal[lineStart] >= '0' && literal[lineStart] <= '9' {
			return true
		}
		offset = lineStart
	}
	return false
}
