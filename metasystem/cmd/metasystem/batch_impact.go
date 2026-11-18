package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

type impactConsumer string

const (
	impactUnit  impactConsumer = "unit"
	impactBatch impactConsumer = "batch"
)

// batchImpactPlan is a read-only proposal. Only proof admission can turn a
// proposal into an execution; a printed plan conveys no acceptance authority.
type batchImpactPlan struct {
	Purpose          string              `json:"purpose"`
	Base             string              `json:"base"`
	Scope            string              `json:"scope"`
	Command          string              `json:"command"`
	Policy           string              `json:"policy"`
	Paths            []string            `json:"paths"`
	Instruments      []string            `json:"instruments,omitempty"`
	Groups           []testpolicy.Group  `json:"groups,omitempty"`
	PackageTests     map[string][]string `json:"package_tests,omitempty"`
	TestCount        int                 `json:"test_count,omitempty"`
	GroupInvocations int                 `json:"group_invocations,omitempty"`
	// GroupTests retains each group's build and execution obligations through Groups.
	// PackageTests is the distinct identity union, not an execution request.
	GroupTests map[string]map[string][]string `json:"group_tests,omitempty"`
}

// Instrument paths are module-relative except for the configured contract,
// which the caller resolves relative to the repository. Disabling rename
// detection in the diff presents both names as ordinary changed paths.
func batchInstruments(paths []string, prefix, contract string) []string {
	exact := []string{"go.mod", "go.sum", "metasystem.conf", "metasystem.conf.local",
		"cmd/metasystem/test_impact.go", "cmd/metasystem/batch_impact.go", "cmd/metasystem/test.go",
		"cmd/metasystem/publication_requirements.go", "cmd/metasystem/intent_landing_prove.go"}
	directories := []string{"proof/", "cmd/devgate/", "internal/repoproof/", "internal/proofrun/",
		"internal/testpolicy/", "internal/testenv/", "internal/testutil/", "internal/testrun/",
		"internal/landing/batch/goadapter/", "internal/gopackages/", "internal/gittree/",
		"internal/pathpattern/", "internal/landing/plain/"}
	var instruments []string
	for _, path := range paths {
		relative, inModule := strings.CutPrefix(path, prefix)
		if path == contract || inModule && (slices.Contains(exact, relative) || slices.ContainsFunc(directories, func(dir string) bool { return strings.HasPrefix(relative, dir) })) {
			instruments = append(instruments, path)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(instruments)))
}

func (sources impactSources) batchPlan(root, base string) (batchImpactPlan, error) {
	plan := batchImpactPlan{Purpose: "batch-impact", Scope: "impact", Policy: "batch-impact-v1"}
	if base == "" {
		return plan, fmt.Errorf("the batch plan needs its comparison base")
	}
	sha, err := sources.git(root, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return plan, err
	}
	plan.Base = sha
	if _, err = sources.git(root, "merge-base", "--is-ancestor", sha, "HEAD"); err != nil {
		return plan, fmt.Errorf("batch base is not an ancestor of HEAD: %w", err)
	}
	// Git's full-tree diff includes changes outside the installation, staged
	// changes and deletions. The adapter separately reads the module's Go diff.
	diff, err := sources.git(root, "diff", "--name-only", "--no-renames", "--no-relative", "-z", sha, "--", ":/")
	if err != nil {
		return plan, err
	}
	untracked, err := sources.git(root, "ls-files", "--others", "--exclude-standard", "--full-name", "-z", "--", ":/")
	if err != nil {
		return plan, err
	}
	plan.Paths = slices.Compact(slices.Sorted(slices.Values(slices.DeleteFunc(strings.Split(diff+"\x00"+untracked, "\x00"), func(s string) bool { return s == "" }))))
	prefix := ""
	if config.TemplateMode(root) {
		prefix = "metasystem/"
	}
	// Check fixed instruments before loading a possibly deleted contract.
	plan.Instruments = batchInstruments(plan.Paths, prefix, "")
	if len(plan.Instruments) == 0 {
		contractRel, found, err := config.CommittedLookup(filepath.Join(root, "metasystem.conf"), "testing.contract")
		if err != nil {
			return plan, err
		}
		if !found {
			return plan, fmt.Errorf("testing.contract is required in committed metasystem.conf")
		}
		plan.Instruments = batchInstruments(plan.Paths, prefix, prefix+contractRel)
	}
	key := "proof.cheap"
	if len(plan.Instruments) > 0 {
		plan.Scope, key = "full", "proof.full"
	}
	plan.Command, _, err = sources.get(config.GetParams{ConfPath: filepath.Join(root, "metasystem.conf"), Key: key})
	if err != nil {
		return plan, err
	}
	if strings.TrimSpace(plan.Command) == "" {
		return plan, fmt.Errorf("%s is empty", key)
	}
	if plan.Scope == "full" {
		return plan, nil
	}
	_, contract, _, err := testrun.LoadContract(root)
	if err != nil {
		return plan, err
	}
	for _, id := range testpolicy.BatchRosterIDs() {
		if !slices.ContainsFunc(contract.Groups, func(g testpolicy.Group) bool { return g.ID == id && g.Adapter == "go" && g.Phase == "acceptance" }) {
			return plan, fmt.Errorf("batch roster group %s must be an acceptance-phase Go group", id)
		}
	}
	data, err := json.Marshal(contract)
	if err != nil {
		return plan, err
	}
	plan.Policy = "batch-impact-v1:" + bytesSHA256(data)
	var diagnostic bytes.Buffer
	// No check, cache, replay, environment report or proof state is written.
	if code := sources.runConsumer([]string{"--root", root, "--base", sha, "--plan"}, &diagnostic, &diagnostic, "", impactBatch, &plan); code != 0 {
		return plan, fmt.Errorf("batch planning failed: %s", strings.TrimSpace(diagnostic.String()))
	}
	return plan, nil
}

func (plan *batchImpactPlan) expand(installation string, contract testpolicy.Contract, ids, environment []string) error {
	closure, err := testpolicy.WithPrerequisiteClosure(contract, testpolicy.Plan{SelectedGroups: ids, Stages: []testpolicy.Stage{{ID: "batch-impact", Groups: ids}}})
	if err != nil {
		return err
	}
	root := installation
	if config.TemplateMode(installation) {
		root = filepath.Dir(installation)
	}
	plan.PackageTests = map[string][]string{}
	plan.GroupTests = map[string]map[string][]string{}
	plan.TestCount, plan.GroupInvocations = 0, 0
	plan.Groups = nil
	for _, id := range closure.SelectedGroups {
		i := slices.IndexFunc(contract.Groups, func(g testpolicy.Group) bool { return g.ID == id })
		group := contract.Groups[i]
		plan.Groups = append(plan.Groups, group)
		if group.Adapter != "go" {
			continue
		}
		cwd := filepath.Join(root, group.CWD)
		context := goadapter.EffectiveBuildContext(cwd, proofrun.GroupTestingEnvironment(environment, group))
		if context == nil {
			return fmt.Errorf("batch group %s: Go build context is unavailable", id)
		}
		plainContext := *context
		context.BuildTags = append(slices.Clone(context.BuildTags), group.BuildTags...)
		plan.GroupTests[id] = map[string][]string{}
		var requested []string
		all := string(group.Tests) == `"all"`
		if !all {
			if err := json.Unmarshal(group.Tests, &requested); err != nil {
				return err
			}
		}
		found := map[string]bool{}
		for _, pkg := range group.Packages {
			directory := filepath.Join(cwd, pkg)
			available, err := batchPackageTests(context, directory)
			if err != nil {
				return fmt.Errorf("batch group %s package %s: %w", id, pkg, err)
			}
			var plain []string
			if len(group.BuildTags) > 0 {
				plain, err = batchPackageTests(&plainContext, directory)
				if _, onlyTagged := err.(*build.NoGoError); onlyTagged {
					err = nil
				}
				if err != nil {
					return fmt.Errorf("batch group %s untagged package %s: %w", id, pkg, err)
				}
			}
			key, err := filepath.Rel(installation, directory)
			if err != nil {
				return err
			}
			key = filepath.ToSlash(key)
			var selected []string
			for _, name := range available {
				if !all && !slices.Contains(requested, name) {
					continue
				}
				found[name] = true
				// Match the full proof's extra pass: only identities absent
				// from the untagged inventory need the tagged build.
				if len(group.BuildTags) > 0 && slices.Contains(plain, name) {
					continue
				}
				selected = append(selected, name)
			}
			plan.GroupTests[id][key] = selected
			plan.GroupInvocations += len(selected)
			if len(selected) > 0 {
				plan.PackageTests[key] = slices.Compact(slices.Sorted(slices.Values(append(plan.PackageTests[key], selected...))))
			}
		}
		for _, name := range requested {
			if !found[name] {
				return fmt.Errorf("batch group %s names missing test %s", id, name)
			}
		}
	}
	for _, names := range plan.PackageTests {
		plan.TestCount += len(names)
	}
	return nil
}

func (plan batchImpactPlan) text() string {
	var out strings.Builder
	fmt.Fprintf(&out, "batch plan: %s against %s; command %s\n", plan.Scope, plan.Base, plan.Command)
	if plan.Scope != "full" {
		fmt.Fprintf(&out, "tests: %d; group invocations: %d\n", plan.TestCount, plan.GroupInvocations)
	}
	for _, path := range plan.Instruments {
		fmt.Fprintln(&out, "instrument: "+path)
	}
	for _, group := range plan.Groups {
		fmt.Fprintln(&out, "group: "+group.ID)
	}
	for _, pkg := range slices.Sorted(maps.Keys(plan.PackageTests)) {
		fmt.Fprintln(&out, "selection: "+pkg+"="+strings.Join(plan.PackageTests[pkg], ","))
	}
	return out.String()
}

// batchPackageTests discovers native top-level tests and executable examples in
// the supplied build context, without compiling or running the package.
func batchPackageTests(context *build.Context, directory string) ([]string, error) {
	listed, err := context.ImportDir(directory, 0)
	if err != nil {
		return nil, err
	}
	var names []string
	var files []*ast.File
	for _, file := range append(slices.Clone(listed.TestGoFiles), listed.XTestGoFiles...) {
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, file), nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, parsed)
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && testpolicy.GoTestName(fn.Name.Name) {
				names = append(names, fn.Name.Name)
			}
		}
	}
	for _, example := range doc.Examples(files...) {
		if example.Output != "" || example.EmptyOutput {
			names = append(names, "Example"+example.Name)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(names))), nil
}
