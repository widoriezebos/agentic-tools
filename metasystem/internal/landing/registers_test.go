package landing

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestWorkspaceProjection(t *testing.T) {
	for _, fixture := range []struct {
		name string
		new  func(*testing.T) *observeFixture
	}{
		{name: "nested installation", new: newObserveFixture},
		{name: "toplevel installation", new: newAdoptedObserveFixture},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			f := fixture.new(t)
			installation := gittree.Workspace{Dir: f.root}
			top, err := installation.TopLevel()
			if err != nil {
				t.Fatal(err)
			}
			project := gittree.Workspace{Dir: top}
			before, err := project.StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			beforeInstallation, err := installation.TreeOf(before)
			if err != nil {
				t.Fatal(err)
			}
			f.write("plans/goals/x.md", "goal revision\n")
			f.write("plans/goals.md", "legacy goal revision\n")
			f.git("add", "plans/goals/x.md", "plans/goals.md")
			after, err := project.StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			afterInstallation, err := installation.TreeOf(after)
			if err != nil {
				t.Fatal(err)
			}
			beforeWorkspace, err := ProjectWorkspaceTree(f.root, before)
			if err != nil {
				t.Fatal(err)
			}
			afterWorkspace, err := ProjectWorkspaceTree(f.root, after)
			if err != nil {
				t.Fatal(err)
			}
			if beforeWorkspace != afterWorkspace {
				t.Fatal("project workspace projection retained excluded goal paths")
			}
			beforeInstallationWorkspace, err := InstallationWorkspaceTree(f.root, beforeInstallation)
			if err != nil {
				t.Fatal(err)
			}
			afterInstallationWorkspace, err := InstallationWorkspaceTree(f.root, afterInstallation)
			if err != nil {
				t.Fatal(err)
			}
			if beforeInstallationWorkspace != afterInstallationWorkspace {
				t.Fatal("installation workspace projection retained excluded goal paths")
			}
		})
	}
}

func TestWorkspaceExclusionsAreStableCopies(t *testing.T) {
	t.Parallel()
	want := []string{
		"memory/receipts.log",
		"plans/goals",
		"plans/goals-accepted.json",
		"plans/goals.md",
		"records/counselor",
		"records/goals",
		"records/narrator-digest.log",
	}
	got := WorkspaceExclusions()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WorkspaceExclusions() = %v, want %v", got, want)
	}
	got[0] = "mutated"
	if reflect.DeepEqual(WorkspaceExclusions(), got) {
		t.Fatal("WorkspaceExclusions returned shared mutable state")
	}
}

func TestAppendOnlyRegistersPins(t *testing.T) {
	receiptsRoot, err := stateroot.RelativeRoot(stateroot.Receipts)
	if err != nil {
		t.Fatal(err)
	}
	recordsRoot, err := stateroot.RelativeRoot(stateroot.Records)
	if err != nil {
		t.Fatal(err)
	}
	// Pin 1: the declared registers are exactly the design's two, each under
	// its state root; the counselor registers stay outside (they keep the
	// held-goal rule at the carriage gate and are excluded from the delivery
	// workspace through ledgerPaths, see TestWorkspaceProjection).
	want := []string{
		filepath.ToSlash(filepath.Join(receiptsRoot, "receipts.log")),
		filepath.ToSlash(filepath.Join(recordsRoot, "narrator-digest.log")),
	}
	if !reflect.DeepEqual(appendOnlyRegisters, want) {
		t.Fatalf("append-only registers = %v, want %v", appendOnlyRegisters, want)
	}

	// Pin 2: every declared register is outside the LANDING projection, so
	// commit.sh's comparison and the receipt's filter agree on it.
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, register := range appendOnlyRegisters {
		for _, test := range []struct {
			path   string
			prefix string
		}{
			{path: "metasystem/" + register, prefix: "metasystem/"},
			{path: register},
		} {
			included, err := policy.Includes(behaviorsurface.Landing, test.path, test.prefix)
			if err != nil {
				t.Fatal(err)
			}
			if included {
				t.Fatalf("append-only register %s is included in the landing projection for prefix %q", register, test.prefix)
			}
		}
	}
}
