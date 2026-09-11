package landing

import (
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestWorkspaceProjection(t *testing.T) {
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
