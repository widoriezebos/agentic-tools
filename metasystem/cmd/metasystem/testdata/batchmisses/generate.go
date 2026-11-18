//go:build ignore

// Run explicitly; normal tests read only the recorded fixtures.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type record struct {
	Attempt         string              `json:"attempt"`
	Base            string              `json:"base"`
	Head            string              `json:"head"`
	Source          string              `json:"source"`
	Package         string              `json:"package"`
	Test            string              `json:"test"`
	Scope           string              `json:"scope"`
	Reason          string              `json:"reason"`
	CurrentMapping  string              `json:"current_mapping,omitempty"`
	Paths           []string            `json:"paths"`
	Status          string              `json:"status_no_renames"`
	ContractSHA     string              `json:"contract_sha256"`
	FullCommand     string              `json:"full_command"`
	Impact          *goadapter.Impact   `json:"impact,omitempty"`
	Inventory       map[string][]string `json:"inventory,omitempty"`
	TaggedInventory map[string][]string `json:"tagged_inventory,omitempty"`
	Roster          []testpolicy.Group  `json:"roster_overlay,omitempty"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func git(root string, args ...string) []byte {
	cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-C", root}, args...)...)
	data, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("git %v: %v", args, err))
	}
	return data
}
func write(path string, data []byte) { must(os.WriteFile(path, data, 0600)) }
func jsonWrite(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	must(err)
	write(path, append(data, '\n'))
}
func main() {
	repo := flag.String("repo", "", "repository with original objects")
	out := flag.String("out", "cmd/metasystem/testdata/batchmisses", "fixture directory")
	scratch := flag.String("scratch", "", "new disposable directory inside the authorized checkout")
	flag.Parse()
	if *repo == "" || *scratch == "" {
		panic("-repo and -scratch are required")
	}
	*repo, _ = filepath.Abs(*repo)
	*out, _ = filepath.Abs(*out)
	*scratch, _ = filepath.Abs(*scratch)
	rel, err := filepath.Rel(*repo, *scratch)
	must(err)
	if rel == "." || strings.HasPrefix(rel, "..") {
		panic("scratch must be inside repo")
	}
	if _, err := os.Stat(*scratch); !os.IsNotExist(err) {
		panic("scratch must not exist")
	}
	data, err := os.ReadFile(filepath.Join(*out, "manifest.json"))
	must(err)
	var rows []record
	must(json.Unmarshal(data, &rows))
	// A standalone clone mutates neither the source checkout nor its index.
	git(*repo, "clone", "--shared", "--no-checkout", "--quiet", *repo, *scratch)
	for i := range rows {
		r := &rows[i]
		for _, sha := range []string{r.Base, r.Head} {
			resolved := strings.TrimSpace(string(git(*repo, "rev-parse", "--verify", sha+"^{commit}")))
			if resolved != sha {
				panic("manifest must pin full commit identities")
			}
		}
		paths := strings.Split(strings.TrimRight(string(git(*repo, "diff", "--name-only", "--no-renames", "-z", r.Base, r.Head, "--")), "\x00"), "\x00")
		if len(paths) == 0 || paths[0] == "" {
			panic("empty historical diff")
		}
		r.Paths = paths
		r.Status = string(git(*repo, "diff", "--name-status", "--no-renames", r.Base, r.Head, "--"))
		contract := git(*repo, "show", r.Head+":metasystem/testing.json")
		r.ContractSHA = fmt.Sprintf("%x", sha256.Sum256(contract))
		write(filepath.Join(*out, r.Attempt+"-contract.json"), contract)
		conf := git(*repo, "show", r.Head+":metasystem/metasystem.conf")
		// Read only the committed command needed for the full route.
		confPath := filepath.Join(*out, "candidate.conf.tmp")
		write(confPath, conf)
		r.FullCommand, _, err = config.CommittedLookup(confPath, "proof.full")
		must(err)
		must(os.Remove(confPath))
		if r.FullCommand == "" {
			panic("candidate has no committed proof.full command")
		}
		// Resolve the original failing identity in its original package, even on
		// full routes that never call the impact adapter.
		matches := strings.Fields(string(git(*repo, "grep", "-l", "-F", "func "+r.Test+"(", r.Head, "--", "metasystem/"+r.Package+"/*_test.go")))
		found := false
		for _, match := range matches {
			_, path, ok := strings.Cut(match, ":")
			if !ok || filepath.ToSlash(filepath.Dir(path)) != "metasystem/"+r.Package {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, git(*repo, "show", match), parser.SkipObjectResolution)
			must(err)
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && fn.Name.Name == r.Test {
					found = true
				}
			}
		}
		if !found {
			panic("unresolvable original failure: " + r.Attempt)
		}
		if r.Scope == "impact" {
			// Refuse a tracked private configuration before checkout, rather than
			// reading or copying credentials into the recording workspace.
			private := strings.TrimSpace(string(git(*repo, "ls-tree", "-r", "--name-only", r.Head, "--", "metasystem/metasystem.conf.local")))
			if private != "" {
				panic("candidate tracks private configuration")
			}
			git(*scratch, "checkout", "--quiet", "--detach", r.Head)
			module := filepath.Join(*scratch, "metasystem")
			impact, err := goadapter.UnitImpact(module, r.Base, false)
			must(err)
			if len(impact.Paths) == 0 || len(impact.Packages) == 0 {
				panic("unexpected empty recorded impact")
			}
			r.Impact = &impact
			r.Inventory = inventory(module, nil)
			r.TaggedInventory = inventory(module, []string{"batchtest"})
			if !slices.Contains(r.Inventory[r.Package], r.Test) {
				panic("failure absent from candidate inventory")
			}
			r.Roster = roster(module, r.Inventory)
		}
		jsonWrite(filepath.Join(*out, r.Attempt+".json"), r)
		fmt.Println(r.Attempt, r.Scope, len(r.Paths), "paths")
	}
}

func inventory(root string, tags []string) map[string][]string {
	result := map[string][]string{}
	context := build.Default
	context.BuildTags = tags
	must(filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && (strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || slices.Contains([]string{"vendor", "node_modules", "testdata", "artifacts", "bin"}, entry.Name())) {
			return filepath.SkipDir
		}
		if path != root {
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		pkg, err := context.ImportDir(path, 0)
		if _, ok := err.(*build.NoGoError); ok {
			return nil
		}
		if err != nil {
			return err
		}
		var files []*ast.File
		var names []string
		for _, name := range append(slices.Clone(pkg.TestGoFiles), pkg.XTestGoFiles...) {
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(path, name), nil, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			files = append(files, f)
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && testpolicy.GoTestName(fn.Name.Name) {
					names = append(names, fn.Name.Name)
				}
			}
		}
		for _, ex := range doc.Examples(files...) {
			if ex.Output != "" || ex.EmptyOutput {
				names = append(names, "Example"+ex.Name)
			}
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = slices.Compact(slices.Sorted(slices.Values(names)))
		return nil
	}))
	return result
}

func roster(root string, inventory map[string][]string) []testpolicy.Group {
	ids := testpolicy.BatchRosterIDs()
	groups := make([]testpolicy.Group, len(ids))
	for i, id := range ids {
		groups[i] = testpolicy.Group{ID: id, Adapter: "go", Kind: "unit", CWD: "metasystem", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"metasystem/cmd/**", "metasystem/internal/**"}, Platforms: []string{"any"}, TargetMS: 120000}
	}
	for i := 0; i < 2; i++ {
		groups[i].Packages = []string{"cmd/metasystem"}
		var names []string
		for _, name := range inventory["cmd/metasystem"] {
			if i == 0 && strings.HasPrefix(name, "TestAudit") || i == 1 && (strings.HasPrefix(name, "TestVerbRatchet") || strings.Contains(name, "Bed") && strings.Contains(name, "Coverage")) {
				names = append(names, name)
			}
		}
		groups[i].Tests, _ = json.Marshal(names)
	}
	groups[2].Packages = []string{"internal/testpolicy"}
	groups[2].Tests = json.RawMessage(`"all"`)
	var names []string
	for _, pkg := range []string{"internal/goal", "internal/testutil"} {
		groups[3].Packages = append(groups[3].Packages, pkg)
		sourceFiles, err := filepath.Glob(filepath.Join(root, pkg, "*_source_test.go"))
		must(err)
		for _, name := range inventory[pkg] {
			required := strings.HasPrefix(name, "TestAudit")
			for _, file := range sourceFiles {
				data, err := os.ReadFile(file)
				must(err)
				required = required || strings.Contains(string(data), "func "+name+"(")
			}
			if required {
				names = append(names, name)
			}
		}
	}
	groups[3].Tests, _ = json.Marshal(slices.Compact(slices.Sorted(slices.Values(names))))
	return groups
}
