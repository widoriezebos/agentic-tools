package launch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestUnitCheckComposesItsEvidence(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, audit, selection, tree string
		manual, failed, auditFailed  bool
	}{
		{name: "declared", audit: "printf audits"},
		{name: "true is cheap only", audit: "true"},
		{name: "publication only is cheap only", audit: "metasystem test impact --publication-only"},
		{name: "manual is cheap only", audit: "printf audits", manual: true},
		{name: "failed cheap removes stale evidence", audit: "printf audits", failed: true},
		{name: "failed audit removes stale evidence", audit: "exit 1", auditFailed: true},
		{name: "changed tree is incomplete", audit: "printf audits", tree: "changed"},
		{name: "missing selection is incomplete", audit: "printf audits", selection: "missing"},
		{name: "unreadable selection is incomplete", audit: "printf audits", selection: "unreadable"},
		{name: "malformed selection is incomplete", audit: "printf audits", selection: "malformed"},
		{name: "missing tree seam is incomplete", audit: "printf audits", tree: "missing"},
		{name: "tree read error is incomplete", audit: "printf audits", tree: "error"},
		{name: "different audit is cheap only", audit: "printf replacement"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			f := newUnitFixture(t, "")
			selection := CheckSelection{Version: 1, Base: "base", BaseTree: "base-tree", BaseReason: "frozen",
				Groups: []string{"static", "tests"}, GroupDigests: map[string]string{"static": "definition"},
				PackageTests: map[string][]string{"internal/example": {"TestExample"}}, Mode: "impact", NotRunnable: 2,
				ContractDigest: "contract", EngineDigest: "engine", GoVersion: "go-fixture", GOOS: "fixture-os", GOARCH: "fixture-arch",
				EnvironmentSHA256: "environment", BuildTags: map[string][]string{"tests": {"fixture"}},
				Settings: map[string]string{"landing.impact-max-share": "0.5"}, Workers: 9, VerdictPolicyVersion: 1}
			fixture := filepath.Join(t.TempDir(), "selection.json")
			if err := selection.Write(fixture); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			if err := testexec.WriteFile(filepath.Join(bin, "metasystem"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cheap := `/bin/cp "$FIXTURE" "$METASYSTEM_CHECK_SELECTION"`
			switch row.selection {
			case "missing":
				cheap = ":"
			case "unreadable":
				cheap = `/bin/mkdir "$METASYSTEM_CHECK_SELECTION"`
			case "malformed":
				cheap = `printf '{broken' > "$METASYSTEM_CHECK_SELECTION"`
			}
			if row.failed || row.auditFailed {
				cheap += `; printf stale > "${METASYSTEM_CHECK_SELECTION%/*}/evidence.json"`
			}
			if row.failed {
				cheap += "; exit 1"
			}
			check := UnitCheck{Cheap: cheap, Audits: row.audit, Minutes: 15, Directory: f.worktree,
				Environment: []string{"FIXTURE=" + fixture, "PATH=" + bin},
				Declaration: &UnitDeclaration{Values: [4]string{cheap, row.audit, "15", "full"}, SourceSHA256: "configuration-digest"}}
			if row.manual {
				check.Declaration = nil
			} else if row.name == "different audit is cheap only" {
				check.Declaration.Values[1] = "printf declared"
			}
			context := CheckContext{CheckExecution: CheckExecution{Run: "run", Round: "round-1"}, Worktree: f.worktree,
				Now: f.manager.Now, After: func(time.Duration) <-chan time.Time { return nil }}
			trees := 0
			context.Tree = func() (string, error) {
				trees++
				tree, err := f.runner.CheckTree(f.worktree)
				if trees == 2 && row.tree == "changed" {
					tree += "-changed"
				}
				if row.tree == "error" {
					return tree, errors.New("tree unavailable")
				}
				return tree, err
			}
			if row.tree == "missing" {
				context.Tree = nil
			}
			var execution string
			var exits []CheckExit
			var checkErr error
			f.starter.onStart = func(record Record) error {
				if record.Kind == "proof" {
					execution, exits, checkErr = check.Run(f.worktree, t.TempDir(), context)
				}
				return nil
			}
			if _, err := f.runner.Advance(UnitRequest{Plan: f.plan}); err != nil {
				t.Fatal(err)
			}
			failed := row.failed || row.auditFailed
			if (checkErr != nil) != failed || len(exits) != 2 {
				t.Fatalf("check result changed: exits=%+v error=%v", exits, checkErr)
			}
			data, err := os.ReadFile(filepath.Join(execution, "evidence.json"))
			if failed {
				if !os.IsNotExist(err) {
					t.Fatalf("failed check retained evidence: %s, %v", data, err)
				}
				return
			}
			var evidence CheckEvidence
			if err != nil || json.Unmarshal(data, &evidence) != nil {
				t.Fatalf("evidence: %s, %v", data, err)
			}
			complete := row.tree == "" && row.selection == ""
			audited := complete && !row.manual && row.audit == "printf audits"
			if evidence.Version != 1 || evidence.Source != filepath.Base(execution) || evidence.Manual != row.manual ||
				evidence.Strength.Complete != complete || evidence.Strength.Cheap != (row.selection == "") || evidence.Strength.Audited != audited || evidence.Strength.Coverage || evidence.Strength.Race {
				t.Fatalf("wrong identity or strength: %+v", evidence)
			}
			if !reflect.DeepEqual(evidence.Declaration, check.Declaration) || evidence.Commands != [2]string{cheap, row.audit} || evidence.RunDirectory != "." || evidence.CheapDirectory != "." {
				t.Fatalf("declaration or resolved commands lost: %+v", evidence)
			}
			if row.selection == "" && !reflect.DeepEqual(evidence.Selection, selection) {
				t.Fatalf("selection, base or context changed: %+v", evidence.Selection)
			}
			if complete && (evidence.Content.Before != "round-tree" || evidence.Content.After != "round-tree") {
				t.Fatalf("content tree missing: %+v", evidence.Content)
			}
		})
	}
}
