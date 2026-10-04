package rootaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Site is one place where a state-root value reaches run state.
type Site struct {
	File     string // relative to the module directory
	Line     int
	Test     bool
	Kind     string // direct, flow or concat
	Function string
	Source   string // what carries the state root
	Sink     string // where it goes
	Witness  string // the run-state join the sink reaches
}

// Key is a site without its line: what the list records and counts.
type Key struct{ File, Function, Source, Sink string }

func (s Site) Key() Key { return Key{s.File, s.Function, s.Source, s.Sink} }

func (k Key) String() string {
	return fmt.Sprintf("%s %s: %s -> %s", k.File, k.Function, k.Source, k.Sink)
}

type listedPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Export     string
	ImportMap  map[string]string
	ForTest    string
	DepOnly    bool
	Standard   bool
	Module     *struct {
		Path, Dir, GoVersion string
		Main                 bool
	}
	Error      *struct{ Err string }
	DepsErrors []*struct{ Err string }
}

// Scan type-checks every package of the module in dir, with its tests, and
// returns every site where a state-root value reaches run state, ordered by
// file and line.
func Scan(ctx context.Context, dir string) ([]Site, error) {
	listed, err := list(ctx, dir)
	if err != nil {
		return nil, err
	}
	exports := map[string]string{}
	variants := map[string]listedPackage{}
	var module, moduleDir, goVersion string
	for _, p := range listed {
		exports[p.ImportPath] = p.Export
		if p.Standard || p.Module == nil || !p.Module.Main {
			continue
		}
		if p.Error != nil {
			return nil, fmt.Errorf("run-state audit cannot load %s: %s", p.ImportPath, p.Error.Err)
		}
		if len(p.DepsErrors) > 0 {
			return nil, fmt.Errorf("run-state audit cannot load %s: %s", p.ImportPath, p.DepsErrors[0].Err)
		}
		module, moduleDir, goVersion = p.Module.Path, p.Module.Dir, p.Module.GoVersion
		if p.DepOnly || strings.HasSuffix(p.ImportPath, ".test") {
			continue
		}
		// One variant per package: the test variant, whose files are the
		// package's own plus its in-package tests. An external test package
		// has its own path.
		path, _, _ := strings.Cut(p.ImportPath, " ")
		if prev, ok := variants[path]; ok && (len(prev.GoFiles) > len(p.GoFiles) || len(prev.GoFiles) == len(p.GoFiles) && p.ForTest == "") {
			continue
		}
		variants[path] = p
	}
	if module == "" {
		return nil, fmt.Errorf("run-state audit found no module packages in %s", dir)
	}
	paths := make([]string, 0, len(variants))
	for path := range variants {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	fset := token.NewFileSet()
	pkgs := make([]*checkedPackage, len(paths))
	errs := make([]error, len(paths))
	next := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range next {
				pkgs[i], errs[i] = check(fset, paths[i], variants[paths[i]], exports, goVersion)
			}
		})
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return newAnalyzer(moduleDir, module, pkgs).run(), nil
}

// list runs go list, which also compiles the export data every import is
// read from.
func list(ctx context.Context, dir string) ([]listedPackage, error) {
	command := exec.CommandContext(ctx, "go", "list", "-e", "-json", "-export", "-deps", "-test", "./...")
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("run-state audit: go list failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var listed []listedPackage
	decoder := json.NewDecoder(&stdout)
	for {
		var p listedPackage
		if err := decoder.Decode(&p); err == io.EOF {
			return listed, nil
		} else if err != nil {
			return nil, fmt.Errorf("run-state audit: go list output unreadable: %w", err)
		}
		listed = append(listed, p)
	}
}

// check type-checks one package variant from source, reading its imports
// from the export data go list reported for the variant each one names.
func check(fset *token.FileSet, path string, p listedPackage, exports map[string]string, goVersion string) (*checkedPackage, error) {
	files := make([]*ast.File, 0, len(p.GoFiles))
	for _, name := range p.GoFiles {
		file, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("run-state audit: %w", err)
		}
		files = append(files, file)
	}
	lookup := func(imported string) (io.ReadCloser, error) {
		if mapped, ok := p.ImportMap[imported]; ok {
			imported = mapped
		}
		if exports[imported] == "" {
			return nil, fmt.Errorf("no export data for %s", imported)
		}
		return os.Open(exports[imported])
	}
	var problems []error
	config := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup), GoVersion: "go" + goVersion,
		Error: func(err error) { problems = append(problems, err) }}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	_, _ = config.Check(path, fset, files, info)
	if len(problems) > 0 {
		return nil, fmt.Errorf("run-state audit cannot type-check %s: %w", p.ImportPath, errors.Join(problems...))
	}
	return &checkedPackage{path: path, fset: fset, files: files, info: info}, nil
}
