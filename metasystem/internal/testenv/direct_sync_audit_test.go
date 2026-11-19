package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// syncPlatform is one configuration the module is built for. The module has
// no build tags of its own; a new one fails the audit until it gets a
// configuration here.
type syncPlatform struct{ goos, goarch string }

func (p syncPlatform) String() string { return p.goos + "/" + p.goarch }

// directSyncPlatforms are the platforms the module builds for: the host
// (darwin) and the two Linux architectures the go gate cross-builds.
var directSyncPlatforms = []syncPlatform{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}}

// directSyncExempt is the adapter itself, the one package that calls
// File.Sync.
const directSyncExempt = "internal/durable"

// Every sync of the engine goes through internal/durable, so a Go test
// process syncs at its own level and a production binary at full (design
// a-test-does-not-flush-the-drive, D1 and D6). The audit type-checks every
// non-test file of the module once per platform and refuses any selection of
// the method (*os.File).Sync outside the adapter: a call, a method value, a
// method expression, through an alias or an embedded *os.File. A Sync
// selector it cannot resolve, a file no platform builds that names Sync, and
// a build tag it has no configuration for are refused as well. A Sync reached
// through an interface, a raw syscall or a synchronous open flag is outside
// its sight.
func TestNoEngineCodeSyncsAFileOutsideTheAdapter(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := findDirectSyncs(context.Background(), module, directSyncPlatforms, directSyncExempt)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		t.Errorf("%s: call durable.Sync or durable.SyncDir instead of (*os.File).Sync", finding)
	}
}

func TestDirectSyncAuditFindsEveryShapeOnEveryPlatform(t *testing.T) {
	t.Parallel()
	findings, err := findDirectSyncs(context.Background(), filepath.Join("testdata", "directsync"), directSyncPlatforms, directSyncExempt)
	if err != nil {
		t.Fatal(err)
	}
	all := "[darwin/amd64 darwin/arm64 linux/amd64 linux/arm64]"
	want := []string{
		"direct/direct.go:11: call " + all,
		"direct/direct.go:15: method value " + all,
		"direct/direct.go:19: method expression " + all,
		"direct/direct.go:26: call " + all,
		"direct/direct.go:33: call " + all,
		"direct/direct.go:39: call " + all,
		"direct/direct_darwin.go:7: call [darwin/amd64 darwin/arm64]",
		"direct/direct_linux.go:7: call [linux/amd64 linux/arm64]",
		"direct/direct_other.go:9: Sync selector in a file no audited platform builds",
		"direct/tagged.go: build tag fixturetag has no audit configuration",
	}
	if !slices.Equal(findings, want) {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(findings, "\n"), strings.Join(want, "\n"))
	}
}

func TestDirectSyncAuditRefusesWhatItCannotTypeCheck(t *testing.T) {
	t.Parallel()
	_, err := findDirectSyncs(context.Background(), filepath.Join("testdata", "directsyncbroken"), directSyncPlatforms, directSyncExempt)
	if err == nil || !strings.Contains(err.Error(), "cannot type-check example.test/directsyncbroken/broken on linux/amd64") {
		t.Fatalf("a package that does not type-check on one platform: err = %v", err)
	}
}

type syncListedPackage struct {
	ImportPath     string
	Dir            string
	Name           string
	GoFiles        []string
	CgoFiles       []string
	IgnoredGoFiles []string
	Imports        []string
	ImportMap      map[string]string
	Export         string
	Standard       bool
	Module         *struct {
		Path, Dir, GoVersion string
		Main                 bool
	}
	Error      *struct{ Err string }
	DepsErrors []*struct{ Err string }
}

// syncFinding is one refused site, with the platforms that build it.
type syncFinding struct {
	file, shape string
	line        int
	platforms   []string
}

// findDirectSyncs returns the refused sites of the module in dir, ordered by
// file and line, each with the platforms that build it. It returns an error
// when it cannot list or type-check a package on some platform.
func findDirectSyncs(ctx context.Context, dir string, platforms []syncPlatform, exempt string) ([]string, error) {
	found := map[string]*syncFinding{}
	add := func(file string, line int, shape, platform string) {
		key := fmt.Sprintf("%s\x00%08d\x00%s", file, line, shape)
		f := found[key]
		if f == nil {
			f = &syncFinding{file: file, line: line, shape: shape}
			found[key] = f
		}
		if platform != "" && !slices.Contains(f.platforms, platform) {
			f.platforms = append(f.platforms, platform)
		}
	}
	type checkedConfig struct {
		signature string
		platforms []string
	}
	var configs []*checkedConfig
	// Every non-test file of a module package, built or not, keyed by its
	// path relative to the module, and the files some platform type-checks.
	every, built := map[string]string{}, map[string]bool{}
	var moduleDir string
	for _, platform := range platforms {
		listed, err := listForSyncAudit(ctx, dir, platform, "-e", "-json", "-deps", "./...")
		if err != nil {
			return nil, err
		}
		modulePackages, deps, signature, err := splitSyncListing(listed, platform)
		if err != nil {
			return nil, err
		}
		for _, p := range modulePackages {
			moduleDir = p.Module.Dir
			for _, name := range append(slices.Clone(p.GoFiles), p.IgnoredGoFiles...) {
				if strings.HasSuffix(name, "_test.go") {
					continue
				}
				relative, err := filepath.Rel(moduleDir, filepath.Join(p.Dir, name))
				if err != nil {
					return nil, err
				}
				every[filepath.ToSlash(relative)] = filepath.Join(p.Dir, name)
			}
		}
		// Platforms that build the same files of the module are checked once.
		index := slices.IndexFunc(configs, func(c *checkedConfig) bool { return c.signature == signature })
		if index >= 0 {
			configs[index].platforms = append(configs[index].platforms, platform.String())
			continue
		}
		configs = append(configs, &checkedConfig{signature: signature, platforms: []string{platform.String()}})
		sites, files, err := checkForDirectSyncs(ctx, dir, platform, modulePackages, deps, exempt)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			built[file] = true
		}
		for _, site := range sites {
			add(site.file, site.line, site.shape, "@"+fmt.Sprint(len(configs)-1))
		}
	}
	if moduleDir == "" {
		return nil, fmt.Errorf("direct sync audit found no module packages in %s", dir)
	}
	// A file no platform builds cannot be resolved: any Sync selector in it
	// is refused. Every file's build tags must be ones a platform decides.
	for relative, path := range every {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("direct sync audit: %w", err)
		}
		for _, tag := range unauditedBuildTags(file) {
			add(relative, 0, "build tag "+tag+" has no audit configuration", "")
		}
		if built[relative] || exemptFile(relative, exempt) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Sync" {
				add(relative, fset.Position(selector.Sel.Pos()).Line, "Sync selector in a file no audited platform builds", "")
			}
			return true
		})
	}
	var findings []string
	keys := make([]string, 0, len(found))
	for key := range found {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		f := found[key]
		var names []string
		for _, platform := range f.platforms {
			var config int
			_, _ = fmt.Sscanf(platform, "@%d", &config)
			names = append(names, configs[config].platforms...)
		}
		sort.Strings(names)
		location := f.file
		if f.line > 0 {
			location = fmt.Sprintf("%s:%d", f.file, f.line)
		}
		finding := location + ": " + f.shape
		if len(names) > 0 {
			finding += " [" + strings.Join(names, " ") + "]"
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

func exemptFile(relative, exempt string) bool { return pathDir(relative) == exempt }

func pathDir(relative string) string { return filepath.ToSlash(filepath.Dir(relative)) }

// listForSyncAudit runs go list for one platform, without cgo, with no build
// tags whatever GOFLAGS says, and outside any workspace.
func listForSyncAudit(ctx context.Context, dir string, platform syncPlatform, args ...string) ([]syncListedPackage, error) {
	command := GoContext(ctx, append([]string{"list", "-tags="}, args...)...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOOS="+platform.goos, "GOARCH="+platform.goarch, "CGO_ENABLED=0", "GOWORK=off")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("direct sync audit: go list on %s failed: %w: %s", platform, err, strings.TrimSpace(stderr.String()))
	}
	var listed []syncListedPackage
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var p syncListedPackage
		if err := decoder.Decode(&p); err == io.EOF {
			return listed, nil
		} else if err != nil {
			return nil, fmt.Errorf("direct sync audit: go list output on %s unreadable: %w", platform, err)
		}
		listed = append(listed, p)
	}
}

// splitSyncListing separates the module's packages from their dependencies
// and returns a signature of the module files the platform builds. A module
// package that does not load, or has cgo files the audit cannot check from
// source, is an error; one whose files are all excluded on this platform is
// kept for its ignored files.
func splitSyncListing(listed []syncListedPackage, platform syncPlatform) (module []syncListedPackage, deps []string, signature string, err error) {
	var lines []string
	for _, p := range listed {
		if p.Standard || p.Module == nil || !p.Module.Main {
			if p.Error != nil {
				return nil, nil, "", fmt.Errorf("direct sync audit cannot load %s on %s: %s", p.ImportPath, platform, p.Error.Err)
			}
			deps = append(deps, p.ImportPath)
			continue
		}
		if p.Error != nil && !strings.Contains(p.Error.Err, "build constraints exclude all Go files") {
			return nil, nil, "", fmt.Errorf("direct sync audit cannot load %s on %s: %s", p.ImportPath, platform, p.Error.Err)
		}
		if len(p.DepsErrors) > 0 {
			return nil, nil, "", fmt.Errorf("direct sync audit cannot load %s on %s: %s", p.ImportPath, platform, p.DepsErrors[0].Err)
		}
		if len(p.CgoFiles) > 0 {
			return nil, nil, "", fmt.Errorf("direct sync audit cannot check the cgo files of %s on %s", p.ImportPath, platform)
		}
		module = append(module, p)
		lines = append(lines, p.ImportPath+"\x00"+strings.Join(p.GoFiles, "\x00"))
	}
	sort.Strings(lines)
	return module, deps, strings.Join(lines, "\n"), nil
}

// syncSite is one selection of (*os.File).Sync, or one Sync selector the
// type checker did not resolve.
type syncSite struct {
	file, shape string
	line        int
}

type syncFuture struct {
	done  chan struct{}
	pkg   *types.Package
	err   error
	sites []syncSite
	files []string
}

// checkForDirectSyncs type-checks the module's packages from source on one
// platform, dependencies first, reading every other package from the export
// data go list compiles for that platform. It returns the sites and the
// files it checked, relative to the module.
func checkForDirectSyncs(ctx context.Context, dir string, platform syncPlatform, module []syncListedPackage, deps []string, exempt string) ([]syncSite, []string, error) {
	exports := map[string]string{}
	if len(deps) > 0 {
		listed, err := listForSyncAudit(ctx, dir, platform, append([]string{"-e", "-json", "-export"}, deps...)...)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range listed {
			if p.ImportPath == "unsafe" {
				continue // the importer knows it
			}
			if p.Error != nil || p.Export == "" {
				return nil, nil, fmt.Errorf("direct sync audit has no export data for %s on %s", p.ImportPath, platform)
			}
			exports[p.ImportPath] = p.Export
		}
	}
	fset := token.NewFileSet()
	var mu sync.Mutex
	gc := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		if exports[path] == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(exports[path])
	})
	futures := map[string]*syncFuture{}
	for _, p := range module {
		futures[p.ImportPath] = &syncFuture{done: make(chan struct{})}
	}
	workers := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for _, p := range module {
		future := futures[p.ImportPath]
		wg.Go(func() {
			defer close(future.done)
			if len(p.GoFiles) == 0 {
				// Excluded on this platform: a use of it fails its importer's check.
				future.pkg = types.NewPackage(p.ImportPath, p.Name)
				return
			}
			for _, imported := range p.Imports {
				if dep := futures[imported]; dep != nil {
					<-dep.done
					if dep.err != nil {
						future.err = fmt.Errorf("direct sync audit skipped %s on %s: its import %s failed", p.ImportPath, platform, imported)
						return
					}
				}
			}
			workers <- struct{}{}
			defer func() { <-workers }()
			future.pkg, future.sites, future.files, future.err = checkSyncPackage(fset, p, platform, futures, &mu, gc, exempt)
		})
	}
	wg.Wait()
	var sites []syncSite
	var files []string
	var errs []error
	for _, p := range module {
		future := futures[p.ImportPath]
		if future.err != nil {
			errs = append(errs, future.err)
		}
		sites = append(sites, future.sites...)
		files = append(files, future.files...)
	}
	return sites, files, errors.Join(errs...)
}

type syncImporter struct {
	futures map[string]*syncFuture
	mu      *sync.Mutex
	gc      types.Importer
	mapped  map[string]string
}

func (i syncImporter) Import(path string) (*types.Package, error) {
	if mapped, ok := i.mapped[path]; ok {
		path = mapped
	}
	if future := i.futures[path]; future != nil {
		return future.pkg, nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.gc.Import(path)
}

func checkSyncPackage(fset *token.FileSet, p syncListedPackage, platform syncPlatform, futures map[string]*syncFuture, mu *sync.Mutex, gc types.Importer, exempt string) (*types.Package, []syncSite, []string, error) {
	var files []*ast.File
	var relatives []string
	for _, name := range p.GoFiles {
		path := filepath.Join(p.Dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("direct sync audit: %w", err)
		}
		relative, err := filepath.Rel(p.Module.Dir, path)
		if err != nil {
			return nil, nil, nil, err
		}
		files = append(files, file)
		relatives = append(relatives, filepath.ToSlash(relative))
	}
	var problems []error
	config := types.Config{
		Importer:  syncImporter{futures: futures, mu: mu, gc: gc, mapped: p.ImportMap},
		GoVersion: "go" + p.Module.GoVersion,
		Sizes:     types.SizesFor("gc", platform.goarch),
		Error:     func(err error) { problems = append(problems, err) },
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	pkg, _ := config.Check(p.ImportPath, fset, files, info)
	if len(problems) > 0 {
		if len(problems) > 5 {
			problems = problems[:5]
		}
		return nil, nil, nil, fmt.Errorf("direct sync audit cannot type-check %s on %s: %w", p.ImportPath, platform, errors.Join(problems...))
	}
	var sites []syncSite
	for index, file := range files {
		relative := relatives[index]
		if exemptFile(relative, exempt) {
			continue
		}
		called := map[ast.Expr]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				called[ast.Unparen(call.Fun)] = true
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Sync" {
				return true
			}
			line := fset.Position(selector.Sel.Pos()).Line
			selection, ok := info.Selections[selector]
			if !ok {
				if _, qualified := info.Uses[selector.Sel]; !qualified {
					sites = append(sites, syncSite{file: relative, line: line, shape: "unresolved Sync selector"})
				}
				return true
			}
			method, ok := selection.Obj().(*types.Func)
			if !ok || method.FullName() != "(*os.File).Sync" {
				return true
			}
			shape := "method value"
			switch {
			case selection.Kind() == types.MethodExpr:
				shape = "method expression"
			case called[selector]:
				shape = "call"
			}
			sites = append(sites, syncSite{file: relative, line: line, shape: shape})
			return true
		})
	}
	return pkg, sites, relatives, nil
}

// unauditedBuildTags returns the tags of a file's build constraint that are
// neither an operating system, an architecture, a Go version nor a tag the
// toolchain sets itself: a configuration the platforms do not decide.
func unauditedBuildTags(file *ast.File) []string {
	var tags []string
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, comment := range group.List {
			if !constraint.IsGoBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				tags = append(tags, "(unparsable constraint)")
				continue
			}
			for _, tag := range constraintTags(expr) {
				if !toolchainBuildTags[tag] && !strings.HasPrefix(tag, "go1.") && !slices.Contains(tags, tag) {
					tags = append(tags, tag)
				}
			}
		}
	}
	return tags
}

// constraintTags returns every tag of a constraint (Eval would stop early).
func constraintTags(expr constraint.Expr) []string {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		return []string{e.Tag}
	case *constraint.NotExpr:
		return constraintTags(e.X)
	case *constraint.AndExpr:
		return append(constraintTags(e.X), constraintTags(e.Y)...)
	case *constraint.OrExpr:
		return append(constraintTags(e.X), constraintTags(e.Y)...)
	}
	return nil
}

// toolchainBuildTags are the tags go/build itself decides: every known
// GOOS and GOARCH, unix, the compiler, cgo, and ignore.
var toolchainBuildTags = func() map[string]bool {
	known := map[string]bool{"unix": true, "gc": true, "gccgo": true, "cgo": true, "ignore": true}
	for _, name := range strings.Fields(`aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos
		386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm`) {
		known[name] = true
	}
	return known
}()
