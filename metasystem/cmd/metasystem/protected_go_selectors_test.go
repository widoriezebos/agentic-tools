package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	protectedBase      = "base-tree"
	protectedCandidate = "candidate-tree"
	protectedModule    = "module example.invalid/protected\n\ngo 1.27\n"
)

type protectedSelectorFixture struct {
	t                           *testing.T
	root, module, base, changed string
}

func newProtectedSelectorFixture(t *testing.T, changed string) *protectedSelectorFixture {
	t.Helper()
	root := t.TempDir()
	f := &protectedSelectorFixture{t: t, root: root, module: filepath.Join(root, "metasystem"), base: filepath.Join(root, "base", "metasystem"), changed: changed}
	files := map[string]string{
		"go.mod":                    protectedModule,
		"app/app.go":                "package app\n",
		"app/testdata/data.txt":     "original\n",
		"base/base.go":              "package base\nconst Value = 1\n",
		"consumer/consumer.go":      "package consumer\n",
		"consumer/consumer_test.go": "package consumer\nimport _ \"example.invalid/protected/base\"\n",
	}
	for path, content := range files {
		for _, dir := range []string{f.base, f.module} {
			full := filepath.Join(dir, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, ok := files[changed]; !ok {
		t.Fatalf("undeclared changed path %q", changed)
	}
	if err := os.WriteFile(filepath.Join(f.module, filepath.FromSlash(changed)), []byte(files[changed]+"// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

type protectedRawReply struct {
	args   []string
	output string
}

func (f *protectedSelectorFixture) selectPackages(root, base, candidate string, tags, environment []string) (gopackages.Selection, error) {
	f.t.Helper()
	if root != f.module || base != protectedBase || candidate != protectedCandidate {
		f.t.Fatalf("selector root/revisions = %q %q %q", root, base, candidate)
	}
	const modOID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const oldOID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const newOID = "cccccccccccccccccccccccccccccccccccccccc"
	entry := func(oid, path string) string { return fmt.Sprintf("100644 blob %s\t%s\x00", oid, path) }
	ls := func(tree, path string) []string {
		return []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", path}
	}
	replies := []protectedRawReply{
		{ls(protectedBase, "go.mod"), entry(modOID, "go.mod")},
		{[]string{"cat-file", "blob", modOID}, protectedModule},
		{ls(protectedCandidate, "go.mod"), entry(modOID, "go.mod")},
		{[]string{"cat-file", "blob", modOID}, protectedModule},
		{[]string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", protectedBase, protectedCandidate, "--"}, f.changed + "\x00"},
		{ls(protectedBase, f.changed), entry(oldOID, f.changed)},
		{ls(protectedCandidate, f.changed), entry(newOID, f.changed)},
		{ls(protectedCandidate, "go.mod"), entry(modOID, "go.mod")},
		{[]string{"cat-file", "blob", modOID}, protectedModule},
	}
	pins := []string{"-C", f.module, "-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	next, opened, closed := 0, false, false
	workspace := gittree.Workspace{Dir: f.module, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		f.t.Helper()
		if next == len(replies) {
			f.t.Fatalf("unexpected raw Git call: %v", request.Args)
		}
		want := replies[next]
		next++
		if request.Dir != f.module || !slices.Equal(request.Args, append(slices.Clone(pins), want.args...)) ||
			request.Operation != "git "+strings.Join(want.args, " ") || request.Stdin != nil ||
			!slices.Equal(request.Env, gittree.ScrubbedEnviron()) {
			f.t.Fatalf("raw Git call %d = %+v; want args %v", next, request, want.args)
		}
		return gittree.RawResult{Stdout: []byte(want.output)}
	}}
	selected, err := gopackages.SelectWithWorkspaceSnapshot(workspace, base, candidate, tags, environment, func(tree string) (string, func() error, error) {
		if tree != protectedCandidate || opened {
			f.t.Fatalf("unexpected snapshot %q", tree)
		}
		opened = true
		return f.module, func() error {
			if closed {
				f.t.Fatal("snapshot closed twice")
			}
			closed = true
			return nil
		}, nil
	})
	if next != len(replies) || !opened || !closed {
		f.t.Fatalf("raw/snapshot closure: calls=%d/%d opened=%t closed=%t", next, len(replies), opened, closed)
	}
	return selected, err
}

func protectedGoContract(t *testing.T) (testpolicy.Contract, testpolicy.Group) {
	t.Helper()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range contract.Groups {
		if group.ID == "go-affected" {
			if group.PackageSelection != "changed-and-consumers" || group.CWD != "metasystem" {
				t.Fatalf("Go template changed: %+v", group)
			}
			return contract, group
		}
	}
	t.Fatal("missing go-affected template")
	return testpolicy.Contract{}, testpolicy.Group{}
}

func protectedConcreteGroup(t *testing.T, contract testpolicy.Contract, pkg string) testpolicy.Group {
	t.Helper()
	var matches []testpolicy.Group
	for _, group := range contract.Groups {
		if strings.HasPrefix(group.ID, "go-affected/") && slices.Equal(group.Packages, []string{pkg}) {
			matches = append(matches, group)
		}
	}
	if len(matches) != 1 || matches[0].PackageSelection != "" || !slices.Contains(contract.Always.Standard, matches[0].ID) {
		t.Fatalf("concrete group for %s: %+v", pkg, matches)
	}
	return matches[0]
}

func protectedExpand(t *testing.T, f *protectedSelectorFixture) testpolicy.Contract {
	t.Helper()
	contract, template := protectedGoContract(t)
	environment := []string{"GOARCH=arm64", "GOOS=linux", "GOWORK=off", "PATH=/selector-tools"}
	calls := 0
	expanded, err := proofrun.ExpandGoPackageGroupsWithSelector(contract, f.root, protectedBase, protectedCandidate, environment,
		func(root, base, candidate string, tags, actual []string) (gopackages.Selection, error) {
			calls++
			if calls != 1 || !slices.Equal(tags, template.BuildTags) || !slices.Equal(actual, environment) {
				t.Fatalf("selector tags/environment/calls: %v %v %d", tags, actual, calls)
			}
			return f.selectPackages(root, base, candidate, tags, actual)
		})
	if err != nil || calls != 1 {
		t.Fatalf("expand: calls=%d err=%v", calls, err)
	}
	if err := expanded.Validate(); err != nil {
		t.Fatal(err)
	}
	return expanded
}

func TestGoalLandingGoAssetOnlyChangeExpandsConcretePackage(t *testing.T) {
	t.Parallel()
	f := newProtectedSelectorFixture(t, "app/testdata/data.txt")
	selected, err := f.selectPackages(f.module, protectedBase, protectedCandidate, nil, []string{"GOOS=linux", "GOARCH=arm64"})
	if err != nil || !slices.Equal(selected.Changed, []string{"./app"}) || !slices.Equal(selected.Packages, []string{"./app"}) {
		t.Fatalf("asset selection: %+v err=%v", selected, err)
	}
	group := protectedConcreteGroup(t, protectedExpand(t, f), "./app")
	if !slices.Contains(group.Inputs, "metasystem/app/**") {
		t.Fatalf("asset input missing: %+v", group)
	}
}

func TestGoalLandingGoExpansionCoversPriorChangedAndConsumers(t *testing.T) {
	t.Parallel()
	f := newProtectedSelectorFixture(t, "base/base.go")
	selected, err := f.selectPackages(f.module, protectedBase, protectedCandidate, nil, []string{"GOOS=linux", "GOARCH=arm64"})
	if err != nil || !slices.Equal(selected.Changed, []string{"./base"}) || !slices.Equal(selected.Dependents, []string{"./consumer"}) || !slices.Equal(selected.Packages, []string{"./base", "./consumer"}) || !slices.Contains(selected.InputDirs["./consumer"], "./base") {
		t.Fatalf("base/consumer selection: %+v err=%v", selected, err)
	}
	expanded := protectedExpand(t, f)
	base, consumer := protectedConcreteGroup(t, expanded, "./base"), protectedConcreteGroup(t, expanded, "./consumer")
	if base.ID == consumer.ID || !slices.Contains(base.Inputs, "metasystem/base/**") || !slices.Contains(consumer.Inputs, "metasystem/consumer/**") || !slices.Contains(consumer.Inputs, "metasystem/base/**") {
		t.Fatalf("concrete groups: base=%+v consumer=%+v", base, consumer)
	}
	if again := protectedConcreteGroup(t, protectedExpand(t, f), "./base"); !reflect.DeepEqual(base, again) {
		t.Fatalf("base identity changed: %+v vs %+v", base, again)
	}
	plan := testpolicy.Plan{SelectedGroups: []string{base.ID}}
	declarations, err := testingRelevantInputs("metasystem/", "testing.json", expanded, plan)
	if err != nil || !slices.Contains(declarations, "metasystem/base/**") {
		t.Fatalf("selected inputs: %v err=%v", declarations, err)
	}
	fact := &deliverySnapshotFact{t: t, candidate: protectedCandidate, declarations: declarations, tree: "working-tree", wantCalls: 1}
	err = checkDeliveryInputParityWith(protectedCandidate, "metasystem/", "testing.json", expanded, plan, fact.snapshot)
	fact.assertConsumed()
	if err == nil || !strings.Contains(err.Error(), "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("mismatched selected inputs accepted: %v", err)
	}
}

func TestProductionJoinGateSelectsPackagesAgainstTheLandingRoot(t *testing.T) {
	t.Parallel()
	f := newProtectedSelectorFixture(t, "base/base.go")
	command := batchTreePlanCommand("metasystem", f.root, "goal-a", protectedCandidate, testpolicy.ModeAuto)
	if command.Dir != f.module || !slices.Equal(command.Args, []string{"metasystem", "test", "plan", "--root", f.module, "--goal", "goal-a", "--tree", protectedCandidate, "--mode", "auto", "--purpose", "delivery", "--json"}) {
		t.Fatalf("join plan root/CWD: dir=%q args=%v", command.Dir, command.Args)
	}
	if !slices.Equal(command.Env, gittree.ScrubbedEnviron()) {
		t.Fatalf("join plan environment differs from scrubbed environment")
	}
	selected, err := f.selectPackages(command.Dir, protectedBase, protectedCandidate, nil, command.Env)
	if err != nil || selected.Tree != protectedCandidate || selected.ModulePath != "example.invalid/protected" || !slices.Equal(selected.Packages, []string{"./base", "./consumer"}) || !slices.Contains(selected.InputDirs["./consumer"], "./base") {
		t.Fatalf("join selection: %+v err=%v", selected, err)
	}
}
