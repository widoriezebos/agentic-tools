package branch_test

import (
	"bytes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttestationGitAdapterPublishesAndClonesEvidence(t *testing.T) {
	t.Parallel()
	t.Run("nested publication and portable binding", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		nested := filepath.Join(f.root, "metasystem")
		writeReadJob(t, nested, "critic-adapter", unit, "completed", false)
		read, att, err := branch.CommitRead(branch.CommitReadRequest{Repo: nested, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", Unit: "u1", OpID: "adapter-read", RootJob: "critic-adapter", CheckClaim: claimAllowed,
			GateRunID: "adapter-fast", GateTree: unitTree(t, f, unit)})
		if err != nil || read == "" || att.Source.ClosureSHA256 == "" {
			t.Fatalf("nested read=%q source=%+v err=%v", read, att.Source, err)
		}
		if got := git(t, nested, "rev-parse", "refs/heads/goal/goal-a"); got != read {
			t.Fatalf("installed tip %s want %s", got, read)
		}
		if got, want := git(t, nested, "write-tree"), git(t, nested, "rev-parse", read+"^{tree}"); got != want {
			t.Fatalf("index tree %s want commit tree %s", got, want)
		}
		attPath := "metasystem/records/reads/goal-a/" + unit + ".json"
		for _, path := range []string{attPath, "metasystem/records/reads/goal-a/" + unit + ".closure.json"} {
			data, readErr := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
			if readErr != nil || strings.TrimSpace(string(data)) != git(t, nested, "show", read+":"+path) {
				t.Fatalf("published %s differs from commit: %v", path, readErr)
			}
		}
		git(t, f.root, "push", "-q", "origin", read+":refs/heads/goal/goal-a")
		clone := filepath.Join(t.TempDir(), "fresh")
		git(t, filepath.Dir(clone), "clone", "-q", "--branch", "goal/goal-a", f.origin, clone)
		if _, err := os.Stat(filepath.Join(clone, "artifacts", "agents")); !os.IsNotExist(err) {
			t.Fatalf("fresh clone has job store: %v", err)
		}
		nestedClone := filepath.Join(clone, "metasystem")
		after := git(t, nestedClone, "rev-parse", unit+":metasystem")
		bound, err := branch.BindLandedUnit(nestedClone, read, f.base, "goal-a", unit, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", after)
		if err != nil || bound.CriticRoot != "critic-adapter" || bound.Digest != att.Subject.UnitDigest || bound.GoalRevision != 1 {
			t.Fatalf("clone binding=%+v err=%v", bound, err)
		}
	})
	t.Run("ref lock restores staged index and reader bytes", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		record := readerRecord(t, f, unit)
		original, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(record)))
		if err != nil {
			t.Fatal(err)
		}
		git(t, f.root, "add", "--", record)
		indexBefore := git(t, f.root, "write-tree")
		stagedBefore := git(t, f.root, "diff", "--cached", "--name-only")
		if stagedBefore != record {
			t.Fatalf("setup staged paths %q", stagedBefore)
		}
		lock := filepath.Join(f.root, ".git", "refs", "heads", "goal", "goal-a.lock")
		if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lock, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err = branch.CommitRead(branch.CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", Unit: "u1", OpID: "adapter-refusal", ReaderRecord: record, CheckClaim: claimAllowed,
			GateRunID: "adapter-fast", GateTree: unitTree(t, f, unit)})
		if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
			t.Fatalf("actual ref-lock refusal: %v", err)
		}
		if got := git(t, f.root, "write-tree"); got != indexBefore {
			t.Fatalf("restored index %s want %s", got, indexBefore)
		}
		if got := git(t, f.root, "diff", "--cached", "--name-only"); got != stagedBefore {
			t.Fatalf("restored staged paths %q want %q", got, stagedBefore)
		}
		for _, path := range []string{"metasystem/records/reads/goal-a/" + unit + ".json", "metasystem/records/reads/goal-a/" + unit + ".closure.json"} {
			if _, statErr := os.Stat(filepath.Join(f.root, filepath.FromSlash(path))); !os.IsNotExist(statErr) {
				t.Fatalf("generated file %s remains: %v", path, statErr)
			}
		}
		actual, readErr := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(record)))
		if readErr != nil || !bytes.Equal(actual, original) {
			t.Fatalf("reader record changed: %v", readErr)
		}
		if got := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a"); got != unit {
			t.Fatalf("goal tip moved to %s", got)
		}
	})
}
