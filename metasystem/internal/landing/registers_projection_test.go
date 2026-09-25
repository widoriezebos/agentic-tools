package landing

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type projectionStep struct {
	dir, index string
	args       []string
	stdin, out []byte
}

// These are repository answers at the projection boundary. The workspace
// still selects the paths and runs its real tree filtering code.
type projectionFacts struct {
	t       *testing.T
	steps   []projectionStep
	next    int
	indices map[string]string
}

func (f *projectionFacts) add(dir, index, output string, stdin []byte, args ...string) {
	f.steps = append(f.steps, projectionStep{dir: dir, index: index, args: args, stdin: stdin, out: []byte(output)})
}

func (f *projectionFacts) raw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	if f.next == len(f.steps) {
		f.t.Fatalf("undeclared projection Git operation: %+v", request)
	}
	want := f.steps[f.next]
	f.next++
	pins := []string{
		"-c", "core.fileMode=true", "-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no",
		"-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false",
	}
	argv := append(append([]string{"-C", want.dir}, pins...), want.args...)
	if request.Dir != want.dir || !slices.Equal(request.Args, argv) ||
		request.Operation != "git "+strings.Join(want.args, " ") ||
		!bytes.Equal(request.Stdin, want.stdin) || (request.Stdin == nil) != (want.stdin == nil) {
		f.t.Fatalf("projection Git call %d = %+v, want dir=%q args=%q stdin=%q", f.next, request, want.dir, argv, want.stdin)
	}
	index := ""
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			index = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	expectedEnv := gittree.ScrubbedEnviron()
	if want.index != "" {
		if !filepath.IsAbs(index) || filepath.Base(index) != "index" {
			f.t.Fatalf("projection index path = %q", index)
		}
		if prior := f.indices[want.index]; prior != "" && prior != index {
			f.t.Fatalf("projection index for %s moved from %q to %q", want.index, prior, index)
		}
		f.indices[want.index] = index
		expectedEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + index)
	}
	if !reflect.DeepEqual(request.Env, expectedEnv) {
		f.t.Fatalf("projection environment differs for %q", want.args)
	}
	return gittree.RawResult{Stdout: want.out}
}

func (f *projectionFacts) done() {
	f.t.Helper()
	if f.next != len(f.steps) {
		f.t.Fatalf("projection consumed %d of %d declared Git facts", f.next, len(f.steps))
	}
}

func (f *projectionFacts) filter(root, top, prefix, tree, result, label string, after bool) {
	f.add(root, "", top+"\n", nil, "rev-parse", "--show-toplevel")
	if label == "project" {
		f.add(root, "", prefix+"\n", nil, "rev-parse", "--show-prefix")
	}
	paths := []string{
		"memory/receipts.log", "plans/goals", "plans/goals-accepted.json",
		"plans/goals.md", "records/counselor", "records/goals",
		"records/narrator-digest.log",
	}
	if label == "project" {
		for i := range paths {
			paths[i] = prefix + paths[i]
		}
	}
	index := label + tree
	f.add(top, index, "", nil, "read-tree", tree)
	f.add(top, "", top+"\n", nil, "rev-parse", "--show-toplevel")
	lsFiles := append([]string{"ls-files", "-z", "--"}, paths...)
	matched := []string{prefix + "memory/receipts.log", prefix + "records/narrator-digest.log"}
	if label != "project" {
		matched = []string{"memory/receipts.log", "records/narrator-digest.log"}
	}
	if after {
		goalPrefix := prefix
		if label != "project" {
			goalPrefix = ""
		}
		matched = append(matched, goalPrefix+"plans/goals.md", goalPrefix+"plans/goals/x.md")
	}
	slices.Sort(matched)
	removed := []byte(strings.Join(matched, "\x00") + "\x00")
	f.add(top, index, string(removed), nil, lsFiles...)
	f.add(top, "", top+"\n", nil, "rev-parse", "--show-toplevel")
	f.add(top, index, "", removed, "update-index", "-z", "--force-remove", "--stdin")
	f.add(top, index, result+"\n", nil, "write-tree")
}

func TestWorkspaceProjection(t *testing.T) {
	t.Parallel()
	for _, specimen := range []struct {
		name, prefix string
		new          func(*testing.T) *repositoryObservationFixture
	}{
		{"nested installation", "metasystem/", newRepositoryObservationFixture},
		{"toplevel installation", "", newAdoptedRepositoryObservationFixture},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			t.Parallel()
			bed := specimen.new(t)
			top, root, prefix := bed.repository, bed.root, specimen.prefix
			before, after := strings.Repeat("a", 40), strings.Repeat("b", 40)
			beforeInstallation, afterInstallation := before, after
			if prefix != "" {
				beforeInstallation, afterInstallation = strings.Repeat("c", 40), strings.Repeat("d", 40)
			}
			projectedProject, projectedInstallation := strings.Repeat("e", 40), strings.Repeat("f", 40)
			if prefix == "" {
				projectedInstallation = projectedProject
			}
			facts := &projectionFacts{t: t, indices: map[string]string{}}
			for i, tree := range []string{before, after} {
				subtree := []string{beforeInstallation, afterInstallation}[i]
				facts.add(root, "", tree+"\n", nil, "rev-parse", tree+"^{tree}")
				facts.add(root, "", prefix+"\n", nil, "rev-parse", "--show-prefix")
				if prefix != "" {
					facts.add(root, "", subtree+"\n", nil, "rev-parse", tree+":metasystem")
				}
			}
			for i, tree := range []string{before, after} {
				facts.filter(root, top, prefix, tree, projectedProject, "project", i == 1)
			}
			for i, tree := range []string{beforeInstallation, afterInstallation} {
				facts.filter(root, top, "", tree, projectedInstallation, "installation", i == 1)
			}
			installation := gittree.Workspace{Dir: root, RawSource: facts.raw}
			gotBeforeInstallation, err := installation.TreeOf(before)
			if err != nil || gotBeforeInstallation != beforeInstallation {
				t.Fatalf("before installation tree = %q, %v", gotBeforeInstallation, err)
			}
			bed.write("plans/goals/x.md", "goal revision\n")
			bed.write("plans/goals.md", "legacy goal revision\n")
			for path, want := range map[string]string{
				"plans/goals/x.md": "goal revision\n",
				"plans/goals.md":   "legacy goal revision\n",
			} {
				got, err := os.ReadFile(filepath.Join(root, path))
				if err != nil || string(got) != want {
					t.Fatalf("goal input %s = %q, %v", path, got, err)
				}
			}
			gotAfterInstallation, err := installation.TreeOf(after)
			if err != nil || gotAfterInstallation != afterInstallation {
				t.Fatalf("after installation tree = %q, %v", gotAfterInstallation, err)
			}
			access := gitProjectionAccess{RawSource: facts.raw}
			beforeProject, err := ProjectWorkspaceTreeWith(root, before, access)
			if err != nil {
				t.Fatal(err)
			}
			afterProject, err := ProjectWorkspaceTreeWith(root, after, access)
			if err != nil || beforeProject != projectedProject || afterProject != beforeProject {
				t.Fatalf("project projection before=%q after=%q, error %v", beforeProject, afterProject, err)
			}
			beforeWorkspace, err := installationWorkspaceTreeWithWorkspace(root, beforeInstallation, installation)
			if err != nil {
				t.Fatal(err)
			}
			afterWorkspace, err := installationWorkspaceTreeWithWorkspace(root, afterInstallation, installation)
			if err != nil || beforeWorkspace != projectedInstallation || afterWorkspace != beforeWorkspace {
				t.Fatalf("installation projection before=%q after=%q, error %v", beforeWorkspace, afterWorkspace, err)
			}
			facts.done()
		})
	}
}
