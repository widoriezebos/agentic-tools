package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func carryRecoveryBed(t *testing.T) (*workBed, intentOwners, []string, string) {
	t.Helper()
	b, owners, _ := rebaseIntentBed(t)
	marker := filepath.Join(t.TempDir(), "fail")
	script := filepath.Join(t.TempDir(), "check")
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf checked\nif test -f '"+marker+"'; then exit 9; fi\n"), 0700); err != nil {
		t.Fatal(err)
	}
	subject := branch.AttestationSubject{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)}
	owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
	original := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		switch {
		case len(args) == 4 && args[0] == "show" && args[1] == "-s":
			return []byte("Goal-Unit: " + b.id + "/u1"), nil
		case slices.Equal(args, []string{"rev-parse", "HEAD"}):
			return []byte(subject.Commit), nil
		case slices.Equal(args, []string{"rev-parse", "origin/main"}) || args[0] == "merge-base":
			return []byte("seed"), nil
		case args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
			return []byte(subject.Tree), nil
		case args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf"):
			cheap := script
			if strings.HasPrefix(args[1], "seed:") {
				cheap = "printf seed"
			}
			return []byte("proof.cheap=" + cheap + "\nproof.audits=true\nproof.deadline=15\nproof.full=printf full\n"), nil
		case args[0] == "status":
			return nil, nil
		}
		return original(root, args...)
	}
	args := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only"}
	code, held := b.runJSON(owners, args...)
	if code != 1 {
		t.Fatalf("expected declaration hold: %d %+v", code, held)
	}
	acts := checkActs(t, b)
	if len(acts) != 1 {
		t.Fatalf("carry proposal: %v", acts)
	}
	act := acts[0]
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	return b, owners, append(args, "--act", act.ID), marker
}
func TestCarryInterruptedCheckPublicRecovery(t *testing.T) {
	t.Parallel()
	b, owners, args, _ := carryRecoveryBed(t)
	code, result := b.runJSON(owners, args...)
	if code != 0 {
		t.Fatalf("admission: %d %+v", code, result)
	}
	records := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
	results, err := filepath.Glob(filepath.Join(records, "check-*", "result.json"))
	if err != nil || len(results) != 1 {
		t.Fatal(results, err)
	}
	before, err := os.ReadFile(filepath.Join(records, "subject.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(results[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(records, "gate.json")); err != nil {
		t.Fatal(err)
	}
	code, result = b.runJSON(owners, args...)
	if code != 0 {
		t.Fatalf("interrupted check blocked carry: %d %+v", code, result)
	}
	dirs, _ := filepath.Glob(filepath.Join(records, "check-*"))
	results, _ = filepath.Glob(filepath.Join(records, "check-*", "result.json"))
	after, _ := os.ReadFile(filepath.Join(records, "subject.json"))
	if len(dirs) != 2 || len(results) != 1 || string(before) != string(after) {
		t.Fatalf("recovery replaced admission or reused incomplete evidence: dirs=%v results=%v", dirs, results)
	}
}
func TestCarryRetainedFailurePublicRerun(t *testing.T) {
	t.Parallel()
	b, owners, args, marker := carryRecoveryBed(t)
	actID := args[len(args)-1]
	if err := os.WriteFile(marker, []byte("fail"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, result := b.runJSON(owners, args...); code != 1 {
		t.Fatalf("failed check accepted: %d %+v", code, result)
	}
	code, result := b.runJSON(owners, args...)
	if code != 1 || !strings.Contains(result.Summary, "--rerun") || !strings.Contains(result.Summary, args[len(args)-1]) {
		t.Fatalf("failed retained check has no same-act remedy: %d %+v", code, result)
	}
	records := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
	before, _ := os.ReadFile(filepath.Join(records, "subject.json"))
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if code, result = b.runJSON(owners, append(slices.Clone(args), "--rerun")...); code != 0 {
		t.Fatalf("printed rerun failed: %d %+v", code, result)
	}
	results, _ := filepath.Glob(filepath.Join(records, "check-*", "result.json"))
	after, _ := os.ReadFile(filepath.Join(records, "subject.json"))
	if len(results) != 2 || string(before) != string(after) {
		t.Fatal("rerun did not preserve the admitted snapshot")
	}
	for _, path := range results {
		data, err := os.ReadFile(path)
		var retained struct{ Check launch.UnitCheck }
		if err != nil || json.Unmarshal(data, &retained) != nil || retained.Check.ProcessAct != actID {
			t.Fatalf("wrong act: got %q want %q, error %v", retained.Check.ProcessAct, actID, err)
		}
	}
	t.Log(fmt.Sprintf("same act retained by %d real check executions", len(results)))
}
