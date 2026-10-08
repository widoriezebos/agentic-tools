package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// The goldens of group G4a (delivery): design write, work
// brief/build/wait/workspace/review/revise/land/finish, test
// run/declare-moves/wait and settings show/keys/set/check. A refusal case
// for every verb; settings show and set, and work wait --list, also show
// what they print when they act.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, g4aLayoutCases)
	return true
}()

func g4aLayoutCases() []layoutCase {
	return []layoutCase{
		{name: "settings-show", args: []string{"settings", "show"}, bed: workLayoutBed},
		{name: "settings-show-refusal", args: []string{"settings", "show", "no.such.key"}, bed: workLayoutBed},
		{name: "settings-set", args: []string{"settings", "set", "launch.read.model", "fixture-model"}, bed: settingsSetLayoutBed},
		{name: "settings-set-refusal", args: []string{"settings", "set", "launch.read.model"}, bed: workLayoutBed},
		{name: "settings-keys-refusal", args: []string{"settings", "keys"}, bed: outsideLayoutBed},
		{name: "settings-check-refusal", args: []string{"settings", "check"}, bed: outsideLayoutBed},
		{name: "design-write-refusal", args: []string{"design", "write"}, bed: workLayoutBed},
		{name: "work-brief-refusal", args: []string{"work", "brief", "standing-validation"}, bed: workLayoutBed},
		{name: "work-build-refusal", args: []string{"work", "build", "standing-validation"}, bed: workLayoutBed},
		{name: "work-wait-list", args: []string{"work", "wait", "--list"}, bed: workLayoutBed},
		{name: "work-wait-refusal", args: []string{"work", "wait"}, bed: workLayoutBed},
		{name: "work-workspace-refusal", args: []string{"work", "workspace"}, bed: workLayoutBed},
		{name: "work-review-refusal", args: []string{"work", "review"}, bed: workLayoutBed},
		{name: "work-revise-refusal", args: []string{"work", "revise"}, bed: workLayoutBed},
		{name: "work-land-refusal", args: []string{"work", "land"}, bed: workLayoutBed},
		{name: "work-rebase-refusal", args: []string{"work", "rebase"}, bed: workLayoutBed},
		{name: "work-finish-refusal", args: []string{"work", "finish"}, bed: workLayoutBed},
		{name: "test-run-refusal", args: []string{"test", "run"}, bed: outsideLayoutBed},
		{name: "test-declare-moves-refusal", args: []string{"test", "declare-moves"}, bed: workLayoutBed},
		{name: "test-wait-refusal", args: []string{"test", "wait"}, bed: workLayoutBed},
	}
}

// workLayoutBed is the work bed: an installation with an approved goal,
// standing-validation, and its goal worktree.
func workLayoutBed(t *testing.T) layoutBed {
	b := newWorkBed(t)
	conf := filepath.Join(b.root(), "metasystem.conf")
	data, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	// Each lane's model for the bed's fake runtime, so the launch settings
	// read.
	for _, lane := range []string{"build", "design", "read", "critique"} {
		data = append(data, []byte("\nlaunch."+lane+".model.fake=fake-model")...)
	}
	if err := os.WriteFile(conf, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	owners.helm.zone = layoutZone(t)
	owners.helm.machine = func(string) (string, error) { return "m1e", nil }
	root := realpath.Resolve(b.root())
	return layoutBed{owners: owners, cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e",
		realpath.Resolve(b.worktree), "/Users/wido/GitHub/agentic-tools-m1e-work")}
}

// settingsSetLayoutBed is the work bed whose seat already set the value
// the case sets, so the golden's run and its coloured rerun read the same.
func settingsSetLayoutBed(t *testing.T) layoutBed {
	bed := workLayoutBed(t)
	if err := os.MkdirAll(filepath.Join(bed.cwd, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	bed.owners.policies.ConfPath = func(checkout string) (string, error) { return filepath.Join(checkout, "settings.conf"), nil }
	if err := os.WriteFile(filepath.Join(bed.cwd, "settings.conf.local"), []byte("launch.read.model=fixture-model\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return bed
}
